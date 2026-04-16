---
layout: default
title: GitHub Actions
description: GitHub Actions ワークフローで glasp を使って Google Apps Script のデプロイを自動化する方法。
---

# GitHub Actions

glasp は Composite Action を提供しており、GitHub Actions ワークフローの中で glasp のインストールと認証をまとめて行えます。バイナリの手動ダウンロードや `glasp login` の実行は不要です。

## Action の入力パラメータ

| 入力 | 必須 | デフォルト | 説明 |
|------|------|-----------|------|
| `version` | いいえ | 最新版 | インストールする glasp のバージョン（例: `v1.2.0`）。省略すると最新リリースを使用します。 |
| `auth` | いいえ | | `.clasprc.json` の JSON 内容。リポジトリシークレットを指定します。設定すると `GLASP_AUTH`（JSON 内容）と `GLASP_AUTH_FILE`（JSON を書き出した一時ファイルのパス）が後続のステップで使用できます。 |
| `working-directory` | いいえ | | `.clasp.json` が置かれているディレクトリ（ワークスペースルートからの相対パス）。設定すると `GLASP_DIR` 環境変数にエクスポートされ、後続のすべての `glasp` コマンドがそのディレクトリで実行されます。 |

## セットアップ

### 1. `.glasp/access.json`,`.clasprc.json` の内容を取得する

ローカルマシンで glasp または clasp でログインします：

```bash
glasp login
cat .glasp/access.json
```

```bash
clasp login
cat ~/.clasprc.json
```

シェル変数に読み込んで動作確認することもできます：

```bash
GLASP_AUTH=$(cat .glasp/access.json)   # glasp login の場合
GLASP_AUTH=$(cat ~/.clasprc.json)      # clasp login の場合
```

### 2. リポジトリシークレットに追加する

`.glasp/access.json`, `.clasprc.json` の JSON 内容をコピーし `GLASP_AUTH` という名前のリポジトリシークレットとして登録します：

**GitHub → リポジトリ → Settings → Secrets and variables → Actions → New repository secret**

- Name: `GLASP_AUTH`
- Value: *（JSON の内容を貼り付け）*

### 3. ワークフローに Action を追加する

```yaml
steps:
  - uses: actions/checkout@v4
  - uses: takihito/glasp@v1.2.0
    with:
      version: 'v1.2.0'
      auth: ${{ secrets.GLASP_AUTH }}  # 登録したシークレットを指定しglaspに認証情報を渡す
  - run: glasp push
```

## 使用例

### main ブランチへのコミット時に自動 push

```yaml
name: Deploy to Google Apps Script

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: takihito/glasp@v1.2.0
        with:
          version: 'v1.2.0'
          auth: ${{ secrets.GLASP_AUTH }}  # 登録したシークレットを指定しglaspに認証情報を渡す

      - name: Push to Apps Script
        run: glasp push
```

### push 後にデプロイメントを作成する

```yaml
name: Deploy to Google Apps Script

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: takihito/glasp@v1.2.0
        with:
          version: 'v1.2.0'
          auth: ${{ secrets.GLASP_AUTH }}  # 登録したシークレットを指定

      - name: ファイルを push
        run: glasp push

      - name: デプロイメントを作成
        run: glasp create-deployment --description "Deploy from CI"
```

### TypeScript プロジェクト

glasp は`.clasp.json`の設定に従って`.ts` ファイルを自動検出し、push 前に esbuild でトランスパイルします。追加設定は不要です：

```yaml
- uses: takihito/glasp@v1.2.0
  with:
    version: 'v1.2.0'
    auth: ${{ secrets.GLASP_AUTH }}

- name: TypeScript プロジェクトを push
  run: glasp push
```

## 認証の仕組み

`auth` を設定すると、Action はその値を `GLASP_AUTH` 環境変数としてエクスポートします。glasp はこの環境変数を読み取り、JSON 内容をそのまま認証情報として使用します。ファイルへの書き込みや `glasp login` の実行は不要です。glasp 内部の認証ソースの優先順位：

1. `--auth <path>` フラグ
2. `GLASP_AUTH` 環境変数 ← この Action が設定します
3. プロジェクトキャッシュ（`.glasp/access.json`）

## モノレポ / サブディレクトリのプロジェクト

`.clasp.json` がサブディレクトリにある場合（モノレポ構成など）、`working-directory` input を使います：

```yaml
- uses: takihito/glasp@v1.2.0
  with:
    version: 'v1.2.0'
    auth: ${{ secrets.GLASP_AUTH }}
    working-directory: 'apps-script'   # .clasp.json があるディレクトリ
```

これにより `GLASP_DIR=<絶対パス>` が環境変数としてエクスポートされます。後続のすべての `glasp` コマンドが自動的にこの値を読み取るため、各ステップへの `--dir` フラグや `working-directory:` の指定は不要です。

`--dir` フラグや `GLASP_DIR` 環境変数でコマンドごとに指定することもできます：

```yaml
- run: glasp push
  env:
    GLASP_DIR: apps-script

# または同等の表記：
- run: glasp --dir apps-script push
```

## バージョンの固定

再現性のあるワークフローのために、バージョンを明示的に指定することを推奨します：

```yaml
- uses: takihito/glasp@v1.2.0   # 推奨: リリースタグで固定
  with:
    version: 'v1.2.0'
```

GitHub Release の artifact は immutable（変更不可）であるため、`version` を固定すると毎回同じバイナリがインストールされることが保証されます。

より厳密なサプライチェーン管理が必要な場合は、コミット SHA で Action 自体を固定することもできます：

```yaml
- uses: takihito/glasp@1ae5afb   # 特定のコミットに固定
  with:
    version: 'v1.2.0'
```
