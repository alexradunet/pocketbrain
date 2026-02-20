# Coding Architecture Guide

This page explains how the code is structured at package level so contributors can jump in fast.

## 1) Composition Root (How the app is wired)

`main.go` -> `cmd/` (Cobra CLI) -> `internal/app/app.go` is the single composition root.

```mermaid
flowchart TB
  Main[main.go / cmd/]
  Main --> App[app.go composition root]
  App --> Config[config.Config]
  App --> Store[SQLite Repositories]
  App --> AI[AI Provider + Tool Registry]
  App --> Core[AssistantCore]
  App --> Scheduler[HeartbeatScheduler]
  App --> Channels[ChannelManager + WhatsAppAdapter]
  App --> TUI[Terminal UI]
```

Why this is good:
- No hidden global initialization
- Dependency graph is explicit
- Easier testing and refactoring

## 2) Core vs Adapters (Port-Adapter Style)

```mermaid
flowchart LR
  subgraph Core[Core]
    AC[AssistantCore]
    SM[SessionManager]
    PB[PromptBuilder]
    Ports[core/ports.go interfaces]
  end

  subgraph Adapters[Adapters]
    WA[channel/whatsapp/]
    SQL[store/]
    Tools[ai/tools_*.go]
  end

  subgraph Infra[Infrastructure]
    Provider[AI Provider API]
    DB[(SQLite state.db)]
    WS[(Workspace files)]
  end

  AC --> Ports
  WA --> AC
  SQL --> DB
  Tools --> WS
  AC --> Provider
  Ports --> SQL
```

Rule of thumb:
- Core defines behavior/contracts (`internal/core/`).
- Adapters implement I/O details (`internal/store/`, `internal/channel/`, `internal/ai/`).

## 3) Chat Request Code Path

Main entrypoint: `AssistantCore.Ask()` in `internal/core/assistant.go`.

```mermaid
sequenceDiagram
  participant A as AssistantCore
  participant SR as SessionManager
  participant MR as MemoryRepository
  participant PB as PromptBuilder
  participant AI as AI Provider
  participant CR as ChannelRepository

  A->>SR: GetOrCreateMainSession()
  A->>CR: SaveLastChannel()
  A->>MR: GetAll()
  A->>PB: BuildSystemPrompt(memory)
  A->>AI: SendMessage(system + input)
  AI-->>A: response (with tool calls)
  A->>A: Tool loop (if tool calls)
  A-->>Caller: extracted text response
```

## 4) Heartbeat Code Path

Heartbeat scheduler lives in `internal/scheduler/heartbeat.go`, execution in `AssistantCore.RunHeartbeatTasks()`.

```mermaid
sequenceDiagram
  participant HS as HeartbeatScheduler
  participant AC as AssistantCore
  participant HR as HeartbeatRepository
  participant SM as SessionManager
  participant AI as AI Provider
  participant OR as OutboxRepository

  HS->>AC: RunHeartbeatTasks()
  AC->>HR: GetTasks()
  AC->>SM: GetOrCreateHeartbeatSession()
  AC->>SM: GetOrCreateMainSession()
  AC->>AI: prompt heartbeat tasks
  AC->>AI: inject summary into main session
  AC->>AI: proactive notification decision prompt
  HS->>OR: enqueue failure notification (on repeated failures)
```

## 5) Workspace Code Path

Workspace operations are in `internal/workspace/workspace.go`; tools are exposed via `internal/ai/tools_workspace.go`.

```mermaid
flowchart TD
  Tool[workspace_* tools]
  Tool --> WS[Workspace]
  WS --> FS[Filesystem under .data/workspace]
  WS --> Search[workspace_search]
  WS --> Stats[workspace_stats]
```

## 6) Persistence Model (SQLite)

Schema bootstrap: `internal/store/db.go`.

Main tables:
- `session` (session IDs)
- `memory` (durable facts)
- `whitelist` (channel access)
- `outbox` (proactive queued messages + retry metadata)
- `heartbeat_tasks` (scheduled agent tasks)
- `channel` (last-used channel tracking)

## 7) Test Strategy

222+ tests across 11 packages. Test files live alongside source (`*_test.go`).

```bash
go test ./... -count=1        # run all tests
go test ./... -count=1 -race  # with race detection
go vet ./...                  # static analysis
```

No external mocking libraries — all mocks are hand-written stubs in test files.

## 8) Practical Extension Points

- Add new channel: implement `core.ChannelAdapter` (`internal/core/ports.go`), register in `app.go`.
- Add new AI tool: create in `internal/ai/tools_*.go`, register via `ai.Registry`.
- Add new persistence: implement core port interface, bind in composition root.
- Add new workspace feature: extend `workspace.Workspace`, expose via tool.
