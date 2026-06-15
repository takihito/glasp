---
layout: default
title: glasp Usage
description: Usage instructions for glasp, the Google Apps Script CLI in Go.
---

# Usage

## Commands

| Command | Alias | Description |
|---------|-------|-------------|
| `login` | | Log in to Google account |
| `logout` | | Log out from Google account |
| `create-script` | `create` | Create a new Apps Script project |
| `clone` | | Clone an existing Apps Script project |
| `pull` | | Download project files from Apps Script |
| `push` | | Upload project files to Apps Script |
| `open-script` | `open` | Open the Apps Script project in browser |
| `create-deployment` | | Create a deployment |
| `update-deployment` | `deploy` | Update an existing deployment |
| `list-deployments` | | List deployments |
| `run-function` | | Run an Apps Script function remotely |
| `convert` | | Convert project files (TS ↔ GAS) |
| `history` | | Show command execution history |
| `config init` | | Create `.glasp/config.json` |
| `version` | | Show version |

## Push

```bash
glasp push              # Push local files
glasp push --force      # Ignore .claspignore (.glasp/ is always excluded)
glasp push --archive    # Archive pushed files
glasp push --dryrun     # Dry run without API calls
```

### TypeScript Support

glasp automatically detects and transpiles `.ts` files on push, regardless of `fileExtension` setting in `.clasp.json`.

- `.ts` → transpiled to JavaScript via esbuild before push
- `.js`, `.gs` → passed through unchanged
- `.d.ts` → excluded (not deployable)

### History Replay

```bash
glasp push --history-id <id>          # Re-push from archive
glasp push --history-id <id> --dryrun # Dry run replay
```

## Pull

```bash
glasp pull              # Pull remote files
glasp pull --archive    # Archive pulled files
```

When `fileExtension` is `"ts"`, pulled files are automatically converted from GAS JavaScript to TypeScript.

## Authentication

Auth tokens are stored at `.glasp/access.json` (permission `0600`).

```bash
# A browser opens automatically to start the login flow
glasp login
```

### PKCE

Enable OAuth2 PKCE (RFC 7636) to protect against authorization code interception attacks:

```bash
glasp login --pkce

# Or via environment variable
GLASP_USE_PKCE=1 glasp login
```

PKCE applies only to the interactive `glasp login` flow.

### Reuse clasp credentials

```bash
# Reuse clasp credentials. They are imported via `glasp login` and saved to `.glasp/access.json`.
glasp login --auth ~/.clasprc.json

# Or use --auth directly on each command
glasp push --auth ~/.clasprc.json
glasp pull --auth ~/.clasprc.json
glasp clone SCRIPT_ID --auth ~/.clasprc.json
```

Start using glasp immediately when migrating from clasp — no re-authentication needed.

### Auth source priority

1. `--auth` flag (path to `.clasprc.json`)
2. Project cache (`.glasp/access.json`)
3. Interactive login flow

## Timeout

You can configure the HTTP request timeout for Script API calls. The default is **180 seconds**.

### Priority

1. `--no-timeout` flag / `GLASP_NO_TIMEOUT` environment variable (unlimited)
2. `--timeout` flag / `GLASP_TIMEOUT` environment variable
3. `timeoutSeconds` in `.glasp/config.json`
4. Default (180 seconds)

### Configuration

```bash
# Set via CLI flag (seconds)
glasp push --timeout 60

# Set via environment variable
GLASP_TIMEOUT=60 glasp push

# Disable timeout (unlimited)
glasp push --no-timeout

# Disable timeout via environment variable
GLASP_NO_TIMEOUT=1 glasp push
```

You can also set a project-wide value in `.glasp/config.json`:

```json
{
  "archive": { "pull": false, "push": false },
  "timeoutSeconds": 60
}
```

## Configuration

### .clasp.json

Standard clasp configuration file. glasp reads the same format:

```json
{
  "scriptId": "your-script-id",
  "rootDir": "src",
  "fileExtension": "ts"
}
```

### .claspignore

Gitignore syntax for excluding files. glasp default excludes:

- `.glasp/` — glasp internal directory (always excluded)
- `node_modules/` — npm dependencies

## Convert

```bash
glasp convert --gas-to-ts              # GAS JS → TypeScript
glasp convert --ts-to-gas              # TypeScript → GAS JS
glasp convert --ts-to-gas src/main.ts  # Convert specific files
```

## History

```bash
glasp history                          # Show all history
glasp history --limit 10               # Last 10 entries
glasp history --status success         # Filter by status
glasp history --command push           # Filter by command
glasp history --order asc              # Oldest first (default: desc)
```
