---
layout: default
title: glasp - Google Apps Script CLI in Go
description: glasp is a Go-based CLI tool for managing Google Apps Script projects, fully compatible 
---

# glasp

[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/takihito/glasp/badge)](https://scorecard.dev/viewer/?uri=github.com/takihito/glasp)

glasp is a Go CLI tool for managing Google Apps Script projects.
A single-binary, high-performance replacement for Node.js-based [clasp](https://github.com/google/clasp).

[日本語ドキュメント](ja/)

## Features

- **Full clasp compatibility** — works with `.clasp.json`, `.claspignore`, `.clasprc.json`
- **TypeScript auto-transpilation** — `.ts` files are automatically converted on push
- **OAuth2 authentication** — smooth login via local callback server
- **Command history** — execution history with replay support
- **Archive** — snapshot push/pull operations
- **Single binary** — download and run, no installation dependencies

## Quick Start

```bash
# Install (Linux / macOS)
curl -sSL https://takihito.github.io/glasp/install.sh | sh

# Create a project directory
mkdir project-gas
cd project-gas

# Create .clasp.json (set the script ID and rootDir)
glasp config init --script-id=<SCRIPT_ID> --rootDir=src

# Log in by reusing clasp credentials (or run `glasp login` for the browser flow)
glasp login --auth ~/.clasprc.json

# Pull remote files into src/
glasp pull
```

Installs to `~/.local/bin` by default. To change the install directory:

```bash
curl -sSL https://takihito.github.io/glasp/install.sh | GLASP_INSTALL_DIR=/usr/local/bin sh
```

> **Windows:** `irm https://takihito.github.io/glasp/install.ps1 | iex`

See [Installation](installation), [Usage](usage), and [GitHub Actions](github-actions) for details.

## Supply-Chain Security

- Release binaries are signed with [cosign](https://github.com/sigstore/cosign)
- [SLSA Level 3](https://slsa.dev/) provenance attached to releases
- [https://socket.dev/](https://socket.dev/) for dependency analysis and monitoring
- [OpenSSF Scorecard](https://scorecard.dev/viewer/?uri=github.com/takihito/glasp) published
