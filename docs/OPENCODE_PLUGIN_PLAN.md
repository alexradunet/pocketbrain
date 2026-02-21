# OpenCode Plugin Architecture Plan for PocketBrain

## Goal
Explore a deep, production‑grade path to move PocketBrain’s core runtime (especially WhatsApp I/O) into an OpenCode plugin, while preserving current behavior.

## Scope
- Primary: WhatsApp channel as plugin‑owned long‑running connection.
- Secondary: Orchestration (queueing, routing, session lifecycle).
- Optional: Scheduling strategy (internal scheduler vs. opencode‑scheduler).

## Non‑Goals
- Refactoring existing PocketBrain to multiple channels.
- Replacing SQLite with a different DB.
- Rewriting OpenCode SDK or OpenCode core.

---

## Current State Summary (PocketBrain)
- **WhatsApp I/O**: long‑lived Baileys socket with auth on disk, reconnection, outbound queue, LID mapping. `src/channels/whatsapp.ts`
- **Routing**: chat‑jid ownership, XML formatting, internal tag stripping. `src/router.ts`
- **Session orchestration**: per‑group sessions, streaming output forwarding, idle timeouts. `src/opencode-manager.ts`, `src/index.ts`
- **Concurrency control**: `GroupQueue` ensures per‑group serialized processing. `src/group-queue.ts`
- **Scheduler**: internal loop + task store in SQLite; IPC to MCP. `src/task-scheduler.ts`, `src/ipc.ts`, `src/mcp-tools.ts`
- **Persistence**: SQLite for messages, tasks, sessions, cursors. `src/db.ts`

---

## Constraints & Assumptions
- OpenCode plugin runs inside OpenCode process and remains alive while OpenCode is running.
- OpenCode is started in a persistent mode (e.g. `opencode serve`) so the plugin can maintain the WhatsApp socket.
- Plugin API allows hooks + SDK access, but does not provide built‑in channel abstractions.
- WhatsApp auth material must be persisted somewhere under `.opencode/` or another durable path.

---

## Architecture Options

### Option A: Full Plugin (Single Process)
**Description**: Implement WhatsApp connection, routing, session orchestration, queueing, scheduling inside a plugin.

**Pros**
- Single runtime process (OpenCode).
- Unified configuration and packaging.
- No external “wrapper” process.

**Cons**
- Largest rewrite surface.
- Plugin lifecycle tied to OpenCode server uptime.
- Need to recreate PocketBrain orchestration (queue, DB, scheduler) in plugin context.

**When to choose**: Strong desire for “pure plugin” architecture and willingness to rebuild internal runtime.

---

### Option B: Hybrid Bridge + Plugin
**Description**: Keep a slim external WhatsApp bridge that speaks to OpenCode server via SDK; plugin provides tools / scheduling / UX enhancements.

**Pros**
- Much smaller rewrite.
- Can preserve current PocketBrain behavior with minimal changes.
- WhatsApp remains isolated from OpenCode runtime.

**Cons**
- Not “all inside OpenCode.”
- Two services to manage.

**When to choose**: Need production stability quickly with minimal risk.

---

## Deep Spike Plan (Full Plugin)

### Phase 0 — Discovery & Validation
1. **Confirm OpenCode plugin API surface**
   - Determine lifecycle hooks available (startup, shutdown, message, etc.).
   - Verify SDK usage inside plugin context.
   - Confirm storage locations and permissions for durable auth state.
2. **Confirm operational mode**
   - Validate that `opencode serve` keeps plugin resident for long‑running sockets.
   - Identify any plugin sandbox or process termination rules.

**Exit Criteria**
- Clear understanding of plugin lifecycle and persistence limits.

---

### Phase 1 — Minimal WhatsApp Plugin Spike
1. **Plugin scaffold**
   - Create a plugin package (local or npm) with a single entry.
   - Register on startup to initialize WhatsApp socket.
2. **Auth persistence**
   - Persist Baileys auth state under `.opencode/` or configurable path.
3. **Inbound flow**
   - On WhatsApp message, create or resume an OpenCode session via SDK.
   - Send prompt with a simple format (no DB yet).
4. **Outbound flow**
   - Stream results back to WhatsApp.
5. **Resilience**
   - Reconnect on disconnect.
   - Basic outbound queue while disconnected.

**Exit Criteria**
- WhatsApp <-> OpenCode loop works end‑to‑end in plugin.
- Basic reconnect and auth persistence verified.

---

### Phase 2 — Orchestration Parity
1. **Session mapping**
   - Map `chatJid` -> `sessionId` with a durable store.
2. **Concurrency control**
   - Port `GroupQueue` or equivalent per‑chat locking to prevent overlap.
3. **Routing & formatting**
   - Port `formatMessages`, `formatOutbound`, `stripInternalTags`.
4. **State recovery**
   - On startup, recover pending messages and resume sessions where possible.

**Exit Criteria**
- Multi‑chat routing works correctly.
- No duplicated or overlapped agent responses.

---

### Phase 3 — Scheduling Strategy
Choose one:

**A. Internal Scheduler (Plugin‑owned)**
- Port `task-scheduler.ts` to plugin.
- Use the same DB schema to track tasks and next run.
- Reuse MCP‑style tool surfaces inside plugin.

**B. External opencode‑scheduler Integration**
- Use opencode‑scheduler for OS‑level scheduling.
- On each scheduled run, the prompt is executed and the result is sent to WhatsApp by the plugin.

**Exit Criteria**
- Scheduled jobs work and can message the user reliably.

---

### Phase 4 — Persistence & Storage
- Decide between SQLite in plugin context or simplified JSON state.
- Migrate data model:
  - groups
  - sessions
  - messages
  - scheduled tasks

**Exit Criteria**
- Data survives restarts.
- State corruption is handled safely.

---

## Design Decisions to Resolve
- **Plugin packaging**: local repo plugin vs. npm package.
- **State storage**: SQLite vs. JSON for spike phase.
- **Scheduling**: internal vs. `opencode-scheduler`.
- **Auth path**: `.opencode/` vs. user‑defined path.
- **Session compaction strategy**: port current context‑prefix scheme or adopt a different prompt context strategy.

---

## Risks & Mitigations
- **OpenCode plugin lifecycle may not guarantee long‑running sockets**
  - Mitigation: run OpenCode in `serve` mode; add watchdog or reconnect logic.
- **In‑process failures affect OpenCode**
  - Mitigation: aggressive error handling, circuit breakers for WhatsApp errors.
- **Session overlap/duplication**
  - Mitigation: port `GroupQueue` semantics and per‑chat locks.
- **Auth state loss**
  - Mitigation: explicit auth path + file integrity checks.

---

## TDD Plan (If We Implement)
All changes follow repository TDD rules:
1. Add tests for new plugin behaviors (routing, session mapping, scheduling).
2. Validate failure cases (disconnects, duplicate messages, unauthorized routing).
3. Run `bun run docker:test` after each change.

---

## Deliverables
- `docs/OPENCODE_PLUGIN_PLAN.md` (this file)
- Spike plugin skeleton (if approved)
- Minimal working WhatsApp‑OpenCode loop inside plugin

---

## Open Questions
1. Should the spike be a **local plugin** in this repo or a **separate npm package**?
2. For the spike phase, should we use **SQLite** (parity) or **JSON** (speed)?
3. Scheduling: internal port vs. external `opencode-scheduler`?

