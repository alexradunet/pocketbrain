# PocketBrain: Repository Guide

PocketBrain is a minimal assistant layer on top of the OpenCode SDK.
OpenCode handles core agent/runtime behavior; this repo adds channels, memory, heartbeat cron tasks, and a few tools.

## Design Goals

- Keep custom code small ("less is more").
- Reuse OpenCode-native mechanisms (auth, sessions, tools/plugins).
- Reuse Bun native libraries and quality of life features.
- Persist all state in a single SQLite database via `bun:sqlite`.
- Share one main conversation session across channels.
- Use Context& 7 MCP to retrieve the latest documentation regarding OpenCode and Bun.

## Runtime Overview

Entry point: `src/index.ts`

Startup flow:
1. Load config from `src/config.ts`.
2. Set `OPENCODE_CONFIG_DIR` to this repo.
3. Initialize `AssistantCore`.
4. Create `ChannelManager` and register adapters.
5. Start heartbeat scheduler if `heartbeat_tasks` table has rows.

## OpenCode Integration

`AssistantCore` (`src/core/assistant.ts`) owns OpenCode client usage:
- One shared **main** OpenCode session across all channels.
- One separate **heartbeat** session.
- Each user message uses `session.prompt` with a dynamic `system` prompt.
- Falls back to message polling only if prompt output cannot be parsed.

Model selection:
- `OPENCODE_MODEL` if set.
- Else first recent model in `~/.local/state/opencode/model.json` (`XDG_STATE_HOME` respected).

## Channels

### Architecture

Channels implement the `ChannelAdapter` interface (`src/core/ports/channel-adapter.ts`):
```typescript
interface ChannelAdapter {
  readonly name: string
  start(handler: MessageHandler): Promise<void>
  stop(): Promise<void>
  send(userID: string, text: string): Promise<void>
}
```

### WhatsApp Adapter

`src/adapters/channels/whatsapp/adapter.ts`:
- Uses `@whiskeysockets/baileys`
- Commands: `/pair`, `/new`, `/remember`
- QR login + automatic reconnect
- Enforces whitelist and chunks long replies
- **Rate limiting**: 500ms between chunks, 1s per-user throttle

### Channel Manager

`src/core/channel-manager.ts`:
- Registry for channel adapters
- Lifecycle management (start/stop)
- Message routing

## Persistence: SQLite

All state lives in `.data/state.db` (WAL mode). Module: `src/store/db.ts`.

Tables:
- `kv` — key-value pairs (session IDs, last-channel)
- `whitelist` — per-channel allowed users
- `outbox` — queued proactive messages (with retry support)
- `memory` — durable user memory facts (with deduplication)
- `heartbeat_tasks` — recurring cron task descriptions

No separate JSON files, no plain-text state files.

## Memory

Stored as rows in the `memory` table. Always reconstructed as a text block and injected into the system prompt.

Features:
- **Deduplication**: Facts are normalized (lowercase, whitespace collapsed) before insert
- **Deletion**: `deleteMemory(id)` removes specific facts
- **Update**: `updateMemory(id, fact)` modifies existing facts

Tools:
- `save_memory` — append durable fact (skips duplicates)
- `delete_memory` — remove fact by ID

## Outbox (Proactive Messages)

Messages queued in `outbox` table with retry support:

Columns:
- `retry_count` — current retry attempts
- `max_retries` — maximum retries (default: 3)
- `next_retry_at` — when to retry after failure

Retry behavior:
- Exponential backoff: `60s * 2^retry_count`
- Dropped after max retries exceeded

## Heartbeat (Cron Tasks)

Table: `heartbeat_tasks` in `.data/state.db`

Features:
- Runs in its own session
- **Per-run retries with exponential backoff**
- Scheduler cadence remains fixed by `HEARTBEAT_INTERVAL_MINUTES`
- **User notification** after 3 consecutive failures
- Uses the same system prompt + memory

Retry backoff calculation:
- Base: 60 seconds
- Max: 30 minutes
- Formula: `min(base * 2^attempt, max)`

To add or remove tasks:
```sql
INSERT INTO heartbeat_tasks (task) VALUES ('your task description');
DELETE FROM heartbeat_tasks WHERE id = 1;
```

## Proactive Messaging

Tool: `send_channel_message` (plugin)

Flow:
1. Agent decides to notify.
2. Tool inserts a row into `outbox` table.
3. Channel adapters poll `outbox` and send.
4. Failed sends are retried with exponential backoff.

Destination:
- Last used channel/user, stored in `kv` table under key `last_channel`.

## Tools (Plugins)

Configured in `opencode.json`:

- `install_skill` → installs GitHub tree URL skill into `.agents/skills/`
- `save_memory` → insert fact into `memory` table (with deduplication)
- `delete_memory` → remove fact from `memory` table
- `send_channel_message` → insert row into `outbox` table

Plugins import shared functions from `src/store/db.ts` to ensure single database connection.

## Security / Pairing

Whitelist: `whitelist` table in `.data/state.db`
- `/pair <token>` if `WHITELIST_PAIR_TOKEN` is set
- Otherwise direct SQL insert by admin

## Persistent Data

All in `.data/state.db`:
- `kv` — sessions and last-channel
- `whitelist`
- `outbox` — with retry columns
- `memory` — with deduplication column
- `heartbeat_tasks`

Also:
- `.data/whatsapp-auth/` — Baileys auth state (file-based, managed by Baileys)

## Commands

User:
- `/new` — start new session
- `/remember <text>` — save to memory
- `/pair <token>` — whitelist yourself

Developer:
- `bun run dev` — development with watch
- `bun run start` — production run
- `bun run typecheck` — TypeScript check
- `bun test` — run tests

## Testing Requirements

Every adapter and core component **MUST** have comprehensive unit tests. This ensures reliability and makes refactoring safe.

### Test Structure

```
tests/
├── adapters/           # Adapter tests mirror src structure
│   └── channels/       # Channel adapter tests
├── core/               # Core module tests
├── vault/              # Vault service tests
├── scheduler/          # Scheduler tests
└── store/              # Store tests
```

### Test Standards

1. **Every public method must be tested**
   - Happy path (normal operation)
   - Error cases (exceptions, null inputs)
   - Edge cases (empty strings, boundary values)

2. **Test files mirror source structure**
    ```
    src/adapters/channels/rate-limiter.ts
    tests/adapters/channels/rate-limiter.test.ts
    ```

3. **Use bun:test native testing**
   ```typescript
   import { describe, test, expect, beforeEach } from "bun:test"
   ```

4. **Test file template:**
   ```typescript
   import { describe, test, expect, beforeEach, afterEach } from "bun:test"
   import { Component } from "../../../src/path/to/component"
   
   describe("ComponentName", () => {
     let component: Component
     
     beforeEach(() => {
       component = new Component()
     })
     
     afterEach(() => {
       // Cleanup
     })
     
     test("should do something", () => {
       // Arrange
       const input = "test"
       
       // Act
       const result = component.method(input)
       
       // Assert
       expect(result).toBe(expected)
     })
   })
   ```

5. **File system tests use temp directories**
   ```typescript
   const TEST_DIR = join(__dirname, ".test-data", "test-name")
   
   beforeEach(() => {
     mkdirSync(TEST_DIR, { recursive: true })
   })
   
   afterEach(() => {
     rmSync(TEST_DIR, { recursive: true, force: true })
   })
   ```

6. **Async operations use await**
   ```typescript
   test("async operation", async () => {
     const result = await component.asyncMethod()
     expect(result).toBeDefined()
   })
   ```

### Current Test Coverage

- ✅ `tests/adapters/channels/rate-limiter.test.ts`
- ✅ `tests/core/channel-adapter.test.ts`
- ✅ `tests/scheduler/heartbeat-backoff.test.ts`
- ✅ `tests/store/db.test.ts`
- ✅ `tests/vault/vault-service.test.ts`

## Architectural Patterns

### 1. Dependency Injection

Components receive dependencies through constructors, not global imports:

```typescript
// Good
class VaultService {
  constructor(private options: VaultOptions) {}
}

// Avoid
class VaultService {
  private db = getGlobalDb() // Don't do this
}
```

### 2. Port/Adapter Pattern

Define interfaces (ports) in core, implement in adapters:

```typescript
// Core port
export interface MemoryRepository {
  append(fact: string, source?: string): boolean
  readAll(): string
}

// Adapter implementation
export class SQLiteMemoryRepository implements MemoryRepository {
  // Implementation
}
```

### 3. Single Responsibility

Each module does one thing:

- `rate-limiter.ts` → Only handles message throttling
- `message-chunker.ts` → Only splits messages
- `message-sender.ts` → Only coordinates sending

### 4. Testability Patterns

**Pure functions where possible:**
```typescript
// Easy to test
export function escapeXml(str: string): string {
  return str.replace(/&/g, '&amp;')
}

// Harder to test (has side effects)
export function logAndEscape(str: string): string {
  console.log(str) // Side effect
  return str.replace(/&/g, '&amp;')
}
```

**Interface-based design for mocking:**
```typescript
export interface ChannelAdapter {
  readonly name: string
  start(handler: MessageHandler): Promise<void>
  stop(): Promise<void>
  send(userID: string, text: string): Promise<void>
}

// Test can pass mock implementation
class MockAdapter implements ChannelAdapter {
  readonly name = "mock"
  // Implementation for testing
}
```

### 5. Error Handling

Always handle errors explicitly:

```typescript
// Good
try {
  await operation()
} catch (error) {
  console.error('[Component] Operation failed:', error)
  return null // Or appropriate fallback
}

// Avoid
try {
  await operation()
} catch (e) {
  // Silent failure - never do this
}
```

## Tradeoffs

- Message polling is a fallback (not streaming).
- Memory has deduplication but limited fuzzy matching.
- Heartbeat tasks managed via SQL, not a text file.
- Whitelist is SQLite-based.
- Session cleanup requires SDK support (placeholder).

## Extension Points

- Add channels: implement `ChannelAdapter` interface in `src/adapters/channels/`
- Add tools: create `.agents/plugins/*.plugin.js` and register in `opencode.json`
- Add skills: place under `.agents/skills/`
- Add ports: define interfaces in `src/core/ports/`, implement in `src/adapters/`

## Utilities

- `src/adapters/channels/rate-limiter.ts` — throttling for message sends
- `src/lib/types.ts` — type guards for SDK responses
- `src/lib/logger.ts` — structured logging utilities
