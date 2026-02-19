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
3. Runtime starts successfully
4. `pocketbrain` starts cleanly
5. Operator receives handoff commands for logs and updates

## Canonical references

- Install workflow: `docs/runbooks/runtime-install.md`
- Deploy workflow details: `docs/runbooks/runtime-deploy.md`

## Operational handoff

```bash
make logs
git pull && bun install && make start
```
