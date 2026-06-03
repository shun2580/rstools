---
codd:
  node_id: design:system-design
  type: design
  depends_on:
  - id: test:acceptance-criteria
    relation: constrained_by
    semantic: governance
  - id: governance:adr-protocol-and-auth
    relation: constrained_by
    semantic: governance
  - id: governance:adr-cross-platform
    relation: constrained_by
    semantic: governance
  depended_by:
  - id: design:auth-design
    relation: depends_on
    semantic: technical
  - id: design:sync-transfer-design
    relation: depends_on
    semantic: technical
  - id: design:cli-design
    relation: depends_on
    semantic: technical
  - id: operations:runbook
    relation: depends_on
    semantic: technical
  conventions:
  - targets:
    - module:remotestorage
    reason: HTTPS必須(--insecureは開発・テスト環境のみ)。本番通信でのHTTP許容はリリース不可。
  - targets:
    - module:auth
    reason: トークン保護(パーミッション0600/Keychain/Credential Manager)をアーキテクチャレベルで保証必須。
  - targets:
    - module:cli
    reason: シグナル処理によるsync_state.jsonのatomic write保証はデータ整合性のリリース必須要件。
  - targets:
    - module:cli
    reason: ロックファイルによる同時実行制御必須。PID記録とstale自動削除を含む。
  modules:
  - auth
  - sync
  - transfer
  - cli
  - config
  - remotestorage
---

# システム設計

## 1. Overview

`rscli` は remoteStorage プロトコル（draft-dejong-remotestorage-22）を操作する Go 製 CLI ツールである。WebFinger によるサーバー自動検出、OAuth2 Authorization Code + PKCE による認証、ファイル操作（ls / get / put / rm）、双方向同期（push / pull）、リトライ、クロスプラットフォームのトークン保護を提供する。

対象プラットフォームは Linux（amd64 / arm64）、macOS（amd64 / arm64）、Windows（amd64）の 3 OS であり、いずれか 1 つでも未対応の場合リリース不可とする。WSL 環境は Linux 扱いとする。

### 1.1 リリースブロッキング制約

本設計書は以下の非交渉制約に準拠し、各制約がアーキテクチャ上どのように保証されるかを本文中で明示する。

| # | 対象モジュール | 制約 | 本設計書での反映箇所 |
|---|---|---|---|
| 1 | `module:remotestorage` | HTTPS 必須。`--insecure` は開発・テスト環境のみ。本番通信での HTTP 許容はリリース不可 | 2.3 通信レイヤー |
| 2 | `module:auth` | トークン保護（パーミッション 0600 / Keychain / Credential Manager）をアーキテクチャレベルで保証 | 2.4 トークンストア |
| 3 | `module:cli` | シグナル処理による `sync_state.json` の atomic write 保証はデータ整合性のリリース必須要件 | 2.7 同期エンジン |
| 4 | `module:cli` | ロックファイルによる同時実行制御必須。PID 記録と stale 自動削除を含む | 2.8 排他制御 |

追加のリリースブロッキング制約:

- **終了コード体系**: 0（成功）/ 1（部分的失敗）/ 2（認証失敗）/ 3（ネットワークエラー）/ 4（その他）を正確に返すこと。
- **リトライ仕様**: HTTP 429 / 500 / 502 / 503 を対象に exponential backoff（初回 1 秒・最大 30 秒・フルジッター）でリトライ。429 は `Retry-After` ヘッダを尊重。
- **プロトコル準拠**: draft-dejong-remotestorage-22 準拠必須。プロトコルバージョン逸脱はリリース不可。
- **PKCE 必須**: OAuth2 フローで PKCE（RFC 7636、S256）を省略した実装はリリース不可。
- **RFC 7591 動的クライアント登録優先**: 動的登録を第一手段とし、非対応サーバーに対してのみインタラクティブフォールバックを提供。
- **3 OS 配布**: Linux / macOS / Windows 全てのビルド済みバイナリが GitHub Releases で配布されていること。

### 1.2 プロトコルバージョン

唯一の対象バージョンは draft-dejong-remotestorage-22 とする。WebFinger 応答の JRD からストレージ URL・OAuth2 認証エンドポイントを取得する際のプロパティ名、ディレクトリリスティングの JSON 構造、ETag 付き条件付きリクエスト（`If-Match` / `If-None-Match`）は全て draft-22 の定義に準拠する。WebFinger 応答のバージョンフィールド検証を接続時に実施し、draft-22 以外のバージョンのみを返すサーバーには接続エラー（終了コード 4）で応答する。

---

## 2. Architecture

### 2.1 モジュール構成

```
cmd/rscli/
  main.go                  … エントリポイント、サブコマンド登録、シグナルハンドラ初期化

internal/
  auth/                    … module:auth — OAuth2 認証・トークンライフサイクル
    webfinger.go           … WebFinger 発見（RFC 7033 + draft-22）
    oauth2.go              … Authorization Code + PKCE フロー
    pkce.go                … code_verifier / code_challenge 生成（S256 固定）
    registration.go        … RFC 7591 動的クライアント登録
    tokenstore.go          … TokenStore インターフェース定義
    tokenstore_linux.go    … //go:build linux — 0600 パーミッション方式
    tokenstore_darwin.go   … //go:build darwin — Keychain 方式
    tokenstore_windows.go  … //go:build windows — Credential Manager 方式
    refresh.go             … refresh token 自動更新・ブラウザ再認証フォールバック

  remotestorage/           … module:remotestorage — プロトコル実装
    client.go              … HTTPクライアント（HTTPS 強制、TLS 設定）
    directory.go           … ディレクトリリスティング（draft-22 JSON 形式）
    etag.go                … ETag / If-Match / If-None-Match 処理
    path.go                … パス URL エンコーディング自動処理

  sync/                    … module:sync — 同期エンジン
    engine.go              … push / pull 同期ロジック
    state.go               … sync_state.json 読み書き（atomic write）
    conflict.go            … コンフリクト検出・スキップ / --force 上書き
    ignore.go              … .rsignore / --exclude パターンマッチ
    delete.go              … 削除伝播 / --no-delete 制御

  transfer/                … module:transfer — ファイル転送
    stream.go              … ストリーミング転送（メモリ非読込）
    parallel.go            … 並列ワーカープール（デフォルト 3）
    progress.go            … プログレス表示（stderr、N/M files 形式）
    retry.go               … リトライロジック（429/500/502/503、exponential backoff）
    contenttype.go         … Content-Type 自動検出 / --content-type 上書き

  config/                  … module:config — 設定・パス解決
    dir.go                 … os.UserConfigDir() + remotestorage-cli/
    lock.go                … ロックファイル管理（PID 記録、stale 検出）
    lock_unix.go           … //go:build !windows — シグナル 0 送信
    lock_windows.go        … //go:build windows — OpenProcess 判定

  cli/                     … module:cli — コマンド定義・シグナル処理
    connect.go             … rscli connect [--yes] [--scope]
    disconnect.go          … rscli disconnect
    ls.go                  … rscli ls [--json] [-R]
    get.go                 … rscli get
    put.go                 … rscli put [--content-type]
    rm.go                  … rscli rm [-r]
    push.go                … rscli push [--no-delete] [--force] [--exclude] [--dry-run]
    pull.go                … rscli pull [--no-delete] [--force] [--exclude] [--dry-run]
    flags.go               … 共通フラグ（--verbose, --no-interactive, --insecure, --parallel, --reset-state, --force-unlock, --dry-run）
    exit.go                … 終了コード定数・エラーマッピング
    signal.go              … SIGINT/SIGTERM ハンドラ
```

### 2.2 コマンド体系と終了コード

#### サブコマンド一覧

| コマンド | 用途 | 主要フラグ |
|---|---|---|
| `rscli connect user@host` | WebFinger 発見 → OAuth2 認証 → トークン保存 | `--yes`, `--scope`, `--insecure`, `--no-interactive` |
| `rscli disconnect` | トークン削除 | — |
| `rscli ls /path` | ファイル一覧表示 | `--json`, `-R` |
| `rscli get /remote ./local` | ファイル単体ダウンロード | — |
| `rscli put ./local /remote` | ファイル単体アップロード | `--content-type` |
| `rscli rm /path` | ファイル / ディレクトリ削除 | `-r` |
| `rscli push ./local /remote` | ローカル→リモート同期 | `--no-delete`, `--force`, `--exclude`, `--dry-run`, `--parallel`, `--reset-state`, `--force-unlock` |
| `rscli pull /remote ./local` | リモート→ローカル同期 | `--no-delete`, `--force`, `--exclude`, `--dry-run`, `--parallel`, `--reset-state`, `--force-unlock` |

#### 終了コード体系（リリースブロッキング）

| コード | 意味 | 発生条件 |
|---|---|---|
| 0 | 成功 | 全操作完了 |
| 1 | 部分的失敗 | 一部ファイルの転送・同期が失敗し、一部が成功 |
| 2 | 認証失敗 | トークン無効、refresh 失敗 + `--no-interactive`、RFC 7591 非対応 + `--no-interactive` |
| 3 | ネットワークエラー | DNS 解決失敗、接続タイムアウト（30 秒）、TLS エラー |
| 4 | その他エラー | ロックファイル競合、`sync_state.json` 破損、プロトコルバージョン不一致 |

`--dry-run` フラグは `push` / `pull` / `rm` にのみ適用し、実際の変更を行わず実行予定の操作を表示する。`connect` / `disconnect` / `ls` / `get` / `put` では無視する（エラーにはしない）。

### 2.3 通信レイヤー（module:remotestorage）

**HTTPS 強制（リリースブロッキング制約 #1 への準拠）**: `module:remotestorage` の HTTP クライアントは全リクエストの URL スキームが `https://` であることを送信前に検証する。スキームが `http://` の場合はリクエストを拒否しエラーを返す。この検証は `--insecure` フラグの有無に関係なく適用される。`--insecure` フラグは自己署名証明書の TLS 検証スキップのみを制御し、HTTP（非暗号化）通信を許可するものではない。`--insecure` の利用は開発・テスト環境に限定される。

**タイムアウト設定**:

| 種別 | 値 | 超過時の動作 |
|---|---|---|
| 接続タイムアウト | 30 秒 | ネットワークエラー（終了コード 3） |
| アイドルタイムアウト | 60 秒 | 接続切断、リトライ対象外 |

**リトライ（リリースブロッキング）**: `module:transfer` のリトライロジックは以下の仕様に厳密に従う。

- **対象ステータス**: 429 / 500 / 502 / 503 のみ。400 / 401 / 404 等はリトライしない。
- **最大リトライ回数**: 3 回。
- **バックオフ方式**: exponential backoff。初回待機 1 秒、最大待機 30 秒、フルジッター（`sleep = random(0, min(cap, base * 2^attempt))`）。
- **429 の Retry-After**: `Retry-After` ヘッダが存在する場合、算出した backoff 値と `Retry-After` 値の大きい方を待機時間とする。
- **並列転送時の 429**: いずれか 1 ワーカーが 429 を受信した場合、全ワーカーを一時停止し、`Retry-After` の値を全ワーカーに適用して一斉に再開する。
- **部分失敗時の継続**: リトライ上限に達して失敗したファイルがあっても処理を続行し、全処理完了後にエラー一覧をまとめて表示する。終了コードは 1（部分的失敗）。

**パスエンコーディング**: リモートパスの URL エンコーディングは CLI 側で自動処理する。日本語ファイル名・特殊文字を含むパスを透過的に扱い、ユーザーは生のパスを入力する。ローカルファイルパスは `filepath` パッケージで OS 標準の区切り文字に変換する。remoteStorage との通信パスは常に `/` を使用する。

### 2.4 トークンストア（module:auth）

**トークン保護（リリースブロッキング制約 #2 への準拠）**: `module:auth` は `TokenStore` インターフェース（`Save(token)` / `Load() token` / `Delete()`）を定義し、ビルドタグで OS 別実装を切り替える。トークン本体（AccessToken / RefreshToken）のストレージ方式は以下のとおりであり、いずれかの OS で保護が欠如した状態はリリース不可とする。

| OS | ビルドタグ | 保護方式 | トークン本体の格納先 | token.json の役割 |
|---|---|---|---|---|
| Linux | `//go:build linux` | ファイルパーミッション 0600 | `~/.config/remotestorage-cli/token.json` | トークン本体 + メタデータ |
| WSL | （Linux 扱い） | ファイルパーミッション 0600 | `~/.config/remotestorage-cli/token.json` | トークン本体 + メタデータ |
| macOS | `//go:build darwin` | macOS Keychain | Keychain 項目 | 不使用（作成しない） |
| Windows | `//go:build windows` | Credential Manager | Generic Credential | 不使用（作成しない） |

Linux/WSL における 0600 パーミッション強制の実装:
1. `token.json` 作成時に `os.OpenFile` でパーミッション `0600` を明示指定する。
2. 既存 `token.json` のパーミッションが `0600` より緩い場合、書き込み前に `os.Chmod` で修正する。
3. パーミッション修正に失敗した場合はエラー終了（終了コード 4）。

### 2.5 認証フロー（module:auth）

`rscli connect user@host` の実行フローは以下のとおりである。

```
1. WebFinger 発見
   GET https://{host}/.well-known/webfinger
       ?resource=acct:{user}@{host}
       &rel=http://tools.ietf.org/id/draft-dejong-remotestorage
   → JRD からストレージ URL (href) と OAuth2 認証エンドポイント (properties) を抽出
   → 失敗時: ストレージ URL 直接入力フォールバックプロンプトを表示

2. RFC 7591 動的クライアント登録（優先）
   POST {registration_endpoint}
   Body: { client_name: "remotestorage-cli",
           redirect_uris: ["http://localhost/callback"],
           token_endpoint_auth_method: "none" }
   → 成功時: 返却された client_id を使用
   → 失敗時: インタラクティブプロンプトで client_id 手動入力
             （--no-interactive 時はエラー終了、終了コード 2）

3. PKCE 生成
   code_verifier: 暗号学的ランダム文字列
   code_challenge: SHA-256(code_verifier) の Base64URL エンコード
   code_challenge_method: S256（固定、フォールバックなし）

4. OAuth2 Authorization Code フロー
   - localhost で一時 HTTP サーバー起動（ポート 0 で動的割当）
   - ブラウザを開いて認可エンドポイントへリダイレクト
     パラメータ: response_type=code, client_id, redirect_uri=http://localhost:{port}/callback,
                scope=*:rw（--scope で変更可能）, code_challenge, code_challenge_method=S256, state
   - コールバックで認可コードを受け取り、トークンエンドポイントに交換
   - AccessToken / RefreshToken を OS 別セキュアストレージに保存

5. 既存トークンの上書き確認
   - 既存トークンがある場合: 確認プロンプト表示後に上書き
   - --yes フラグ: 確認プロンプトスキップ
```

**トークンライフサイクル**:

| イベント | 動作 |
|---|---|
| API リクエスト時にトークン期限切れ検出 | refresh token が存在する場合は自動更新を試行 |
| refresh token 更新成功 | 新しい AccessToken（および RefreshToken）を保存して処理を継続 |
| refresh token 更新失敗（インタラクティブ） | ブラウザ再認証フローを自動起動 |
| refresh token 更新失敗（`--no-interactive`） | エラー終了（終了コード 2） |
| `rscli disconnect` | OS 別セキュアストレージからトークンを削除 |

### 2.6 ファイル操作（module:transfer）

#### ls

`rscli ls /path` は remoteStorage ディレクトリリスティング API（draft-22 JSON 形式）を呼び出し、ファイル名・サイズ・更新日時を表示する。`--json` で JSON 形式出力、`-R` でサブディレクトリを含む再帰的表示を行う。

#### get / put

- `get` はファイル単体のダウンロードのみ対応。ディレクトリパス指定時はエラー。
- `put` はファイル単体のアップロードのみ対応。ディレクトリパス指定時はエラーメッセージを表示し `push` の使用を案内。
- Content-Type 自動検出は `mime.TypeByExtension` ベース。不明拡張子は `application/octet-stream`。`--content-type` で明示上書き可能。

#### rm

`rscli rm /path` で単一ファイル削除。`rscli rm -r /path/` で再帰削除（確認プロンプトあり）。

#### 転送共通

- **ストリーミング方式**: ファイル全体をメモリに読み込まない。ファイルサイズ上限なし。
- **並列転送**: デフォルト 3 ワーカー。`--parallel N` で変更可能。
- **プログレス表示**: stderr に `N/M files` 形式で出力。`golang.org/x/term.IsTerminal()` で TTY 判定を行い、非 TTY 時は自動抑制。
- **シンボリックリンク・特殊ファイル**: OS 問わずスキップし、warning を stderr に出力。

### 2.7 同期エンジン（module:sync）

#### 変更検出

| 対象 | 検出方式 | フォールバック |
|---|---|---|
| リモートファイル | ETag（優先） | `Last-Modified` ヘッダ（ETag 非対応サーバー） |
| ローカルファイル | mtime | — |

#### sync_state.json

`sync_state.json` はファイル毎に ETag・mtime を記録し、`{設定ディレクトリ}/sync_state.json` に配置する。

**atomic write（リリースブロッキング制約 #3 への準拠）**: `sync_state.json` の書き込みは一時ファイル書き込み → `os.Rename` の 2 段階で実行し、書き込み中の中断でファイルが破損しないことを保証する。この atomic write は `module:cli` のシグナルハンドラと連携し、SIGINT / SIGTERM 受信時に以下の動作を行う:

1. 現在転送中のファイルの完了を待たない。
2. 完了済みファイルのみ `sync_state.json` に記録する。
3. 再実行で中断箇所から再開可能。

**破損時の動作**: `sync_state.json` のパースに失敗した場合、エラーメッセージを表示して終了コード 4 で終了する。自動修復は行わない。`--reset-state` フラグで `sync_state.json` を削除し、初回 sync 扱いにリセットする。

#### 同期ルール

| シナリオ | push | pull |
|---|---|---|
| 初回 sync（`sync_state.json` 未存在） | ローカルのみ→アップロード、リモートのみ→スキップ | リモートのみ→ダウンロード、ローカルのみ→スキップ |
| 削除伝播 | ローカル削除→リモート削除 | リモート削除→ローカル削除 |
| `--no-delete` 指定時 | 削除伝播無効 | 削除伝播無効 |
| コンフリクト（双方変更） | デフォルト: スキップ + 通知 | デフォルト: スキップ + 通知 |
| `--force` 指定時 | 強制上書き | 強制上書き |

#### 除外パターン

- `.rsignore` ファイル: sync ルートに配置。`.gitignore` 形式でパターン指定。
- `--exclude` オプション: コマンドラインから除外パターンを指定。`.rsignore` との併用時は両方を適用。
- 空ディレクトリは remoteStorage プロトコル仕様に従いスキップ。

### 2.8 排他制御（module:config）

**ロックファイル（リリースブロッキング制約 #4 への準拠）**: `push` / `pull` 実行時に `{設定ディレクトリ}/lock` ファイルを作成し、実行中プロセスの PID を記録する。排他制御の仕様は以下のとおり。

| 動作 | 詳細 |
|---|---|
| ロック取得 | `lock` ファイルに PID を書き込み。ファイル作成は `O_CREATE \| O_EXCL` で排他的に行う |
| stale 検出 | 起動時に `lock` ファイルが存在する場合、記録された PID のプロセス存在を確認。Unix 系: シグナル 0 送信（`//go:build !windows`）。Windows: `OpenProcess` API（`//go:build windows`）。プロセス不在時は stale として自動削除 |
| 競合時 | ロックファイルが有効な状態（PID のプロセスが存在）で新たに push/pull を実行→終了コード 4 でエラー終了 |
| 手動解除 | `--force-unlock` フラグでロックファイルを強制削除 |

### 2.9 設定ディレクトリ（module:config）

Go 標準ライブラリの `os.UserConfigDir()` を使用し、戻り値配下に `remotestorage-cli/` サブディレクトリを作成する。

| OS | 設定パス |
|---|---|
| Linux | `$XDG_CONFIG_HOME/remotestorage-cli/`（未設定時 `~/.config/remotestorage-cli/`） |
| macOS | `~/Library/Application Support/remotestorage-cli/` |
| Windows | `%AppData%\remotestorage-cli\` |
| WSL | `~/.config/remotestorage-cli/`（Linux 扱い） |

配置されるファイル:

| ファイル | 用途 |
|---|---|
| `token.json` | OAuth トークン（Linux/WSL のみ。macOS / Windows では不使用） |
| `sync_state.json` | 同期状態（ファイル毎の ETag / mtime） |
| `lock` | ロックファイル（PID 記録） |

### 2.10 クロスプラットフォーム配布

GitHub Actions で Go のクロスコンパイル（`GOOS` / `GOARCH` 環境変数指定）を実行し、タグプッシュ時に自動リリースする。全バイナリのビルド成功をリリースの前提条件とする。

| OS | アーキテクチャ | バイナリ名 |
|---|---|---|
| Linux | amd64 | `rscli-linux-amd64` |
| Linux | arm64 | `rscli-linux-arm64` |
| macOS | amd64 | `rscli-darwin-amd64` |
| macOS | arm64 | `rscli-darwin-arm64` |
| Windows | amd64 | `rscli-windows-amd64.exe` |

`go install github.com/<org>/remotestorage-cli/cmd/rscli@latest` によるソースからのインストールも可能とする。

### 2.11 非交渉制約の設計レベル保証まとめ

| 制約 | アーキテクチャ上の保証メカニズム |
|---|---|
| HTTPS 必須（制約 #1） | `module:remotestorage` の HTTP クライアントが送信前に URL スキームを検証し、`http://` を拒否。`--insecure` は TLS 証明書検証スキップのみを制御し、HTTP 通信は許可しない |
| トークン保護（制約 #2） | `TokenStore` インターフェースとビルドタグによる OS 別実装強制。Linux: 0600 パーミッション（作成時設定 + 既存ファイル修正）。macOS: Keychain API。Windows: Credential Manager API。token.json は macOS/Windows では生成しない |
| atomic write（制約 #3） | `sync_state.json` は一時ファイル書き込み → `os.Rename` で更新。シグナルハンドラが SIGINT/SIGTERM 受信時に完了済みファイルのみ記録し、進行中の書き込みを中断 |
| ロックファイル（制約 #4） | `O_CREATE \| O_EXCL` による排他的ファイル作成。PID 記録と OS 別プロセス存在確認（Unix: シグナル 0、Windows: OpenProcess）による stale 検出。`--force-unlock` による手動解除 |
| 終了コード体系 | `cli/exit.go` に定数定義。全コマンドハンドラはエラー種別（認証 / ネットワーク / 部分失敗 / その他）をマッピングして適切なコードを返す |
| リトライ仕様 | `transfer/retry.go` に 429/500/502/503 判定、exponential backoff（初回 1 秒・最大 30 秒・フルジッター）、`Retry-After` ヘッダ尊重、429 受信時の全ワーカー一時停止を実装 |
| プロトコル準拠 | WebFinger 応答のバージョンフィールドを検証し、draft-22 非対応サーバーを拒否 |
| PKCE 必須 | `auth/pkce.go` で S256 固定。PKCE パラメータなしの認可リクエストを送信しない設計 |

---

## 3. Open Questions

| # | 問い | 背景 | 判断時期 |
|---|---|---|---|
| OQ-1 | draft-22 以降のドラフトバージョンが公開された場合、バージョンネゴシエーション層を導入するか | 現在は draft-22 固定だが、主要サーバー実装が新ドラフトをリリースした場合に対応範囲が問題となる | 主要サーバー実装（php-remote-storage、armadietto、remotestorage-server）が draft-22 以降をリリースした時点 |
| OQ-2 | RFC 7591 動的クライアント登録時の送信メタデータ（client_name、client_uri、logo_uri 等）の最適構成 | 現在は最小構成（client_name=remotestorage-cli、redirect_uris、token_endpoint_auth_method=none）としているが、サーバー実装によっては追加メタデータが必要となる可能性がある | 主要 remoteStorage サーバーでの動作検証完了後 |
| OQ-3 | 複数アカウント（複数サーバー）対応の設計 | 現スコープは 1 アカウントのみ。将来的にトークン保存構造をアカウント別（`user@host` キー付き構造またはアカウント別ファイル）に分離する必要がある | ユーザーフィードバックで複数アカウント需要が確認された時点 |
| OQ-4 | PKCE の S256 非対応サーバーへの対応方針 | RFC 7636 は S256 を推奨しており、plain へのフォールバックはセキュリティリスクを伴う。現在は S256 非対応サーバーにはエラーメッセージで通知する方針 | S256 非対応サーバーへの接続要望が発生した時点 |
| OQ-5 | Linux デスクトップ向けキーリング統合（GNOME Keyring / KDE Wallet）の導入 | 現在は全 Linux 環境でファイルパーミッション 0600 方式を採用しているが、デスクトップ環境では D-Bus 経由のキーリング統合が UX を向上させる可能性がある。ヘッドレス環境では 0600 フォールバックを維持する | デスクトップ Linux ユーザーからのフィードバックに基づき判断 |
| OQ-6 | Windows arm64 ビルドの追加 | 初期リリースのビルドマトリクスに Windows arm64 は含まれていない。Snapdragon 搭載 PC の普及状況に応じて追加を判断する | 需要調査結果に基づく |
| OQ-7 | OAuth 2.1（draft-ietf-oauth-v2-1）正式 RFC 化時の再検証 | 現設計（Authorization Code + PKCE 必須、client_secret 不要、Implicit Grant 不使用）は OAuth 2.1 ドラフトの方向性と整合しているが、正式化時に差分がないか確認が必要 | OAuth 2.1 が正式 RFC として公開された時点 |
