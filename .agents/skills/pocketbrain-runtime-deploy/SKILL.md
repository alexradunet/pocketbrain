---
name: pocketbrain-runtime-deploy
description: Deploy PocketBrain runtime on Debian with Bun and verify healthy always-on services.
compatibility: opencode
metadata:
  audience: operators
  scope: runtime
---

## What I do

- Install runtime prerequisites on Debian
- Configure runtime environment
- Start and verify PocketBrain and Syncthing services
- Hand off update and health commands

## When to use me

Use this for first-time runtime deployment or redeployment on a server.

## Canonical workflow

1. Install prerequisites:

```bash
make setup-runtime
```

2. Configure environment:

```bash
cp .env.example .env
```

Set at minimum:

```dotenv
TS_AUTHKEY=tskey-auth-...
```

3. Start runtime:

```bash
make up
```

4. Verify health:

```bash
make logs
```

5. Confirm always-on behavior:
- Runtime service enabled on boot
- Runtime process restarts on failure

## Operational handoff

```bash
make logs
git pull && bun install && bun run start
```
