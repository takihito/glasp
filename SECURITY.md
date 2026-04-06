# Security Policy

## Supported Versions

| Version | Supported          |
|---------|--------------------|
| Latest  | :white_check_mark: |
| Older   | :x:                |

Only the latest minor release receives security updates.

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, use [GitHub Security Advisories](https://github.com/takihito/glasp/security/advisories/new) to report vulnerabilities privately.

Please include:

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

You should receive a response within 7 days. If the vulnerability is confirmed, a fix will be released as soon as possible.

## Scope

### In scope

- CLI command execution and argument handling
- OAuth2 authentication flow and token management
- File read/write operations (`.clasp.json`, `.glasp/`, token cache)
- Google Apps Script API interactions
- TypeScript transpilation via esbuild

### Out of scope

- Misconfiguration of `.clasp.json` or `.claspignore`
- Compromised Google OAuth tokens obtained outside of glasp
- Issues in upstream dependencies (report to the respective project)
- Social engineering attacks

## Secure Development

- OAuth client secrets are never cached locally (only tokens)
- Token cache files use `0600` permissions (user-only)
- `.glasp/` directory uses `0700` permissions
- Sensitive CLI arguments are redacted in command history
- Atomic file writes with backup for token persistence
- Path traversal protection in file sync operations
