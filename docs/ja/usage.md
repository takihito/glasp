---
layout: default
title: 使い方
---

# 使い方

## コマンド一覧

| コマンド | エイリアス | 説明 |
|---------|-----------|------|
| `login` | | Google アカウントにログイン |
| `logout` | | Google アカウントからログアウト |
| `create-script` | `create` | 新しい Apps Script プロジェクトを作成 |
| `clone` | | 既存の Apps Script プロジェクトをクローン |
| `pull` | | Apps Script からファイルをダウンロード |
| `push` | | ファイルを Apps Script にアップロード |
| `open-script` | `open` | Apps Script プロジェクトをブラウザで開く |
| `create-deployment` | | デプロイメントを作成 |
| `update-deployment` | `deploy` | 既存のデプロイメントを更新 |
| `list-deployments` | | デプロイメント一覧を表示 |
| `run-function` | | Apps Script 関数をリモート実行 |
| `convert` | | プロジェクトファイルを変換 (TS ↔ GAS) |
| `history` | | コマンド実行履歴を表示 |
| `config init` | | `.glasp/config.json` を作成 |
| `version` | | バージョンを表示 |

## Push

```bash
glasp push              # ローカルファイルをプッシュ
glasp push --force      # .claspignore を無視（.glasp/ は常に除外）
glasp push --archive    # プッシュしたファイルをアーカイブ
glasp push --dryrun     # API 呼び出しなしのドライラン
```

### TypeScript サポート

glasp は push 時に `.ts` ファイルを自動検出してトランスパイルします（`.clasp.json` の `fileExtension` 設定に関係なく）。

- `.ts` → esbuild で JavaScript に変換してプッシュ
- `.js`, `.gs` → そのままプッシュ
- `.d.ts` → 除外（デプロイ不可）

### 履歴からのリプレイ

```bash
glasp push --history-id <id>          # アーカイブから再プッシュ
glasp push --history-id <id> --dryrun # ドライランでリプレイ
```

## Pull

```bash
glasp pull              # リモートファイルを取得
glasp pull --archive    # 取得したファイルをアーカイブ
```

`fileExtension` が `"ts"` の場合、取得時に GAS JavaScript を TypeScript に自動変換します。

## 認証

認証トークンは `.glasp/access.json` に保存されます（パーミッション `0600`）。

```bash
# ブラウザが自動で開いてログインフローが開始されます
glasp login 
```

### PKCE

OAuth2 PKCE（RFC 7636）をオプトインで有効化して、認可コードの横取り攻撃を防ぐことができます：

```bash
glasp login --pkce

# または環境変数で有効化
GLASP_USE_PKCE=1 glasp login
```

PKCE は `glasp login` フローにのみ適用されます。

### clasp の認証情報を再利用

```bash
# clasp の認証情報を再利用します。`glasp login` コマンドでインポートされ、`.glasp/access.json` に保存されます。
glasp login --auth ~/.clasprc.json

# または、各コマンドで直接 --auth を指定
glasp push --auth ~/.clasprc.json
glasp pull --auth ~/.clasprc.json
glasp clone SCRIPT_ID --auth ~/.clasprc.json
```

clasp から glasp への移行時に、再認証なしですぐに使い始められます。

### 認証ソースの優先順位:

1. `--auth` フラグ（`.clasprc.json` のパス）
2. プロジェクトキャッシュ（`.glasp/access.json`）
3. インタラクティブログイン



## タイムアウト

Script API への HTTP リクエストのタイムアウト時間を設定できます。デフォルトは **180 秒** です。

### 優先順位

1. `--no-timeout` フラグ / `GLASP_NO_TIMEOUT` 環境変数（無制限）
2. `--timeout` フラグ / `GLASP_TIMEOUT` 環境変数
3. `.glasp/config.json` の `timeoutSeconds`
4. デフォルト（180 秒）

`0` は *未設定* を意味し、次の優先順位にフォールバックします。負の値は不正な入力として無視され、警告が表示されます（タイムアウトを無効化したい場合は `--no-timeout` を使用してください）。

### 設定方法

```bash
# CLI フラグで指定（秒）
glasp push --timeout 60

# 環境変数で指定
GLASP_TIMEOUT=60 glasp push

# タイムアウトを無制限にする
glasp push --no-timeout

# 環境変数で無制限にする
GLASP_NO_TIMEOUT=1 glasp push
```

`.glasp/config.json` でプロジェクトごとに固定値を設定することもできます:

```json
{
  "archive": { "pull": false, "push": false },
  "timeoutSeconds": 60
}
```

## リトライ

glasp は一時的な Script API 障害（HTTP 5xx・429・ネットワークエラー）を自動的にリトライします。リトライが適用されるのは冪等なコマンドのみです: **push**、**pull**、**list-deployments**、**clone**。非冪等なコマンド（create-script・create-deployment・update-deployment・run-function）はリソースの重複作成を防ぐためリトライしません。

デフォルトのリトライ回数は **3 回**（初回実行を含めると最大 4 回試行）です。

### 優先順位

1. `--max-retries` フラグ / `GLASP_MAX_RETRIES` 環境変数
2. `.glasp/config.json` の `maxRetries`
3. デフォルト（3）

`0` は *未設定* を意味し、次の優先順位にフォールバックします。負の値は不正な入力として無視され、警告が表示されます。リトライを完全に無効化するには `--no-retries` を使用してください。

> **注意:** `--no-timeout` と `--no-retries` は独立しています。`--no-timeout` はリトライを無効化しません。`http.Client.Timeout`（`--timeout` で設定）はリトライ全体を含む合計時間の予算です。タイムアウトが短すぎるとリトライが実行される前に打ち切られる場合があります。

### 設定方法

```bash
# リトライ回数を増やす
glasp push --max-retries 5

# 環境変数で指定
GLASP_MAX_RETRIES=5 glasp push

# リトライを無効化
glasp push --no-retries

# 環境変数でリトライを無効化
GLASP_NO_RETRIES=1 glasp push
```

`.glasp/config.json` でプロジェクトごとに固定値を設定することもできます:

```json
{
  "archive": { "pull": false, "push": false },
  "timeoutSeconds": 60,
  "maxRetries": 5
}
```

## 設定

### .clasp.json

clasp と同じ設定ファイルを利用します:

```json
{
  "scriptId": "your-script-id",
  "rootDir": "src",
  "fileExtension": "ts"
}
```

### .claspignore

gitignore 構文でファイルを除外します。glasp のデフォルト除外:

- `.glasp/` — glasp 内部ディレクトリ（常に除外）
- `node_modules/` — npm 依存

## Convert

```bash
glasp convert --gas-to-ts              # GAS JS → TypeScript
glasp convert --ts-to-gas              # TypeScript → GAS JS
glasp convert --ts-to-gas src/main.ts  # 特定ファイルのみ変換
```

## 履歴

```bash
glasp history                          # 全履歴を表示
glasp history --limit 10               # 直近10件
glasp history --status success         # ステータスでフィルタ
glasp history --command push           # コマンドでフィルタ
glasp history --order asc              # 昇順（デフォルト: 降順）
```
