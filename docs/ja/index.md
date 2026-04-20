---
layout: default
title: glasp - Google Apps Script CLI in Go
description: glasp is a Go-based CLI tool for managing Google Apps Script projects, fully compatible 
---

# glasp

[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/takihito/glasp/badge)](https://scorecard.dev/viewer/?uri=github.com/takihito/glasp)

glasp は Google Apps Script プロジェクトを管理する Go 製 CLI ツールです。
Node.js ベースの [clasp](https://github.com/google/clasp) をシングルバイナリで置き換え、高速に動作します。

[English](../)

## Features

- **clasp 完全互換** — `.clasp.json`, `.claspignore`, `.clasprc.json` をそのまま利用可能できます
- **TypeScript 自動トランスパイル** — push 時に `.ts` ファイルを自動変換します
- **OAuth2 認証** — ローカルコールバックサーバーを利用したログイン　
- **コマンド履歴** — 実行履歴の記録とリプレイ機能
- **アーカイブ** — push/pull 操作のスナップショット保存
- **シングルバイナリ** — ダウンロードしてすぐ使用可能です

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
curl -sSL https://takihito.github.io/glasp/install.sh | GLASP_INSTALL_DIR=/usr/local/bin sh
```

> **Windows:** `irm https://takihito.github.io/glasp/install.ps1 | iex` でインストールできます。

詳しくは [インストール](installation)、[使い方](usage)、[GitHub Actions](github-actions) をご覧ください。

## Supply-Chain Security

- リリースバイナリは [cosign](https://github.com/sigstore/cosign) で署名済み
- [SLSA Level 3](https://slsa.dev/) の来歴証明 (provenance) を付与
- [https://socket.dev/](https://socket.dev/) で依存関係の分析と監視を行っています
- [OpenSSF Scorecard](https://scorecard.dev/viewer/?uri=github.com/takihito/glasp) でセキュリティスコアを公開しています
