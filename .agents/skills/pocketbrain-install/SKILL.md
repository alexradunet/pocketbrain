---
name: pocketbrain-install
description: Install and deploy PocketBrain on a Debian server from zero to healthy always-on runtime using OpenCode-driven guidance.
compatibility: opencode
---

## Purpose

Use this skill when the user wants a guided, end-to-end PocketBrain deployment on Debian, especially from a fresh server.

## Trigger phrases

- install pocketbrain on debian
- deploy pocketbrain on vps
- set up pocketbrain from scratch
- make pocketbrain always on
- opencode guided install for pocketbrain

## Required outcomes

1. Docker Engine + Compose plugin installed
2. PocketBrain repository cloned and configured
3. `.env` created with `TS_AUTHKEY` and core settings
4. `docker compose up -d --build` succeeds
5. `docker compose ps` shows healthy services
6. User receives clear operational commands (logs, restart, update)

## Canonical workflow

### 1) Prepare Debian host

Run:

```bash
sudo apt update
sudo apt install -y ca-certificates curl gnupg lsb-release git
```

### 2) Install Docker + Compose

Run:

```bash
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/debian/gpg | sudo gpg --dearmor --batch --yes -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/debian $(. /etc/os-release && echo $VERSION_CODENAME) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo systemctl enable --now docker
```

### 3) Clone and enter app directory

```bash
git clone https://github.com/CefBoud/PocketBrain.git
cd PocketBrain
```

### 4) Configure environment

- Ensure `.env` exists (copy from `.env.example` if needed)
- Require `TS_AUTHKEY` from user (Tailscale reusable + ephemeral key)
- Set `ENABLE_WHATSAPP=true` only if user wants WhatsApp channel

### 5) Deploy

```bash
mkdir -p data/syncthing-config
docker compose up -d --build
```

### 6) Verify

```bash
docker compose ps
docker compose logs --tail=120 pocketbrain
docker compose logs --tail=120 syncthing
```

### 7) Always-on confirmation

Confirm both conditions:
- Docker enabled on boot
- Compose services use restart policy (`unless-stopped`)

## Troubleshooting playbook

If deployment fails, check in this order:

1. Missing `TS_AUTHKEY` in `.env`
2. Docker daemon inactive (`systemctl status docker`)
3. Permission issues on data path (`chown -R 1000:1000 ./data`)
4. Container logs for runtime errors (`docker compose logs pocketbrain`)

## Operational handoff commands

Provide these after successful setup:

```bash
docker compose ps
docker compose logs -f pocketbrain
docker compose logs -f syncthing
git pull && docker compose up -d --build
```

## Invocation from OpenCode session

After user installs OpenCode with:

```bash
curl -fsSL https://opencode.ai/install | bash
opencode
```

Use this instruction:

"Use the `pocketbrain-install` skill to perform zero-to-deploy setup on this Debian host, then verify both services are healthy and always-on."
