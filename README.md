# glasp

A Go CLI tool that replaces Node.js-based [clasp](https://github.com/google/clasp) for managing Google Apps Script projects. Single-binary, high-performance alternative with full clasp compatibility.

## Features

- Full clasp compatibility (`.clasp.json`, `.claspignore`, `.clasprc.json`)
- TypeScript auto-transpilation on push (with or without `fileExtension` setting)
- OAuth2 authentication with local callback server
- Command execution history with replay support
- Archive support for push/pull operations

## Installation

### go install

```bash
go install github.com/takihito/glasp/cmd/glasp@latest
```

### Pre-built binaries

Download from the [Releases](https://github.com/takihito/glasp/releases) page:

```bash
VERSION=0.1.0
OS=${OS:-Darwin}     # Darwin, Linux, or Windows
ARCH=${ARCH:-arm64}  # arm64 or amd64
ARTIFACT="glasp_v${VERSION}_${OS}_${ARCH}.tar.gz"

curl -L -o "${ARTIFACT}" \
  "https://github.com/takihito/glasp/releases/download/v${VERSION}/${ARTIFACT}"

# Verify checksum
echo "SHA256_FROM_RELEASE  ${ARTIFACT}" | shasum -a 256 --check

# Install
sudo tar -xzf "${ARTIFACT}" -C /usr/local/bin glasp
```

### Build from source

```bash
git clone https://github.com/takihito/glasp.git
cd glasp
make build    # Build binary to bin/glasp
make install  # Build and install globally
```

OAuth credentials (`GLASP_CLIENT_ID`, `GLASP_CLIENT_SECRET`) are injected at build time via `-ldflags` from `.env`.

## Quick Start

```bash
# Login to Google account
glasp login

# Clone an existing project
glasp clone <script-id>

# Pull remote files
glasp pull

# Push local files
glasp push

# Create a new project
glasp create-script --title "My Project"
```

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
| `list-deployments` | | List deployments for a script project |
| `run-function` | | Run an Apps Script function remotely |
| `convert` | | Convert project files (TS <-> GAS) |
| `history` | | Show command execution history |
| `config init` | | Create `.glasp/config.json` |
| `version` | | Show glasp version |

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

Standard clasp ignore file using gitignore syntax. glasp applies the following default excludes even without `.claspignore`:

- `.glasp/` — glasp internal directory (always excluded, even with `--force`)
- `node_modules/` — npm dependencies

### .glasp/

glasp-specific directory (created automatically, excluded from push):

| File | Description |
|------|-------------|
| `access.json` | OAuth token cache (0600 perms) |
| `config.json` | glasp-specific settings (archive config) |
| `history.jsonl` | Command execution history (JSON Lines) |
| `archive/` | Push/pull archives |

The `.glasp/` directory is created with `0700` permissions. When first created, glasp automatically adds `.glasp/` to `.claspignore` so that clasp will not push glasp's internal files.

## Push

```bash
glasp push              # Push local files
glasp push --force      # Ignore .claspignore (but .glasp/ is always excluded)
glasp push --archive    # Archive pushed files
glasp push --dryrun     # Dry run without API calls
glasp push --auth path  # Use specific .clasprc.json for auth
```

### TypeScript Support

glasp automatically detects and transpiles `.ts` files on push, regardless of `fileExtension` setting in `.clasp.json`. This matches clasp v2.4.2 behavior.

- `.ts` files are transpiled to JavaScript via esbuild before push
- `.js` and `.gs` files are passed through unchanged
- `.d.ts` files are excluded (declaration files are not deployable)
- Mixed `.ts` + `.js` projects are supported (`.ts` is always collected even when `fileExtension` is `"js"`)
- To customize which extensions are collected, use `scriptExtensions` in `.clasp.json`

### History Replay

```bash
glasp push --history-id <id>          # Re-push from archived payload
glasp push --history-id <id> --dryrun # Dry run replay
```

## Pull

```bash
glasp pull              # Pull remote files
glasp pull --archive    # Archive pulled files
```

When `fileExtension` is set to `"ts"` in `.clasp.json`, pulled files are automatically converted from GAS JavaScript to TypeScript.

## History

```bash
glasp history                          # Show all history
glasp history --limit 10               # Last 10 entries
glasp history --status success         # Filter by status (all|success|error)
glasp history --command push           # Filter by command name
glasp history --order asc              # Oldest first (default: desc)
```

Output format is JSON array (`[]` when no entries).

## Convert

```bash
glasp convert --gas-to-ts              # Convert GAS JS to TypeScript
glasp convert --ts-to-gas              # Convert TypeScript to GAS JS
glasp convert --ts-to-gas src/main.ts  # Convert specific files
```

## Authentication

Auth tokens are stored at `.glasp/access.json` with `0600` permissions. Auth source priority:

1. `--auth` flag (path to `.clasprc.json`)
2. Project cache (`.glasp/access.json`)
3. Interactive login flow

### Using `--auth` with `.clasprc.json`

If you already use clasp, you likely have a `~/.clasprc.json` file containing your OAuth credentials. The `--auth` option lets you reuse this file directly with glasp — **no need to run `glasp login` separately**.

```bash
# Use your existing clasp credentials
glasp push --auth ~/.clasprc.json
glasp pull --auth ~/.clasprc.json
glasp clone SCRIPT_ID --auth ~/.clasprc.json

# You can also pass a directory — glasp will look for .clasprc.json inside it
glasp push --auth ~/
```

This is especially useful when:

- **Migrating from clasp to glasp** — start using glasp immediately without re-authenticating
- **CI/CD pipelines** — share a single `.clasprc.json` across clasp and glasp workflows
- **Multiple Google accounts** — keep separate `.clasprc.json` files per account and switch with `--auth`

The `--auth` option is available on all commands that require authentication: `push`, `pull`, `clone`, `create-script`, `create-deployment`, `update-deployment`, `list-deployments`, and `run-function`.

#### Supported `.clasprc.json` formats

glasp reads the same `.clasprc.json` format that clasp produces. The following credential layouts are supported:

```json
{
  "oauth2ClientSettings": {
    "clientId": "...",
    "clientSecret": "..."
  },
  "token": {
    "access_token": "...",
    "refresh_token": "...",
    "token_type": "Bearer",
    "expiry_date": 1234567890000
  }
}
```

Also supported: `installed` and `web` credential formats from Google Cloud Console downloads.

#### Token refresh

When `--auth` is used with a file containing a `refresh_token` and OAuth client credentials, glasp automatically refreshes the access token and persists the updated token back to the file. This keeps your credentials valid across sessions without manual intervention.

## Development

```bash
make build              # Build binary to bin/glasp
make install            # Build and install globally
make test               # Run all tests
make clean              # Remove binary and clear test cache

go test ./internal/auth/...          # Test specific package
go test ./cmd/glasp -run TestName    # Run single test
go test -v ./...                     # Full verbose test suite
```
