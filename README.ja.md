# glasp

[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/takihito/glasp/badge)](https://scorecard.dev/viewer/?uri=github.com/takihito/glasp)

Google Apps Script プロジェクトを管理するための Go 製 CLI ツール。Node.js ベースの [clasp](https://github.com/google/clasp) を置き換える、シングルバイナリの高速な代替ツールです。clasp との完全な互換性を備えています。

## 特徴

- clasp 完全互換（`.clasp.json`、`.claspignore`、`.clasprc.json`）
- push 時の TypeScript 自動トランスパイル（`fileExtension` 設定の有無を問わず動作）
- ローカルコールバックサーバーを使った OAuth2 認証
- コマンド実行履歴とリプレイ機能
- push/pull 操作のアーカイブ機能

## インストール

### クイックインストール（推奨）

**Linux / macOS:**

```bash
curl -sSL https://takihito.github.io/glasp/install.sh | sh
```

**Windows (PowerShell):**

```powershell
irm https://takihito.github.io/glasp/install.ps1 | iex
```

デフォルトで `~/.local/bin` にインストールされます。`sudo` は不要です。インストール先を変更する場合:

```bash
GLASP_INSTALL_DIR=/usr/local/bin curl -sSL https://takihito.github.io/glasp/install.sh | sh
```

### go install

```bash
go install github.com/takihito/glasp/cmd/glasp@latest
```

> この方法では OAuth 認証情報が含まれません。`GLASP_CLIENT_ID` と `GLASP_CLIENT_SECRET` 環境変数を設定してください。

### ソースからビルド

```bash
git clone https://github.com/takihito/glasp.git
cd glasp
make build    # bin/glasp にバイナリをビルド
make install  # ビルドしてグローバルにインストール
```

### OAuth 認証情報

| インストール方法 | 認証情報 |
|-----------------|---------|
| クイックインストール（ビルド済みバイナリ） | 埋め込み済み、環境変数で上書き可能 |
| `go install` / ソースビルド | 環境変数で指定が必要 |

```bash
export GLASP_CLIENT_ID="your-client-id"
export GLASP_CLIENT_SECRET="your-client-secret"
```

環境変数は埋め込み認証情報より優先されます。[Google Cloud Console](https://console.cloud.google.com/apis/credentials) からデスクトップアプリケーション用の OAuth 2.0 認証情報を作成してください。

## クイックスタート

```bash
# インストール（Linux / macOS）
curl -sSL https://takihito.github.io/glasp/install.sh | sh

# Google アカウントにログイン
glasp login

# または、clasp の認証情報がある場合:
glasp login --auth ~/.clasprc.json

# 既存のプロジェクトをクローン
glasp clone <script-id>

# リモートファイルを取得
glasp pull

# ローカルファイルをアップロード
glasp push
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

すでに clasp を使っている場合、`~/.clasprc.json` に OAuth 認証情報が保存されています。`--auth` オプションを使えば、このファイルを glasp でそのまま再利用できます。**対話型ログインフローは不要です。**

```bash
# clasp の認証情報を glasp にインポート（.glasp/access.json に保存）
glasp login --auth ~/.clasprc.json

# または、login せずに各コマンドで直接 --auth を指定
glasp push --auth ~/.clasprc.json
glasp pull --auth ~/.clasprc.json
glasp clone SCRIPT_ID --auth ~/.clasprc.json

# ディレクトリを指定することも可能 — glasp がディレクトリ内の .clasprc.json を自動的に探します
glasp push --auth ~/
```

`glasp login --auth` は `.clasprc.json` の認証情報を glasp のプロジェクトキャッシュ（`.glasp/access.json`）にインポートします。以降のコマンドでは `--auth` の指定なしで動作します。インポートせずに使いたい場合は、各コマンドに `--auth` を直接指定してください。

以下のようなケースで特に便利です：

- **clasp から glasp への移行時** — 再認証なしですぐに glasp を使い始められます
- **CI/CD パイプライン** — clasp と glasp のワークフローで単一の `.clasprc.json` を共有できます
- **複数の Google アカウント** — アカウントごとに別々の `.clasprc.json` を用意し、`--auth` で切り替えられます

`--auth` オプションは認証が必要なすべてのコマンドで利用できます：`login`、`push`、`pull`、`clone`、`create-script`、`create-deployment`、`update-deployment`、`list-deployments`、`run-function`。

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
