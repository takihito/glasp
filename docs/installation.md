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

Automatically detects the latest version, verifies checksums, and installs to `~/.local/bin`. No `sudo` required.

If `~/.local/bin` is not in your PATH, add the following to your shell profile (`~/.bashrc`, `~/.zshrc`, etc.):

```bash
export PATH="$HOME/.local/bin:$PATH"
```

To change the install directory:

```bash
curl -sSL https://takihito.github.io/glasp/install.sh | GLASP_INSTALL_DIR=/usr/local/bin sh
```

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
| Quick Install (pre-built binaries) | Embedded, override with env vars |
| `go install` / source build | Env vars required |

```bash
export GLASP_CLIENT_ID="your-client-id"
export GLASP_CLIENT_SECRET="your-client-secret"
```

Environment variables take precedence over embedded credentials. See [Google Cloud Console](https://console.cloud.google.com/apis/credentials) to create OAuth 2.0 credentials for a Desktop application.
