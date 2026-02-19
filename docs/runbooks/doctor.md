# Doctor Runbook

Use doctor to verify runtime/security posture and optionally apply guided repairs.

## Commands

```bash
make doctor
make doctor ARGS="--yes"
make doctor ARGS="--repair"
make doctor ARGS="--repair --force"
make doctor ARGS="--non-interactive"
make doctor ARGS="--deep"
```

## What doctor checks

- `.env` presence
- Data/vault/WhatsApp auth directory existence and writability
- SQLite state DB presence
- Service health (`fail2ban`, `unattended-upgrades`, `tailscaled`, `pocketbrain` when installed)
- UFW baseline (`active`, `deny incoming`, `OpenSSH` allow)
- SSH hardening (`PasswordAuthentication no`, `PermitRootLogin no`, `KbdInteractiveAuthentication no`)
- Tailscale login status
- Syncthing integration checks when enabled (`SYNCTHING_*` policy, API ping, folder ID probe)
- Deep mode: `bun run typecheck`

## Repair model

- Default: checks only
- `--repair`: safe repairs (create dirs, enable/start services, baseline firewall)
- `--repair --force`: includes aggressive SSH hardening drop-in rewrite

## Exit codes

- `0` all checks pass
- `0` all checks pass or warnings only
- `2` one or more failures
