---
name: pocketbrain-runtime-deploy
description: Deploy PocketBrain runtime on Debian with Bun and verify healthy always-on services.
compatibility: opencode
metadata:
  audience: operators
  scope: runtime
---

## What I do

- Execute the canonical runtime deployment workflow on Debian
- Verify startup and health checks
- Hand off stable update/log commands

## When to use me

Use this for first-time runtime deployment or redeployment on a server.

## Canonical references

- Primary workflow: `docs/runbooks/runtime-deploy.md`
- First-time install context: `docs/runbooks/runtime-install.md`

## Operational handoff

```bash
make logs
git pull && bun install && make start
```
