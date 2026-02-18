---
name: pocketbrain-dev-setup
description: Set up and validate a PocketBrain developer environment from repository root.
compatibility: opencode
metadata:
  audience: contributors
  scope: development
---

## What I do

- Prepare local prerequisites for development
- Install dependencies and initialize local configuration
- Validate the basic dev workflow (`dev`, `test`, `build`)

## When to use me

Use this when onboarding a developer machine or fixing a broken local setup.

## Canonical workflow

1. Run setup:

```bash
make setup-dev
```

2. Ensure environment file exists:

```bash
cp .env.example .env
```

3. Run CLI onboarding if needed:

```bash
bun run setup
```

4. Validate developer commands:

```bash
make test
make build
```

5. Start local dev runtime:

```bash
make dev
```

## Troubleshooting

- `bun: command not found`: re-open shell and ensure Bun is in PATH.
- `.env` missing: create from `.env.example`.
- test/build errors: run `bun install --frozen-lockfile` and retry.
