# Coding Architecture Guide 💻

This page explains how the code is structured at module/class level so contributors can jump in fast.

## 1) Composition Root (How the app is wired)

`src/index.ts` is the single composition root.

```mermaid
flowchart TB
  Index[src/index.ts]
  Index --> Repos[SQLite Repositories]
  Index --> Runtime[RuntimeProvider]
  Index --> Sessions[SessionManager]
  Index --> Prompts[PromptBuilder]
  Index --> Assistant[AssistantCore]
  Index --> Vault[VaultService + vaultProvider]
  Index --> Scheduler[HeartbeatScheduler]
  Index --> Channels[ChannelManager + WhatsAppAdapter]
```

Why this is good:
- ✅ No hidden global initialization
- ✅ Dependency graph is explicit
- ✅ Easier testing and refactoring

## 2) Core vs Adapters (Port-Adapter Style)

```mermaid
flowchart LR
  subgraph Core[Core 🧠]
    AC[AssistantCore]
    SM[SessionManager]
    PB[PromptBuilder]
    Ports[ports/* interfaces]
  end

  subgraph Adapters[Adapters 🔌]
    WA[channels/whatsapp/*]
    SQL[adapters/persistence/repositories/*]
    Plugins[adapters/plugins/*]
  end

  subgraph Infra[Infrastructure 🧱]
    OC[OpenCode SDK]
    DB[(SQLite state.db)]
    VF[(Vault files)]
  end

  AC --> Ports
  WA --> AC
  SQL --> DB
  Plugins --> VF
  AC --> OC
  Ports --> SQL
```

Rule of thumb:
- Core defines behavior/contracts.
- Adapters implement I/O details.

## 3) Chat Request Code Path

Main entrypoint: `AssistantCore.ask()` in `src/core/assistant.ts`.

```mermaid
sequenceDiagram
  participant A as AssistantCore
  participant SR as SessionManager
  participant MR as MemoryRepository
  participant PB as PromptBuilder
  participant OC as OpenCode Client
  participant CR as ChannelRepository

  A->>SR: getOrCreateMainSession()
  A->>CR: saveLastChannel() (whatsapp)
  A->>MR: getAll()
  A->>PB: buildAgentSystemPrompt(memory)
  A->>OC: session.prompt(system + input)
  OC-->>A: parts[]
  A-->>Caller: extracted text response
```

## 4) Heartbeat Code Path

Heartbeat scheduler lives in `src/scheduler/heartbeat.ts`, execution in `AssistantCore.runHeartbeatTasks()`.

```mermaid
sequenceDiagram
  participant HS as HeartbeatScheduler
  participant AC as AssistantCore
  participant HR as HeartbeatRepository
  participant SM as SessionManager
  participant OC as OpenCode Client
  participant OR as OutboxRepository

  HS->>AC: runHeartbeatTasks()
  AC->>HR: getTasks()
  AC->>SM: getOrCreateHeartbeatSession()
  AC->>SM: getOrCreateMainSession()
  AC->>OC: prompt heartbeat tasks
  AC->>OC: inject summary into main session
  AC->>OC: proactive notification decision prompt
  HS->>OR: enqueue failure notification (on repeated failures)
```

## 5) Vault + PKM Code Path

Vault API surface is in `src/vault/vault-service.ts`; tools are exposed via `src/adapters/plugins/vault.plugin.ts`.

```mermaid
flowchart TD
  Tool[vault_* tools]
  Tool --> VS[VaultService]
  VS --> FS[Filesystem under data/vault]
  VS --> Links[markdown-links.ts]
  VS --> Tags[markdown-tags.ts]

  Tool --> Search[vault_search mode: name/content/both]
  Tool --> Backlinks[vault_backlinks]
  Tool --> TagSearch[vault_tag_search]
```

Recent PKM-related code points:
- `searchFiles(query, folder, mode)`
- `findBacklinks(target, folder)`
- `searchByTag(tag, folder)`

## 6) Persistence Model (SQLite)

Schema bootstrap: `src/store/db.ts`.

Main tables:
- `kv` (session IDs, last-channel)
- `memory` (durable facts)
- `whitelist` (channel access)
- `outbox` (proactive queued messages + retry metadata)
- `heartbeat_tasks` (scheduled agent tasks)

## 7) Test Strategy Map 🧪

```mermaid
flowchart LR
  Unit[Unit tests]
  Unit --> VaultTests[tests/vault/*]
  Unit --> CoreTests[tests/core/*]
  Unit --> RepoTests[tests/adapters/persistence/*]
  Unit --> ChannelTests[tests/adapters/channels/*]
```

Command baseline:
```bash
bun run typecheck
bun test
```

## 8) Practical Extension Points

- Add new channel: implement `ChannelAdapter` and register in `src/index.ts`.
- Add new tool/plugin: create under `src/adapters/plugins/` and wire via OpenCode config.
- Add new persistence implementation: implement core port, bind in composition root.
- Add new vault capability: extend `VaultService` first, then expose tool.
