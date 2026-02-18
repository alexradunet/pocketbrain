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

1. Confirm impacted services (`pocketbrain`, `syncthing`).
2. Capture state:

```bash
docker compose -p pocketbrain-runtime -f docker-compose.yml ps
docker compose -p pocketbrain-runtime -f docker-compose.yml logs --tail=200 pocketbrain
docker compose -p pocketbrain-runtime -f docker-compose.yml exec pocketbrain tailscale status
```

3. Freeze risky manual changes until root cause is identified.

## Scenario playbooks

### PocketBrain unhealthy
- inspect startup and health logs
- validate env/config
- rollback with `make release TAG=<last-known-good-tag>` when needed

### Tailscale disconnected
- validate `TS_AUTHKEY`
- restart stack with corrected key
- verify `tailscale status` in container

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
