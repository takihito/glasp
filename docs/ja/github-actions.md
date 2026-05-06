---
layout: default
title: GitHub Actions
description: GitHub Actions ワークフローで glasp を使って Google Apps Script のデプロイを自動化する方法。
---

# GitHub Actions

glasp は Composite Action を提供しており、GitHub Actions ワークフローの中で glasp のインストールと認証をまとめて行えます。バイナリの手動ダウンロードや `glasp login` の実行は不要です。

## パフォーマンス比較: glasp と clasp

glasp はシングルバイナリの Go 製ツールであるため、GitHub Actions でのセットアップ時間が npm 経由で clasp をインストールする場合と比べて大幅に短縮されます。以下は `ubuntu-latest` 上で計測したベンチマーク結果です（glasp v0.2.9 / @google/clasp 3.3.0、push/pull は 5 回平均）。

| 指標 | glasp | clasp | 速度比（glasp / clasp）|
|:----|------:|------:|:--------------:|
| セットアップ時間 | 1337ms | 19150ms | **14.3x** |
| Push 時間（平均） | 1015ms | 1229ms | **1.2x** |
| Pull 時間（平均） | 359ms | 1270ms | **3.5x** |
| **合計時間** | **2711ms** | **21649ms** | **7.9x** |

最大の優位点はセットアップ時間です。clasp は毎回 `npm install -g @google/clasp` でパッケージをグローバルインストールする必要があり約 19 秒かかりますが、glasp はコンパイル済みシングルバイナリの取得のみで約 1.3 秒で完了します。セットアップ → push → pull の一連のサイクル全体では **7.9 倍高速**であり、デプロイ頻度の高いパイプラインほど効果が大きくなります。

## Action の入力パラメータ

| 入力 | 必須 | デフォルト | 説明 |
|------|------|-----------|------|
| `version` | いいえ | 最新版 | インストールする glasp のバージョン（例: `v0.2.7`）。省略すると最新リリースを使用します。 |
| `auth` | いいえ | | `.clasprc.json` の JSON 内容。repository secret を指定します。設定すると後続のステップで `GLASP_AUTH` 環境変数として使用できます。 |
| `working-directory` | いいえ | | `.clasp.json` が置かれているディレクトリ（ワークスペースルートからの相対パス）。設定すると `GLASP_DIR` 環境変数にエクスポートされ、後続のすべての `glasp` コマンドがそのディレクトリで実行されます。 |
| `client-id` | いいえ | | OAuth2 クライアント ID。repository secret を指定します。設定すると `GLASP_CLIENT_ID` 環境変数にエクスポートされます。`client-secret` と同時に指定する必要があります。 |
| `client-secret` | いいえ | | OAuth2 クライアントシークレット。repository secret を指定します。設定すると `GLASP_CLIENT_SECRET` 環境変数にエクスポートされます。`client-id` と同時に指定する必要があります。 |

## セットアップ

### 1. `.glasp/access.json`,`.clasprc.json` の内容を取得する

GitHub Actions で glasp を認証するために、ローカルマシンで `glasp login` または `clasp login` を実行して認証情報を取得します。
これらのコマンドはそれぞれ `.glasp/access.json` と `~/.clasprc.json` に認証情報を保存します。

ローカルマシンで glasp または clasp でログインを実行します

```bash
glasp login
cat .glasp/access.json
```

```bash
clasp login
cat ~/.clasprc.json
```

シェル変数に読み込んでCLIを実行することもできます：

```bash
export GLASP_AUTH=$(cat ~/.clasprc.json) && glasp push # clasp login の場合
```

### 2. repository secret に追加する

`.glasp/access.json`, `.clasprc.json` の JSON 内容をコピーし `CLASPRC_JSON` という名前の repository secret として登録します：

**GitHub → リポジトリ → Settings → Secrets and variables → Actions → New repository secret**

- Name: `CLASPRC_JSON`
- Value: *（JSON の内容を貼り付け）*

### 3. ワークフローに Action を追加する

```yaml
steps:
  - uses: actions/checkout@v4
  - uses: takihito/glasp@v0.2.9
    with:
      version: 'v0.2.9'
{% raw %}
      auth: '${{ secrets.CLASPRC_JSON }}'  # 登録したシークレットを指定しglaspに認証情報を渡す
{% endraw %}
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
  
      - uses: takihito/glasp@v0.2.9
        with:
          version: 'v0.2.9'
{% raw %}
          auth: '${{ secrets.CLASPRC_JSON }}'  # 登録したシークレットを指定しglaspに認証情報を渡す
{% endraw %}

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

      - uses: takihito/glasp@v0.2.9
        with:
          version: 'v0.2.9'
{% raw %}
          auth: '${{ secrets.CLASPRC_JSON }}'  # 登録したシークレットを指定
{% endraw %}

      - name: ファイルを push
        run: glasp push

      - name: デプロイメントを作成
        run: glasp create-deployment --description "Deploy from CI"
```

### プロジェクト

glasp は`.clasp.json`の設定に従って`.ts` ファイルを自動検出し、push 前に esbuild でトランスパイルします。追加設定は不要です：

```yaml
- uses: takihito/glasp@v0.2.9
  with:
    version: 'v0.2.9'
{% raw %}
    auth: '${{ secrets.CLASPRC_JSON }}'
    working-directory: 'apps-script/dir' # .clasp.json があるディレクトリ / 省略した場合はワークスペースルートが使用されます
    client-id: ${{ secrets.GLASP_CLIENT_ID }}         # OAuth2 クライアントIDを指定する場合
    client-secret: ${{ secrets.GLASP_CLIENT_SECRET }} # OAuth2 クライアントシークレットを指定する場合
{% endraw %}

- name: プロジェクトを push
  run: glasp push
```

## 認証の仕組み

`auth` を設定すると、Action はその値を `GLASP_AUTH` 環境変数としてエクスポートします。glasp はこの環境変数を読み取り、JSON 内容をそのまま認証情報として使用します。ファイルへの書き込みや `glasp login` の実行は不要です。glasp 内部の認証ソースの優先順位：

1. `--auth <path>` フラグ
2. `GLASP_AUTH` 環境変数 ← GitHub Actions が設定します
3. プロジェクトキャッシュ（`.glasp/access.json`）

`GLASP_AUTH` にセットする値は、`glasp login` 後の `.glasp/access.json` または `clasp login` 後の `~/.clasprc.json` の JSON 内容を repository secret にコピーしたものです。

`client-id` と `client-secret` を設定すると、`GLASP_CLIENT_ID` と `GLASP_CLIENT_SECRET` もエクスポートされ、glasp の OAuth フローでビルド時埋め込みのデフォルト認証情報の代わりにカスタムクレデンシャルを使用できます。

## モノレポ / サブディレクトリのプロジェクト

`.clasp.json` がサブディレクトリにある場合（モノレポ構成など）、`working-directory` input を使います：

```yaml
- uses: takihito/glasp@v0.2.9
  with:
    version: 'v0.2.9'
{% raw %}
    auth: '${{ secrets.CLASPRC_JSON }}'
{% endraw %}
    working-directory: 'apps-script/dir'   # .clasp.json があるディレクトリ
```

これにより `GLASP_DIR=<絶対パス>` が環境変数としてエクスポートされます。後続のすべての `glasp` コマンドが自動的にこの値を読み取るため、各ステップへの `--dir` フラグや `working-directory:` の指定は不要です。

`--dir` フラグや `GLASP_DIR` 環境変数でコマンドごとに指定することもできます：

```yaml
- run: glasp push
  env:
    GLASP_DIR: apps-script/dir

# または同等の表記：
- run: glasp --dir apps-script/dir push
```

## バージョンの固定

再現性のあるワークフローのために、バージョンを明示的に指定することを推奨します：

```yaml
- uses: takihito/glasp@v0.2.9   # 推奨: リリースタグで固定
  with:
    version: 'v0.2.9'
```

GitHub Release の artifact は immutable（変更不可）であるため、`version` を固定すると毎回同じバイナリがインストールされることが保証されます。

より厳密なサプライチェーン管理が必要な場合は、コミット SHA で Action 自体を固定することもできます：

```yaml
- uses: takihito/glasp@75089df   # 特定のコミットに固定
  with:
    version: 'v0.2.9'
```
