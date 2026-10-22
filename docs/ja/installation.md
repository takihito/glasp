---
layout: default
title: インストール
---

# インストール

## クイックインストール（推奨）

**Linux / macOS:**

```bash
curl -fsSL https://takihito.github.io/glasp/install.sh | sh
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
curl -fsSL https://takihito.github.io/glasp/install.sh | GLASP_INSTALL_DIR=/usr/local/bin sh
```

## ダウンロードの検証

リリースの `checksums.txt` には、このリポジトリのリリースワークフローが [cosign](https://docs.sigstore.dev/cosign/system_config/installation/) で署名しています（キーレス署名）。署名は TLS 接続とは別に Sigstore の信頼ルートで検証するため、通信を傍受されて改ざんされたダウンロードも検出できます。

- **クイックインストール:** `cosign` がインストールされていれば、インストーラーが自動で署名を検証します。ない場合は警告を出して続行します。`cosign` がないときに失敗させるには `GLASP_VERIFY_SIGNATURE=required` を指定します:

  ```bash
  curl -fsSL https://takihito.github.io/glasp/install.sh | GLASP_VERIFY_SIGNATURE=required sh
  ```

- **GitHub Actions:** `takihito/glasp` アクションは `cosign` をインストールし、常に署名を検証します。

`curl | sh` では、インストーラースクリプト自体は守れません。ダウンロードを改ざんできる攻撃者は `install.sh` も差し替えられるためです。OS と CA 証明書を最新に保ち、より確実にするには手動でダウンロードして検証してください:

```bash
VERSION=v0.5.2
ARTIFACT=glasp_${VERSION}_linux_amd64.tar.gz   # OS/アーキテクチャに合わせて選択
BASE=https://github.com/takihito/glasp/releases/download/${VERSION}
curl -fsSLO "${BASE}/${ARTIFACT}"
curl -fsSLO "${BASE}/checksums.txt"
curl -fsSLO "${BASE}/checksums.txt.sigstore.json"

cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/takihito/glasp/.github/workflows/release.yml@refs/tags/${VERSION}" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
grep "  ${ARTIFACT}\$" checksums.txt | shasum -a 256 -c -   # または: sha256sum -c -
```

## Homebrew（macOS / Linux）

```bash
brew tap takihito/tap
brew trust --formula takihito/tap/glasp
brew install glasp
```

> [takihito/tap](https://github.com/takihito/homebrew-tap) は非公式 tap のため `brew trust` が必要です。ビルド済みバイナリ（OAuth credentials 埋め込み済み）がインストールされます。

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
| クイックインストール / Homebrew（ビルド済みバイナリ） | 埋め込み済み、環境変数で上書き可能 |
| `go install` / ソースビルド | 環境変数で指定が必要 |

```bash
export GLASP_CLIENT_ID="your-client-id"
export GLASP_CLIENT_SECRET="your-client-secret"
```

環境変数は埋め込み credentials より優先されます。[Google Cloud Console](https://console.cloud.google.com/apis/credentials) からデスクトップアプリケーション用の OAuth 2.0 credentials を作成してください。
