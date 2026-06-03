---
codd:
  node_id: governance:adr-cross-platform
  type: governance
  depends_on:
  - id: req:remotestorage-cli-requirements
    relation: derives_from
    semantic: governance
  depended_by:
  - id: design:system-design
    relation: constrained_by
    semantic: governance
  - id: design:credential-storage-detail
    relation: constrained_by
    semantic: governance
  conventions:
  - targets:
    - module:config
    - module:auth
    reason: os.UserConfigDir()使用、OS別トークン保護(Keychain/Credential Manager/0600)はリリース必須要件。
  - targets:
    - cli_command:rscli
    reason: Linux/macOS/Windowsの3 OS対応は配布要件。未対応OSがあればリリース不可。
  modules:
  - config
  - auth
  - cli
---

# ADR: クロスプラットフォーム対応方針

## 1. Overview

本ADRは、remotestorage-cli（`rscli`）をLinux・macOS・Windowsの3 OSで動作させるための設計判断を記録する。対象は設定ディレクトリの解決、トークンの保護方式、およびバイナリ配布の3領域である。

### スコープ

本ADRの判断は以下のモジュールおよびコマンドに直接影響する。

| 対象 | 種別 | 影響内容 |
|------|------|----------|
| `module:config` | モジュール | `os.UserConfigDir()` による設定パス解決、sync_state.json・ロックファイルの配置 |
| `module:auth` | モジュール | OS別トークン保護（Keychain / Credential Manager / 0600パーミッション） |
| `cli_command:rscli` | CLIバイナリ | Linux / macOS / Windows 向けクロスコンパイルと配布 |

### リリースブロッキング制約

以下の2点はリリース必須要件であり、いずれか一方でも未達成の場合はリリース不可とする。

1. **`module:config` および `module:auth` のクロスプラットフォーム実装完了**: `os.UserConfigDir()` によるOS標準設定パスの使用、および各OSに応じたトークン保護機構（macOS Keychain / Windows Credential Manager / Linux ファイルパーミッション0600）が全て実装・テスト済みであること。
2. **`cli_command:rscli` の3 OS配布**: Linux・macOS・Windowsの全てに対してビルド済みバイナリがGitHub Releasesで配布されていること。未対応OSが1つでもあればリリース不可。

## 2. Decision Log

### ADR-CP-001: 設定ディレクトリの解決に `os.UserConfigDir()` を採用

- **ステータス**: 承認済み
- **日付**: 2026-05-23
- **コンテキスト**: `rscli` は `token.json`、`sync_state.json`、`lock` ファイルなど複数の設定・状態ファイルを永続化する。OS間でハードコードされたパスを使い分ける方式と、Go標準ライブラリの `os.UserConfigDir()` を使う方式が候補に挙がった。
- **決定**: Go標準ライブラリの `os.UserConfigDir()` を使用し、その戻り値配下に `remotestorage-cli/` サブディレクトリを作成する。
- **結果として各OSでの設定パス**:
  - **Linux**: `$XDG_CONFIG_HOME/remotestorage-cli/`（未設定時 `~/.config/remotestorage-cli/`）
  - **macOS**: `~/Library/Application Support/remotestorage-cli/`
  - **Windows**: `%AppData%\remotestorage-cli\`
- **根拠**: OS標準のパス慣習に自動で従うため、各OSのユーザーにとって予測可能な場所にファイルが配置される。条件分岐やビルドタグによるパス管理が不要になり、保守コストが下がる。
- **配置されるファイル一覧**:
  - `token.json` — OAuthトークン（Linux・macOSのファイルベース保存時のみ。Windowsでは不使用）
  - `sync_state.json` — 同期状態（ファイル毎のETag・mtime記録、atomic write）
  - `lock` — ロックファイル（PID記録、stale検出・`--force-unlock`対応）

### ADR-CP-002: OS別トークン保護方式

- **ステータス**: 承認済み
- **日付**: 2026-05-23
- **コンテキスト**: OAuth2トークン（AccessToken・RefreshToken）はセンシティブな認証情報であり、ファイルシステム上に平文で放置するとセキュリティリスクがある。各OSが提供するネイティブのシークレットストアを活用すべきか、統一的なファイルベース保存にすべきかが論点となった。
- **決定**: 各OSのネイティブ保護機構を最大限活用する。具体的には以下のとおり。

| OS | 保護方式 | 実装詳細 |
|----|----------|----------|
| **macOS** | macOS Keychain | `module:auth` がKeychain APIを通じてAccessToken・RefreshTokenを保存・取得する。`token.json` はメタデータ（スコープ・有効期限）のみ保持してもよいが、トークン本体はKeychain内に格納する。 |
| **Windows** | Windows Credential Manager | AccessToken・RefreshTokenをCredential Managerに保存する。`token.json` ファイルは作成しない。WSL環境はLinux扱いとする（後述ADR-CP-004）。 |
| **Linux** | ファイルパーミッション 0600 | `token.json` に全トークン情報を保存し、ファイル作成時にパーミッション `0600` を強制適用する。既存ファイルのパーミッションが `0600` より緩い場合は起動時に自動修正する。 |

- **根拠**: macOSとWindowsにはOSレベルの暗号化シークレットストアが存在し、ユーザーの認証情報管理の慣習にも合致する。Linuxにはデスクトップ環境依存のキーリング（GNOME Keyring、KDE Wallet等）が存在するが、サーバー環境やヘッドレス環境では利用できないため、最も確実な `0600` パーミッション方式を採用する。
- **`module:auth` の実装方針**: ビルドタグ（`//go:build darwin`、`//go:build windows`、`//go:build linux`）でOS別のトークンストア実装を切り替える。共通インターフェースとして `TokenStore` を定義し、`Save(token)` / `Load() token` / `Delete()` を各OS実装が満たす。

### ADR-CP-003: バイナリ配布方式

- **ステータス**: 承認済み
- **日付**: 2026-05-23
- **コンテキスト**: `rscli` はGo製CLIツールであり、`go install` によるソースからのインストールと、プリビルドバイナリ配布の両方が求められる。
- **決定**: 以下の2経路で配布する。
  1. **GitHub Releases**: `GOOS`/`GOARCH` の組み合わせでクロスコンパイルしたバイナリを各リリースに添付する。
  2. **`go install`**: `go install github.com/<org>/remotestorage-cli/cmd/rscli@latest` でインストール可能にする。
- **ビルドマトリクス**:

| OS | アーキテクチャ | バイナリ名 |
|----|---------------|-----------|
| Linux | amd64, arm64 | `rscli-linux-amd64`, `rscli-linux-arm64` |
| macOS | amd64, arm64 | `rscli-darwin-amd64`, `rscli-darwin-arm64` |
| Windows | amd64 | `rscli-windows-amd64.exe` |

- **根拠**: Linux arm64はRaspberry PiやARM系クラウドインスタンスで需要がある。macOS arm64はApple Siliconで必須。Windows arm64は現時点ではユーザー数が限定的であるため初期リリースでは対象外とし、需要に応じて追加する。
- **CI/CDパイプライン**: GitHub ActionsでGoのクロスコンパイル（`GOOS`/`GOARCH` 環境変数指定）を実行し、タグプッシュ時に自動リリースする。各OSのバイナリが全てビルド成功しない限りリリースジョブは失敗とする（3 OS対応のリリースブロッキング制約を機械的に保証）。

### ADR-CP-004: WSL環境の扱い

- **ステータス**: 承認済み
- **日付**: 2026-05-23
- **コンテキスト**: WSL（Windows Subsystem for Linux）上で `rscli` を実行した場合、ホストOSはWindowsだがランタイムはLinuxカーネルである。Windows Credential Managerへのアクセスは標準的には利用できない。
- **決定**: WSL環境はLinux扱いとする。トークン保護はファイルパーミッション `0600` 方式を適用する。
- **根拠**: WSL上のGoバイナリは `runtime.GOOS == "linux"` を返すため、ビルドタグによる自然な分岐でLinux実装が選択される。Windows Credential ManagerへのWSLからのブリッジは安定性・セキュリティの両面で課題があり、追加の依存関係を導入するリスクに見合わない。
- **影響**: WSLユーザーは `~/.config/remotestorage-cli/token.json`（パーミッション `0600`）にトークンが保存される。

### ADR-CP-005: パス区切り文字とエンコーディングの統一

- **ステータス**: 承認済み
- **日付**: 2026-05-23
- **コンテキスト**: remoteStorageプロトコルはパス区切りに `/` を使用する。Windowsのファイルシステムは `\` を使用する。また、日本語ファイル名や特殊文字を含むパスの扱いにOS間差異がある。
- **決定**:
  - remoteStorageとの通信パスは常に `/` を使用する。
  - ローカルファイルパスは `filepath` パッケージを使用してOS標準の区切り文字に変換する。
  - URLエンコーディングはCLI側で自動処理し、日本語・特殊文字は透過的に扱う。
  - シンボリックリンクおよび特殊ファイル（デバイスファイル、名前付きパイプ等）はOS問わずスキップし、warning を stderr に出力する。
- **根拠**: Go の `path/filepath` パッケージがOS間のパス変換を適切に処理する。プロトコル仕様（draft-dejong-remotestorage-22）がURLエンコーディングを前提としているため、CLI側で自動エンコードすることでユーザーの負担を排除する。

### ADR-CP-006: プログレス表示のTTY検出

- **ステータス**: 承認済み
- **日付**: 2026-05-23
- **コンテキスト**: プログレス表示は stderr に出力するが、Windows の `cmd.exe`、PowerShell、Windows Terminal、macOS/Linux のターミナルエミュレータで TTY 検出方式が異なる。
- **決定**: Go の `os.Stderr.Fd()` に対して `golang.org/x/term.IsTerminal()` を使用してTTY判定を行う。非TTY時（パイプ、リダイレクト、CI環境）はプログレス表示を自動抑制する。
- **根拠**: `golang.org/x/term` は Windows・macOS・Linux の全てで正確な TTY 判定を提供する Go 準標準ライブラリであり、OS別の条件分岐が不要になる。

### ADR-CP-007: ロックファイルのクロスプラットフォーム動作

- **ステータス**: 承認済み
- **日付**: 2026-05-23
- **コンテキスト**: ロックファイルにPIDを記録してstale検出を行う仕様があるが、PIDベースのプロセス存在確認はOS間で方式が異なる。
- **決定**:
  - ロックファイルは `{設定ディレクトリ}/lock` に配置する。
  - PID記録後のプロセス存在確認は `os.FindProcess()` + シグナル0送信（Unix系）および Windows API の `OpenProcess` で実装する。
  - staleなロックファイル（対象PIDのプロセスが存在しない）は自動削除する。
  - `--force-unlock` フラグで手動解除も可能とする。
- **根拠**: Go の `os.FindProcess` は全OSで利用可能だが、Unix系ではプロセス存在確認にシグナル0の送信が必要である一方、Windowsでは `OpenProcess` の成否で判定する。ビルドタグで分岐する。

## 3. Follow-ups

| ID | 内容 | 関連ADR | 優先度 | 完了条件 |
|----|------|---------|--------|----------|
| FU-CP-001 | macOS Keychain統合の実装とE2Eテスト（macOS CI runnerでの自動テスト含む） | ADR-CP-002 | 高（リリースブロッキング） | `module:auth` の macOS ビルドタグ実装が完了し、Keychainへのトークン保存・取得・削除が自動テストで検証済み |
| FU-CP-002 | Windows Credential Manager統合の実装とE2Eテスト | ADR-CP-002 | 高（リリースブロッキング） | `module:auth` の Windows ビルドタグ実装が完了し、Credential Managerへのトークン保存・取得・削除が自動テストで検証済み |
| FU-CP-003 | GitHub Actions CI/CDパイプラインにクロスコンパイルマトリクスを構成（Linux amd64/arm64, macOS amd64/arm64, Windows amd64） | ADR-CP-003 | 高（リリースブロッキング） | タグプッシュ時に5バイナリが自動ビルド・リリースされ、いずれか1つでも失敗した場合リリースジョブ全体が失敗すること |
| FU-CP-004 | Windows環境での日本語ファイル名を含むpush/pullのE2Eテスト | ADR-CP-005 | 中 | 日本語・絵文字・スペースを含むファイル名でのアップロード・ダウンロードが正常動作することを確認 |
| FU-CP-005 | Windows arm64ビルドの需要調査と追加判断 | ADR-CP-003 | 低 | 需要調査結果に基づき、追加する場合はビルドマトリクスを更新 |
| FU-CP-006 | Linux向けデスクトップキーリング（GNOME Keyring / KDE Wallet）統合の検討 | ADR-CP-002 | 低 | デスクトップLinuxユーザーからのフィードバックに基づき、D-Bus経由のキーリング統合を判断。ヘッドレス環境では引き続き0600フォールバックを維持する |
| FU-CP-007 | `module:config` のパス解決ユニットテスト（各OS向けビルドタグ別） | ADR-CP-001 | 高（リリースブロッキング） | `os.UserConfigDir()` の戻り値に基づくサブディレクトリ作成・ファイル配置が全3 OSで正しく動作することをテストで検証 |
