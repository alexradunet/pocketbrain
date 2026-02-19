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

## Canonical references

- Primary workflow: `docs/runbooks/ci-e2e.md`
- CI pipeline file: `.github/workflows/quality-gates.yml`

## Troubleshooting

- Step skipped unexpectedly: check secret name typo.
- Step fails: validate provider/model still available and credentials are valid.
