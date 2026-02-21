# PocketBrain

Personal OpenCode assistant. See [README.md](README.md) for philosophy and setup. See [docs/REQUIREMENTS.md](docs/REQUIREMENTS.md) for architecture decisions.

## Quick Context

Single Bun process running inside a Debian 13 Docker container with Tailscale networking. Connects to WhatsApp, routes messages to OpenCode SDK. Agent has full power inside the container. All data lives in `/workspace` volume.

## Key Files

| File | Purpose |
|------|---------|
| `src/index.ts` | Orchestrator: state, message loop, agent invocation |
| `src/channels/whatsapp.ts` | WhatsApp connection, auth, send/receive |
| `src/opencode-manager.ts` | OpenCode SDK session management |
| `src/ipc.ts` | IPC watcher and task processing |
| `src/mcp-tools.ts` | MCP tools server (send_message, schedule_task, etc.) |
| `src/router.ts` | Message formatting and outbound routing |
| `src/config.ts` | Trigger pattern, paths, intervals |
| `src/task-scheduler.ts` | Runs scheduled tasks |
| `src/db.ts` | SQLite operations |
| `Dockerfile` | Debian 13 + Bun + Tailscale |
| `docker-compose.yml` | Container orchestration with workspace volume |
| `scripts/entrypoint.sh` | Tailscale + PocketBrain startup |

## Skills

| Skill | When to Use |
|-------|-------------|
| `/setup` | First-time installation, authentication, service configuration |
| `/customize` | Adding channels, integrations, changing behavior |
| `/debug` | Container issues, logs, troubleshooting |

## Development

Run commands directly — don't tell the user to run them.

```bash
bun run dev          # Run dev mode in Docker (watch mode)
bun run build        # Build Docker image
bun run docker:build # Build Docker image
bun run docker:up    # Start container (detached)
bun run docker:down  # Stop container
bun run docker:logs  # Tail container logs
bun run docker:test  # Run test suite in Docker
```

