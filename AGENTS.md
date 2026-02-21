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

## Engineering Discipline — Test-Driven Development

**All changes — features, bug fixes, security patches, refactors — must follow TDD.**

### The Law
1. **Write a failing test first.** No production code before a test that demonstrates the problem or specifies the behavior. The test must fail before you write any implementation code.
2. **Write the minimal implementation** to make the test pass. Nothing more.
3. **Refactor** with all tests green.
4. **Run `bun run docker:test` after every change** to confirm no regressions.

### What this means in practice
- **Bug fix**: Write a test that reproduces the bug. Confirm it fails. Fix. Confirm it passes.
- **New feature**: Write a test that specifies the feature's behavior. Confirm it fails. Implement.
- **Security fix**: Write a test that demonstrates the vulnerability (e.g. path traversal input). Confirm it fails. Patch.
- **Refactor**: Tests must stay green throughout — no new tests needed if behavior is unchanged.

### What requires a test
Everything observable:
- New IPC task types or authorization rules (`ipc-auth.test.ts`)
- DB operations and schema changes (`db.test.ts`)
- Queue behavior (concurrency, retry, shutdown) (`group-queue.test.ts`)
- Message formatting and routing (`formatting.test.ts`, `routing.test.ts`)
- Any fix for a reported bug — the test documents what was broken

### What does NOT need a test
- Log messages
- Private helper functions with no observable side effects
- Code paths already covered transitively by existing tests

### Test files
| Test file | What it covers |
|-----------|----------------|
| `src/db.test.ts` | SQLite operations |
| `src/group-queue.test.ts` | GroupQueue concurrency and state |
| `src/ipc-auth.test.ts` | IPC authorization and task processing |
| `src/formatting.test.ts` | Message formatting |
| `src/routing.test.ts` | Channel routing logic |

