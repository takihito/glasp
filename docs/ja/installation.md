---
layout: default
title: インストール
---

# インストール

## クイックインストール（推奨）

**Linux / macOS:**

```bash
curl -sSL https://takihito.github.io/glasp/install.sh | sh
```

**Windows (PowerShell):**

```powershell
irm https://takihito.github.io/glasp/install.ps1 | iex
```

最新バージョンを自動検出し、チェックサム検証後に `~/.local/bin` にインストールします。`sudo` は不要です。

`~/.local/bin` が PATH に含まれていない場合は、シェルの設定ファイル（`~/.bashrc`, `~/.zshrc` 等）に以下を追記してください:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

インストール先を変更する場合:

```bash
curl -sSL https://takihito.github.io/glasp/install.sh | GLASP_INSTALL_DIR=/usr/local/bin sh
```

## go install

```bash
go install github.com/takihito/glasp/cmd/glasp@latest
```

> この方法では OAuth credentials が含まれません。`GLASP_CLIENT_ID` と `GLASP_CLIENT_SECRET` 環境変数を設定してください。

## Pre-built binaries

手動でダウンロードする場合は [Releases](https://github.com/takihito/glasp/releases) ページを参照してください。

## ソースからビルド

```bash
git clone https://github.com/takihito/glasp.git
cd glasp
make build    # bin/glasp にビルド
make install  # グローバルにインストール
```

## OAuth credentials

| インストール方法 | credentials |
|-----------------|-------------|
| クイックインストール（ビルド済みバイナリ） | 埋め込み済み、環境変数で上書き可能 |
| `go install` / ソースビルド | 環境変数で指定が必要 |

```bash
export GLASP_CLIENT_ID="your-client-id"
export GLASP_CLIENT_SECRET="your-client-secret"
```

環境変数は埋め込み credentials より優先されます。[Google Cloud Console](https://console.cloud.google.com/apis/credentials) からデスクトップアプリケーション用の OAuth 2.0 credentials を作成してください。
