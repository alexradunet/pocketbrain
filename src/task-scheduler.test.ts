import { describe, it, expect, beforeEach, afterEach, vi, mock } from 'bun:test';
import fs from 'fs';
import path from 'path';
import os from 'os';

// Mock opencode-manager before importing task-scheduler so that startSession
// and writeTasksSnapshot are replaced with controllable stubs.
mock.module('./opencode-manager.js', () => ({
  startSession: vi.fn(),
  abortSession: vi.fn(),
  writeTasksSnapshot: vi.fn(),
}));

// Mock config constants
mock.module('./config.js', () => ({
  DATA_DIR: '/tmp/test-data',
  IDLE_TIMEOUT: 30000,
  SCHEDULER_POLL_INTERVAL: 60000,
  TIMEZONE: 'UTC',
}));

import {
  createTask,
  getTaskById,
  updateTask,
  getAllTasks,
  _setTestDataDir,
  _resetDataDir,
} from './store.js';
import { SessionQueue } from './session-queue.js';
import { ChatConfig, ScheduledTask } from './types.js';
import {
  runTask,
  startSchedulerLoop,
  _resetSchedulerState,
  SchedulerDependencies,
} from './task-scheduler.js';

// Grab the mocked module references so we can configure them per test.
import * as opencodeManager from './opencode-manager.js';

// ---- helpers ----

const NOW_ISO = new Date().toISOString();
const PAST_ISO = '2020-01-01T00:00:00.000Z';

let tmpDir: string;

function makeChat(folder: string): ChatConfig {
  return {
    jid: 'test@g.us',
    name: `Chat ${folder}`,
    folder,
    addedAt: NOW_ISO,
  };
}

/** Returns a minimal ScheduledTask with sensible defaults. */
function makeTask(overrides: Partial<ScheduledTask> = {}): Omit<ScheduledTask, 'last_run' | 'last_result'> {
  return {
    id: 'task-1',
    chatFolder: 'test-chat',
    chat_jid: 'test@g.us',
    prompt: 'do something',
    schedule_type: 'once',
    schedule_value: '2030-01-01T00:00:00.000Z',
    context_mode: 'isolated',
    next_run: '2030-01-01T00:00:00.000Z',
    status: 'active',
    created_at: NOW_ISO,
    ...overrides,
  };
}

/** Inserts a task into the store and returns the full object. */
function seedTask(overrides: Partial<ScheduledTask> = {}): ScheduledTask {
  const task = makeTask(overrides);
  createTask(task);
  return { ...task, last_run: null, last_result: null };
}

/** A SchedulerDependencies factory with controllable stubs. */
function makeSchedulerDeps(
  overrides: Partial<SchedulerDependencies> = {},
): SchedulerDependencies {
  const queue = new SessionQueue();
  return {
    chats: vi.fn(() => ({})),
    getSessions: vi.fn(() => ({})),
    queue,
    sendMessage: vi.fn(async () => {}),
    ...overrides,
  };
}

// ---- setup ----

beforeEach(() => {
  tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'scheduler-test-'));
  _setTestDataDir(tmpDir);
  fs.mkdirSync(path.join(tmpDir, 'chats'), { recursive: true });
  fs.mkdirSync(path.join(tmpDir, 'logs'), { recursive: true });

  _resetSchedulerState();

  // Default: startSession resolves with a success result
  (opencodeManager.startSession as ReturnType<typeof vi.fn>).mockResolvedValue({
    status: 'success',
    result: 'Task output',
    error: undefined,
  });
  (opencodeManager.writeTasksSnapshot as ReturnType<typeof vi.fn>).mockReturnValue(undefined);
  (opencodeManager.abortSession as ReturnType<typeof vi.fn>).mockReturnValue(undefined);
});

afterEach(() => {
  vi.clearAllMocks();
  _resetDataDir();
  fs.rmSync(tmpDir, { recursive: true, force: true });
});

// ---- runTask — basic execution ----

describe('runTask — basic execution', () => {
  it('calls startSession with isScheduledTask: true and logs a success run', async () => {
    const chat = makeChat('test-chat');
    const task = seedTask();

    const deps = makeSchedulerDeps({
      chats: vi.fn(() => ({ 'test@g.us': chat })),
    });

    await runTask(task, deps);

    // startSession must have been called
    expect(opencodeManager.startSession).toHaveBeenCalledTimes(1);
    const callArg = (opencodeManager.startSession as ReturnType<typeof vi.fn>).mock.calls[0][1];
    expect(callArg.isScheduledTask).toBe(true);
    expect(callArg.prompt).toBe('do something');

    // Store should reflect success (updateTaskAfterRun sets last_result)
    const updated = getTaskById('task-1');
    expect(updated).toBeDefined();
    expect(updated!.last_result).toContain('Task output');
  });
});

// ---- runTask — chat not found ----

describe('runTask — chat not found', () => {
  it('logs an error task run and does not call startSession', async () => {
    const task = seedTask({ chatFolder: 'nonexistent-folder' });

    // No chat registered that has folder === 'nonexistent-folder'
    const deps = makeSchedulerDeps({
      chats: vi.fn(() => ({})),
    });

    await runTask(task, deps);

    // startSession must NOT have been called
    expect(opencodeManager.startSession).not.toHaveBeenCalled();

    const task1 = getTaskById('task-1');
    expect(task1).toBeDefined();
    // last_result is null because updateTaskAfterRun was never called
    expect(task1!.last_result).toBeNull();
  });
});

// ---- runTask — context_mode=group selects existing sessionId ----

describe('runTask — context_mode=group', () => {
  it('passes the stored sessionId to startSession when context_mode is group', async () => {
    const chat = makeChat('test-chat');
    const task = seedTask({ context_mode: 'group' });

    const deps = makeSchedulerDeps({
      chats: vi.fn(() => ({ 'test@g.us': chat })),
      getSessions: vi.fn(() => ({ 'test-chat': 'existing-session-id' })),
    });

    await runTask(task, deps);

    const callArg = (opencodeManager.startSession as ReturnType<typeof vi.fn>).mock.calls[0][1];
    expect(callArg.sessionId).toBe('existing-session-id');
  });
});

// ---- runTask — context_mode=isolated uses fresh session ----

describe('runTask — context_mode=isolated', () => {
  it('passes undefined sessionId to startSession when context_mode is isolated', async () => {
    const chat = makeChat('test-chat');
    const task = seedTask({ context_mode: 'isolated' });

    const deps = makeSchedulerDeps({
      chats: vi.fn(() => ({ 'test@g.us': chat })),
      // Even if a session exists in the map, isolated mode must ignore it
      getSessions: vi.fn(() => ({ 'test-chat': 'should-not-be-used' })),
    });

    await runTask(task, deps);

    const callArg = (opencodeManager.startSession as ReturnType<typeof vi.fn>).mock.calls[0][1];
    expect(callArg.sessionId).toBeUndefined();
  });
});

// ---- runTask — cron next-run advances correctly ----

describe('runTask — cron next-run', () => {
  it('advances next_run to the next cron occurrence after running', async () => {
    const chat = makeChat('test-chat');

    // Every minute cron
    const task = seedTask({
      schedule_type: 'cron',
      schedule_value: '* * * * *',
      next_run: PAST_ISO,
    });

    const deps = makeSchedulerDeps({
      chats: vi.fn(() => ({ 'test@g.us': chat })),
    });

    const before = Date.now();
    await runTask(task, deps);

    const updated = getTaskById('task-1');
    expect(updated).toBeDefined();
    // next_run must be in the future (cron advanced past the old value)
    const nextRunMs = new Date(updated!.next_run!).getTime();
    expect(nextRunMs).toBeGreaterThan(before);
  });
});

// ---- runTask — interval drift-prevention ----

describe('runTask — interval drift-prevention', () => {
  it('computes next_run from next_run anchor, not from Date.now()', async () => {
    const chat = makeChat('test-chat');

    // Interval of 60 seconds (60000 ms)
    const intervalMs = 60000;
    // Anchor: a known past next_run time — 30 seconds ago
    const anchorMs = Date.now() - 30000;
    const anchorISO = new Date(anchorMs).toISOString();

    const task = seedTask({
      schedule_type: 'interval',
      schedule_value: String(intervalMs),
      next_run: anchorISO,
    });

    const deps = makeSchedulerDeps({
      chats: vi.fn(() => ({ 'test@g.us': chat })),
    });

    await runTask(task, deps);

    const updated = getTaskById('task-1');
    expect(updated).toBeDefined();

    const expectedNextRun = anchorMs + intervalMs;
    const actualNextRun = new Date(updated!.next_run!).getTime();

    // Must be anchored from the scheduled time: within a 2-second window around expectedNextRun
    expect(actualNextRun).toBeGreaterThanOrEqual(expectedNextRun - 2000);
    expect(actualNextRun).toBeLessThanOrEqual(expectedNextRun + 2000);

    // Critically: it must NOT be anchored from Date.now().
    const naiveNextRun = Date.now() + intervalMs;
    expect(actualNextRun).toBeLessThan(naiveNextRun);
  });
});

// ---- startSchedulerLoop — deduplication skips paused tasks ----

describe('startSchedulerLoop — deduplication skips paused tasks', () => {
  it('does not execute a task that is paused between getDueTasks and re-check', async () => {
    // Create a task that is due now
    seedTask({
      id: 'dup-task',
      chatFolder: 'test-chat',
      next_run: PAST_ISO,
      status: 'active',
    });

    // Pause it so the re-check in startSchedulerLoop finds it paused
    updateTask('dup-task', { status: 'paused' });

    const chat = makeChat('test-chat');
    const enqueueSpy = vi.fn();

    const fakeQueue = {
      enqueueTask: enqueueSpy,
      registerSession: vi.fn(),
    } as unknown as SessionQueue;

    const deps = makeSchedulerDeps({
      chats: vi.fn(() => ({ 'test@g.us': chat })),
      queue: fakeQueue,
    });

    // Run one iteration of the scheduler
    startSchedulerLoop(deps);

    // Allow the synchronous first iteration to complete (it's async internally)
    await new Promise<void>((resolve) => setTimeout(resolve, 50));

    expect(enqueueSpy).not.toHaveBeenCalled();
  });
});

// ---- startSchedulerLoop — processes due tasks ----

describe('startSchedulerLoop — processes due tasks', () => {
  it('enqueues an active due task on the first loop iteration', async () => {
    // Seed an active, past-due task
    seedTask({
      id: 'due-task',
      chatFolder: 'test-chat',
      next_run: PAST_ISO,
      status: 'active',
    });

    const enqueueSpy = vi.fn();
    const fakeQueue = {
      enqueueTask: enqueueSpy,
      registerSession: vi.fn(),
    } as unknown as SessionQueue;

    const deps = makeSchedulerDeps({
      chats: vi.fn(() => ({})),
      queue: fakeQueue,
    });

    startSchedulerLoop(deps);

    // Give the async loop iteration time to run
    await new Promise<void>((resolve) => setTimeout(resolve, 50));

    expect(enqueueSpy).toHaveBeenCalledTimes(1);
    expect(enqueueSpy.mock.calls[0][1]).toBe('due-task');
  });
});
