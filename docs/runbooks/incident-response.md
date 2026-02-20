# Incident Response Runbook

Use this runbook during runtime outages, degraded behavior, or suspected data integrity incidents.

## 1) First 10 minutes

1. Confirm affected runtime process and blast radius.
2. Capture current state:

```bash
journalctl -u pocketbrain -f
```

3. Freeze risky manual changes until root cause is identified.

## 2) Scenario actions

### Runtime unhealthy
- inspect startup and health logs
- validate `.env` and host/runtime configuration
- restart service if configuration was corrected

### SQLite/state corruption
- preserve current `.data/` snapshot for forensics
- restore known-good state via VPS/provider backup workflow
- verify service health and chat path recovery

### WhatsApp reconnect loop
- validate auth directory persistence and permissions
- rotate/re-pair auth state if stale

## 3) Recovery verification

- service is stable after restart
- logs do not show repeated exceptions
- one known command path succeeds
