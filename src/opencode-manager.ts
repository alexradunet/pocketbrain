/**
 * OpenCode Manager for PocketBrain
 * Runs one OpenCode server natively,
 * manages per-group sessions via the SDK client.
 */
import fs from 'fs';
import path from 'path';
import { createOpencode } from '@opencode-ai/sdk';

import { DATA_DIR, GROUPS_DIR, MAIN_GROUP_FOLDER } from './config.js';
import { readEnvFile } from './env.js';
import { logger } from './logger.js';
import { RegisteredGroup } from './types.js';

// --- Public types ---

export interface AgentInput {
  prompt: string;
  sessionId?: string;
  groupFolder: string;
  chatJid: string;
  isMain: boolean;
  isScheduledTask?: boolean;
}

export interface AgentOutput {
  status: 'success' | 'error';
  result: string | null;
  newSessionId?: string;
  error?: string;
}

export interface AvailableGroup {
  jid: string;
  name: string;
  lastActivity: string;
  isRegistered: boolean;
}

// --- Internal state ---

type OpencodeClient = Awaited<ReturnType<typeof createOpencode>>;

interface ActiveSession {
  sessionId: string;
  onOutput: (output: AgentOutput) => Promise<void>;
  resolveEnd: () => void;
  busy: boolean;
  /** pocketbrain_context block re-injected on every follow-up to survive compaction */
  contextPrefix: string;
}

let opencodeInstance: OpencodeClient | null = null;
const activeSessions = new Map<string, ActiveSession>();
const PROMPT_STREAM_TIMEOUT_MS = 120000;

/** @internal - for tests only. Injects a mock opencode instance. */
export function _setTestOpencodeInstance(mock: unknown): void {
  opencodeInstance = mock as OpencodeClient;
}

/** @internal - for tests only. Clears the activeSessions map. */
export function _clearActiveSessions(): void {
  activeSessions.clear();
}

/** @internal - for tests only. Exposes buildContextPrefix. */
export function _buildContextPrefix(group: RegisteredGroup, input: AgentInput): string {
  return buildContextPrefix(group, input);
}

// --- Server lifecycle ---

export async function boot(): Promise<void> {
  // Load optional OpenCode env vars from .env so the server inherits them.
  const secrets = readEnvFile([
    'OPENCODE_API_KEY',
    'OPENCODE_MODEL',
    'OPENCODE_BASE_URL',
  ]);
  if (secrets.OPENCODE_API_KEY) {
    process.env.OPENCODE_API_KEY = secrets.OPENCODE_API_KEY;
  }
  if (secrets.OPENCODE_MODEL) {
    process.env.OPENCODE_MODEL = secrets.OPENCODE_MODEL;
  }
  if (secrets.OPENCODE_BASE_URL) {
    process.env.OPENCODE_BASE_URL = secrets.OPENCODE_BASE_URL;
  }

  const mcpServerPath = path.join(process.cwd(), 'dist', 'pocketbrain-mcp');

  // Sync skills from container/skills/ to a location OpenCode can discover
  const skillsSrc = path.join(process.cwd(), 'container', 'skills');
  const skillsDst = path.join(process.cwd(), '.opencode', 'skills');
  if (fs.existsSync(skillsSrc)) {
    try {
      for (const skillDir of fs.readdirSync(skillsSrc)) {
        const srcDir = path.join(skillsSrc, skillDir);
        if (!fs.statSync(srcDir).isDirectory()) continue;
        const dstDir = path.join(skillsDst, skillDir);
        fs.mkdirSync(dstDir, { recursive: true });
        for (const file of fs.readdirSync(srcDir)) {
          fs.copyFileSync(path.join(srcDir, file), path.join(dstDir, file));
        }
      }
    } catch (err) {
      logger.warn({ err }, 'Skills sync failed — continuing without skills');
    }
  }

  // Build global instructions list
  const instructions: string[] = [];
  const globalInstructionsPath = path.join(GROUPS_DIR, 'global', 'AGENTS.md');
  if (fs.existsSync(globalInstructionsPath)) {
    instructions.push(globalInstructionsPath);
  }

  const config: Record<string, unknown> = {
    permission: {
      edit: 'allow',
      bash: 'allow',
      webfetch: 'allow',
    },
    mcp: {
      pocketbrain: {
        type: 'local',
        command: [mcpServerPath],
        environment: {
          POCKETBRAIN_IPC_DIR: path.join(DATA_DIR, 'ipc'),
        },
      },
    },
    tools: {
      bash: true,
      edit: true,
      write: true,
      read: true,
      glob: true,
      grep: true,
      websearch: true,
      webfetch: true,
      task: true,
    },
    instructions: instructions.length > 0 ? instructions : undefined,
  };
  // Model selection is env-driven to follow OpenCode defaults/configuration flow.
  if (process.env.OPENCODE_MODEL) {
    config.model = process.env.OPENCODE_MODEL;
  }
  // Inject Ollama provider when model uses the ollama/ prefix.
  // baseURL defaults to localhost for local dev; override via OPENCODE_BASE_URL
  // in Docker Compose (e.g. http://ollama:11434/v1).
  if (process.env.OPENCODE_MODEL?.startsWith('ollama/')) {
    const baseURL = process.env.OPENCODE_BASE_URL || 'http://localhost:11434/v1';
    config.providers = {
      ollama: {
        npm: '@ai-sdk/openai-compatible',
        name: 'Ollama',
        options: { baseURL },
      },
    };
    logger.info({ baseURL }, 'Ollama provider configured');
  }

  logger.info('Starting OpenCode server...');
  opencodeInstance = await createOpencode({
    hostname: '127.0.0.1',
    port: 4096,
    timeout: 10000,
    config,
  });
  logger.info('OpenCode server started on port 4096');
}

export async function shutdown(): Promise<void> {
  // Abort all active sessions
  for (const [groupFolder, session] of activeSessions) {
    logger.info({ groupFolder }, 'Aborting session on shutdown');
    session.resolveEnd();
  }
  activeSessions.clear();

  if (opencodeInstance) {
    opencodeInstance.server.close();
    opencodeInstance = null;
    logger.info('OpenCode server stopped');
  }
}

// --- Session lifecycle ---

/**
 * Start (or resume) an agent session for a group.
 * Runs the first prompt, calls onOutput with the result, then blocks
 * until the session is ended via abortSession(). Follow-ups arrive
 * through sendFollowUp() while the session is alive.
 */
export async function startSession(
  group: RegisteredGroup,
  input: AgentInput,
  onOutput: (output: AgentOutput) => Promise<void>,
): Promise<AgentOutput> {
  if (!opencodeInstance) throw new Error('OpenCode server not booted');
  const { client } = opencodeInstance;

  // Ensure IPC directories exist for this group
  const groupIpcDir = path.join(DATA_DIR, 'ipc', input.groupFolder);
  fs.mkdirSync(path.join(groupIpcDir, 'messages'), { recursive: true });
  fs.mkdirSync(path.join(groupIpcDir, 'tasks'), { recursive: true });

  // Create or resume session (with timeout to avoid hanging on a stalled server)
  const SESSION_INIT_TIMEOUT_MS = 15000;
  let sessionId: string;
  let isNewSession = !input.sessionId;
  try {
    if (input.sessionId) {
      logger.debug({ sessionId: input.sessionId }, 'Resuming session');
      const resumed = await Promise.race([
        client.session.get({ path: { id: input.sessionId } }).then(() => true, () => false),
        Bun.sleep(SESSION_INIT_TIMEOUT_MS).then(() => false),
      ]);
      if (resumed) {
        sessionId = input.sessionId;
      } else {
        logger.warn({ sessionId: input.sessionId }, 'Stale session ID, creating new session');
        isNewSession = true;
        const resp = await Promise.race([
          client.session.create({ body: { title: `PocketBrain: ${group.name}` } }),
          Bun.sleep(SESSION_INIT_TIMEOUT_MS).then(() => {
            throw new Error('session.create timed out');
          }),
        ]);
        const newId = resp.data?.id;
        if (!newId) throw new Error('session.create returned no session ID');
        sessionId = newId;
        logger.info({ sessionId, group: group.name }, 'New session created (recovered stale session)');
      }
    } else {
      logger.debug({ group: group.name }, 'Creating new session');
      const resp = await Promise.race([
        client.session.create({ body: { title: `PocketBrain: ${group.name}` } }),
        Bun.sleep(SESSION_INIT_TIMEOUT_MS).then(() => {
          throw new Error('session.create timed out');
        }),
      ]);
      const newId = resp.data?.id;
      if (!newId) throw new Error('session.create returned no session ID');
      sessionId = newId;
      logger.info({ sessionId, group: group.name }, 'New session created');
    }
  } catch (err) {
    const error = err instanceof Error ? err.message : String(err);
    logger.error({ group: group.name, error }, 'Failed to create/resume session');
    return { status: 'error', result: null, error };
  }

  // Create end-of-session promise
  let resolveEnd!: () => void;
  const endPromise = new Promise<void>((r) => {
    resolveEnd = r;
  });

  // Build the context prefix once — re-injected on every follow-up to survive compaction
  const contextPrefix = buildContextPrefix(group, input);

  // Register active session
  activeSessions.set(input.groupFolder, {
    sessionId,
    onOutput,
    resolveEnd,
    busy: false,
    contextPrefix,
  });

  // Build prompt
  let prompt = input.prompt;
  if (input.isScheduledTask) {
    prompt = `[SCHEDULED TASK - The following message was sent automatically and is not coming directly from the user or group.]\n\n${prompt}`;
  }
  // Prepend group context for new sessions (including recovered stale sessions)
  if (isNewSession) {
    prompt = buildGroupContext(group, input) + '\n\n' + prompt;
  }

  // Run first prompt
  const session = activeSessions.get(input.groupFolder)!;
  session.busy = true;
  try {
    const result = await runPrompt(client, sessionId, prompt);
    await onOutput(result);
    if (result.status === 'error') {
      activeSessions.delete(input.groupFolder);
      return result;
    }
    // Emit session-update marker so host can track session ID
    await onOutput({ status: 'success', result: null, newSessionId: sessionId });
  } catch (err) {
    const error = err instanceof Error ? err.message : String(err);
    logger.error({ group: group.name, error }, 'Prompt error');
    await onOutput({ status: 'error', result: null, error });
    activeSessions.delete(input.groupFolder);
    return { status: 'error', result: null, error };
  } finally {
    session.busy = false;
  }

  // Wait for session to end (via abortSession or shutdown)
  await endPromise;
  activeSessions.delete(input.groupFolder);

  logger.info({ group: group.name, sessionId }, 'Session ended');
  return { status: 'success', result: null, newSessionId: sessionId };
}

/**
 * Send a follow-up prompt to an active session.
 * Returns true if the prompt was sent, false if no active session.
 */
export async function sendFollowUp(
  groupFolder: string,
  text: string,
): Promise<boolean> {
  if (!opencodeInstance) return false;
  const session = activeSessions.get(groupFolder);
  if (!session || session.busy) return false;

  const { client } = opencodeInstance;
  session.busy = true;
  try {
    // Re-inject context prefix on every follow-up so it survives session compaction
    const promptText = session.contextPrefix
      ? `${session.contextPrefix}\n\n${text}`
      : text;
    const result = await runPrompt(client, session.sessionId, promptText);
    await session.onOutput(result);
    if (result.status === 'error') return true;
    await session.onOutput({
      status: 'success',
      result: null,
      newSessionId: session.sessionId,
    });
  } catch (err) {
    const error = err instanceof Error ? err.message : String(err);
    logger.error({ groupFolder, error }, 'Follow-up error');
    await session.onOutput({ status: 'error', result: null, error });
  } finally {
    session.busy = false;
  }
  return true;
}

/**
 * Abort an active session. Resolves the startSession() promise.
 */
export function abortSession(groupFolder: string): void {
  const session = activeSessions.get(groupFolder);
  if (!session) return;

  logger.debug({ groupFolder, sessionId: session.sessionId }, 'Aborting session');

  // Abort the running prompt if busy
  if (session.busy && opencodeInstance) {
    opencodeInstance.client.session
      .abort({ path: { id: session.sessionId } })
      .catch((err) => logger.debug({ err, sessionId: session.sessionId }, 'Session abort failed (non-fatal)'));
  }

  session.resolveEnd();
}

/**
 * Check if a group has an active session.
 */
export function hasActiveSession(groupFolder: string): boolean {
  return activeSessions.has(groupFolder);
}

// --- Internal helpers ---

async function runPrompt(
  client: OpencodeClient['client'],
  sessionId: string,
  text: string,
): Promise<AgentOutput> {
  const messageId = crypto.randomUUID();
  const signal = new AbortController();
  const textParts = new Map<string, string>();
  const textPartOrder: string[] = [];
  let messageError: string | null = null;
  let sawTargetMessage = false;

  const eventStream = await client.event.subscribe({
    signal: signal.signal,
  });

  const streamDone = (async () => {
    try {
      for await (const rawEvent of eventStream.stream) {
        const event = asRecord(rawEvent);
        if (!event) continue;
        const eventType = asString(event.type);
        const properties = asRecord(event.properties);

        if (eventType === 'message.part.updated' && properties) {
          const part = asRecord(properties.part);
          if (!part) continue;
          if (asString(part.sessionID) !== sessionId) continue;
          if (asString(part.messageID) !== messageId) continue;
          if (asString(part.type) !== 'text') continue;

          const partId = asString(part.id);
          if (!partId) continue;
          if (!textParts.has(partId)) textPartOrder.push(partId);

          const current = textParts.get(partId) || '';
          const delta = asString(properties.delta);
          if (delta) {
            textParts.set(partId, current + delta);
            continue;
          }
          const nextText = asString(part.text);
          if (nextText !== null) {
            textParts.set(partId, nextText);
          }
          continue;
        }

        if (eventType === 'message.updated' && properties) {
          const info = asRecord(properties.info);
          if (!info) continue;
          if (asString(info.sessionID) !== sessionId) continue;
          if (asString(info.id) !== messageId) continue;
          sawTargetMessage = true;
          const err = info.error;
          if (err) {
            messageError = formatSessionError(err);
          }
          continue;
        }

        if (eventType === 'session.idle' && properties) {
          if (asString(properties.sessionID) === sessionId && sawTargetMessage) {
            break;
          }
        }
      }
    } finally {
      signal.abort();
    }
  })();

  try {
    await client.session.promptAsync({
      path: { id: sessionId },
      body: {
        messageID: messageId,
        parts: [{ type: 'text', text }],
      },
    });
  } catch (err) {
    signal.abort();
    await streamDone.catch(() => {});
    const error = err instanceof Error ? err.message : String(err);
    return {
      status: 'error',
      result: null,
      error,
      newSessionId: sessionId,
    };
  }

  let timedOut = false;
  await Promise.race([
    streamDone,
    Bun.sleep(PROMPT_STREAM_TIMEOUT_MS).then(() => {
      timedOut = true;
      signal.abort();
    }),
  ]);

  // Ensure we always have a canonical final snapshot, even if events were partial
  // or the stream disconnected. Cap with a timeout to avoid hanging indefinitely.
  const MESSAGE_FETCH_TIMEOUT_MS = 30000;
  const messageRespData = await Promise.race([
    client.session.message({ path: { id: sessionId, messageID: messageId } })
      .then((r) => r.data),
    Bun.sleep(MESSAGE_FETCH_TIMEOUT_MS).then(() => null),
  ]);
  const info = messageRespData?.info;
  if (info && 'error' in info && info.error) {
    return {
      status: 'error',
      result: null,
      error: formatSessionError(info.error),
      newSessionId: sessionId,
    };
  }

  const canonicalText = extractTextFromParts(messageRespData?.parts ?? []);
  const streamedText = joinTextParts(textParts, textPartOrder);
  const fullText = canonicalText || streamedText;

  if (messageError) {
    return {
      status: 'error',
      result: fullText || null,
      error: timedOut ? `${messageError} (stream timeout)` : messageError,
      newSessionId: sessionId,
    };
  }

  return {
    status: 'success',
    result: fullText || null,
    newSessionId: sessionId,
  };
}

function formatSessionError(err: unknown): string {
  if (!err || typeof err !== 'object') return String(err);
  const e = err as { name?: string; data?: unknown };
  const name = e.name || 'SessionError';
  if (typeof e.data === 'string') return `${name}: ${e.data}`;
  if (e.data && typeof e.data === 'object' && 'message' in (e.data as object)) {
    const msg = (e.data as { message?: unknown }).message;
    if (typeof msg === 'string' && msg) return `${name}: ${msg}`;
  }
  return `${name}: ${JSON.stringify(e.data ?? {})}`;
}

function extractTextFromParts(parts: Array<unknown>): string {
  return parts
    .map((p) => asRecord(p))
    .filter((p) => p && asString(p.type) === 'text')
    .map((p) => asString((p as Record<string, unknown>).text) ?? '')
    .join('');
}

function joinTextParts(
  textParts: Map<string, string>,
  order: string[],
): string {
  return order.map((id) => textParts.get(id) || '').join('');
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object'
    ? (value as Record<string, unknown>)
    : null;
}

function asString(value: unknown): string | null {
  return typeof value === 'string' ? value : null;
}

/**
 * Build the pocketbrain_context XML block for this session.
 * This is re-injected on every prompt (including follow-ups) so it survives
 * session compaction.
 */
function buildContextPrefix(group: RegisteredGroup, input: AgentInput): string {
  return `<pocketbrain_context>
chatJid: ${input.chatJid}
groupFolder: ${input.groupFolder}
isMain: ${input.isMain}

When using PocketBrain MCP tools (send_message, schedule_task, list_tasks, pause_task, resume_task, cancel_task, register_group), you MUST pass these values as parameters:
- chatJid: "${input.chatJid}"
- groupFolder: "${input.groupFolder}"
- isMain: ${input.isMain}
</pocketbrain_context>`;
}

function buildGroupContext(group: RegisteredGroup, input: AgentInput): string {
  const parts: string[] = [buildContextPrefix(group, input)];

  // Per-group AGENTS.md — injected only on new sessions (not follow-ups)
  const groupInstructionsPath = path.join(GROUPS_DIR, group.folder, 'AGENTS.md');
  if (fs.existsSync(groupInstructionsPath)) {
    parts.push(fs.readFileSync(groupInstructionsPath, 'utf-8'));
  }

  return parts.join('\n\n');
}

// --- Snapshot helpers ---

export function writeTasksSnapshot(
  groupFolder: string,
  isMain: boolean,
  tasks: Array<{
    id: string;
    groupFolder: string;
    prompt: string;
    schedule_type: string;
    schedule_value: string;
    status: string;
    next_run: string | null;
  }>,
): void {
  const groupIpcDir = path.join(DATA_DIR, 'ipc', groupFolder);
  fs.mkdirSync(groupIpcDir, { recursive: true });

  // Main sees all tasks, others only see their own
  const filteredTasks = isMain
    ? tasks
    : tasks.filter((t) => t.groupFolder === groupFolder);

  const tasksFile = path.join(groupIpcDir, 'current_tasks.json');
  fs.writeFileSync(tasksFile, JSON.stringify(filteredTasks, null, 2));
}

export function writeGroupsSnapshot(
  groupFolder: string,
  isMain: boolean,
  groups: AvailableGroup[],
  registeredJids: Set<string>,
): void {
  const groupIpcDir = path.join(DATA_DIR, 'ipc', groupFolder);
  fs.mkdirSync(groupIpcDir, { recursive: true });

  // Main sees all groups; others see nothing
  const visibleGroups = isMain ? groups : [];

  const groupsFile = path.join(groupIpcDir, 'available_groups.json');
  fs.writeFileSync(
    groupsFile,
    JSON.stringify(
      {
        groups: visibleGroups,
        lastSync: new Date().toISOString(),
      },
      null,
      2,
    ),
  );
}


