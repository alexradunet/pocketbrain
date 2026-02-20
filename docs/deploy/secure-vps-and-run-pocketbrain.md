# Secure VPS and Run PocketBrain

Use this guide to harden a fresh Debian VPS. Runtime deploy steps are canonicalized in runbooks.

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
make build
```

## 6) Run as a systemd service

Use the template shipped in the repository:

```bash
sudo cp docs/deploy/systemd/pocketbrain.service /etc/systemd/system/pocketbrain.service
```

If your server user or install path differs, edit these fields in `/etc/systemd/system/pocketbrain.service`:

- `User`
- `WorkingDirectory`
- `Environment=HOME`
- `Environment=PATH`
- `ExecStart`

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

## 7) Backup and restore drill

Use your VPS/provider snapshot backup workflow for this drill:

1. Capture a snapshot/backup that includes PocketBrain data directories.
2. Restore to a test instance or rollback target.
3. Restart PocketBrain service and verify:
   - `sudo systemctl status pocketbrain`
   - `make logs`

Run this drill regularly and keep off-host backup retention enabled.

## 8) Runtime deployment and updates

Use canonical runbooks:

- `docs/runbooks/runtime-install.md`
- `docs/runbooks/runtime-deploy.md`
- `docs/runbooks/release-ops.md`
