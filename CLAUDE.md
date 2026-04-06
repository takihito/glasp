# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**glasp** is a Go CLI tool that replaces Node.js-based `clasp` for managing Google Apps Script projects. It's a single-binary, high-performance alternative with full clasp compatibility.

## Build & Test Commands

```bash
make build              # Build binary to bin/glasp (injects OAuth credentials from .env via ldflags)
make install            # Build and install globally
make test               # Run all tests (loads .env for credentials)
make clean              # Remove binary and clear test cache

# Run tests incrementally (recommended order):
go test ./internal/auth/...          # Target package first
go test ./internal/...               # Related packages
go test ./cmd/glasp -run TestName    # Single test
go test -v ./...                     # Full suite
```

## Architecture

### Entry Point & CLI

All CLI commands are defined in `cmd/glasp/main.go` as a single struct parsed by `github.com/alecthomas/kong`. Commands: login, logout, create, clone, pull, push, open, create-deployment, deploy (alias for update-deployment), list-deployments, run, convert, history, config, version.

### Internal Packages

- **auth** — OAuth2 flow with local HTTP callback server; tokens stored at `.glasp/access.json` (0600 perms). Auth source priority: `--auth` flag → project cache → login flow. Supports `.clasprc.json` for clasp compatibility.
- **config** — Reads `.clasp.json` (clasp compat) and `.glasp/config.json` (glasp-specific archive settings). Parses `.claspignore` using go-gitignore. Default ignores: `.glasp/`, `node_modules/`. `EnsureGlaspDir` creates `.glasp/` (0700 perms) and auto-adds `.glasp/` to `.claspignore`.
- **scriptapi** — Thin wrapper around Google Apps Script API v1.
- **syncer** — Collects local files (respecting .claspignore), builds API content payloads, and applies remote content to local disk. Default file extensions include `.js`, `.gs`, `.ts`, `.html`. `.d.ts` files and the `.glasp/` directory are always excluded from collection.
- **transform** — TypeScript ↔ GAS conversion using esbuild Go API. Auto-applied on push when `.ts` files are detected (regardless of `fileExtension` setting). On pull, applied when `fileExtension` is "ts".
- **history** — Command execution history in `.glasp/history.jsonl` (JSON Lines). File-locked writes, sensitive args masked. Supports replay via `push --history-id <id>`.

### Push/Pull Flow

**Push**: CollectLocalFiles → (optional TS→GAS transform) → BuildContent → UpdateContent API → (optional archive to `.glasp/archive/`)

**Pull**: GetContent API → (optional GAS→TS transform) → ApplyRemoteContent → (optional archive)

### Archive Structure

Archives live under `.glasp/archive/<scriptId>/<push|pull>/<timestamp>/` with `manifest.json`, `working/`, and (push only) `payload/` + `payloadIndex`.

## Key Conventions

- Language: Go (latest stable), standard project layout (`cmd/`, `internal/`)
- CLI framework: `github.com/alecthomas/kong` — declarative, type-safe struct tags
- Must maintain compatibility with clasp's `.clasp.json`, `.claspignore`, and file path conversion logic
- Errors must be wrapped with context (Go idiomatic check-and-return)
- OAuth credentials (`GLASP_CLIENT_ID`, `GLASP_CLIENT_SECRET`) injected at build time via `-ldflags` from `.env`
- Detailed AI development guidelines in `AGENTS.md` and `docs/ai/static/`
