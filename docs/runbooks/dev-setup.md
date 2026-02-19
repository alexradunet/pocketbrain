# Developer Setup Runbook

Use this runbook for onboarding or repairing a contributor machine.

## 1) Bootstrap local prerequisites

```bash
make setup-dev
```

## 2) Initialize local environment

```bash
cp .env.example .env
bun run setup
```

## 3) Validate core workflows

```bash
make test
make build
make dev
```

PocketBrain runtime skill/config home defaults to `VAULT_PATH/99-system/99-pocketbrain` when vault is enabled.

## 4) Troubleshooting

- `bun: command not found`: ensure Bun is installed and in `PATH`.
- Missing `.env`: create it from `.env.example`.
- Test/build failures: run `bun install --frozen-lockfile` and retry.
