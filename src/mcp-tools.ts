/**
 * Stdio MCP Server for PocketBrain
 * Runs as a child process of the OpenCode server.
 * Tools accept chatJid/chatFolder as parameters (no env var context).
 * IPC directory comes from POCKETBRAIN_IPC_DIR environment variable.
 *
 * Server-side identity validation:
 * - MCP_CHAT_FOLDER: authoritative chat folder set by parent process at spawn time.
 *   When set, the agent-provided chatFolder is IGNORED — the env var value is used instead.
 * If not set (backwards compat), the agent-provided value is used as-is
 * (the IPC watcher still enforces server-side authorization on the receiving end).
 */

import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { z } from 'zod';
import fs from 'fs';
import path from 'path';
import { CronExpressionParser } from 'cron-parser';

const IPC_DIR = process.env.POCKETBRAIN_IPC_DIR || path.join(process.cwd(), 'data', 'ipc');

/** Authoritative chat identity from environment, set by parent process at spawn time */
const ENV_CHAT_FOLDER: string | undefined = process.env.MCP_CHAT_FOLDER
  ? path.basename(process.env.MCP_CHAT_FOLDER)
  : undefined;

/**
 * Resolve the authoritative chatFolder for a tool call.
 * If MCP_CHAT_FOLDER is set, it overrides the agent-provided value entirely.
 * Falls back to safeFolder(agentFolder) for backwards compat.
 */
function resolveChatFolder(agentFolder: string): string {
  if (ENV_CHAT_FOLDER !== undefined) {
    return ENV_CHAT_FOLDER;
  }
  return safeFolder(agentFolder);
}

/** Sanitize chatFolder to prevent path traversal */
function safeFolder(folder: string): string {
  const sanitized = path.basename(folder);
  if (!sanitized || sanitized === '.' || sanitized === '..') {
    throw new Error(`Invalid chat folder: "${folder}"`);
  }
  return sanitized;
}

function writeIpcFile(dir: string, data: object): string {
  fs.mkdirSync(dir, { recursive: true });

  const filename = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}.json`;
  const filepath = path.join(dir, filename);

  // Atomic write: temp file then rename
  const tempPath = `${filepath}.tmp`;
  fs.writeFileSync(tempPath, JSON.stringify(data, null, 2));
  fs.renameSync(tempPath, filepath);

  return filename;
}

const server = new McpServer({
  name: 'pocketbrain',
  version: '1.0.0',
});

server.tool(
  'send_message',
  "Send a message to the user immediately while you're still running. Use this for progress updates or to send multiple messages. You can call this multiple times. Note: when running as a scheduled task, your final output is NOT sent to the user — use this tool if you need to communicate with the user.",
  {
    text: z.string().describe('The message text to send'),
    chatJid: z.string().describe('The chat JID to send to (from your pocketbrain_context)'),
    chatFolder: z.string().describe('The chat folder name (from your pocketbrain_context)'),
    sender: z.string().optional().describe('Your role/identity name (e.g. "Researcher"). When set, messages appear from a dedicated bot in Telegram.'),
  },
  async (args) => {
    const chatFolder = resolveChatFolder(args.chatFolder);
    const messagesDir = path.join(IPC_DIR, chatFolder, 'messages');
    const data: Record<string, string | undefined> = {
      type: 'message',
      chatJid: args.chatJid,
      text: args.text,
      sender: args.sender || undefined,
      chatFolder,
      timestamp: new Date().toISOString(),
    };

    writeIpcFile(messagesDir, data);

    return { content: [{ type: 'text' as const, text: 'Message sent.' }] };
  },
);

server.tool(
  'schedule_task',
  `Schedule a recurring or one-time task. The task will run as a full agent with access to all tools.

CONTEXT MODE - Choose based on task type:
\u2022 "group": Task runs in the conversation context, with access to chat history. Use for tasks that need context about ongoing discussions, user preferences, or recent interactions.
\u2022 "isolated": Task runs in a fresh session with no conversation history. Use for independent tasks that don't need prior context. When using isolated mode, include all necessary context in the prompt itself.

If unsure which mode to use, you can ask the user. Examples:
- "Remind me about our discussion" \u2192 group (needs conversation context)
- "Check the weather every morning" \u2192 isolated (self-contained task)
- "Follow up on my request" \u2192 group (needs to know what was requested)
- "Generate a daily report" \u2192 isolated (just needs instructions in prompt)

MESSAGING BEHAVIOR - The task agent's output is sent to the user. It can also use send_message for immediate delivery, or wrap output in <internal> tags to suppress it. Include guidance in the prompt about whether the agent should:
\u2022 Always send a message (e.g., reminders, daily briefings)
\u2022 Only send a message when there's something to report (e.g., "notify me if...")
\u2022 Never send a message (background maintenance tasks)

SCHEDULE VALUE FORMAT (all times are LOCAL timezone):
\u2022 cron: Standard cron expression (e.g., "*/5 * * * *" for every 5 minutes, "0 9 * * *" for daily at 9am LOCAL time)
\u2022 interval: Milliseconds between runs (e.g., "300000" for 5 minutes, "3600000" for 1 hour)
\u2022 once: Local time WITHOUT "Z" suffix (e.g., "2026-02-01T15:30:00"). Do NOT use UTC/Z suffix.`,
  {
    prompt: z.string().describe('What the agent should do when the task runs. For isolated mode, include all necessary context here.'),
    schedule_type: z.enum(['cron', 'interval', 'once']).describe('cron=recurring at specific times, interval=recurring every N ms, once=run once at specific time'),
    schedule_value: z.string().describe('cron: "*/5 * * * *" | interval: milliseconds like "300000" | once: local timestamp like "2026-02-01T15:30:00" (no Z suffix!)'),
    context_mode: z.enum(['group', 'isolated']).default('group').describe('group=runs with chat history and memory, isolated=fresh session (include context in prompt)'),
    chatJid: z.string().describe('The chat JID (from your pocketbrain_context)'),
    chatFolder: z.string().describe('The chat folder name (from your pocketbrain_context)'),
  },
  async (args) => {
    // Validate schedule_value before writing IPC
    if (args.schedule_type === 'cron') {
      try {
        CronExpressionParser.parse(args.schedule_value);
      } catch {
        return {
          content: [{ type: 'text' as const, text: `Invalid cron: "${args.schedule_value}". Use format like "0 9 * * *" (daily 9am) or "*/5 * * * *" (every 5 min).` }],
          isError: true,
        };
      }
    } else if (args.schedule_type === 'interval') {
      const ms = parseInt(args.schedule_value, 10);
      if (isNaN(ms) || ms <= 0) {
        return {
          content: [{ type: 'text' as const, text: `Invalid interval: "${args.schedule_value}". Must be positive milliseconds (e.g., "300000" for 5 min).` }],
          isError: true,
        };
      }
    } else if (args.schedule_type === 'once') {
      const date = new Date(args.schedule_value);
      if (isNaN(date.getTime())) {
        return {
          content: [{ type: 'text' as const, text: `Invalid timestamp: "${args.schedule_value}". Use ISO 8601 format like "2026-02-01T15:30:00".` }],
          isError: true,
        };
      }
    }

    const chatFolder = resolveChatFolder(args.chatFolder);
    const tasksDir = path.join(IPC_DIR, chatFolder, 'tasks');
    const data = {
      type: 'schedule_task',
      prompt: args.prompt,
      schedule_type: args.schedule_type,
      schedule_value: args.schedule_value,
      context_mode: args.context_mode || 'group',
      targetJid: args.chatJid,
      createdBy: chatFolder,
      timestamp: new Date().toISOString(),
    };

    const filename = writeIpcFile(tasksDir, data);

    return {
      content: [{ type: 'text' as const, text: `Task scheduled (${filename}): ${args.schedule_type} - ${args.schedule_value}` }],
    };
  },
);

server.tool(
  'list_tasks',
  "List all scheduled tasks for the current chat.",
  {
    chatFolder: z.string().describe('The chat folder name (from your pocketbrain_context)'),
  },
  async (args) => {
    const chatFolder = resolveChatFolder(args.chatFolder);
    const tasksFile = path.join(IPC_DIR, chatFolder, 'current_tasks.json');

    try {
      if (!fs.existsSync(tasksFile)) {
        return { content: [{ type: 'text' as const, text: 'No scheduled tasks found.' }] };
      }

      const tasks = JSON.parse(fs.readFileSync(tasksFile, 'utf-8'));

      if (tasks.length === 0) {
        return { content: [{ type: 'text' as const, text: 'No scheduled tasks found.' }] };
      }

      const formatted = tasks
        .map(
          (t: { id: string; prompt: string; schedule_type: string; schedule_value: string; status: string; next_run: string }) =>
            `- [${t.id}] ${t.prompt.slice(0, 50)}... (${t.schedule_type}: ${t.schedule_value}) - ${t.status}, next: ${t.next_run || 'N/A'}`,
        )
        .join('\n');

      return { content: [{ type: 'text' as const, text: `Scheduled tasks:\n${formatted}` }] };
    } catch (err) {
      return {
        content: [{ type: 'text' as const, text: `Error reading tasks: ${err instanceof Error ? err.message : String(err)}` }],
      };
    }
  },
);

server.tool(
  'pause_task',
  'Pause a scheduled task. It will not run until resumed.',
  {
    task_id: z.string().describe('The task ID to pause'),
    chatFolder: z.string().describe('The chat folder name (from your pocketbrain_context)'),
  },
  async (args) => {
    const chatFolder = resolveChatFolder(args.chatFolder);
    const tasksDir = path.join(IPC_DIR, chatFolder, 'tasks');
    const data = {
      type: 'pause_task',
      taskId: args.task_id,
      chatFolder,
      timestamp: new Date().toISOString(),
    };

    writeIpcFile(tasksDir, data);

    return { content: [{ type: 'text' as const, text: `Task ${args.task_id} pause requested.` }] };
  },
);

server.tool(
  'resume_task',
  'Resume a paused task.',
  {
    task_id: z.string().describe('The task ID to resume'),
    chatFolder: z.string().describe('The chat folder name (from your pocketbrain_context)'),
  },
  async (args) => {
    const chatFolder = resolveChatFolder(args.chatFolder);
    const tasksDir = path.join(IPC_DIR, chatFolder, 'tasks');
    const data = {
      type: 'resume_task',
      taskId: args.task_id,
      chatFolder,
      timestamp: new Date().toISOString(),
    };

    writeIpcFile(tasksDir, data);

    return { content: [{ type: 'text' as const, text: `Task ${args.task_id} resume requested.` }] };
  },
);

server.tool(
  'cancel_task',
  'Cancel and delete a scheduled task.',
  {
    task_id: z.string().describe('The task ID to cancel'),
    chatFolder: z.string().describe('The chat folder name (from your pocketbrain_context)'),
  },
  async (args) => {
    const chatFolder = resolveChatFolder(args.chatFolder);
    const tasksDir = path.join(IPC_DIR, chatFolder, 'tasks');
    const data = {
      type: 'cancel_task',
      taskId: args.task_id,
      chatFolder,
      timestamp: new Date().toISOString(),
    };

    writeIpcFile(tasksDir, data);

    return { content: [{ type: 'text' as const, text: `Task ${args.task_id} cancellation requested.` }] };
  },
);

// Graceful shutdown: exit cleanly on signals to avoid leaving .json.tmp orphans
process.on('SIGTERM', () => process.exit(0));
process.on('SIGINT', () => process.exit(0));

// Start the stdio transport
const transport = new StdioServerTransport();
await server.connect(transport);
