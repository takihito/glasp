---
layout: default
title: Home
---

# glasp

[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/takihito/glasp/badge)](https://scorecard.dev/viewer/?uri=github.com/takihito/glasp)

**glasp** is a Go CLI tool for managing Google Apps Script projects.
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

# Login
glasp login

# Clone an existing project
glasp clone <script-id>

# Pull and push files
glasp pull
glasp push
```

Installs to `~/.local/bin` by default. To change the install directory:

```bash
curl -sSL https://takihito.github.io/glasp/install.sh | GLASP_INSTALL_DIR=/usr/local/bin sh
```

> **Windows:** `irm https://takihito.github.io/glasp/install.ps1 | iex`

See [Installation](installation) and [Usage](usage) for details.

## Supply-Chain Security

glasp takes supply-chain security seriously.

- Release binaries are signed with [cosign](https://github.com/sigstore/cosign)
- [SLSA Level 3](https://slsa.dev/) provenance attached to releases
- [OpenSSF Scorecard](https://scorecard.dev/viewer/?uri=github.com/takihito/glasp) published
- All GitHub Actions pinned to commit SHA
