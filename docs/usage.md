---
layout: default
title: Usage
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

Auth source priority:
1. `--auth` flag (path to `.clasprc.json`)
2. Project cache (`.glasp/access.json`)
3. Interactive login flow

### Reuse clasp credentials

```bash
# Import clasp credentials into glasp
glasp login --auth ~/.clasprc.json

# Or use --auth directly on each command
glasp push --auth ~/.clasprc.json
glasp pull --auth ~/.clasprc.json
glasp clone SCRIPT_ID --auth ~/.clasprc.json
```

Start using glasp immediately when migrating from clasp — no re-authentication needed.

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
