# PocketBrain Specification

A personal OpenCode assistant accessible via WhatsApp (and optionally Telegram/Discord), with persistent memory per conversation, scheduled tasks, and extensible channel support.

---

## Table of Contents

1. [Architecture](#architecture)
2. [Folder Structure](#folder-structure)
3. [Configuration](#configuration)
4. [Memory System](#memory-system)
5. [Session Management](#session-management)
6. [Message Flow](#message-flow)
7. [Commands](#commands)
8. [Scheduled Tasks](#scheduled-tasks)
9. [MCP Servers](#mcp-servers)
10. [Deployment](#deployment)
11. [Security Considerations](#security-considerations)

---

## Architecture

PocketBrain runs as a **single long-lived Bun process** inside a Docker container. The OpenCode SDK starts an embedded OpenCode server (port 4096) and manages per-group agent sessions in-process. There are no per-invocation containers.

```
┌─────────────────────────────────────────────────────────────────────┐
│                    DOCKER CONTAINER (Debian)                         │
│                    (Single Bun Process)                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌──────────────┐    ┌──────────────────────────────────────────┐   │
│  │  WhatsApp    │    │          SQLite Database                  │   │
│  │  (baileys)   │◀──▶│          (store/messages.db)              │   │
│  └──────────────┘    └─────────────────┬────────────────────────┘   │
│                                        │                             │
│         ┌──────────────────────────────┘                             │
│         ▼                                                            │
│  ┌──────────────────┐    ┌──────────────────┐    ┌───────────────┐  │
│  │  Message Loop    │    │  Scheduler Loop  │    │  IPC Watcher  │  │
│  │  (polls SQLite)  │    │  (checks tasks)  │    │  (file-based) │  │
│  └────────┬─────────┘    └────────┬─────────┘    └───────┬───────┘  │
│           │                       │                       │          │
│           └───────────┬───────────┘                       │          │
│                       ▼                                   │          │
│  ┌─────────────────────────────────────────────────────┐  │          │
│  │              OpenCode Manager                        │  │          │
│  │  (src/opencode-manager.ts)                           │  │          │
│  │                                                      │  │          │
│  │  createOpencode() → OpenCode server on :4096         │  │          │
│  │  Per-group sessions via client.session.*             │  │          │
│  │  SSE event streaming for agent output                │◀─┘          │
│  └─────────────────────┬───────────────────────────────┘            │
│                        │ stdio                                        │
│                        ▼                                             │
│  ┌─────────────────────────────────────────────────────┐            │
│  │          PocketBrain MCP Server                      │            │
│  │  (dist/pocketbrain-mcp — stdio child process)        │            │
│  │                                                      │            │
│  │  Tools: send_message, schedule_task, list_tasks,     │            │
│  │         pause_task, resume_task, cancel_task,         │            │
│  │         register_group                               │            │
│  └──────────────────────────────────────────────────────┘           │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### Technology Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| WhatsApp Connection | `@whiskeysockets/baileys` | Connect to WhatsApp, send/receive messages |
| Message Storage | SQLite (`bun:sqlite`) | Store messages, sessions, tasks, groups |
| Agent Engine | `@opencode-ai/sdk` (`createOpencode`) | Embedded OpenCode server + per-group sessions |
| MCP Server | `@modelcontextprotocol/sdk` (stdio) | Custom tools for the agent (scheduler, messaging) |
| Runtime | Bun 1.x inside Docker | Host process, container isolation |

---

## Folder Structure

```
pocketbrain/
├── AGENTS.md                      # Project context for OpenCode
├── docs/
│   ├── SPEC.md                    # This specification document
│   ├── REQUIREMENTS.md            # Architecture decisions
│   └── nanorepo-architecture.md   # Skills system design (historical)
├── README.md                      # User documentation
├── package.json                   # Bun-managed dependencies
├── tsconfig.json                  # TypeScript configuration
├── docker-compose.yml             # Container orchestration
│
├── src/
│   ├── index.ts                   # Orchestrator: message loop, group queue, agent invocation
│   ├── opencode-manager.ts        # OpenCode SDK: boot, startSession, sendFollowUp, abortSession
│   ├── mcp-tools.ts               # Stdio MCP server (agent tools: send_message, schedule_task, …)
│   ├── channels/
│   │   └── whatsapp.ts            # Baileys WhatsApp connection, auth, send/receive
│   ├── ipc.ts                     # File-based IPC: reads agent-written JSON from data/ipc/
│   ├── router.ts                  # Message formatting, trigger matching, channel routing
│   ├── config.ts                  # Configuration constants (paths, timeouts, env vars)
│   ├── types.ts                   # TypeScript interfaces (Channel, RegisteredGroup, …)
│   ├── logger.ts                  # Pino logger setup
│   ├── db.ts                      # SQLite schema + query helpers
│   ├── group-queue.ts             # Per-group concurrency with MAX_CONCURRENT_SESSIONS
│   ├── task-scheduler.ts          # Runs scheduled tasks when due
│   ├── env.ts                     # .env file reader
│   └── ipc.ts                     # IPC watcher and task processing
│
├── .opencode/
│   └── skills/
│       ├── setup/SKILL.md              # /setup - First-time installation
│       ├── customize/SKILL.md          # /customize - Add capabilities
│       ├── debug/SKILL.md              # /debug - Debugging guide
│       ├── add-telegram/SKILL.md       # /add-telegram - Telegram channel
│       ├── add-discord/SKILL.md        # /add-discord - Discord channel
│       ├── add-gmail/SKILL.md          # /add-gmail - Gmail integration
│       ├── add-voice-transcription/    # /add-voice-transcription - Whisper ASR
│       ├── add-parallel/SKILL.md       # /add-parallel - Parallel agent sessions
│       ├── add-telegram-swarm/SKILL.md # /add-telegram-swarm - Multi-bot swarm
│       └── x-integration/SKILL.md      # /x-integration - X/Twitter posting
│
├── groups/
│   ├── AGENTS.md                  # Global memory (all groups read this)
│   ├── global/AGENTS.md           # Global instructions loaded at boot
│   ├── main/                      # Self-chat (main control channel)
│   │   └── AGENTS.md              # Main channel memory
│   └── {Group Name}/              # Per-group folders (created on registration)
│       ├── AGENTS.md              # Group-specific memory
│       └── *.md                   # Files created by the agent
│
├── store/                         # Local data (gitignored)
│   ├── auth/                      # WhatsApp authentication state (Baileys)
│   └── messages.db                # SQLite: messages, chats, scheduled_tasks,
│                                  #         task_run_logs, sessions, registered_groups,
│                                  #         router_state
│
├── data/                          # Application state (gitignored)
│   └── ipc/                       # File-based IPC directories
│       └── {groupFolder}/
│           ├── messages/          # Agent → host: outbound messages
│           ├── tasks/             # Agent → host: task operations
│           ├── current_tasks.json # Snapshot: scheduled tasks visible to this group
│           └── available_groups.json # Snapshot: registered groups (main only)
│
└── logs/                          # Runtime logs (gitignored)
    ├── pocketbrain.log
    └── pocketbrain.error.log
```

---

## Configuration

Configuration constants are in `src/config.ts`. Key environment variables:

```bash
# Authentication (set at least one)
OPENCODE_API_KEY=sk-ant-api03-...       # Anthropic API key
OPENCODE_MODEL=claude-sonnet-4-5        # Override default model

# Identity
ASSISTANT_NAME=Andy                     # Trigger word (@Andy in messages)

# Tuning
MAX_CONCURRENT_SESSIONS=3              # Max parallel group sessions
POLL_INTERVAL=2000                     # Message poll interval (ms)
SCHEDULER_POLL_INTERVAL=60000          # Task scheduler poll interval (ms)
IDLE_TIMEOUT=1800000                   # Session idle timeout (ms, 30min)
```

All environment variables are read from `.env` in the project root. The Docker container inherits them via `env_file` in `docker-compose.yml`.

### OpenCode Authentication

```bash
# Option 1: API key (pay-per-use)
OPENCODE_API_KEY=sk-ant-api03-...

# Option 2: Custom base URL (e.g. for OpenRouter)
OPENCODE_BASE_URL=https://openrouter.ai/api/v1
OPENCODE_API_KEY=sk-or-...
OPENCODE_MODEL=anthropic/claude-sonnet-4-5
```

---

## Memory System

PocketBrain uses a hierarchical memory system based on AGENTS.md files.

### Memory Hierarchy

| Level | Location | Read By | Written By | Purpose |
|-------|----------|---------|------------|---------|
| **Global** | `groups/global/AGENTS.md` | All groups (via OpenCode instructions) | Main only | Preferences, facts shared across conversations |
| **Group** | `groups/{name}/AGENTS.md` | That group (injected at session start) | That group | Group-specific context, conversation memory |
| **Files** | `groups/{name}/*.md` | That group | That group | Notes, research, documents |

### How Memory Works

1. **Global context**: `groups/global/AGENTS.md` is passed to `createOpencode()` as an `instructions` path — OpenCode loads it automatically for every session.

2. **Group context**: At the start of each new session, `buildGroupContext()` reads `groups/{name}/AGENTS.md` and injects it into the first prompt, along with the session's `pocketbrain_context` (chatJid, groupFolder, isMain).

3. **Writing memory**: Agent writes to `./AGENTS.md` for group memory, or uses the global file for cross-session facts.

4. **Main channel privileges**: Only the "main" group can write to global memory and schedule tasks for other groups.

---

## Session Management

Sessions enable conversation continuity — OpenCode remembers the full conversation history.

### How Sessions Work

1. Each group has a session ID stored in SQLite (`sessions` table, keyed by `group_folder`)
2. On new message: `startSession()` creates a new session or resumes the existing one via `client.session.get()`
3. Follow-up messages from the same group (while session is active) are sent via `sendFollowUp()`
4. Sessions remain open (blocking `startSession()`) until `abortSession()` is called or the host shuts down
5. Session transcripts are stored by OpenCode in its working directory

### Group Context

Every new session starts with a `<pocketbrain_context>` block containing:
- `chatJid` — the WhatsApp/Telegram/Discord JID for this chat
- `groupFolder` — the folder name for IPC and file operations
- `isMain` — whether this is the privileged main channel

This context is also re-injected on follow-up prompts to survive session compaction.

---

## Message Flow

### Incoming Message Flow

```
1. User sends WhatsApp message
   │
   ▼
2. Baileys receives message via WhatsApp Web protocol
   │
   ▼
3. Message stored in SQLite (store/messages.db)
   │
   ▼
4. Message loop polls SQLite (every 2 seconds)
   │
   ▼
5. Router checks:
   ├── Is chat_jid in registered_groups? → No: ignore
   └── Does message match trigger pattern? → No: store but don't process
   │
   ▼
6. GroupQueue serializes processing per group:
   ├── If session active: sendFollowUp() with new messages
   └── If no session: startSession() with full catch-up history
   │
   ▼
7. OpenCode processes message:
   ├── Reads AGENTS.md files for context
   ├── Uses tools (search, file ops, MCP tools)
   └── Streams output via SSE events
   │
   ▼
8. Output sent via WhatsApp, session ID saved to SQLite
```

### Trigger Word Matching

Messages must start with the trigger pattern (default: `@Andy`):
- `@Andy what's the weather?` → ✅ Triggers OpenCode
- `@andy help me` → ✅ Triggers (case insensitive)
- `Hey @Andy` → ❌ Ignored (trigger not at start)
- `What's up?` → ❌ Ignored (no trigger)

### Conversation Catch-Up

When a triggered message arrives, the agent receives all messages since its last interaction in that chat:

```
[Jan 31 2:32 PM] John: hey everyone, should we do pizza tonight?
[Jan 31 2:33 PM] Sarah: sounds good to me
[Jan 31 2:35 PM] John: @Andy what toppings do you recommend?
```

---

## Commands

### Available in Any Group

| Command | Effect |
|---------|--------|
| `@Andy [message]` | Talk to the agent |
| `@Andy list my scheduled tasks` | View this group's tasks |
| `@Andy pause/resume/cancel task [id]` | Manage tasks |

### Available in Main Channel Only

| Command | Effect |
|---------|--------|
| `@Andy add group "Name"` | Register a new group |
| `@Andy remove group "Name"` | Unregister a group |
| `@Andy list groups` | Show registered groups |
| `@Andy list all tasks` | View tasks from all groups |
| `@Andy schedule task for "Group": [prompt]` | Schedule for another group |

---

## Scheduled Tasks

The scheduler runs tasks as full agent sessions in their group's context.

### Schedule Types

| Type | Value Format | Example |
|------|--------------|---------|
| `cron` | Cron expression | `0 9 * * 1` (Mondays at 9am) |
| `interval` | Milliseconds | `3600000` (every hour) |
| `once` | ISO timestamp | `2024-12-25T09:00:00Z` |

### Creating a Task

```
User: @Andy remind me every Monday at 9am to review the weekly metrics

Agent: [calls mcp__pocketbrain__schedule_task]
       { "prompt": "Send a reminder to review weekly metrics.",
         "schedule_type": "cron", "schedule_value": "0 9 * * 1" }

Agent: Done! I'll remind you every Monday at 9am.
```

---

## MCP Servers

### PocketBrain MCP (built-in)

The `pocketbrain` MCP server runs as a stdio child process of the OpenCode server, started once at boot.

**Available Tools:**

| Tool | Purpose |
|------|---------|
| `send_message` | Send a message to the group (via IPC → WhatsApp) |
| `schedule_task` | Schedule a recurring or one-time task |
| `list_tasks` | Show tasks (group's tasks, or all if main) |
| `pause_task` | Pause a task |
| `resume_task` | Resume a paused task |
| `cancel_task` | Delete a task |
| `register_group` | Register a new chat as a group |

### IPC Flow

The MCP tools write JSON files to `data/ipc/{groupFolder}/` using atomic temp-file-then-rename writes. The IPC watcher (`src/ipc.ts`) polls these directories and processes the commands on the host side.

---

## Deployment

PocketBrain runs via Docker Compose.

### Startup Sequence

When PocketBrain starts:
1. Reads secrets from `.env`
2. Calls `createOpencode()` to start the embedded OpenCode server on port 4096
3. Syncs skills from `container/skills/` to `.opencode/skills/` (if present)
4. Initializes SQLite and migrates schema
5. Connects to WhatsApp via Baileys
6. On `connection.open`: starts message loop, scheduler, and IPC watcher

### Docker Compose Commands

```bash
bun run docker:build   # Build the Docker image
bun run docker:up      # Start container detached
bun run docker:down    # Stop container
bun run docker:logs    # Tail container logs
bun run docker:test    # Run tests in container
```

### Development

```bash
bun run dev            # Run dev container with interactive terminal
```

---

## Security Considerations

### Isolation Model

PocketBrain runs inside a Docker container (Linux). The Docker container provides:
- **Filesystem isolation**: Host filesystem is not accessible unless explicitly mounted
- **Process isolation**: Container processes can't affect the host
- **Network isolation**: Configurable via Docker networks

The agent (OpenCode) runs **inside the same container** as the host process. It has access to the container's filesystem. This is safe for personal use but means the agent can read/write any file within the container.

### Prompt Injection Risk

WhatsApp messages could contain malicious instructions attempting to manipulate the agent's behavior.

**Mitigations:**
- Only registered groups are processed
- Trigger word required (reduces accidental processing)
- OpenCode's built-in safety training
- Docker container limits filesystem and network access

**Recommendations:**
- Only register trusted groups/chats
- Review scheduled tasks periodically
- Monitor logs for unusual activity

### Credential Storage

| Credential | Storage Location |
|------------|------------------|
| API keys | `.env` file (gitignored) |
| WhatsApp session | `store/auth/` (gitignored) |
| Session transcripts | OpenCode's working directory |

```bash
# Protect sensitive directories
chmod 700 store/ groups/ data/
```

---

## Troubleshooting

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| No response to messages | Container not running | `bun run docker:logs` to check; `bun run docker:up` to start |
| Agent not responding | Session error | Check logs for OpenCode errors |
| "QR code expired" | WhatsApp session expired | Delete `store/auth/` and restart |
| "No groups registered" | Haven't added groups | Use `@Andy add group "Name"` in main |
| Context lost after long session | Session compaction | Context is re-injected on each follow-up |

### Log Location

```bash
bun run docker:logs          # Live container logs
bun run docker:logs | grep ERROR   # Filter errors
```

### Debug Mode

```bash
bun run dev    # Interactive dev container with verbose output
```
