# Runtime Install Runbook

Use this runbook for first-time installation on a fresh Debian host.

## 1) Harden host

Follow:
- `docs/deploy/secure-vps-and-run-pocketbrain.md`

## 2) Deploy PocketBrain runtime

Follow:
- `docs/runbooks/runtime-deploy.md`

## 3) Handoff checks

- Logs stream works: `make logs`
- Service is healthy: `sudo systemctl status pocketbrain`
- PocketBrain vault home exists: `ls .data/vault/99-system/99-pocketbrain`
- Update path validated:

```bash
git pull
bun install
make start
```
