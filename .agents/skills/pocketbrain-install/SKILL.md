---
name: pocketbrain-install
description: Install and deploy PocketBrain on Debian from zero to healthy always-on runtime.
compatibility: opencode
metadata:
  audience: operators
  scope: bootstrap
---

## Purpose

Use this for end-to-end first install on a Debian host.

## Required outcomes

1. Bun runtime installed
2. `.env` created with required runtime values
3. Runtime stack starts successfully
4. `pocketbrain` starts cleanly
5. Operator receives handoff commands for logs and updates

## Canonical workflow

1. Install runtime prerequisites:

```bash
make setup-runtime
```

2. Configure environment:

```bash
cp .env.example .env
```

Set required value:

```dotenv
TS_AUTHKEY=tskey-auth-...
```

3. Deploy runtime:

```bash
make up
```

4. Verify:

```bash
make logs
```

## Operational handoff

```bash
make logs
git pull && bun install && bun run start
```
