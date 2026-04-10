---
layout: default
title: Usage
---

# Usage

## Commands

| Command | Alias | Description |
|---------|-------|-------------|
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

### TypeScript Support

glasp は push 時に `.ts` ファイルを自動検出してトランスパイルします（`.clasp.json` の `fileExtension` 設定に関係なく）。

- `.ts` → esbuild で JavaScript に変換してプッシュ
- `.js`, `.gs` → そのままプッシュ
- `.d.ts` → 除外（デプロイ不可）

### History Replay

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

## Authentication

認証トークンは `.glasp/access.json` に保存されます（パーミッション `0600`）。

認証ソースの優先順位:
1. `--auth` フラグ（`.clasprc.json` のパス）
2. プロジェクトキャッシュ（`.glasp/access.json`）
3. インタラクティブログイン

### clasp の認証情報を再利用

```bash
# 既存の clasp credentials を使用
glasp push --auth ~/.clasprc.json
glasp pull --auth ~/.clasprc.json
glasp clone SCRIPT_ID --auth ~/.clasprc.json
```

clasp から glasp への移行時に、再認証なしですぐに使い始められます。

## Configuration

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

## History

```bash
glasp history                          # 全履歴を表示
glasp history --limit 10               # 直近10件
glasp history --status success         # ステータスでフィルタ
glasp history --command push           # コマンドでフィルタ
glasp history --order asc              # 昇順（デフォルト: 降順）
```
