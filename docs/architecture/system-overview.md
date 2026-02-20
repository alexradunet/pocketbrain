# PocketBrain Architecture Overview

This page explains how PocketBrain works end-to-end, with diagrams and a fast mental model.

## 1) Big Picture

- Goal: one assistant runtime with persistent memory and workspace file management.
- Style: dependency-injected core, adapter-based infrastructure, SQLite state.
- Runtime: single static Go binary, zero runtime dependencies.

```mermaid
flowchart LR
  User[User] --> WA[WhatsApp Adapter]
  WA --> CM[ChannelManager]
  CM --> AC[AssistantCore]
  AC --> AI[AI Provider API]
  AC --> DB[(SQLite state.db)]
  AC --> WS[Workspace Tools]
  WS --> Files[(.data/workspace)]
```

## 2) Request Flow (Normal Chat)

```mermaid
sequenceDiagram
  participant U as User
  participant W as WhatsAppAdapter
  participant C as ChannelManager
  participant A as AssistantCore
  participant S as SessionManager
  participant P as AI Provider
  participant D as SQLite

  U->>W: message
  W->>C: normalized text
  C->>A: Ask(channel, userID, text)
  A->>S: GetOrCreateMainSession()
  A->>D: load memory + save last channel
  A->>P: SendMessage(system + user text)
  P-->>A: response (+ tool calls)
  A->>A: tool loop (if needed)
  A-->>C: final text
  C-->>W: send response
  W-->>U: message delivered
```

## 3) Data Ownership Model

- `.data/workspace/` = long-lived knowledge files.
- `.data/state.db` = runtime state (sessions, memory, whitelist, outbox, heartbeat tasks).
- Content stays in files, operational state stays in SQLite.

## 4) Core Components

- `main.go` / `cmd/` — entry point and Cobra CLI.
- `internal/app/app.go` — composition root, wires all dependencies.
- `internal/core/assistant.go` — orchestrates prompts/sessions/memory context.
- `internal/core/session.go` — main + heartbeat session lifecycle.
- `internal/scheduler/heartbeat.go` — periodic tasks with retry and notification.
- `internal/workspace/workspace.go` — file operations with path security.
- `internal/store/db.go` — SQLite schema bootstrapping.

## 5) Layer Map

```mermaid
flowchart TB
  subgraph Core[Core Layer]
    AC[AssistantCore]
    SM[SessionManager]
    PB[PromptBuilder]
    Ports[Ports / Interfaces]
  end

  subgraph Adapters[Adapter Layer]
    WA[WhatsApp Adapter]
    Repos[SQLite Repositories]
    Tools[AI Tool Registry]
  end

  subgraph Infra[Infrastructure]
    AI[AI Provider API]
    SQL[(SQLite)]
    WS[(Workspace Files)]
  end

  WA --> AC
  AC --> SM
  AC --> PB
  AC --> Ports
  Ports --> Repos
  Tools --> WS
  Repos --> SQL
  AC --> AI
```

## 6) Why This Architecture Is Practical

- Easy to reason about: composition root + explicit dependencies.
- Testable: core depends on ports, tests can mock adapters.
- Single static binary: zero runtime dependencies, easy deployment.
- Reliable operations: outbox retries + heartbeat retries + WAL SQLite.

## 7) Where To Read Next

- Coding architecture: `docs/architecture/coding-architecture.md`
- Security model: `docs/architecture/security-threat-model.md`
- Developer setup: `docs/runbooks/dev-setup.md`
