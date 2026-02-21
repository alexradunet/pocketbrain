# 🌳 Level 3 — Senior / Architect Guide

> **Who this is for:** A senior engineer or system designer who wants to
> understand *why* PocketBrain is built the way it is — the tradeoffs,
> constraints, and design principles behind every major decision.
> Assumes you've read [🌿 Level 2](./GUIDE_INTERMEDIATE.md).
> See the [emoji legend](./GUIDE.md#️⃣-emoji-concept-legend) for visual anchors.

---

## 💡 Core Architectural Thesis

PocketBrain is built on a single principle: **complexity is the enemy of
security and understandability**. Every design decision traces back to this.

The project exists as a reaction to OpenClaw/ClawBot — a system that grew
to 52+ modules, 8 config management files, 45+ dependencies, and application-
level permission checks trying to prevent agents from accessing things they
shouldn't. The complexity made it impossible to audit, understand, or trust.

PocketBrain's answer: **fewer moving parts, stronger boundaries**.

---

## 🏗️ Macro Architecture: Why a Single Process?

### The Alternative
A naive design would run each agent invocation in its own process (or
container), with a message broker (Redis, RabbitMQ) between the 💬 WhatsApp
receiver and 🧠 agent runners. This is what OpenClaw does.

### 💡 Why PocketBrain Doesn't

**1. Complexity scales with components.** Every additional process boundary
adds serialization, network calls, failure modes, and debugging surface.
A message broker has its own operations burden. One process eliminates all of this.

**2. The 🐳 container IS the sandbox.** The security argument for per-invocation
containers (isolate each agent run) is compelling, but Docker-in-Docker is
operationally painful. Instead, PocketBrain runs ONE container that provides
OS-level isolation. Agents run with full power inside, sandboxed from the host.

**3. 🗄️ SQLite is sufficient.** SQLite in WAL mode handles the read-write
patterns here easily — one writer (message loop), multiple readers (scheduler,
IPC watcher). No connection pooling required.

**4. Shared memory is fine at this scale.** State is small (hundreds of
messages, tens of sessions). A global in-memory `Map` for active 🔄 sessions
is far simpler than distributed state.

> ⚠️ **Tradeoff accepted:** Single-process means a crash takes down everything.
> Acceptable for a personal-use tool — Docker's `restart: unless-stopped`
> handles it. All durable state is in 🗄️ SQLite and files.

---

## 📁 The IPC Design: Why Files?

### The Problem
The 🧠 AI agent (running inside OpenCode SDK) needs to trigger side effects on
the host — send 💬 WhatsApp messages, create ⏰ scheduled tasks, register 👥 groups.
How does the agent communicate with the host process?

### Options Considered

| Option | Problem |
|--------|---------|
| Shared memory | Only works within one process |
| Unix sockets | Requires connection management, auth |
| HTTP to host | Port conflicts, adds a server to the host |
| 📁 File-based IPC | Simple, atomic, no connections, no auth tokens |

### 💡 Why Files Won

**⚡ Atomicity via `rename`.** A file write is not atomic — a reader might see
a half-written file. But `rename()` is atomic on POSIX filesystems. The 🔌 MCP
server writes to `.json.tmp`, then renames to `.json`. The 📁 IPC watcher only
processes `.json` files, so it never sees partial writes.

**🛡️ Identity from path, not content.** The 📁 IPC watcher determines 👥 group
identity from the **directory** the file was written to
(`data/ipc/main/tasks/`) — not from what the file claims. This prevents
privilege escalation: an agent cannot forge a different group identity by
writing a JSON field.

**🔁 Durability across restarts.** If the process crashes between the agent
writing a file and the watcher processing it, the file survives. On restart,
the watcher processes it.

> ⚠️ **Tradeoff accepted:** 1-second poll latency on IPC (acceptable for
> messaging). Startup cleanup needed for orphaned `.json.tmp` files.

---

## 🔄 Session Management Design

### The Problem
OpenCode SDK sessions have a context window. Long conversations cause the
session to "compact" — summarize old content to free context. After
compaction, information the 🧠 agent needs (👥 group identity, chatJid, etc.)
may be lost.

### 💡 The Solution: Context Re-injection

Every prompt sent to the agent — **including follow-ups** — prepends a
`<pocketbrain_context>` XML block:

```typescript
function buildContextPrefix(group, input): string {
  return `<pocketbrain_context>
chatJid: ${input.chatJid}
groupFolder: ${input.groupFolder}
isMain: ${input.isMain}
...
</pocketbrain_context>`;
}
```

This is **stateless** from the host's perspective — it doesn't matter whether
the agent remembers the context from before compaction, because the host
re-injects it every time.

> ⚠️ **Tradeoff:** Slightly larger prompt on every follow-up. Negligible in
> practice (< 200 characters).

### 🔄 Session Persistence

Session IDs are stored in 🗄️ SQLite (`sessions` table). On restart, all
sessions are loaded and resumed on the next message. The OpenCode SDK server
recreates session state from its own store.

This gives conversational continuity across process restarts without the
host storing full conversation history.

---

## 🔀 GroupQueue: Concurrency Design

### The Problem
Multiple 👥 WhatsApp groups can receive 💬 messages simultaneously. Each group
needs its own 🧠 agent session. But running unlimited concurrent sessions would
exhaust memory, overload the OpenCode server, and hit API rate limits.

### 💡 The Design

`GroupQueue` implements a **two-level** concurrency model:

**Level 1: Per-group exclusivity.** Only one 🔄 session runs per 👥 group at
a time. Messages for an active group are buffered (`pendingMessages = true`)
and processed immediately after the current session ends. This ensures
message ordering within a group.

**Level 2: Global cap.** `MAX_CONCURRENT_SESSIONS` (default: 5) limits total
concurrent sessions. 👥 Groups beyond the cap join `waitingGroups` (FIFO).
When a session completes, the next waiting group gets a slot.

**Priority:** ⏰ Scheduled tasks are drained before pending 💬 messages within
a group. Rationale: tasks are enqueued with specific timing and shouldn't
wait behind an interactive session indefinitely.

**🔁 Retry with exponential backoff:** Agent failures are retried up to 5
times with delays of 5s, 10s, 20s, 40s, 80s. After 5 failures, messages
are dropped with a warning (they'll retry on the next incoming message).

> ⚠️ **Tradeoff:** Message ordering is strictly preserved within a group.
> This is correct behavior for a chat assistant. The global cap is a
> configuration knob to tune for available memory/API quotas.

---

## 🛡️ Security Architecture

### The Threat Model

| Threat | Mitigation |
|--------|-----------|
| Malicious user in a 👥 group | Non-main groups cannot control other groups; 🎯 trigger required |
| Prompt injection via 💬 messages | XML-escaped message content; agent output filtered |
| Agent exceeds its authority | 📁 IPC authorization from directory identity, not agent claims |
| 🐳 Container escape | Not in scope (Docker security boundary) |
| Host credential leak | WhatsApp auth never mounted in agent context |

### 💡 Why Container Isolation > Application Permissions

OpenClaw tried to prevent agents from accessing files via allowlists and
permission checks in application code. This is inherently fragile — a clever
prompt might find an indirect path to restricted operations.

PocketBrain takes the opposite approach: the agent **can** do anything in
the 🐳 container, and the container **cannot** do anything to the host beyond
the explicit volume mount. There's no permission check to bypass; the OS
enforces the boundary.

> ⚠️ **Tradeoff accepted:** The agent can modify anything in `/workspace`.
> This is acceptable — the user chose to give the agent access to that
> directory, just as they'd choose what to put in a shared folder with a
> trusted collaborator.

### 🛡️ IPC Authorization Layers

```
🧠 Agent (inside OpenCode)
  │
  │ writes JSON to data/ipc/{sourceGroup}/tasks/
  │
📁 IPC Watcher
  │
  ├─ Identity: sourceGroup = directory name (🛡️ OS-enforced, not agent-claimed)
  │
  ├─ isMain = (sourceGroup === MAIN_GROUP_FOLDER)
  │
  └─ Authorization table:
      schedule_task:  targetFolder === sourceGroup OR isMain
      cancel_task:    task.group_folder === sourceGroup OR isMain
      register_group: isMain only 👑
      refresh_groups: isMain only 👑
```

**Path traversal defense** in the 🔌 MCP server:
```typescript
function safeFolder(folder: string): string {
  const sanitized = path.basename(folder);  // strips any ../../..
  if (!sanitized || sanitized === '.' || sanitized === '..') throw ...;
  return sanitized;
}
```
`path.basename()` extracts only the last path component, so
`../../etc/passwd` becomes `passwd` which fails to match any group directory.

---

## 🔄 The Two-Timestamp Cursor System

### Why Two Cursors?

```
Timeline of 💬 messages in a group:
  [msg1][msg2][msg3][msg4][msg5][msg6]
                ↑                 ↑
    lastAgentTimestamp       lastTimestamp
    (🧠 agent processed        (💬 WhatsApp saw
      up to here)               up to here)
```

**`lastTimestamp`** (global): Advances immediately when any new 💬 messages
are seen. Prevents re-processing in the poll loop.

**`lastAgentTimestamp[groupJid]`** (per-group): Advances only after the 🧠
agent successfully processes a batch. The "pending context"
(`msg4, msg5, msg6` in the example) accumulates between 🎯 trigger invocations
and is included in full when the next trigger arrives.

**🔁 The cursor rollback pattern:**
- Agent fails **before** any output → `lastAgentTimestamp` rolls back → 🔁 retry
- Agent fails **after** sending output → cursor **not** rolled back (no duplicates)

> 💡 This is a deliberate "at-most-once retry after user sees output" guarantee.

---

## 📡 OpenCode SDK Integration Pattern

### Server Lifecycle

The OpenCode server is embedded in-process via `createOpencode()`. It runs
an HTTP server on `127.0.0.1:4096` (localhost only). This is not a remote
API call — it's an in-process server that the SDK starts. 🧠

### 📡 SSE Streaming vs Final Fetch

Response collection has two layers for resilience:

**Layer 1 (📡 SSE stream):** Real-time `message.part.updated` deltas accumulate
text as the model generates. When `session.idle` fires, streaming stops.

**Layer 2 (✅ canonical fetch):** After streaming, `client.session.message()`
fetches the canonical final message state. Handles cases where the SSE
stream was interrupted or delivered out-of-order events.

```typescript
const canonicalText = extractTextFromParts(messageRespData?.parts ?? []);
const streamedText = joinTextParts(textParts, textPartOrder);
const fullText = canonicalText || streamedText;  // ✅ canonical wins
```

**⏳ Timeouts:**

| Operation | Timeout | Rationale |
|-----------|---------|-----------|
| Session create/resume | 15s | Hangs would block a 👥 group indefinitely |
| Prompt stream | 120s | Long 🧠 agent runs need time |
| Canonical fetch | 30s | Final safety net |

### 🔌 MCP Server as Child Process

The 🔌 MCP server (`src/mcp-tools.ts`) runs as a **stdio** child process of the
OpenCode server. Why stdio?
- No ports, no authentication
- Process lifecycle tied to the parent
- When OpenCode terminates, the 🔌 MCP server terminates automatically

---

## 🧩 The Skills Architecture

### Design Problem
Every user wants different integrations (Telegram, Gmail, Slack). Adding all
of them to the core codebase creates bloat. But a plugin system adds
complexity.

### 💡 The Solution: Code Transformation Skills

Instead of a plugin system, PocketBrain uses OpenCode's native 🧩 skills system.
A skill is a `SKILL.md` file containing instructions for the 🧠 AI agent to
modify the codebase. When a user runs `/add-telegram`, the agent:

1. Reads the SKILL.md instructions 📝
2. Modifies `src/channels/` to add a Telegram channel 💬
3. Updates `src/index.ts` to register the new channel 🔄
4. Runs tests ✅

The user ends up with **clean code** that does exactly what they need — no
dead code for features they don't use, no conditional branching for different
providers.

> ⚠️ **Tradeoff:** Skills are one-directional — they add/replace code. There's
> no version management. Fine for a personal-use project where you fork and
> own the code.

---

## 🔌 Extension Points

If you're planning to extend PocketBrain, here are the designed surfaces:

### 💬 Adding a New Channel

1. Implement the `Channel` interface (`src/types.ts:46`):
   ```typescript
   interface Channel {
     name: string;
     connect(): Promise<void>;
     sendMessage(jid: string, text: string): Promise<void>;
     isConnected(): boolean;
     ownsJid(jid: string): boolean;  // 🔀 routes by JID pattern
     disconnect(): Promise<void>;
     setTyping?(jid: string, isTyping: boolean): Promise<void>;
   }
   ```
2. Call `connect()` in `main()` and add to `channels[]`
3. The router (`src/router.ts:39`) selects the right channel by `ownsJid()`

### 🔌 Adding New MCP Tools

Add `server.tool(...)` calls in `src/mcp-tools.ts`. The tool is immediately
available to the 🧠 agent. Follow the 📁 IPC file pattern for operations that
need the host process.

### 📁 Adding New IPC Operations

Add a new `case` in `src/ipc.ts:processTaskIpc()`. Always check `isMain`
and `sourceGroup` against 🛡️ authorization requirements.

### 🧠 Changing the AI Model

Set `OPENCODE_MODEL` and `OPENCODE_API_KEY` in `.env`. The model is passed
through `createOpencode()` config. OpenCode SDK is model-agnostic.

---

## 🚫 What's NOT in PocketBrain (Intentionally)

| Feature | Why It's Absent |
|---------|----------------|
| Message broker (Redis/RabbitMQ) | Adds operational complexity for no functional gain |
| WebSocket/HTTP API server | No external consumers; 📁 IPC is sufficient |
| Config management system | Code changes are cleaner than config files |
| Plugin registry | 🧩 Skills do the same thing without a plugin runtime |
| Per-invocation containers | 🐳 Container IS the sandbox; re-spawning adds latency |
| Multi-user auth | Built for one user; YAGNI |
| Monitoring dashboard | Ask the 🧠 AI ("what's in the logs?") |
| Admin UI | 👑 Main WhatsApp group IS the admin UI |

---

## ⚡ Performance Characteristics

| Operation | Latency | Notes |
|-----------|---------|-------|
| 💬 Message detection | ~2s | Poll interval |
| 📁 IPC processing | ~1s | Poll interval |
| 🔄 Session creation | ~1-3s | Network + SDK init |
| 📡 First token from model | 2-10s | Model dependent |
| 🧠 Full response (simple) | 5-30s | Task complexity |
| ⏰ Scheduled task detection | ~60s | Scheduler poll |

> 💡 The system is optimized for **correctness and simplicity** over low
> latency. A 2-second poll is fine for a conversational assistant.

---

## ⚠️ Known Constraints and Future Work

**Context window pressure:** Long conversations eventually trigger OpenCode's
compaction. The `pocketbrain_context` re-injection handles this, but
very long-running sessions may lose non-injected context. Per-group 📝
`AGENTS.md` files help keep important context durable.

**No delivery receipts:** WhatsApp delivery/read receipts are not tracked.
The system sends 💬 and moves on.

**Single 🧠 LLM endpoint:** All 👥 groups share one OpenCode SDK instance.
Different models per group would require multiple instances.

**⏰ Scheduled task drift:** `interval` tasks are anchored to the previous
`next_run` (not wall clock) to prevent drift. `cron` tasks always compute
from the expression. `once` tasks are one-shot.

---

*Back to [🗺️ Guide Index](./GUIDE.md)*
