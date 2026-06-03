---
codd:
  node_id: governance:adr-protocol-and-auth
  type: governance
  depends_on:
  - id: req:remotestorage-cli-requirements
    relation: derives_from
    semantic: governance
  depended_by:
  - id: design:system-design
    relation: constrained_by
    semantic: governance
  - id: design:auth-design
    relation: constrained_by
    semantic: governance
  conventions:
  - targets:
    - module:remotestorage
    reason: draft-dejong-remotestorage-22準拠必須。プロトコルバージョン逸脱はリリース不可。
  - targets:
    - module:auth
    reason: PKCE(RFC 7636)必須、RFC 7591動的クライアント登録優先はプロトコル準拠要件。
  modules:
  - auth
  - remotestorage
---

# ADR: プロトコルバージョンとOAuth2方式の決定

## 1. Overview

本ADRは、remotestorage-cliにおけるremoteStorageプロトコルバージョンの選定およびOAuth2認証方式の決定を記録する。対象モジュールは `module:remotestorage` および `module:auth` であり、以下の2つのリリース不可制約に直結する。

**制約1（module:remotestorage）**: draft-dejong-remotestorage-22準拠必須。プロトコルバージョン逸脱はリリース不可。remotestorage-cliが実装するすべてのHTTPリクエスト・レスポンス処理、WebFinger発見、ETagベースの変更検出、ディレクトリリスティング形式はdraft-22の仕様に厳密に従う。draft-22以外のプロトコルバージョンとの互換性は本リリースのスコープ外とする。

**制約2（module:auth）**: PKCE（RFC 7636）必須、RFC 7591動的クライアント登録優先はプロトコル準拠要件。OAuth2フローにおいてPKCEを省略した実装はリリース不可。クライアント登録はRFC 7591を第一手段とし、非対応サーバーに対してのみフォールバックを提供する。

本ADRで決定する事項は以下の3領域にわたる。

1. **プロトコルバージョン固定**: draft-dejong-remotestorage-22を唯一の対象バージョンとする根拠
2. **OAuth2認証フロー設計**: Authorization Code + PKCE + localhostリダイレクト方式の採用根拠
3. **クライアント登録戦略**: RFC 7591動的登録を優先し、インタラクティブフォールバックを提供する方針の根拠

## 2. Decision Log

### Decision 2-1: プロトコルバージョンをdraft-dejong-remotestorage-22に固定する

- **ステータス**: 承認済み
- **日付**: 2026-05-23
- **コンテキスト**: remoteStorageプロトコルには複数のドラフトバージョンが存在する。各バージョン間でWebFinger応答形式、認証エンドポイント発見方法、ディレクトリリスティングのJSON構造、ETagの扱いに差異がある。複数バージョンへの対応は実装・テストコストを増大させ、エッジケースにおけるバグリスクを高める。
- **決定**: draft-dejong-remotestorage-22を唯一の対象バージョンとする。WebFinger応答からストレージURL・認証エンドポイントを取得する際のプロパティ名、ディレクトリリスティングのレスポンス形式、ETag付きの条件付きリクエスト（If-Match / If-None-Match）はすべてdraft-22の定義に準拠する。
- **根拠**:
  - draft-22は2026年5月時点で最新のドラフトであり、主要なremoteStorageサーバー実装（php-remote-storage、armadietto、remotestorage-server）が対応済みまたは対応進行中である。
  - CLIツールのターゲットユーザーはremoteStorageを使っている開発者・個人ユーザーであり、レガシーサーバーへの後方互換性より最新仕様への準拠を優先する方が利用者の期待に合致する。
  - プロトコルバージョン差異の吸収レイヤーを設けると、テストマトリクスが指数的に拡大する。単一バージョン固定により、ETag変更検出・ディレクトリリスティング・Content-Type自動検出・削除伝播といった機能を確定的にテスト可能となる。
- **影響**: draft-22より古いバージョンのみをサポートするサーバーには接続できない。WebFinger応答がdraft-22形式に合致しない場合、`rscli connect` はエラー終了（終了コード4）する。
- **制約への準拠**: `module:remotestorage` — draft-dejong-remotestorage-22準拠必須の制約を充足。プロトコルバージョン逸脱をコード上で排除するため、WebFinger応答のバージョンフィールド検証を接続時に実施する。

### Decision 2-2: OAuth2 Authorization Code + PKCE + localhostリダイレクト方式を採用する

- **ステータス**: 承認済み
- **日付**: 2026-05-23
- **コンテキスト**: remoteStorageプロトコルはOAuth2による認可を前提とする。CLIツールにおけるOAuth2フローには複数の選択肢がある: (a) Authorization Code + localhostリダイレクト、(b) Device Authorization Grant（RFC 8628）、(c) Resource Owner Password Credentials。CLIツールでは秘密鍵を安全に保持できないため、パブリッククライアントとして動作する必要がある。
- **決定**: OAuth2 Authorization Code Grant + PKCE（RFC 7636）をlocalhostリダイレクトで実装する。具体的な設計は以下のとおり。

  | 項目 | 仕様 |
  |------|------|
  | グラントタイプ | Authorization Code |
  | PKCE | 必須。code_challenge_method は S256 |
  | リダイレクトURI | `http://localhost:{動的ポート}/callback` |
  | ポート選択 | OS任意ポート割当（port 0 → 実際のポートを取得） |
  | デフォルトスコープ | `*:rw` （`--scope` オプションで変更可能） |
  | client_secret | 不要（パブリッククライアント） |
  | トークン保存先 | `~/.config/remotestorage-cli/token.json`（Linux/WSL: パーミッション0600、macOS: Keychain、Windows: Credential Manager） |
  | refresh token | 利用可能な場合は自動更新。更新失敗時はブラウザ再認証へフォールバック |
  | 非インタラクティブ環境 | `--no-interactive` フラグ指定時はトークンが事前存在する場合のみ動作。トークン不在または期限切れ時はエラー終了（終了コード2） |

- **根拠**:
  - **PKCE必須の理由**: パブリッククライアントではauthorization codeの横取り攻撃（RFC 7636 Section 1で定義）が現実的な脅威となる。PKCEはcode_verifierとcode_challengeのペアにより、認可コードを取得した攻撃者がトークン交換を完了できないようにする。RFC 7636準拠は `module:auth` のリリース不可制約であり、省略は許容されない。
  - **localhostリダイレクトの理由**: CLIツールがローカルHTTPサーバーを一時的に起動してリダイレクトを受け取る方式は、ユーザーがコードを手動コピーする必要がなく、UXが優れる。ポートを動的に割り当てることで、ポート競合を回避する。
  - **Device Authorization Grant不採用の理由**: remoteStorageサーバー側でRFC 8628対応が普及していないため、接続可能なサーバーが大幅に制限される。
  - **Resource Owner Password Credentials不採用の理由**: OAuth2 Security Best Current Practice（RFC 6819）で非推奨。CLIがユーザーのパスワードを直接取り扱うことはセキュリティリスクとなる。
- **セキュリティ制御**:
  - token.jsonのファイルパーミッションは0600を強制（Linux/WSL）。書き込み時にパーミッションを明示的に設定し、既存ファイルが緩いパーミッションである場合は修正する。
  - macOSではKeychain Services APIを使用し、AccessToken・RefreshTokenをKeychain項目として保存する。token.jsonファイルは生成しない。
  - WindowsではCredential Manager APIを使用し、AccessToken・RefreshTokenをGeneric Credentialとして保存する。token.jsonファイルは生成しない。WSL環境はLinux扱い（ファイルパーミッション0600）とする。
  - HTTPS必須。`--insecure` フラグで自己署名証明書を許可するが、開発・テスト環境向けに限定される。
- **制約への準拠**: `module:auth` — PKCE（RFC 7636）必須の制約を充足。code_challenge_method = S256を固定とし、PKCEパラメータなしの認可リクエストは送信しない設計とする。

### Decision 2-3: RFC 7591動的クライアント登録を優先し、フォールバックを提供する

- **ステータス**: 承認済み
- **日付**: 2026-05-23
- **コンテキスト**: OAuth2ではクライアント（本ツール）がauthorization serverに登録されている必要がある。CLIツールを不特定のremoteStorageサーバーに接続する場合、事前のクライアント登録は非現実的である。RFC 7591は動的クライアント登録プロトコルを定義しており、クライアントが実行時に自身をauthorization serverに登録できる。ただし、すべてのremoteStorageサーバーがRFC 7591をサポートしているわけではない。
- **決定**: `rscli connect user@host` 実行時のクライアント登録は以下の優先順位で試行する。

  1. **RFC 7591動的クライアント登録**（第一選択）: WebFinger応答から取得したauthorization server情報をもとに、registration endpointへPOSTリクエストを送信する。登録成功時はサーバーから返却されたclient_idを使用する。redirect_urisには`http://localhost/callback`を指定する（ポート番号は認可リクエスト時に確定するため、登録時は省略またはワイルドカード対応）。
  2. **インタラクティブフォールバック**（RFC 7591非対応時）: registration endpointが存在しない、または登録リクエストが失敗した場合、ユーザーにclient_idの手動入力を求めるインタラクティブプロンプトを表示する。client_secretは不要。`--no-interactive` フラグ指定時はエラー終了（終了コード2）。
  3. 取得したclient_idはtoken.jsonと同じ保存先（OS別のセキュアストレージ）に永続化し、同一サーバーへの再接続時に再利用する。

- **根拠**:
  - RFC 7591動的登録は、ユーザーがサーバー管理画面でclient_idを取得する手間を排除し、`rscli connect` コマンドの実行だけで認証フローを完結させる。
  - 動的登録非対応サーバーが存在する現実を踏まえ、フォールバックを提供することで接続可能なサーバー範囲を最大化する。
  - client_secretを不要とすることで、パブリッククライアントとしての一貫性を維持し、シークレット漏洩リスクを排除する。
- **制約への準拠**: `module:auth` — RFC 7591動的クライアント登録優先の制約を充足。実装は動的登録を第一手段とし、フォールバックはRFC 7591非対応が確認された場合にのみ発動する。

### Decision 2-4: WebFingerによるサーバー発見とフォールバック

- **ステータス**: 承認済み
- **日付**: 2026-05-23
- **コンテキスト**: remoteStorageプロトコル（draft-22）はWebFinger（RFC 7033）によるサーバー発見を規定している。`user@host` 形式の入力からストレージURL、認証エンドポイント、プロトコルバージョンを取得する。ただし、WebFingerが利用できない環境（ファイアウォール、DNS設定、サーバー設定不備）への対応が必要となる。
- **決定**: `rscli connect user@host` の発見フローは以下のとおり。

  1. `https://{host}/.well-known/webfinger?resource=acct:{user}@{host}&rel=http://tools.ietf.org/id/draft-dejong-remotestorage` へGETリクエストを送信する。
  2. レスポンスのJRD（JSON Resource Descriptor）からstorage URL（`href`）およびOAuth2認証エンドポイント（`properties`内の`http://tools.ietf.org/id/draft-dejong-remotestorage#auth`）を抽出する。
  3. WebFingerリクエストが失敗した場合（ネットワークエラー、404、タイムアウト等）、ストレージURLを直接入力するフォールバックプロンプトを表示する。
  4. HTTPタイムアウトは接続30秒・アイドル60秒を適用する。

- **根拠**: draft-22がWebFingerをサーバー発見の標準手段として規定しており、準拠のため第一手段とする。フォールバックは要件定義で明示されている。

### Decision 2-5: トークンライフサイクル管理

- **ステータス**: 承認済み
- **日付**: 2026-05-23
- **コンテキスト**: OAuth2トークンには有効期限があり、CLIツールとして適切なライフサイクル管理が求められる。ユーザー体験を損なわずにセキュリティを維持する設計が必要である。
- **決定**: トークンライフサイクルは以下のとおり管理する。

  | イベント | 動作 |
  |----------|------|
  | 初回認証（`rscli connect`） | ブラウザを開いてOAuth2フローを実行。取得したaccess_token・refresh_tokenをOS別セキュアストレージに保存 |
  | トークン期限切れ（APIリクエスト時） | refresh_tokenが存在する場合は自動更新を試行 |
  | refresh token更新成功 | 新しいaccess_token（およびrefresh_token）を保存して処理を継続 |
  | refresh token更新失敗 | インタラクティブモードではブラウザ再認証へ移行。`--no-interactive` 時はエラー終了（終了コード2） |
  | `rscli connect` 再実行（既存トークンあり） | 確認プロンプトを表示後に上書き。`--yes` フラグで確認スキップ |
  | `rscli disconnect` | OS別セキュアストレージからトークンを削除。token.jsonが存在する場合は削除 |

- **根拠**: refresh tokenの自動更新により、ユーザーは通常の操作でブラウザ認証を繰り返す必要がない。更新失敗時のフォールバックにより、サーバー側でrefresh tokenが無効化された場合にも対応できる。

## 3. Follow-ups

### Follow-up 3-1: プロトコルバージョンネゴシエーション（将来検討）

draft-22以外のバージョンをサポートするremoteStorageサーバーが増加した場合、WebFinger応答のバージョン情報に基づくプロトコルバージョンネゴシエーション層の導入を検討する。現時点ではdraft-22固定とし、非対応サーバーへの接続時は明確なエラーメッセージ（「このサーバーはdraft-dejong-remotestorage-22に対応していません」）を表示する。判断時期: 主要サーバー実装がdraft-22以降のドラフトをリリースした時点。

### Follow-up 3-2: RFC 7591登録パラメータの最適化

動的クライアント登録時に送信するメタデータ（client_name、client_uri、logo_uri等）の内容を、主要remoteStorageサーバー実装での動作検証を経て確定する。現段階ではclient_name = `remotestorage-cli`、redirect_uris = `["http://localhost/callback"]`、token_endpoint_auth_method = `none` を送信する最小構成とする。

### Follow-up 3-3: 複数アカウント対応

現在のスコープでは1アカウントのみをサポートする。将来的に複数アカウントを管理する場合、トークン保存構造をアカウント別に分離する設計変更が必要となる。具体的にはtoken.jsonをアカウント識別子（`user@host`）でキー付きの構造に拡張するか、アカウント別ファイル（`token_{host}_{user}.json`）に分割するかを検討する。

### Follow-up 3-4: PKCE code_challenge_methodの拡張

現在はS256のみをサポートする。authorization serverがS256を非サポートの場合、plain methodへのフォールバックを追加するか判断する。RFC 7636はS256を推奨しており、plainへのフォールバックはセキュリティリスクを伴うため、原則として追加しない方針とする。S256非対応サーバーに遭遇した場合はエラーメッセージで通知する。

### Follow-up 3-5: OAuth2セキュリティベストプラクティスの継続追従

OAuth 2.1（draft-ietf-oauth-v2-1）の正式RFC化に伴い、本ADRの決定事項がOAuth 2.1の要件を満たしているか再検証する。現時点の設計（Authorization Code + PKCE必須、client_secret不要、Implicit Grant不使用）はOAuth 2.1ドラフトの方向性と整合しているが、正式化時に差分がないか確認する。
