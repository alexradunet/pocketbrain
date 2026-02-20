# PocketBrain

PocketBrain is a Go assistant runtime with SQLite-backed state, WhatsApp integration, and AI tool calling.

Single binary. Zero runtime dependencies. Just build and run.

## Quick Start

```bash
cp .env.example .env   # configure provider/model (API key needed for non-Kronk providers)
make build             # produces ./pocketbrain binary
./pocketbrain start    # start with TUI
./pocketbrain start --headless  # start headless (for servers)
```

## Commands

```bash
make build    # compile binary
make test     # run all tests
make dev      # run with TUI (go run)
make start    # run headless (go run)
make setup    # interactive setup wizard
make clean    # remove binary
```

## Data Paths

- Runtime data root: `.data/` (via `DATA_DIR`)
- SQLite state: `.data/state.db`
- Workspace: `.data/workspace/`
- WhatsApp auth: `.data/whatsapp-auth/`

## Repository Layout

```
main.go              entry point
cmd/                 CLI commands (cobra)
internal/
  ai/                AI providers (Anthropic, OpenAI-compatible) + tool registry
  app/               composition root and shutdown
  channel/           channel manager + message chunking/rate limiting
  channel/whatsapp/  WhatsApp adapter (whatsmeow)
  config/            environment configuration
  core/              assistant, session manager, prompt builder, ports
  scheduler/         heartbeat cron scheduler
  skills/            skill management and installation
  store/             SQLite repositories
  taildrive/         Taildrive file sharing
  tui/               terminal UI (bubbletea)
  workspace/         file operations with path security
docs/                architecture, deploy, and runbooks
```

## Documentation

- Architecture: `docs/architecture/`
- Deploy: `docs/deploy/`
- Runbooks: `docs/runbooks/`
