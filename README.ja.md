# glasp

Google Apps Script プロジェクトを管理するための Go 製 CLI ツール。Node.js ベースの [clasp](https://github.com/google/clasp) を置き換える、シングルバイナリの高速な代替ツールです。clasp との完全な互換性を備えています。

## 特徴

- clasp 完全互換（`.clasp.json`、`.claspignore`、`.clasprc.json`）
- push 時の TypeScript 自動トランスパイル（`fileExtension` 設定の有無を問わず動作）
- ローカルコールバックサーバーを使った OAuth2 認証
- コマンド実行履歴とリプレイ機能
- push/pull 操作のアーカイブ機能

## インストール

### go install

```bash
go install github.com/takihito/glasp/cmd/glasp@latest
```

### ビルド済みバイナリ

[Releases](https://github.com/takihito/glasp/releases) ページからダウンロード:

```bash
VERSION=0.1.0
OS=${OS:-Darwin}     # Darwin, Linux, Windows
ARCH=${ARCH:-arm64}  # arm64, amd64
ARTIFACT="glasp_v${VERSION}_${OS}_${ARCH}.tar.gz"

curl -L -o "${ARTIFACT}" \
  "https://github.com/takihito/glasp/releases/download/v${VERSION}/${ARTIFACT}"

# チェックサム検証
echo "SHA256_FROM_RELEASE  ${ARTIFACT}" | shasum -a 256 --check

# インストール
sudo tar -xzf "${ARTIFACT}" -C /usr/local/bin glasp
```

### ソースからビルド

```bash
git clone https://github.com/takihito/glasp.git
cd glasp
make build    # bin/glasp にバイナリをビルド
make install  # ビルドしてグローバルにインストール
```

OAuth 認証情報（`GLASP_CLIENT_ID`、`GLASP_CLIENT_SECRET`）はビルド時に `.env` から `-ldflags` で注入されます。

## クイックスタート

```bash
# Google アカウントにログイン
glasp login

# 既存のプロジェクトをクローン
glasp clone <script-id>

# リモートファイルを取得
glasp pull

# ローカルファイルをアップロード
glasp push

# 新しいプロジェクトを作成
glasp create-script --title "My Project"
```

## コマンド一覧

| コマンド | エイリアス | 説明 |
|---------|-----------|------|
| `login` | | Google アカウントにログイン |
| `logout` | | Google アカウントからログアウト |
| `create-script` | `create` | 新しい Apps Script プロジェクトを作成 |
| `clone` | | 既存の Apps Script プロジェクトをクローン |
| `pull` | | Apps Script からプロジェクトファイルをダウンロード |
| `push` | | Apps Script にプロジェクトファイルをアップロード |
| `open-script` | `open` | Apps Script プロジェクトをブラウザで開く |
| `create-deployment` | | デプロイメントを作成 |
| `update-deployment` | `deploy` | 既存のデプロイメントを更新 |
| `list-deployments` | | スクリプトプロジェクトのデプロイメント一覧を表示 |
| `run-function` | | Apps Script の関数をリモート実行 |
| `convert` | | プロジェクトファイルを変換（TS <-> GAS） |
| `history` | | コマンド実行履歴を表示 |
| `config init` | | `.glasp/config.json` を作成 |
| `version` | | glasp のバージョンを表示 |

## 設定

### .clasp.json

clasp 標準の設定ファイル。glasp は同じフォーマットを読み取ります：

```json
{
  "scriptId": "your-script-id",
  "rootDir": "src",
  "fileExtension": "ts"
}
```

### .claspignore

gitignore 構文を使った clasp 標準の除外ファイル。glasp は `.claspignore` がなくても以下をデフォルトで除外します：

- `.glasp/` — glasp 内部ディレクトリ（`--force` 使用時も常に除外）
- `node_modules/` — npm 依存パッケージ

### .glasp/

glasp 固有のディレクトリ（自動作成、push 対象から除外）：

| ファイル | 説明 |
|---------|------|
| `access.json` | OAuth トークンキャッシュ（パーミッション 0600） |
| `config.json` | glasp 固有の設定（アーカイブ設定等） |
| `history.jsonl` | コマンド実行履歴（JSON Lines 形式） |
| `archive/` | push/pull のアーカイブ |

`.glasp/` ディレクトリはパーミッション `0700` で作成されます。初回作成時に glasp が自動的に `.claspignore` に `.glasp/` を追加するため、clasp が glasp の内部ファイルを push することを防ぎます。

## Push

```bash
glasp push              # ローカルファイルを push
glasp push --force      # .claspignore を無視（ただし .glasp/ は常に除外）
glasp push --archive    # push したファイルをアーカイブ
glasp push --dryrun     # API 呼び出しなしのドライラン
glasp push --auth path  # 指定した .clasprc.json で認証
```

### TypeScript サポート

glasp は `.clasp.json` の `fileExtension` 設定の有無に関わらず、push 時に `.ts` ファイルを自動検出してトランスパイルします。これは clasp v2.4.2 と同じ挙動です。

- `.ts` ファイルは esbuild で JavaScript に変換してから push
- `.js` と `.gs` ファイルはそのまま push
- `.d.ts` ファイルは除外（型定義ファイルはデプロイ不要）
- `.ts` + `.js` 混在プロジェクトに対応（`fileExtension` が `"js"` でも `.ts` は常に収集される）
- 収集する拡張子をカスタマイズするには `.clasp.json` の `scriptExtensions` を使用

### 履歴からのリプレイ

```bash
glasp push --history-id <id>          # アーカイブされたペイロードから再 push
glasp push --history-id <id> --dryrun # リプレイのドライラン
```

## Pull

```bash
glasp pull              # リモートファイルを pull
glasp pull --archive    # pull したファイルをアーカイブ
```

`.clasp.json` で `fileExtension` を `"ts"` に設定している場合、pull したファイルは GAS JavaScript から TypeScript に自動変換されます。

## 履歴

```bash
glasp history                          # 全履歴を表示
glasp history --limit 10               # 直近 10 件
glasp history --status success         # ステータスで絞り込み（all|success|error）
glasp history --command push           # コマンド名で絞り込み
glasp history --order asc              # 古い順（デフォルト: desc）
```

出力形式は JSON 配列（エントリがない場合は `[]`）。

## Convert

```bash
glasp convert --gas-to-ts              # GAS JavaScript を TypeScript に変換
glasp convert --ts-to-gas              # TypeScript を GAS JavaScript に変換
glasp convert --ts-to-gas src/main.ts  # 特定のファイルを変換
```

## 認証

認証トークンは `.glasp/access.json` にパーミッション `0600` で保存されます。認証ソースの優先順位：

1. `--auth` フラグ（`.clasprc.json` のパスを指定）
2. プロジェクトキャッシュ（`.glasp/access.json`）
3. 対話型ログインフロー

### `--auth` で `.clasprc.json` を利用する

すでに clasp を使っている場合、`~/.clasprc.json` に OAuth 認証情報が保存されています。`--auth` オプションを使えば、このファイルを glasp でそのまま再利用できます。**`glasp login` を実行する必要はありません。**

```bash
# 既存の clasp 認証情報を使用
glasp push --auth ~/.clasprc.json
glasp pull --auth ~/.clasprc.json
glasp clone SCRIPT_ID --auth ~/.clasprc.json

# ディレクトリを指定することも可能 — glasp がディレクトリ内の .clasprc.json を自動的に探します
glasp push --auth ~/
```

以下のようなケースで特に便利です：

- **clasp から glasp への移行時** — 再認証なしですぐに glasp を使い始められます
- **CI/CD パイプライン** — clasp と glasp のワークフローで単一の `.clasprc.json` を共有できます
- **複数の Google アカウント** — アカウントごとに別々の `.clasprc.json` を用意し、`--auth` で切り替えられます

`--auth` オプションは認証が必要なすべてのコマンドで利用できます：`push`、`pull`、`clone`、`create-script`、`create-deployment`、`update-deployment`、`list-deployments`、`run-function`。

#### 対応する `.clasprc.json` のフォーマット

glasp は clasp が生成する `.clasprc.json` と同じフォーマットを読み取ります。以下の認証情報レイアウトに対応しています：

```json
{
  "oauth2ClientSettings": {
    "clientId": "...",
    "clientSecret": "..."
  },
  "token": {
    "access_token": "...",
    "refresh_token": "...",
    "token_type": "Bearer",
    "expiry_date": 1234567890000
  }
}
```

Google Cloud Console からダウンロードした `installed` および `web` 形式の認証情報にも対応しています。

#### トークンの自動更新

`--auth` で指定したファイルに `refresh_token` と OAuth クライアント認証情報が含まれている場合、glasp はアクセストークンを自動的に更新し、更新後のトークンをファイルに書き戻します。手動での更新操作なしに、セッションをまたいで認証情報を有効に保ちます。

## 開発

```bash
make build              # bin/glasp にバイナリをビルド
make install            # ビルドしてグローバルにインストール
make test               # 全テストを実行
make clean              # バイナリ削除とテストキャッシュクリア

go test ./internal/auth/...          # 特定パッケージのテスト
go test ./cmd/glasp -run TestName    # 単一テストの実行
go test -v ./...                     # 全テストの詳細表示
```
