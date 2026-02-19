---
name: pocketbrain-incident-response
description: Drive first-response diagnosis and recovery for PocketBrain runtime incidents.
compatibility: opencode
metadata:
  audience: operators
  scope: incident
---

## What I do

- Triage incident severity
- Run first-10-minute diagnostics
- Execute scenario-based mitigation and recovery verification

## When to use me

Use this during runtime outages, degraded behavior, or suspected data integrity incidents.

## First 10 minutes

1. Confirm impacted runtime process (`pocketbrain`).
2. Capture state:

```bash
make logs
```

3. Freeze risky manual changes until root cause is identified.

## Scenario playbooks

### PocketBrain unhealthy
- inspect startup and health logs
- validate env/config
- rollback with `make release TAG=<last-known-good-tag>` when needed

### Tailscale disconnected
- validate `TS_AUTHKEY`
- restart runtime with corrected key
- verify `tailscale status` on host

### SQLite/state corruption
- backup current `data/`
- restore known-good backup via `make restore FILE=<backup-file>`
- verify recovery health and behavior

### WhatsApp reconnect loop
- validate auth directory persistence and permissions
- rotate/re-pair auth state if stale

## Recovery verification

- both services healthy
- logs stable without repeated exceptions
- one chat command path succeeds
