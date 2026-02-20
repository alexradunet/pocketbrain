# PocketBrain: Repository Guide

PocketBrain is a self-contained Go assistant runtime. It talks directly to AI provider APIs (Anthropic, OpenAI-compatible, Google) with tool calling, SQLite persistence, and WhatsApp as a messaging channel.

## Design Goals

- Single static binary, zero runtime dependencies.
- Direct AI provider integration (no SDK intermediaries).
- All state in a single SQLite database.
- One shared conversation session across channels.
- Hexagonal architecture with compiler-enforced boundaries.

## Runtime Overview

Entry point: `main.go` -> `cmd/` (cobra CLI) -> `internal/app/app.go` (composition root).

Startup flow:
1. Load config from environment (`internal/config/config.go`).
2. Open SQLite database and create repositories.
3. Initialize workspace, tool registry, and AI provider.
4. Create `AssistantCore` with all dependencies injected.
5. Start heartbeat scheduler.
6. Wire and start WhatsApp adapter (if enabled).
7. Launch TUI or run headless.

## AI Integration

`AssistantCore` (`internal/core/assistant.go`) orchestrates all AI interactions:
- One shared **main** session across all channels.
- One separate **heartbeat** session for background tasks.
- Dynamic system prompt with injected memory facts.

Providers:
- `AnthropicProvider` (`internal/ai/anthropic.go`) — native Anthropic Messages API with tool calling.
- `FantasyProvider` (`internal/ai/fantasy.go`) — OpenAI-compatible Chat Completions API (works with OpenAI, Google, local models).
- `StubProvider` — returns canned replies when no API key is configured.

Tool registry (`internal/ai/tools.go`) with explicit tool loop (`internal/ai/toolloop.go`):
- `workspace_*` (7 tools) — file CRUD, search, stats.
- `save_memory`, `delete_memory` — durable fact storage.
- `send_channel_message` — proactive outbox messaging.
- `skill_*` (4 tools) — list, load, create, install from GitHub.

## Channels

### Architecture

Channels implement `core.ChannelAdapter` (`internal/core/ports.go`):
```go
type ChannelAdapter interface {
    Name() string
    Start(handler MessageHandler) error
    Stop() error
    Send(userID, text string) error
}
```

### WhatsApp Adapter

`internal/channel/whatsapp/`:
- Uses `go.mau.fi/whatsmeow` (mature Go WhatsApp library).
- Commands: `/pair`, `/new`, `/remember`.
- QR login displayed in TUI and logged to stdout.
- Brute-force protection on `/pair` attempts.
- Message chunking and per-user rate limiting via `internal/channel/message.go`.

## Persistence: SQLite

All state in `.data/state.db` (WAL mode). Schema in `internal/store/db.go`.

Tables:
- `session` — key-value session IDs.
- `whitelist` — per-channel allowed users.
- `outbox` — queued proactive messages with retry metadata.
- `memory` — durable user facts with deduplication.
- `heartbeat_tasks` — recurring task descriptions.
- `channel` — last-used channel tracking.

## Memory

Stored in `memory` table. Reconstructed as text block and injected into system prompt.

Features:
- Deduplication via normalized fact comparison.
- CRUD: append, delete, update, get all.

## Heartbeat (Cron Tasks)

Scheduler: `internal/scheduler/heartbeat.go`.

Features:
- Runs tasks in dedicated heartbeat session.
- Injects summary into main session.
- Exponential backoff on failures (base 60s, max 30min).
- Notification after consecutive failures.

## Commands

User (via WhatsApp):
- `/new` — start new conversation session.
- `/remember <text>` — save fact to memory.
- `/pair <token>` — whitelist yourself.

Developer:
- `go build -o pocketbrain .` — compile binary.
- `go test ./... -count=1` — run all tests.
- `go run . start` — run with TUI.
- `go run . start --headless` — run headless.

## Testing

222 tests across 11 packages. Run with `go test ./... -count=1`.

Test files live alongside source (`*_test.go`). No external mocking libraries — all mocks are hand-written stubs in test files.

## Extension Points

- Add channels: implement `core.ChannelAdapter`, register in `app.go`.
- Add AI tools: create in `internal/ai/tools_*.go`, register via `ai.Registry`.
- Add persistence: implement core port interface, bind in composition root.
- Add workspace features: extend `workspace.Workspace`, expose via tool.
