# CI E2E Enablement (OpenCode Model Path)

The `quality-gates` workflow runs `test:opencode:e2e` only when `OPENCODE_MODEL` is present.

## Required CI Secret

- Secret name: `OPENCODE_MODEL`
- Expected format: `provider/model`
- Example: `openai/gpt-5`

## GitHub Setup Steps

1. Open repository settings -> Secrets and variables -> Actions.
2. Add secret `OPENCODE_MODEL` with valid provider/model value.
3. Re-run a pull request workflow and verify `Run OpenCode E2E test (optional)` executes.

## Validation

Expected job behavior in `.github/workflows/quality-gates.yml`:

- Without secret: step is skipped.
- With secret: step runs `bun run test:opencode:e2e` and must pass.

## Operational Note

Treat this secret as production-impacting configuration for test fidelity.
If the provider/model is deprecated or unavailable, update the secret and re-run CI.
