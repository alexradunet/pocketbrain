import fs from 'fs';
import path from 'path';

import { CronExpressionParser } from 'cron-parser';

import {
  DATA_DIR,
  IPC_POLL_INTERVAL,
  TIMEZONE,
} from './config.js';
import { createTask, deleteTask, getTaskById, updateTask } from './store.js';
import { logger } from './logger.js';
import { ChatConfig } from './types.js';

export interface IpcDeps {
  sendMessage: (jid: string, text: string) => Promise<void>;
  chats: () => Record<string, ChatConfig>;
}

let ipcWatcherRunning = false;

export function startIpcWatcher(deps: IpcDeps): void {
  if (ipcWatcherRunning) {
    logger.debug('IPC watcher already running, skipping duplicate start');
    return;
  }
  ipcWatcherRunning = true;

  const ipcBaseDir = path.join(DATA_DIR, 'ipc');
  fs.mkdirSync(ipcBaseDir, { recursive: true });

  // On startup: clean up stale error files (>7 days) and orphaned .json.tmp files
  try {
    const errorDir = path.join(ipcBaseDir, 'errors');
    if (fs.existsSync(errorDir)) {
      const cutoffMs = Date.now() - 7 * 24 * 60 * 60 * 1000;
      for (const f of fs.readdirSync(errorDir)) {
        const fp = path.join(errorDir, f);
        try {
          if (fs.statSync(fp).mtimeMs < cutoffMs) {
            fs.unlinkSync(fp);
            logger.debug({ file: f }, 'Cleaned up stale IPC error file');
          }
        } catch { /* ignore per-file errors */ }
      }
    }
    // Clean up orphaned temp files from interrupted atomic writes
    for (const entry of fs.readdirSync(ipcBaseDir)) {
      if (entry === 'errors') continue;
      const entryPath = path.join(ipcBaseDir, entry);
      if (!fs.statSync(entryPath).isDirectory()) continue;
      for (const sub of ['messages', 'tasks']) {
        const subDir = path.join(entryPath, sub);
        if (!fs.existsSync(subDir)) continue;
        for (const f of fs.readdirSync(subDir)) {
          if (f.endsWith('.json.tmp')) {
            try { fs.unlinkSync(path.join(subDir, f)); } catch { /* ignore */ }
          }
        }
      }
    }
  } catch (err) {
    logger.warn({ err }, 'IPC startup cleanup failed (non-fatal)');
  }

  const processIpcFiles = async () => {
    // Scan all chat IPC directories (identity determined by directory)
    let chatFolders: string[];
    try {
      chatFolders = fs.readdirSync(ipcBaseDir).filter((f) => {
        const stat = fs.statSync(path.join(ipcBaseDir, f));
        return stat.isDirectory() && f !== 'errors';
      });
    } catch (err) {
      logger.error({ err }, 'Error reading IPC base directory');
      setTimeout(processIpcFiles, IPC_POLL_INTERVAL);
      return;
    }

    const chats = deps.chats();

    for (const sourceFolder of chatFolders) {
      const messagesDir = path.join(ipcBaseDir, sourceFolder, 'messages');
      const tasksDir = path.join(ipcBaseDir, sourceFolder, 'tasks');

      // Process messages from this chat's IPC directory
      try {
        if (fs.existsSync(messagesDir)) {
          const messageFiles = fs
            .readdirSync(messagesDir)
            .filter((f) => f.endsWith('.json'));
          for (const file of messageFiles) {
            const filePath = path.join(messagesDir, file);
            try {
              const data = JSON.parse(fs.readFileSync(filePath, 'utf-8'));
              if (data.type === 'message' && data.chatJid && data.text) {
                // Authorization: the source folder must own this chatJid
                const targetChat = chats[data.chatJid];
                if (targetChat && targetChat.folder === sourceFolder) {
                  await deps.sendMessage(data.chatJid, data.text);
                  logger.info(
                    { chatJid: data.chatJid, sourceFolder },
                    'IPC message sent',
                  );
                } else {
                  logger.warn(
                    { chatJid: data.chatJid, sourceFolder },
                    'Unauthorized IPC message attempt blocked',
                  );
                }
              }
              fs.unlinkSync(filePath);
            } catch (err) {
              logger.error(
                { file, sourceFolder, err },
                'Error processing IPC message',
              );
              const errorDir = path.join(ipcBaseDir, 'errors');
              fs.mkdirSync(errorDir, { recursive: true });
              fs.renameSync(
                filePath,
                path.join(errorDir, `${sourceFolder}-${file}`),
              );
            }
          }
        }
      } catch (err) {
        logger.error(
          { err, sourceFolder },
          'Error reading IPC messages directory',
        );
      }

      // Process tasks from this chat's IPC directory
      try {
        if (fs.existsSync(tasksDir)) {
          const taskFiles = fs
            .readdirSync(tasksDir)
            .filter((f) => f.endsWith('.json'));
          for (const file of taskFiles) {
            const filePath = path.join(tasksDir, file);
            try {
              const data = JSON.parse(fs.readFileSync(filePath, 'utf-8'));
              // Pass source folder identity to processTaskIpc for authorization
              await processTaskIpc(data, sourceFolder, deps);
              fs.unlinkSync(filePath);
            } catch (err) {
              logger.error(
                { file, sourceFolder, err },
                'Error processing IPC task',
              );
              const errorDir = path.join(ipcBaseDir, 'errors');
              fs.mkdirSync(errorDir, { recursive: true });
              fs.renameSync(
                filePath,
                path.join(errorDir, `${sourceFolder}-${file}`),
              );
            }
          }
        }
      } catch (err) {
        logger.error({ err, sourceFolder }, 'Error reading IPC tasks directory');
      }
    }

    setTimeout(processIpcFiles, IPC_POLL_INTERVAL);
  };

  processIpcFiles();
  logger.info('IPC watcher started (per-chat namespaces)');
}

export async function processTaskIpc(
  data: {
    type: string;
    taskId?: string;
    prompt?: string;
    schedule_type?: string;
    schedule_value?: string;
    context_mode?: string;
    chatFolder?: string;
    chatJid?: string;
    targetJid?: string;
  },
  sourceFolder: string, // Verified identity from IPC directory
  deps: IpcDeps,
): Promise<void> {
  const chats = deps.chats();

  switch (data.type) {
    case 'schedule_task':
      if (
        data.prompt &&
        data.schedule_type &&
        data.schedule_value &&
        data.targetJid
      ) {
        // Resolve the target chat from JID
        const targetJid = data.targetJid as string;
        const targetChat = chats[targetJid];

        if (!targetChat) {
          logger.warn(
            { targetJid },
            'Cannot schedule task: target chat not registered',
          );
          break;
        }

        const targetFolder = targetChat.folder;

        // Authorization: can only schedule for own chat
        if (targetFolder !== sourceFolder) {
          logger.warn(
            { sourceFolder, targetFolder },
            'Unauthorized schedule_task attempt blocked',
          );
          break;
        }

        const scheduleType = data.schedule_type as 'cron' | 'interval' | 'once';

        let nextRun: string | null = null;
        if (scheduleType === 'cron') {
          try {
            const interval = CronExpressionParser.parse(data.schedule_value, {
              tz: TIMEZONE,
            });
            nextRun = interval.next().toISOString();
          } catch {
            logger.warn(
              { scheduleValue: data.schedule_value },
              'Invalid cron expression',
            );
            break;
          }
        } else if (scheduleType === 'interval') {
          const ms = parseInt(data.schedule_value, 10);
          if (isNaN(ms) || ms <= 0) {
            logger.warn(
              { scheduleValue: data.schedule_value },
              'Invalid interval',
            );
            break;
          }
          nextRun = new Date(Date.now() + ms).toISOString();
        } else if (scheduleType === 'once') {
          const scheduled = new Date(data.schedule_value);
          if (isNaN(scheduled.getTime())) {
            logger.warn(
              { scheduleValue: data.schedule_value },
              'Invalid timestamp',
            );
            break;
          }
          if (scheduled.getTime() <= Date.now()) {
            logger.warn(
              { scheduleValue: data.schedule_value },
              'Once task timestamp is in the past, rejecting',
            );
            break;
          }
          nextRun = scheduled.toISOString();
        }

        const taskId = `task-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
        const contextMode =
          data.context_mode === 'group' || data.context_mode === 'isolated'
            ? data.context_mode
            : 'isolated';
        createTask({
          id: taskId,
          chatFolder: targetFolder,
          chat_jid: targetJid,
          prompt: data.prompt,
          schedule_type: scheduleType,
          schedule_value: data.schedule_value,
          context_mode: contextMode,
          next_run: nextRun,
          status: 'active',
          created_at: new Date().toISOString(),
        });
        logger.info(
          { taskId, sourceFolder, targetFolder, contextMode },
          'Task created via IPC',
        );
      }
      break;

    case 'pause_task':
      if (data.taskId) {
        const task = getTaskById(data.taskId);
        if (task && task.chatFolder === sourceFolder) {
          updateTask(data.taskId, { status: 'paused' });
          logger.info(
            { taskId: data.taskId, sourceFolder },
            'Task paused via IPC',
          );
        } else {
          logger.warn(
            { taskId: data.taskId, sourceFolder },
            'Unauthorized task pause attempt',
          );
        }
      }
      break;

    case 'resume_task':
      if (data.taskId) {
        const task = getTaskById(data.taskId);
        if (task && task.chatFolder === sourceFolder) {
          // Recompute next_run from now so stale past timestamps don't fire immediately
          let resumeNextRun: string | undefined;
          if (task.schedule_type === 'cron') {
            try {
              const interval = CronExpressionParser.parse(task.schedule_value, { tz: TIMEZONE });
              resumeNextRun = interval.next().toISOString() ?? undefined;
            } catch {
              // Invalid cron — leave next_run as-is
            }
          } else if (task.schedule_type === 'interval') {
            const ms = parseInt(task.schedule_value, 10);
            if (!isNaN(ms) && ms > 0) {
              resumeNextRun = new Date(Date.now() + ms).toISOString();
            }
          }
          // 'once' tasks keep their original next_run
          updateTask(data.taskId, { status: 'active', ...(resumeNextRun ? { next_run: resumeNextRun } : {}) });
          logger.info(
            { taskId: data.taskId, sourceFolder },
            'Task resumed via IPC',
          );
        } else {
          logger.warn(
            { taskId: data.taskId, sourceFolder },
            'Unauthorized task resume attempt',
          );
        }
      }
      break;

    case 'cancel_task':
      if (data.taskId) {
        const task = getTaskById(data.taskId);
        if (task && task.chatFolder === sourceFolder) {
          deleteTask(data.taskId);
          logger.info(
            { taskId: data.taskId, sourceFolder },
            'Task cancelled via IPC',
          );
        } else {
          logger.warn(
            { taskId: data.taskId, sourceFolder },
            'Unauthorized task cancel attempt',
          );
        }
      }
      break;

    default:
      logger.warn({ type: data.type }, 'Unknown IPC task type');
  }
}
