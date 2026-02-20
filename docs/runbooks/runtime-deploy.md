# Runtime Deploy Runbook

Use this runbook for deploying or redeploying PocketBrain runtime on Debian.

## Runtime Profile (V1)

PocketBrain V1 runs in `workspace-only` profile:

- WhatsApp + memory + workspace tools are enabled.
- No host/system command execution is exposed to chat users.
- Self-evolution and autonomous code changes are out of scope.

## 1) Install prerequisites

- Go 1.25+
- Git

## 2) Build binary

```bash
make build
```

This produces a single `./pocketbrain` binary with zero runtime dependencies.

## 3) Configure environment

```bash
./pocketbrain setup
```

Required minimum:

```dotenv
ENABLE_WHATSAPP=true
DATA_DIR=.data
```

Optional:

```dotenv
WHITELIST_PAIR_TOKEN=your-secure-token
PROVIDER=anthropic
MODEL=claude-sonnet-4-20250514
API_KEY=sk-ant-...
WHATSAPP_AUTH_DIR=.data/whatsapp-auth
WORKSPACE_PATH=.data/workspace
TAILSCALE_ENABLED=true
TS_AUTHKEY=tskey-auth-...
TS_HOSTNAME=pocketbrain
TS_STATE_DIR=.data/tsnet
TAILDRIVE_ENABLED=true
TAILDRIVE_SHARE_NAME=workspace
TAILDRIVE_AUTO_SHARE=true
```

Alternative (non-interactive/manual): create `.env` from `.env.example` and fill values directly.

## 4) Start runtime

```bash
make start
```

Or run the binary directly:

```bash
./pocketbrain start --headless
```

## 5) Verify health

```bash
make logs
```

## 6) Configure always-on runtime

- Use `docs/deploy/systemd/pocketbrain.service`.
- Enable service on boot:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now pocketbrain
sudo systemctl status pocketbrain
```

## 7) Update workflow

```bash
git pull
make build
sudo systemctl restart pocketbrain
```
