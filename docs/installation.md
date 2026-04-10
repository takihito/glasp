---
layout: default
title: Installation
---

# Installation

## Quick Install (recommended)

**Linux / macOS:**

```bash
curl -sSL https://takihito.github.io/glasp/install.sh | sh
```

**Windows (PowerShell):**

```powershell
irm https://takihito.github.io/glasp/install.ps1 | iex
```

最新バージョンを自動検出し、チェックサム検証後にインストールします。

> インストール先を変更するには `GLASP_INSTALL_DIR` 環境変数を設定してください。

## go install

```bash
go install github.com/takihito/glasp/cmd/glasp@latest
```

> この方法では OAuth credentials が含まれません。`GLASP_CLIENT_ID` と `GLASP_CLIENT_SECRET` 環境変数を設定してください。

## Pre-built binaries

[Releases](https://github.com/takihito/glasp/releases) ページからダウンロードできます。
リリースバイナリには OAuth credentials が埋め込み済みで、すぐに利用できます。

```bash
VERSION=0.1.2
OS=${OS:-darwin}     # darwin, linux, or windows
ARCH=${ARCH:-arm64}  # arm64 or amd64
ARTIFACT="glasp_v${VERSION}_${OS}_${ARCH}.tar.gz"
CHECKSUMS="checksums.txt"

curl -L -o "${ARTIFACT}" \
  "https://github.com/takihito/glasp/releases/download/v${VERSION}/${ARTIFACT}"
curl -L -o "${CHECKSUMS}" \
  "https://github.com/takihito/glasp/releases/download/v${VERSION}/${CHECKSUMS}"

# Verify checksum
if command -v sha256sum >/dev/null 2>&1; then
  grep "  ${ARTIFACT}$" "${CHECKSUMS}" | sha256sum -c
else
  grep "  ${ARTIFACT}$" "${CHECKSUMS}" | shasum -a 256 -c
fi

# Install
sudo tar -xzf "${ARTIFACT}" -C /usr/local/bin glasp
```

> **Windows:** `.zip` アーカイブをダウンロードしてください。

## Build from source

```bash
git clone https://github.com/takihito/glasp.git
cd glasp
make build    # bin/glasp にビルド
make install  # グローバルにインストール
```

## OAuth credentials

| インストール方法 | credentials |
|-----------------|-------------|
| Pre-built binaries | 埋め込み済み,環境変数で上書き可能 |
| `go install` / ソースビルド | 環境変数で指定が必要 |

```bash
export GLASP_CLIENT_ID="your-client-id"
export GLASP_CLIENT_SECRET="your-client-secret"
```

環境変数は埋め込み credentials より優先されます。
OAuth credentials の作成は [Google Cloud Console](https://console.cloud.google.com/apis/credentials) からデスクトップアプリケーション用の OAuth 2.0 credentials を作成してください。
