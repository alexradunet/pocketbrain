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
- Workspace directory exists: `ls .data/workspace`
- Update path validated:

```bash
git pull
make build
sudo systemctl restart pocketbrain
```
