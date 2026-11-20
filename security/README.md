# Threat model

This directory holds glasp's threat model in the [Threat Composer](https://github.com/awslabs/threat-composer) format (schema v1).

| File | Description |
|---|---|
| `threatmodel.tc.json` | Threat model (English, canonical) |
| `threatmodel.ja.tc.json` | Threat model (Japanese). Same IDs and links as the English version |
| `tools/build_viewer.py` | Builds a self-contained HTML viewer from a `.tc.json` file |
| `tools/viewer-template.html` | Template used by `build_viewer.py` |

## Scope

- OAuth 2.0 login flow (local callback server, state, PKCE)
- Token storage (`.glasp/access.json`, `.clasprc.json`, `GLASP_AUTH`)
- Apps Script API communication and writing pulled files
- Command history and archives (`.glasp/history.jsonl`, `.glasp/archive/`, `push --history-id`)
- Passing credentials in CI (`action.yml` and this repository's smoke workflows)

Google's own services, the build and release pipeline, and the safety of the Apps Script code you deploy are out of scope.

## Viewing

Open [Threat Composer](https://awslabs.github.io/threat-composer/) and use its import feature to load a `.tc.json` file. The tool runs in the browser and stores data locally.

## Updating

1. Edit the model in Threat Composer and export it, or edit the JSON directly. Keep the English and Japanese files in sync (same IDs, statuses and links).
2. When a mitigation is completed, set its status to `mitigationResolved` and note the pull request in its comments. Resolve a threat only when its remaining risk is fixed or explicitly accepted, and record the reason in the threat's comments.
3. Rebuild the Japanese HTML viewer:

   ```sh
   python3 security/tools/build_viewer.py security/threatmodel.ja.tc.json docs/ja/threat-model.html
   ```

## Disclosure rule

This repository is public. **Do not add a threat that describes an unfixed, exploitable vulnerability** that is not already evident from the public code or documentation. Report it privately as described in [SECURITY.md](../SECURITY.md), fix it first, and then record it here.
