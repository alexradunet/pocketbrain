---
name: add-telegram
description: Add Telegram as a channel. Can replace WhatsApp entirely or run alongside it. Also configurable as a control-only channel (triggers actions) or passive channel (receives notifications only).
---

# Add Telegram Channel

This skill adds Telegram support to PocketBrain. Read the existing codebase first, then apply the changes below.

## Phase 1: Pre-flight

### Ask the user

1. **Mode**: Replace WhatsApp or add alongside it?
   - Replace → will set `TELEGRAM_ONLY=true`
   - Alongside → both channels active (default)

2. **Do they already have a bot token?** If yes, collect it now. If no, we'll create one in Phase 3.

## Phase 2: Apply Code Changes

Read `src/channels/whatsapp.ts` and `src/types.ts` to understand the `Channel` interface, then apply these changes:

### Install dependency

```bash
bun add grammy
```

### Create `src/channels/telegram.ts`

Implement a `TelegramChannel` class following the same `Channel` interface as `WhatsAppChannel`:

- Import `Bot` from `grammy`
- Constructor: accept `token: string`, `db: Database` (SQLite)
- `connect()` — start the bot with `bot.start()` (non-blocking, use `bot.start({ onStart: resolve })`)
- `disconnect()` — call `bot.stop()`
- `sendMessage(jid: string, text: string)` — parse `tg:<chatId>` JID format, call `bot.api.sendMessage(chatId, text)` with message splitting for messages over 4096 chars
- `ownsJid(jid: string)` — return `jid.startsWith('tg:')`
- Handle incoming messages: on `bot.on('message:text')`, store in SQLite `messages` table and `chats` table (same schema as WhatsApp uses), then trigger the group queue
- For group messages, sender name comes from `ctx.from?.first_name`
- JID format: `tg:<chatId>` for private chats, `tg:<groupId>` for groups

### Modify `src/index.ts`

Add TelegramChannel alongside WhatsApp:

```typescript
import { TelegramChannel } from './channels/telegram.js';

// After WhatsApp init:
if (TELEGRAM_BOT_TOKEN) {
  const telegram = new TelegramChannel(TELEGRAM_BOT_TOKEN, db);
  channels.push(telegram);
  await telegram.connect();
}
```

Check how `findChannel()` in `src/router.ts` works to ensure the channel routing handles `tg:` prefixed JIDs correctly.

### Add to `src/config.ts`

```typescript
export const TELEGRAM_BOT_TOKEN = process.env.TELEGRAM_BOT_TOKEN || '';
export const TELEGRAM_ONLY = process.env.TELEGRAM_ONLY === 'true';
```

If `TELEGRAM_ONLY=true`, skip initializing WhatsApp in `src/index.ts`.

### Update `.env.example`

```bash
TELEGRAM_BOT_TOKEN=
TELEGRAM_ONLY=false
```

### Validate changes

```bash
bun test
```

All existing tests must pass before proceeding.

## Phase 3: Setup

### Create Telegram Bot (if needed)

If the user doesn't have a bot token, tell them:

> I need you to create a Telegram bot:
>
> 1. Open Telegram and search for `@BotFather`
> 2. Send `/newbot` and follow prompts:
>    - Bot name: Something friendly (e.g., "Andy Assistant")
>    - Bot username: Must end with "bot" (e.g., "andy_ai_bot")
> 3. Copy the bot token (looks like `123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11`)

Wait for the user to provide the token.

### Configure environment

Add to `.env`:

```bash
TELEGRAM_BOT_TOKEN=<their-token>
```

If they chose to replace WhatsApp:

```bash
TELEGRAM_ONLY=true
```

### Disable Group Privacy (for group chats)

Tell the user:

> **Important for group chats**: By default, Telegram bots only see @mentions and commands in groups. To let the bot see all messages:
>
> 1. Open Telegram and search for `@BotFather`
> 2. Send `/mybots` and select your bot
> 3. Go to **Bot Settings** > **Group Privacy** > **Turn off**
>
> This is optional if you only want trigger-based responses via @mentioning the bot.

### Build and restart

```bash
bun run docker:build
bun run docker:up
```

## Phase 4: Registration

### Get Chat ID

Tell the user:

> 1. Open your bot in Telegram (search for its username)
> 2. Send `/chatid` — it will reply with the chat ID
> 3. For groups: add the bot to the group first, then send `/chatid` in the group

Wait for the user to provide the chat ID.

### Register the chat

Use the `register_group` MCP tool or add directly to SQLite. The JID format is `tg:<chatId>`.

For a main chat (responds to all messages, uses the `main` folder):

```typescript
registerGroup("tg:<chat-id>", {
  name: "<chat-name>",
  folder: "main",
  trigger: `@${ASSISTANT_NAME}`,
  added_at: new Date().toISOString(),
  requiresTrigger: false,
});
```

For additional chats (trigger-only):

```typescript
registerGroup("tg:<chat-id>", {
  name: "<chat-name>",
  folder: "<folder-name>",
  trigger: `@${ASSISTANT_NAME}`,
  added_at: new Date().toISOString(),
  requiresTrigger: true,
});
```

## Phase 5: Verify

### Test the connection

Tell the user:

> Send a message to your registered Telegram chat:
> - For main chat: Any message works
> - For non-main: `@Andy hello` or @mention the bot
>
> The bot should respond within a few seconds.

### Check logs if needed

```bash
bun run docker:logs
```

## Troubleshooting

### Bot not responding

1. Check `TELEGRAM_BOT_TOKEN` is set in `.env`
2. Check chat is registered: look in SQLite `registered_groups` table for `tg:` prefixed JIDs
3. For non-main chats: message must include trigger pattern
4. Check container logs: `bun run docker:logs`

### Bot only responds to @mentions in groups

Group Privacy is enabled (default). Fix:
1. `@BotFather` > `/mybots` > select bot > **Bot Settings** > **Group Privacy** > **Turn off**
2. Remove and re-add the bot to the group (required for the change to take effect)

### Getting chat ID

If `/chatid` doesn't work:
- Verify token: `curl -s "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getMe"`
- Check container logs: `bun run docker:logs`

## After Setup

Ask the user:

> Would you like to add Agent Swarm support? Each subagent appears as a different bot in the Telegram group. If interested, run `/add-telegram-swarm`.
