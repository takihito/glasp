# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

## [v0.1.0] - 2026-04-06

### Added

- **Core commands**: login, logout, create-script, clone, pull, push, open-script
- **Deployment commands**: create-deployment, update-deployment, list-deployments
- **Utility commands**: run-function, convert, history, config init, version
- **TypeScript support**: Auto-transpile `.ts` files on push via esbuild, GAS-to-TS conversion on pull
- **OAuth2 authentication**: Browser-based login with local callback server, token caching and auto-refresh
- **clasp compatibility**: `.clasp.json`, `.claspignore`, `.clasprc.json` support (drop-in replacement)
- **Command history**: JSON Lines format with filtering, ordering, and push replay
- **Archive support**: Configurable push/pull archiving under `.glasp/archive/`
- **Cross-platform**: Darwin, Linux, Windows (amd64, arm64)
- **Security**: Atomic token writes with backup, sensitive arg redaction in history, path traversal protection

### Security

- Short-form CLI flags (e.g. `-p`) are now redacted in command history
- Atomic file write uses backup-rename-cleanup pattern to prevent data loss

[Unreleased]: https://github.com/takihito/glasp/compare/v0.1.0...HEAD
[v0.1.0]: https://github.com/takihito/glasp/releases/tag/v0.1.0
