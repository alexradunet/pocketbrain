import {
  IDLE_TIMEOUT,
} from './config.js';
import { WhatsAppChannel } from './channels/whatsapp.js';
import {
  AgentOutput,
  abortSession,
  boot as bootOpenCode,
  sendFollowUp,
  shutdown as shutdownOpenCode,
  startSession,
  writeTasksSnapshot,
} from './opencode-manager.js';
import {
  ensureDataDirs,
  getAllTasks,
  loadAllChats,
  saveChat,
  loadState,
  saveState,
} from './store.js';
import { SessionQueue } from './session-queue.js';
import { startIpcWatcher } from './ipc.js';
import { findChannel, formatMessages, formatOutbound } from './router.js';
import { startSchedulerLoop } from './task-scheduler.js';
import { Channel, ChatConfig, NewMessage } from './types.js';
import { logger } from './logger.js';


// --- In-memory state ---

/** Chats keyed by JID. Loaded from data/chats/*/config.json at startup. */
let chats: Record<string, ChatConfig> = {};

/** In-memory message buffer: new messages arrive here, drained on processing. */
const messageBuffer = new Map<string, NewMessage[]>();

const channels: Channel[] = [];
const queue = new SessionQueue();

// --- Chat helpers ---

function getSessionId(chatFolder: string): string | undefined {
  const chat = Object.values(chats).find((c) => c.folder === chatFolder);
  return chat?.sessionId;
}

function setSessionId(chatFolder: string, sessionId: string): void {
  const chat = Object.values(chats).find((c) => c.folder === chatFolder);
  if (chat) {
    chat.sessionId = sessionId;
    saveChat(chat);
  }
}

function getSessions(): Record<string, string> {
  const result: Record<string, string> = {};
  for (const chat of Object.values(chats)) {
    if (chat.sessionId) result[chat.folder] = chat.sessionId;
  }
  return result;
}

// --- Message buffer ---

function bufferMessage(chatJid: string, msg: NewMessage): void {
  const existing = messageBuffer.get(chatJid);
  if (existing) {
    existing.push(msg);
  } else {
    messageBuffer.set(chatJid, [msg]);
  }
}

function drainMessages(chatJid: string): NewMessage[] {
  const msgs = messageBuffer.get(chatJid) || [];
  messageBuffer.delete(chatJid);
  return msgs;
}

function reBufferMessages(chatJid: string, msgs: NewMessage[]): void {
  const existing = messageBuffer.get(chatJid) || [];
  // Prepend the re-buffered messages (they came first)
  messageBuffer.set(chatJid, [...msgs, ...existing]);
}

// --- Message processing ---

/**
 * Process all pending messages for a registered chat.
 * Called by the SessionQueue when it's this chat's turn.
 */
async function processChatMessages(chatJid: string): Promise<boolean> {
  const chat = chats[chatJid];
  if (!chat) return true;

  const channel = findChannel(channels, chatJid);
  if (!channel) {
    logger.warn({ chatJid }, 'No channel owns JID, skipping messages');
    return true;
  }

  const pendingMessages = drainMessages(chatJid);
  if (pendingMessages.length === 0) return true;

  const prompt = formatMessages(pendingMessages);

  logger.info(
    { chat: chat.name, messageCount: pendingMessages.length },
    'Processing messages',
  );

  // Track idle timer for closing session when agent is idle
  let idleTimer: ReturnType<typeof setTimeout> | null = null;

  const resetIdleTimer = () => {
    if (idleTimer) clearTimeout(idleTimer);
    idleTimer = setTimeout(() => {
      logger.debug({ chat: chat.name }, 'Idle timeout, aborting session');
      queue.closeStdin(chatJid);
    }, IDLE_TIMEOUT);
  };

  await channel.setTyping?.(chatJid, true);
  let hadError = false;
  let outputSentToUser = false;

  const output = await runAgent(chat, prompt, chatJid, async (result) => {
    // Streaming output callback — called for each agent result
    if (result.result) {
      const raw = typeof result.result === 'string' ? result.result : JSON.stringify(result.result);
      // Strip <internal>...</internal> blocks — agent uses these for internal reasoning
      const text = raw.replace(/<internal>[\s\S]*?<\/internal>/g, '').trim();
      logger.info({ chat: chat.name }, `Agent output: ${raw.slice(0, 200)}`);
      if (text) {
        await channel.sendMessage(chatJid, text);
        outputSentToUser = true;
      }
      // Only reset idle timer on actual results, not session-update markers (result: null)
      resetIdleTimer();
    }

    if (result.status === 'error') {
      hadError = true;
    }
  });

  await channel.setTyping?.(chatJid, false);
  if (idleTimer) clearTimeout(idleTimer);

  if (output === 'error' || hadError) {
    // If we already sent output to the user, don't re-buffer —
    // the user got their response and re-processing would send duplicates.
    if (outputSentToUser) {
      logger.warn({ chat: chat.name }, 'Agent error after output was sent, skipping re-buffer to prevent duplicates');
      return true;
    }
    // Re-buffer messages so retries can re-process them
    reBufferMessages(chatJid, pendingMessages);
    logger.warn({ chat: chat.name }, 'Agent error, re-buffered messages for retry');
    return false;
  }

  return true;
}

async function runAgent(
  chat: ChatConfig,
  prompt: string,
  chatJid: string,
  onOutput?: (output: AgentOutput) => Promise<void>,
): Promise<'success' | 'error'> {
  const sessionId = getSessionId(chat.folder);

  // Update tasks snapshot for agent to read
  const tasks = getAllTasks();
  writeTasksSnapshot(
    chat.folder,
    tasks.map((t) => ({
      id: t.id,
      chatFolder: t.chatFolder,
      prompt: t.prompt,
      schedule_type: t.schedule_type,
      schedule_value: t.schedule_value,
      status: t.status,
      next_run: t.next_run,
    })),
  );

  // Wrap onOutput to track session ID from results
  const wrappedOnOutput = onOutput
    ? async (output: AgentOutput) => {
        if (output.newSessionId) {
          setSessionId(chat.folder, output.newSessionId);
        }
        await onOutput(output);
      }
    : undefined;

  try {
    // Register the session in the queue for follow-ups
    queue.registerSession(chatJid, chat.folder, sessionId);

    const output = await startSession(
      chat,
      {
        prompt,
        sessionId,
        chatFolder: chat.folder,
        chatJid,
      },
      wrappedOnOutput ?? (async () => {}),
    );

    if (output.newSessionId) {
      setSessionId(chat.folder, output.newSessionId);
    }

    if (output.status === 'error') {
      logger.error(
        { chat: chat.name, error: output.error },
        'Agent error',
      );
      return 'error';
    }

    return 'success';
  } catch (err) {
    logger.error({ chat: chat.name, err }, 'Agent error');
    return 'error';
  }
}

// --- Inbound message handler (called directly by channel) ---

function onInboundMessage(chatJid: string, msg: NewMessage): void {
  // Only process messages for registered chats
  if (!chats[chatJid]) return;

  // Skip bot messages to avoid self-triggering
  if (msg.is_bot_message) return;

  bufferMessage(chatJid, msg);

  const chat = chats[chatJid];
  const channel = findChannel(channels, chatJid);

  // Try to pipe to active session first (follow-up message)
  const formatted = formatMessages([msg]);
  channel?.setTyping?.(chatJid, true);
  queue.sendMessage(chatJid, formatted).then((piped) => {
    if (piped) {
      // Message was piped to active session — drain it from buffer
      drainMessages(chatJid);
      logger.debug({ chatJid, chat: chat.name }, 'Piped message to active session');
    } else {
      // No active session — enqueue for a new one
      queue.enqueueMessageCheck(chatJid);
    }
    channel?.setTyping?.(chatJid, false);
  }).catch((err) => {
    logger.error({ chatJid, err }, 'Error piping message');
    queue.enqueueMessageCheck(chatJid);
    channel?.setTyping?.(chatJid, false);
  });
}

// --- Main ---

async function main(): Promise<void> {
  ensureDataDirs();
  logger.info('Data directories initialized');

  chats = loadAllChats();
  const state = loadState();
  logger.info(
    { chatCount: Object.keys(chats).length },
    'State loaded',
  );

  // Boot OpenCode server
  await bootOpenCode();

  // Wire up queue with OpenCode SDK functions
  queue.setProcessMessagesFn(processChatMessages);
  queue.setSendFollowUpFn(sendFollowUp);
  queue.setAbortSessionFn(abortSession);

  // Graceful shutdown handlers
  const shutdown = async (signal: string) => {
    logger.info({ signal }, 'Shutdown signal received');
    await queue.shutdown(10000);
    await shutdownOpenCode();
    for (const ch of channels) await ch.disconnect();
    process.exit(0);
  };
  process.on('SIGTERM', () => shutdown('SIGTERM'));
  process.on('SIGINT', () => shutdown('SIGINT'));

  // Channel callbacks (shared by all channels)
  const channelOpts = {
    onMessage: onInboundMessage,
    chats: () => chats,
  };

  // Create and connect channels — set CHANNEL=mock for e2e test mode
  const channelMode = process.env.CHANNEL || 'whatsapp';
  if (channelMode === 'mock') {
    const { MockChannel } = await import('./channels/mock.js');
    const mock = new MockChannel(channelOpts);
    channels.push(mock);
    await mock.connect();
  } else {
    const wa = new WhatsAppChannel(channelOpts);
    channels.push(wa);
    await wa.connect();
  }

  // Start subsystems
  startSchedulerLoop({
    chats: () => chats,
    getSessions,
    queue,
    sendMessage: async (jid, rawText) => {
      const channel = findChannel(channels, jid);
      if (!channel) {
        logger.warn({ jid }, 'No channel owns JID, cannot send message');
        return;
      }
      const text = formatOutbound(rawText);
      if (text) await channel.sendMessage(jid, text);
    },
  });
  startIpcWatcher({
    sendMessage: (jid, text) => {
      const channel = findChannel(channels, jid);
      if (!channel) throw new Error(`No channel for JID: ${jid}`);
      return channel.sendMessage(jid, text);
    },
    chats: () => chats,
  });

  logger.info('PocketBrain running');
}

// Guard: only run when executed directly, not when imported by tests
const isDirectRun = import.meta.main;

if (isDirectRun) {
  main().catch((err) => {
    logger.error({ err }, 'Failed to start PocketBrain');
    process.exit(1);
  });
}
