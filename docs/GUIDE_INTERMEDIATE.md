# 🌿 Level 2 — Intermediate Developer Guide

> **Who this is for:** A developer comfortable with TypeScript/Node who wants
> to understand how PocketBrain is implemented well enough to modify it.
> Assumes you've read [🌱 Level 1](./GUIDE_JUNIOR.md).
> See the [emoji legend](./GUIDE.md#️⃣-emoji-concept-legend) for visual anchors.

---

## 🏗️ System Components

PocketBrain is **one Bun process** with several internal subsystems:

```
┌─────────────────────────────────────────────────────────────────┐
│              🔄 src/index.ts (Orchestrator)                      │
│                                                                  │
│  ┌──────────────┐  ┌──────────────────┐  ┌──────────────────┐  │
│  │ 💬 WhatsApp  │  │  🔀 GroupQueue   │  │  ⏰ Scheduler    │  │
│  │   Channel    │  │  (concurrency)   │  │  (cron/interval) │  │
│  └──────┬───────┘  └────────┬─────────┘  └────────┬─────────┘  │
│         │                   │                      │            │
│         ▼                   ▼                      ▼            │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                  🗄️ SQLite (src/db.ts)                    │  │
│  └──────────────────────────────────────────────────────────┘  │
│                              │                                   │
│                              ▼                                   │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │       🧠 OpenCode Manager (src/opencode-manager.ts)       │  │
│  │  • OpenCode SDK server (port 4096)                        │  │
│  │  • 🔄 Session create/resume/prompt/abort                  │  │
│  │  • 📡 SSE event streaming                                 │  │
│  └──────────────────────────────────────────────────────────┘  │
│                              │  ▲                               │
│                    📁 IPC    │  │                               │
│                   (JSON)     │  │                               │
│                              ▼  │                               │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │            📁 IPC Watcher (src/ipc.ts)                    │  │
│  │  • Polls data/ipc/*/messages/*.json every 1s              │  │
│  │  • Polls data/ipc/*/tasks/*.json every 1s                 │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                               │  ▲
               🔌 MCP stdio   │  │
                               ▼  │
┌─────────────────────────────────────────────────────────────────┐
│         🔌 MCP Server (src/mcp-tools.ts) — child process         │
│  • send_message    → writes 📁 messages/*.json                   │
│  • schedule_task   → writes 📁 tasks/*.json                      │
│  • list_tasks      → reads  📁 current_tasks.json                │
│  • pause/resume/cancel_task                                      │
└─────────────────────────────────────────────────────────────────┘
```

---

## 🗺️ Message Flow (With Code References)

### Step 1 — 💬 Receiving a WhatsApp message

`src/channels/whatsapp.ts` — `WhatsAppChannel.connectInternal()`

The Baileys library emits `messages.upsert` events. For each message:
1. JID is translated (LID → phone number for newer WhatsApp accounts)
2. `onChatMetadata()` is called for 👥 group discovery
3. If the group is registered, `onMessage()` stores the full message in 🗄️ SQLite

```typescript
// whatsapp.ts:153
this.sock.ev.on('messages.upsert', async ({ messages }) => {
  for (const msg of messages) {
    const chatJid = await this.translateJid(rawJid);  // LID → phone
    this.opts.onChatMetadata(chatJid, timestamp, ...); // 👥 discover group
    if (groups[chatJid]) {
      this.opts.onMessage(chatJid, { id, chat_jid, sender, content, ... });
    }
  }

});
```

### Step 2 — 🗄️ Storing the message

`src/db.ts` — `storeMessage()`

Messages are written to SQLite with a timestamp-based index. The message
loop later polls for messages newer than `lastTimestamp`.

### Step 3 — 🔄 The message loop detects new messages

`src/index.ts` — `startMessageLoop()`

Polls every **2 seconds** (`POLL_INTERVAL = 2000`). For each registered
👥 group with new messages:

```typescript
// index.ts:310
const { messages, newTimestamp } = getNewMessages(jids, lastTimestamp);
// Groups new messages by chat JID
// → Active session? pipes as follow-up
// → No session? enqueues new one via 🔀 GroupQueue
```

**Two paths:**
- 🔄 **Active session exists** → `queue.sendMessage()` → pipes as follow-up
- 🆕 **No session** → `queue.enqueueMessageCheck()` → starts new session

### Step 4 — 🔀 GroupQueue manages concurrency

`src/group-queue.ts` — `GroupQueue`

The queue ensures:
- Only **one session** runs per 👥 group at a time
- Globally, **`MAX_CONCURRENT_SESSIONS`** (default: 5) sessions run at once
- 🔁 Failed sessions retry with exponential backoff (5s, 10s, 20s… up to 5 retries)

### Step 5 — 🧠 Agent runs via OpenCode SDK

`src/opencode-manager.ts` — `startSession()` and `runPrompt()`

```typescript
// opencode-manager.ts:199 — new session
const resp = await client.session.create({ body: { title: `PocketBrain: ${group.name}` } });

// opencode-manager.ts:190 — resume existing session
await client.session.get({ path: { id: input.sessionId } });

// opencode-manager.ts:409 — send prompt
await client.session.promptAsync({ path: { id }, body: { messageID, parts: [{ type: 'text', text }] } });
```

📡 The response is collected via SSE (Server-Sent Events):
- `message.part.updated` events accumulate text delta by delta
- `session.idle` event signals the 🧠 agent has finished
- A fallback `client.session.message()` fetch gets the canonical final result

### Step 6 — 🔌 Agent uses MCP tools

When the agent calls `send_message` or `schedule_task`:
1. **🔌 MCP server** (`src/mcp-tools.ts`) writes a JSON file **atomically**:
   ```typescript
   // mcp-tools.ts:26
   const tempPath = `${filepath}.tmp`;
   fs.writeFileSync(tempPath, JSON.stringify(data, null, 2)); // write
   fs.renameSync(tempPath, filepath);  // ⚡ atomic rename (POSIX)
   ```
2. **📁 IPC watcher** (`src/ipc.ts`) polls every second, finds the file, executes
3. File is deleted after processing (or moved to `📁 errors/` on failure)

### Step 7 — 💬 Sending the response

`src/index.ts` — `processGroupMessages()` callback

```typescript
// index.ts:180
if (result.result) {
  // Strip <internal>...</internal> — agent's private reasoning
  const text = raw.replace(/<internal>[\s\S]*?<\/internal>/g, '').trim();
  await channel.sendMessage(chatJid, text); // 💬 sends to WhatsApp
}
```

> 💡 The `<internal>...</internal>` tag lets the 🧠 agent think out loud
> without those thoughts reaching the user.

---

## 🗄️ Data Model

### SQLite Tables (`src/db.ts`)

```sql
-- 💬 All messages received from registered groups
CREATE TABLE messages (
  id TEXT, chat_jid TEXT, sender TEXT, sender_name TEXT,
  content TEXT, timestamp TEXT,
  is_from_me INTEGER, is_bot_message INTEGER,
  PRIMARY KEY (id, chat_jid)
);

-- 👥 All chats seen (for group discovery, even unregistered)
CREATE TABLE chats (
  jid TEXT PRIMARY KEY, name TEXT,
  last_message_time TEXT, channel TEXT, is_group INTEGER
);

-- ⏰ Scheduled tasks
CREATE TABLE scheduled_tasks (
  id TEXT PRIMARY KEY, group_folder TEXT, chat_jid TEXT,
  prompt TEXT, schedule_type TEXT, schedule_value TEXT,
  context_mode TEXT,  -- 'group' (uses chat history) or 'isolated' (fresh)
  next_run TEXT, last_run TEXT, last_result TEXT,
  status TEXT         -- 'active', 'paused', 'completed'
);

-- 🔄 Per-group OpenCode session IDs (for resuming conversations)
CREATE TABLE sessions (
  group_folder TEXT PRIMARY KEY, session_id TEXT
);

-- 👥 Chats that PocketBrain responds to
CREATE TABLE registered_groups (
  jid TEXT PRIMARY KEY, name TEXT, folder TEXT UNIQUE, added_at TEXT
);

-- 🔄 Key-value store for runtime state (last_timestamp, etc.)
CREATE TABLE router_state (key TEXT PRIMARY KEY, value TEXT);
```

### 🔄 Dual Timestamp Cursor System

PocketBrain tracks **two cursors** per group:

| Cursor | Scope | Advances when… |
|--------|-------|----------------|
| `lastTimestamp` | Global | Any new 💬 message is seen (immediately) |
| `lastAgentTimestamp[groupJid]` | Per-group | 🧠 Agent successfully processes a batch |

💡 The gap between the two is **unprocessed context** — all messages since
the last agent run are included next time, so no message is ever missed
even if the agent was busy with another session.

---

## 🔑 Configuration

All config in `src/config.ts`, driven by environment variables:

| Variable | Default | Purpose |
|----------|---------|---------|
| `ASSISTANT_HAS_OWN_NUMBER` | `false` | Bot has its own phone number |
| `IDLE_TIMEOUT` | `1800000` | ⏳ Session idle timeout (30 min) |
| `MAX_CONCURRENT_SESSIONS` | `5` | 🔀 Global concurrency limit |
| `OPENCODE_API_KEY` | — | 🔑 LLM API key |
| `OPENCODE_MODEL` | — | 🧠 Model override |
| `OPENCODE_BASE_URL` | — | 🌐 API base URL override |
| `TZ` | system | ⏰ Timezone for cron tasks |
| `WORKSPACE_DIR` | `/workspace` | 📁 Root data directory |

---

## 🧠 OpenCode SDK Integration

The OpenCode SDK is initialized in `boot()` with a full config object:

```typescript
// opencode-manager.ts:104
const config = {
  permission: { edit: 'allow', bash: 'allow', webfetch: 'allow' },
  mcp: {
    pocketbrain: {
      type: 'local',
      command: [mcpServerPath],           // 🔌 runs mcp-tools.ts as child
      environment: { POCKETBRAIN_IPC_DIR: ipcDir },
    },
  },
  tools: { bash, edit, write, read, glob, grep, websearch, webfetch, task },
  instructions: [globalAgentsPath],       // 📝 loads groups/global/AGENTS.md
};
opencodeInstance = await createOpencode({ hostname: '127.0.0.1', port: 4096, config });
```

**🔄 Context re-injection on every prompt:**
To survive session compaction (when the AI's context window fills up and
it summarizes), PocketBrain re-injects a `<pocketbrain_context>` XML block
with **every** follow-up prompt:

```typescript
// opencode-manager.ts
function buildContextPrefix(group, input): string {
  return `<pocketbrain_context>
chatJid: ${input.chatJid}
groupFolder: ${input.groupFolder}
...
</pocketbrain_context>`;
}
```

💡 This ensures the 🔌 MCP tools always have the correct 👥 chat identity
to authorize operations against, even after context compaction.

---

## 🛡️ IPC Authorization Model

The 📁 IPC watcher enforces security from **directory path identity** — not
from what the agent *claims* in the file content. The source chat is
determined by the directory the IPC file was written to, not by any field
inside the file.

| 🔌 Operation | Result |
|-----------|--------|
| Send 💬 to own chat | ✅ |
| Send 💬 to other chats | ❌ blocked |
| ⏰ Schedule task for self | ✅ |
| ⏰ Schedule task for others | ❌ blocked |
| Cancel task in own chat | ✅ |
| Cancel task in other chat | ❌ blocked |

---

## ⏰ Task Scheduler

`src/task-scheduler.ts` — `startSchedulerLoop()`

Runs a loop every **60 seconds**. For each due task:

1. Re-reads from 🗄️ DB (checks it hasn't been paused/cancelled)
2. Calls `queue.enqueueTask()` — same 🔀 GroupQueue as messages
3. Runs `startSession()` with `isScheduledTask: true`
4. Output is forwarded to the 👥 group's WhatsApp chat via `sendMessage` 💬

**Schedule types:**

| Type | `schedule_value` example | ⏰ Recalculation |
|------|--------------------------|----------------|
| `cron` | `"0 9 * * *"` (daily 9am) | Next from cron expression |
| `interval` | `"3600000"` (every hour) | Anchored to previous `next_run` (⚡ no drift) |
| `once` | `"2026-02-01T15:30:00"` | No next run (deleted after) |

---

## 🔄 Session Lifecycle

```
🚀 startSession() called
        │
        ├─ sessionId provided? ──► client.session.get() (🔄 resume)
        │
        └─ no sessionId ──────► client.session.create() (🆕 new)
               │
               ▼
        Register in activeSessions Map 🗄️
               │
               ▼
        runPrompt() ──► 📡 SSE stream ──► collect text
               │
               ▼
        onOutput(result) ──► 💬 send to WhatsApp
               │
               ▼
        ⏳ Wait for endPromise
           (resolved by abortSession or shutdown)
               │
               ▼
        Remove from activeSessions 🔄
```

An ⏳ idle timer (`IDLE_TIMEOUT = 30min`) calls `abortSession()` if no
output arrives, preventing zombie 🧠 sessions.

---

## ✅ Testing

Tests use Bun's built-in test runner. Run with:
```bash
bun run docker:test
```

| Test file | What it covers |
|-----------|----------------|
| `src/db.test.ts` | 🗄️ SQLite schema, CRUD, timestamps |
| `src/group-queue.test.ts` | 🔀 Concurrency, retry backoff, drain logic |
| `src/ipc-auth.test.ts` | 🛡️ IPC authorization rules (cross-chat blocking) |
| `src/formatting.test.ts` | 💬 XML escaping, message formatting |
| `src/routing.test.ts` | 🔀 Channel routing by JID |

⚠️ **TDD is mandatory.** Every bug fix and feature requires a failing test
first. See `AGENTS.md` for the full TDD law.

### 🧪 End-to-End Tests

E2E tests run PocketBrain inside Docker with `CHANNEL=mock`, replacing
WhatsApp with an HTTP test double (`src/channels/mock.ts`). The mock
exposes `POST /inbox` (inject a message) and `GET /outbox` (capture the
agent's replies) — no phone or WhatsApp session needed.

```bash
bun run e2e            # cloud LLM (needs OPENCODE_API_KEY + ANTHROPIC_API_KEY)
bun run e2e:local      # local LLM via Ollama — no cloud key for the agent
bun run e2e:down       # reset volumes
```

| Test file | What it covers | Needs API key? |
|-----------|----------------|----------------|
| `src/e2e/agent.test.ts` | AI quality: math, geography, multi-turn | Yes — uses `llmAssert()` (Claude Haiku judge) |
| `src/e2e/infra.test.ts` | Routing, outbox delivery, session continuity | No — string-presence checks only |

**Local LLM option:** `bun run e2e:local` adds an Ollama service to the
compose stack and defaults to `qwen2.5:3b` (1.9 GB). Override with
`OLLAMA_MODEL=qwen2.5:1.5b` for a lighter model. Only `infra.test.ts`
runs in the local stack (AI quality assertions need a capable model).

Key files:
```
src/channels/mock.ts       — MockChannel (HTTP server: /inbox, /outbox, /health)
src/e2e/harness.ts         — injectMessage(), waitForResponse(), llmAssert()
scripts/e2e-seed.ts        — seeds registered_groups + chats before boot
scripts/e2e-entrypoint.sh  — seeds DB → writes AGENTS.md → starts app
Dockerfile.e2e             — e2e image (bun, curl, no Tailscale)
docker-compose.e2e.yml     — pocketbrain-e2e + e2e-runner services
docker-compose.e2e.local.yml — compose override: adds Ollama service
opencode.json              — Ollama provider registration
```

---

## 🧩 Skills System

Skills are OpenCode skill files in `.opencode/skills/*/SKILL.md`. They:

1. 📁 Sync from `container/skills/` to `.opencode/skills/` at boot
2. Are discovered automatically by OpenCode 🧠
3. Run by typing `/skill-name` in the OpenCode CLI

💡 Skills let contributors add capabilities (Telegram, Gmail, etc.) without
bloating the core codebase. Users run a skill, it modifies the code, and
they end up with exactly what they need.

---

## 💬 WhatsApp Specifics

**LID translation:** Newer WhatsApp accounts use "LID" JIDs
(e.g. `abc123@lid`) instead of phone-based JIDs. PocketBrain maintains a
`lidToPhoneMap` and resolves LIDs via the signal repository.

**📬 Outgoing queue:** If WhatsApp disconnects mid-operation, 💬 messages are
queued in memory (`outgoingQueue`) and flushed on reconnect. Messages are
only removed from the queue after confirmed send.

**🤖 Bot message detection:**
- `ASSISTANT_HAS_OWN_NUMBER=true` → `fromMe === true` (reliable ✅)
- `ASSISTANT_HAS_OWN_NUMBER=false` → content starts with `Andy:` (prefix check)

---

*Next: [🌳 Level 3 — Architect Guide](./GUIDE_ARCHITECT.md)*
