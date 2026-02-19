# PocketBrain Architecture Overview 🧠

This page explains how PocketBrain works end-to-end, with diagrams and a fast mental model.

## 1) Big Picture

- Goal: one assistant runtime with persistent memory and a local markdown vault.
- Style: dependency-injected core, adapter-based infrastructure, SQLite state.
- Runtime service: `pocketbrain`.

```mermaid
flowchart LR
  User[User 📱] --> WA[WhatsApp Adapter]
  WA --> CM[ChannelManager]
  CM --> AC[AssistantCore]
  AC --> OC[OpenCode Runtime]
  AC --> DB[(SQLite state.db)]
  AC --> VP[Vault Plugin]
  VP --> Vault[(.data/vault Markdown)]
```

## 2) Request Flow (Normal Chat)

```mermaid
sequenceDiagram
  participant U as User
  participant W as WhatsAppAdapter
  participant C as ChannelManager
  participant A as AssistantCore
  participant S as SessionManager
  participant O as OpenCode
  participant D as SQLite

  U->>W: message
  W->>C: normalized text
  C->>A: ask(channel, userID, text)
  A->>S: getOrCreateMainSession()
  A->>D: load memory + save last channel
  A->>O: session.prompt(system + user text)
  O-->>A: assistant parts
  A-->>C: final text
  C-->>W: send response
  W-->>U: message delivered
```

## 3) Data Ownership Model

- `.data/vault/` = long-lived knowledge files (Markdown).
- `.data/state.db` = runtime state (sessions, memory, whitelist, outbox, heartbeat tasks).
- Content stays in files, operational state stays in SQLite.

## 4) Core Components

- `src/index.ts` — composition root, wires dependencies.
- `src/core/assistant.ts` — orchestrates prompts/sessions/memory context.
- `src/core/session-manager.ts` — main + heartbeat session lifecycle.
- `src/scheduler/heartbeat.ts` — periodic tasks with retry and notification.
- `src/vault/vault-service.ts` — vault reads/writes/search/backlinks/tags.
- `src/store/db.ts` — SQLite schema bootstrapping.

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
    Plugins[Tool Plugins]
  end

  subgraph Infra[Infrastructure]
    OC[OpenCode Runtime]
    SQL[(SQLite)]
    VF[(Vault Files)]
  end

  WA --> AC
  AC --> SM
  AC --> PB
  AC --> Ports
  Ports --> Repos
  Plugins --> VF
  Repos --> SQL
  AC --> OC
```

## 6) Why This Architecture Is Practical

- Easy to reason about: composition root + explicit dependencies.
- Testable: core depends on ports, tests can mock adapters.
- Markdown vault remains tool-agnostic.
- Reliable operations: outbox retries + heartbeat retries + WAL SQLite.

## 7) Where To Read Next

- Repo structure contract: `docs/architecture/repository-structure.md`
- Security model: `docs/architecture/security-threat-model.md`
- Dev onboarding: `docs/setup/developer-onboarding.md`
