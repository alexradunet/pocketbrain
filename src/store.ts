/**
 * File-based storage layer for PocketBrain.
 * Replaces SQLite (db.ts) with JSON files in data/.
 *
 * Layout:
 *   data/chats/{folder}/config.json   — per-chat config (jid, name, sessionId)
 *   data/tasks.json                   — all scheduled tasks
 *   data/state.json                   — router state
 *   data/logs/{taskId}.jsonl          — per-task run logs
 *   data/ipc/                         — IPC files (unchanged)
 *
 * All writes use atomic temp+rename to prevent corruption.
 */
import fs from 'fs';
import path from 'path';

import { DATA_DIR } from './config.js';
import type { ChatConfig, ScheduledTask, TaskRunLog } from './types.js';

// --- Internal state ---

let testDataDir: string | null = null;

function getDataDir(): string {
  return testDataDir ?? DATA_DIR;
}

/** @internal - for tests only. Override the data directory. */
export function _setTestDataDir(dir: string): void {
  testDataDir = dir;
}

/** @internal - for tests only. Reset to default data directory. */
export function _resetDataDir(): void {
  testDataDir = null;
}

// --- Helpers ---

function chatsDir(): string {
  return path.join(getDataDir(), 'chats');
}

function atomicWrite(filePath: string, data: string): void {
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  const tmpPath = `${filePath}.tmp`;
  fs.writeFileSync(tmpPath, data);
  fs.renameSync(tmpPath, filePath);
}

// --- Data dir setup ---

export function ensureDataDirs(): void {
  const dir = getDataDir();
  fs.mkdirSync(path.join(dir, 'chats'), { recursive: true });
  fs.mkdirSync(path.join(dir, 'ipc'), { recursive: true });
  fs.mkdirSync(path.join(dir, 'logs'), { recursive: true });
}

// --- Chat CRUD ---

export function loadAllChats(): Record<string, ChatConfig> {
  const dir = chatsDir();
  if (!fs.existsSync(dir)) return {};

  const result: Record<string, ChatConfig> = {};
  for (const folder of fs.readdirSync(dir)) {
    const configPath = path.join(dir, folder, 'config.json');
    if (!fs.existsSync(configPath)) continue;
    try {
      const chat = JSON.parse(fs.readFileSync(configPath, 'utf-8')) as ChatConfig;
      result[chat.jid] = chat;
    } catch {
      // skip corrupt config files
    }
  }
  return result;
}

export function getChatByJid(jid: string): ChatConfig | undefined {
  const all = loadAllChats();
  return all[jid];
}

export function getChatByFolder(folder: string): ChatConfig | undefined {
  const all = loadAllChats();
  return Object.values(all).find((c) => c.folder === folder);
}

export function saveChat(chat: ChatConfig): void {
  const configPath = path.join(chatsDir(), chat.folder, 'config.json');
  atomicWrite(configPath, JSON.stringify(chat, null, 2));
}

// --- Task CRUD ---

function tasksPath(): string {
  return path.join(getDataDir(), 'tasks.json');
}

export function loadTasks(): ScheduledTask[] {
  const fp = tasksPath();
  if (!fs.existsSync(fp)) return [];
  try {
    return JSON.parse(fs.readFileSync(fp, 'utf-8')) as ScheduledTask[];
  } catch {
    return [];
  }
}

export function saveTasks(tasks: ScheduledTask[]): void {
  atomicWrite(tasksPath(), JSON.stringify(tasks, null, 2));
}

export function getTaskById(id: string): ScheduledTask | undefined {
  return loadTasks().find((t) => t.id === id);
}

export function createTask(
  task: Omit<ScheduledTask, 'last_run' | 'last_result'>,
): void {
  const tasks = loadTasks();
  tasks.push({ ...task, last_run: null, last_result: null });
  saveTasks(tasks);
}

export function updateTask(
  id: string,
  updates: Partial<
    Pick<
      ScheduledTask,
      'prompt' | 'schedule_type' | 'schedule_value' | 'next_run' | 'status'
    >
  >,
): void {
  const tasks = loadTasks();
  const idx = tasks.findIndex((t) => t.id === id);
  if (idx === -1) return;
  Object.assign(tasks[idx], updates);
  saveTasks(tasks);
}

export function deleteTask(id: string): void {
  const tasks = loadTasks();
  saveTasks(tasks.filter((t) => t.id !== id));
  // Clean up log file
  const logFile = path.join(getDataDir(), 'logs', `${id}.jsonl`);
  try {
    fs.unlinkSync(logFile);
  } catch {
    /* ignore — log file may not exist */
  }
}

export function getDueTasks(): ScheduledTask[] {
  const now = new Date().toISOString();
  return loadTasks().filter(
    (t) => t.status === 'active' && t.next_run != null && t.next_run <= now,
  );
}

export function updateTaskAfterRun(
  id: string,
  nextRun: string | null,
  lastResult: string,
): void {
  const tasks = loadTasks();
  const idx = tasks.findIndex((t) => t.id === id);
  if (idx === -1) return;
  tasks[idx].last_run = new Date().toISOString();
  tasks[idx].last_result = lastResult;
  tasks[idx].next_run = nextRun;
  if (nextRun === null) tasks[idx].status = 'completed';
  saveTasks(tasks);
}

export function getAllTasks(): ScheduledTask[] {
  return loadTasks();
}

export function getTasksForChat(chatFolder: string): ScheduledTask[] {
  return loadTasks().filter((t) => t.chatFolder === chatFolder);
}

// --- Task run logging ---

export function logTaskRun(log: TaskRunLog): void {
  const logDir = path.join(getDataDir(), 'logs');
  fs.mkdirSync(logDir, { recursive: true });
  const logFile = path.join(logDir, `${log.task_id}.jsonl`);
  fs.appendFileSync(logFile, JSON.stringify(log) + '\n');
}

// --- Router state ---

export interface RouterState {
  lastGroupSync?: string;
  [key: string]: unknown;
}

function statePath(): string {
  return path.join(getDataDir(), 'state.json');
}

export function loadState(): RouterState {
  const fp = statePath();
  if (!fs.existsSync(fp)) return {};
  try {
    return JSON.parse(fs.readFileSync(fp, 'utf-8')) as RouterState;
  } catch {
    return {};
  }
}

export function saveState(state: RouterState): void {
  atomicWrite(statePath(), JSON.stringify(state, null, 2));
}
