# 🌱 Level 1 — Junior Developer Guide

> **Who this is for:** Someone new to the project, or a user who wants to
> understand what's happening under the hood. No deep TypeScript knowledge
> required. See the [emoji legend](./GUIDE.md#️⃣-emoji-concept-legend) for
> the visual anchors used throughout.

---

## 💬 What Does PocketBrain Actually Do?

Imagine you have an 🧠 AI assistant living inside a box (🐳 Docker container)
connected to your WhatsApp. You can:

- **💬 Chat with it** — ask it questions, give it tasks
- **⏰ Schedule it** — "every Monday at 9am, summarize my week"
- **⚡ Let it act** — it can browse the 🌐 web, run shell commands, read/write files

The key insight 💡: **the AI runs inside a container**, so it has full power
(can install packages, run any command) but cannot touch your real computer
outside the 🐳 container.

---

## 🗺️ The Journey of a Single Message

Here's what happens when you send `@Andy what's the weather in Berlin?` in WhatsApp:

```
1. 💬 Your phone ──► WhatsApp servers ──► Baileys library
                                                │
2.                                   🔄 PocketBrain receives it
                                                │
3.                                   👥 Is it a registered group? ─── No ──► ignore
                                                │ Yes
4.                                   🎯 Does it have "@Andy"? ────── No ──► store, wait
                                                │ Yes
5.                                   🗄️ Save message to SQLite database
                                                │
6.                                   🧠 Send to AI agent (OpenCode SDK)
                                                │
7.                                   🌐 Agent thinks, browses web, etc.
                                                │
8.                                   📝 Agent writes response
                                                │
9.                                   💬 PocketBrain sends reply via WhatsApp
                                                │
10.                                  📱 You see the response on your phone
```

---

## 📖 Key Concepts (Plain English)

### 👥 Registered Groups
PocketBrain only responds to specific WhatsApp groups/chats that you've
"registered" with it. Think of it like a VIP list. Unregistered groups are
completely ignored — even if you send `@Andy` there, nothing happens.

### 👑 The Main Group
One special registered group is called "main." This is typically your own
self-chat (messaging yourself in WhatsApp). The main group is like the
admin panel:

- It can register new 👥 groups
- It can see and manage all ⏰ scheduled tasks
- It can send 💬 messages to any other group

### 🎯 Trigger Word
In non-main groups, the 🧠 AI only wakes up when someone says `@Andy` (or
whatever name you configured). In the 👑 main group, every message gets a
response.

### 🔄 Sessions
The AI remembers your conversation. Each 👥 group has its own "session" that
persists between messages. This is how it can say "remember earlier when
you mentioned…" — it's reading from the ongoing conversation context stored
in its 🧠 memory.

### ⏰ Scheduled Tasks
You can ask the AI to schedule recurring jobs:
```
@Andy every morning at 8am, check Hacker News for AI news and message me a summary
```
The agent sets up a task in the 🗄️ database. Every minute, PocketBrain checks
if any tasks are due, and runs the 🧠 AI again with that prompt.

---

## 🔌 What the AI Can Do (Tools)

The 🧠 AI agent has access to these capabilities:

| Tool | What it does |
|------|-------------|
| 🖥️ **Bash** | Run shell commands inside the 🐳 container |
| 🌐 **Web search** | Search the internet |
| 🌐 **Web fetch** | Read any webpage |
| 📁 **Read/Edit/Write** | Files in the container filesystem |
| 💬 **send_message** | Send a WhatsApp message immediately (progress updates!) |
| ⏰ **schedule_task** | Create a new scheduled task |
| 👥 **register_group** | Add a new group to respond to (👑 main only) |

---

## 🗂️ Where Data Lives

Everything is stored in the `/workspace` directory (a 🐳 Docker volume on
your host machine):

```
workspace/
├── 🗄️ store/
│   ├── messages.db      ← SQLite database (all messages, groups, tasks)
│   └── auth/            ← WhatsApp login credentials 🔒
├── 📁 data/
│   └── ipc/             ← 🧠 AI writes JSON files here → host reads them
│       ├── main/
│       │   ├── messages/   ← pending 💬 messages to send
│       │   └── tasks/      ← pending ⏰ task operations
│       └── [group-name]/
└── 📝 groups/
    ├── global/
    │   └── AGENTS.md    ← Instructions for ALL groups
    ├── main/
    │   └── AGENTS.md    ← Instructions just for 👑 main group
    └── [group-name]/
        └── AGENTS.md    ← Instructions for this 👥 group
```

---

## 🚀 How to Run It

```bash
# 1. Clone and configure
git clone https://github.com/qwibitai/pocketbrain.git
cd pocketbrain
cp .env.example .env       # Add OPENCODE_API_KEY and TS_AUTHKEY 🔑

# 2. Build and start
bun run docker:build        # 🐳 Build the container image
bun run docker:up           # 🚀 Start the container

# 3. Check it's working
bun run docker:logs         # 📋 Watch the logs
bun run docker:test         # ✅ Run the test suite
```

---

## 💬 How to Talk to the AI

1. Open WhatsApp
2. Message yourself (self-chat) — this is your 👑 "main" group
3. Type: `@Andy hello, what can you do?` 🎯
4. Wait a moment — you'll see a typing indicator, then a response 🧠

To add another group:
1. From your self-chat: `@Andy register the group "Family Chat"` 👥
2. The AI will look at available groups and register it

---

## 📝 Customizing Behavior

PocketBrain doesn't use configuration files for behavior. Instead:

- **Per-group instructions**: Edit `workspace/groups/[group-name]/AGENTS.md` 📝
- **Global instructions**: Edit `workspace/groups/global/AGENTS.md` 📝
- **Code changes**: The codebase is small enough to modify directly 🧩

Example `AGENTS.md` for a group:
```markdown
# 👥 Family Chat Agent

Always respond in Spanish. Keep answers short. When someone asks about
the schedule, check the calendar file at /workspace/groups/family/calendar.md.
```

---

## ❓ Frequently Asked Questions

**Q: Why does the 🧠 AI sometimes take a while to respond?**
The AI runs a full reasoning process: it might search the 🌐 web, run commands,
read files. Complex requests take longer.

**Q: Can other people in my WhatsApp group use it?**
Yes! Anyone in a registered 👥 group can use `@Andy`. The AI responds to
all their messages.

**Q: What happens if the 🐳 container restarts?**
All state is saved in 🗄️ SQLite and files. The AI picks up where it left off —
same 🔄 sessions, same ⏰ scheduled tasks.

**Q: Is my data private? 🔒**
Your WhatsApp messages are stored in the 🗄️ SQLite database inside your
workspace volume (on your machine). The AI processes them via the OpenCode
API (cloud). Your WhatsApp auth credentials never leave your machine.

---

*Next: [🌿 Level 2 — Intermediate Guide](./GUIDE_INTERMEDIATE.md)*
