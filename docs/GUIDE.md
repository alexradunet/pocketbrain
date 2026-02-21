# 🧠 PocketBrain Architecture Guide

> **Choose your level.** This guide explains PocketBrain at three depths.
> Read the one that fits your background, or read all three in order.

---

## 💬 What is PocketBrain?

PocketBrain is a personal AI assistant that you talk to via WhatsApp.
You send a message like `summarize my week`, and an AI agent replies —
with full ability to run commands, browse the web, read/write files, and
schedule future tasks.

It runs entirely inside a 🐳 Docker container on your machine (or a VPS),
using [OpenCode SDK](https://opencode.ai) as the 🧠 AI backbone and
[WhatsApp Web](https://github.com/WhiskeySockets/Baileys) for 💬 messaging.

---

## 📚 The Guides

| Guide | Audience | What You'll Learn |
|-------|----------|-------------------|
| [🌱 Level 1 — Junior](./GUIDE_JUNIOR.md) | New developer or curious user | What PocketBrain does, how messages flow, key concepts explained simply |
| [🌿 Level 2 — Intermediate](./GUIDE_INTERMEDIATE.md) | Developer familiar with Node/TypeScript | Component responsibilities, data model, code references, configuration |
| [🌳 Level 3 — Architect](./GUIDE_ARCHITECT.md) | Senior engineer or system designer | Design decisions, tradeoffs, security model, extension points, concurrency |
| [🏗️ Build Your Own Agent](./GUIDE_BUILDER.md) | Anyone who wants to build their own personal AI agent | 10 reusable patterns extracted from PocketBrain — input channels, MCP tools, IPC, sessions, scheduling, security, skills |

---

## 🗺️ Emoji Concept Legend

These emojis are used **consistently** across all three guides as visual anchors:

| Emoji | Concept |
|-------|---------|
| 💬 | WhatsApp message / chat communication |
| 🧠 | AI agent / OpenCode SDK / intelligence |
| 🗄️ | SQLite database / persistent storage |
| 📁 | File system / IPC (inter-process communication) |
| ⏰ | Scheduler / cron / timed tasks |
| 🐳 | Docker container / runtime environment |
| 🌐 | Web access / network / internet |
| 👥 | Registered chat / conversation |
| 🔌 | MCP tools (send_message, schedule_task…) |
| 🧩 | Skills / extensions / code transformations |
| 🔄 | Session / state / conversation continuity |
| 🔀 | Queue / concurrency / parallelism |
| 🔑 | Configuration / environment variables |
| 📝 | AGENTS.md / memory / instructions |
| 🛡️ | Security / authorization / trust boundary |
| 📡 | SSE streaming / real-time events |
| 🔁 | Retry / backoff / recovery |
| 🚀 | Startup / boot / initialization |
| ⚡ | Performance / speed |
| 💡 | Key insight / design decision |
| ⚠️ | Warning / tradeoff / limitation |

---

## 🏗️ Quick Reference Map

```
💬 User (WhatsApp phone)
        │
        │  WhatsApp Web protocol (Baileys)
        ▼
┌─────────────────────────────────────────────────┐
│           🐳 Docker Container                    │
│                                                 │
│  💬 WhatsApp ──► 🗄️ SQLite ──► 🔄 Message Loop  │
│                                    │            │
│                                    ▼            │
│                             🔀 GroupQueue        │
│                                    │            │
│                                    ▼            │
│                           🧠 OpenCode SDK        │
│                           (AI Agent)            │
│                             │     ▲             │
│                      📁 IPC│     │              │
│                      (JSON)│     │              │
│                             ▼     │             │
│                       🔌 MCP Server             │
│                   (send_message,                │
│                    schedule_task…)              │
│                                                 │
│  ⏰ Task Scheduler ──────────────────────────► │
└─────────────────────────────────────────────────┘
        │
        │  WhatsApp Web protocol (Baileys)
        ▼
💬 User (WhatsApp phone)
```

---

## 🗂️ Source File Map

| File | One-line purpose |
|------|-----------------|
| `src/index.ts` | 🔄 Entry point, main message loop, orchestrator |
| `src/channels/whatsapp.ts` | 💬 WhatsApp connect/send/receive via Baileys |
| `src/channels/mock.ts` | 🧪 HTTP-based test double — replaces WhatsApp when `CHANNEL=mock` |
| `src/opencode-manager.ts` | 🧠 OpenCode SDK sessions — create, run, follow-up |
| `src/mcp-tools.ts` | 🔌 MCP tool server (send_message, schedule_task, …) |
| `src/ipc.ts` | 📁 Polls IPC files written by agent, executes on host |
| `src/group-queue.ts` | 🔀 Per-group concurrency control with retry backoff |
| `src/task-scheduler.ts` | ⏰ Runs due scheduled tasks on a 60-second loop |
| `src/db.ts` | 🗄️ All SQLite operations (messages, sessions, groups, tasks) |
| `src/router.ts` | 🔄 Message formatting and outbound channel routing |
| `src/config.ts` | 🔑 Environment-based constants (name, paths, timeouts) |
| `src/types.ts` | 📐 Shared TypeScript types |
| `src/logger.ts` | 📋 Pino structured logger |
| `src/e2e/harness.ts` | 🧪 E2E helpers — injectMessage, waitForResponse, llmAssert |
| `src/e2e/agent.test.ts` | 🧪 AI quality tests (math, geography, multi-turn) |
| `src/e2e/infra.test.ts` | 🧪 Infrastructure tests — routing, outbox, sessions (no API key) |
