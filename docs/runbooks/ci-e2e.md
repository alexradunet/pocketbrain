# CI Quality Gates Runbook

Use this runbook to understand and verify CI quality gate checks.

## 1) Quality gates

The `.github/workflows/quality-gates.yml` workflow runs on every PR and push to main:

- `go build ./...` — compilation check
- `go test ./... -count=1 -race` — full test suite with race detection
- `go vet ./...` — static analysis

## 2) Structure contract

The `.github/workflows/structure-contract.yml` workflow validates Go project structure on PRs:

- Required files: `main.go`, `go.mod`, `go.sum`, `Makefile`, `README.md`
- Required directories: `cmd/`, `internal/`, `docs/`
- No stale TypeScript artifacts

## 3) Local validation

Run the same checks locally before pushing:

```bash
go build ./...
go test ./... -count=1 -race
go vet ./...
```

## 4) Troubleshooting

- Build fails: check `go mod tidy` for missing dependencies.
- Test fails with race: investigate concurrent access patterns.
- Vet warnings: fix reported issues before merging.
