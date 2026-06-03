---
codd:
  node_id: design:auth-design
  type: design
  depends_on:
  - id: design:system-design
    relation: depends_on
    semantic: technical
  - id: governance:adr-protocol-and-auth
    relation: constrained_by
    semantic: governance
  depended_by:
  - id: design:component-dependency-map
    relation: depends_on
    semantic: technical
  - id: design:auth-sequence
    relation: depends_on
    semantic: technical
  - id: design:credential-storage-detail
    relation: depends_on
    semantic: technical
  conventions:
  - targets:
    - module:auth
    reason: PKCE(RFC 7636)必須。localhostリダイレクトのポート動的選択必須。refresh token失敗時のブラウザ再認証フォールバック必須。
  - targets:
    - module:auth
    - module:config
    reason: 'トークン保存: Linux/WSL=ファイル0600、macOS=Keychain、Windows=Credential Manager。違反はリリース不可。'
  - targets:
    - module:auth
    reason: --no-interactive時はトークン事前存在必須。認証プロンプト表示はエラー終了(コード2)。
  modules:
  - auth
  - config
---

# 認証設計

## 1. Overview

本設計書は `rscli` における認証サブシステム（`module:auth`）の設計を定義する。認証サブシステムは WebFinger（RFC 7033）によるサーバー自動検出、OAuth2 Authorization Code + PKCE（RFC 7636）によるトークン取得、OS 別セキュアストレージへのトークン保護、refresh token の自動更新、および非インタラクティブ環境での動作制御を担う。

### 1.1 リリースブロッキング制約

本設計書は以下の非交渉制約に完全に準拠する。いずれか 1 件でも違反した実装はリリース不可とする。

| # | 制約 | 本設計書での反映箇所 |
|---|---|---|
| AUTH-1 | PKCE（RFC 7636）必須。code_challenge_method は S256 固定。フォールバックなし | 2.3 PKCE 実装 |
| AUTH-2 | localhost リダイレクトのポートは OS 任意ポート割当（port 0）で動的選択 | 2.4 OAuth2 フロー |
| AUTH-3 | refresh token 更新失敗時、インタラクティブモードでブラウザ再認証へフォールバック | 2.6 トークンライフサイクル |
| AUTH-4 | トークン保存: Linux/WSL = ファイルパーミッション 0600、macOS = Keychain、Windows = Credential Manager | 2.5 トークンストア |
| AUTH-5 | `--no-interactive` 時、トークンが事前存在しない場合または期限切れかつ refresh 失敗時は認証プロンプトを表示せずエラー終了（終了コード 2） | 2.7 非インタラクティブ制御 |

### 1.2 スコープ

- **対象モジュール**: `module:auth`、`module:config`（トークンパス解決）
- **対象プラットフォーム**: Linux（amd64 / arm64）、WSL（Linux 扱い）、macOS（amd64 / arm64）、Windows（amd64）
- **プロトコルバージョン**: draft-dejong-remotestorage-22 のみ。他バージョンは接続エラー（終了コード 4）

---

## 2. Architecture

### 2.1 モジュール構成

```
internal/auth/
  webfinger.go           … WebFinger 発見（RFC 7033 + draft-22 プロパティ解析）
  oauth2.go              … Authorization Code フロー制御
  pkce.go                … code_verifier / code_challenge 生成（S256 固定）
  registration.go        … RFC 7591 動的クライアント登録
  tokenstore.go          … TokenStore インターフェース定義
  tokenstore_linux.go    … //go:build linux — ファイルパーミッション 0600 方式
  tokenstore_darwin.go   … //go:build darwin — macOS Keychain Services API
  tokenstore_windows.go  … //go:build windows — Windows Credential Manager API
  refresh.go             … refresh token 自動更新・ブラウザ再認証フォールバック
```

`TokenStore` インターフェース:

```go
type TokenStore interface {
    Save(token *Token) error
    Load() (*Token, error)
    Delete() error
}
```

OS 別実装はビルドタグ（`//go:build linux`、`//go:build darwin`、`//go:build windows`）で切り替える。`module:config` の `os.UserConfigDir()` + `remotestorage-cli/` によるパス解決を利用する。

### 2.2 WebFinger によるサーバー発見

`rscli connect user@host` の最初のステップとして、以下のエンドポイントへ GET リクエストを送信する。

```
GET https://{host}/.well-known/webfinger
    ?resource=acct:{user}@{host}
    &rel=http://tools.ietf.org/id/draft-dejong-remotestorage
```

レスポンスの JRD（JSON Resource Descriptor）から以下の値を抽出する。

| 抽出値 | JRD 上の位置 |
|---|---|
| ストレージ URL | `links[rel=...remotestorage].href` |
| OAuth2 認証エンドポイント | `links[rel=...remotestorage].properties["http://tools.ietf.org/id/draft-dejong-remotestorage#auth"]` |
| プロトコルバージョン | `links[rel=...remotestorage].properties` のバージョンフィールド |

**バージョン検証**: プロトコルバージョンフィールドが draft-dejong-remotestorage-22 以外の値のみを示す場合、接続を拒否してエラー終了（終了コード 4）し、「このサーバーは draft-dejong-remotestorage-22 に対応していません」というメッセージを stderr に出力する。

**フォールバック**: WebFinger リクエストが失敗した場合（ネットワークエラー、404、接続タイムアウト 30 秒超過）、ストレージ URL の直接入力を促すインタラクティブプロンプトを表示する。`--no-interactive` 指定時は終了コード 3 で終了する。

**通信仕様**: すべてのリクエストは HTTPS のみ。HTTP スキームは `module:remotestorage` の HTTP クライアント共通検証により拒否される。タイムアウトは接続 30 秒・アイドル 60 秒。

### 2.3 PKCE 実装（AUTH-1 への準拠）

`auth/pkce.go` で実装する。code_challenge_method は S256 に固定し、plain へのフォールバックは実装しない。

| パラメータ | 仕様 |
|---|---|
| `code_verifier` | 暗号学的ランダムバイト列（43〜128 文字）を Base64URL エンコード |
| `code_challenge` | `BASE64URL(SHA-256(ASCII(code_verifier)))` |
| `code_challenge_method` | `S256`（固定、フォールバックなし） |

PKCE パラメータを含まない認可リクエストは送信しない設計とする。S256 非対応サーバーから `invalid_request` エラーが返った場合は、エラーメッセージで通知して終了コード 2 で終了する。plain フォールバックは行わない（RFC 7636 準拠、セキュリティリスク排除）。

### 2.4 OAuth2 フロー（AUTH-2 への準拠）

#### RFC 7591 動的クライアント登録

WebFinger 発見後、クライアント登録を以下の優先順位で実施する。

1. **RFC 7591 動的登録（第一選択）**: registration endpoint へ POST リクエストを送信する。

   ```json
   {
     "client_name": "remotestorage-cli",
     "redirect_uris": ["http://localhost/callback"],
     "token_endpoint_auth_method": "none"
   }
   ```

   成功時: サーバーから返却された `client_id` を使用し、OS 別セキュアストレージに永続化する。

2. **インタラクティブフォールバック（RFC 7591 非対応時）**: registration endpoint が存在しない、またはリクエストが失敗した場合、`client_id` の手動入力プロンプトを表示する。`--no-interactive` 指定時は終了コード 2 で終了する。

#### 認可フロー

```
1. localhost で一時 HTTP サーバー起動
   - OS 任意ポート割当: net.Listen("tcp", "127.0.0.1:0") でポート 0 を指定し、
     実際に割り当てられたポート番号を取得する（AUTH-2 への準拠）
   - 取得ポートを redirect_uri に組み込む:
     http://localhost:{port}/callback

2. 認可リクエスト URL 構築
   {auth_endpoint}?
     response_type=code
     &client_id={client_id}
     &redirect_uri=http://localhost:{port}/callback
     &scope={scope}        ← デフォルト *:rw、--scope で変更可能
     &code_challenge={challenge}
     &code_challenge_method=S256
     &state={random_state}

3. ブラウザを開いて認可エンドポイントへ誘導
   - Linux: xdg-open
   - macOS: open
   - Windows: start

4. localhost サーバーでコールバック受信
   - state パラメータを検証（CSRF 対策）
   - authorization code を抽出

5. トークンエンドポイントへ code 交換
   POST {token_endpoint}
   Body: grant_type=authorization_code
         &code={code}
         &redirect_uri=http://localhost:{port}/callback
         &client_id={client_id}
         &code_verifier={verifier}

6. AccessToken / RefreshToken を OS 別セキュアストレージへ保存
```

**既存トークンの上書き**: 既存トークンが存在する場合、確認プロンプトを表示してから上書きする。`--yes` フラグ指定時は確認をスキップする。

**スコープ**: デフォルトは `*:rw`。`--scope` オプションで変更可能。

### 2.5 トークンストア（AUTH-4 への準拠）

トークン本体（`access_token`、`refresh_token`、`expires_at`、`client_id`）の保護方式は OS ごとに以下のとおりとする。いずれかの OS で保護が欠如した状態はリリース不可。

| OS | ビルドタグ | 保護方式 | 格納先 | token.json |
|---|---|---|---|---|
| Linux | `//go:build linux` | ファイルパーミッション 0600 | `~/.config/remotestorage-cli/token.json` | トークン本体 + メタデータ |
| WSL | （Linux 扱い） | ファイルパーミッション 0600 | `~/.config/remotestorage-cli/token.json` | トークン本体 + メタデータ |
| macOS | `//go:build darwin` | Keychain Services API | Keychain 項目（service: `remotestorage-cli`） | 生成しない |
| Windows | `//go:build windows` | Credential Manager API | Generic Credential（target: `remotestorage-cli`） | 生成しない |

#### Linux / WSL: ファイルパーミッション 0600 強制

1. `token.json` 作成時: `os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)` でパーミッション `0600` を明示指定する。
2. 既存 `token.json` のパーミッションが `0600` より緩い（例: `0644`）場合、書き込み前に `os.Chmod(path, 0600)` で修正する。
3. `os.Chmod` が失敗した場合はエラー終了（終了コード 4）とし、修正に失敗した旨を stderr に出力する。

#### macOS: Keychain Services API

`auth/tokenstore_darwin.go` で `Security.framework` の `SecItemAdd` / `SecItemCopyMatching` / `SecItemDelete` を使用する。`access_token` と `refresh_token` を個別の Keychain 項目として保存する（kSecClassGenericPassword、service: `remotestorage-cli`、account: `access_token` / `refresh_token`）。

#### Windows: Credential Manager API

`auth/tokenstore_windows.go` で `wincred` パッケージ経由の `CredWrite` / `CredRead` / `CredDelete` を使用する。`access_token` と `refresh_token` を個別の Generic Credential として保存する（target: `remotestorage-cli/access_token`、`remotestorage-cli/refresh_token`）。

### 2.6 トークンライフサイクル（AUTH-3 への準拠）

| イベント | 動作 |
|---|---|
| API リクエスト時にトークン期限切れを検出 | `refresh_token` が存在する場合、`auth/refresh.go` が自動更新を試行 |
| refresh token 更新成功 | 新しい `access_token`（および `refresh_token` が返却された場合はそれも）を OS 別セキュアストレージへ保存し、処理を継続 |
| refresh token 更新失敗（インタラクティブモード） | ブラウザ再認証フローを自動起動（2.4 の認可フローを再実行）（AUTH-3 への準拠） |
| refresh token 更新失敗（`--no-interactive`） | 認証プロンプトを表示せずエラー終了（終了コード 2）（AUTH-5 への準拠） |
| `rscli disconnect` | OS 別セキュアストレージからトークンを削除。Linux/WSL では `token.json` ファイルも削除 |

`auth/refresh.go` はトークンエンドポイントへ以下のリクエストを送信する。

```
POST {token_endpoint}
Body: grant_type=refresh_token
      &refresh_token={refresh_token}
      &client_id={client_id}
```

更新失敗の判定条件: HTTP 400（`invalid_grant`）、HTTP 401、ネットワークエラー（リトライ上限到達後）。

### 2.7 非インタラクティブ制御（AUTH-5 への準拠）

`--no-interactive` フラグが指定された場合、認証に関するすべてのインタラクティブ操作を禁止する。

| 状況 | 動作 |
|---|---|
| トークンが事前存在し有効 | 通常どおり処理を続行 |
| トークンが存在しない | 認証プロンプトを表示せずエラー終了（終了コード 2） |
| トークンが期限切れかつ refresh 成功 | 自動更新して処理を続行 |
| トークンが期限切れかつ refresh 失敗 | ブラウザ再認証を起動せずエラー終了（終了コード 2） |
| RFC 7591 非対応サーバーで client_id が未登録 | 手動入力プロンプトを表示せずエラー終了（終了コード 2） |
| WebFinger 失敗でストレージ URL が不明 | 直接入力プロンプトを表示せずエラー終了（終了コード 3） |

エラー終了時は具体的な失敗理由を stderr に出力する（例: `error: no valid token found; re-run without --no-interactive to authenticate`）。

### 2.8 終了コード体系（認証関連）

| コード | 認証サブシステムでの発生条件 |
|---|---|
| 0 | 認証成功（`rscli connect`）、切断成功（`rscli disconnect`） |
| 2 | トークン不在（`--no-interactive`）、refresh 失敗（`--no-interactive`）、S256 非対応サーバー、RFC 7591 非対応（`--no-interactive`） |
| 3 | WebFinger ネットワークエラー（タイムアウト・DNS 解決失敗）、WebFinger フォールバック時の URL 未入力（`--no-interactive`） |
| 4 | プロトコルバージョン不一致（draft-22 非対応サーバー）、token.json パーミッション修正失敗、Keychain/Credential Manager API エラー |

### 2.9 セキュリティ制御まとめ

| 制御 | 実装箇所 | 根拠 |
|---|---|---|
| PKCE S256 必須 | `auth/pkce.go` | authorization code 横取り攻撃（RFC 7636 Section 1）の防止 |
| state パラメータ検証 | `auth/oauth2.go` | CSRF 防止 |
| client_secret 不使用 | 全実装 | パブリッククライアントとしての一貫性、シークレット漏洩リスク排除 |
| ファイルパーミッション 0600（Linux/WSL） | `auth/tokenstore_linux.go` | 他ユーザーによるトークン読み取り防止 |
| OS ネイティブキーリング（macOS/Windows） | `auth/tokenstore_darwin.go`、`auth/tokenstore_windows.go` | OS セキュリティ境界によるトークン保護 |
| HTTPS 強制 | `module:remotestorage` HTTP クライアント | トークン交換の平文送信防止（`--insecure` は TLS 証明書検証スキップのみ、HTTP 通信は許可しない） |
| localhost リダイレクト限定 | `auth/oauth2.go` | 認可コードの外部サーバーへの漏洩防止 |
| Implicit Grant 不採用 | 設計方針 | OAuth 2.1 方向性との整合、access_token の URL フラグメント露出防止 |

---

## 3. Open Questions

| # | 問い | 背景 | 判断時期 |
|---|---|---|---|
| OQ-AUTH-1 | S256 非対応サーバーへの接続要望が発生した場合、plain フォールバックを追加するか | RFC 7636 は S256 を推奨。plain フォールバックは authorization code 横取り攻撃への耐性を低下させるため、原則追加しない方針。ただし特定サーバーへの接続需要が生じた場合に再検討が必要 | S256 非対応サーバーへの接続要望が発生した時点 |
| OQ-AUTH-2 | RFC 7591 動的登録時に送信するメタデータ（`client_uri`、`logo_uri`、`software_version` 等）の最適構成 | 現在の最小構成（`client_name`、`redirect_uris`、`token_endpoint_auth_method=none`）でサーバーが登録を拒否するケースが存在する可能性がある。主要 remoteStorage サーバー（php-remote-storage、armadietto、remotestorage-server）での動作検証が必要 | 主要サーバーでの動作検証完了後 |
| OQ-AUTH-3 | Linux デスクトップ向けキーリング統合（GNOME Keyring / KDE Wallet）の導入 | 現在は全 Linux 環境でファイルパーミッション 0600 方式を採用。デスクトップ環境では D-Bus 経由のキーリング統合（`github.com/zalando/go-keyring` 等）が UX 向上につながる可能性がある。ヘッドレス・WSL 環境では 0600 ファイル方式へのフォールバックを維持する必要がある | デスクトップ Linux ユーザーからのフィードバックに基づき判断 |
| OQ-AUTH-4 | 複数アカウント（複数サーバー）対応時のトークンストア構造 | 現在は 1 アカウントのみをサポート。トークン保存構造を `user@host` キー付き構造（`token.json` のトップレベルをマップ化）またはアカウント別ファイル（`token_{host}_{user}.json`）に分離する設計変更が必要。Keychain / Credential Manager の項目命名規則も変更が必要 | 複数アカウント需要がユーザーフィードバックで確認された時点 |
| OQ-AUTH-5 | OAuth 2.1 正式 RFC 化時の差分検証 | 現設計（Authorization Code + PKCE 必須、client_secret 不要、Implicit Grant 不使用）は OAuth 2.1（draft-ietf-oauth-v2-1）の方向性と整合しているが、正式化時に要件差分が生じないか確認が必要 | OAuth 2.1 が正式 RFC として公開された時点 |
| OQ-AUTH-6 | refresh token の有効期限管理とローテーション戦略 | サーバーによっては refresh token にも有効期限を設定する場合がある。現在の設計では refresh token の有効期限フィールド（`refresh_token_expires_in`）を token.json / セキュアストレージに保存・検証する処理を明示していない。refresh token 失効時の事前検出が UX 改善につながる可能性がある | 主要 remoteStorage サーバーでの refresh token 有効期限仕様の確認後 |
