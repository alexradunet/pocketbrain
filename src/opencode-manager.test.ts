import { afterEach, describe, expect, it } from 'bun:test';

import {
  abortSession,
  startSession,
  _setTestOpencodeInstance,
} from './opencode-manager.js';
import type { RegisteredGroup } from './types.js';

const MOCK_GROUP: RegisteredGroup = {
  name: 'Test Group',
  folder: 'test-group',
  trigger: '@bot',
  added_at: '2024-01-01T00:00:00.000Z',
};

const BASE_INPUT = {
  prompt: 'hello',
  groupFolder: 'test-group',
  chatJid: 'test@g.us',
  isMain: false,
};

/** Build a minimal mock opencode instance. Pass overrides for specific session methods. */
function makeMockInstance(sessionOverrides: Record<string, unknown> = {}) {
  return {
    client: {
      session: {
        get: async () => ({ data: { id: 'existing-session' } }),
        create: async () => ({ data: { id: 'new-session-id' } }),
        abort: async () => ({}),
        promptAsync: async () => ({}),
        message: async () => ({ data: { info: null, parts: [] } }),
        ...sessionOverrides,
      },
      event: {
        // Empty async generator — stream ends immediately so runPrompt doesn't hang
        subscribe: async () => ({
          stream: (async function* () {})(),
        }),
      },
    },
    server: { close: () => {} },
  };
}

/** onOutput that aborts the session after the session-update marker fires. */
function makeAutoAbortOnOutput() {
  return async (output: { newSessionId?: string; result: unknown }) => {
    if (output.newSessionId && !output.result) {
      abortSession('test-group');
    }
  };
}

afterEach(() => {
  // Clean up mock instance and any lingering session state
  _setTestOpencodeInstance(null);
});

describe('startSession — session.create null data', () => {
  it('returns a descriptive error when session.create returns no session ID', async () => {
    _setTestOpencodeInstance(
      makeMockInstance({
        create: async () => ({ data: null }),
      }),
    );

    const result = await startSession(MOCK_GROUP, BASE_INPUT, async () => {});

    expect(result.status).toBe('error');
    expect(result.error).toMatch(/no session ID/i);
  });
});

describe('startSession — stale session recovery', () => {
  it('falls back to session.create when session.get fails', async () => {
    let createCallCount = 0;

    _setTestOpencodeInstance(
      makeMockInstance({
        get: async () => {
          throw new Error('session not found');
        },
        create: async () => {
          createCallCount++;
          return { data: { id: 'recovery-session' } };
        },
      }),
    );

    const result = await startSession(
      MOCK_GROUP,
      { ...BASE_INPUT, sessionId: 'stale-id' },
      makeAutoAbortOnOutput(),
    );

    expect(result.status).toBe('success');
    expect(result.newSessionId).toBe('recovery-session');
    expect(createCallCount).toBe(1);
  });

  it('does NOT fall back when session.get succeeds', async () => {
    let createCallCount = 0;

    _setTestOpencodeInstance(
      makeMockInstance({
        get: async () => ({ data: { id: 'existing-session' } }),
        create: async () => {
          createCallCount++;
          return { data: { id: 'should-not-be-used' } };
        },
      }),
    );

    const result = await startSession(
      MOCK_GROUP,
      { ...BASE_INPUT, sessionId: 'existing-session' },
      makeAutoAbortOnOutput(),
    );

    expect(result.status).toBe('success');
    expect(result.newSessionId).toBe('existing-session');
    expect(createCallCount).toBe(0);
  });

  it('returns error when both session.get and recovery session.create fail', async () => {
    _setTestOpencodeInstance(
      makeMockInstance({
        get: async () => {
          throw new Error('session not found');
        },
        create: async () => {
          throw new Error('create failed');
        },
      }),
    );

    const result = await startSession(
      MOCK_GROUP,
      { ...BASE_INPUT, sessionId: 'stale-id' },
      async () => {},
    );

    expect(result.status).toBe('error');
    expect(result.error).toMatch(/create failed/i);
  });
});
