# Changelog

All notable changes to this project will be documented in this file.

## [v0.2.5](https://github.com/takihito/glasp/compare/v0.2.4...v0.2.5) - 2026-04-11
- PATH 設定手順をドキュメントに追記 by @takihito in https://github.com/takihito/glasp/pull/35

## [v0.2.4](https://github.com/takihito/glasp/compare/v0.2.3...v0.2.4) - 2026-04-11
- Immutable Releases 対応: ドラフト維持 + provenance 後に公開 by @takihito in https://github.com/takihito/glasp/pull/33

## [v0.2.3](https://github.com/takihito/glasp/compare/v0.2.2...v0.2.3) - 2026-04-11
- Fix SLSA workflow ref format (@v2.1.0) by @takihito in https://github.com/takihito/glasp/pull/31

## [v0.2.2](https://github.com/takihito/glasp/compare/v0.2.1...v0.2.2) - 2026-04-11
- Fix SLSA provenance: タグ参照に変更 by @takihito in https://github.com/takihito/glasp/pull/29

## [v0.2.1](https://github.com/takihito/glasp/compare/v0.2.0...v0.2.1) - 2026-04-11
- Fix SLSA provenance: private-repository 誤検知を修正 by @takihito in https://github.com/takihito/glasp/pull/27

## [v0.2.0](https://github.com/takihito/glasp/compare/v0.1.2...v0.2.0) - 2026-04-10
- GitHub Pages サイトを追加 (Cayman テーマ) by @takihito in https://github.com/takihito/glasp/pull/23
- Add one-liner install scripts for Linux/macOS/Windows by @takihito in https://github.com/takihito/glasp/pull/25
- インストールスクリプトの改善 (sudo不要化・macOS対応) by @takihito in https://github.com/takihito/glasp/pull/26

## [v0.1.2](https://github.com/takihito/glasp/compare/v0.1.1...v0.1.2) - 2026-04-09
- Add OpenSSF Scorecard workflow and badge by @takihito in https://github.com/takihito/glasp/pull/16
- Fix upload-artifact commit hash in scorecard workflow by @takihito in https://github.com/takihito/glasp/pull/18
- Improve Signed-Releases scorecard with sigstore and SLSA provenance by @takihito in https://github.com/takihito/glasp/pull/19
- GoReleaser に use_existing_draft を追加 (Immutable Releases 対応) by @takihito in https://github.com/takihito/glasp/pull/21
- tagpr で PAT を使用してワークフロートリガーを有効化 by @takihito in https://github.com/takihito/glasp/pull/22

## [v0.1.1](https://github.com/takihito/glasp/compare/v0.1.0...v0.1.1) - 2026-04-08
- Fix cosign signing to use bundle format by @takihito in https://github.com/takihito/glasp/pull/14

## [v0.1.0](https://github.com/takihito/glasp/commits/v0.1.0) - 2026-04-08
- Start Project  by @takihito in https://github.com/takihito/glasp/pull/1
- Prepare for v0.1.0 public release by @takihito in https://github.com/takihito/glasp/pull/2
- Bump google.golang.org/grpc from 1.78.0 to 1.79.3 in the go_modules group across 1 directory by @dependabot[bot] in https://github.com/takihito/glasp/pull/3
- Bump github.com/alecthomas/kong from 1.13.0 to 1.15.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/4
- Bump actions/setup-go from 6.2.0 to 6.4.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/5
- Bump actions/dependency-review-action from 4.8.3 to 4.9.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/7
- Bump github.com/evanw/esbuild from 0.27.3 to 0.28.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/6
- Bump google.golang.org/api from 0.260.0 to 0.274.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/8
- Bump github/codeql-action from 4.32.5 to 4.35.1 by @dependabot[bot] in https://github.com/takihito/glasp/pull/10
- Bump sigstore/cosign-installer from 4.1.0 to 4.1.1 by @dependabot[bot] in https://github.com/takihito/glasp/pull/11
- Bump step-security/harden-runner from 2.16.0 to 2.16.1 by @dependabot[bot] in https://github.com/takihito/glasp/pull/12

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
