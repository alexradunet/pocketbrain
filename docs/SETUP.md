# Pocketbrain - Development Machine Setup

A guide for provisioning a Debian VPS as a remote development machine with
Claude Code running as an OS-level assistant with root access.

## Overview

| Component       | Detail                                      |
|-----------------|---------------------------------------------|
| OS              | Debian 13 (trixie) x86_64                   |
| Host            | OVH VPS (`vps-672d3569`)                    |
| Public IP       | `51.38.141.38` (firewalled, no SSH)         |
| Tailscale IP    | `100.98.149.63`                             |
| Tailscale name  | `pocketbrain-host`                          |
| User            | `debian` (uid 1000, member of `sudo` group) |
| Access          | **Tailscale SSH only**                      |
| Claude Code     | v2.1.49                                     |

---

## 1. Access (Tailscale SSH Only)

All remote access is through Tailscale SSH. There is no public-facing SSH.

```bash
tailscale ssh debian@pocketbrain-host
```

The public IP (`51.38.141.38`) does **not** accept SSH connections. OpenSSH
listens only on `127.0.0.1:22` for local/Tailscale use.

---

## 2. Security Hardening

### SSH (`/etc/ssh/sshd_config`)

| Setting              | Value         |
|----------------------|---------------|
| ListenAddress        | `127.0.0.1`  |
| PermitRootLogin      | `no`          |
| PasswordAuthentication | `no`        |
| AllowUsers           | `debian`      |
| MaxAuthTries         | `3`           |
| X11Forwarding        | `no`          |

Cloud-init is prevented from re-enabling password auth via
`/etc/cloud/cloud.cfg.d/99-disable-ssh-pwauth.cfg`.

### Firewall (nftables)

`/etc/nftables.conf` — input policy is **drop**. Allowed traffic:

| Rule                      | Interface / Port            |
|---------------------------|-----------------------------|
| Established/related       | all                         |
| Loopback                  | `lo`                        |
| Tailscale                 | `tailscale0`                |
| Docker bridge             | `docker0`                   |
| ICMP (v4/v6)              | all                         |
| Tailscale WireGuard       | `ens3` UDP 41641            |
| DHCP client               | `ens3` UDP 67→68            |

Docker manages its own forwarding rules via iptables-nft.

### fail2ban

SSH jail enabled — 3 retries within 10 minutes triggers a 1-hour ban.

```bash
sudo fail2ban-client status sshd
```

---

## 3. Install Tailscale

```bash
curl -fsSL https://tailscale.com/install.sh | sh
sudo tailscale up
# Open the auth URL printed in your browser and approve the device
```

Enable Tailscale SSH:

```bash
tailscale set --ssh
```

---

## 4. Install Core Tools

```bash
sudo apt-get update
sudo apt-get install -y git
```

---

## 5. Install Claude Code

```bash
curl -fsSL https://claude.ai/install.sh | bash
```

Add to PATH if the installer says `~/.local/bin` is not in your PATH:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

Verify:

```bash
claude --version
```

---

## 6. Grant Claude Code Root Access

The `debian` user is in the `sudo` group. Passwordless sudo is configured:

```
# /etc/sudoers.d/debian-nopasswd
debian ALL=(ALL) NOPASSWD: ALL
```

Claude Code runs as `debian` and uses `sudo` for privileged operations. The
admin-approval layer is built into Claude Code itself — it shows you what it
intends to do and waits for approval before running tool calls.

---

## 7. Project Workspace

```bash
mkdir -p ~/pocketbrain
cd ~/pocketbrain
git init
git branch -m main
```

Launch Claude Code:

```bash
cd ~/pocketbrain && claude
```

---

## 8. Docker Dev Container

### Host setup

Docker is installed from Debian repos (`docker.io`). Log rotation is configured
in `/etc/docker/daemon.json` (10MB max, 3 files).

### Container

The dev container (`pocketbrain-dev`) runs Debian trixie-slim with common
development tools pre-installed. It mounts the project directory as a volume.

**Build and start:**

```bash
docker compose up -d --build
```

**Enter the container:**

```bash
docker exec -it pocketbrain-dev bash
```

**Stop:**

```bash
docker compose down
```

**Rebuild after Dockerfile changes:**

```bash
docker compose up -d --build
```

### Dockerfile

`~/pocketbrain/Dockerfile` — Debian trixie-slim base with:
- Build tools: `build-essential`, `git`, `curl`
- Python: `python3`, `pip`, `venv`
- Utilities: `vim`, `less`, `htop`, `net-tools`
- User `dev` with passwordless sudo

### docker-compose.yml

- Container name: `pocketbrain-dev`
- Volume: `.` → `/home/dev/workspace`
- Resource limits: 4 CPUs, 8GB RAM
- Restart policy: `unless-stopped`

---

## Quick Reference

| Task                        | Command                                        |
|-----------------------------|------------------------------------------------|
| SSH into VPS                | `tailscale ssh debian@pocketbrain-host`        |
| Start Claude Code           | `cd ~/pocketbrain && claude`                   |
| Check Tailscale status      | `tailscale status`                             |
| Build/start dev container   | `docker compose up -d --build`                 |
| Enter dev container         | `docker exec -it pocketbrain-dev bash`         |
| Stop dev container          | `docker compose down`                          |
| Check firewall              | `sudo nft list ruleset`                        |
| Check fail2ban              | `sudo fail2ban-client status sshd`             |
| Check SSH binding           | `sudo ss -tlnp \| grep :22`                   |
