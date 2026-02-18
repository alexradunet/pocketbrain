# PocketBrain Incident Response Runbook

## Severity Levels

- Sev-1: service unavailable or data loss risk.
- Sev-2: degraded functionality with workaround.
- Sev-3: non-critical issue with low user impact.

## First 10 Minutes Checklist

1. Confirm impacted services: `pocketbrain`, `syncthing`.
2. Capture current status:
   - `docker compose -p pocketbrain-runtime -f docker-compose.yml ps`
   - `docker compose -p pocketbrain-runtime -f docker-compose.yml logs --tail=200 pocketbrain`
3. Check health and Tailscale state:
   - `docker compose -p pocketbrain-runtime -f docker-compose.yml exec pocketbrain tailscale status`
4. Freeze risky manual changes until root cause is identified.

## Incident Scenarios

### 1) PocketBrain container unhealthy

- Validate process and health logs.
- Check env/config errors in startup logs.
- If caused by a bad release, run rollback release path:
  - `make release TAG=<last-known-good-tag>`

### 2) Tailscale disconnected

- Verify `TS_AUTHKEY` in `.env` is valid.
- Re-run stack with fresh auth key.
- Confirm `tailscale status` from inside container.

### 3) SQLite errors or state corruption

- Stop runtime stack.
- Create immediate backup of current `data/`.
- Restore latest known-good backup using `make restore FILE=<backup-file>`.
- Start stack and verify health.

### 4) WhatsApp reconnect loop

- Check auth path permissions in configured `DATA_DIR/whatsapp-auth`.
- Validate container can persist auth state.
- If auth state is stale, rotate by backing up then re-pairing.

## Recovery Verification

After mitigation or rollback:

1. `docker compose ... ps` shows both services healthy.
2. PocketBrain logs show startup complete without repeated exceptions.
3. Send a test WhatsApp command (`/new` or `/remember`) and verify response.

## Incident Closure

- Record timeline, root cause, and exact remediation.
- Capture permanent fix ticket and owner.
- Add missing alert/runbook step if detection or recovery was delayed.
