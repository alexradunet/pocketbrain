import { MAX_CONCURRENT_SESSIONS } from './config.js';
import { logger } from './logger.js';

interface QueuedTask {
  id: string;
  chatJid: string;
  fn: () => Promise<void>;
}

const MAX_RETRIES = 5;
const BASE_RETRY_MS = 5000;

interface ChatState {
  active: boolean;
  pendingMessages: boolean;
  pendingTasks: QueuedTask[];
  chatFolder: string | null;
  sessionId: string | null;
  retryCount: number;
  runningTaskId: string | null;
}

export class SessionQueue {
  private chats = new Map<string, ChatState>();
  private activeCount = 0;
  private waitingChats: string[] = [];
  private processMessagesFn: ((chatJid: string) => Promise<boolean>) | null =
    null;
  private sendFollowUpFn:
    | ((chatFolder: string, text: string) => Promise<boolean>)
    | null = null;
  private abortSessionFn: ((chatFolder: string) => void) | null = null;
  private shuttingDown = false;

  private getChat(chatJid: string): ChatState {
    let state = this.chats.get(chatJid);
    if (!state) {
      state = {
        active: false,
        pendingMessages: false,
        pendingTasks: [],
        chatFolder: null,
        sessionId: null,
        retryCount: 0,
        runningTaskId: null,
      };
      this.chats.set(chatJid, state);
    }
    return state;
  }

  setProcessMessagesFn(fn: (chatJid: string) => Promise<boolean>): void {
    this.processMessagesFn = fn;
  }

  setSendFollowUpFn(
    fn: (chatFolder: string, text: string) => Promise<boolean>,
  ): void {
    this.sendFollowUpFn = fn;
  }

  setAbortSessionFn(fn: (chatFolder: string) => void): void {
    this.abortSessionFn = fn;
  }

  enqueueMessageCheck(chatJid: string): void {
    if (this.shuttingDown) return;

    const state = this.getChat(chatJid);

    if (state.active) {
      state.pendingMessages = true;
      logger.debug({ chatJid }, 'Session active, message queued');
      return;
    }

    if (this.activeCount >= MAX_CONCURRENT_SESSIONS) {
      state.pendingMessages = true;
      if (!this.waitingChats.includes(chatJid)) {
        this.waitingChats.push(chatJid);
      }
      logger.debug(
        { chatJid, activeCount: this.activeCount },
        'At concurrency limit, message queued',
      );
      return;
    }

    this.runForChat(chatJid, 'messages');
  }

  enqueueTask(chatJid: string, taskId: string, fn: () => Promise<void>): void {
    if (this.shuttingDown) return;

    const state = this.getChat(chatJid);

    // Prevent double-queuing of the same task (even if currently running)
    if (state.pendingTasks.some((t) => t.id === taskId) || state.runningTaskId === taskId) {
      logger.debug({ chatJid, taskId }, 'Task already queued or running, skipping');
      return;
    }

    if (state.active) {
      state.pendingTasks.push({ id: taskId, chatJid, fn });
      logger.debug({ chatJid, taskId }, 'Session active, task queued');
      return;
    }

    if (this.activeCount >= MAX_CONCURRENT_SESSIONS) {
      state.pendingTasks.push({ id: taskId, chatJid, fn });
      if (!this.waitingChats.includes(chatJid)) {
        this.waitingChats.push(chatJid);
      }
      logger.debug(
        { chatJid, taskId, activeCount: this.activeCount },
        'At concurrency limit, task queued',
      );
      return;
    }

    // Run immediately
    this.runTask(chatJid, { id: taskId, chatJid, fn });
  }

  /**
   * Register the active session for a chat so follow-ups can be routed.
   */
  registerSession(chatJid: string, chatFolder: string, sessionId?: string): void {
    const state = this.getChat(chatJid);
    state.chatFolder = chatFolder;
    if (sessionId) state.sessionId = sessionId;
  }

  /**
   * Send a follow-up message to the active session via the OpenCode SDK.
   * Returns true if the message was accepted, false if no active session or
   * the session rejected it (e.g. busy). Callers must NOT advance their
   * message cursor when this returns false.
   */
  async sendMessage(chatJid: string, text: string): Promise<boolean> {
    const state = this.getChat(chatJid);
    if (!state.active || !state.chatFolder) return false;
    if (!this.sendFollowUpFn) return false;

    try {
      return await this.sendFollowUpFn(state.chatFolder, text);
    } catch (err) {
      logger.error({ chatJid, err }, 'Follow-up send error');
      return false;
    }
  }

  /**
   * Signal the active session to wind down.
   */
  closeStdin(chatJid: string): void {
    const state = this.getChat(chatJid);
    if (!state.active || !state.chatFolder) return;

    if (this.abortSessionFn) {
      this.abortSessionFn(state.chatFolder);
    }
  }

  private async runForChat(
    chatJid: string,
    reason: 'messages' | 'drain',
  ): Promise<void> {
    const state = this.getChat(chatJid);
    state.active = true;
    state.pendingMessages = false;
    this.activeCount++;

    logger.debug(
      { chatJid, reason, activeCount: this.activeCount },
      'Starting session for chat',
    );

    try {
      if (this.processMessagesFn) {
        const success = await this.processMessagesFn(chatJid);
        if (success) {
          state.retryCount = 0;
        } else {
          this.scheduleRetry(chatJid, state);
        }
      }
    } catch (err) {
      logger.error({ chatJid, err }, 'Error processing messages for chat');
      this.scheduleRetry(chatJid, state);
    } finally {
      state.active = false;
      state.chatFolder = null;
      state.sessionId = null;
      this.activeCount--;
      this.drainChat(chatJid);
    }
  }

  private async runTask(chatJid: string, task: QueuedTask): Promise<void> {
    const state = this.getChat(chatJid);
    state.active = true;
    state.runningTaskId = task.id;
    this.activeCount++;

    logger.debug(
      { chatJid, taskId: task.id, activeCount: this.activeCount },
      'Running queued task',
    );

    try {
      await task.fn();
    } catch (err) {
      logger.error({ chatJid, taskId: task.id, err }, 'Error running task');
    } finally {
      state.active = false;
      state.runningTaskId = null;
      state.chatFolder = null;
      state.sessionId = null;
      this.activeCount--;
      this.drainChat(chatJid);
    }
  }

  private scheduleRetry(chatJid: string, state: ChatState): void {
    state.retryCount++;
    if (state.retryCount > MAX_RETRIES) {
      logger.error(
        { chatJid, retryCount: state.retryCount },
        'Max retries exceeded, dropping messages (will retry on next incoming message)',
      );
      state.retryCount = 0;
      return;
    }

    const delayMs = BASE_RETRY_MS * Math.pow(2, state.retryCount - 1);
    logger.info(
      { chatJid, retryCount: state.retryCount, delayMs },
      'Scheduling retry with backoff',
    );
    setTimeout(() => {
      if (!this.shuttingDown) {
        this.enqueueMessageCheck(chatJid);
      }
    }, delayMs);
  }

  private drainChat(chatJid: string): void {
    if (this.shuttingDown) return;

    const state = this.getChat(chatJid);

    // Tasks first (they won't be re-discovered like messages)
    if (state.pendingTasks.length > 0) {
      const task = state.pendingTasks.shift()!;
      this.runTask(chatJid, task);
      return;
    }

    // Then pending messages
    if (state.pendingMessages) {
      this.runForChat(chatJid, 'drain');
      return;
    }

    // Nothing pending for this chat; check if other chats are waiting for a slot
    this.drainWaiting();
  }

  private drainWaiting(): void {
    while (
      this.waitingChats.length > 0 &&
      this.activeCount < MAX_CONCURRENT_SESSIONS
    ) {
      const nextJid = this.waitingChats.shift()!;
      const state = this.getChat(nextJid);

      // Prioritize tasks over messages
      if (state.pendingTasks.length > 0) {
        const task = state.pendingTasks.shift()!;
        this.runTask(nextJid, task);
      } else if (state.pendingMessages) {
        this.runForChat(nextJid, 'drain');
      }
      // If neither pending, skip this chat
    }
  }

  async shutdown(gracePeriodMs: number): Promise<void> {
    this.shuttingDown = true;

    const activeSessions: string[] = [];
    for (const [_jid, state] of this.chats) {
      if (state.active && state.chatFolder) {
        activeSessions.push(state.chatFolder);
      }
    }

    logger.info(
      { activeCount: this.activeCount, activeSessions },
      'SessionQueue shutting down',
    );

    if (this.activeCount === 0) return;

    // Wait for active sessions to finish, up to gracePeriodMs
    const deadline = Date.now() + gracePeriodMs;
    while (this.activeCount > 0 && Date.now() < deadline) {
      await Bun.sleep(100);
    }

    if (this.activeCount > 0) {
      logger.warn({ activeCount: this.activeCount }, 'Shutdown grace period expired with active sessions');
    }
  }
}
