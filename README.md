# PocketBrain

PocketBrain is a Go assistant runtime with SQLite-backed state, WhatsApp integration, and AI tool calling.

Single binary. Zero runtime dependencies. Just build and run.

## Quick Start (Interactive)

```bash
go build .                   # produces ./pocketbrain binary
./pocketbrain setup          # first-run interactive setup (creates/patches .env)
./pocketbrain start          # start with TUI
./pocketbrain start --headless  # start headless (for servers)
```

If you choose `kronk` in setup, the wizard pulls the current model list from
the Kronk catalog and can download selected models directly via the Kronk SDK.

## Quick Deploy (Headless Server)

```bash
go build .
./pocketbrain setup          # run once in an interactive shell
./pocketbrain start --headless
```

Headless mode requires a complete `.env`. If missing/incomplete, startup fails with instructions.

## Quick Dev Setup

```bash
go build .
./pocketbrain setup
go test ./... -count=1
go run . start
```

## Commands

```bash
go build .    # compile binary
go test ./... -count=1       # run all tests
go run . start               # run with TUI (dev)
go run . start --headless    # run headless
go run . setup               # interactive setup wizard
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
  tailscale/         embedded tsnet + Taildrive share orchestration
  taildrive/         local file serving components
  tui/               terminal UI (bubbletea)
  workspace/         file operations with path security
docs/                architecture, deploy, and runbooks
```

## Documentation

- Deployment quick guide: `README.DEPLOY.md`
- Architecture: `docs/architecture/`
- Deploy: `docs/deploy/`
- Runbooks: `docs/runbooks/`

## Documentation Map

- Runtime deployment: `docs/runbooks/runtime-deploy.md`
- Developer setup: `docs/runbooks/dev-setup.md`
- Taildrive and embedded tailscale operations: `docs/runbooks/taildrive-ops.md`
