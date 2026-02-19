# CI OpenCode E2E Runbook

Use this runbook to enable and verify model-path E2E checks in CI.

## 1) Configure required secret

- Secret name: `OPENCODE_MODEL`
- Format: `provider/model` (example: `openai/gpt-5`)

## 2) GitHub setup

1. Open repository `Settings -> Secrets and variables -> Actions`.
2. Add `OPENCODE_MODEL`.
3. Re-run PR workflow.
4. Confirm `Run OpenCode E2E test (optional)` executes in `.github/workflows/quality-gates.yml`.

## 3) Validation expectations

- Without secret: step is skipped.
- With secret: step runs `bun run test:opencode:e2e` and passes.

## 4) Troubleshooting

- Step skipped unexpectedly: check exact secret name.
- Step fails: verify model identifier format and provider credentials.
