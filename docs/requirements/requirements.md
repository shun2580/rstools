---
codd:
  node_id: "req:remotestorage-cli-requirements"
  type: requirement
  status: approved
  confidence: 0.95
---

# remotestorage-cli 要件定義

## AI出力言語設定
- findingsを含む全ての出力は日本語で行うこと
- 技術用語は英語のままでよいが、説明文は日本語で記述すること

## 概要
remoteStorageプロトコルを操作するGo製CLIツール

## やりたいこと
- remoteStorageサーバーに接続・認証する
- ファイルの一覧表示・アップロード・ダウンロード・削除ができる
- ローカルフォルダとremoteStorageを同期できる（まずpush/pull一方向、後に双方向）

## 認証
- OAuth2 Authorization Code + localhostリダイレクト方式
- スコープはデフォルト`*:rw`、オプションで変更可能
- トークンは`~/.config/remotestorage-cli/token.json`に保存
- トークン期限切れ時は自動でブラウザを開いて再認証
- WebFinger対応（user@host形式でサーバーを自動検出）
- 複数アカウントは今回スコープ外（1アカウントのみ）

## Sync仕様
- まずpush（ローカル→リモート）とpull（リモート→ローカル）を別コマンドで実装
- 将来的にsync（双方向）を追加予定
- 変更検出はETagとタイムスタンプを使用
- Sync状態は`~/.config/remotestorage-cli/sync_state.json`に保存
- コンフリクト時はデフォルトでスキップしてユーザーに通知、`--force`で強制上書き
- `.gitignore`形式の除外パターン対応
- 空ディレクトリはremoteStorageの仕様に合わせてスキップ

## ファイル転送
- ストリーミング転送（大きなファイル対応）
- Content-Typeは自動検出

## エラー・リトライ
- 3回リトライ
- 失敗しても続行してエラー一覧を最後に表示

## CLIコマンド設計
- `rscli connect user@host` — 認証
- `rscli ls /path` — 一覧表示
- `rscli get /remote ./local` — ダウンロード
- `rscli put ./local /remote` — アップロード
- `rscli rm /path` — 削除
- `rscli push ./local /remote` — ローカル→リモート同期
- `rscli pull /remote ./local` — リモート→ローカル同期
- グローバルオプション: `--dry-run`, `--verbose`

## 技術スタック
- 言語: Go
- 配布: GitHub Releases（go installも対応）
- 対応OS: Linux / macOS / Windows

## ターゲットユーザー
- remoteStorageを使っている開発者・個人ユーザー

## プロトコル仕様
- 対象バージョン: draft-dejong-remotestorage-22（最新）
- プロトコルバージョンによる差異はdraft-22を基準とする

## OAuth2詳細
- localhostリダイレクトのポートは動的に選択
- PKCE（RFC 7636）を使用
- refresh token利用可能な場合は自動更新
- client_idは動的登録（RFC 7591）を優先、非対応サーバーはユーザーが手動入力

## コンフリクト検出ロジック
- ETagを優先して変更検出
- 初回sync時（sync_state.json未存在）は既存ファイルをコンフリクトとしてスキップ

## リトライ詳細
- 対象: HTTPステータス429・500・502・503
- Exponential backoff方式
- 429はRetry-Afterヘッダを尊重

## ファイル転送詳細
- ファイルサイズ上限なし
- 並列転送数はデフォルト3（`--parallel`オプションで変更可能）
- プログレス表示あり

## 特殊ファイル
- シンボリックリンク・特殊ファイルはスキップしてwarning表示

## セキュリティ
- token.jsonのパーミッションは0600を強制

## lsコマンド出力
- デフォルト: ファイル名・サイズ・更新日時を表示
- `--json`オプションでJSON形式出力
- `-R`オプションで再帰的表示

## 除外パターン
- syncルートの`.rsignore`ファイルで定義
- `--exclude`オプションでコマンドライン指定も可能

## CLIコマンド追加
- `rscli disconnect` — ローカルトークンを削除してログアウト
- `rscli rm -r /path/` — ディレクトリを再帰削除（確認プロンプトあり）
- get/putはファイル単体のみ対応。ディレクトリ操作はpush/pullを使用

## クロスプラットフォーム対応
- 設定ディレクトリは`os.UserConfigDir()`を使用（OS標準パスに従う）
- WindowsのトークンはWindows Credential Managerで保護

## 変更検出の詳細
- ローカルファイルの変更検出はmtimeを使用
- リモートの変更検出はETag優先、非対応時はLast-Modifiedにフォールバック
- sync_state.jsonはファイル毎に更新（中断後の再実行で再開可能）

## 削除の伝播
- push時: ローカルで削除したファイルはリモートからも削除
- pull時: リモートで削除したファイルはローカルからも削除
- `--no-delete`オプションで削除伝播を無効化可能

## OAuth2クライアント登録フォールバック
- RFC 7591非対応サーバーはインタラクティブプロンプトでclient_idを入力
- client_secretは不要

## ネットワーク
- HTTPS必須
- `--insecure`フラグで自己署名証明書を許可（開発・テスト環境向け）

## 終了コード
- 0: 成功
- 1: 部分的失敗（一部ファイルのみ転送成功）
- 2: 認証失敗
- 3: ネットワークエラー
- 4: その他エラー

## シグナル処理
- sync_state.jsonはatomic writeで保存
- 中断時は完了済みファイルのみ記録。再実行で再開可能
- ロックファイルで同時実行を制御。既に実行中の場合はエラー終了（終了コード4）

## macOSトークン保護
- macOS Keychainを使用

## Windowsトークン保護
- Windows Credential ManagerにAccessToken・RefreshTokenを保存
- token.jsonファイルは不要
- WSL環境はLinux扱い（ファイルパーミッション0600）

## 初回sync時の挙動
- push時: ローカルのみ存在→アップロード、リモートのみ存在→スキップ（削除しない）
- pull時: リモートのみ存在→ダウンロード、ローカルのみ存在→スキップ（削除しない）

## Exponential backoffパラメータ
- 初回待機: 1秒
- 最大待機: 30秒
- フルジッターあり

## HTTPタイムアウト
- 接続タイムアウト: 30秒
- アイドルタイムアウト: 60秒

## プログレス表示
- stderr出力
- 全体進捗（N/M files）形式
- 非TTY時は自動抑制

## WebFinger失敗時のフォールバック
- WebFingerが失敗した場合、ストレージURLを直接入力するフォールバックあり

## ロックファイル
- 保存場所: `{設定ディレクトリ}/lock`
- PIDを記録し、起動時に対象プロセスが存在しない場合はstaleとして自動削除
- `--force-unlock`フラグで手動解除可能

## --dry-runの適用範囲
- 適用: push / pull / rm（変更を伴うコマンドのみ）
- 非適用: connect / disconnect / ls / get / put（意味がないため無視）

## connect再実行時の動作
- 既存トークンがある場合は確認プロンプトを表示
- 確認後に上書き。`--yes`フラグで確認スキップ

## refresh token失敗時
- 失敗時はブラウザ再認証へ移行
- `--no-interactive`フラグ指定時はエラー終了（終了コード2）

## sync_state.json破損時
- パースエラー時はエラー終了してユーザーに通知
- `--reset-state`フラグで初回sync扱いにリセット可能

## 非インタラクティブ環境
- `--no-interactive`フラグで全プロンプトをスキップ
- 認証はトークンが事前に存在する場合のみ動作

## Content-Type検出
- 拡張子ベース（mime.TypeByExtension）を優先
- 不明な場合は`application/octet-stream`
- `--content-type`オプションで明示指定可能

## 大規模ディレクトリ
- プロトコル仕様に従い一括取得
- メモリ警告なし（プロトコルの制約として受け入れる）

## 並列転送時の429処理
- 429受信時は全ワーカーを一時停止
- Retry-Afterヘッダの値を全ワーカーに適用して再開

## パスエンコーディング
- URLエンコーディングはCLI側で自動的に処理
- 日本語・特殊文字は透過的に扱う