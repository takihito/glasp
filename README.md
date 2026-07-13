# glasp

[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/takihito/glasp/badge)](https://scorecard.dev/viewer/?uri=github.com/takihito/glasp)

A CLI tool for developing, managing, and deploying GAS (Google Apps Script) code locally.

A single-binary alternative that replaces Node.js-based [clasp](https://github.com/google/clasp), with full clasp compatibility.

**Documentation:** [https://takihito.github.io/glasp/](https://takihito.github.io/glasp/)

## Features

- Full clasp compatibility (`.clasp.json`, `.claspignore`, `.clasprc.json`)
- TypeScript auto-transpilation on push (with or without `fileExtension` setting)
- OAuth2 authentication with local callback server
- Command execution history with replay support
- Archive support for push/pull operations

## Installation

### Quick Install (recommended)

**Linux / macOS:**

```bash
curl -sSL https://takihito.github.io/glasp/install.sh | sh
```

**Windows (PowerShell):**

```powershell
irm https://takihito.github.io/glasp/install.ps1 | iex
```

Installs to `~/.local/bin` by default (Linux/macOS). No `sudo` required. If `~/.local/bin` is not in your PATH, add it to your shell profile (`~/.bashrc`, `~/.zshrc`):

```bash
export PATH="$HOME/.local/bin:$PATH"
```

> **Windows:** The PowerShell installer adds `%LOCALAPPDATA%\glasp\bin` to PATH automatically.

To change the install directory:

```bash
curl -sSL https://takihito.github.io/glasp/install.sh | GLASP_INSTALL_DIR=/usr/local/bin sh
```

### Homebrew (macOS / Linux)

```bash
brew tap takihito/tap
brew trust --formula takihito/tap/glasp
brew install glasp
```

> `brew trust` is required because [takihito/tap](https://github.com/takihito/homebrew-tap) is a non-official tap. Installs a pre-built binary with OAuth credentials embedded.

### go install

```bash
go install github.com/takihito/glasp/cmd/glasp@latest
```

> This method does not embed OAuth credentials. Set `GLASP_CLIENT_ID` and `GLASP_CLIENT_SECRET` environment variables.

### Build from source

```bash
git clone https://github.com/takihito/glasp.git
cd glasp
make build    # Build binary to bin/glasp
make install  # Build and install globally
```

### OAuth credentials

| Install method | Credentials |
|---------------|-------------|
| Quick Install / Homebrew (pre-built binaries) | Embedded, override with env vars |
| `go install` / source build | Env vars required |

```bash
export GLASP_CLIENT_ID="your-client-id"
export GLASP_CLIENT_SECRET="your-client-secret"
```

Environment variables take precedence over embedded credentials. See [Google Cloud Console](https://console.cloud.google.com/apis/credentials) to create OAuth 2.0 credentials for a Desktop application.

## Quick Start

```bash
# Install (Linux / macOS)
curl -sSL https://takihito.github.io/glasp/install.sh | sh

# Login to Google account
glasp login

# Or, if you already have clasp credentials:
glasp login --auth ~/.clasprc.json

# Clone an existing project
glasp clone <script-id>

# Pull remote files
glasp pull

# Push local files
glasp push
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

### PKCE (Proof Key for Code Exchange)

The interactive login flow optionally supports PKCE (RFC 7636) to harden the OAuth2 authorization code exchange against code interception attacks. PKCE is opt-in:

```bash
# Enable via CLI flag
glasp login --pkce

# Or enable via environment variable
GLASP_USE_PKCE=1 glasp login
```

glasp generates a cryptographic `code_verifier` per login, sends an `S256` `code_challenge` to Google, and provides the verifier at token exchange. The verifier itself is never logged or persisted. PKCE coexists with `client_secret`; existing credentials continue to work.

PKCE applies only to the interactive `glasp login` flow.

### Using `--auth` with `.clasprc.json`

If you already use clasp, you likely have a `~/.clasprc.json` file containing your OAuth credentials. The `--auth` option lets you reuse this file directly with glasp — **no need to go through the interactive login flow**.

```bash
# Import clasp credentials into glasp (saves to .glasp/access.json)
glasp login --auth ~/.clasprc.json

# Or use --auth directly on each command without login
glasp push --auth ~/.clasprc.json
glasp pull --auth ~/.clasprc.json
glasp clone SCRIPT_ID --auth ~/.clasprc.json

# You can also pass a directory — glasp will look for .clasprc.json inside it
glasp push --auth ~/
```

`glasp login --auth` imports the credentials from `.clasprc.json` into glasp's project cache (`.glasp/access.json`), so subsequent commands work without specifying `--auth` each time. If you prefer not to import, you can pass `--auth` directly on individual commands instead.

This is especially useful when:

- **Migrating from clasp to glasp** — start using glasp immediately without re-authenticating
- **CI/CD pipelines** — share a single `.clasprc.json` across clasp and glasp workflows
- **Multiple Google accounts** — keep separate `.clasprc.json` files per account and switch with `--auth`

The `--auth` option is available on all commands that require authentication: `login`, `push`, `pull`, `clone`, `create-script`, `create-deployment`, `update-deployment`, `list-deployments`, and `run-function`.

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

## GitHub Actions

glasp provides a composite action to install and authenticate inside a GitHub Actions workflow.

### Setup

- Obtain credentials locally:
  - Run `clasp login` and copy the contents of `~/.clasprc.json`, or
  - Run `glasp login` and copy the contents of `.glasp/access.json`
  ```bash
  GLASP_AUTH=$(cat .glasp/access.json)   # glasp login
  GLASP_AUTH=$(cat ~/.clasprc.json)      # clasp login
  ```
- Add it as a repository secret named `GLASP_AUTH` (**Settings → Secrets and variables → Actions**)

### Usage

```yaml
steps:
  - uses: actions/checkout@v4
  - uses: takihito/glasp@v0.4.0
    with:
      version: 'v0.4.0'
      auth: '${{ secrets.CLASPRC_JSON }}'  # pass the registered secret
  - run: glasp push
```

When `auth` is provided, glasp automatically picks it up via the `GLASP_AUTH` environment variable — no `--auth` flag needed on each command.

Auth source priority: `--auth` flag → `GLASP_AUTH` env var → project cache

If `.clasp.json` is in a subdirectory, use the `working-directory` input (sets `GLASP_DIR`):

```yaml
- uses: takihito/glasp@v0.4.0
  with:
    version: 'v0.4.0'
    auth: '${{ secrets.CLASPRC_JSON }}'
    working-directory: 'apps-script/dir' # directory containing .clasp.json (optional)
    client-id: ${{ secrets.GLASP_CLIENT_ID }}         # Optional: specify OAuth2 client ID
    client-secret: ${{ secrets.GLASP_CLIENT_SECRET }} # Optional: specify OAuth2 client secret
```

See the [GitHub Actions documentation](https://takihito.github.io/glasp/github-actions) for full examples including deployments and TypeScript projects.

### Hardening egress with step-security/harden-runner

When running glasp inside a workflow, pair it with [`step-security/harden-runner`](https://github.com/step-security/harden-runner) in `block` mode to restrict outbound traffic to only the endpoints glasp actually needs:

```yaml
steps:
  - name: Harden the runner
    uses: step-security/harden-runner@v2
    with:
      egress-policy: block
      allowed-endpoints: >
        api.github.com:443
        github.com:443
        objects.githubusercontent.com:443
        script.googleapis.com:443
        www.googleapis.com:443
        oauth2.googleapis.com:443

  - uses: actions/checkout@v4
  - uses: takihito/glasp@v0.4.0
    with:
      auth: ${{ secrets.GLASP_AUTH }}
  - run: glasp push
```

Endpoint reference:

| Endpoint | Why glasp needs it |
| --- | --- |
| `api.github.com:443` | The action installer queries the latest release metadata. |
| `github.com:443`, `objects.githubusercontent.com:443` | Downloading the released `glasp` binary. |
| `script.googleapis.com:443` | Apps Script API (`push`, `pull`, deployments, `run-function`). |
| `www.googleapis.com:443` | Drive scope used during project creation/clone. |
| `oauth2.googleapis.com:443` | Refreshing the OAuth access token. |

## Environment Variables

| Variable | Equivalent flag | Description |
| --- | --- | --- |
| `GLASP_DIR` | `-C`, `--dir` | Change to this directory before executing any command. |
| `GLASP_USE_PKCE` | `--pkce` (on `login`) | Enable PKCE for the interactive OAuth login flow. Accepts `1`, `true`, etc. (parsed by kong). |
| `GLASP_AUTH` | `--auth` | Raw `.clasprc.json` content. Used by CI workflows where mounting a file is inconvenient. |
| `GLASP_CLIENT_ID` | — | OAuth client ID. Overrides the value baked in at build time via `-ldflags`. |
| `GLASP_CLIENT_SECRET` | — | OAuth client secret. Overrides the value baked in at build time via `-ldflags`. |

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
