# PocketBrain

PocketBrain is a personal assistant runtime built on Bun + OpenCode, with persistent local state and Docker-first deployment.

## 30-Second Quickstart

### Users / Operators

```bash
cp .env.example .env
# set TS_AUTHKEY in .env
make up
make ps
```

Requires Docker + Compose on the host. Use `make setup-runtime` if needed.

### Developers

```bash
bun install
cp .env.example .env
bun run setup
make dev
```

Requires Bun 1.3.x+.

## Project Overview

PocketBrain includes:
- OpenCode SDK integration for model/runtime workflows
- WhatsApp channel adapter (optional)
- SQLite-backed state and memory
- Syncthing-backed vault synchronization
- Operational scripts for deploy, release, backup, and restore

Core runtime services:
- `tailscale`: networking sidecar for tailnet connectivity
- `pocketbrain`: assistant process
- `syncthing`: file sync service for `data/vault`

## Architecture at a Glance 🗺️

```mermaid
flowchart LR
  User[User 📱] --> WA[WhatsApp Adapter]
  WA --> Core[AssistantCore]
  Core --> OpenCode[OpenCode Runtime]
  Core --> DB[(SQLite state.db)]
  Core --> Vault[(data/vault Markdown)]
  Syncthing[Syncthing 🔄] <--> Vault
```

- Full walkthrough: `docs/architecture/system-overview.md`
- Repository structure contract: `docs/architecture/repository-structure.md`
- Security model: `docs/architecture/security-threat-model.md`

## Project Data

Default runtime data is stored in `data/`:
- `data/state.db` SQLite state (sessions, memory, whitelist, outbox)
- `data/vault/` synced markdown vault
- `data/whatsapp-auth/` WhatsApp auth state
- `data/tailscale/` Tailscale state
- `data/syncthing-config/` Syncthing config

Environment/config:
- `.env.example` -> copy to `.env`
- required for runtime: `TS_AUTHKEY`

## Repository Layout

- `src/` application code
- `tests/` automated tests
- `scripts/` setup, runtime, and ops scripts
- `docs/` architecture, setup, deploy, and references
- `.agents/skills/` reusable OpenCode-compatible workflows
- `development/` CI and structure-contract tooling

## Quick Launch (Users / Operators)

Use the 30-second quickstart above for the fastest path.
For full host bootstrap + runtime flow:

```bash
make setup-runtime
cp .env.example .env
# set TS_AUTHKEY in .env
make up
make ps
make logs
```

Update runtime:

```bash
git pull
make up
```

## Quick Launch (Developers)

Use the 30-second quickstart above for the fastest path.
For full local setup + validation:

```bash
make setup-dev
cp .env.example .env
bun run setup
make test
make dev
```

## Common Commands

```bash
make test
make build
make up
make down
make ps
make logs
make release TAG=$(git rev-parse --short HEAD)
make backup
make restore FILE=backups/<backup.tar.gz>
```

## Skills-First Operations

Operational workflows are externalized as skills in `.agents/skills/`.
Skill catalog:
- `docs/setup/agent-skills.md`

## Documentation

- Developer setup: `docs/setup/developer-onboarding.md`
- Runtime deploy: `docs/deploy/debian-runtime-zero-to-deploy.md`
- Runbooks index: `docs/runbooks/README.md`
