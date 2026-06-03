---
codd:
  node_id: test:acceptance-criteria
  type: test
  depends_on:
  - id: req:remotestorage-cli-requirements
    relation: derives_from
    semantic: governance
  depended_by:
  - id: design:system-design
    relation: constrained_by
    semantic: governance
  - id: test:test-strategy
    relation: depends_on
    semantic: governance
  conventions:
  - targets:
    - cli_command:rscli
    reason: 終了コード体系(0:成功/1:部分的失敗/2:認証失敗/3:ネットワークエラー/4:その他)は受入判定基準であり違反時リリース不可。
  - targets:
    - module:auth
    - module:config
    reason: トークン保護(Linux/WSL:0600、macOS:Keychain、Windows:Credential Manager)は受入基準に含まれる必須セキュリティ要件。
  - targets:
    - module:transfer
    reason: リトライ対象(429/500/502/503)とexponential backoff(初回1秒/最大30秒/フルジッター)は受入基準として検証必須。
  modules:
  - auth
  - sync
  - transfer
  - cli
  - config
  - remotestorage
---

# 受入基準

## 1. Overview

本ドキュメントは `rscli`（remotestorage-cli）の受入基準を定義する。`rscli` は remoteStorage プロトコル（draft-dejong-remotestorage-22）を操作する Go 製 CLI ツールであり、認証・ファイル操作・同期・リトライ・セキュリティの各機能を備える。

受入判定は以下の3つのリリースブロッキング制約を含む:

1. **終了コード体系**: `rscli` は終了コード 0（成功）/ 1（部分的失敗）/ 2（認証失敗）/ 3（ネットワークエラー）/ 4（その他エラー）を正確に返すこと。違反時リリース不可。
2. **トークン保護**: Linux/WSL 環境では `token.json` のパーミッション 0600 を強制、macOS では Keychain、Windows では Credential Manager を使用すること。違反時リリース不可。
3. **リトライ仕様**: HTTP 429/500/502/503 を対象に exponential backoff（初回 1 秒・最大 30 秒・フルジッター）でリトライすること。429 は `Retry-After` ヘッダを尊重すること。違反時リリース不可。

---

## 2. Acceptance Criteria

### 2.1 設計-テスト トレーサビリティマトリクス

以下に要件定義から抽出した全検証可能動作と、対応するテストシナリオIDを列挙する。

| # | 検証可能動作 | テストシナリオID |
|---|---|---|
| VB-01 | WebFinger による remoteStorage サーバー自動検出 | AC-AUTH-01 |
| VB-02 | WebFinger 失敗時のストレージURL直接入力フォールバック | AC-AUTH-02 |
| VB-03 | OAuth2 Authorization Code + localhost リダイレクトによる認証フロー | AC-AUTH-03 |
| VB-04 | PKCE（RFC 7636）の使用 | AC-AUTH-04 |
| VB-05 | localhost リダイレクトポートの動的選択 | AC-AUTH-05 |
| VB-06 | RFC 7591 動的クライアント登録 | AC-AUTH-06 |
| VB-07 | RFC 7591 非対応時のインタラクティブ client_id 入力フォールバック | AC-AUTH-07 |
| VB-08 | デフォルトスコープ `*:rw` の適用 | AC-AUTH-08 |
| VB-09 | スコープのオプション変更 | AC-AUTH-09 |
| VB-10 | トークンの `token.json` 保存（Linux/WSL） | AC-SEC-01 |
| VB-11 | `token.json` パーミッション 0600 強制（Linux/WSL） | AC-SEC-02 |
| VB-12 | macOS Keychain によるトークン保護 | AC-SEC-03 |
| VB-13 | Windows Credential Manager によるトークン保護 | AC-SEC-04 |
| VB-14 | トークン期限切れ時の自動ブラウザ再認証 | AC-AUTH-10 |
| VB-15 | refresh token 利用可能時の自動更新 | AC-AUTH-11 |
| VB-16 | refresh token 失敗時のブラウザ再認証移行 | AC-AUTH-12 |
| VB-17 | `--no-interactive` 時の refresh token 失敗でエラー終了（終了コード2） | AC-AUTH-13 |
| VB-18 | `connect` 再実行時の確認プロンプトと上書き | AC-AUTH-14 |
| VB-19 | `connect --yes` による確認スキップ | AC-AUTH-15 |
| VB-20 | `disconnect` によるトークン削除 | AC-AUTH-16 |
| VB-21 | HTTPS 必須 | AC-SEC-05 |
| VB-22 | `--insecure` フラグで自己署名証明書許可 | AC-SEC-06 |
| VB-23 | `ls` でファイル名・サイズ・更新日時を表示 | AC-LS-01 |
| VB-24 | `ls --json` で JSON 形式出力 | AC-LS-02 |
| VB-25 | `ls -R` で再帰的表示 | AC-LS-03 |
| VB-26 | `get /remote ./local` でファイルダウンロード | AC-GET-01 |
| VB-27 | `get` はファイル単体のみ対応 | AC-GET-02 |
| VB-28 | `put ./local /remote` でファイルアップロード | AC-PUT-01 |
| VB-29 | `put` はファイル単体のみ対応 | AC-PUT-02 |
| VB-30 | Content-Type 自動検出（拡張子ベース） | AC-PUT-03 |
| VB-31 | 不明拡張子時の `application/octet-stream` フォールバック | AC-PUT-04 |
| VB-32 | `--content-type` による明示指定 | AC-PUT-05 |
| VB-33 | ストリーミング転送（大ファイル対応） | AC-TRANSFER-01 |
| VB-34 | 並列転送デフォルト3、`--parallel` で変更可能 | AC-TRANSFER-02 |
| VB-35 | プログレス表示（stderr、N/M files 形式） | AC-TRANSFER-03 |
| VB-36 | 非 TTY 時のプログレス自動抑制 | AC-TRANSFER-04 |
| VB-37 | `rm /path` でファイル削除 | AC-RM-01 |
| VB-38 | `rm -r /path/` で再帰削除（確認プロンプトあり） | AC-RM-02 |
| VB-39 | `push ./local /remote` ローカル→リモート同期 | AC-SYNC-01 |
| VB-40 | `pull /remote ./local` リモート→ローカル同期 | AC-SYNC-02 |
| VB-41 | ETag 優先の変更検出 | AC-SYNC-03 |
| VB-42 | ETag 非対応時の Last-Modified フォールバック | AC-SYNC-04 |
| VB-43 | ローカルファイル変更検出に mtime 使用 | AC-SYNC-05 |
| VB-44 | `sync_state.json` のファイル毎更新 | AC-SYNC-06 |
| VB-45 | 初回 push 時: ローカルのみ→アップロード、リモートのみ→スキップ | AC-SYNC-07 |
| VB-46 | 初回 pull 時: リモートのみ→ダウンロード、ローカルのみ→スキップ | AC-SYNC-08 |
| VB-47 | push 時の削除伝播（ローカル削除→リモート削除） | AC-SYNC-09 |
| VB-48 | pull 時の削除伝播（リモート削除→ローカル削除） | AC-SYNC-10 |
| VB-49 | `--no-delete` で削除伝播無効化 | AC-SYNC-11 |
| VB-50 | コンフリクト時デフォルトスキップ+通知 | AC-SYNC-12 |
| VB-51 | `--force` でコンフリクト強制上書き | AC-SYNC-13 |
| VB-52 | `.rsignore` ファイルによる除外パターン | AC-SYNC-14 |
| VB-53 | `--exclude` オプションによる除外 | AC-SYNC-15 |
| VB-54 | 空ディレクトリのスキップ | AC-SYNC-16 |
| VB-55 | シンボリックリンク・特殊ファイルのスキップ+warning | AC-SYNC-17 |
| VB-56 | HTTP 429/500/502/503 に対する3回リトライ | AC-RETRY-01 |
| VB-57 | exponential backoff（初回1秒・最大30秒・フルジッター） | AC-RETRY-02 |
| VB-58 | 429 の Retry-After ヘッダ尊重 | AC-RETRY-03 |
| VB-59 | 並列転送時の 429 全ワーカー一時停止 | AC-RETRY-04 |
| VB-60 | 失敗しても続行し最後にエラー一覧表示 | AC-RETRY-05 |
| VB-61 | 終了コード 0: 成功 | AC-EXIT-01 |
| VB-62 | 終了コード 1: 部分的失敗 | AC-EXIT-02 |
| VB-63 | 終了コード 2: 認証失敗 | AC-EXIT-03 |
| VB-64 | 終了コード 3: ネットワークエラー | AC-EXIT-04 |
| VB-65 | 終了コード 4: その他エラー | AC-EXIT-05 |
| VB-66 | `sync_state.json` の atomic write | AC-SIGNAL-01 |
| VB-67 | 中断時に完了済みファイルのみ記録、再実行で再開可能 | AC-SIGNAL-02 |
| VB-68 | ロックファイルによる同時実行制御 | AC-LOCK-01 |
| VB-69 | stale ロックファイルの自動削除（PID 不存在時） | AC-LOCK-02 |
| VB-70 | `--force-unlock` による手動解除 | AC-LOCK-03 |
| VB-71 | 同時実行時のエラー終了（終了コード4） | AC-LOCK-04 |
| VB-72 | `sync_state.json` 破損時のエラー終了+通知 | AC-STATE-01 |
| VB-73 | `--reset-state` による初回 sync リセット | AC-STATE-02 |
| VB-74 | `--dry-run` が push/pull/rm にのみ適用 | AC-DRY-01 |
| VB-75 | `--dry-run` が connect/disconnect/ls/get/put で無視 | AC-DRY-02 |
| VB-76 | `--verbose` フラグで詳細出力 | AC-CLI-01 |
| VB-77 | `--no-interactive` で全プロンプトスキップ | AC-CLI-02 |
| VB-78 | 設定ディレクトリは `os.UserConfigDir()` 準拠 | AC-PLATFORM-01 |
| VB-79 | 接続タイムアウト 30 秒 | AC-TIMEOUT-01 |
| VB-80 | アイドルタイムアウト 60 秒 | AC-TIMEOUT-02 |
| VB-81 | パスの URL エンコーディング自動処理 | AC-PATH-01 |
| VB-82 | 日本語・特殊文字の透過的な扱い | AC-PATH-02 |

### 2.2 認証（Auth）

**AC-AUTH-01**: `rscli connect user@host` 実行時、WebFinger（`https://{host}/.well-known/webfinger?resource=acct:{user}@{host}`）を用いて remoteStorage サーバーの認証エンドポイントおよびストレージ URL を自動検出する。

**AC-AUTH-02**: WebFinger が失敗した場合（DNS 解決失敗、HTTP エラー、不正レスポンス）、ユーザーにストレージ URL の直接入力を求めるフォールバックプロンプトを表示する。

**AC-AUTH-03**: OAuth2 Authorization Code フローを使用し、localhost へのリダイレクトで認可コードを受け取る。ブラウザが自動的に開く。

**AC-AUTH-04**: 全認証フローで PKCE（RFC 7636）の `code_verifier` / `code_challenge` を使用する。

**AC-AUTH-05**: localhost リダイレクトのポートは動的に空きポートを選択し、ポート競合を起こさない。

**AC-AUTH-06**: RFC 7591 動的クライアント登録を優先的に試行する。

**AC-AUTH-07**: RFC 7591 非対応サーバーの場合、インタラクティブプロンプトで `client_id` の入力を求める。`client_secret` は不要。

**AC-AUTH-08**: デフォルトスコープは `*:rw` で動作する。

**AC-AUTH-09**: `--scope` オプションでスコープを変更可能。

**AC-AUTH-10**: 保存済みトークンが期限切れの場合、自動でブラウザを開いて再認証フローを開始する。

**AC-AUTH-11**: refresh token が利用可能な場合、ブラウザ再認証なしにトークンを自動更新する。

**AC-AUTH-12**: refresh token による更新が失敗した場合、ブラウザ再認証フローに自動移行する。

**AC-AUTH-13**: `--no-interactive` フラグ指定時に refresh token 失敗した場合、ブラウザを開かずに終了コード 2 で終了する。

**AC-AUTH-14**: `rscli connect` 再実行時に既存トークンがある場合、上書き前に確認プロンプトを表示する。確認後に上書きを実行する。

**AC-AUTH-15**: `rscli connect --yes` で確認プロンプトをスキップし、即座に上書きする。

**AC-AUTH-16**: `rscli disconnect` で保存済みトークンを削除する（Linux/WSL: ファイル削除、macOS: Keychain エントリ削除、Windows: Credential Manager エントリ削除）。

### 2.3 セキュリティ（Security） — リリースブロッキング

**AC-SEC-01**: Linux/WSL 環境で `token.json` を `~/.config/remotestorage-cli/token.json` に保存する。

**AC-SEC-02** 🔒: Linux/WSL 環境で `token.json` のファイルパーミッションを 0600 に強制する。既存ファイルのパーミッションが 0600 でない場合、書き込み前に修正する。**リリースブロッキング制約**。

**AC-SEC-03** 🔒: macOS 環境でトークンを macOS Keychain に保存する。`token.json` ファイルは使用しない。**リリースブロッキング制約**。

**AC-SEC-04** 🔒: Windows 環境で AccessToken および RefreshToken を Windows Credential Manager に保存する。`token.json` ファイルは使用しない。WSL 環境は Linux 扱い（ファイルパーミッション 0600）。**リリースブロッキング制約**。

**AC-SEC-05**: 全 HTTP 通信は HTTPS を使用する。HTTP 接続は拒否する。

**AC-SEC-06**: `--insecure` フラグ指定時に限り、自己署名証明書を許可する。

### 2.4 ファイル一覧表示（ls）

**AC-LS-01**: `rscli ls /path` でファイル名・サイズ・更新日時を表示する。

**AC-LS-02**: `rscli ls --json /path` で JSON 形式の構造化出力を返す。

**AC-LS-03**: `rscli ls -R /path` でサブディレクトリを含む再帰的なファイル一覧を表示する。

### 2.5 ファイルダウンロード（get）

**AC-GET-01**: `rscli get /remote/path ./local/path` でリモートファイルをローカルにダウンロードする。

**AC-GET-02**: `get` コマンドはファイル単体のみ対応する。ディレクトリパスが指定された場合はエラーメッセージを表示する。

### 2.6 ファイルアップロード（put）

**AC-PUT-01**: `rscli put ./local/file /remote/path` でローカルファイルをリモートにアップロードする。

**AC-PUT-02**: `put` コマンドはファイル単体のみ対応する。ディレクトリパスが指定された場合はエラーメッセージを表示し、`push` の使用を案内する。

**AC-PUT-03**: アップロード時の Content-Type は `mime.TypeByExtension` による拡張子ベースの自動検出を優先する。

**AC-PUT-04**: 拡張子から Content-Type を判定できない場合、`application/octet-stream` をフォールバックとして使用する。

**AC-PUT-05**: `--content-type` オプションで Content-Type を明示指定した場合、自動検出結果を上書きする。

### 2.7 ファイル転送共通（Transfer）

**AC-TRANSFER-01**: ファイル転送はストリーミング方式を使用し、ファイルサイズ上限を設けない。ファイル全体をメモリに読み込まない。

**AC-TRANSFER-02**: 並列転送数はデフォルト 3 で動作し、`--parallel N` オプションで変更可能。

**AC-TRANSFER-03**: 転送中のプログレスを stderr に `N/M files` 形式で表示する。

**AC-TRANSFER-04**: 標準エラー出力が TTY でない場合、プログレス表示を自動的に抑制する。

### 2.8 ファイル削除（rm）

**AC-RM-01**: `rscli rm /path` で指定ファイルを削除する。

**AC-RM-02**: `rscli rm -r /path/` で指定ディレクトリを再帰的に削除する。実行前に確認プロンプトを表示する。

### 2.9 同期（Sync）

**AC-SYNC-01**: `rscli push ./local /remote` でローカルディレクトリの内容をリモートに同期する。

**AC-SYNC-02**: `rscli pull /remote ./local` でリモートディレクトリの内容をローカルに同期する。

**AC-SYNC-03**: リモートファイルの変更検出は ETag を優先する。

**AC-SYNC-04**: ETag 非対応サーバーの場合、`Last-Modified` ヘッダにフォールバックする。

**AC-SYNC-05**: ローカルファイルの変更検出は mtime を使用する。

**AC-SYNC-06**: `sync_state.json` はファイル毎に更新する。中断後の再実行で完了済みファイルをスキップして再開可能。

**AC-SYNC-07**: 初回 push 時（`sync_state.json` 未存在）: ローカルのみ存在するファイルはアップロード、リモートのみ存在するファイルはスキップ（削除しない）。

**AC-SYNC-08**: 初回 pull 時（`sync_state.json` 未存在）: リモートのみ存在するファイルはダウンロード、ローカルのみ存在するファイルはスキップ（削除しない）。

**AC-SYNC-09**: push 時にローカルで削除されたファイル（前回 sync 時に存在）はリモートからも削除する。

**AC-SYNC-10**: pull 時にリモートで削除されたファイル（前回 sync 時に存在）はローカルからも削除する。

**AC-SYNC-11**: `--no-delete` オプション指定時、削除伝播を無効化する（push/pull 共通）。

**AC-SYNC-12**: コンフリクト検出時（ローカル・リモート双方で変更あり）、デフォルトでスキップしてユーザーに通知する。

**AC-SYNC-13**: `--force` オプション指定時、コンフリクトを検出しても強制上書きする。

**AC-SYNC-14**: sync ルートの `.rsignore` ファイルで定義されたパターンに一致するファイル・ディレクトリを除外する（`.gitignore` 形式）。

**AC-SYNC-15**: `--exclude` オプションでコマンドラインから除外パターンを指定可能。`.rsignore` との併用時は両方を適用する。

**AC-SYNC-16**: 空ディレクトリは remoteStorage プロトコル仕様に従いスキップする。

**AC-SYNC-17**: シンボリックリンクおよび特殊ファイル（デバイスファイル、FIFO 等）はスキップし、warning を stderr に表示する。

### 2.10 リトライ（Retry） — リリースブロッキング

**AC-RETRY-01** 🔒: HTTP ステータス 429・500・502・503 を受信した場合、最大 3 回リトライする。**リリースブロッキング制約**。

**AC-RETRY-02** 🔒: リトライ間隔は exponential backoff 方式で、初回待機 1 秒・最大待機 30 秒・フルジッターを適用する。**リリースブロッキング制約**。

**AC-RETRY-03** 🔒: HTTP 429 レスポンスに `Retry-After` ヘッダが含まれる場合、その値を待機時間として尊重する。**リリースブロッキング制約**。

**AC-RETRY-04**: 並列転送中に 429 を受信した場合、全ワーカーを一時停止し、`Retry-After` ヘッダの値を全ワーカーに適用して再開する。

**AC-RETRY-05**: リトライ上限に達して失敗したファイルがあっても処理を続行し、全処理完了後にエラー一覧をまとめて表示する。

### 2.11 終了コード（Exit Codes） — リリースブロッキング

**AC-EXIT-01** 🔒: 全操作が成功した場合、終了コード 0 を返す。**リリースブロッキング制約**。

**AC-EXIT-02** 🔒: 一部ファイルの転送・同期が失敗し一部が成功した場合、終了コード 1 を返す。**リリースブロッキング制約**。

**AC-EXIT-03** 🔒: 認証失敗（トークン無効、refresh 失敗+非インタラクティブ等）の場合、終了コード 2 を返す。**リリースブロッキング制約**。

**AC-EXIT-04** 🔒: ネットワークエラー（DNS 解決失敗、接続タイムアウト、TLS エラー等）の場合、終了コード 3 を返す。**リリースブロッキング制約**。

**AC-EXIT-05** 🔒: 上記以外のエラー（ロックファイル競合、`sync_state.json` 破損等）の場合、終了コード 4 を返す。**リリースブロッキング制約**。

### 2.12 シグナル処理・中断耐性（Signal Handling）

**AC-SIGNAL-01**: `sync_state.json` は atomic write（一時ファイル書き込み→rename）で保存する。書き込み中の中断でファイルが破損しない。

**AC-SIGNAL-02**: SIGINT/SIGTERM 受信時、現在転送中のファイルの完了を待たず、完了済みファイルのみ `sync_state.json` に記録する。再実行で中断箇所から再開可能。

### 2.13 ロックファイル（Lock File）

**AC-LOCK-01**: push/pull 実行時に `{設定ディレクトリ}/lock` ファイルを作成し、PID を記録する。

**AC-LOCK-02**: 起動時にロックファイルが存在し、記録された PID のプロセスが存在しない場合、stale ロックファイルとして自動削除する。

**AC-LOCK-03**: `--force-unlock` フラグでロックファイルを手動解除可能。

**AC-LOCK-04**: ロックファイルが有効な状態で新たに push/pull を実行した場合、終了コード 4 でエラー終了する。

### 2.14 sync_state.json 破損・リセット

**AC-STATE-01**: `sync_state.json` のパースに失敗した場合、エラーメッセージを表示して終了コード 4 で終了する。自動修復は行わない。

**AC-STATE-02**: `--reset-state` フラグで `sync_state.json` を削除し、初回 sync 扱いにリセットする。

### 2.15 dry-run

**AC-DRY-01**: `--dry-run` フラグは `push`・`pull`・`rm` コマンドに適用され、実際の変更を行わず実行予定の操作を表示する。

**AC-DRY-02**: `--dry-run` フラグは `connect`・`disconnect`・`ls`・`get`・`put` では無視される（エラーにはしない）。

### 2.16 CLI 共通オプション

**AC-CLI-01**: `--verbose` フラグで HTTP リクエスト/レスポンスの詳細情報を含む詳細出力を行う。

**AC-CLI-02**: `--no-interactive` フラグで全プロンプトをスキップする。認証はトークンが事前に存在する場合のみ動作する。

### 2.17 クロスプラットフォーム

**AC-PLATFORM-01**: 設定ディレクトリは `os.UserConfigDir()` の返すパスに従う（Linux: `~/.config/remotestorage-cli`、macOS: `~/Library/Application Support/remotestorage-cli`、Windows: `%AppData%\remotestorage-cli`）。

### 2.18 タイムアウト

**AC-TIMEOUT-01**: HTTP 接続タイムアウトは 30 秒。30 秒以内に接続が確立しない場合、ネットワークエラーとして扱う。

**AC-TIMEOUT-02**: HTTP アイドルタイムアウトは 60 秒。60 秒間データ送受信がない場合、接続を切断する。

### 2.19 パスエンコーディング

**AC-PATH-01**: リモートパスの URL エンコーディングは CLI が自動的に処理する。ユーザーは生のパスを入力する。

**AC-PATH-02**: 日本語ファイル名・特殊文字を含むパスを透過的に扱う。`rscli put ./日本語ファイル.txt /documents/日本語ファイル.txt` が正常に動作する。

---

## 3. Failure Criteria

以下の条件に該当する場合、受入テストは不合格とする。

### 3.1 リリースブロッキング失敗条件

| ID | 失敗条件 | 対応制約 |
|---|---|---|
| FC-BLOCK-01 | 終了コードが仕様と異なる値を返す（例: 認証失敗時に終了コード 1 を返す） | 終了コード体系 |
| FC-BLOCK-02 | Linux/WSL で `token.json` のパーミッションが 0600 以外で保存される | トークン保護 |
| FC-BLOCK-03 | macOS で Keychain 以外の場所にトークンが保存される | トークン保護 |
| FC-BLOCK-04 | Windows で Credential Manager 以外の場所にトークンが保存される | トークン保護 |
| FC-BLOCK-05 | WSL 環境が Windows 扱いされる（Credential Manager を使用する） | トークン保護 |
| FC-BLOCK-06 | HTTP 429/500/502/503 以外のステータスコード（例: 400、404）でリトライが発生する | リトライ仕様 |
| FC-BLOCK-07 | リトライが exponential backoff なしに固定間隔で実行される | リトライ仕様 |
| FC-BLOCK-08 | リトライの初回待機が 1 秒から大幅に乖離する（ジッター考慮後） | リトライ仕様 |
| FC-BLOCK-09 | リトライの最大待機が 30 秒を超える（ジッター考慮後） | リトライ仕様 |
| FC-BLOCK-10 | 429 の `Retry-After` ヘッダが無視される | リトライ仕様 |

### 3.2 機能的失敗条件

| ID | 失敗条件 |
|---|---|
| FC-FUNC-01 | WebFinger の検出結果が正しいにもかかわらず接続に失敗する |
| FC-FUNC-02 | PKCE なしで認証フローが完了する |
| FC-FUNC-03 | 有効な refresh token が存在するのにブラウザ再認証が起動する |
| FC-FUNC-04 | `--no-interactive` 指定時にプロンプトが表示される |
| FC-FUNC-05 | `get` または `put` でディレクトリ操作が実行される |
| FC-FUNC-06 | sync 中にシンボリックリンクが追従される（実体が転送される） |
| FC-FUNC-07 | `--no-delete` 指定時に削除伝播が発生する |
| FC-FUNC-08 | 初回 sync 時に相手側のみ存在するファイルが削除される |
| FC-FUNC-09 | `--dry-run` 指定時に実際のファイル変更が発生する |
| FC-FUNC-10 | HTTP（非 HTTPS）接続が `--insecure` 未指定時に許可される |
| FC-FUNC-11 | ロックファイルが存在する状態で 2 つ目のプロセスが sync を開始する |
| FC-FUNC-12 | `sync_state.json` の書き込みが非 atomic で行われ、中断時にファイルが破損する |
| FC-FUNC-13 | プログレス表示が stdout に出力される（stderr であるべき） |
| FC-FUNC-14 | 並列転送時に 429 を受信しても他ワーカーが停止しない |

### 3.3 データ整合性失敗条件

| ID | 失敗条件 |
|---|---|
| FC-DATA-01 | ダウンロードしたファイルの内容がリモート側と一致しない |
| FC-DATA-02 | アップロードしたファイルのリモート側 Content-Type が自動検出結果と一致しない |
| FC-DATA-03 | push/pull 完了後の `sync_state.json` に記録されたファイル数が実際の転送ファイル数と一致しない |
| FC-DATA-04 | 日本語ファイル名の push → pull ラウンドトリップでファイル名が変化する |

---

## 4. E2E Test Generation Meta-Prompt

以下は `codd propagate` が E2E テストを自動生成するための機械可読な指示である。

### 4.1 テストレベル分離

E2E テストは以下の2つのレベルに厳密に分離する:

- **API インテグレーションテスト**: `rscli` コマンドをサブプロセスとして実行し、終了コード・stdout・stderr の出力を検証する。HTTP モックサーバーを使用して remoteStorage プロトコルのレスポンスをシミュレートする。
- **CLI インタラクションテスト**: 擬似 TTY（pty）を用いて対話的プロンプト（確認ダイアログ、client_id 入力等）の動作を検証する。

本プロジェクトは CLI ツールであり Web アプリケーションではないため、ブラウザテストは OAuth2 コールバックの受信確認に限定する。ただし、以下の原則を CLI テストに適用する:

- 全 HTTP レスポンスの検証で、ビジネスロジックのステータスコード（200、302、401 等）をチェックする前に、レスポンスステータスが 500 未満であることを先行検証する。5xx はサーバーエラー（未処理例外、DB ダウン）であり、4xx（認証失敗、未検出）とは根本的に異なる。
- コマンド実行結果の検証では、終了コードだけでなく stdout/stderr の内容も検証する。終了コード 0 でも不正な出力を返すケースを検出する。

### 4.2 MECE ドメイン分割

| ドメイン | 責務 | 出力ファイル（API） | 出力ファイル（CLI インタラクション） |
|---|---|---|---|
| auth | OAuth2 認証フロー、WebFinger、PKCE、トークン管理、refresh token | `tests/e2e/auth.spec.ts` | `tests/e2e/auth.browser.spec.ts` |
| security | トークン保護（0600/Keychain/CredentialManager）、HTTPS 強制 | `tests/e2e/security.spec.ts` | — |
| file-ops | ls、get、put、rm コマンド | `tests/e2e/file-ops.spec.ts` | `tests/e2e/file-ops.browser.spec.ts` |
| sync | push、pull、変更検出、コンフリクト、削除伝播、除外パターン | `tests/e2e/sync.spec.ts` | `tests/e2e/sync.browser.spec.ts` |
| retry | リトライ対象判定、exponential backoff、429 全ワーカー停止 | `tests/e2e/retry.spec.ts` | — |
| exit-codes | 全終了コードの正確性 | `tests/e2e/exit-codes.spec.ts` | — |
| resilience | シグナル処理、ロックファイル、sync_state.json 破損・リセット、atomic write | `tests/e2e/resilience.spec.ts` | — |
| transfer | ストリーミング、並列転送、プログレス表示、タイムアウト | `tests/e2e/transfer.spec.ts` | — |
| platform | クロスプラットフォームパス、パスエンコーディング、日本語ファイル名 | `tests/e2e/platform.spec.ts` | — |
| cli-options | dry-run、verbose、no-interactive、content-type | `tests/e2e/cli-options.spec.ts` | — |

### 4.3 シナリオ導出ルール

受入基準（セクション2）の各項目から以下の要領でテストシナリオを導出する:

1. **正常系**: 各受入基準を直接検証するシナリオ。入力→期待出力→終了コードを明示。
2. **異常系**: 失敗基準（セクション3）を反転して「正しく失敗すること」を検証するシナリオ。
3. **境界値**: タイムアウト境界（30秒/60秒）、リトライ回数上限（3回）、並列数（1/3/カスタム値）。
4. **状態遷移チェーン**: 認証フローの各ステップ（WebFinger → OAuth2 → トークン保存 → コマンド実行）をチェーンとして検証し、各リンクに個別のアサーションを設ける。

### 4.4 テスト環境・実行方式

本プロジェクトは Go 製 CLI ツールのため、テスト実行前に以下の準備を行う:

```
1. go build -o ./bin/rscli ./cmd/rscli
2. モックサーバー起動（remoteStorage プロトコル互換の HTTP サーバー）
3. モックサーバーのヘルスチェック待機（GET /health → 200）
4. テスト実行
5. モックサーバー停止・クリーンアップ
```

- モックサーバーは `tests/e2e/mock-server/` に配置し、WebFinger エンドポイント、OAuth2 認可エンドポイント、remoteStorage API エンドポイントを提供する。
- CI 環境ではモックサーバーをバックグラウンドで起動し、`wait-on` 等でヘルスチェック完了を待つ。
- CLI インタラクションテストは擬似 TTY 環境を使用するため、`node-pty` または Go の `os/exec` + pty パッケージを使用する。

### 4.5 アーキテクチャ適応ルール

- テスト生成時に `cmd/rscli/main.go` および各サブコマンドの登録コードをスキャンし、実際のコマンド・フラグ構造を検出する。
- 要件定義に記載されているが未実装のコマンド・フラグは `test.fixme()` でマークし、スキップしない。
- 要件定義にないコマンド・フラグが実装されている場合、警告コメントを付与して基本的な動作テストを生成する。

### 4.6 品質ゲート

テストスイート全体の合格基準:

- 全テストが PASS であること（SKIP は 0 件）。`test.fixme()` の未実装マーカーは別途カウントし、未実装機能一覧として報告する。
- 受入基準（セクション2）の全項目（AC-AUTH-01 〜 AC-PATH-02）に対応するテストが存在すること。
- リリースブロッキング制約（終了コード体系、トークン保護、リトライ仕様）の全テストが PASS であること。リリースブロッキングテストが 1 件でも FAIL の場合、テストスイート全体を FAIL とする。
- `test.fixme()` の件数が 0 であることがリリース条件。0 でない場合はリリース不可。

### 4.7 共有ヘルパー

`tests/e2e/helpers/` ディレクトリに以下の共有ヘルパーを配置する:

| ファイル | 責務 |
|---|---|
| `tests/e2e/helpers/mock-server.ts` | モックサーバーの起動・停止・設定 |
| `tests/e2e/helpers/auth-flow.ts` | OAuth2 認証フローのセットアップ（トークン事前配置、モック認可サーバー応答設定） |
| `tests/e2e/helpers/test-data.ts` | テスト用ファイル・ディレクトリの作成・クリーンアップ |
| `tests/e2e/helpers/assertions.ts` | 終了コード検証、stdout/stderr パターンマッチ、ファイルパーミッション検証、HTTP ステータス 5xx 先行検証 |
| `tests/e2e/helpers/cli-runner.ts` | `rscli` コマンドの実行ラッパー（サブプロセス起動、タイムアウト制御、出力キャプチャ） |
| `tests/e2e/helpers/pty-runner.ts` | 擬似 TTY 環境でのインタラクティブコマンド実行（プロンプト応答の自動化） |

### 4.8 生成マーカー

全生成ファイルの先頭に以下のヘッダを含める:

```typescript
// @generated-from: docs/acceptance-criteria.md
// @generated-by: codd propagate
```

手動で追加・編集したテストには `// @manual` マーカーを付与する。`codd propagate` の再生成時に `// @manual` マーカー付きのテストは保持し、上書きしない。

### 4.9 非交渉制約の反映

| 制約 | テストでの反映方法 |
|---|---|
| 終了コード体系（0/1/2/3/4） | `tests/e2e/exit-codes.spec.ts` で全コードを網羅。各ドメインテストでも該当シナリオの終了コードをアサーション。`assertions.ts` の `assertExitCode()` ヘルパーで統一的に検証。 |
| トークン保護（0600/Keychain/CredentialManager） | `tests/e2e/security.spec.ts` で OS 別にトークン保存場所とアクセス制御を検証。Linux テストでは `stat` コマンドでパーミッションを検証。CI が Linux の場合は 0600 テストを必須実行、macOS/Windows テストは対応 CI ランナーで実行。 |
| リトライ仕様（429/500/502/503、exponential backoff） | `tests/e2e/retry.spec.ts` でモックサーバーが指定ステータスを返す条件下でリトライ回数・待機時間・ジッター範囲を検証。`Retry-After` ヘッダの尊重はタイミングアサーション（許容誤差 ±200ms）で確認。 |
