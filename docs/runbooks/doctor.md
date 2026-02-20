# Doctor Runbook

Use doctor to verify runtime health and configuration.

## Quick checks

```bash
# Verify build
go build ./...

# Run tests
go test ./... -count=1

# Check service status
sudo systemctl status pocketbrain
```

## What to check

- `.env` presence and required variables set
- Data directories exist and are writable (`.data/`, `.data/workspace/`, `.data/whatsapp-auth/`)
- SQLite state DB presence (`.data/state.db`)
- Service health (`pocketbrain`)
- UFW baseline (`active`, `deny incoming`, `OpenSSH` allow)
- SSH hardening (`PasswordAuthentication no`, `PermitRootLogin no`)
- WebDAV server status (if `WEBDAV_ENABLED=true`)

## Deep validation

```bash
go vet ./...
go test ./... -count=1 -race
```

## Exit criteria

- All checks pass
- Service running and healthy
- End-to-end message path works
