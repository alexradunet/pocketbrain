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

## Documentation — Keep the Guides Current

The `docs/` directory contains a five-file guide suite that serves as both
architecture reference and a learning resource for building personal AI agents.
**Every code change that affects behaviour, interfaces, or design must update
the relevant guide(s) before the work is considered done.**

### Guide files and their scope

| File | Update when… |
|------|-------------|
| `docs/GUIDE.md` | The emoji legend, file map, or architecture diagram changes |
| `docs/GUIDE_JUNIOR.md` | User-facing concepts change (trigger word, groups, sessions, tools, data paths) |
| `docs/GUIDE_INTERMEDIATE.md` | Component responsibilities, SQLite schema, config vars, code references, or message flow change |
| `docs/GUIDE_ARCHITECT.md` | A design decision is added/changed, a tradeoff changes, or an extension point is added |
| `docs/GUIDE_BUILDER.md` | A reusable pattern changes (IPC, MCP tools, sessions, scheduling, concurrency, security) |

### Style rules — match the existing guides exactly

1. **Use the emoji system as concept anchors.** Every guide uses the same
   emoji legend defined in `docs/GUIDE.md`. Do not invent new emoji for
   existing concepts. Do not drop emoji from existing sections.

   | Emoji | Concept |
   |-------|---------|
   | 💬 | WhatsApp message / chat |
   | 🧠 | AI agent / OpenCode |
   | 🗄️ | SQLite / database |
   | 📁 | Files / IPC |
   | ⏰ | Scheduler / cron |
   | 🐳 | Docker container |
   | 🌐 | Web / network |
   | 🎯 | Trigger word |
   | 👥 | WhatsApp group |
   | 👑 | Main group (admin) |
   | 🔌 | MCP tools |
   | 🧩 | Skills / extensions |
   | 🔄 | Session / state |
   | 🔀 | Queue / concurrency |
   | 🔑 | Config / env vars |
   | 📝 | AGENTS.md / memory |
   | 🛡️ | Security / authorization |
   | 📡 | SSE streaming |
   | 🔁 | Retry / backoff |
   | 🚀 | Startup / boot |
   | ⚡ | Performance |
   | 💡 | Key insight / design decision |
   | ⚠️ | Tradeoff / warning |

2. **Code references use `file:line` format.**
   `src/opencode-manager.ts:409` not just "in opencode-manager".
   Update line numbers when code moves.

3. **Tables over prose for structured data.**
   Config vars, schema columns, auth rules, timing values → tables.

4. **Three-part structure in `GUIDE_BUILDER.md`.**
   Every pattern section must have: 🎓 The Concept, 📐 The Pattern
   (generic code), 🔍 PocketBrain Implementation, ✅ The Lesson.

5. **`⚠️` for every tradeoff.** When a design decision has a downside,
   call it out with `> ⚠️ **Tradeoff accepted:**`.

6. **`💡` for every non-obvious insight.** When explaining *why* something
   is done a certain way, lead with `> 💡`.

### What does NOT need a doc update

- Log message wording changes
- Test-only changes with no behaviour difference
- Dependency version bumps with no API surface change
- Refactors that preserve all existing behaviour exactly

