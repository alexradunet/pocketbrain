# PocketBrain Deployment Guide

This guide gives quick, practical deployment tracks for:
- End-users running PocketBrain as a service/runtime
- Developers running and iterating locally

All commands assume repository root.

## End-User Quick Deployment

### 1) Build binary

```bash
go build .
```

### 2) Run interactive setup once

```bash
./pocketbrain setup
```

The setup wizard creates or patches `.env` and configures:
- LLM provider/model/key
- WhatsApp settings
- Workspace directory
- Embedded Tailscale (`tsnet`) and Taildrive sharing

When `kronk` is selected as provider, setup fetches the live model list from:
`https://github.com/ardanlabs/kronk_catalogs/blob/main/CATALOG.md`
and lets you choose model(s) to download directly via the Kronk SDK
(no separate `kronk` CLI required).

### 3) Start runtime

With TUI:

```bash
./pocketbrain start
```

Headless:

```bash
./pocketbrain start --headless
```

Note: headless start requires a complete `.env`. If setup is incomplete, startup exits with a clear error.

### 4) Verify health

```bash
go test ./... -count=1
```

Optional runtime runbooks:
- `docs/runbooks/runtime-deploy.md`
- `docs/runbooks/taildrive-ops.md`

## Developer Quick Deployment

### 1) Prereqs
- Go 1.26+
- Git

### 2) Build + configure

```bash
go build .
./pocketbrain setup
```

### 3) Validate + run

```bash
go test ./... -count=1
go run . start
```

### 4) Common developer commands

```bash
go build .    # compile binary
go test ./... -count=1       # run all tests
go run . start               # run with TUI (dev)
go run . start --headless    # run headless
go run . setup               # interactive setup wizard
```

## Canonical Environment Keys

Primary keys used by runtime/setup:
- `PROVIDER`, `MODEL`, `API_KEY`
- `ENABLE_WHATSAPP`, `WHATSAPP_AUTH_DIR`, `WHATSAPP_WHITELIST_NUMBERS`
- `WORKSPACE_ENABLED`, `WORKSPACE_PATH`
- `TAILSCALE_ENABLED`, `TS_AUTHKEY`, `TS_HOSTNAME`, `TS_STATE_DIR`
- `TAILDRIVE_ENABLED`, `TAILDRIVE_SHARE_NAME`, `TAILDRIVE_AUTO_SHARE`

Reference template:
- `.env.example`
