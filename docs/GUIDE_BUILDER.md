# 🏗️ Build Your Own Personal AI Agent
### *Using PocketBrain as a Reference Implementation*

> **Who this is for:** A developer who wants to build their own personal AI
> agent and use PocketBrain as a learning reference. Each section teaches
> a reusable pattern, then shows exactly how PocketBrain implements it.
>
> You don't need to fork PocketBrain — you can lift any of these patterns
> into your own project.

---

## 🗺️ The Blueprint

A personal AI agent has **six core concerns**:

```
  1. 📥 Input         — How does the agent receive requests?
  2. 🧠 Intelligence  — What model/harness runs the AI?
  3. 🔌 Tools         — What can the agent actually do?
  4. 🔄 Memory        — How does it remember across conversations?
  5. ⏰ Autonomy      — Can it act without being asked?
  6. 🛡️ Boundaries    — What is it NOT allowed to do?
```

PocketBrain solves all six. This guide walks through each one,
teaching the pattern and showing the implementation.

---

## 📥 Pattern 1: Input Channel Abstraction

### 🎓 The Concept

Your agent needs to receive messages from somewhere — WhatsApp, Telegram,
Slack, a CLI, a REST endpoint. The key insight: **decouple the messaging
protocol from the agent logic**.

Define a minimal `Channel` interface. Your agent only ever talks to this
interface, never to WhatsApp directly. Swap the channel without touching
the agent.

### 📐 The Pattern

```typescript
interface Channel {
  name: string;
  connect(): Promise<void>;
  sendMessage(jid: string, text: string): Promise<void>;
  isConnected(): boolean;
  ownsJid(jid: string): boolean;   // "does this channel own this address?"
  disconnect(): Promise<void>;
  setTyping?(jid: string, isTyping: boolean): Promise<void>; // optional
}
```

**`ownsJid()`** is the routing key. Each channel declares which addresses
it handles:
- WhatsApp: `jid.endsWith('@g.us') || jid.endsWith('@s.whatsapp.net')`
- Telegram: `jid.startsWith('tg:')`
- CLI: `jid === 'cli'`

The router then finds the right channel by calling `ownsJid()` on each.

### 🔍 PocketBrain Implementation

```
src/types.ts       — Channel interface definition
src/channels/whatsapp.ts  — WhatsApp implementation (Baileys)
src/router.ts      — findChannel() picks the right channel by ownsJid()
src/index.ts       — channels[] array; add new channels here
```

### 🧪 Testing Corollary: MockChannel

The same `Channel` interface makes testing trivial. PocketBrain ships a
`MockChannel` (`src/channels/mock.ts`) that replaces WhatsApp entirely
during e2e tests. It exposes three HTTP endpoints:

```
POST /inbox   — inject a test message (replaces "user sends WhatsApp message")
GET  /outbox  — read captured responses (replaces "check phone for reply")
DELETE /outbox — clear between tests
```

Set `CHANNEL=mock` in the environment and the real WhatsApp connection is
never opened. No phone, no QR code, no Baileys session — just HTTP in and
HTTP out.

```typescript
// e2e test — injects a message, waits for the agent's response
await injectMessage('What is 7 × 6?', { jid: 'test@mock.test' });
const msgs = await waitForResponse(60_000);
const text = msgs.map(m => m.text).join('');
// assert on text...
```

This is possible *only* because the agent logic never imports WhatsApp directly —
it only calls `Channel` methods. Zero test-specific branches inside the agent.

### ✅ Lesson

Don't hardcode WhatsApp (or any channel) into your agent logic.
Write to the `Channel` interface from day one. Adding Telegram later
is then a new file, not a refactor. Testing later is also a new file.

---

## 🗄️ Pattern 2: Message Persistence Before Processing

### 🎓 The Concept

A common mistake: process the message immediately when it arrives.
The problem: if processing fails (agent error, network timeout, restart),
the message is gone.

The correct pattern: **store first, process second**.

```
Message arrives → Store in DB → Process from DB
```

This decouples receipt from processing. Failures are retried. Crashes
don't lose messages. You can replay.

### 📐 The Pattern

```typescript
// Step 1: store unconditionally
onMessage(chatJid, message) {
  db.storeMessage(message);  // always succeeds
}

// Step 2: poll and process
while (true) {
  const pending = db.getMessagesSince(lastTimestamp);
  for (const msg of pending) {
    await processMessage(msg);   // may fail → retry
  }
  await sleep(POLL_INTERVAL);
}
```

### 🔍 PocketBrain Implementation

```
src/db.ts:storeMessage()        — stores to SQLite on receipt
src/db.ts:getNewMessages()      — polls for unprocessed messages
src/index.ts:startMessageLoop() — the 2-second poll loop
src/config.ts:POLL_INTERVAL     — 2000ms poll interval
```

The dual-timestamp system (`lastTimestamp` vs `lastAgentTimestamp`) is
the key refinement: advance the "seen" cursor immediately, but only advance
the "processed" cursor after the agent succeeds. Rollback on failure.

### ✅ Lesson

Never process a message from the delivery callback. Store it, then process
it in a separate loop. This makes your agent restartable and retryable.

---

## 🧠 Pattern 3: AI Harness Selection

### 🎓 The Concept

The **AI harness** is the layer between your code and the AI model. It
handles tool calling, context management, session state, streaming, and
model-specific quirks. Choosing (or building) the right harness is one of
the most important decisions.

**Don't call the LLM API directly** unless you have a very simple use case.
Harnesses save enormous amounts of code.

### 📐 The Pattern

A good harness gives you:

| Capability | Without harness | With harness |
|------------|----------------|--------------|
| Tool calling | Parse JSON, retry, validate | `tools: [...]` in config |
| Context management | Count tokens, truncate | Automatic compaction |
| Streaming | SSE parsing, reconnect | `for await (event of stream)` |
| Session persistence | Store/restore manually | `session.create()` / `session.get()` |
| Multi-model | Per-model API differences | Config swap |

### 🔍 PocketBrain Implementation

PocketBrain uses **OpenCode SDK** as its harness:

```typescript
// Boot the harness (src/opencode-manager.ts:boot())
const instance = await createOpencode({
  hostname: '127.0.0.1',
  port: 4096,
  config: {
    permission: { bash: 'allow', edit: 'allow' },
    tools: { bash, edit, write, read, websearch, webfetch },
    mcp: { pocketbrain: { type: 'local', command: [mcpServerPath] } },
  },
});

// Create a session (src/opencode-manager.ts:startSession())
const session = await client.session.create({ body: { title: 'My Session' } });

// Send a prompt (src/opencode-manager.ts:runPrompt())
await client.session.promptAsync({
  path: { id: session.id },
  body: { messageID: uuid(), parts: [{ type: 'text', text: userMessage }] },
});
```

### 🔄 Alternatives to OpenCode SDK

| Harness | Best for |
|---------|---------|
| **OpenCode SDK** | Full agentic loops with tools, sessions, web access |
| **Vercel AI SDK** | Web apps, streaming chat UIs |
| **LangChain.js** | Complex agent chains, many integrations |
| **Anthropic SDK direct** | Simple one-shot calls, custom tool handling |
| **Raw fetch** | Learning; production if harness is overkill |

### ✅ Lesson

Pick a harness before writing any agent logic. The harness shapes everything:
how you pass tools, how you handle context limits, how you stream responses.
OpenCode SDK is a strong choice for personal agents that need tool use and
persistent sessions.

---

## 🔌 Pattern 4: Extending the Agent with MCP Tools

### 🎓 The Concept

**MCP (Model Context Protocol)** is an open standard for giving AI agents
access to external tools and data. Instead of hardcoding tool implementations
into your agent, you run an **MCP server** — a separate process that exposes
tools the agent can call.

Benefits:
- 🔒 **Isolation**: tool server runs separately, with its own permissions
- 🧩 **Composability**: swap or add MCP servers without changing agent code
- 🌐 **Ecosystem**: use community MCP servers (filesystem, GitHub, Postgres…)

### 📐 The Pattern

```typescript
// mcp-server.ts — your tool server
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { z } from 'zod';

const server = new McpServer({ name: 'my-tools', version: '1.0.0' });

server.tool(
  'send_notification',              // tool name
  'Send a push notification',       // description the AI reads
  {
    title: z.string(),
    body: z.string(),
  },
  async (args) => {
    await pushService.send(args.title, args.body);
    return { content: [{ type: 'text', text: 'Notification sent.' }] };
  },
);

// Connect via stdio (parent process starts this as a child)
const transport = new StdioServerTransport();
await server.connect(transport);
```

Then register the MCP server in your harness config:

```typescript
mcp: {
  'my-tools': {
    type: 'local',
    command: ['bun', 'run', 'mcp-server.ts'],
  },
}
```

The AI can now call `send_notification` as naturally as any other tool.

### 🔍 PocketBrain Implementation

```
src/mcp-tools.ts — the MCP server (runs as stdio child process)
```

PocketBrain's MCP tools:
- `send_message` → writes a 📁 JSON file → IPC watcher sends via WhatsApp
- `schedule_task` → writes a 📁 JSON file → IPC watcher creates DB task
- `list_tasks` → reads a snapshot file the host pre-writes
- `register_group` → writes a 📁 JSON file → IPC watcher registers

Notice: the MCP tools don't act directly — they **write files**. The host
process acts. This is a deliberate security pattern (see Pattern 6 below).

### ✅ Lesson

Build your custom capabilities as MCP tools. Keep the MCP server small and
focused. Use the IPC file pattern (Pattern 5) when the tool needs to trigger
host-side operations.

---

## 📁 Pattern 5: File-Based IPC for Agent → Host Communication

### 🎓 The Concept

Your agent (running inside the AI harness) needs to trigger effects on the
**host** (your Bun/Node process): send a message, create a database record,
call an API. How?

**File-based IPC** is surprisingly effective:
1. Agent writes a JSON file (via MCP tool)
2. Host polls a directory and processes new files
3. File is deleted after processing

Why files over sockets or HTTP?
- **Atomic**: `rename()` is atomic on POSIX — no partial reads
- **Identity from path**: the directory tells you *who* wrote the file
- **Durable**: files survive process restarts
- **Simple**: no connection management, no auth tokens

### 📐 The Pattern

```typescript
// MCP side — write atomically
function writeIpcFile(dir: string, data: object): void {
  fs.mkdirSync(dir, { recursive: true });
  const filepath = path.join(dir, `${Date.now()}-${crypto.randomUUID()}.json`);
  const tempPath = `${filepath}.tmp`;
  fs.writeFileSync(tempPath, JSON.stringify(data));
  fs.renameSync(tempPath, filepath);  // ⚡ atomic
}

// Host side — poll and process
async function processIpcFiles() {
  const files = fs.readdirSync(ipcDir).filter(f => f.endsWith('.json'));
  for (const file of files) {
    const data = JSON.parse(fs.readFileSync(path.join(ipcDir, file), 'utf-8'));
    await handleIpcAction(data);
    fs.unlinkSync(path.join(ipcDir, file));  // delete after processing
  }
  setTimeout(processIpcFiles, IPC_POLL_INTERVAL);
}
```

### 🔍 PocketBrain Implementation

```
src/mcp-tools.ts:writeIpcFile()  — atomic file write
src/ipc.ts:processIpcFiles()     — polls every 1 second
src/ipc.ts:processTaskIpc()      — handles each action type
```

The directory structure enforces identity:
```
data/ipc/
  main/          ← 👑 only main group can write here
    messages/
    tasks/
  family-chat/   ← 👥 this group can only write to its own folder
    messages/
    tasks/
```

### ✅ Lesson

For personal-scale agents, file-based IPC is simpler and more reliable than
socket or HTTP IPC. Use subdirectories to enforce identity — the directory
path is OS-enforced, unlike a field in the JSON payload.

---

## 🔄 Pattern 6: Persistent Sessions with Context Re-injection

### 🎓 The Concept

Personal agents need **memory**. A conversation shouldn't start from scratch
every time. The agent should remember what you said yesterday, what tasks are
scheduled, what your preferences are.

OpenCode SDK (and most harnesses) handle session persistence. But there's
a subtle problem: **context compaction**. When the conversation grows too long,
the harness summarizes old content to free context. Critical system information
(which group is this? what are my permissions?) may be lost in the summary.

The solution: **re-inject critical context on every prompt**.

### 📐 The Pattern

```typescript
// Build a small context block — re-sent on EVERY prompt
function buildContextPrefix(sessionInfo: SessionInfo): string {
  return `<system_context>
user_id: ${sessionInfo.userId}
group: ${sessionInfo.groupName}
timezone: ${sessionInfo.timezone}
permissions: ${sessionInfo.permissions.join(', ')}
</system_context>`;
}

// Prepend it to every prompt, including follow-ups
async function sendPrompt(sessionId: string, userText: string) {
  const context = buildContextPrefix(currentSession);
  const fullPrompt = `${context}\n\n${userText}`;
  await harness.prompt(sessionId, fullPrompt);
}
```

This costs ~200 extra tokens per prompt. Worth it for correctness.

### 🔍 PocketBrain Implementation

```
src/opencode-manager.ts:buildContextPrefix() — builds the XML block
src/opencode-manager.ts:sendFollowUp()       — re-injects on every follow-up
```

```typescript
// opencode-manager.ts
function buildContextPrefix(group, input): string {
  return `<pocketbrain_context>
chatJid: ${input.chatJid}
groupFolder: ${input.groupFolder}
isMain: ${input.isMain}
</pocketbrain_context>`;
}
```

### 📝 Per-Group Memory with AGENTS.md

Beyond context injection, PocketBrain uses `AGENTS.md` files for durable
long-term memory. The agent reads and writes these Markdown files.

```
groups/
  global/AGENTS.md    ← shared memory for all groups
  family/AGENTS.md    ← memory just for the family group
  work/AGENTS.md      ← memory just for work
```

The agent can write to these files directly (using the `edit` tool). This
is "write your own memory" — no vector database needed.

### ✅ Lesson

Two kinds of memory:
- **Short-term** (session context): re-inject critical fields on every prompt
- **Long-term** (AGENTS.md): let the agent read/write Markdown files

Start with Markdown files. Add vector search only if you actually need
semantic retrieval over large corpora.

---

## ⏰ Pattern 7: Autonomous Scheduled Tasks

### 🎓 The Concept

The most powerful feature of a personal agent isn't answering questions —
it's **acting proactively**. "Every Monday, compile my weekly report."
"Alert me if the server goes down." "Send a morning briefing at 8am."

For this, you need a **task scheduler** that runs agent prompts on a schedule.

### 📐 The Pattern

```typescript
interface ScheduledTask {
  id: string;
  prompt: string;           // what to tell the agent
  schedule_type: 'cron' | 'interval' | 'once';
  schedule_value: string;   // "0 9 * * *" | "3600000" | "2026-01-01T09:00:00"
  context_mode: 'group' | 'isolated';  // use conversation history?
  next_run: string | null;
  status: 'active' | 'paused' | 'completed';
}

// Scheduler loop
async function schedulerLoop() {
  const dueTasks = db.getDueTasks();
  for (const task of dueTasks) {
    // Run agent with the task prompt
    await runAgent(task.prompt, task.context_mode);
    // Calculate next run
    const nextRun = computeNextRun(task);
    db.updateTask(task.id, { next_run: nextRun });
  }
  setTimeout(schedulerLoop, 60_000);  // check every minute
}
```

**`context_mode`** is a key design choice:
- `'group'`: task runs with full conversation history — "follow up on what we discussed"
- `'isolated'`: task runs in a fresh session — "check the weather (no context needed)"

### 🔍 PocketBrain Implementation

```
src/task-scheduler.ts:startSchedulerLoop()  — 60s poll loop
src/db.ts:getDueTasks()                     — SQL query for due tasks
src/mcp-tools.ts:'schedule_task'            — how the agent creates tasks
src/ipc.ts:processTaskIpc():'schedule_task' — how the IPC watcher saves them
```

Cron expressions use `cron-parser` for timezone-aware next-run calculation.
Interval tasks are anchored to the previous `next_run` (not wall clock) to
prevent drift over time.

### ✅ Lesson

Give your agent the ability to schedule its own tasks. This is what
transforms it from a chatbot into an autonomous agent. The agent creates
tasks by calling the `schedule_task` MCP tool — no special code needed,
just natural language from the user.

---

## 🔀 Pattern 8: Concurrency Control

### 🎓 The Concept

If multiple groups message simultaneously, you need to run multiple agent
sessions concurrently. But you can't run unlimited sessions — memory, API
rate limits, and cost all bound you. You need a **bounded concurrency queue**.

Two rules:
1. **One session per group at a time** (messages are ordered within a group)
2. **N sessions globally** (tunable limit)

### 📐 The Pattern

```typescript
class AgentQueue {
  private activeCount = 0;
  private readonly maxConcurrent: number;
  private groupStates = new Map<string, { active: boolean; pending: boolean }>();
  private waitingGroups: string[] = [];

  async enqueue(groupId: string, task: () => Promise<void>) {
    const state = this.groupStates.get(groupId) ?? { active: false, pending: false };

    if (state.active) {
      state.pending = true;   // will run after current finishes
      return;
    }

    if (this.activeCount >= this.maxConcurrent) {
      state.pending = true;
      this.waitingGroups.push(groupId);  // will run when slot frees
      return;
    }

    await this.run(groupId, task);
  }

  private async run(groupId: string, task: () => Promise<void>) {
    this.activeCount++;
    const state = this.groupStates.get(groupId)!;
    state.active = true;
    try {
      await task();
    } finally {
      state.active = false;
      this.activeCount--;
      this.drain(groupId);  // process pending tasks/messages
    }
  }
}
```

### 🔍 PocketBrain Implementation

```
src/group-queue.ts — full implementation with retry backoff
```

PocketBrain adds exponential backoff: failures retry at 5s, 10s, 20s, 40s,
80s before giving up. This handles transient API errors gracefully without
hammering the endpoint.

### ✅ Lesson

Never run agent sessions without a concurrency limit. Start with
`MAX_CONCURRENT_SESSIONS = 3` and tune up. The queue also gives you a
natural place to add retry logic, priority queuing, and rate limiting.

---

## 🛡️ Pattern 9: Security Boundaries for Personal Agents

### 🎓 The Concept

Giving an AI agent tool access is powerful and dangerous. You need to think
carefully about what the agent can and cannot do.

**Two schools of thought:**

| Approach | How | Problem |
|----------|-----|---------|
| Application-level permissions | Allowlists, checks in code | Fragile; prompt injection may bypass |
| OS-level isolation | Container/VM | Robust; no application code to bypass |

PocketBrain chooses OS-level isolation: the 🐳 Docker container IS the
security boundary. The agent has full power inside the container, but
the container is sandboxed from the host.

### 📐 The Pattern: Tiered Trust

```
Tier 0 — Untrusted:  external input (WhatsApp messages, web content)
Tier 1 — Trusted:    your host process (IPC watcher, scheduler)
Tier 2 — Sandboxed:  agent execution (inside container)
```

Rules:
- Tier 0 → Tier 1: sanitize/escape all input
- Tier 1 → Tier 2: explicit mounts only; no host credentials
- Tier 2 → Tier 1: only via IPC (file-based), authorization checked on host

### 🔍 PocketBrain Implementation

```
docs/SECURITY.md       — full trust model documentation
src/ipc.ts             — 🛡️ authorization enforcement (Tier 2 → Tier 1)
src/mcp-tools.ts       — path traversal defense (safeFolder())
src/channels/whatsapp.ts — bot message filtering
```

The IPC authorization table (from the Intermediate guide):
- Main group 👑 can do everything
- Non-main groups 👥 can only affect themselves
- Identity is from **directory path**, not from JSON payload

### ✅ Lesson

For a personal agent:
1. Run the agent in a container (Docker/Podman)
2. Only mount directories you're OK with the agent reading/writing
3. Never mount SSH keys, cloud credentials, `.env` with secrets
4. Enforce authorization on the host, not inside the agent

---

## 🧩 Pattern 10: Skills as Code Transformations

### 🎓 The Concept

Personal agents evolve. You start with WhatsApp and later want Telegram.
You start with no database and later want Postgres. How do you add
capabilities without creating a monolith?

**Skills** are a pattern for capability addition: instead of a plugin
system (runtime hot-loading), skills are **AI-assisted code modifications**.
A skill file (`SKILL.md`) contains instructions for the AI to modify the
codebase. Run the skill, get clean code.

Benefits over a plugin system:
- No runtime plugin loading machinery
- No version compatibility matrix
- End result is clean, readable code
- Users understand exactly what's installed

### 📐 The Pattern

```markdown
# SKILL.md — Add Telegram Channel

## What This Skill Does
Adds Telegram as a messaging channel alongside WhatsApp.

## Steps
1. Install the Telegram Bot API library: `bun add node-telegram-bot-api`
2. Create `src/channels/telegram.ts` implementing the Channel interface
3. Register the Telegram channel in `src/index.ts`
4. Add `TELEGRAM_BOT_TOKEN` to `.env.example`
5. Run tests: `bun run docker:test`
```

When a user runs `/add-telegram`, the AI agent reads this skill and
executes the steps against the codebase.

### 🔍 PocketBrain Implementation

```
container/skills/          — skill files ship with PocketBrain
.opencode/skills/          — synced from container/skills/ at boot
src/opencode-manager.ts:boot() — line 79-95, the sync logic
```

### ✅ Lesson

For personal tools, prefer skills over plugins. Your agent already has
code-editing capabilities — use them to extend itself. The result is always
clean, understood code rather than a black-box plugin runtime.

---

## 🚀 Putting It All Together: Your Agent in 60 Minutes

Here's the minimal path to your own personal AI agent, using these patterns:

### Step 1 — Choose your input channel (📥 Pattern 1)
Pick one: WhatsApp (Baileys), Telegram (telegraf), CLI, HTTP webhook.
Implement the `Channel` interface.

### Step 2 — Set up message persistence (🗄️ Pattern 2)
Add SQLite (Bun has it built-in) with a `messages` table.
Store every message on receipt; poll for unprocessed messages.

### Step 3 — Pick your AI harness (🧠 Pattern 3)
For a personal agent with tools and sessions: OpenCode SDK.
For a simpler chatbot: Vercel AI SDK.

### Step 4 — Build your MCP tools (🔌 Pattern 4)
What should your agent do beyond talking? Write files? Call APIs?
Send notifications? Each capability = one MCP tool.

### Step 5 — Add IPC for agent→host actions (📁 Pattern 5)
For each MCP tool that needs host-side effects, use the file-based IPC
pattern. Keep the MCP server thin; put logic in the host.

### Step 6 — Implement session persistence (🔄 Pattern 6)
Store session IDs in SQLite. Re-inject critical context on every prompt.
Write long-term memory as Markdown files.

### Step 7 — Add the scheduler (⏰ Pattern 7)
Give your agent `schedule_task` as an MCP tool. Add a 60-second poll loop
that calls the agent when tasks come due.

### Step 8 — Add concurrency control (🔀 Pattern 8)
Wrap agent invocations in a bounded queue. Start with `maxConcurrent = 3`.

### Step 9 — Set your security boundaries (🛡️ Pattern 9)
Run in a container. Only mount what you need. Enforce auth on the host.

### Step 10 — Define extension skills (🧩 Pattern 10)
Write `SKILL.md` files for things you might want later. Let the agent
add capabilities when you need them.

---

## 📚 Further Reading

| Resource | What You'll Learn |
|----------|-------------------|
| [OpenCode SDK docs](https://opencode.ai) | 🧠 Session management, tool use, streaming |
| [MCP specification](https://modelcontextprotocol.io) | 🔌 Building MCP servers and clients |
| [Baileys docs](https://github.com/WhiskeySockets/Baileys) | 💬 WhatsApp Web protocol |
| [Bun SQLite docs](https://bun.sh/docs/api/sqlite) | 🗄️ Fast embedded database |
| [cron-parser](https://github.com/harrisiirak/cron-parser) | ⏰ Cron expression parsing |
| `docs/SECURITY.md` (this repo) | 🛡️ Full security model |
| `docs/GUIDE_ARCHITECT.md` (this repo) | 🌳 Deep design decision rationale |

---

*Back to [🗺️ Guide Index](./GUIDE.md)*
