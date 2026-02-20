# PocketBrain

PocketBrain is a Go assistant runtime with SQLite-backed state, WhatsApp integration, and AI tool calling.

Single binary. Zero runtime dependencies. Just build and run.

## Quick Start (Interactive)

```bash
go build .                       # produces ./pocketbrain binary
./pocketbrain                    # first run auto-launches setup wizard TUI
./pocketbrain --setup            # force re-run setup wizard
./pocketbrain --headless         # start headless (for servers)
```

If you choose `kronk` in setup, the wizard pulls the current model list from
the Kronk catalog and can download selected models directly via the Kronk SDK.

## Quick Deploy (Headless Server)

```bash
go build .
./pocketbrain                    # run once interactively to complete setup
./pocketbrain --headless
```

Headless mode requires a complete `.env`. If missing/incomplete, startup fails with instructions.

## Quick Dev Setup

```bash
go build .
./pocketbrain                    # complete setup wizard
go test ./... -count=1
go run .                         # run with TUI
```

## Flags

```
--headless    Run without TUI (daemon mode for systemd/Docker)
--setup       Force run setup wizard even if .env is complete
```

## Data Paths

- Runtime data root: `.data/` (via `DATA_DIR`)
- SQLite state: `.data/state.db`
- Workspace: `.data/workspace/`
- WhatsApp auth: `.data/whatsapp-auth/`

## WhatsApp Access Control

- Access is whitelist-only.
- Configure allowed numbers with `WHATSAPP_WHITELIST_NUMBERS`.
- `/pair` self-service onboarding is disabled.

## Repository Layout

```
main.go              entry point (flag-based, TUI-first)
internal/
  ai/                AI providers (Anthropic, OpenAI-compatible) + tool registry
  app/               composition root, backend wiring, and shutdown
  channel/           channel manager + message chunking/rate limiting
  channel/whatsapp/  WhatsApp adapter (whatsmeow)
  config/            environment configuration
  core/              assistant, session manager, prompt builder, ports
  scheduler/         heartbeat cron scheduler
  setup/             setup wizard logic, env file management
  skills/            skill management and installation
  store/             SQLite repositories
  tui/               terminal UI (Bubble Tea) — dashboard + setup wizard
  webdav/            WebDAV workspace file server
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
