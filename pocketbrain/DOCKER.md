# PocketBrain Docker Deployment

Secure, containerized deployment with Tailscale networking and Syncthing file sync.

## 🚀 Quick Start (Runtime)

```bash
# 1. Clone and enter directory
cd pocketbrain

# 2. Configure
cp .env.example .env
# Edit .env and add your TS_AUTHKEY from https://login.tailscale.com/admin/settings/keys

# 3. Start
mkdir -p data
docker compose up -d

# 4. Check logs
docker compose logs -f pocketbrain
```

Runtime project name used by scripts and docs: `pocketbrain-runtime`.

## 🧪 Dev-Control Stack (Optional)

Run a separate development control container that mounts the repo and can manage runtime updates:

```bash
docker compose -p pocketbrain-dev -f docker-compose.dev.yml up -d --build
docker compose -p pocketbrain-dev -f docker-compose.dev.yml exec -it dev-control sh
```

From inside `dev-control`:
- repo root: `/workspace`
- app root: `/workspace/pocketbrain`

Use this only for controlled development and release operations.

## 📁 Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Docker Compose Stack                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────────────┐    ┌─────────────────────┐        │
│  │   PocketBrain       │    │    Syncthing        │        │
│  │                     │    │                     │        │
│  │  • AI Assistant     │    │  • File Sync        │        │
│  │  • WhatsApp Bot     │    │  • Conflict Resolve │        │
│  │  • SQLite Memory    │    │  • Version History  │        │
│  │  • Tailscale        │    │                     │        │
│  │                     │    │                     │        │
│  │   Tailscale IP      │    │   Tailscale IP      │        │
│  │   100.x.x.x         │    │   100.x.x.x         │        │
│  │        │            │    │        │            │        │
│  └────────┼────────────┘    └────────┼────────────┘        │
│           │                          │                     │
│           └──────────┬───────────────┘                     │
│                      │                                      │
│              ┌───────┴───────┐                             │
│              │  Shared Volume │                             │
│              │  /data/vault   │                             │
│              └───────────────┘                             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**Services:**
- **PocketBrain**: AI assistant (Tailscale + WhatsApp + SQLite)
- **Syncthing**: File synchronization (separate container, shares vault)

## 🔐 Security Features

| Feature | Implementation |
|---------|---------------|
| **Container Sandboxing** | Non-root user, read-only filesystem |
| **AI Agent Isolation** | OpenCode runs inside container, no host access |
| **Network Security** | All traffic via Tailscale (encrypted WireGuard) |
| **No Privileges** | Userspace networking (no NET_ADMIN needed) |
| **Resource Limits** | CPU/memory limits prevent DoS |
| **Separate Sync** | Syncthing isolated in its own container |

## 📂 Data Persistence

Data is stored in the `./data` directory:

```
data/
├── state.db              # SQLite database (sessions, memory, whitelist)
├── vault/                # Markdown vault (synced via Syncthing)
│   ├── inbox/
│   ├── daily/           # Daily notes
│   ├── journal/
│   ├── projects/
│   ├── areas/
│   ├── resources/
│   └── archive/
├── tailscale/           # Tailscale state
│   └── tailscaled.state
├── whatsapp-auth/       # WhatsApp Baileys auth
└── syncthing-config/    # Syncthing configuration
    └── config.xml
```

## 🌐 Accessing Services

Once running, access via Tailscale:

| Service | Access Method | URL |
|---------|--------------|-----|
| **PocketBrain** | Tailscale | `pocketbrain.<tailnet>.ts.net` |
| **Syncthing UI** | Local + Tailscale | `http://localhost:8384` |
| **Vault Files** | Syncthing | Synced to your devices |

### Setting Up Syncthing

1. **Access UI**:
   ```bash
   # Local access (SSH tunnel if remote)
   ssh -L 8384:localhost:8384 your-server
   # Then open http://localhost:8384
   ```

2. **Initial Setup**:
   - Set admin user/password
   - Add your device ID (from your local Syncthing)
   - Share the `/data/vault` folder

3. **Connect Devices**:
   - Add your phone, laptop, etc.
   - Accept the share on each device
   - Files sync automatically

## 🛠️ Configuration

### Required Settings

| Variable | Description | Get From |
|----------|-------------|----------|
| `TS_AUTHKEY` | Tailscale auth key | [Tailscale Admin](https://login.tailscale.com/admin/settings/keys) |

Create a **Reusable** + **Ephemeral** key for containers.

### Optional Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `TS_HOSTNAME` | `pocketbrain` | Name in tailnet |
| `DATA_PATH` | `./data` | Host data directory |
| `ENABLE_WHATSAPP` | `false` | Enable WhatsApp bot |
| `TZ` | `UTC` | Timezone for Syncthing |

## 🔧 Syncthing Configuration

### Adding Remote Devices

1. Get your device's Syncthing ID:
   ```
   Actions → Show ID
   ```

2. In PocketBrain's Syncthing:
   ```
   Add Remote Device → Paste ID
   ```

3. Accept on your device when prompted

### Folder Sharing

Default shared folder: `/data/vault`

To add more folders:
1. Edit `docker-compose.yml`
2. Add volume mount:
   ```yaml
   syncthing:
     volumes:
       - ./other-folder:/data/other-folder
   ```
3. Restart: `docker compose up -d`

## 🔄 Updating

```bash
# Pull latest code
git pull

# Rebuild and restart
docker compose up -d --build

# View logs
docker compose logs -f
```

Managed release with health-check + rollback:

```bash
./scripts/release.sh
./scripts/dev-release.sh
```

Update Syncthing only:
```bash
docker compose pull syncthing
docker compose up -d syncthing
```

## 📊 Monitoring

### View Logs
```bash
# All services
docker compose logs -f

# Just PocketBrain
docker compose logs -f pocketbrain

# Just Syncthing
docker compose logs -f syncthing

# Tailscale status
docker compose exec pocketbrain tailscale status
```

### Syncthing Status
```bash
# Container status
docker compose ps

# Syncthing REST API
curl -H "X-API-Key: YOUR_API_KEY" http://localhost:8384/rest/system/status
```

## 🧹 Backup & Restore

### Backup
```bash
# Stop containers
docker compose stop

# Backup data directory
tar -czf pocketbrain-backup-$(date +%Y%m%d).tar.gz data/

# Start containers
docker compose start
```

### Restore
```bash
# Stop containers
docker compose down

# Restore data
rm -rf data/
tar -xzf pocketbrain-backup-YYYYMMDD.tar.gz

# Start containers
docker compose up -d
```

## 🔧 Troubleshooting

### PocketBrain Won't Start

```bash
# Check logs
docker compose logs pocketbrain

# Common issues:
# 1. Missing TS_AUTHKEY - add to .env
# 2. Check Tailscale status
docker compose exec pocketbrain tailscale status
```

### Syncthing Not Accessible

```bash
# Check if running
docker compose ps

# Check logs
docker compose logs syncthing

# Access directly via Tailscale IP
docker compose exec syncthing wget -qO- http://localhost:8384
```

### Files Not Syncing

1. **Check folder paths**:
   ```bash
   docker compose exec syncthing ls -la /data/vault
   ```

2. **Check Syncthing UI** for errors:
   - Open http://localhost:8384
   - Look for red folders or devices

3. **Restart Syncthing**:
   ```bash
   docker compose restart syncthing
   ```

### Permission Issues

Both containers run as UID 1000. Ensure your data directory is writable:
```bash
sudo chown -R 1000:1000 ./data
```

## 🌍 Multi-Platform Support

Build for multiple platforms:

```bash
# Create builder
docker buildx create --use

# Build and push
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t yourusername/pocketbrain:latest \
  --push .
```

## 🚫 Security Hardening (Optional)

### Enable User Namespace Remapping

In `/etc/docker/daemon.json`:
```json
{
  "userns-remap": "default"
}
```

### Restrict Syncthing to Tailscale Only

Add to Syncthing environment in `docker-compose.yml`:
```yaml
syncthing:
  environment:
    - STGUIADDRESS=127.0.0.1:8384  # Only localhost
```

Then access via Tailscale:
```
# SSH tunnel through Tailscale
ssh -L 8384:localhost:8384 pocketbrain.<tailnet>.ts.net
```

## 📚 Resources

- [Tailscale Docker Guide](https://tailscale.com/kb/1282/docker)
- [Syncthing Documentation](https://docs.syncthing.net/)
- [Docker Security](https://docs.docker.com/engine/security/)

## 💡 Tips

1. **First Run**: Syncthing UI will ask for initial setup
2. **Conflict Resolution**: Syncthing handles conflicts automatically with `.sync-conflict` files
3. **Versioning**: Enable in Syncthing for deleted file recovery
4. **Mobile**: Use Syncthing-Fork on Android for Obsidian mobile sync
