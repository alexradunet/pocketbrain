# PocketBrain Deployment Guide

This guide gives quick, practical deployment tracks for:
- End-users running PocketBrain as a service/runtime
- Developers running and iterating locally

All commands assume repository root.

## End-User Quick Deployment

### 1) Build binary

```bash
make build
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
make test
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
make build
./pocketbrain setup
```

### 3) Validate + run

```bash
make test
make dev
```

### 4) Common developer commands

```bash
make build
make test
make dev
make start
make setup
```

## Canonical Environment Keys

Primary keys used by runtime/setup:
- `PROVIDER`, `MODEL`, `API_KEY`
- `ENABLE_WHATSAPP`, `WHATSAPP_AUTH_DIR`, `WHITELIST_PAIR_TOKEN`
- `WORKSPACE_ENABLED`, `WORKSPACE_PATH`
- `TAILSCALE_ENABLED`, `TS_AUTHKEY`, `TS_HOSTNAME`, `TS_STATE_DIR`
- `TAILDRIVE_ENABLED`, `TAILDRIVE_SHARE_NAME`, `TAILDRIVE_AUTO_SHARE`

Reference template:
- `.env.example`
