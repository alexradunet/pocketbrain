import { CronExpressionParser } from 'cron-parser';
import fs from 'fs';
import path from 'path';

import {
  DATA_DIR,
  IDLE_TIMEOUT,
  SCHEDULER_POLL_INTERVAL,
  TIMEZONE,
} from './config.js';
import {
  AgentOutput,
  abortSession,
  startSession,
  writeTasksSnapshot,
} from './opencode-manager.js';
import {
  getAllTasks,
  getDueTasks,
  getTaskById,
  logTaskRun,
  updateTaskAfterRun,
} from './store.js';
import { SessionQueue } from './session-queue.js';
import { logger } from './logger.js';
import { ChatConfig, ScheduledTask } from './types.js';

export interface SchedulerDependencies {
  chats: () => Record<string, ChatConfig>;
  getSessions: () => Record<string, string>;
  queue: SessionQueue;
  sendMessage: (jid: string, text: string) => Promise<void>;
}

export async function runTask(
  task: ScheduledTask,
  deps: SchedulerDependencies,
): Promise<void> {
  const startTime = Date.now();
  const chatDir = path.join(DATA_DIR, 'chats', task.chatFolder);
  fs.mkdirSync(chatDir, { recursive: true });

  logger.info(
    { taskId: task.id, chat: task.chatFolder },
    'Running scheduled task',
  );

  const chats = deps.chats();
  const chat = Object.values(chats).find(
    (c) => c.folder === task.chatFolder,
  );

  if (!chat) {
    logger.error(
      { taskId: task.id, chatFolder: task.chatFolder },
      'Chat not found for task',
    );
    logTaskRun({
      task_id: task.id,
      run_at: new Date().toISOString(),
      duration_ms: Date.now() - startTime,
      status: 'error',
      result: null,
      error: `Chat not found: ${task.chatFolder}`,
    });
    return;
  }

  // Update tasks snapshot for agent to read
  const tasks = getAllTasks();
  writeTasksSnapshot(
    task.chatFolder,
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

  let result: string | null = null;
  let error: string | null = null;

  // For group context mode, use the chat's current session
  const sessions = deps.getSessions();
  const sessionId =
    task.context_mode === 'group' ? sessions[task.chatFolder] : undefined;

  // Idle timer: aborts session after IDLE_TIMEOUT of no output
  let idleTimer: ReturnType<typeof setTimeout> | null = null;

  const resetIdleTimer = () => {
    if (idleTimer) clearTimeout(idleTimer);
    idleTimer = setTimeout(() => {
      logger.debug({ taskId: task.id }, 'Scheduled task idle timeout, aborting session');
      abortSession(task.chatFolder);
    }, IDLE_TIMEOUT);
  };

  try {
    // Register session in queue for tracking
    deps.queue.registerSession(task.chat_jid, task.chatFolder, sessionId);

    const output = await startSession(
      chat,
      {
        prompt: task.prompt,
        sessionId,
        chatFolder: task.chatFolder,
        chatJid: task.chat_jid,
        isScheduledTask: true,
      },
      async (streamedOutput: AgentOutput) => {
        if (streamedOutput.result) {
          result = streamedOutput.result;
          // Forward result to user (sendMessage handles formatting)
          await deps.sendMessage(task.chat_jid, streamedOutput.result);
          // Only reset idle timer on actual results, not session-update markers
          resetIdleTimer();
        }
        if (streamedOutput.status === 'error') {
          error = streamedOutput.error || 'Unknown error';
        }
      },
    );

    if (idleTimer) clearTimeout(idleTimer);

    if (output.status === 'error') {
      error = output.error || 'Unknown error';
    } else if (output.result) {
      result = output.result;
    }

    logger.info(
      { taskId: task.id, durationMs: Date.now() - startTime },
      'Task completed',
    );
  } catch (err) {
    if (idleTimer) clearTimeout(idleTimer);
    error = err instanceof Error ? err.message : String(err);
    logger.error({ taskId: task.id, error }, 'Task failed');
  }

  const durationMs = Date.now() - startTime;

  logTaskRun({
    task_id: task.id,
    run_at: new Date().toISOString(),
    duration_ms: durationMs,
    status: error ? 'error' : 'success',
    result,
    error,
  });

  let nextRun: string | null = null;
  if (task.schedule_type === 'cron') {
    const interval = CronExpressionParser.parse(task.schedule_value, {
      tz: TIMEZONE,
    });
    nextRun = interval.next().toISOString();
  } else if (task.schedule_type === 'interval') {
    const ms = parseInt(task.schedule_value, 10);
    // Anchor to the scheduled run time to prevent drift accumulating over executions
    const anchor = task.next_run ? new Date(task.next_run).getTime() : Date.now();
    nextRun = new Date(anchor + ms).toISOString();
  }
  // 'once' tasks have no next run

  const resultSummary = error
    ? `Error: ${error}`
    : result
      ? result.slice(0, 200)
      : 'Completed';
  updateTaskAfterRun(task.id, nextRun, resultSummary);
}

let schedulerRunning = false;

/** @internal - for tests only. Resets the scheduler running state. */
export function _resetSchedulerState(): void {
  schedulerRunning = false;
}

export function startSchedulerLoop(deps: SchedulerDependencies): void {
  if (schedulerRunning) {
    logger.debug('Scheduler loop already running, skipping duplicate start');
    return;
  }
  schedulerRunning = true;
  logger.info('Scheduler loop started');

  const loop = async () => {
    try {
      const dueTasks = getDueTasks();
      if (dueTasks.length > 0) {
        logger.info({ count: dueTasks.length }, 'Found due tasks');
      }

      for (const task of dueTasks) {
        // Re-check task status in case it was paused/cancelled
        const currentTask = getTaskById(task.id);
        if (!currentTask || currentTask.status !== 'active') {
          continue;
        }

        deps.queue.enqueueTask(
          currentTask.chat_jid,
          currentTask.id,
          () => runTask(currentTask, deps),
        );
      }
    } catch (err) {
      logger.error({ err }, 'Error in scheduler loop');
    }

    setTimeout(loop, SCHEDULER_POLL_INTERVAL);
  };

  loop();
}
