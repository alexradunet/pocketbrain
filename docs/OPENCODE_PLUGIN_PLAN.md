# OpenCode Plugin Architecture Plan for PocketBrain (Option A Only)

## Goal
Define a decision‑complete, production‑grade path to move PocketBrain’s core runtime (especially WhatsApp I/O) into a single OpenCode plugin, while preserving current behavior.

## Scope
- Primary: WhatsApp channel as plugin‑owned long‑running connection.
- Secondary: Orchestration (queueing, routing, session lifecycle).
- Scheduling: Internal scheduler parity with current PocketBrain.

## Non‑Goals
- Refactoring existing PocketBrain to multiple channels.
- Redesigning the existing SQLite schema (parity required in final phase).
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
- WhatsApp auth material must be persisted under a durable plugin data path (default below).

---

## Deep Analysis (Option A Only)

### Process Model and Lifecycle
- The plugin is loaded at OpenCode startup and runs inside the OpenCode process.
- The WhatsApp socket must remain resident for real‑time messaging.
- **Operational requirement**: OpenCode must run in a persistent server mode (`opencode serve` or equivalent) for the socket to remain active.

### In‑Process Risk
- Any unhandled error in the plugin can affect OpenCode.
- Mitigation: strict error boundaries, backoff‑driven reconnects, and circuit breakers for repeated failures.

### Data Flow (End‑to‑End)
1. WhatsApp message arrives → plugin inbound handler.
2. Per‑chat queue serializes processing.
3. Session lookup (chatJid → sessionId) or session creation via SDK.
4. Prompt formatting and OpenCode SDK prompt call.
5. Streaming output to WhatsApp via plugin.

### State and Persistence
- **Phase 1–3**: JSON files for speed of iteration.
- **Phase 4**: SQLite parity using existing PocketBrain schema.
- **Default data root**: `.opencode/data/pocketbrain/`

### Security & Authorization
- Auth state must be persisted with atomic writes.
- All session and task operations are scoped to the originating chat JID.
- No cross‑chat message delivery without explicit authorization.

---

## Architecture Overview (Single Plugin)

Components (plugin‑internal modules):
- **WhatsAppAdapter**: Baileys socket, auth persistence, reconnect, outbound queue.
- **SessionManager**: chatJid ↔ sessionId mapping, context prefix, session recovery.
- **GroupQueue**: per‑chat serialization, global concurrency cap.
- **Router**: format inbound XML, strip internal tags, format outbound.
- **Scheduler**: internal loop for tasks.
- **StateStore**: JSON store in early phases → SQLite adapter for parity.

Textual flow:
```
WhatsApp → WhatsAppAdapter → GroupQueue → SessionManager → OpenCode SDK
                                   ↘ Router ↗
OpenCode SDK → SessionManager → WhatsAppAdapter → WhatsApp
```

---

## Implementation Plan (Option A)

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
   - Create a **local repo plugin** under `.opencode/plugins/pocketbrain/`.
   - Single entry that initializes WhatsApp socket on plugin startup.
2. **Auth persistence**
   - Persist Baileys auth under `.opencode/data/pocketbrain/auth/`.
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
   - Map `chatJid` -> `sessionId` with JSON store under `.opencode/data/pocketbrain/state/`.
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

### Phase 3 — Internal Scheduling (Parity)
- Port `task-scheduler.ts` to plugin.
- Track tasks in JSON store initially.
- Provide tool‑level APIs (plugin hooks) to schedule, pause, resume, cancel tasks.
- Ensure scheduled runs deliver output to WhatsApp (same behavior as current scheduler).

**Exit Criteria**
- Scheduled jobs work and can message the user reliably.

---

### Phase 4 — Persistence & Storage
- Replace JSON store with SQLite using existing PocketBrain schema.
- Migration plan:
  - export JSON state to SQLite tables
  - verify parity for sessions, messages, and tasks
- Maintain compatibility with existing database consumers (if any).

**Exit Criteria**
- Data survives restarts.
- State corruption is handled safely.

---

## Public Interfaces and APIs
- **Plugin entrypoint**: `.opencode/plugins/pocketbrain/index.ts`
- **Module layout**:
  - `.opencode/plugins/pocketbrain/whatsapp.ts`
  - `.opencode/plugins/pocketbrain/session-manager.ts`
  - `.opencode/plugins/pocketbrain/group-queue.ts`
  - `.opencode/plugins/pocketbrain/router.ts`
  - `.opencode/plugins/pocketbrain/scheduler.ts`
  - `.opencode/plugins/pocketbrain/state-store.ts`
- **OpenCode SDK usage**:
  - `client.session.create`
  - `client.session.get`
  - `client.session.promptAsync`
  - `client.session.abort`

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

## Open Questions (Decision Required)
1. Should auth data be encrypted at rest in `.opencode/data/pocketbrain/auth/`?
2. Should we support a configurable data root path via env var?
