# PocketBrain

PocketBrain is a minimal assistant layer on top of the OpenCode SDK. 

- WhatsApp adapter
- SQLite based configuration
- Runtime state persisted in SQLite (`.data/state.db`) with a synced Markdown vault for knowledge files.
- Easy to setup and extend by leveraging OpenCode SDK
- Kept simple and minimal with a highly modular codebase so that it can customized however you wish with the possibility of OpenCode
- Simple and easy module to quickly download an LLM model and run on your device. For pure all in one solution.

## Why?

I was always obsessed by PKM solutions like Obsidian, LogSeq, Capacities but somehow they did not feel right, I would leave them after a few months and try new apps. They also lacked the "proactive atttitude" that LLM can bring. 

Also ... I was really fascinated by what OpenClaw can do. But seeing how easily it was for me to mess up my environment soo many times, and due to getting my google account blocked due to using Antigravity OAuth for it, I decided that I want to learn how it works and try to build my own. I went through multiple versions, where firstly I wrote it as a wrapper behing pocketbase, but I ended over-engineering it too much, and relied too much on AI.

This is my third attempt, and the third attempt always works.

I decided to use OpenCode SDK, as it is already a very mature solution and is also the perfect case for this. By learning OpenCode I will also enhance my productivity by learning to work with this.

Another goal of this would be to have multiple instances of this running on different devices and when they are in a mesh, most probably with tailscale, we will have one master and multiple sub-pocket-brains. This master will configure the configuration of the entire pocket-brain meshes. 

For example : 1. I have a VPS that runs some small QWEN 8B LLM or uses an API 
If I open my PC, which will run pocketbrain, then the master can identify it and maybe choose to use the LLM from the Desktop PC that runs on VRAM meaning it's much faster. But this is too advanced, for now just Obsidian sync with Voice to text and text to voice.

## Auth model (OpenCode-native)

This project is set up to reuse OpenCode's existing auth mechanisms.

- Default path: uses `createOpencode(...)` so SDK starts/manages a local OpenCode server and uses OpenCode auth/config.
- Alternate path: set `OPENCODE_SERVER_URL` to connect to an already-running OpenCode server/client setup.
- No app-specific API key is required in this repo.

## 🐳 Docker Deployment (Recommended)

The Docker setup provides **complete isolation** - OpenCode agents run sandboxed with no access to your host system.

### Quick Start

```bash
# 1. Clone repo
git clone https://github.com/CefBoud/PocketBrain.git && cd PocketBrain

# 2. Get Tailscale auth key
# Visit: https://login.tailscale.com/admin/settings/keys
# Create a "Reusable" + "Ephemeral" key

# 3. Configure
cp .env.example .env
# Edit .env and set: TS_AUTHKEY=tskey-auth-xxxxx

# 4. Start
mkdir -p data
docker compose up -d

# 5. Check logs
docker compose logs -f
```

Your PocketBrain is now running with:
- **AI Assistant**: Connected to your tailnet
- **File Sync**: Syncthing at `http://localhost:8384` (setup required)
- **Vault**: Markdown files synced to your devices via Syncthing

### Why Docker?

| Feature | Benefit |
|---------|---------|
| 🔒 **Sandboxed** | AI agents isolated from host system |
| 🔐 **Secure Network** | All traffic via Tailscale (encrypted WireGuard) |
| 🚀 **Zero Config** | Works out of the box with `docker compose up` |
| 💾 **Persistent** | Data survives container restarts |
| 🔄 **Auto-Restart** | Container restarts if it crashes |

See [DOCKER.md](DOCKER.md) for detailed configuration, troubleshooting, and advanced options.

## 🛠️ Development Setup

For local development without Docker:

### Prerequisites

1. Install Bun and dependencies:

```bash
# install bun
curl -fsSL https://bun.com/install | bash

# clone repo
git clone https://github.com/CefBoud/PocketBrain.git && cd PocketBrain

# install
bun install
```

2. Log in using the OpenCode CLI:

```bash
bun run opencode auth login
```

Then open the TUI with `bun run opencode` and pick a model using `/models`.

3. Create the env file:

```bash
cp .env.example .env
```

4. Fill required values in `.env` (manually or via the setup script below):

Optional:

- `OPENCODE_MODEL` in `provider/model` format
- `ENABLE_WHATSAPP=true`
- `HEARTBEAT_INTERVAL_MINUTES` (default 30)
- `WHITELIST_PAIR_TOKEN` (required for self-pairing via chat command)
- `VAULT_ENABLED` (default `true`)
- `VAULT_PATH` (default `/data/vault` in Docker, configurable for local runs)

5. Run:

```bash
bun run dev
```

To keep it running after an SSH session ends:

```bash
nohup bun run dev > pocketbrain.log 2>&1 &
disown
```

## CLI onboarding

Run the interactive setup to configure channels and auth:

```bash
bun run setup
```

This will:
- Enable WhatsApp via QR login
- Update `.env`
- Check OpenCode model auth (launches `opencode` if missing)

## OpenCode E2E health check

Run a local end-to-end check that starts its own OpenCode server via SDK, sends a prompt, and verifies a model reply:

```bash
bun run test:opencode:e2e
```

## Commands

In WhatsApp chat:

- `/remember <text>`: save durable memory to SQLite (`memory` table)
- `/pair <token>`: add your account to whitelist (if pairing token is configured)
- `/new`: start a new shared main OpenCode session across all channels
- Any normal message: sent to OpenCode SDK session, with relevant memory context injected

## Data layout

- `.data/state.db`: SQLite database (sessions, memory, whitelist, outbox)
- `.data/vault/`: Markdown vault synced via Syncthing (inbox, daily, journal, etc.)
- `.data/whatsapp-auth/`: Baileys auth state
- `.data/tailscale/`: Tailscale state (Docker only)

## Security

- **Docker**: AI agents run in isolated container with no host access
- **Tailscale**: All networking encrypted via WireGuard
- **Warning**: This project is experimental. Use at your own risk and exercise extreme care and caution, especially in production or with sensitive data.

## License

MIT
