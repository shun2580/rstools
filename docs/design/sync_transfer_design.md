---
codd:
  node_id: design:sync-transfer-design
  type: design
  depends_on:
  - id: design:system-design
    relation: depends_on
    semantic: technical
  depended_by:
  - id: design:component-dependency-map
    relation: depends_on
    semantic: technical
  - id: design:sync-state-machine
    relation: depends_on
    semantic: technical
  - id: design:transfer-retry-flow
    relation: depends_on
    semantic: technical
  conventions:
  - targets:
    - module:sync
    reason: '初回sync時の挙動(push: ローカルのみ→アップロード/リモートのみ→スキップ、pull: 逆)は仕様必須。違反はデータ損失リスクでリリース不可。'
  - targets:
    - module:sync
    reason: sync_state.jsonはatomic write必須。中断後の再開可能性はリリース要件。ファイル毎に更新。
  - targets:
    - module:transfer
    reason: 429受信時は全ワーカー一時停止しRetry-After値を全ワーカーに適用して再開。並列転送デフォルト3。
  - targets:
    - module:sync
    reason: '削除伝播: push時ローカル削除→リモート削除、pull時リモート削除→ローカル削除。--no-deleteで無効化可能。初回syncでは削除伝播しない。'
  modules:
  - sync
  - transfer
---

# 同期・転送設計

## 1. Overview

本設計書は `rscli` の `module:sync` および `module:transfer` の実装仕様を定義する。`rscli push` / `rscli pull` サブコマンドが提供する双方向ファイル同期、並列ファイル転送エンジン、429 レートリミット制御、`sync_state.json` の atomic write による再開可能性を網羅する。

### 1.1 対象モジュールと責務分担

| モジュール | ソースパス | 主責務 |
|---|---|---|
| `module:sync` | `internal/sync/` | 変更検出・同期ルール・削除伝播・`sync_state.json` 管理・除外パターン |
| `module:transfer` | `internal/transfer/` | ストリーミング転送・並列ワーカープール・リトライ・429 全ワーカー一時停止・プログレス表示 |

### 1.2 リリースブロッキング制約一覧

本設計書は以下 4 件のリリースブロッキング制約に完全準拠する。各制約がどのメカニズムで保証されるかを Section 2 で明示する。

| # | 対象 | 制約 | 本設計書での反映箇所 |
|---|---|---|---|
| SC-1 | `module:sync` | 初回 sync 時の挙動（push: ローカルのみ→アップロード、リモートのみ→スキップ。pull: リモートのみ→ダウンロード、ローカルのみ→スキップ）は仕様必須。違反はデータ損失リスクでリリース不可 | 2.3 同期ルール詳細 |
| SC-2 | `module:sync` | `sync_state.json` は atomic write 必須。中断後の再開可能性はリリース要件。ファイル毎に更新 | 2.4 sync_state.json |
| SC-3 | `module:transfer` | 429 受信時は全ワーカー一時停止し `Retry-After` 値を全ワーカーに適用して再開。並列転送デフォルト 3 | 2.6 並列転送と 429 制御 |
| SC-4 | `module:sync` | 削除伝播: push 時ローカル削除→リモート削除、pull 時リモート削除→ローカル削除。`--no-delete` で無効化可能。初回 sync では削除伝播しない | 2.3 同期ルール詳細 |

---

## 2. Architecture

### 2.1 モジュール内ファイル構成

```
internal/sync/
  engine.go       … push / pull 同期ロジックのエントリポイント
  state.go        … sync_state.json 読み書き（atomic write、ファイル毎更新）
  conflict.go     … コンフリクト検出・スキップ通知・--force 上書き
  ignore.go       … .rsignore / --exclude パターンマッチ（.gitignore 形式）
  delete.go       … 削除伝播ロジック・--no-delete ガード・初回 sync 保護

internal/transfer/
  stream.go       … ストリーミング転送（メモリ非読込）
  parallel.go     … ワーカープール（デフォルト 3、--parallel N で変更）
  progress.go     … stderr プログレス表示（N/M files 形式、非 TTY 時抑制）
  retry.go        … exponential backoff・429/500/502/503 対象・Retry-After 尊重
  contenttype.go  … Content-Type 自動検出（mime.TypeByExtension）・--content-type 上書き
```

### 2.2 変更検出方式

変更検出は `module:sync` の `engine.go` が `module:remotestorage` を呼び出して実行する。

| 対象 | 一次方式 | フォールバック |
|---|---|---|
| リモートファイル | ETag（`If-None-Match` / `If-Match`） | `Last-Modified` ヘッダ（ETag 非対応サーバー） |
| ローカルファイル | mtime | なし |

`sync_state.json` に記録された前回 ETag / mtime と現在値を比較し、変更有無を判定する。初回 sync（`sync_state.json` 未存在）では前回値がないため、比較なしに SC-1 で規定される初回 sync ルールを直接適用する。

### 2.3 同期ルール詳細

#### 2.3.1 初回 sync 挙動（SC-1 準拠 — リリースブロッキング）

`sync_state.json` が存在しない、または `--reset-state` で削除された場合を初回 sync とする。初回 sync での削除伝播は禁止（SC-4 の初回 sync 保護）。

| ファイル存在状態 | `push` の動作 | `pull` の動作 |
|---|---|---|
| ローカルのみ存在 | **アップロード**（リモートに送信） | **スキップ**（何もしない） |
| リモートのみ存在 | **スキップ**（何もしない） | **ダウンロード**（ローカルに取得） |
| 両方に存在 | ETag / mtime 比較で変更検出し通常ルールを適用 | ETag / mtime 比較で変更検出し通常ルールを適用 |
| 削除伝播 | **実行しない**（初回 sync 保護） | **実行しない**（初回 sync 保護） |

`engine.go` は `sync_state.json` の有無を起動時に確認し、未存在時は `isFirstSync = true` フラグを立てて上記ルール分岐に入る。このフラグが `false` に設定されるのは `sync_state.json` のパースが成功した場合のみである。

#### 2.3.2 通常 sync 挙動（2 回目以降）

| シナリオ | `push` の動作 | `pull` の動作 |
|---|---|---|
| ローカルのみ変更 | アップロード | スキップ |
| リモートのみ変更 | スキップ | ダウンロード |
| 双方変更（コンフリクト） | デフォルト: スキップ + stderr 通知 | デフォルト: スキップ + stderr 通知 |
| `--force` 指定時コンフリクト | 強制上書き（ローカルでリモートを上書き） | 強制上書き（リモートでローカルを上書き） |
| ローカルのみ存在（新規追加） | アップロード | スキップ |
| リモートのみ存在（新規追加） | スキップ | ダウンロード |

#### 2.3.3 削除伝播（SC-4 準拠 — リリースブロッキング）

削除伝播は `sync/delete.go` が担当する。初回 sync フラグが `true` の場合、`delete.go` は無条件にノーオペレーションを返す。

| 条件 | `push` の削除伝播 | `pull` の削除伝播 |
|---|---|---|
| 通常 sync かつ `--no-delete` 未指定 | `sync_state.json` に記録済みのファイルがローカルに存在しない場合→リモートを削除 | `sync_state.json` に記録済みのファイルがリモートに存在しない場合→ローカルを削除 |
| `--no-delete` 指定 | 削除伝播を実行しない | 削除伝播を実行しない |
| 初回 sync（`sync_state.json` 未存在） | 削除伝播を実行しない（SC-4 初回 sync 保護） | 削除伝播を実行しない（SC-4 初回 sync 保護） |

削除伝播の判定根拠は `sync_state.json` に記録された「前回 sync 時の既知ファイル集合」との差分である。`sync_state.json` が存在しない場合は既知ファイル集合が空であり、全ファイルを「前回から削除された」と誤判定するリスクがある。SC-4 の初回 sync 保護はこのデータ損失シナリオを防ぐためのリリースブロッキング要件である。

`--dry-run` 指定時は削除伝播の実行予定を stderr に出力するが、実際の削除は行わない。

### 2.4 sync_state.json（SC-2 準拠 — リリースブロッキング）

#### 2.4.1 配置とスキーマ

配置先: `{設定ディレクトリ}/sync_state.json`（`os.UserConfigDir()` + `remotestorage-cli/`）

```json
{
  "schema_version": 1,
  "entries": {
    "/path/to/file.txt": {
      "etag": "\"abc123\"",
      "mtime": 1716547200,
      "size": 1024,
      "synced_at": "2025-05-24T12:00:00Z"
    }
  }
}
```

エントリはファイルパス毎に独立して管理する。1 ファイルの転送完了ごとに該当エントリを更新する（ SC-2「ファイル毎に更新」の要件）。

#### 2.4.2 Atomic Write 実装（SC-2 準拠）

`sync/state.go` の `WriteState()` 関数は以下の手順で `sync_state.json` を更新する。

```
1. 同一ディレクトリに一時ファイルを作成
   path: {設定ディレクトリ}/sync_state.json.tmp.{pid}
2. 更新内容を一時ファイルに書き込み、fsync を呼び出してディスクに永続化
3. os.Rename(tmpPath, finalPath) でアトミックに置換
   （同一ファイルシステム上での Rename は POSIX 保証によりアトミック）
```

手順 3 の `os.Rename` が完了する前にプロセスが中断された場合、`sync_state.json` は前回の完全な状態を維持する。一時ファイルは次回起動時に自動削除する。

#### 2.4.3 シグナル処理との連携

`module:cli` の `cli/signal.go` が SIGINT / SIGTERM を受信した場合:

1. `module:transfer` のワーカープールに停止シグナルを送信する（新規転送タスクのキューイングを停止）。
2. **現在転送中のファイルの完了を待たない**（即時停止）。
3. 完了済みファイルのエントリのみを対象に `WriteState()` を呼び出し、`sync_state.json` を atomic write で更新する。
4. 未完了ファイルのエントリは `sync_state.json` に記録しない。
5. 次回 `push` / `pull` 実行時、`sync_state.json` に記録されていないファイルは未同期として再度処理される。これにより中断箇所からの再開が可能となる。

#### 2.4.4 破損と --reset-state

`sync_state.json` のパースに失敗した場合（不正 JSON 等）、エラーメッセージを stderr に出力して終了コード 4 で終了する。自動修復は行わない。`--reset-state` フラグ指定時は `sync_state.json` を削除してから同期を開始し、初回 sync 扱いにリセットする。

### 2.5 除外パターン

除外パターンの評価は `sync/ignore.go` が担当する。

| ソース | 形式 | 配置 |
|---|---|---|
| `.rsignore` | `.gitignore` 互換パターン | sync ルートディレクトリ（`push ./local` の場合 `./local/.rsignore`） |
| `--exclude <pattern>` | シェルグロブ（複数指定可） | コマンドライン引数 |

両ソースが同時に存在する場合、両方のパターンを論理 OR で適用する。いずれかにマッチするファイルはスキップされ、`sync_state.json` にも記録しない。空ディレクトリは remoteStorage プロトコル仕様に従いスキップする。シンボリックリンクおよび特殊ファイル（デバイスファイル等）は OS 問わずスキップし、warning を stderr に出力する。

### 2.6 並列転送と 429 制御（SC-3 準拠 — リリースブロッキング）

#### 2.6.1 ワーカープール

`transfer/parallel.go` はゴルーチンベースのワーカープールを実装する。

- **デフォルトワーカー数**: 3（SC-3 準拠）
- **変更方法**: `--parallel N` フラグ（`rscli push` / `rscli pull` で有効）
- タスクキューはバッファ付きチャネル。各ワーカーはチャネルからタスクを取得し `transfer/stream.go` を呼び出す。

#### 2.6.2 429 全ワーカー一時停止（SC-3 準拠）

SC-3 は 429 受信時の全ワーカー一時停止を義務付けるリリースブロッキング制約である。実装は以下のとおり。

```
1. いずれかのワーカーが HTTP 429 を受信した場合:
   a. そのワーカーは globalPauseRequest チャネルに Retry-After 値（秒）を送信する
   b. ワーカープールのスーパーバイザーが globalPauseRequest を受信し、
      全ワーカーへの一時停止シグナルをブロードキャストする（sync.Cond または select）
   c. 全ワーカーは現在処理中のリクエストを中断せず、次のタスク取得をブロックする
2. スーパーバイザーは Retry-After 秒（なければ指数バックオフ値）待機する
3. 待機完了後、スーパーバイザーは全ワーカーの再開シグナルをブロードキャストする
4. 全ワーカーが一斉に再開し、タスクキューからの取得を再開する
```

`Retry-After` ヘッダが存在しない場合、`transfer/retry.go` の通常 exponential backoff 値（初回 1 秒・最大 30 秒・フルジッター）を使用する。`Retry-After` が存在する場合は `Retry-After` 値と backoff 算出値の大きい方を待機時間とする。

#### 2.6.3 リトライ仕様

`transfer/retry.go` のリトライ仕様は `system_design.md` セクション 2.3 のリリースブロッキング仕様に準拠する。

| パラメータ | 値 |
|---|---|
| リトライ対象ステータス | 429、500、502、503 のみ |
| 最大リトライ回数 | 3 回 |
| バックオフ初回待機 | 1 秒 |
| バックオフ最大待機 | 30 秒 |
| バックオフ方式 | exponential backoff フルジッター（`sleep = random(0, min(30, 1 * 2^attempt))`） |
| 429 Retry-After 処理 | backoff 値と Retry-After 値の大きい方を採用 |
| 部分失敗時の継続 | リトライ上限到達ファイルがあっても残りのファイルを処理継続、全完了後にエラー一覧を stderr に出力、終了コード 1 |

400、401、403、404 等のクライアントエラーはリトライしない。

### 2.7 ストリーミング転送

`transfer/stream.go` はファイル全体をメモリに読み込まないストリーミング方式で転送する。アップロード（push）では `os.Open` で取得した `io.Reader` を HTTP リクエストボディに直接渡す。ダウンロード（pull）では HTTP レスポンスボディを `os.Create` で取得した `io.Writer` に `io.Copy` でストリーミングする。ファイルサイズに上限は設けない。

### 2.8 プログレス表示

`transfer/progress.go` はファイル転送の進捗を stderr に出力する。

- **形式**: `N/M files`（例: `3/10 files`）
- **TTY 判定**: `golang.org/x/term.IsTerminal(os.Stderr.Fd())` で判定し、非 TTY 環境（CI、パイプ接続等）ではプログレス表示を自動抑制する
- **更新タイミング**: 1 ファイルの転送完了ごとに更新

### 2.9 Content-Type 処理

`transfer/contenttype.go` はアップロード時の `Content-Type` ヘッダを決定する。

- **自動検出**: `mime.TypeByExtension(filepath.Ext(filename))` を使用
- **不明拡張子**: `application/octet-stream` を使用
- **明示上書き**: `--content-type` フラグで任意の MIME タイプを指定可能（`put` および `push` で有効）

### 2.10 --dry-run 動作

`push` / `pull` に `--dry-run` を指定した場合、実際のファイル転送・削除・`sync_state.json` 更新を行わず、実行予定の操作一覧を stdout に出力して終了コード 0 で終了する。プログレス表示は行わない。

### 2.11 HTTPS 強制との連携

`module:sync` および `module:transfer` が `module:remotestorage` の HTTP クライアントを経由してリモートストレージと通信する際、`system_design.md` セクション 2.3 で規定されるとおり URL スキームが `https://` であることの検証が HTTP クライアント側で強制される。`module:sync` / `module:transfer` はこの検証を重複して実装しない。`--insecure` フラグは TLS 証明書検証スキップのみを制御し、HTTP（非暗号化）通信を許可しない。

### 2.12 非交渉制約の設計レベル保証まとめ

| 制約 | 実装ファイル | 保証メカニズム |
|---|---|---|
| SC-1: 初回 sync 挙動 | `sync/engine.go`、`sync/state.go` | `sync_state.json` 未存在時に `isFirstSync = true` フラグを立て、ローカルのみ→push アップロード、リモートのみ→push スキップ、リモートのみ→pull ダウンロード、ローカルのみ→pull スキップのルール分岐に入る |
| SC-2: sync_state.json atomic write | `sync/state.go` | 一時ファイル書き込み → `os.Rename` による置換。ファイル完了ごとに `WriteState()` を呼び出し。SIGINT/SIGTERM 時は完了済みエントリのみ記録 |
| SC-3: 429 全ワーカー一時停止・デフォルト 3 並列 | `transfer/parallel.go`、`transfer/retry.go` | globalPauseRequest チャネルによる全ワーカーへの一斉停止・Retry-After 適用後の一斉再開。デフォルトワーカー数定数 = 3 |
| SC-4: 削除伝播・--no-delete・初回 sync 保護 | `sync/delete.go`、`sync/engine.go` | `isFirstSync = true` 時は `delete.go` がノーオペレーション。`--no-delete` 指定時も同様。通常 sync では `sync_state.json` 既知集合との差分で削除対象を特定 |

---

## 3. Open Questions

| # | 問い | 背景 | 判断時期 |
|---|---|---|---|
| OQ-S1 | `sync_state.json` のスキーマバージョニング戦略 | 現在 `schema_version: 1` を定義しているが、エントリ構造の変更（チェックサム追加等）が必要になった場合のマイグレーション方針が未定 | スキーマ変更を伴う機能追加が計画された時点 |
| OQ-S2 | コンフリクト解決の自動化オプション | 現在はデフォルトスキップ + 通知、`--force` で強制上書き。ユーザーが双方向差分を確認して選択できるインタラクティブモードの需要があるか | ユーザーフィードバックでコンフリクト頻度が報告された時点 |
| OQ-S3 | 並列転送数の上限値 | `--parallel N` に上限を設けるべきか。サーバー側がレートリミットを設定している場合、高い並列数が 429 を誘発しやすくなる。推奨値や警告閾値を設定するかどうかが未定 | 実サーバーでの性能測定結果に基づき判断 |
| OQ-S4 | 大規模ディレクトリ（10 万ファイル超）での `sync_state.json` サイズ問題 | エントリ数が増えると `sync_state.json` の読み書きコストが増大する可能性がある。SQLite 等の組み込み DB への移行を検討するタイミングが不明 | ベンチマークで JSON 方式のボトルネックが確認された時点 |
| OQ-S5 | `--dry-run` 出力の機械可読フォーマット | 現在の `--dry-run` 出力は人間可読テキストのみ。CI パイプラインでの利用を想定した `--dry-run --json` 出力の需要があるか | CI 連携ユースケースが報告された時点 |
| OQ-S6 | 複数アカウント対応時の `sync_state.json` 分離 | `system_design.md` OQ-3 で複数アカウント対応が将来課題として挙げられている。複数アカウント時は `sync_state.{user}@{host}.json` 等のアカウント別ファイルに分離する必要があるが、現設計では単一ファイルを前提としている | `system_design.md` OQ-3 の判断と同タイミング |
