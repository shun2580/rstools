---
codd:
  node_id: design:cli-design
  type: design
  depends_on:
  - id: design:system-design
    relation: depends_on
    semantic: technical
  depended_by:
  - id: design:component-dependency-map
    relation: depends_on
    semantic: technical
  conventions:
  - targets:
    - cli_command:rscli
    reason: 終了コード(0/1/2/3/4)の意味は厳守。部分的失敗を0で返すとユーザーのスクリプトが誤動作するためリリース不可。
  - targets:
    - cli_command:rscli
    reason: '--dry-run適用範囲: push/pull/rmのみ。connect/disconnect/ls/get/putでは無視。適用範囲の誤りはリリース不可。'
  - targets:
    - module:cli
    reason: プログレス表示はstderr出力。非TTY時は自動抑制。stdout汚染はパイプ連携を破壊するためリリース不可。
  - targets:
    - module:cli
    reason: シンボリックリンク・特殊ファイルはスキップしてwarning表示必須。追従するとセキュリティリスク。
  modules:
  - cli
---

# CLI設計

## 1. Overview

`rscli` は remoteStorage プロトコル（draft-dejong-remotestorage-22）を操作する Go 製 CLI ツールである。WebFinger によるサーバー自動検出、OAuth2 Authorization Code + PKCE による認証、ファイル操作（`ls` / `get` / `put` / `rm`）、双方向同期（`push` / `pull`）、リトライ、クロスプラットフォームのトークン保護を提供する。

対象プラットフォームは Linux（amd64 / arm64）、macOS（amd64 / arm64）、Windows（amd64）の 3 OS であり、いずれか 1 つでも未対応の場合リリース不可とする。WSL 環境は Linux 扱いとする。

本設計書は以下のリリースブロッキング制約に準拠し、各制約の実現方法を本文中で明示する。

| # | 対象 | 制約 | 本設計書での反映箇所 |
|---|---|---|---|
| RB-1 | `cli_command:rscli` | 終了コード（0/1/2/3/4）の意味を厳守。部分的失敗を 0 で返すとユーザーのスクリプトが誤動作するためリリース不可 | 2.3 終了コード体系 |
| RB-2 | `cli_command:rscli` | `--dry-run` 適用範囲は `push` / `pull` / `rm` のみ。`connect` / `disconnect` / `ls` / `get` / `put` では無視（エラーにしない）。適用範囲の誤りはリリース不可 | 2.4 フラグ仕様 |
| RB-3 | `module:cli` | プログレス表示は stderr 出力。非 TTY 時は自動抑制。stdout 汚染はパイプ連携を破壊するためリリース不可 | 2.6 プログレス表示 |
| RB-4 | `module:cli` | シンボリックリンク・特殊ファイルはスキップして warning 表示必須。追従するとセキュリティリスク | 2.7 ファイルスキャン |

加えて、システム設計から継承する制約を本設計でも適用する。

| # | 対象 | 制約 |
|---|---|---|
| SD-1 | `module:remotestorage` | HTTPS 必須。`--insecure` は開発・テスト環境のみ。本番通信での HTTP 許容はリリース不可 |
| SD-2 | `module:auth` | トークン保護（パーミッション 0600 / Keychain / Credential Manager）をアーキテクチャレベルで保証 |
| SD-3 | `module:cli` | `sync_state.json` の atomic write 保証はデータ整合性のリリース必須要件 |
| SD-4 | `module:cli` | ロックファイルによる同時実行制御必須。PID 記録と stale 自動削除を含む |

---

## 2. Architecture

### 2.1 コマンド体系

`rscli` が提供するサブコマンド一覧を以下に示す。サブコマンド名はすべて kebab-case とする。

| サブコマンド | 用途 | 主要フラグ |
|---|---|---|
| `rscli connect user@host` | WebFinger 発見 → OAuth2 認証 → トークン保存 | `--yes`, `--scope`, `--insecure`, `--no-interactive` |
| `rscli disconnect` | トークン削除 | — |
| `rscli ls /path` | ファイル一覧表示 | `--json`, `-R` |
| `rscli get /remote ./local` | ファイル単体ダウンロード | — |
| `rscli put ./local /remote` | ファイル単体アップロード | `--content-type` |
| `rscli rm /path` | ファイル / ディレクトリ削除 | `-r`, `--dry-run` |
| `rscli push ./local /remote` | ローカル→リモート同期 | `--no-delete`, `--force`, `--exclude`, `--dry-run`, `--parallel`, `--reset-state`, `--force-unlock` |
| `rscli pull /remote ./local` | リモート→ローカル同期 | `--no-delete`, `--force`, `--exclude`, `--dry-run`, `--parallel`, `--reset-state`, `--force-unlock` |

### 2.2 モジュール構成

```
cmd/rscli/
  main.go                  … エントリポイント、サブコマンド登録、シグナルハンドラ初期化

internal/
  auth/                    … module:auth — OAuth2 認証・トークンライフサイクル
    webfinger.go
    oauth2.go
    pkce.go                … code_verifier / code_challenge 生成（S256 固定）
    registration.go        … RFC 7591 動的クライアント登録
    tokenstore.go          … TokenStore インターフェース定義
    tokenstore_linux.go    … //go:build linux — 0600 パーミッション方式
    tokenstore_darwin.go   … //go:build darwin — Keychain 方式
    tokenstore_windows.go  … //go:build windows — Credential Manager 方式
    refresh.go

  remotestorage/           … module:remotestorage — プロトコル実装
    client.go              … HTTPS 強制検証を含む HTTP クライアント
    directory.go
    etag.go
    path.go

  sync/                    … module:sync — 同期エンジン
    engine.go
    state.go               … sync_state.json 読み書き（atomic write）
    conflict.go
    ignore.go
    delete.go

  transfer/                … module:transfer — ファイル転送
    stream.go
    parallel.go
    progress.go            … stderr 出力・非 TTY 自動抑制
    retry.go
    contenttype.go

  config/                  … module:config — 設定・パス解決
    dir.go
    lock.go
    lock_unix.go           … //go:build !windows — シグナル 0 送信
    lock_windows.go        … //go:build windows — OpenProcess 判定

  cli/                     … module:cli — コマンド定義・シグナル処理
    connect.go
    disconnect.go
    ls.go
    get.go
    put.go
    rm.go
    push.go
    pull.go
    flags.go               … 共通フラグ定義
    exit.go                … 終了コード定数・エラーマッピング
    signal.go              … SIGINT/SIGTERM ハンドラ
```

### 2.3 終了コード体系（リリースブロッキング RB-1）

`cli/exit.go` に定数として定義し、全コマンドハンドラはこの定数のみを使用してプロセスを終了する。エラー種別判定は `exit.go` のマッピング関数で一元化し、コマンドハンドラ内での直接的な数値指定を禁止する。

| コード | 意味 | 発生条件 |
|---|---|---|
| 0 | 成功 | すべての操作が完了 |
| 1 | 部分的失敗 | 一部ファイルの転送・同期が失敗し、一部が成功。**コード 0 で返すことはリリース不可** |
| 2 | 認証失敗 | トークン無効、refresh 失敗 + `--no-interactive`、RFC 7591 非対応 + `--no-interactive` |
| 3 | ネットワークエラー | DNS 解決失敗、接続タイムアウト（30 秒超過）、TLS エラー |
| 4 | その他エラー | ロックファイル競合、`sync_state.json` 破損、プロトコルバージョン不一致、パーミッション修正失敗 |

部分的失敗（コード 1）の扱い: リトライ上限（3 回）に達して失敗したファイルがある場合も処理を続行し、全操作完了後にエラー一覧を stderr にまとめて表示する。その後コード 1 で終了する。成功ファイルが 0 件であっても、少なくとも 1 ファイルの処理を試みた後に一部のみ失敗した場合はコード 1 とする。全件失敗の場合もコード 1 を返す（コード 0 へのフォールバックは不可）。

### 2.4 フラグ仕様（リリースブロッキング RB-2）

#### 共通フラグ（全コマンドで有効）

| フラグ | 型 | 説明 |
|---|---|---|
| `--verbose` | bool | 詳細ログを stderr に出力 |
| `--no-interactive` | bool | インタラクティブプロンプトを無効化。認証不能時はコード 2 で終了 |
| `--insecure` | bool | TLS 証明書検証をスキップ（開発・テスト環境のみ。HTTP 通信は引き続き拒否） |

#### `--dry-run` の適用範囲（RB-2 厳守）

`--dry-run` は `push` / `pull` / `rm` の 3 コマンドにのみ有効なフラグとして定義する。

| コマンド | `--dry-run` 動作 |
|---|---|
| `push` | 実際の転送・削除を行わず、実行予定の操作一覧を stdout に出力 |
| `pull` | 実際の転送・削除を行わず、実行予定の操作一覧を stdout に出力 |
| `rm` | 実際の削除を行わず、削除対象ファイル一覧を stdout に出力 |
| `connect` | `--dry-run` フラグを受け付けるが**無視する**（エラーにしない） |
| `disconnect` | `--dry-run` フラグを受け付けるが**無視する**（エラーにしない） |
| `ls` | `--dry-run` フラグを受け付けるが**無視する**（エラーにしない） |
| `get` | `--dry-run` フラグを受け付けるが**無視する**（エラーにしない） |
| `put` | `--dry-run` フラグを受け付けるが**無視する**（エラーにしない） |

実装上、`--dry-run` は `flags.go` でグローバルフラグとして受け付けるが、有効に機能するのは `push.go` / `pull.go` / `rm.go` 内のみとする。`connect` / `disconnect` / `ls` / `get` / `put` のハンドラはフラグ値を参照しない。

#### `push` / `pull` 固有フラグ

| フラグ | 型 | デフォルト | 説明 |
|---|---|---|---|
| `--no-delete` | bool | false | 削除伝播を無効化 |
| `--force` | bool | false | コンフリクト時に強制上書き |
| `--exclude` | string（複数指定可） | — | 除外パターン（`.rsignore` との併用可） |
| `--parallel` | int | 3 | 並列ワーカー数 |
| `--reset-state` | bool | false | `sync_state.json` を削除して初回 sync 状態にリセット |
| `--force-unlock` | bool | false | ロックファイルを強制削除 |

### 2.5 認証フロー（module:auth）

`rscli connect user@host` の実行フローを以下に示す。

```
1. WebFinger 発見
   GET https://{host}/.well-known/webfinger
       ?resource=acct:{user}@{host}
       &rel=http://tools.ietf.org/id/draft-dejong-remotestorage
   → JRD からストレージ URL (href) と OAuth2 認証エンドポイント (properties) を抽出
   → 失敗時: ストレージ URL 直接入力のインタラクティブプロンプトを表示
             （--no-interactive 時はコード 4 でエラー終了）

2. RFC 7591 動的クライアント登録（優先）
   POST {registration_endpoint}
   Body: { client_name: "remotestorage-cli",
           redirect_uris: ["http://localhost/callback"],
           token_endpoint_auth_method: "none" }
   → 成功時: 返却された client_id を使用
   → 失敗時: インタラクティブプロンプトで client_id 手動入力
             （--no-interactive 時はコード 2 でエラー終了）

3. PKCE 生成（S256 固定・フォールバックなし）
   code_verifier: 暗号学的ランダム文字列
   code_challenge: Base64URL(SHA-256(code_verifier))
   code_challenge_method: S256

4. OAuth2 Authorization Code フロー
   - localhost 一時 HTTP サーバーをポート 0（OS 動的割当）で起動
   - ブラウザを認可エンドポイントへリダイレクト
     パラメータ: response_type=code, client_id,
                redirect_uri=http://localhost:{port}/callback,
                scope=*:rw（--scope で変更可能）,
                code_challenge, code_challenge_method=S256, state
   - コールバックで認可コードを受け取り、トークンエンドポイントで交換
   - AccessToken / RefreshToken を OS 別セキュアストレージに保存

5. 既存トークンの上書き
   - 既存トークンが存在する場合: 確認プロンプト表示後に上書き
   - --yes: 確認プロンプトをスキップ
```

**トークンストア（SD-2）**: `TokenStore` インターフェース（`Save` / `Load` / `Delete`）をビルドタグで OS 別実装に切り替える。

| OS | ビルドタグ | 保護方式 | トークン本体の格納先 |
|---|---|---|---|
| Linux / WSL | `//go:build linux` | ファイルパーミッション 0600 | `~/.config/remotestorage-cli/token.json` |
| macOS | `//go:build darwin` | macOS Keychain | Keychain 項目（`token.json` は生成しない） |
| Windows | `//go:build windows` | Credential Manager | Generic Credential（`token.json` は生成しない） |

Linux / WSL での 0600 強制: `os.OpenFile` でパーミッション `0600` を明示指定して作成する。既存ファイルのパーミッションが `0600` より緩い場合、書き込み前に `os.Chmod` で修正する。修正失敗時はコード 4 でエラー終了。

**トークンライフサイクル**: API リクエスト時にトークン期限切れを検出した場合、refresh token が存在する場合は自動更新を試みる。refresh 成功時は新 AccessToken / RefreshToken を保存して処理を継続する。refresh 失敗時は `--no-interactive` 環境ではコード 2 で終了し、インタラクティブ環境ではブラウザ再認証フローを自動起動する。

### 2.6 プログレス表示（リリースブロッキング RB-3）

`transfer/progress.go` に実装する。

- 出力先: **stderr のみ**。stdout への出力は一切行わない（stdout 汚染によるパイプ連携の破壊を防止）。
- TTY 判定: `golang.org/x/term.IsTerminal(int(os.Stderr.Fd()))` で判定する。
- 非 TTY 時（パイプ、リダイレクト、CI 環境等）: プログレス表示を**自動抑制**する。警告・エラーメッセージは非 TTY 時も stderr に出力する。
- 表示形式: `N/M files`（例: `3/10 files`）。
- 抑制条件の上書き: `--no-interactive` フラグが指定された場合も非 TTY と同様に抑制する。

```go
// transfer/progress.go での判定例
func shouldShowProgress() bool {
    return term.IsTerminal(int(os.Stderr.Fd())) && !noInteractive
}
```

`--verbose` フラグ指定時は、転送中のファイルパスを1行ずつ stderr に出力する（プログレス表示とは別に、TTY 判定によらず出力する）。

### 2.7 ファイルスキャン（リリースブロッキング RB-4）

`push` / `put` の実行前にローカルファイルシステムをスキャンする際、シンボリックリンクおよび特殊ファイル（デバイスファイル、名前付きパイプ、ソケット等）を検出した場合は以下の処理を行う。

- **スキップ**: 対象ファイルの転送処理を行わない。
- **warning 出力**: `WARNING: skipping symlink: {path}` または `WARNING: skipping special file: {path}` を **stderr** に出力する。
- **継続**: warning 出力後も処理を続行し、残りのファイルの転送を実行する。
- **終了コード**: warning 対象ファイルのみがスキップされ、他のファイルがすべて成功した場合はコード 0 を返す。warning ファイルに加えて他のファイルも失敗した場合はコード 1 を返す。

シンボリックリンクへの追従（`os.Stat` による解決）は行わない。`os.Lstat` を使用してリンク自体のメタデータを取得し、`os.ModeSymlink` を検出した時点でスキップする。これはシンボリックリンクチェーンを利用したパストラバーサル攻撃を防止するためのセキュリティ要件である。

### 2.8 通信レイヤー（module:remotestorage）

**HTTPS 強制（SD-1）**: `module:remotestorage` の HTTP クライアントは全リクエストの URL スキームが `https://` であることを送信前に検証し、`http://` の場合はリクエストを拒否してエラーを返す。`--insecure` フラグは TLS 証明書検証スキップのみを制御し、HTTP 通信を許可しない。`--insecure` の利用は開発・テスト環境に限定する。

**タイムアウト**:

| 種別 | 値 | 超過時の終了コード |
|---|---|---|
| 接続タイムアウト | 30 秒 | 3（ネットワークエラー） |
| アイドルタイムアウト | 60 秒 | リトライ対象外、コード 3 |

**リトライ**: 対象ステータスは 429 / 500 / 502 / 503 のみ。最大リトライ回数 3 回。バックオフは exponential backoff（初回待機 1 秒・最大 30 秒・フルジッター）。429 受信時は `Retry-After` ヘッダ値と算出 backoff 値の大きい方を採用する。並列転送中にいずれかのワーカーが 429 を受信した場合は全ワーカーを一時停止し、`Retry-After` 値を全ワーカーに適用して一斉再開する。

**パスエンコーディング**: リモートパスの URL エンコーディングは CLI 側で自動処理する。ローカルパスは `filepath` パッケージで OS 標準区切り文字を扱い、remoteStorage との通信パスは常に `/` を使用する。

### 2.9 同期エンジン（module:sync）

#### 変更検出

| 対象 | 検出方式 | フォールバック |
|---|---|---|
| リモートファイル | ETag（優先） | `Last-Modified` ヘッダ（ETag 非対応サーバー） |
| ローカルファイル | mtime | — |

#### sync_state.json（SD-3）

`{設定ディレクトリ}/sync_state.json` にファイル毎の ETag / mtime を記録する。書き込みは一時ファイル書き込み → `os.Rename` の 2 段階で atomic write を保証する。`sync_state.json` のパースに失敗した場合はエラーメッセージを stderr に出力してコード 4 で終了する（自動修復は行わない）。`--reset-state` フラグで `sync_state.json` を削除し、初回 sync 扱いにリセットする。

`cli/signal.go` の SIGINT / SIGTERM ハンドラは受信時に、転送中のファイルの完了を待たず、完了済みファイルのみを `sync_state.json` に atomic write で記録し、次回実行で中断箇所から再開可能な状態を保証する。

#### 同期ルール

| シナリオ | `push` | `pull` |
|---|---|---|
| 初回 sync（`sync_state.json` 未存在） | ローカルのみ→アップロード、リモートのみ→スキップ | リモートのみ→ダウンロード、ローカルのみ→スキップ |
| 削除伝播 | ローカル削除→リモート削除 | リモート削除→ローカル削除 |
| `--no-delete` | 削除伝播無効 | 削除伝播無効 |
| コンフリクト（双方変更） | デフォルト: スキップ + 通知 | デフォルト: スキップ + 通知 |
| `--force` | 強制上書き | 強制上書き |

**除外パターン**: `.rsignore`（sync ルートに配置、`.gitignore` 形式）と `--exclude` オプションを併用可能。両方が指定された場合はすべてのパターンを適用する。空ディレクトリは remoteStorage プロトコル仕様に従いスキップする。

### 2.10 排他制御（module:config、SD-4）

`push` / `pull` 実行時に `{設定ディレクトリ}/lock` ファイルを `O_CREATE | O_EXCL` で排他的に作成し、実行プロセスの PID を記録する。

| 動作 | 詳細 |
|---|---|
| stale 検出 | 起動時に `lock` ファイルが存在する場合、記録された PID のプロセス存在を確認。Unix 系（`//go:build !windows`）: シグナル 0 送信。Windows（`//go:build windows`）: `OpenProcess` API。プロセス不在時は stale として自動削除してロックを取得 |
| 競合時 | ロックが有効な状態（PID のプロセスが存在）で `push` / `pull` を実行 → コード 4 でエラー終了 |
| 手動解除 | `--force-unlock` フラグでロックファイルを強制削除 |

### 2.11 設定ディレクトリ（module:config）

`os.UserConfigDir()` を使用し、戻り値配下に `remotestorage-cli/` サブディレクトリを作成する。

| OS | 設定パス |
|---|---|
| Linux | `$XDG_CONFIG_HOME/remotestorage-cli/`（未設定時 `~/.config/remotestorage-cli/`） |
| WSL | `~/.config/remotestorage-cli/`（Linux 扱い） |
| macOS | `~/Library/Application Support/remotestorage-cli/` |
| Windows | `%AppData%\remotestorage-cli\` |

| ファイル | 用途 |
|---|---|
| `token.json` | OAuth トークン（Linux / WSL のみ。macOS / Windows では作成しない） |
| `sync_state.json` | 同期状態（ファイル毎の ETag / mtime） |
| `lock` | ロックファイル（PID 記録） |

### 2.12 クロスプラットフォーム配布

GitHub Actions で Go のクロスコンパイル（`GOOS` / `GOARCH` 環境変数指定）を実行し、タグプッシュ時に GitHub Releases へ自動配布する。全バイナリのビルド成功がリリース前提条件であり、いずれか 1 つのビルドが失敗した場合はリリースをブロックする。

| OS | アーキテクチャ | バイナリ名 |
|---|---|---|
| Linux | amd64 | `rscli-linux-amd64` |
| Linux | arm64 | `rscli-linux-arm64` |
| macOS | amd64 | `rscli-darwin-amd64` |
| macOS | arm64 | `rscli-darwin-arm64` |
| Windows | amd64 | `rscli-windows-amd64.exe` |

`go install github.com/<org>/remotestorage-cli/cmd/rscli@latest` によるソースインストールも提供する。

### 2.13 リリースブロッキング制約の設計レベル保証まとめ

| 制約 | 保証メカニズム |
|---|---|
| 終了コード体系（RB-1） | `cli/exit.go` に定数定義。全コマンドハンドラはマッピング関数を経由して終了コードを決定し、直接的な数値指定を禁止する |
| `--dry-run` 適用範囲（RB-2） | `push.go` / `pull.go` / `rm.go` のハンドラのみがフラグ値を参照する。`connect` / `disconnect` / `ls` / `get` / `put` のハンドラはフラグ値を参照しない設計とし、無視を明示する |
| プログレス表示（RB-3） | `transfer/progress.go` が `golang.org/x/term.IsTerminal()` で TTY 判定を行い、非 TTY 時は自動抑制。プログレス出力先は `os.Stderr` に固定し、`os.Stdout` へは出力しない |
| シンボリックリンク・特殊ファイル（RB-4） | `os.Lstat` を使用してリンク解決を行わずメタデータを取得。`os.ModeSymlink` または特殊ファイルモードを検出した時点でスキップし、warning を stderr に出力する |
| HTTPS 必須（SD-1） | `remotestorage/client.go` が全リクエストの URL スキームを送信前に検証し、`http://` を拒否。`--insecure` は TLS 証明書検証スキップのみを制御 |
| トークン保護（SD-2） | `TokenStore` インターフェースとビルドタグによる OS 別実装強制。Linux: 0600 強制。macOS: Keychain API。Windows: Credential Manager API |
| atomic write（SD-3） | `sync_state.json` は一時ファイル書き込み → `os.Rename` で更新。シグナルハンドラが SIGINT/SIGTERM 受信時に完了済みファイルのみ記録 |
| ロックファイル（SD-4） | `O_CREATE \| O_EXCL` による排他的ファイル作成。PID 記録と OS 別プロセス存在確認による stale 検出 |

---

## 3. Open Questions

| # | 問い | 背景 | 判断時期 |
|---|---|---|---|
| OQ-1 | `--dry-run` を `connect` / `disconnect` / `ls` / `get` / `put` で受け付けて無視する仕様を維持するか、それともフラグ自体を非対応コマンドに定義しないか | 現設計は「受け付けて無視」とすることでスクリプト移植性を高める方針だが、「未定義コマンドへの未知フラグ」としてエラーにした方が誤用を早期に検出できる | コマンドライン UX のユーザーフィードバック収集後 |
| OQ-2 | `--parallel` の上限値を設定するか | 無制限に大きな値を指定するとサーバー側で 429 が多発する可能性がある。`--parallel 100` のような極端な値に対するガードが必要か | 主要 remoteStorage サーバーでの負荷テスト完了後 |
| OQ-3 | warning 扱いのシンボリックリンクが存在した場合の終了コードを 0 とすることの妥当性 | 現設計ではスキップされたシンボリックリンクがある場合も、他のファイルがすべて成功すればコード 0 を返す。コード 1 を返す方が「完全成功ではない」ことをスクリプトに伝えられるが、日常的なワークフローで warning を失敗扱いにすると CI が誤動作する可能性がある | ユーザーフィードバックに基づき判断 |
| OQ-4 | 非 TTY 時のプログレス抑制を `--progress` フラグで強制有効化できるようにするか | CI ログに転送状況を記録したいユーザーがいる可能性がある。ただし stderr への出力であるためパイプ連携は破壊しない | ユーザーフィードバックに基づき判断 |
| OQ-5 | draft-22 以降のバージョンが公開された場合のバージョンネゴシエーション設計 | 現在は draft-22 固定。主要サーバー実装（php-remote-storage、armadietto）が新バージョンをリリースした場合の対応が必要 | 主要サーバー実装が新バージョンをリリースした時点 |
| OQ-6 | 複数アカウント（複数サーバー）対応の設計 | 現スコープは 1 アカウントのみ。トークン保存構造を `user@host` キー付き構造またはアカウント別ファイルに分離する場合、`token.json` のスキーマとロックファイルのスコープが変わる | 複数アカウント需要がユーザーフィードバックで確認された時点 |
| OQ-7 | Linux デスクトップ向けキーリング統合（GNOME Keyring / KDE Wallet）の導入 | 現在は全 Linux 環境でファイルパーミッション 0600 方式を採用。デスクトップ環境では D-Bus 経由のキーリング統合が UX を向上させる可能性があるが、ヘッドレス環境では 0600 フォールバックを維持する必要がある | デスクトップ Linux ユーザーからのフィードバックに基づき判断 |
| OQ-8 | Windows arm64 ビルドの追加 | 初期リリースのビルドマトリクスに Windows arm64 は含まれていない。Snapdragon 搭載 PC の普及状況に応じて追加する | 需要調査結果に基づく |
