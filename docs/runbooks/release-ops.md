# Release Operations Runbook

Use this runbook for every staging or production release candidate.

## 1) Pre-release checks

```bash
go vet ./...
go test ./... -count=1 -race
go build ./...
```

## 2) Deploy

```bash
make build
sudo systemctl restart pocketbrain
```

## 3) Verify runtime

```bash
make logs
```

Validation checklist:
- startup logs include expected commit/tag context
- no repeated startup/runtime errors
- one end-to-end command path succeeds (`/new` or `/remember`)

## 4) Release record

Capture:
- git SHA
- key changes
- known risks
- verification evidence
