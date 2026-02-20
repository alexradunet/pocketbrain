# Security Operations Runbook

Use this runbook for recurring security hygiene and post-incident security actions.

## 1) Secret handling rules

- Store secrets only in runtime `.env` or CI secret store.
- Never commit secrets to repository files or logs.
- Rotate secrets after any suspected exposure.

## 2) Rotation cadence

- `WHITELIST_PAIR_TOKEN`: every 30 days
- Provider API keys: every 60 days or after exposure

## 3) Rotation procedure

1. Generate replacement secret in source system.
2. Update runtime `.env` and CI secrets as needed.
3. Restart runtime and verify health:

```bash
sudo systemctl restart pocketbrain
make logs
```

4. Verify one functional command path.
5. Record rotation timestamp in release notes.

## 4) Dependency hygiene

- Weekly: vulnerability review (`govulncheck ./...` if installed)
- Monthly: dependency refresh and full regression checks
- Critical CVEs: patch within 48 hours

Validation command set:

```bash
go vet ./...
go test ./... -count=1 -race
go build ./...
```

## 5) Residual risk maintenance

Update risk records when:
- new external dependency is added
- security controls change
- incident reveals a missing control
