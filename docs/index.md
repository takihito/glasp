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
# インストール（Linux / macOS）
curl -sSL https://takihito.github.io/glasp/install.sh | sh

# ログイン
glasp login

# 既存プロジェクトのクローン
glasp clone <script-id>

# ファイルの取得・反映
glasp pull
glasp push
```

デフォルトでは `~/.local/bin` にインストールされます。変更する場合:

```bash
GLASP_INSTALL_DIR=/usr/local/bin curl -sSL https://takihito.github.io/glasp/install.sh | sh
```

> **Windows:** `irm https://takihito.github.io/glasp/install.ps1 | iex` でインストールできます。

詳しくは [Installation](installation) と [Usage](usage) をご覧ください。

## Supply-Chain Security

glasp はサプライチェーンセキュリティを重視しています。

- リリースバイナリは [cosign](https://github.com/sigstore/cosign) で署名済み
- [SLSA Level 3](https://slsa.dev/) の来歴証明 (provenance) を付与
- [OpenSSF Scorecard](https://scorecard.dev/viewer/?uri=github.com/takihito/glasp) でセキュリティスコアを公開
- 全 GitHub Actions をコミットハッシュで固定
