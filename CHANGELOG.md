# Changelog

All notable changes to this project will be documented in this file.

## [v0.4.1](https://github.com/takihito/glasp/compare/v0.4.0...v0.4.1) - 2026-07-18

- docs: update commit hash pinning to v0.4.0 by @takihito in https://github.com/takihito/glasp/pull/116
- build(deps): bump actions/setup-go from 6.4.0 to 6.5.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/118
- refactor: migrate diagnostic logging to log/slog with structured output by @takihito in https://github.com/takihito/glasp/pull/125
- build(deps): bump goreleaser/goreleaser-action from 7.2.2 to 7.2.3 by @dependabot[bot] in https://github.com/takihito/glasp/pull/119
- build(deps): bump github/codeql-action/upload-sarif from 4.36.2 to 4.36.3 by @dependabot[bot] in https://github.com/takihito/glasp/pull/122
- build(deps): bump google.golang.org/api from 0.285.0 to 0.287.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/121
- build(deps): bump github/codeql-action init/autobuild/analyze to v4.36.3 by @takihito in https://github.com/takihito/glasp/pull/126
- fix(ci): fetch full history in tagpr checkout to avoid unauthenticated unshallow by @takihito in https://github.com/takihito/glasp/pull/127
- docs: add Homebrew installation instructions by @takihito in https://github.com/takihito/glasp/pull/128
- docs: add Homebrew installation instructions to README.ja.md by @takihito in https://github.com/takihito/glasp/pull/129
- build(deps): bump github/codeql-action/upload-sarif from 4.36.3 to 4.37.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/131
- build(deps): bump google.golang.org/api from 0.287.0 to 0.288.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/132
- update README by @takihito in https://github.com/takihito/glasp/pull/137
- build(deps): bump golang.org/x/sys from 0.46.0 to 0.47.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/134
- build(deps): bump step-security/harden-runner from 2.19.4 to 2.20.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/136
- build(deps): bump github/codeql-action to 4.37.0 and group future updates by @takihito in https://github.com/takihito/glasp/pull/138
- build(deps): bump Songmu/tagpr from 1.20.0 to 1.20.1 by @dependabot[bot] in https://github.com/takihito/glasp/pull/139
- fix: accept array-form parentId in .clasp.json for clasp v2 compatibility by @takihito in https://github.com/takihito/glasp/pull/140

## [v0.4.0](https://github.com/takihito/glasp/compare/v0.3.0...v0.4.0) - 2026-06-25

- build(deps): bump github/codeql-action from 4.36.0 to 4.36.1 by @dependabot[bot] in https://github.com/takihito/glasp/pull/99
- fix(ci): change dependency-review egress-policy from block to audit by @takihito in https://github.com/takihito/glasp/pull/103
- build(deps): bump google.golang.org/api from 0.280.0 to 0.283.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/98
- build(deps): bump actions/checkout from 6.0.2 to 6.0.3 by @dependabot[bot] in https://github.com/takihito/glasp/pull/100
- build(deps): bump Songmu/tagpr from 1.19.0 to 1.20.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/101
- docs: update version references to v0.3.0 by @takihito in https://github.com/takihito/glasp/pull/104
- refactor: restructure cmd/glasp and internal packages (Phase 1-6) by @takihito in https://github.com/takihito/glasp/pull/107
- build(deps): bump golang.org/x/sys from 0.45.0 to 0.46.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/105
- build(deps): bump github/codeql-action from 4.36.1 to 4.36.2 by @dependabot[bot] in https://github.com/takihito/glasp/pull/106
- feat: add configurable HTTP timeout for Script API requests by @takihito in https://github.com/takihito/glasp/pull/108
- build(deps): bump github.com/evanw/esbuild from 0.28.0 to 0.28.1 by @dependabot[bot] in https://github.com/takihito/glasp/pull/109
- build(deps): bump google.golang.org/api from 0.283.0 to 0.284.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/110
- feat: add configurable API retry with exponential backoff by @takihito in https://github.com/takihito/glasp/pull/111
- refactor: replace hand-written retryTransport with go-retryablehttp by @takihito in https://github.com/takihito/glasp/pull/112
- bump version to v0.4.0 by @takihito in https://github.com/takihito/glasp/pull/113
- build(deps): bump actions/checkout from 6.0.3 to 7.0.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/114
- build(deps): bump google.golang.org/api from 0.284.0 to 0.285.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/115

## [v0.3.0](https://github.com/takihito/glasp/compare/v0.2.12...v0.3.0) - 2026-06-02
- bump version to v0.3.0 by @takihito in https://github.com/takihito/glasp/pull/94
- implement OAuth PKCE by @takihito in https://github.com/takihito/glasp/pull/81
- add glasp smoke test workflow by @takihito in https://github.com/takihito/glasp/pull/96
- docs: add PKCE documentation and enable smoke test on push by @takihito in https://github.com/takihito/glasp/pull/97

## [v0.2.12](https://github.com/takihito/glasp/compare/v0.2.11...v0.2.12) - 2026-05-27
- add alllow endpoint .github/workflows/release.yml by @takihito in https://github.com/takihito/glasp/pull/91

## [v0.2.11](https://github.com/takihito/glasp/compare/v0.2.10...v0.2.11) - 2026-05-27
- update documents. v0.2.9 -> v0.2.10 by @takihito in https://github.com/takihito/glasp/pull/79
- build(deps): bump google.golang.org/api from 0.279.0 to 0.280.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/83
- build(deps): bump step-security/harden-runner from 2.19.3 to 2.19.4 by @dependabot[bot] in https://github.com/takihito/glasp/pull/84
- build(deps): bump goreleaser/goreleaser-action from 7.2.1 to 7.2.2 by @dependabot[bot] in https://github.com/takihito/glasp/pull/86
- build(deps): bump golang.org/x/sys from 0.44.0 to 0.45.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/85
- build(deps): bump github/codeql-action from 4.35.5 to 4.36.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/87
- deps: bump golang.org/x/crypto to v0.52.0 by @takihito in https://github.com/takihito/glasp/pull/88
- deps: bump golang.org/x/net to v0.55.0 by @takihito in https://github.com/takihito/glasp/pull/89
- Harden CI workflows: switch harden-runner to block mode by @takihito in https://github.com/takihito/glasp/pull/82
- add allowed endpoints. tagpr.yml by @takihito in https://github.com/takihito/glasp/pull/90

## [v0.2.10](https://github.com/takihito/glasp/compare/v0.2.9...v0.2.10) - 2026-05-19
- build(deps): bump goreleaser/goreleaser-action from 7.1.0 to 7.2.1 by @dependabot[bot] in https://github.com/takihito/glasp/pull/62
- docs: add glasp vs clasp CI benchmark results to GitHub Actions docs by @takihito in https://github.com/takihito/glasp/pull/64
- docs: use comparison framing in CI benchmark section by @takihito in https://github.com/takihito/glasp/pull/68
- build(deps): bump github/codeql-action from 4.35.2 to 4.35.3 by @dependabot[bot] in https://github.com/takihito/glasp/pull/65
- build(deps): bump google.golang.org/api from 0.276.0 to 0.277.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/66
- build(deps): bump step-security/harden-runner from 2.19.0 to 2.19.1 by @dependabot[bot] in https://github.com/takihito/glasp/pull/67
- add pin to documents by @takihito in https://github.com/takihito/glasp/pull/69
- build(deps): bump google.golang.org/api from 0.277.0 to 0.278.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/70
- build(deps): bump golang.org/x/sys from 0.43.0 to 0.44.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/71
- build(deps): bump github/codeql-action from 4.35.3 to 4.35.4 by @dependabot[bot] in https://github.com/takihito/glasp/pull/72
- build(deps): bump Songmu/tagpr from 1.18.3 to 1.19.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/74
- build(deps): bump actions/dependency-review-action from 4.9.0 to 5.0.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/73
- build(deps): bump sigstore/cosign-installer from 4.1.1 to 4.1.2 by @dependabot[bot] in https://github.com/takihito/glasp/pull/75
- build(deps): bump google.golang.org/api from 0.278.0 to 0.279.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/76
- build(deps): bump github/codeql-action from 4.35.4 to 4.35.5 by @dependabot[bot] in https://github.com/takihito/glasp/pull/77
- build(deps): bump step-security/harden-runner from 2.19.1 to 2.19.3 by @dependabot[bot] in https://github.com/takihito/glasp/pull/78

## [v0.2.9](https://github.com/takihito/glasp/compare/v0.2.8...v0.2.9) - 2026-04-21
- update README, docs/github-actions.md by @takihito in https://github.com/takihito/glasp/pull/53
- docs _config.yml render_with_liquid: false by @takihito in https://github.com/takihito/glasp/pull/60
- build(deps): bump google.golang.org/api from 0.275.0 to 0.276.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/56
- build(deps): bump github/codeql-action from 4.35.1 to 4.35.2 by @dependabot[bot] in https://github.com/takihito/glasp/pull/55
- build(deps): bump step-security/harden-runner from 2.17.0 to 2.19.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/57
- added raw tag. change remote_theme by @takihito in https://github.com/takihito/glasp/pull/61
- build(deps): bump goreleaser/goreleaser-action from 7.0.0 to 7.1.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/58
- build(deps): bump Songmu/tagpr from 1.18.2 to 1.18.3 by @dependabot[bot] in https://github.com/takihito/glasp/pull/59

## [v0.2.8](https://github.com/takihito/glasp/compare/v0.2.7...v0.2.8) - 2026-04-20
- feat: add client-id and client-secret inputs to action.yml by @takihito in https://github.com/takihito/glasp/pull/50
- docs: sync EN docs with updated JA docs and README.ja.md by @takihito in https://github.com/takihito/glasp/pull/52

## [v0.2.7](https://github.com/takihito/glasp/compare/v0.2.6...v0.2.7) - 2026-04-16
- Bump google.golang.org/api from 0.274.0 to 0.275.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/40
- Bump Songmu/tagpr from 1.17.1 to 1.18.2 by @dependabot[bot] in https://github.com/takihito/glasp/pull/41
- Bump golang.org/x/sys from 0.42.0 to 0.43.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/42
- Bump actions/upload-artifact from 4.6.2 to 7.0.1 by @dependabot[bot] in https://github.com/takihito/glasp/pull/43
- Bump step-security/harden-runner from 2.16.1 to 2.17.0 by @dependabot[bot] in https://github.com/takihito/glasp/pull/44
- feat: GitHub Actions support (action.yml + GLASP_AUTH env var) by @takihito in https://github.com/takihito/glasp/pull/47
- fix: remove expression syntax from action.yml description field by @takihito in https://github.com/takihito/glasp/pull/48

## [v0.2.6](https://github.com/takihito/glasp/compare/v0.2.5...v0.2.6) - 2026-04-12
- updare Readme and docs by @takihito in https://github.com/takihito/glasp/pull/37

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
