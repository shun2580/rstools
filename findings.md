# Findings

<!-- codd:finding
{"details": {"missing": "ロックファイルのパス、staleロック検出メカニズム、手動解除方法（例: --force-unlockフラグ）", "requirement_text": "ロックファイルで同時実行を制御。既に実行中の場合はエラー終了（終了コード4）"}, "id": "lock_file_location_and_cleanup", "kind": "仕様不足", "name": "ロックファイルの保存場所とクリーンアップ方法が未定義", "question": "ロックファイルはどこに保存されますか？プロセスがクラッシュした場合のstaleロックファイルの検出・削除方法はどうしますか？（例: PIDチェック、タイムアウト）", "rationale": "クラッシュ後にロックファイルが残ると、ユーザーは手動削除しない限りsync操作を再開できなくなる。検出・回復方法の定義が必要。", "related_requirement_ids": ["シグナル処理"], "severity": "high", "source": "greenfield"}
-->
## lock_file_location_and_cleanup - ロックファイルの保存場所とクリーンアップ方法が未定義

- approval: [ ] `lock_file_location_and_cleanup`
- id: `lock_file_location_and_cleanup`
- kind: `仕様不足`
- severity: `high`
- name: ロックファイルの保存場所とクリーンアップ方法が未定義
- question: ロックファイルはどこに保存されますか？プロセスがクラッシュした場合のstaleロックファイルの検出・削除方法はどうしますか？（例: PIDチェック、タイムアウト）
- rationale: クラッシュ後にロックファイルが残ると、ユーザーは手動削除しない限りsync操作を再開できなくなる。検出・回復方法の定義が必要。
- related_requirement_ids: `シグナル処理`

```yaml
requirement_text: ロックファイルで同時実行を制御。既に実行中の場合はエラー終了（終了コード4）
missing: 'ロックファイルのパス、staleロック検出メカニズム、手動解除方法（例: --force-unlockフラグ）'
```

<!-- codd:finding
{"details": {"ambiguity": "dry-runがpush/pull/rm以外のコマンド（connect, disconnect, ls, get, put）でどう動作するか不明", "requirement_text": "グローバルオプション: --dry-run, --verbose"}, "id": "dry_run_behavior_per_command", "kind": "曖昧性", "name": "--dry-runの各コマンドごとの動作が未定義", "question": "--dry-runは全コマンドに適用可能ですか？connect/disconnectでの動作は何を表示しますか？lsでは意味がありますか？", "rationale": "グローバルオプションとして定義されている以上、全コマンドでの動作を明確にしないと、実装者による解釈のブレやユーザーの混乱が生じる。", "related_requirement_ids": ["CLIコマンド設計"], "severity": "medium", "source": "greenfield"}
-->
## dry_run_behavior_per_command - --dry-runの各コマンドごとの動作が未定義

- approval: [ ] `dry_run_behavior_per_command`
- id: `dry_run_behavior_per_command`
- kind: `曖昧性`
- severity: `medium`
- name: --dry-runの各コマンドごとの動作が未定義
- question: --dry-runは全コマンドに適用可能ですか？connect/disconnectでの動作は何を表示しますか？lsでは意味がありますか？
- rationale: グローバルオプションとして定義されている以上、全コマンドでの動作を明確にしないと、実装者による解釈のブレやユーザーの混乱が生じる。
- related_requirement_ids: `CLIコマンド設計`

```yaml
requirement_text: 'グローバルオプション: --dry-run, --verbose'
ambiguity: dry-runがpush/pull/rm以外のコマンド（connect, disconnect, ls, get, put）でどう動作するか不明
```

<!-- codd:finding
{"details": {"missing": "再接続時の既存トークン処理ポリシー", "requirement_text": "複数アカウントは今回スコープ外（1アカウントのみ）"}, "id": "connect_when_already_authenticated", "kind": "仕様不足", "name": "認証済み状態でconnectを再実行した場合の動作が未定義", "question": "既にトークンが存在する状態で `rscli connect` を実行した場合、上書きしますか？確認プロンプトを表示しますか？別アカウントへの切り替えは想定しますか？", "rationale": "ユーザーがサーバーURLやアカウントを変更したい場合の手順が曖昧。disconnect→connectの2ステップが必要か、connectが暗黙的に上書きするかで実装が異なる。", "related_requirement_ids": ["認証", "CLIコマンド設計"], "severity": "medium", "source": "greenfield"}
-->
## connect_when_already_authenticated - 認証済み状態でconnectを再実行した場合の動作が未定義

- approval: [ ] `connect_when_already_authenticated`
- id: `connect_when_already_authenticated`
- kind: `仕様不足`
- severity: `medium`
- name: 認証済み状態でconnectを再実行した場合の動作が未定義
- question: 既にトークンが存在する状態で `rscli connect` を実行した場合、上書きしますか？確認プロンプトを表示しますか？別アカウントへの切り替えは想定しますか？
- rationale: ユーザーがサーバーURLやアカウントを変更したい場合の手順が曖昧。disconnect→connectの2ステップが必要か、connectが暗黙的に上書きするかで実装が異なる。
- related_requirement_ids: `認証`, `CLIコマンド設計`

```yaml
requirement_text: 複数アカウントは今回スコープ外（1アカウントのみ）
missing: 再接続時の既存トークン処理ポリシー
```

<!-- codd:finding
{"details": {"missing": "refresh失敗→再認証フローへの遷移条件、非インタラクティブ環境（CI等）での動作", "requirement_text": "refresh token利用可能な場合は自動更新 / トークン期限切れ時は自動でブラウザを開いて再認証"}, "id": "token_refresh_failure_handling", "kind": "仕様不足", "name": "refresh token失敗時のフォールバック動作が未定義", "question": "refresh tokenによる自動更新が失敗した場合（revoke済み、サーバーエラー等）、即座にブラウザ再認証に移行しますか？それともエラー終了しますか？", "rationale": "refresh token失敗はよくあるケース。特にCI/スクリプト環境ではブラウザを開けないため、エラー終了コードや--no-interactiveフラグの要否を決める必要がある。", "related_requirement_ids": ["認証", "OAuth2詳細"], "severity": "high", "source": "greenfield"}
-->
## token_refresh_failure_handling - refresh token失敗時のフォールバック動作が未定義

- approval: [ ] `token_refresh_failure_handling`
- id: `token_refresh_failure_handling`
- kind: `仕様不足`
- severity: `high`
- name: refresh token失敗時のフォールバック動作が未定義
- question: refresh tokenによる自動更新が失敗した場合（revoke済み、サーバーエラー等）、即座にブラウザ再認証に移行しますか？それともエラー終了しますか？
- rationale: refresh token失敗はよくあるケース。特にCI/スクリプト環境ではブラウザを開けないため、エラー終了コードや--no-interactiveフラグの要否を決める必要がある。
- related_requirement_ids: `認証`, `OAuth2詳細`

```yaml
requirement_text: refresh token利用可能な場合は自動更新 / トークン期限切れ時は自動でブラウザを開いて再認証
missing: refresh失敗→再認証フローへの遷移条件、非インタラクティブ環境（CI等）での動作
```

<!-- codd:finding
{"details": {"missing": "atomic writeは破損リスクを軽減するが、ディスク障害やユーザーの誤編集には対応できない", "requirement_text": "sync_state.jsonはatomic writeで保存"}, "id": "sync_state_corruption_recovery", "kind": "仕様不足", "name": "sync_state.jsonが破損した場合の回復方法が未定義", "question": "sync_state.jsonのJSONパースに失敗した場合、初回sync扱いにしますか？エラー終了しますか？バックアップから復元しますか？", "rationale": "状態ファイルが破損した場合に初回sync扱いにするとデータの重複やコンフリクト扱いが発生する。ユーザーに修復手段を提供する必要がある。", "related_requirement_ids": ["Sync仕様", "シグナル処理"], "severity": "medium", "source": "greenfield"}
-->
## sync_state_corruption_recovery - sync_state.jsonが破損した場合の回復方法が未定義

- approval: [ ] `sync_state_corruption_recovery`
- id: `sync_state_corruption_recovery`
- kind: `仕様不足`
- severity: `medium`
- name: sync_state.jsonが破損した場合の回復方法が未定義
- question: sync_state.jsonのJSONパースに失敗した場合、初回sync扱いにしますか？エラー終了しますか？バックアップから復元しますか？
- rationale: 状態ファイルが破損した場合に初回sync扱いにするとデータの重複やコンフリクト扱いが発生する。ユーザーに修復手段を提供する必要がある。
- related_requirement_ids: `Sync仕様`, `シグナル処理`

```yaml
requirement_text: sync_state.jsonはatomic writeで保存
missing: atomic writeは破損リスクを軽減するが、ディスク障害やユーザーの誤編集には対応できない
```

<!-- codd:finding
{"details": {"impact": "CLIツールはスクリプト化されることが多く、インタラクティブ操作前提だとCI/自動化で使えない", "missing": "トークンの事前プロビジョニング方法、--yesや--no-interactiveフラグ、環境変数によるclient_id指定"}, "id": "non_interactive_environment_support", "kind": "仕様不足", "name": "非インタラクティブ環境（CI/cron）での動作が未定義", "question": "CI環境やcronジョブでの使用は想定していますか？ブラウザ認証やインタラクティブプロンプト（rm -rの確認、client_id入力等）が使えない場合はどう動作しますか？", "rationale": "ターゲットユーザーが開発者であるため、CI/cron連携は高い確率で求められる。認証フローとインタラクティブプロンプトの両方でバッチモードを検討すべき。", "related_requirement_ids": ["認証", "CLIコマンド追加", "OAuth2クライアント登録フォールバック"], "severity": "high", "source": "greenfield"}
-->
## non_interactive_environment_support - 非インタラクティブ環境（CI/cron）での動作が未定義

- approval: [ ] `non_interactive_environment_support`
- id: `non_interactive_environment_support`
- kind: `仕様不足`
- severity: `high`
- name: 非インタラクティブ環境（CI/cron）での動作が未定義
- question: CI環境やcronジョブでの使用は想定していますか？ブラウザ認証やインタラクティブプロンプト（rm -rの確認、client_id入力等）が使えない場合はどう動作しますか？
- rationale: ターゲットユーザーが開発者であるため、CI/cron連携は高い確率で求められる。認証フローとインタラクティブプロンプトの両方でバッチモードを検討すべき。
- related_requirement_ids: `認証`, `CLIコマンド追加`, `OAuth2クライアント登録フォールバック`

```yaml
missing: トークンの事前プロビジョニング方法、--yesや--no-interactiveフラグ、環境変数によるclient_id指定
impact: CLIツールはスクリプト化されることが多く、インタラクティブ操作前提だとCI/自動化で使えない
```

<!-- codd:finding
{"details": {"missing": "検出メソッド、フォールバック値（application/octet-stream等）、ユーザーによる明示指定オプション", "requirement_text": "Content-Typeは自動検出"}, "id": "content_type_detection_method", "kind": "曖昧性", "name": "Content-Type自動検出の方法が未定義", "question": "Content-Typeの自動検出はファイル拡張子ベース（mime.TypeByExtension）ですか？ファイル内容のmagic bytesベースですか？拡張子がない場合のデフォルトは？", "rationale": "remoteStorageはContent-Typeをメタデータとして保存するため、誤検出はデータの取り扱いに影響する。putコマンドでの--content-typeオプションも検討すべき。", "related_requirement_ids": ["ファイル転送"], "severity": "medium", "source": "greenfield"}
-->
## content_type_detection_method - Content-Type自動検出の方法が未定義

- approval: [ ] `content_type_detection_method`
- id: `content_type_detection_method`
- kind: `曖昧性`
- severity: `medium`
- name: Content-Type自動検出の方法が未定義
- question: Content-Typeの自動検出はファイル拡張子ベース（mime.TypeByExtension）ですか？ファイル内容のmagic bytesベースですか？拡張子がない場合のデフォルトは？
- rationale: remoteStorageはContent-Typeをメタデータとして保存するため、誤検出はデータの取り扱いに影響する。putコマンドでの--content-typeオプションも検討すべき。
- related_requirement_ids: `ファイル転送`

```yaml
requirement_text: Content-Typeは自動検出
missing: 検出メソッド、フォールバック値（application/octet-stream等）、ユーザーによる明示指定オプション
```

<!-- codd:finding
{"details": {"missing": "ディレクトリ一覧のサイズ制限、ストリーミング出力、再帰一覧（-R）時のメモリ管理", "protocol_note": "draft-22のディレクトリ一覧はJSON一括レスポンスのため、巨大ディレクトリでのメモリ問題が起こりうる"}, "id": "large_directory_listing_handling", "kind": "仕様不足", "name": "大規模ディレクトリ一覧取得時の動作が未定義", "question": "数万ファイルを含むディレクトリをlsした場合のメモリ使用やページネーションの扱いはどうしますか？remoteStorageプロトコルにおけるディレクトリレスポンスは一括取得ですか？", "rationale": "再帰一覧（-R）でストレージ全体を走査する場合、メモリ消費とHTTPリクエスト数が問題になる可能性がある。", "related_requirement_ids": ["lsコマンド出力"], "severity": "medium", "source": "greenfield"}
-->
## large_directory_listing_handling - 大規模ディレクトリ一覧取得時の動作が未定義

- approval: [ ] `large_directory_listing_handling`
- id: `large_directory_listing_handling`
- kind: `仕様不足`
- severity: `medium`
- name: 大規模ディレクトリ一覧取得時の動作が未定義
- question: 数万ファイルを含むディレクトリをlsした場合のメモリ使用やページネーションの扱いはどうしますか？remoteStorageプロトコルにおけるディレクトリレスポンスは一括取得ですか？
- rationale: 再帰一覧（-R）でストレージ全体を走査する場合、メモリ消費とHTTPリクエスト数が問題になる可能性がある。
- related_requirement_ids: `lsコマンド出力`

```yaml
missing: ディレクトリ一覧のサイズ制限、ストリーミング出力、再帰一覧（-R）時のメモリ管理
protocol_note: draft-22のディレクトリ一覧はJSON一括レスポンスのため、巨大ディレクトリでのメモリ問題が起こりうる
```

<!-- codd:finding
{"details": {"missing": "429受信時の全ワーカーへの影響範囲、ワーカーごとの独立リトライか全体協調か", "requirement_text": "並列転送数はデフォルト3 / 429はRetry-Afterヘッダを尊重"}, "id": "parallel_transfer_error_interaction", "kind": "曖昧性", "name": "並列転送時のリトライ・エラーの相互作用が未定義", "question": "並列転送中に1ファイルが429を受けた場合、他のワーカーも停止しますか？リトライ待機は全体のスロットルに影響しますか？Retry-Afterは全ワーカーに適用しますか？", "rationale": "429はサーバーのレート制限を示すため、1ワーカーの429を他のワーカーが無視すると追加の429やブロックを招く。協調的なバックオフが必要な可能性がある。", "related_requirement_ids": ["ファイル転送詳細", "リトライ詳細"], "severity": "medium", "source": "greenfield"}
-->
## parallel_transfer_error_interaction - 並列転送時のリトライ・エラーの相互作用が未定義

- approval: [ ] `parallel_transfer_error_interaction`
- id: `parallel_transfer_error_interaction`
- kind: `曖昧性`
- severity: `medium`
- name: 並列転送時のリトライ・エラーの相互作用が未定義
- question: 並列転送中に1ファイルが429を受けた場合、他のワーカーも停止しますか？リトライ待機は全体のスロットルに影響しますか？Retry-Afterは全ワーカーに適用しますか？
- rationale: 429はサーバーのレート制限を示すため、1ワーカーの429を他のワーカーが無視すると追加の429やブロックを招く。協調的なバックオフが必要な可能性がある。
- related_requirement_ids: `ファイル転送詳細`, `リトライ詳細`

```yaml
requirement_text: 並列転送数はデフォルト3 / 429はRetry-Afterヘッダを尊重
missing: 429受信時の全ワーカーへの影響範囲、ワーカーごとの独立リトライか全体協調か
```

<!-- codd:finding
{"details": {"missing": "パス内の非ASCII文字・特殊文字の処理、ローカルとリモートでのファイル名マッピングルール", "protocol_note": "remoteStorageのパスはURLパスセグメントとしてエンコードされるため、文字種の制限がある可能性がある"}, "id": "remote_path_encoding_and_special_chars", "kind": "仕様不足", "name": "リモートパスのエンコーディングと特殊文字の取り扱いが未定義", "question": "リモートパスに日本語やスペース、特殊文字（#, %, +等）を含むファイル名はどう扱いますか？URLエンコーディングはCLI側で自動的に行いますか？", "rationale": "日本語ファイル名は対象ユーザー環境で頻出する可能性が高く、エンコーディング不整合はデータ損失の原因になる。", "related_requirement_ids": ["CLIコマンド設計", "ファイル転送"], "severity": "medium", "source": "greenfield"}
-->
## remote_path_encoding_and_special_chars - リモートパスのエンコーディングと特殊文字の取り扱いが未定義

- approval: [ ] `remote_path_encoding_and_special_chars`
- id: `remote_path_encoding_and_special_chars`
- kind: `仕様不足`
- severity: `medium`
- name: リモートパスのエンコーディングと特殊文字の取り扱いが未定義
- question: リモートパスに日本語やスペース、特殊文字（#, %, +等）を含むファイル名はどう扱いますか？URLエンコーディングはCLI側で自動的に行いますか？
- rationale: 日本語ファイル名は対象ユーザー環境で頻出する可能性が高く、エンコーディング不整合はデータ損失の原因になる。
- related_requirement_ids: `CLIコマンド設計`, `ファイル転送`

```yaml
missing: パス内の非ASCII文字・特殊文字の処理、ローカルとリモートでのファイル名マッピングルール
protocol_note: remoteStorageのパスはURLパスセグメントとしてエンコードされるため、文字種の制限がある可能性がある
```
