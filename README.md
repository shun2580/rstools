# rscli

remoteStorage プロトコル（[draft-dejong-remotestorage-22](https://datatracker.ietf.org/doc/html/draft-dejong-remotestorage-22)）を操作する Go 製 CLI ツール。

WebFinger によるサーバー自動検出・OAuth2 + PKCE 認証・ファイル操作・双方向同期をサポートする。

## インストール

### go install

```bash
go install github.com/shun2580/rstools/cmd/rscli@latest
```

### バイナリダウンロード（GitHub Releases）

[Releases ページ](https://github.com/shun2580/rstools/releases/latest)から OS に合ったバイナリをダウンロードして PATH に配置する。

| ファイル名 | OS / アーキテクチャ |
|---|---|
| `rscli-linux-amd64` | Linux (x86_64) |
| `rscli-linux-arm64` | Linux (ARM64) |
| `rscli-darwin-amd64` | macOS (Intel) |
| `rscli-darwin-arm64` | macOS (Apple Silicon) |
| `rscli-windows-amd64.exe` | Windows (x86_64) |

```bash
# Linux の例
curl -L https://github.com/shun2580/rstools/releases/latest/download/rscli-linux-amd64 -o rscli
chmod +x rscli
sudo mv rscli /usr/local/bin/
```

## コマンド一覧

### 認証

```bash
# remoteStorage サーバーに接続（WebFinger 自動検出 → ブラウザで OAuth2 認証）
rscli connect user@example.com

# ログアウト（ローカルのトークンを削除）
rscli disconnect
```

### ファイル操作

```bash
# ファイル一覧表示
rscli ls /

# ディレクトリを再帰的に表示
rscli ls -R /docs/

# JSON 形式で出力
rscli ls --json /

# ファイルをダウンロード
rscli get /remote/file.txt ./local/file.txt

# ファイルをアップロード
rscli put ./local/file.txt /remote/file.txt

# Content-Type を明示指定してアップロード
rscli put --content-type text/markdown ./note.md /note.md

# ファイルを削除
rscli rm /remote/file.txt

# ディレクトリを再帰削除（確認プロンプトあり）
rscli rm -r /remote/old-dir/
```

### 同期

```bash
# ローカル → リモートへ同期（push）
rscli push ./local-dir/ /remote/backup/

# リモート → ローカルへ同期（pull）
rscli pull /remote/backup/ ./local-dir/
```

#### 同期オプション

| フラグ | 説明 |
|---|---|
| `--dry-run` | 実行内容を表示するだけで変更しない |
| `--no-delete` | 削除の伝播を無効にする |
| `--force` | コンフリクト時に強制上書きする |
| `--parallel N` | 並列転送数（デフォルト: 3） |
| `--exclude pattern` | 除外パターン（複数指定可） |
| `--reset-state` | 同期状態をリセットして初回同期扱いにする |
| `--force-unlock` | ロックファイルを強制解除する |

#### 除外パターン（`.rsignore`）

sync ルートに `.rsignore` ファイルを置くことで除外パターンを指定できる（`.gitignore` と同じ書式）。

```
# .rsignore の例
*.log
.DS_Store
node_modules/
tmp/
```

### グローバルオプション

| フラグ | 説明 |
|---|---|
| `-v`, `--verbose` | 詳細ログを出力する |
| `--no-interactive` | 対話プロンプトを無効にする（CI 用） |
| `--insecure` | TLS 証明書の検証をスキップする（開発・テスト環境用） |

## 同期の仕組み

### 変更検出

| 対象 | 検出方法 |
|---|---|
| ローカルファイル | `mtime`（更新日時） |
| リモートファイル | `ETag`（非対応サーバーは `Last-Modified` にフォールバック） |

同期状態は `~/.config/remotestorage-cli/sync_state.json` に保存される。中断後の再実行でも完了済みファイルをスキップして再開できる。

### コンフリクト

ローカルとリモートの両方が変更されている場合、デフォルトではスキップして警告を表示する。`--force` で強制上書きできる。

### 初回同期の挙動

| 操作 | ローカルのみ存在 | リモートのみ存在 |
|---|---|---|
| `push` | アップロード | スキップ（削除しない） |
| `pull` | スキップ（削除しない） | ダウンロード |

## ローカルテスト環境

実際の remoteStorage サーバーなしで動作確認できるテストサーバーを同梱している。

### 起動

```bash
# テストサーバーを Docker で起動（ポート 8443）
docker compose -f docker-compose.test.yml up -d
```

### 接続・操作

```bash
# 接続（ブラウザが開くが即座にリダイレクトされる）
rscli connect --insecure testuser@localhost:8443

# ファイルをアップロード
rscli put --insecure ./README.md /README.md

# 一覧表示
rscli ls --insecure /

# ダウンロード
rscli get --insecure /README.md ./downloaded.md

# 同期（push）
rscli push --insecure ./testdata/ /testdata/

# 同期（pull）
rscli pull --insecure /testdata/ ./pulled/
```

> **注意**
> - `--insecure` は毎回必要（テストサーバーは自己署名証明書を使用）
> - データはコンテナ再起動でリセット（インメモリ）
> - `connect` 時にブラウザが開くが、即座にリダイレクトされるため操作不要

### 停止

```bash
docker compose -f docker-compose.test.yml down
```

## 設定ファイルの場所

| OS | パス |
|---|---|
| Linux / WSL | `~/.config/remotestorage-cli/` |
| macOS | `~/Library/Application Support/remotestorage-cli/` |
| Windows | `%AppData%\remotestorage-cli\` |

| ファイル | 用途 |
|---|---|
| `token.json` | OAuth2 トークン（Linux/WSL のみ） |
| `sync_state.json` | 同期状態（ファイルごとの ETag / mtime） |
| `lock` | 同時実行防止ロックファイル |

## 開発

### ビルド

```bash
go build -o bin/rscli ./cmd/rscli/
```

### CI

`main` ブランチへの push および Pull Request で自動実行される。

```
go vet → go build → go test -race → クロスコンパイル確認（5プラットフォーム）
```

### リリース

バージョンタグを push するとバイナリが自動ビルド・公開される。

```bash
git tag v0.2.0
git push origin v0.2.0
```

GitHub Actions が以下を自動実行する:
1. 5プラットフォーム向けバイナリをビルド（`CGO_ENABLED=0` 静的バイナリ）
2. `sha256` チェックサムを生成
3. GitHub Release を作成してバイナリを添付

## アーキテクチャ

```
cmd/rscli/          エントリポイント（cobra CLIルート）
internal/
  auth/             認証モジュール
    webfinger.go      WebFinger 発見（RFC 7033）
    pkce.go           PKCE コード生成（RFC 7636, S256 固定）
    oauth2.go         Authorization Code フロー（ブラウザ + ローカルHTTPサーバー）
    registration.go   RFC 7591 動的クライアント登録
    refresh.go        トークン自動更新
    tokenstore*.go    OS別トークン保護（Linux:0600, macOS/Windows:stub）
  remotestorage/    プロトコル実装
    client.go         HTTPS強制HTTPクライアント
    directory.go      ディレクトリ一覧（draft-22 JSON形式）
    path.go           URLパスエンコーディング
    walk.go           ディレクトリ再帰走査
  sync/             同期エンジン
    engine.go         push/pull ロジック（並列ワーカープール）
    state.go          sync_state.json（アトミック書き込み）
    ignore.go         .rsignore パターンマッチ
  transfer/         ファイル転送
    retry.go          指数バックオフリトライ（429/500/502/503）
    contenttype.go    Content-Type 自動検出
    progress.go       進捗表示（TTY判定）
  config/           設定・ロック管理
    dir.go            設定ディレクトリパス（os.UserConfigDir）
    lock*.go          PIDロックファイル（Unix: signal 0, Windows: OpenProcess）
  cli/              コマンド定義
testserver/         ローカルテスト用 remoteStorage サーバー
```

## ライセンス

MIT
