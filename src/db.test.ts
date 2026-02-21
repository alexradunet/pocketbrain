import { describe, it, expect, beforeEach } from 'bun:test';

import {
  _initTestDatabase,
  createTask,
  deleteTask,
  getAllChats,
  getAllRegisteredGroups,
  getDueTasks,
  getLastGroupSync,
  getMessagesSince,
  getNewMessages,
  getRegisteredGroup,
  getRouterState,
  getTaskById,
  setLastGroupSync,
  setRegisteredGroup,
  setRouterState,
  storeChatMetadata,
  storeMessage,
  updateTask,
  updateTaskAfterRun,
} from './db.js';

beforeEach(() => {
  _initTestDatabase();
});

// Helper to store a message using the normalized NewMessage interface
function store(overrides: {
  id: string;
  chat_jid: string;
  sender: string;
  sender_name: string;
  content: string;
  timestamp: string;
  is_from_me?: boolean;
}) {
  storeMessage({
    id: overrides.id,
    chat_jid: overrides.chat_jid,
    sender: overrides.sender,
    sender_name: overrides.sender_name,
    content: overrides.content,
    timestamp: overrides.timestamp,
    is_from_me: overrides.is_from_me ?? false,
  });
}

// --- storeMessage (NewMessage format) ---

describe('storeMessage', () => {
  it('stores a message and retrieves it', () => {
    storeChatMetadata('group@g.us', '2024-01-01T00:00:00.000Z');

    store({
      id: 'msg-1',
      chat_jid: 'group@g.us',
      sender: '123@s.whatsapp.net',
      sender_name: 'Alice',
      content: 'hello world',
      timestamp: '2024-01-01T00:00:01.000Z',
    });

    const messages = getMessagesSince('group@g.us', '2024-01-01T00:00:00.000Z');
    expect(messages).toHaveLength(1);
    expect(messages[0].id).toBe('msg-1');
    expect(messages[0].sender).toBe('123@s.whatsapp.net');
    expect(messages[0].sender_name).toBe('Alice');
    expect(messages[0].content).toBe('hello world');
  });

  it('stores empty content', () => {
    storeChatMetadata('group@g.us', '2024-01-01T00:00:00.000Z');

    store({
      id: 'msg-2',
      chat_jid: 'group@g.us',
      sender: '111@s.whatsapp.net',
      sender_name: 'Dave',
      content: '',
      timestamp: '2024-01-01T00:00:04.000Z',
    });

    const messages = getMessagesSince('group@g.us', '2024-01-01T00:00:00.000Z');
    expect(messages).toHaveLength(1);
    expect(messages[0].content).toBe('');
  });

  it('stores is_from_me flag', () => {
    storeChatMetadata('group@g.us', '2024-01-01T00:00:00.000Z');

    store({
      id: 'msg-3',
      chat_jid: 'group@g.us',
      sender: 'me@s.whatsapp.net',
      sender_name: 'Me',
      content: 'my message',
      timestamp: '2024-01-01T00:00:05.000Z',
      is_from_me: true,
    });

    // Message is stored (we can retrieve it — is_from_me doesn't affect retrieval)
    const messages = getMessagesSince('group@g.us', '2024-01-01T00:00:00.000Z');
    expect(messages).toHaveLength(1);
  });

  it('upserts on duplicate id+chat_jid', () => {
    storeChatMetadata('group@g.us', '2024-01-01T00:00:00.000Z');

    store({
      id: 'msg-dup',
      chat_jid: 'group@g.us',
      sender: '123@s.whatsapp.net',
      sender_name: 'Alice',
      content: 'original',
      timestamp: '2024-01-01T00:00:01.000Z',
    });

    store({
      id: 'msg-dup',
      chat_jid: 'group@g.us',
      sender: '123@s.whatsapp.net',
      sender_name: 'Alice',
      content: 'updated',
      timestamp: '2024-01-01T00:00:01.000Z',
    });

    const messages = getMessagesSince('group@g.us', '2024-01-01T00:00:00.000Z');
    expect(messages).toHaveLength(1);
    expect(messages[0].content).toBe('updated');
  });
});

// --- getMessagesSince ---

describe('getMessagesSince', () => {
  beforeEach(() => {
    storeChatMetadata('group@g.us', '2024-01-01T00:00:00.000Z');

    store({
      id: 'm1', chat_jid: 'group@g.us', sender: 'Alice@s.whatsapp.net',
      sender_name: 'Alice', content: 'first', timestamp: '2024-01-01T00:00:01.000Z',
    });
    store({
      id: 'm2', chat_jid: 'group@g.us', sender: 'Bob@s.whatsapp.net',
      sender_name: 'Bob', content: 'second', timestamp: '2024-01-01T00:00:02.000Z',
    });
    storeMessage({
      id: 'm3', chat_jid: 'group@g.us', sender: 'Bot@s.whatsapp.net',
      sender_name: 'Bot', content: 'bot reply', timestamp: '2024-01-01T00:00:03.000Z',
      is_bot_message: true,
    });
    store({
      id: 'm4', chat_jid: 'group@g.us', sender: 'Carol@s.whatsapp.net',
      sender_name: 'Carol', content: 'third', timestamp: '2024-01-01T00:00:04.000Z',
    });
  });

  it('returns messages after the given timestamp', () => {
    const msgs = getMessagesSince('group@g.us', '2024-01-01T00:00:02.000Z');
    // Should exclude m1, m2 (before/at timestamp), m3 (bot message)
    expect(msgs).toHaveLength(1);
    expect(msgs[0].content).toBe('third');
  });

  it('excludes bot messages via is_bot_message flag', () => {
    const msgs = getMessagesSince('group@g.us', '2024-01-01T00:00:00.000Z');
    const botMsgs = msgs.filter((m) => m.content === 'bot reply');
    expect(botMsgs).toHaveLength(0);
  });

  it('returns all non-bot messages when sinceTimestamp is empty', () => {
    const msgs = getMessagesSince('group@g.us', '');
    // 3 user messages (bot message excluded)
    expect(msgs).toHaveLength(3);
  });

});

// --- getNewMessages ---

describe('getNewMessages', () => {
  beforeEach(() => {
    storeChatMetadata('group1@g.us', '2024-01-01T00:00:00.000Z');
    storeChatMetadata('group2@g.us', '2024-01-01T00:00:00.000Z');

    store({
      id: 'a1', chat_jid: 'group1@g.us', sender: 'user@s.whatsapp.net',
      sender_name: 'User', content: 'g1 msg1', timestamp: '2024-01-01T00:00:01.000Z',
    });
    store({
      id: 'a2', chat_jid: 'group2@g.us', sender: 'user@s.whatsapp.net',
      sender_name: 'User', content: 'g2 msg1', timestamp: '2024-01-01T00:00:02.000Z',
    });
    storeMessage({
      id: 'a3', chat_jid: 'group1@g.us', sender: 'user@s.whatsapp.net',
      sender_name: 'User', content: 'bot reply', timestamp: '2024-01-01T00:00:03.000Z',
      is_bot_message: true,
    });
    store({
      id: 'a4', chat_jid: 'group1@g.us', sender: 'user@s.whatsapp.net',
      sender_name: 'User', content: 'g1 msg2', timestamp: '2024-01-01T00:00:04.000Z',
    });
  });

  it('returns new messages across multiple groups', () => {
    const { messages, newTimestamp } = getNewMessages(
      ['group1@g.us', 'group2@g.us'],
      '2024-01-01T00:00:00.000Z',
    );
    // Excludes bot message, returns 3 user messages
    expect(messages).toHaveLength(3);
    expect(newTimestamp).toBe('2024-01-01T00:00:04.000Z');
  });

  it('filters by timestamp', () => {
    const { messages } = getNewMessages(
      ['group1@g.us', 'group2@g.us'],
      '2024-01-01T00:00:02.000Z',
    );
    // Only g1 msg2 (after ts, not bot)
    expect(messages).toHaveLength(1);
    expect(messages[0].content).toBe('g1 msg2');
  });

  it('returns empty for no registered groups', () => {
    const { messages, newTimestamp } = getNewMessages([], '');
    expect(messages).toHaveLength(0);
    expect(newTimestamp).toBe('');
  });

  it('returned messages include is_from_me and is_bot_message fields', () => {
    const { messages } = getNewMessages(
      ['group1@g.us'],
      '2024-01-01T00:00:00.000Z',
    );
    // is_bot_message rows are filtered out; user messages must have boolean fields (not SQLite integers)
    expect(messages.length).toBeGreaterThan(0);
    for (const m of messages) {
      expect(typeof m.is_from_me).toBe('boolean');
      expect(typeof m.is_bot_message).toBe('boolean');
    }
  });
});

// --- storeChatMetadata ---

describe('storeChatMetadata', () => {
  it('stores chat with JID as default name', () => {
    storeChatMetadata('group@g.us', '2024-01-01T00:00:00.000Z');
    const chats = getAllChats();
    expect(chats).toHaveLength(1);
    expect(chats[0].jid).toBe('group@g.us');
    expect(chats[0].name).toBe('group@g.us');
  });

  it('stores chat with explicit name', () => {
    storeChatMetadata('group@g.us', '2024-01-01T00:00:00.000Z', 'My Group');
    const chats = getAllChats();
    expect(chats[0].name).toBe('My Group');
  });

  it('updates name on subsequent call with name', () => {
    storeChatMetadata('group@g.us', '2024-01-01T00:00:00.000Z');
    storeChatMetadata('group@g.us', '2024-01-01T00:00:01.000Z', 'Updated Name');
    const chats = getAllChats();
    expect(chats).toHaveLength(1);
    expect(chats[0].name).toBe('Updated Name');
  });

  it('preserves newer timestamp on conflict', () => {
    storeChatMetadata('group@g.us', '2024-01-01T00:00:05.000Z');
    storeChatMetadata('group@g.us', '2024-01-01T00:00:01.000Z');
    const chats = getAllChats();
    expect(chats[0].last_message_time).toBe('2024-01-01T00:00:05.000Z');
  });
});

// --- Task CRUD ---

describe('task CRUD', () => {
  it('creates and retrieves a task', () => {
    createTask({
      id: 'task-1',
      group_folder: 'main',
      chat_jid: 'group@g.us',
      prompt: 'do something',
      schedule_type: 'once',
      schedule_value: '2024-06-01T00:00:00.000Z',
      context_mode: 'isolated',
      next_run: '2024-06-01T00:00:00.000Z',
      status: 'active',
      created_at: '2024-01-01T00:00:00.000Z',
    });

    const task = getTaskById('task-1');
    expect(task).toBeDefined();
    expect(task!.prompt).toBe('do something');
    expect(task!.status).toBe('active');
  });

  it('updates task status', () => {
    createTask({
      id: 'task-2',
      group_folder: 'main',
      chat_jid: 'group@g.us',
      prompt: 'test',
      schedule_type: 'once',
      schedule_value: '2024-06-01T00:00:00.000Z',
      context_mode: 'isolated',
      next_run: null,
      status: 'active',
      created_at: '2024-01-01T00:00:00.000Z',
    });

    updateTask('task-2', { status: 'paused' });
    expect(getTaskById('task-2')!.status).toBe('paused');
  });

  it('enforces FK: storeMessage without parent chat throws', () => {
    // FK enforcement (PRAGMA foreign_keys = ON) must be active.
    // Storing a message with a chat_jid that has no row in chats should throw.
    expect(() => {
      storeMessage({
        id: 'orphan-msg',
        chat_jid: 'ghost@g.us',
        sender: 'x@s.whatsapp.net',
        sender_name: 'X',
        content: 'hello',
        timestamp: '2024-01-01T00:00:00.000Z',
        is_from_me: false,
      });
    }).toThrow();
  });

  it('deletes a task and its run logs', () => {
    createTask({
      id: 'task-3',
      group_folder: 'main',
      chat_jid: 'group@g.us',
      prompt: 'delete me',
      schedule_type: 'once',
      schedule_value: '2024-06-01T00:00:00.000Z',
      context_mode: 'isolated',
      next_run: null,
      status: 'active',
      created_at: '2024-01-01T00:00:00.000Z',
    });

    deleteTask('task-3');
    expect(getTaskById('task-3')).toBeUndefined();
  });
});

// --- getLastGroupSync / setLastGroupSync ---

describe('getLastGroupSync / setLastGroupSync', () => {
  it('returns null when not set', () => {
    expect(getLastGroupSync()).toBeNull();
  });

  it('returns a timestamp after setLastGroupSync is called', () => {
    const before = Date.now();
    setLastGroupSync();
    const result = getLastGroupSync();
    expect(result).not.toBeNull();
    // The stored timestamp should be a valid ISO string at or after 'before'
    expect(new Date(result!).getTime()).toBeGreaterThanOrEqual(before);
  });

  it('overwrites previous sync time on repeated calls', () => {
    setLastGroupSync();
    const first = getLastGroupSync();
    setLastGroupSync();
    const second = getLastGroupSync();
    // Both are valid; second is >= first
    expect(new Date(second!).getTime()).toBeGreaterThanOrEqual(new Date(first!).getTime());
  });
});

// --- getDueTasks ---

describe('getDueTasks', () => {
  it('returns active tasks whose next_run is in the past', () => {
    storeChatMetadata('group@g.us', '2024-01-01T00:00:00.000Z');
    createTask({
      id: 'due-task',
      group_folder: 'main',
      chat_jid: 'group@g.us',
      prompt: 'due now',
      schedule_type: 'once',
      schedule_value: '2020-01-01T00:00:00.000Z',
      context_mode: 'isolated',
      next_run: '2020-01-01T00:00:00.000Z', // past
      status: 'active',
      created_at: '2020-01-01T00:00:00.000Z',
    });

    const due = getDueTasks();
    expect(due).toHaveLength(1);
    expect(due[0].id).toBe('due-task');
  });

  it('does not return paused tasks', () => {
    storeChatMetadata('group@g.us', '2024-01-01T00:00:00.000Z');
    createTask({
      id: 'paused-task',
      group_folder: 'main',
      chat_jid: 'group@g.us',
      prompt: 'paused',
      schedule_type: 'once',
      schedule_value: '2020-01-01T00:00:00.000Z',
      context_mode: 'isolated',
      next_run: '2020-01-01T00:00:00.000Z', // past
      status: 'paused',
      created_at: '2020-01-01T00:00:00.000Z',
    });

    expect(getDueTasks()).toHaveLength(0);
  });

  it('does not return future-scheduled tasks', () => {
    storeChatMetadata('group@g.us', '2024-01-01T00:00:00.000Z');
    createTask({
      id: 'future-task',
      group_folder: 'main',
      chat_jid: 'group@g.us',
      prompt: 'future',
      schedule_type: 'once',
      schedule_value: '2099-01-01T00:00:00.000Z',
      context_mode: 'isolated',
      next_run: '2099-01-01T00:00:00.000Z', // far future
      status: 'active',
      created_at: '2020-01-01T00:00:00.000Z',
    });

    expect(getDueTasks()).toHaveLength(0);
  });
});

// --- updateTaskAfterRun ---

describe('updateTaskAfterRun', () => {
  it('updates last_run and last_result, advances next_run for interval tasks', () => {
    storeChatMetadata('group@g.us', '2024-01-01T00:00:00.000Z');
    createTask({
      id: 'interval-run',
      group_folder: 'main',
      chat_jid: 'group@g.us',
      prompt: 'recurring',
      schedule_type: 'interval',
      schedule_value: '3600000',
      context_mode: 'isolated',
      next_run: '2020-01-01T00:00:00.000Z',
      status: 'active',
      created_at: '2020-01-01T00:00:00.000Z',
    });

    const nextRunAfter = new Date(Date.now() + 3600000).toISOString();
    const before = Date.now();
    updateTaskAfterRun('interval-run', nextRunAfter, 'ok');

    const task = getTaskById('interval-run');
    expect(task).toBeDefined();
    expect(task!.last_result).toBe('ok');
    expect(new Date(task!.last_run!).getTime()).toBeGreaterThanOrEqual(before);
    expect(task!.next_run).toBe(nextRunAfter);
    // Status should still be active (nextRun is non-null)
    expect(task!.status).toBe('active');
  });

  it('sets status to completed when nextRun is null', () => {
    storeChatMetadata('group@g.us', '2024-01-01T00:00:00.000Z');
    createTask({
      id: 'once-run',
      group_folder: 'main',
      chat_jid: 'group@g.us',
      prompt: 'one shot',
      schedule_type: 'once',
      schedule_value: '2020-01-01T00:00:00.000Z',
      context_mode: 'isolated',
      next_run: '2020-01-01T00:00:00.000Z',
      status: 'active',
      created_at: '2020-01-01T00:00:00.000Z',
    });

    updateTaskAfterRun('once-run', null, 'done');

    const task = getTaskById('once-run');
    expect(task!.status).toBe('completed');
    expect(task!.last_result).toBe('done');
    expect(task!.next_run).toBeNull();
  });
});

// --- getRouterState / setRouterState ---

describe('getRouterState / setRouterState', () => {
  it('returns undefined for a key that has not been set', () => {
    expect(getRouterState('nonexistent-key')).toBeUndefined();
  });

  it('returns the value that was set', () => {
    setRouterState('my-key', 'my-value');
    expect(getRouterState('my-key')).toBe('my-value');
  });

  it('overwrites the previous value on a second set', () => {
    setRouterState('overwrite-key', 'first');
    setRouterState('overwrite-key', 'second');
    expect(getRouterState('overwrite-key')).toBe('second');
  });

  it('stores independent values for different keys', () => {
    setRouterState('key-a', 'value-a');
    setRouterState('key-b', 'value-b');
    expect(getRouterState('key-a')).toBe('value-a');
    expect(getRouterState('key-b')).toBe('value-b');
  });
});

// --- getRegisteredGroup ---

describe('getRegisteredGroup', () => {
  it('returns undefined for unknown jid', () => {
    expect(getRegisteredGroup('missing@g.us')).toBeUndefined();
  });

  it('round-trips a registered group', () => {
    setRegisteredGroup('g1@g.us', {
      name: 'G1',
      folder: 'g1',
      added_at: '2024-01-01T00:00:00.000Z',
    });

    const group = getRegisteredGroup('g1@g.us');
    expect(group).toBeDefined();
    expect(group!.name).toBe('G1');
    expect(group!.folder).toBe('g1');
  });
});
