# Debian Zero-to-Deploy Guide

This runbook takes a fresh Debian server to a running PocketBrain deployment with auto-restart.

## 1) Prepare server

```bash
sudo apt update
sudo apt install -y ca-certificates curl gnupg lsb-release git
```

## 2) Install Docker + Compose

```bash
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/debian/gpg | sudo gpg --dearmor --batch --yes -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg

echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian $(. /etc/os-release && echo $VERSION_CODENAME) stable" \
  | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo systemctl enable --now docker
```

Optional (no sudo for docker commands):

```bash
sudo usermod -aG docker "$USER"
newgrp docker
```

## 3) Clone PocketBrain

```bash
git clone https://github.com/CefBoud/PocketBrain.git
cd PocketBrain/pocketbrain
```

## 4) Configure environment

Get a Tailscale auth key (Reusable + Ephemeral) from:
- https://login.tailscale.com/admin/settings/keys

Then configure:

```bash
cp .env.example .env
```

Set at minimum:

```dotenv
TS_AUTHKEY=tskey-auth-...
ENABLE_WHATSAPP=true
```

Optional but recommended:

```dotenv
WHITELIST_PAIR_TOKEN=your-secure-token
OPENCODE_MODEL=provider/model
DATA_PATH=./data
```

## 5) Start runtime stack

```bash
mkdir -p data/syncthing-config
docker compose -p pocketbrain-runtime -f docker-compose.yml up -d --build
```

## 6) Verify healthy services

```bash
docker compose -p pocketbrain-runtime -f docker-compose.yml ps
docker compose -p pocketbrain-runtime -f docker-compose.yml logs --tail=120 pocketbrain
docker compose -p pocketbrain-runtime -f docker-compose.yml logs --tail=120 syncthing
```

Expected:
- `pocketbrain` is `healthy`
- `syncthing` is `healthy`
- PocketBrain logs show Tailscale connected and app startup

## 7) WhatsApp onboarding (if enabled)

```bash
docker compose -p pocketbrain-runtime -f docker-compose.yml logs -f pocketbrain
```

Scan the QR code shown in logs.

## 8) Always-on behavior

This stack is persistent by default:
- Docker daemon is enabled on boot (`systemctl enable docker`)
- Services use `restart: unless-stopped`

After a reboot, run:

```bash
docker compose -p pocketbrain-runtime -f docker-compose.yml ps
```

## 9) Updates

```bash
git pull
docker compose -p pocketbrain-runtime -f docker-compose.yml up -d --build
```

## 10) Optional dev-control stack

Use this when you want an always-on container that can edit code, run tests, and deploy runtime updates through Docker:

```bash
docker compose -p pocketbrain-dev -f docker-compose.dev.yml up -d --build
docker compose -p pocketbrain-dev -f docker-compose.dev.yml ps
docker compose -p pocketbrain-dev -f docker-compose.dev.yml exec -it dev-control sh
```

Inside `dev-control`, the repository is mounted at `/workspace` and Docker socket access is available for controlled runtime updates.

## 11) Managed release flow

From repo root:

```bash
./scripts/release.sh
./scripts/dev-release.sh
```

This runs:
- typecheck
- tests
- runtime rebuild/deploy with build version stamp
- health wait
- rollback to previous image if health fails

`./scripts/release.sh` runs directly on host.
`./scripts/dev-release.sh` runs the same release flow from `dev-control`.

## 12) OpenCode-first workflow (recommended UX)

If your user wants to start from OpenCode first:

```bash
curl -fsSL https://opencode.ai/install | bash
opencode
```

Then instruct OpenCode:

"Use the `pocketbrain-install` skill to deploy PocketBrain on this Debian host with Docker, configure `.env`, and verify health."
