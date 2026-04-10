---
layout: default
title: Home
---

# glasp

[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/takihito/glasp/badge)](https://scorecard.dev/viewer/?uri=github.com/takihito/glasp)

**glasp** は Google Apps Script プロジェクトを管理する Go 製 CLI ツールです。
Node.js ベースの [clasp](https://github.com/google/clasp) をシングルバイナリで置き換え、高速に動作します。

## Features

- **clasp 完全互換** — `.clasp.json`, `.claspignore`, `.clasprc.json` をそのまま利用可能
- **TypeScript 自動トランスパイル** — push 時に `.ts` ファイルを自動変換
- **OAuth2 認証** — ローカルコールバックサーバーによるスムーズなログイン
- **コマンド履歴** — 実行履歴の記録とリプレイ機能
- **アーカイブ** — push/pull 操作のスナップショット保存
- **シングルバイナリ** — インストール不要、ダウンロードしてすぐ使える

## Quick Start

```bash
# インストール
go install github.com/takihito/glasp/cmd/glasp@latest

# OAuth credentials を設定（go install の場合は必須）
export GLASP_CLIENT_ID="your-client-id"
export GLASP_CLIENT_SECRET="your-client-secret"

# ログイン
glasp login

# 既存プロジェクトのクローン
glasp clone <script-id>

# ファイルの取得・反映
glasp pull
glasp push
```

> **Note:** [Releases](https://github.com/takihito/glasp/releases) ページの Pre-built binary には OAuth credentials が埋め込み済みのため、環境変数の設定は不要です。

詳しくは [Installation](installation) と [Usage](usage) をご覧ください。

## Supply-Chain Security

glasp はサプライチェーンセキュリティを重視しています。

- リリースバイナリは [cosign](https://github.com/sigstore/cosign) で署名済み
- [SLSA Level 3](https://slsa.dev/) の来歴証明 (provenance) を付与
- [OpenSSF Scorecard](https://scorecard.dev/viewer/?uri=github.com/takihito/glasp) でセキュリティスコアを公開
- 全 GitHub Actions をコミットハッシュで固定
