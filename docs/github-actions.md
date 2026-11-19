---
layout: default
title: GitHub Actions
description: Using glasp in GitHub Actions workflows to automate Google Apps Script deployments.
---

# GitHub Actions

glasp provides a composite action that lets you install glasp and authenticate directly inside a GitHub Actions workflow — no manual binary download or login steps required.

## Performance comparison: glasp and clasp in CI

Because glasp is a single precompiled Go binary, setup in GitHub Actions is dramatically faster than installing clasp via npm. The table below shows benchmark results measured on `ubuntu-latest` (glasp v0.4.0 / @google/clasp 3.3.0, push/pull averaged over 5 runs):

| Metric | glasp | clasp | Ratio (glasp / clasp) |
|:-------|------:|------:|:---------------------:|
| Setup Time | 1337ms | 19150ms | **14.3x** |
| Push Time (avg) | 1015ms | 1229ms | **1.2x** |
| Pull Time (avg) | 359ms | 1270ms | **3.5x** |
| **Total Time** | **2711ms** | **21649ms** | **7.9x** |

The biggest win is setup: clasp requires installing the `@google/clasp` npm package globally on every run (~19 seconds), whereas glasp fetches a single precompiled binary (~1.3 seconds). End-to-end, glasp completes the full setup → push → pull cycle **7.9× faster**, which adds up quickly in high-frequency deployment pipelines.

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
  - uses: takihito/glasp@v0.5.0
    with:
      version: 'v0.5.0'
{% raw %}
      auth: '${{ secrets.GLASP_AUTH }}'  # pass the registered secret to glasp
{% endraw %}
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

      - uses: takihito/glasp@v0.5.0
        with:
          version: 'v0.5.0'
{% raw %}
          auth: '${{ secrets.GLASP_AUTH }}'  # pass the registered secret to glasp
{% endraw %}

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

      - uses: takihito/glasp@v0.5.0
        with:
          version: 'v0.5.0'
{% raw %}
          auth: '${{ secrets.GLASP_AUTH }}'  # pass the registered secret
{% endraw %}

      - name: Push files
        run: glasp push

      - name: Create deployment
        run: glasp create-deployment --description "Deploy from CI"
```

### Project

glasp automatically detects `.ts` files according to your `.clasp.json` settings and transpiles them via esbuild before pushing. No additional configuration is needed:

```yaml
- uses: takihito/glasp@v0.5.0
  with:
    version: 'v0.5.0'
{% raw %}
    auth: '${{ secrets.GLASP_AUTH }}'
    working-directory: 'apps-script/dir' # directory containing .clasp.json / workspace root is used if omitted
    client-id: ${{ secrets.GLASP_CLIENT_ID }}         # Optional: specify OAuth2 client ID
    client-secret: ${{ secrets.GLASP_CLIENT_SECRET }} # Optional: specify OAuth2 client secret
{% endraw %}

- name: Push project
  run: glasp push
```

## How authentication works

When `auth` is set, the action exports its value as the `GLASP_AUTH` environment variable. glasp reads this variable and uses the JSON content directly as credentials — no file on disk, no `glasp login` step required. Auth source priority inside glasp:

1. `--auth <path>` flag
2. `GLASP_AUTH` environment variable ← set by this GitHub Action
3. Project cache (`.glasp/access.json`)

To populate `GLASP_AUTH`, copy the JSON content of `.glasp/access.json` (from `glasp login`) or `~/.clasprc.json` (from `clasp login`) into a repository secret.

When `client-id` and `client-secret` are set, the action also exports `GLASP_CLIENT_ID` and `GLASP_CLIENT_SECRET`, allowing glasp's OAuth flow to use custom credentials instead of the built-in defaults.

## Security considerations

`GLASP_AUTH` contains a long-lived OAuth refresh token. Anyone who obtains it can read and modify every Apps Script project the Google account can edit, so treat it like a deploy key.

- **Log masking**: GitHub [does not reliably redact structured data such as JSON](https://docs.github.com/en/actions/reference/security/secrets) when it is stored as a single secret. The action therefore registers each token and client secret value inside the JSON with `::add-mask::`. Never print `GLASP_AUTH` yourself.
- **Visible to later steps**: The `auth` input exports `GLASP_AUTH` through `GITHUB_ENV`, so every later step in the same job, including third-party actions, can read it. Every step in a job that uses the secret must be trusted, however you pass it. To reduce accidental exposure, you can omit `auth` and pass the secret only to the steps that run glasp (the per-value masks above are not registered in this pattern; glasp itself never prints credentials). This does not make untrusted steps safe: such a step could, for example, replace the glasp binary installed under `runner.temp` and capture the credentials when glasp runs. Run untrusted code in a separate job on a fresh GitHub-hosted runner. Passing the secret only to the glasp step looks like this:

  ```yaml
  - uses: takihito/glasp@v0.5.0
    with:
      version: 'v0.5.0'
  - name: Push to Apps Script
    run: glasp push
    env:
{% raw %}
      GLASP_AUTH: ${{ secrets.GLASP_AUTH }}
{% endraw %}
  ```

- **Triggers**: Do not expose the secret to workflows that run untrusted code, such as `pull_request_target` or `workflow_run` triggered from forks. Prefer `push` to protected branches and `workflow_dispatch`.
- **Least privilege**: Set `permissions: contents: read` and check out with `persist-credentials: false`. Pin third-party actions to a commit SHA.
- **Environment protection**: Store the secret in a [GitHub Environment](https://docs.github.com/en/actions/how-tos/deploy/configure-and-manage-deployments/manage-environments) limited to your deployment branch (optionally with required reviewers) instead of as a repository-wide secret, and reference it with `environment:` in the job.
- **Dedicated account**: Log in with a Google account dedicated to CI that can edit only the target projects.
- **Revocation**: If the secret may have leaked, revoke the refresh token by removing access for the OAuth app that issued it in your Google Account (Security → Third-party apps & services). That is the OAuth client used at login: glasp's built-in client for `glasp login`, clasp's client for `clasp login`, or your own client if you logged in with custom credentials. Then log in again and update the secret. `glasp logout` only deletes the local file and does not revoke the token.

## Monorepo / subdirectory projects

If your `.clasp.json` lives in a subdirectory (e.g. a monorepo), use the `working-directory` input:

```yaml
- uses: takihito/glasp@v0.5.0
  with:
    version: 'v0.5.0'
{% raw %}
    auth: '${{ secrets.GLASP_AUTH }}'
{% endraw %}
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
- uses: takihito/glasp@v0.5.0   # recommended: pin to a release tag
  with:
    version: 'v0.5.0'
```

GitHub Release artifacts are immutable, so pinning `version` guarantees the exact same binary is installed on every run.

You can also pin the action itself by commit SHA for stricter supply-chain control:

```yaml
- uses: takihito/glasp@fe69b6400dd50345faf226f36e3acfac48c6c07c # v0.5.0
  with:
    version: 'v0.5.0'
```