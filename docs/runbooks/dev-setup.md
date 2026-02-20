# Developer Setup Runbook

Use this runbook for onboarding or repairing a contributor machine.

## 1) Prerequisites

- Go 1.25+ installed and in `PATH`
- Git

## 2) Initialize local environment

```bash
./pocketbrain setup
```

The wizard creates/patches `.env` with provider, WhatsApp, workspace, and WebDAV settings.

## 3) Validate core workflows

```bash
go test ./... -count=1
go build .
go run . start
```

## 4) Troubleshooting

- `go: command not found`: ensure Go is installed and in `PATH`.
- Missing `.env`: create it from `.env.example`.
- Test/build failures: run `go mod tidy` and retry.
