<p align="center">
  <img src="assets/pocketbrain-logo.png" alt="PocketBrain" width="400">
</p>

<p align="center">
  My personal OpenCode assistant that runs securely in a Docker container with Tailscale. Lightweight and built to be understood and customized for your own needs.
</p>

<p align="center">
  <a href="README_zh.md">中文</a>&nbsp; • &nbsp;
  <a href="https://discord.gg/VDdww8qS42"><img src="https://img.shields.io/discord/1470188214710046894?label=Discord&logo=discord&v=2" alt="Discord" valign="middle"></a>&nbsp; • &nbsp;
  <a href="repo-tokens"><img src="repo-tokens/badge.svg" alt="34.9k tokens, 17% of context window" valign="middle"></a>
</p>

## Why I Built This

[OpenClaw](https://github.com/openclaw/openclaw) is an impressive project with a great vision. But I can't sleep well running software I don't understand with access to my life. OpenClaw has 52+ modules, 8 config management files, 45+ dependencies, and abstractions for 15 channel providers. Security is application-level (allowlists, pairing codes) rather than OS isolation. Everything runs in one Node process with shared memory.

PocketBrain gives you the same core functionality in a codebase you can understand in 8 minutes. One process. A handful of files. Everything runs in a single Docker container with full power — sandboxed from your host.

## Quick Start

```bash
git clone https://github.com/qwibitai/pocketbrain.git
cd pocketbrain
cp .env.example .env  # Add your OPENCODE_API_KEY and TS_AUTHKEY
bun run docker:build
bun run docker:up
bun run docker:test
```

Or with OpenCode CLI:
```bash
opencode
```
Then run `/setup`.

## Philosophy

**Small enough to understand.** One process, a few source files. No microservices, no message queues, no abstraction layers. Have OpenCode CLI walk you through it.

**Full power, sandboxed.** The agent runs inside a Debian 13 Docker container with full root access — it can install packages, configure services, run any command. The container IS the sandbox. Your host machine stays clean and safe.

**Single workspace.** All data lives in one `/workspace` directory as markdown files. Sync it to other devices with your tool of choice (Syncthing, rsync, etc.).

**Tailscale networking.** The container joins your tailnet with a stable hostname. SSH in, access services, share files — all through your private network.

**Built for one user.** This isn't a framework. It's working software that fits my exact needs. You fork it and have OpenCode CLI make it match your exact needs.

**Customization = code changes.** No configuration sprawl. Want different behavior? Modify the code. The codebase is small enough that this is safe.

**AI-native.** No installation wizard; OpenCode CLI guides setup. No monitoring dashboard; ask OpenCode what's happening. No debugging tools; describe the problem, OpenCode fixes it.

**Skills over features.** Contributors shouldn't add features (e.g. support for Telegram) to the codebase. Instead, they contribute [OpenCode CLI skills](https://code.opencode.com/docs/en/skills) like `/add-telegram` that transform your fork. You end up with clean code that does exactly what you need.

**Best harness, best model.** This runs on OpenCode SDK, which means you're running an open-source, model-agnostic agent harness. The harness matters. A bad harness makes even smart models seem dumb, a good harness gives them superpowers. OpenCode is (IMO) the best open-source harness available.

## What It Supports

- **WhatsApp I/O** - Message OpenCode from your phone
- **Full container power** - Agent can install packages, run any command inside the sandbox
- **Single workspace** - All data in `/workspace`, synced to other devices as markdown
- **Tailscale networking** - Container joins your tailnet with a stable hostname
- **Scheduled tasks** - Recurring jobs that run OpenCode and can message you back
- **Web access** - Search and fetch content
- **Agent Swarms** - Spin up teams of specialized agents that collaborate on complex tasks
- **Optional integrations** - Add Gmail (`/add-gmail`) and more via skills

## Usage

Talk to your assistant with the trigger word (default: `@Andy`):

```
@Andy send an overview of the sales pipeline every weekday morning at 9am (has access to my Obsidian vault folder)
@Andy review the git history for the past week each Friday and update the README if there's drift
@Andy every Monday at 8am, compile news on AI developments from Hacker News and TechCrunch and message me a briefing
```

From the main channel (your self-chat), you can manage groups and tasks:
```
@Andy list all scheduled tasks across groups
@Andy pause the Monday briefing task
@Andy join the Family Chat group
```

## Customizing

There are no configuration files to learn. Just tell OpenCode CLI what you want:

- "Change the trigger word to @Bob"
- "Remember in the future to make responses shorter and more direct"
- "Add a custom greeting when I say good morning"
- "Store conversation summaries weekly"

Or run `/customize` for guided changes.

The codebase is small enough that OpenCode can safely modify it.

## Requirements

- Docker (or Docker Desktop)
- [Tailscale account](https://tailscale.com) (free) + auth key
- OpenCode API key
- OpenCode CLI (for development and setup)

Development and testing workflows are Docker-only (`bun run dev`, `bun run build`, `bun run test` all run through Docker Compose).

## Architecture

```
Host Machine
├── docker-compose.yml
└── workspace/           ← synced to other devices
    ├── groups/          ← per-group memory (AGENTS.md)
    ├── data/            ← SQLite DB, IPC
    └── store/           ← WhatsApp auth

Docker Container (Debian 13)
├── Tailscale           ← joins your tailnet
├── Bun + PocketBrain  ← single process
│   ├── WhatsApp (baileys)
│   ├── OpenCode SDK
│   ├── Scheduler
│   └── MCP tools
└── Full system access  ← apt, git, curl, etc.
```

Single Bun process inside a Docker container. WhatsApp messages route through SQLite to OpenCode SDK. Agent has full bash access inside the container. All persistent data in `/workspace` volume.

Key files:
- `src/index.ts` - Orchestrator: state, message loop, agent invocation
- `src/opencode-manager.ts` - OpenCode SDK session management
- `src/channels/whatsapp.ts` - WhatsApp connection, auth, send/receive
- `src/mcp-tools.ts` - MCP tools (send_message, schedule_task, etc.)
- `src/group-queue.ts` - Per-group queue with global concurrency limit
- `src/task-scheduler.ts` - Runs scheduled tasks
- `src/db.ts` - SQLite operations (messages, groups, sessions, state)
- `Dockerfile` - Debian 13 + Bun + Tailscale
- `docker-compose.yml` - Container config with workspace volume

## FAQ

**Why WhatsApp and not Telegram/Signal/etc?**

Because I use WhatsApp. Fork it and run a skill to change it. That's the whole point.

**Why Docker + Tailscale?**

Docker gives the agent full power in a sandbox — it can install anything, run any command, without touching your host. Tailscale gives the container a stable address on your private network, so you can SSH in or sync files.

**Can I run this on any OS?**

Yes. If it runs Docker, it runs PocketBrain.

**Is this secure?**

The agent runs in a Docker container. It has full power inside the container but zero access to your host filesystem (except the workspace volume). The codebase is small enough that you can actually review what you're running.

**Why no configuration files?**

We don't want configuration sprawl. Every user should customize it so that the code matches exactly what they want rather than configuring a generic system.

**How do I debug issues?**

Ask OpenCode CLI. "Why isn't the scheduler running?" "What's in the recent logs?" "Why did this message not get a response?" That's the AI-native approach. Or run `/debug`.

## Contributing

**Don't add features. Add skills.**

If you want to add Telegram support, don't create a PR that adds Telegram alongside WhatsApp. Instead, contribute a skill file (`.opencode/skills/add-telegram/SKILL.md`) that teaches OpenCode CLI how to transform a PocketBrain installation to use Telegram.

Users then run `/add-telegram` on their fork and get clean code that does exactly what they need, not a bloated system trying to support every use case.

Skills should target OpenCode-compatible skills/agents and use Docker-first commands for tests and tooling.

## Community

Questions? Ideas? [Join the Discord](https://discord.gg/VDdww8qS42).

## License

MIT


