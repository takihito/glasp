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
| `version` | No | latest | glasp version to install (e.g. `v1.2.0`). Omit to use the latest release. |
| `auth` | No | | JSON content of `.clasprc.json`. Pass a repository secret here. When provided, sets the `GLASP_AUTH` environment variable for subsequent steps. |

## Setup

### 1. Obtain `.clasprc.json`

Log in with glasp (or clasp) on your local machine:

```bash
glasp login
cat ~/.clasprc.json
```

### 2. Add a repository secret

Copy the entire JSON content of `~/.clasprc.json` and add it as a repository secret named `GLASP_AUTH`:

**GitHub → Repository → Settings → Secrets and variables → Actions → New repository secret**

- Name: `GLASP_AUTH`
- Value: *(paste the JSON content)*

### 3. Add the action to your workflow

```yaml
steps:
  - uses: actions/checkout@v4
  - uses: takihito/glasp@v1.2.0
    with:
      version: 'v1.2.0'
      auth: ${{ secrets.GLASP_AUTH }}
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

      - uses: takihito/glasp@v1.2.0
        with:
          version: 'v1.2.0'
          auth: ${{ secrets.GLASP_AUTH }}

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

      - uses: takihito/glasp@v1.2.0
        with:
          version: 'v1.2.0'
          auth: ${{ secrets.GLASP_AUTH }}

      - name: Push files
        run: glasp push

      - name: Create deployment
        run: glasp create-deployment --description "Deploy from CI"
```

### TypeScript project

glasp automatically detects `.ts` files and transpiles them via esbuild before pushing. No additional configuration is needed:

```yaml
- uses: takihito/glasp@v1.2.0
  with:
    version: 'v1.2.0'
    auth: ${{ secrets.GLASP_AUTH }}

- name: Push TypeScript project
  run: glasp push
```

## How authentication works

When `auth` is set, the action exports its value as the `GLASP_AUTH` environment variable. glasp reads this variable and uses the JSON content directly as credentials — no file on disk, no `glasp login` step required. Auth source priority inside glasp:

1. `--auth <path>` flag
2. `GLASP_AUTH` environment variable ← set by this action
3. Project cache (`.glasp/access.json`)

## Version pinning

Specify an explicit version to make your workflow reproducible:

```yaml
- uses: takihito/glasp@v1.2.0   # recommended: pin to a release tag
  with:
    version: 'v1.2.0'
```

GitHub Release artifacts are immutable, so pinning `version` guarantees the exact same binary is installed on every run.

You can also pin the action itself by commit SHA for stricter supply-chain control:

```yaml
- uses: takihito/glasp@1ae5afb   # pin to a specific commit
  with:
    version: 'v1.2.0'
```
