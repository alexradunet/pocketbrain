# Secure VPS and Run PocketBrain

Use this guide to harden a fresh Debian VPS and run PocketBrain as an always-on Bun service.

## 1) Baseline host security

Run as root once after provisioning:

```bash
apt update
apt install -y sudo ufw fail2ban unattended-upgrades ca-certificates curl git
```

Create or reuse a non-root operator account and grant sudo:

```bash
adduser debian
usermod -aG sudo debian
```

## 2) Lock down SSH

1. Add your public key to `/home/debian/.ssh/authorized_keys`.
2. Update `/etc/ssh/sshd_config`:

```text
PermitRootLogin no
PasswordAuthentication no
KbdInteractiveAuthentication no
PubkeyAuthentication yes
AllowUsers debian
```

Validate config and restart SSH:

```bash
sshd -t
systemctl restart ssh
```

Keep your current session open and verify you can open a second SSH session before closing the first.

## 3) Configure firewall and intrusion protection

```bash
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp
ufw --force enable
systemctl enable --now fail2ban
```

## 4) Enable automatic security updates

```bash
dpkg-reconfigure -plow unattended-upgrades
systemctl enable --now unattended-upgrades
```

## 5) Install PocketBrain runtime prerequisites

As your non-root user:

```bash
git clone <your-repo-url> pocketbrain
cd pocketbrain
make setup-runtime
```

## 6) Configure PocketBrain

```bash
cp .env.example .env
```

Set values appropriate for your environment. Common starting point:

```dotenv
APP_NAME=pocketbrain
LOG_LEVEL=info
ENABLE_WHATSAPP=true
WHITELIST_PAIR_TOKEN=change-me
DATA_DIR=.data
WHATSAPP_AUTH_DIR=.data/whatsapp-auth
```

## 7) Initialize and run

```bash
bun install
bun run setup
bun run start
```

For ongoing operation, run under systemd.

## 8) Run as a systemd service

Create `/etc/systemd/system/pocketbrain.service`:

```ini
[Unit]
Description=PocketBrain Runtime
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=debian
WorkingDirectory=/home/debian/pocketbrain
Environment=HOME=/home/debian
Environment=PATH=/home/debian/.bun/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
ExecStart=/home/debian/.bun/bin/bun run start
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now pocketbrain
sudo systemctl status pocketbrain
```

Stream logs:

```bash
make logs
```

## 9) Update workflow

```bash
git pull
bun install
bun run test
bun run build
sudo systemctl restart pocketbrain
```

## 10) Backup and restore drill

Create backup:

```bash
make backup
```

Restore from backup:

```bash
make restore FILE=backups/<backup-file>.tar.gz
sudo systemctl restart pocketbrain
```

Run this drill regularly and store backups off-host.
