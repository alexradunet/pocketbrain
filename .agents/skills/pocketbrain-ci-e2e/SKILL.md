---
name: pocketbrain-ci-e2e
description: Enable and verify PocketBrain CI OpenCode model-path E2E checks.
compatibility: opencode
metadata:
  audience: maintainers
  scope: ci
---

## What I do

- Configure CI secret required for model-path E2E
- Validate expected workflow behavior with and without secret
- Provide quick troubleshooting steps for skipped/failing E2E

## Required CI secret

- Secret name: `OPENCODE_MODEL`
- Format: `provider/model`
- Example: `openai/gpt-5`

## GitHub setup

1. Open repository settings -> Secrets and variables -> Actions.
2. Add secret `OPENCODE_MODEL`.
3. Re-run a PR workflow.
4. Confirm `Run OpenCode E2E test (optional)` executes in `.github/workflows/quality-gates.yml`.

## Validation

- Without secret: E2E step is skipped.
- With secret: E2E step runs `bun run test:opencode:e2e` and must pass.

## Troubleshooting

- Step skipped unexpectedly: check secret name typo.
- Step fails: validate provider/model still available and credentials are valid.
