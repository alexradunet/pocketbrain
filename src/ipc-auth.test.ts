import { describe, it, expect, beforeEach } from 'bun:test';

import {
  _initTestDatabase,
  createTask,
  getAllTasks,
  getTaskById,
} from './db.js';
import { processTaskIpc, IpcDeps } from './ipc.js';
import { RegisteredGroup } from './types.js';

// Two registered 1-on-1 chats for authorization tests
const CHAT_A: RegisteredGroup = {
  name: 'Chat A',
  folder: 'chat-a',
  added_at: '2024-01-01T00:00:00.000Z',
};

const CHAT_B: RegisteredGroup = {
  name: 'Chat B',
  folder: 'chat-b',
  added_at: '2024-01-01T00:00:00.000Z',
};

let groups: Record<string, RegisteredGroup>;
let deps: IpcDeps;

beforeEach(() => {
  _initTestDatabase();

  groups = {
    'chat-a@s.whatsapp.net': CHAT_A,
    'chat-b@s.whatsapp.net': CHAT_B,
  };

  deps = {
    sendMessage: async () => {},
    registeredGroups: () => groups,
  };
});

// --- schedule_task authorization ---

describe('schedule_task authorization', () => {
  it('chat can schedule for itself', async () => {
    await processTaskIpc(
      {
        type: 'schedule_task',
        prompt: 'self task',
        schedule_type: 'once',
        schedule_value: '2030-06-01T00:00:00.000Z',
        targetJid: 'chat-a@s.whatsapp.net',
      },
      'chat-a',
      deps,
    );

    const allTasks = getAllTasks();
    expect(allTasks.length).toBe(1);
    expect(allTasks[0].group_folder).toBe('chat-a');
  });

  it('chat cannot schedule for another chat', async () => {
    await processTaskIpc(
      {
        type: 'schedule_task',
        prompt: 'unauthorized',
        schedule_type: 'once',
        schedule_value: '2030-06-01T00:00:00.000Z',
        targetJid: 'chat-b@s.whatsapp.net',
      },
      'chat-a',
      deps,
    );

    const allTasks = getAllTasks();
    expect(allTasks.length).toBe(0);
  });

  it('rejects schedule_task for unregistered target JID', async () => {
    await processTaskIpc(
      {
        type: 'schedule_task',
        prompt: 'no target',
        schedule_type: 'once',
        schedule_value: '2030-06-01T00:00:00.000Z',
        targetJid: 'unknown@s.whatsapp.net',
      },
      'chat-a',
      deps,
    );

    const allTasks = getAllTasks();
    expect(allTasks.length).toBe(0);
  });
});

// --- pause_task authorization ---

describe('pause_task authorization', () => {
  beforeEach(() => {
    createTask({
      id: 'task-a',
      group_folder: 'chat-a',
      chat_jid: 'chat-a@s.whatsapp.net',
      prompt: 'task for chat-a',
      schedule_type: 'once',
      schedule_value: '2030-06-01T00:00:00.000Z',
      context_mode: 'isolated',
      next_run: '2030-06-01T00:00:00.000Z',
      status: 'active',
      created_at: '2024-01-01T00:00:00.000Z',
    });
    createTask({
      id: 'task-b',
      group_folder: 'chat-b',
      chat_jid: 'chat-b@s.whatsapp.net',
      prompt: 'task for chat-b',
      schedule_type: 'once',
      schedule_value: '2030-06-01T00:00:00.000Z',
      context_mode: 'isolated',
      next_run: '2030-06-01T00:00:00.000Z',
      status: 'active',
      created_at: '2024-01-01T00:00:00.000Z',
    });
  });

  it('chat can pause its own task', async () => {
    await processTaskIpc({ type: 'pause_task', taskId: 'task-a' }, 'chat-a', deps);
    expect(getTaskById('task-a')!.status).toBe('paused');
  });

  it('chat cannot pause another chat\'s task', async () => {
    await processTaskIpc({ type: 'pause_task', taskId: 'task-b' }, 'chat-a', deps);
    expect(getTaskById('task-b')!.status).toBe('active');
  });
});

// --- resume_task authorization ---

describe('resume_task authorization', () => {
  beforeEach(() => {
    createTask({
      id: 'task-paused-a',
      group_folder: 'chat-a',
      chat_jid: 'chat-a@s.whatsapp.net',
      prompt: 'paused task',
      schedule_type: 'once',
      schedule_value: '2030-06-01T00:00:00.000Z',
      context_mode: 'isolated',
      next_run: '2030-06-01T00:00:00.000Z',
      status: 'paused',
      created_at: '2024-01-01T00:00:00.000Z',
    });
    createTask({
      id: 'task-paused-b',
      group_folder: 'chat-b',
      chat_jid: 'chat-b@s.whatsapp.net',
      prompt: 'paused task b',
      schedule_type: 'once',
      schedule_value: '2030-06-01T00:00:00.000Z',
      context_mode: 'isolated',
      next_run: '2030-06-01T00:00:00.000Z',
      status: 'paused',
      created_at: '2024-01-01T00:00:00.000Z',
    });
  });

  it('chat can resume its own task', async () => {
    await processTaskIpc({ type: 'resume_task', taskId: 'task-paused-a' }, 'chat-a', deps);
    expect(getTaskById('task-paused-a')!.status).toBe('active');
  });

  it('chat cannot resume another chat\'s task', async () => {
    await processTaskIpc({ type: 'resume_task', taskId: 'task-paused-b' }, 'chat-a', deps);
    expect(getTaskById('task-paused-b')!.status).toBe('paused');
  });
});

// --- cancel_task authorization ---

describe('cancel_task authorization', () => {
  it('chat can cancel its own task', async () => {
    createTask({
      id: 'task-own',
      group_folder: 'chat-a',
      chat_jid: 'chat-a@s.whatsapp.net',
      prompt: 'my task',
      schedule_type: 'once',
      schedule_value: '2030-06-01T00:00:00.000Z',
      context_mode: 'isolated',
      next_run: null,
      status: 'active',
      created_at: '2024-01-01T00:00:00.000Z',
    });

    await processTaskIpc({ type: 'cancel_task', taskId: 'task-own' }, 'chat-a', deps);
    expect(getTaskById('task-own')).toBeUndefined();
  });

  it('chat cannot cancel another chat\'s task', async () => {
    createTask({
      id: 'task-foreign',
      group_folder: 'chat-b',
      chat_jid: 'chat-b@s.whatsapp.net',
      prompt: 'not yours',
      schedule_type: 'once',
      schedule_value: '2030-06-01T00:00:00.000Z',
      context_mode: 'isolated',
      next_run: null,
      status: 'active',
      created_at: '2024-01-01T00:00:00.000Z',
    });

    await processTaskIpc({ type: 'cancel_task', taskId: 'task-foreign' }, 'chat-a', deps);
    expect(getTaskById('task-foreign')).toBeDefined();
  });
});

// --- IPC message authorization ---
// Tests the authorization pattern from startIpcWatcher (ipc.ts).
// The logic: targetGroup.folder === sourceGroup

describe('IPC message authorization', () => {
  // Replicate the exact check from the IPC watcher
  function isMessageAuthorized(
    sourceGroup: string,
    targetChatJid: string,
    registeredGroups: Record<string, RegisteredGroup>,
  ): boolean {
    const targetGroup = registeredGroups[targetChatJid];
    return !!targetGroup && targetGroup.folder === sourceGroup;
  }

  it('chat can send to its own JID', () => {
    expect(isMessageAuthorized('chat-a', 'chat-a@s.whatsapp.net', groups)).toBe(true);
    expect(isMessageAuthorized('chat-b', 'chat-b@s.whatsapp.net', groups)).toBe(true);
  });

  it('chat cannot send to another chat\'s JID', () => {
    expect(isMessageAuthorized('chat-a', 'chat-b@s.whatsapp.net', groups)).toBe(false);
  });

  it('chat cannot send to unregistered JID', () => {
    expect(isMessageAuthorized('chat-a', 'unknown@s.whatsapp.net', groups)).toBe(false);
  });
});

// --- schedule_task with cron and interval types ---

describe('schedule_task schedule types', () => {
  it('creates task with cron schedule and computes next_run', async () => {
    await processTaskIpc(
      {
        type: 'schedule_task',
        prompt: 'cron task',
        schedule_type: 'cron',
        schedule_value: '0 9 * * *', // every day at 9am
        targetJid: 'chat-a@s.whatsapp.net',
      },
      'chat-a',
      deps,
    );

    const tasks = getAllTasks();
    expect(tasks).toHaveLength(1);
    expect(tasks[0].schedule_type).toBe('cron');
    expect(tasks[0].next_run).toBeTruthy();
    expect(new Date(tasks[0].next_run!).getTime()).toBeGreaterThan(Date.now() - 60000);
  });

  it('rejects invalid cron expression', async () => {
    await processTaskIpc(
      {
        type: 'schedule_task',
        prompt: 'bad cron',
        schedule_type: 'cron',
        schedule_value: 'not a cron',
        targetJid: 'chat-a@s.whatsapp.net',
      },
      'chat-a',
      deps,
    );

    expect(getAllTasks()).toHaveLength(0);
  });

  it('creates task with interval schedule', async () => {
    const before = Date.now();

    await processTaskIpc(
      {
        type: 'schedule_task',
        prompt: 'interval task',
        schedule_type: 'interval',
        schedule_value: '3600000', // 1 hour
        targetJid: 'chat-a@s.whatsapp.net',
      },
      'chat-a',
      deps,
    );

    const tasks = getAllTasks();
    expect(tasks).toHaveLength(1);
    expect(tasks[0].schedule_type).toBe('interval');
    const nextRun = new Date(tasks[0].next_run!).getTime();
    expect(nextRun).toBeGreaterThanOrEqual(before + 3600000 - 1000);
    expect(nextRun).toBeLessThanOrEqual(Date.now() + 3600000 + 1000);
  });

  it('rejects invalid interval (non-numeric)', async () => {
    await processTaskIpc(
      {
        type: 'schedule_task',
        prompt: 'bad interval',
        schedule_type: 'interval',
        schedule_value: 'abc',
        targetJid: 'chat-a@s.whatsapp.net',
      },
      'chat-a',
      deps,
    );

    expect(getAllTasks()).toHaveLength(0);
  });

  it('rejects invalid interval (zero)', async () => {
    await processTaskIpc(
      {
        type: 'schedule_task',
        prompt: 'zero interval',
        schedule_type: 'interval',
        schedule_value: '0',
        targetJid: 'chat-a@s.whatsapp.net',
      },
      'chat-a',
      deps,
    );

    expect(getAllTasks()).toHaveLength(0);
  });

  it('rejects once task with timestamp in the past', async () => {
    await processTaskIpc(
      {
        type: 'schedule_task',
        prompt: 'past task',
        schedule_type: 'once',
        schedule_value: '2020-01-01T00:00:00.000Z',
        targetJid: 'chat-a@s.whatsapp.net',
      },
      'chat-a',
      deps,
    );

    expect(getAllTasks()).toHaveLength(0);
  });

  it('rejects invalid once timestamp', async () => {
    await processTaskIpc(
      {
        type: 'schedule_task',
        prompt: 'bad once',
        schedule_type: 'once',
        schedule_value: 'not-a-date',
        targetJid: 'chat-a@s.whatsapp.net',
      },
      'chat-a',
      deps,
    );

    expect(getAllTasks()).toHaveLength(0);
  });
});

// --- context_mode defaulting ---

describe('schedule_task context_mode', () => {
  it('accepts context_mode=group', async () => {
    await processTaskIpc(
      {
        type: 'schedule_task',
        prompt: 'group context',
        schedule_type: 'once',
        schedule_value: '2030-06-01T00:00:00.000Z',
        context_mode: 'group',
        targetJid: 'chat-a@s.whatsapp.net',
      },
      'chat-a',
      deps,
    );

    const tasks = getAllTasks();
    expect(tasks[0].context_mode).toBe('group');
  });

  it('accepts context_mode=isolated', async () => {
    await processTaskIpc(
      {
        type: 'schedule_task',
        prompt: 'isolated context',
        schedule_type: 'once',
        schedule_value: '2030-06-01T00:00:00.000Z',
        context_mode: 'isolated',
        targetJid: 'chat-a@s.whatsapp.net',
      },
      'chat-a',
      deps,
    );

    const tasks = getAllTasks();
    expect(tasks[0].context_mode).toBe('isolated');
  });

  it('defaults invalid context_mode to isolated', async () => {
    await processTaskIpc(
      {
        type: 'schedule_task',
        prompt: 'bad context',
        schedule_type: 'once',
        schedule_value: '2030-06-01T00:00:00.000Z',
        context_mode: 'bogus' as any,
        targetJid: 'chat-a@s.whatsapp.net',
      },
      'chat-a',
      deps,
    );

    const tasks = getAllTasks();
    expect(tasks[0].context_mode).toBe('isolated');
  });

  it('defaults missing context_mode to isolated', async () => {
    await processTaskIpc(
      {
        type: 'schedule_task',
        prompt: 'no context mode',
        schedule_type: 'once',
        schedule_value: '2030-06-01T00:00:00.000Z',
        targetJid: 'chat-a@s.whatsapp.net',
      },
      'chat-a',
      deps,
    );

    const tasks = getAllTasks();
    expect(tasks[0].context_mode).toBe('isolated');
  });
});

// --- resume_task recomputes next_run ---

describe('resume_task recomputes next_run', () => {
  it('recomputes next_run for paused cron task', async () => {
    createTask({
      id: 'cron-task',
      group_folder: 'chat-a',
      chat_jid: 'chat-a@s.whatsapp.net',
      prompt: 'test',
      schedule_type: 'cron',
      schedule_value: '* * * * *',
      context_mode: 'isolated',
      next_run: '2020-01-01T00:00:00.000Z',
      status: 'paused',
      created_at: '2020-01-01T00:00:00.000Z',
    });

    await processTaskIpc({ type: 'resume_task', taskId: 'cron-task' }, 'chat-a', deps);

    const task = getTaskById('cron-task');
    expect(task!.status).toBe('active');
    expect(new Date(task!.next_run!).getTime()).toBeGreaterThan(Date.now() - 5000);
  });

  it('recomputes next_run for paused interval task', async () => {
    createTask({
      id: 'interval-task',
      group_folder: 'chat-a',
      chat_jid: 'chat-a@s.whatsapp.net',
      prompt: 'test',
      schedule_type: 'interval',
      schedule_value: '3600000',
      context_mode: 'isolated',
      next_run: '2020-01-01T00:00:00.000Z',
      status: 'paused',
      created_at: '2020-01-01T00:00:00.000Z',
    });

    const before = Date.now();
    await processTaskIpc({ type: 'resume_task', taskId: 'interval-task' }, 'chat-a', deps);

    const task = getTaskById('interval-task');
    expect(task!.status).toBe('active');
    const nextRunMs = new Date(task!.next_run!).getTime();
    expect(nextRunMs).toBeGreaterThanOrEqual(before + 3600000 - 1000);
  });

  it('leaves next_run unchanged for paused once task', async () => {
    createTask({
      id: 'once-task',
      group_folder: 'chat-a',
      chat_jid: 'chat-a@s.whatsapp.net',
      prompt: 'test',
      schedule_type: 'once',
      schedule_value: '2030-06-01T00:00:00.000Z',
      context_mode: 'isolated',
      next_run: '2030-06-01T00:00:00.000Z',
      status: 'paused',
      created_at: '2020-01-01T00:00:00.000Z',
    });

    await processTaskIpc({ type: 'resume_task', taskId: 'once-task' }, 'chat-a', deps);

    const task = getTaskById('once-task');
    expect(task!.status).toBe('active');
    expect(task!.next_run).toBe('2030-06-01T00:00:00.000Z');
  });
});

// --- unknown IPC task type ---

describe('unknown IPC task type', () => {
  it('does not throw and does not create any tasks when type is unrecognized', async () => {
    await expect(
      processTaskIpc({ type: 'unknown_future_type' }, 'chat-a', deps),
    ).resolves.toBeUndefined();

    expect(getAllTasks()).toHaveLength(0);
  });
});
