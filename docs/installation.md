---
layout: default
title: glasp Installation
description: Installation instructions for glasp, the Google Apps Script CLI in Go.
---

# Installation

## Quick Install (recommended)

**Linux / macOS:**

```bash
curl -fsSL https://takihito.github.io/glasp/install.sh | sh
```

**Windows (PowerShell):**

```powershell
irm https://takihito.github.io/glasp/install.ps1 | iex
```

Automatically detects the latest version, verifies checksums, and installs to `~/.local/bin`. No `sudo` required.

If `~/.local/bin` is not in your PATH, add the following to your shell profile (`~/.bashrc`, `~/.zshrc`, etc.):

```bash
export PATH="$HOME/.local/bin:$PATH"
```

To change the install directory:

```bash
curl -fsSL https://takihito.github.io/glasp/install.sh | GLASP_INSTALL_DIR=/usr/local/bin sh
```

## Verifying downloads

Release `checksums.txt` files are signed with [cosign](https://docs.sigstore.dev/cosign/system_config/installation/) (keyless, by this repository's release workflow). The signature is checked against the Sigstore trust root, independently of the TLS connection, so it also detects a download that was tampered with through an intercepted connection.

- **Quick Install:** if `cosign` is installed, the installer verifies the signature automatically; otherwise it prints a warning and continues. Set `GLASP_VERIFY_SIGNATURE=required` to fail when `cosign` is missing:

  ```bash
  curl -fsSL https://takihito.github.io/glasp/install.sh | GLASP_VERIFY_SIGNATURE=required sh
  ```

- **GitHub Actions:** the `takihito/glasp` action installs `cosign` and always verifies the signature.

`curl | sh` cannot protect the installer script itself: whoever can tamper with the download can also replace `install.sh`. Keep your OS and CA certificates up to date, and for the strongest guarantee download and verify the release manually:

```bash
VERSION=v0.5.2
ARTIFACT=glasp_${VERSION}_linux_amd64.tar.gz   # pick your OS/arch
BASE=https://github.com/takihito/glasp/releases/download/${VERSION}
curl -fsSLO "${BASE}/${ARTIFACT}"
curl -fsSLO "${BASE}/checksums.txt"
curl -fsSLO "${BASE}/checksums.txt.sigstore.json"

cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/takihito/glasp/.github/workflows/release.yml@refs/tags/${VERSION}" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
grep "  ${ARTIFACT}\$" checksums.txt | shasum -a 256 -c -   # or: sha256sum -c -
```

## Homebrew (macOS / Linux)

```bash
brew tap takihito/tap
brew trust --formula takihito/tap/glasp
brew install glasp
```

> `brew trust` is required because [takihito/tap](https://github.com/takihito/homebrew-tap) is a non-official tap. Installs a pre-built binary with OAuth credentials embedded.

## go install

```bash
go install github.com/takihito/glasp/cmd/glasp@latest
```

> This method does not embed OAuth credentials. Set `GLASP_CLIENT_ID` and `GLASP_CLIENT_SECRET` environment variables.

## Pre-built binaries

For manual downloads, see the [Releases](https://github.com/takihito/glasp/releases) page.

## Build from source

```bash
git clone https://github.com/takihito/glasp.git
cd glasp
make build    # Build binary to bin/glasp
make install  # Build and install globally
```

## OAuth credentials

| Install method | Credentials |
|---------------|-------------|
| Quick Install / Homebrew (pre-built binaries) | Embedded, override with env vars |
| `go install` / source build | Env vars required |

```bash
export GLASP_CLIENT_ID="your-client-id"
export GLASP_CLIENT_SECRET="your-client-secret"
```

Environment variables take precedence over embedded credentials. See [Google Cloud Console](https://console.cloud.google.com/apis/credentials) to create OAuth 2.0 credentials for a Desktop application.
