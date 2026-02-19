# Runtime Deploy Runbook

Use this runbook for deploying or redeploying PocketBrain runtime on Debian.

## Runtime Profile (V1)

PocketBrain V1 runs in `vault-only` profile:

- WhatsApp + memory + vault tools are enabled.
- No host/system command execution is exposed to chat users.
- Self-evolution and autonomous code changes are out of scope.

## 1) Install prerequisites

```bash
make setup-runtime
```

## 2) Configure environment

```bash
cp .env.example .env
```

Required minimum:

```dotenv
ENABLE_WHATSAPP=true
DATA_DIR=.data
```

Optional:

```dotenv
WHITELIST_PAIR_TOKEN=your-secure-token
OPENCODE_MODEL=provider/model
OPENCODE_CONFIG_DIR=.data/vault/99-system/99-pocketbrain
WHATSAPP_AUTH_DIR=.data/whatsapp-auth
VAULT_ENABLED=true
SYNCTHING_ENABLED=true
SYNCTHING_BASE_URL=http://127.0.0.1:8384
SYNCTHING_API_KEY=replace-with-api-key
SYNCTHING_VAULT_FOLDER_ID=vault
SYNCTHING_AUTO_START=true
SYNCTHING_MUTATION_TOOLS_ENABLED=false
```

Notes:

- If `OPENCODE_CONFIG_DIR` is unset and vault is enabled, PocketBrain defaults it to `VAULT_PATH/99-system/99-pocketbrain`.
- Keep runtime state local (`.data/state.db`, WhatsApp auth, XDG runtime dirs). The vault path stores portable config and skills.

## 3) Start runtime

```bash
bun install
make start
```

## 4) Verify health

```bash
make logs
```

## 5) Configure always-on runtime

- Use `docs/deploy/systemd/pocketbrain.service`.
- Enable service on boot:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now pocketbrain
sudo systemctl status pocketbrain
```

## 6) Update workflow

```bash
git pull
bun install
make start
```

## 7) Managed release

```bash
make release TAG=$(git rev-parse --short HEAD)
```
