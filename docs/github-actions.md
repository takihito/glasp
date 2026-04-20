---
layout: default
title: GitHub Actions
description: Using glasp in GitHub Actions workflows to automate Google Apps Script deployments.
---

# GitHub Actions

glasp provides a composite action that lets you install glasp and authenticate directly inside a GitHub Actions workflow — no manual binary download or login steps required.

## Action inputs

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `version` | No | latest | glasp version to install (e.g. `v0.2.7`). Omit to use the latest release. |
| `auth` | No | | JSON content of `.clasprc.json`. Pass a repository secret here. When provided, sets the `GLASP_AUTH` environment variable for subsequent steps. |
| `working-directory` | No | | Directory containing `.clasp.json`, relative to workspace root. When provided, sets the `GLASP_DIR` environment variable so that all subsequent `glasp` commands run from that directory. |
| `client-id` | No | | OAuth2 client ID. Pass a repository secret here. When provided, sets the `GLASP_CLIENT_ID` environment variable. Must be set together with `client-secret`. |
| `client-secret` | No | | OAuth2 client secret. Pass a repository secret here. When provided, sets the `GLASP_CLIENT_SECRET` environment variable. Must be set together with `client-id`. |

## Setup

### 1. Obtain your credentials

To authenticate glasp in GitHub Actions, run `glasp login` or `clasp login` on your local machine. These commands save credentials to `.glasp/access.json` and `~/.clasprc.json` respectively.

Log in on your local machine:

```bash
glasp login
cat .glasp/access.json
```

```bash
clasp login
cat ~/.clasprc.json
```

You can also load the content into a shell variable and run glasp directly:

```bash
export GLASP_AUTH=$(cat ~/.clasprc.json) && glasp push    # from clasp login
```

### 2. Add a repository secret

Copy the entire JSON content of `.glasp/access.json` or `~/.clasprc.json` and add it as a repository secret named `GLASP_AUTH`:

**GitHub → Repository → Settings → Secrets and variables → Actions → New repository secret**

- Name: `GLASP_AUTH`
- Value: *(paste the JSON content)*

### 3. Add the action to your workflow

```yaml
steps:
  - uses: actions/checkout@v4
  - uses: takihito/glasp@v0.2.8
    with:
      version: 'v0.2.8'
      auth: '${{ secrets.CLASPRC_JSON }}'  # pass the registered secret to glasp
  - run: glasp push
```

## Examples

### Push on every commit to main

```yaml
name: Deploy to Google Apps Script

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: takihito/glasp@v0.2.8
        with:
          version: 'v0.2.8'
          auth: '${{ secrets.CLASPRC_JSON }}'  # pass the registered secret to glasp

      - name: Push to Apps Script
        run: glasp push
```

### Push and create a deployment

```yaml
name: Deploy to Google Apps Script

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: takihito/glasp@v0.2.8
        with:
          version: 'v0.2.8'
          auth: '${{ secrets.CLASPRC_JSON }}'  # pass the registered secret

      - name: Push files
        run: glasp push

      - name: Create deployment
        run: glasp create-deployment --description "Deploy from CI"
```

### Project

glasp automatically detects `.ts` files according to your `.clasp.json` settings and transpiles them via esbuild before pushing. No additional configuration is needed:

```yaml
- uses: takihito/glasp@v0.2.8
  with:
    version: 'v0.2.8'
    auth: '${{ secrets.CLASPRC_JSON }}'
    working-directory: 'apps-script/dir' # directory containing .clasp.json / workspace root is used if omitted
    client-id: ${{ secrets.GLASP_CLIENT_ID }}         # Optional: specify OAuth2 client ID
    client-secret: ${{ secrets.GLASP_CLIENT_SECRET }} # Optional: specify OAuth2 client secret

- name: Push project
  run: glasp push
```

## How authentication works

When `auth` is set, the action exports its value as the `GLASP_AUTH` environment variable. glasp reads this variable and uses the JSON content directly as credentials — no file on disk, no `glasp login` step required. Auth source priority inside glasp:

1. `--auth <path>` flag
2. `GLASP_AUTH` environment variable ← set by this action
3. Project cache (`.glasp/access.json`)

To populate `GLASP_AUTH`, copy the JSON content of `.glasp/access.json` (from `glasp login`) or `~/.clasprc.json` (from `clasp login`) into a repository secret.

When `client-id` and `client-secret` are set, the action also exports `GLASP_CLIENT_ID` and `GLASP_CLIENT_SECRET`, allowing glasp's OAuth flow to use custom credentials instead of the built-in defaults.

## Monorepo / subdirectory projects

If your `.clasp.json` lives in a subdirectory (e.g. a monorepo), use the `working-directory` input:

```yaml
- uses: takihito/glasp@v0.2.8
  with:
    version: 'v0.2.8'
    auth: '${{ secrets.CLASPRC_JSON }}'
    working-directory: 'apps-script/dir'   # contains .clasp.json
```

This sets `GLASP_DIR=<absolute path>` as an environment variable. Every subsequent `glasp` command picks it up automatically — no `--dir` flag or `working-directory:` needed on each step.

You can also set it per-command with the `--dir` flag or the `GLASP_DIR` environment variable directly:

```yaml
- run: glasp push
  env:
    GLASP_DIR: apps-script/dir

# or equivalently:
- run: glasp --dir apps-script/dir push
```

## Version pinning

Specify an explicit version to make your workflow reproducible:

```yaml
- uses: takihito/glasp@v0.2.7   # recommended: pin to a release tag
  with:
    version: 'v0.2.7'
```

GitHub Release artifacts are immutable, so pinning `version` guarantees the exact same binary is installed on every run.

You can also pin the action itself by commit SHA for stricter supply-chain control:

```yaml
- uses: takihito/glasp@1ae5afb   # pin to a specific commit
  with:
    version: 'v0.2.8'
```
