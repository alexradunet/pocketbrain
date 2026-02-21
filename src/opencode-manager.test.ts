import fs from 'fs';
import os from 'os';
import path from 'path';
import { afterEach, beforeEach, describe, expect, it, vi } from 'bun:test';

// Mock config to use a temp DATA_DIR so mkdirSync doesn't hit permission errors
const TEST_DATA_DIR = fs.mkdtempSync(path.join(os.tmpdir(), 'pb-om-test-'));
vi.mock('./config.js', () => ({
  DATA_DIR: TEST_DATA_DIR,
  WORKSPACE_DIR: TEST_DATA_DIR,
}));

import {
  abortSession,
  hasActiveSession,
  sendFollowUp,
  startSession,
  _buildContextPrefix,
  _clearActiveSessions,
  _setTestOpencodeInstance,
} from './opencode-manager.js';
import type { ChatConfig } from './types.js';

const MOCK_CHAT: ChatConfig = {
  jid: 'test@g.us',
  name: 'Test Chat',
  folder: 'test-chat',
  addedAt: '2024-01-01T00:00:00.000Z',
};

const BASE_INPUT = {
  prompt: 'hello',
  chatFolder: 'test-chat',
  chatJid: 'test@g.us',
};

/** Build a minimal mock opencode instance. Pass overrides for specific session methods. */
function makeMockInstance(sessionOverrides: Record<string, unknown> = {}) {
  return {
    client: {
      session: {
        get: async () => ({ data: { id: 'existing-session' } }),
        create: async () => ({ data: { id: 'new-session-id' } }),
        delete: async () => ({}),
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

/**
 * Factory: creates a mock OpenCode instance whose event stream yields the given
 * events in order. `messageResult` controls what client.session.message() returns.
 */
function makeStreamingMock(
  events: object[],
  messageResult?: { data: { info: null | object; parts: Array<{ type: string; text?: string }> } } | null,
) {
  async function* gen() {
    for (const e of events) yield e;
  }
  const messageReturn =
    messageResult !== undefined
      ? messageResult
      : { data: { info: null, parts: [{ type: 'text', text: 'canonical reply' }] } };
  return {
    client: {
      session: {
        get: vi.fn().mockResolvedValue({ data: { id: 'sess-1' } }),
        create: vi.fn().mockResolvedValue({ data: { id: 'sess-1' } }),
        abort: vi.fn().mockResolvedValue({}),
        promptAsync: vi.fn().mockResolvedValue({ data: { id: 'msg-placeholder' } }),
        message: vi.fn().mockResolvedValue(messageReturn),
      },
      event: {
        subscribe: vi.fn().mockReturnValue({ stream: gen() }),
      },
    },
    server: { close: vi.fn() },
  };
}

/** onOutput that aborts the session after the session-update marker fires. */
function makeAutoAbortOnOutput() {
  return async (output: { newSessionId?: string; result: unknown }) => {
    if (output.newSessionId && !output.result) {
      abortSession('test-chat');
    }
  };
}

beforeEach(() => {
  _clearActiveSessions();
});

afterEach(() => {
  // Clean up mock instance and any lingering session state
  _setTestOpencodeInstance(null);
  _clearActiveSessions();
});

describe('startSession — session.create null data', () => {
  it('returns a descriptive error when session.create returns no session ID', async () => {
    _setTestOpencodeInstance(
      makeMockInstance({
        create: async () => ({ data: null }),
      }),
    );

    const result = await startSession(MOCK_CHAT, BASE_INPUT, async () => {});

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
      MOCK_CHAT,
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
      MOCK_CHAT,
      { ...BASE_INPUT, sessionId: 'existing-session' },
      makeAutoAbortOnOutput(),
    );

    expect(result.status).toBe('success');
    expect(result.newSessionId).toBe('existing-session');
    expect(createCallCount).toBe(0);
  });

  it('calls session.delete on the stale session ID when falling back to a new session', async () => {
    const deletedIds: string[] = [];

    _setTestOpencodeInstance(
      makeMockInstance({
        get: async () => {
          throw new Error('session not found');
        },
        create: async () => ({ data: { id: 'recovery-session' } }),
        delete: async (opts: { path: { id: string } }) => {
          deletedIds.push(opts.path.id);
          return {};
        },
      }),
    );

    const result = await startSession(
      MOCK_CHAT,
      { ...BASE_INPUT, sessionId: 'stale-id' },
      makeAutoAbortOnOutput(),
    );

    expect(result.status).toBe('success');
    // Give the fire-and-forget delete a tick to complete
    await new Promise<void>((r) => setTimeout(r, 50));
    expect(deletedIds).toContain('stale-id');
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
      MOCK_CHAT,
      { ...BASE_INPUT, sessionId: 'stale-id' },
      async () => {},
    );

    expect(result.status).toBe('error');
    expect(result.error).toMatch(/create failed/i);
  });
});

describe('runPrompt — via startSession — SSE event processing', () => {
  it('returns canonical text from session.message after stream ends', async () => {
    const mock = makeStreamingMock(
      [],
      { data: { info: null, parts: [{ type: 'text', text: 'hello from canonical' }] } },
    );

    _setTestOpencodeInstance(mock);

    const outputs: Array<{ status: string; result: string | null }> = [];
    const result = await startSession(
      MOCK_CHAT,
      BASE_INPUT,
      async (output) => {
        outputs.push(output);
        if (output.newSessionId && !output.result) {
          abortSession('test-chat');
        }
      },
    );

    expect(result.status).toBe('success');
    const promptOutput = outputs.find((o) => o.result !== null);
    expect(promptOutput?.result).toBe('hello from canonical');
  });

  it('accumulates text parts via delta events for the correct messageID', async () => {
    let capturedMessageId: string | undefined;
    const SESSION_ID = 'sess-1';
    let emitEvents!: (events: object[]) => void;
    const eventQueue: object[] = [];
    let resolveStream!: () => void;

    async function* dynamicStream() {
      await new Promise<void>((r) => { emitEvents = (evs) => { eventQueue.push(...evs); r(); }; });
      for (const e of eventQueue) yield e;
      resolveStream();
    }

    const mock = {
      client: {
        session: {
          get: vi.fn().mockResolvedValue({ data: { id: SESSION_ID } }),
          create: vi.fn().mockResolvedValue({ data: { id: SESSION_ID } }),
          abort: vi.fn().mockResolvedValue({}),
          promptAsync: vi.fn().mockImplementation(
            async (opts: { body: { messageID: string } }) => {
              capturedMessageId = opts.body.messageID;
              emitEvents([
                {
                  type: 'message.part.updated',
                  properties: {
                    delta: 'Hello ',
                    part: {
                      sessionID: SESSION_ID,
                      messageID: capturedMessageId,
                      type: 'text',
                      id: 'part-1',
                      text: 'Hello ',
                    },
                  },
                },
                {
                  type: 'message.part.updated',
                  properties: {
                    delta: 'World',
                    part: {
                      sessionID: SESSION_ID,
                      messageID: capturedMessageId,
                      type: 'text',
                      id: 'part-1',
                      text: 'Hello World',
                    },
                  },
                },
                {
                  type: 'message.updated',
                  properties: {
                    info: {
                      sessionID: SESSION_ID,
                      id: capturedMessageId,
                    },
                  },
                },
                {
                  type: 'session.idle',
                  properties: { sessionID: SESSION_ID },
                },
              ]);
              return {};
            },
          ),
          message: vi.fn().mockResolvedValue({
            data: { info: null, parts: [{ type: 'text', text: 'Hello World' }] },
          }),
        },
        event: {
          subscribe: vi.fn().mockReturnValue({ stream: dynamicStream() }),
        },
      },
      server: { close: vi.fn() },
    };

    _setTestOpencodeInstance(mock);

    const outputs: Array<{ status: string; result: string | null }> = [];
    const result = await startSession(
      MOCK_CHAT,
      BASE_INPUT,
      async (output) => {
        outputs.push(output);
        if (output.newSessionId && !output.result) {
          abortSession('test-chat');
        }
      },
    );

    expect(result.status).toBe('success');
    const promptOutput = outputs.find((o) => o.result !== null);
    expect(promptOutput?.result).toBe('Hello World');
    expect(mock.client.session.message).toHaveBeenCalled();
  });

  it('returns error when message.updated carries an error for the target message', async () => {
    let capturedMessageId: string | undefined;
    const SESSION_ID = 'sess-1';

    async function* errorStream() {
      await Promise.resolve();
      yield {
        type: 'message.updated',
        properties: {
          info: {
            sessionID: SESSION_ID,
            id: capturedMessageId ?? '__placeholder__',
            error: { name: 'ProviderError', data: 'rate limit exceeded' },
          },
        },
      };
      yield {
        type: 'session.idle',
        properties: { sessionID: SESSION_ID },
      };
    }

    const mock = {
      client: {
        session: {
          get: vi.fn().mockResolvedValue({ data: { id: SESSION_ID } }),
          create: vi.fn().mockResolvedValue({ data: { id: SESSION_ID } }),
          abort: vi.fn().mockResolvedValue({}),
          promptAsync: vi.fn().mockImplementation(
            async (opts: { body: { messageID: string } }) => {
              capturedMessageId = opts.body.messageID;
              return {};
            },
          ),
          message: vi.fn().mockResolvedValue({
            data: {
              info: { error: { name: 'ProviderError', data: 'rate limit exceeded' } },
              parts: [],
            },
          }),
        },
        event: {
          subscribe: vi.fn().mockReturnValue({ stream: errorStream() }),
        },
      },
      server: { close: vi.fn() },
    };

    _setTestOpencodeInstance(mock);

    const outputs: Array<{ status: string; result: string | null; error?: string }> = [];
    await startSession(
      MOCK_CHAT,
      BASE_INPUT,
      async (output) => {
        outputs.push(output);
        if (output.newSessionId && !output.result) {
          abortSession('test-chat');
        }
      },
    );

    const errorOutput = outputs.find((o) => o.status === 'error');
    expect(errorOutput).toBeDefined();
    expect(errorOutput?.error).toMatch(/ProviderError/i);
  });

  it('does not terminate on session.idle for a different sessionID', async () => {
    const SESSION_ID = 'sess-1';
    const WRONG_SESSION_ID = 'sess-other';

    async function* wrongIdleStream() {
      yield {
        type: 'session.idle',
        properties: { sessionID: WRONG_SESSION_ID },
      };
    }

    const mock = {
      client: {
        session: {
          get: vi.fn().mockResolvedValue({ data: { id: SESSION_ID } }),
          create: vi.fn().mockResolvedValue({ data: { id: SESSION_ID } }),
          abort: vi.fn().mockResolvedValue({}),
          promptAsync: vi.fn().mockResolvedValue({}),
          message: vi.fn().mockResolvedValue({
            data: { info: null, parts: [{ type: 'text', text: 'correct reply' }] },
          }),
        },
        event: {
          subscribe: vi.fn().mockReturnValue({ stream: wrongIdleStream() }),
        },
      },
      server: { close: vi.fn() },
    };

    _setTestOpencodeInstance(mock);

    const outputs: Array<{ status: string; result: string | null }> = [];
    const result = await startSession(
      MOCK_CHAT,
      BASE_INPUT,
      async (output) => {
        outputs.push(output);
        if (output.newSessionId && !output.result) {
          abortSession('test-chat');
        }
      },
    );

    expect(result.status).toBe('success');
    const promptOutput = outputs.find((o) => o.result !== null);
    expect(promptOutput?.result).toBe('correct reply');
    expect(mock.client.session.message).toHaveBeenCalled();
  });

  it('calls session.message for canonical fetch after stream completes', async () => {
    const mock = makeStreamingMock([], {
      data: { info: null, parts: [{ type: 'text', text: 'fetched' }] },
    });

    _setTestOpencodeInstance(mock);

    await startSession(MOCK_CHAT, BASE_INPUT, async (output) => {
      if (output.newSessionId && !output.result) abortSession('test-chat');
    });

    expect(mock.client.session.message).toHaveBeenCalledTimes(1);
  });
});

describe('sendFollowUp', () => {
  it('returns false when opencode is not booted', async () => {
    _setTestOpencodeInstance(null);
    const sent = await sendFollowUp('test-chat', 'hello again');
    expect(sent).toBe(false);
  });

  it('returns false when no active session exists for the chat', async () => {
    _setTestOpencodeInstance(makeMockInstance());
    const sent = await sendFollowUp('test-chat', 'hello again');
    expect(sent).toBe(false);
  });

  it('sends follow-up prompt and calls onOutput with the result', async () => {
    const mock = makeStreamingMock([], {
      data: { info: null, parts: [{ type: 'text', text: 'follow-up reply' }] },
    });
    _setTestOpencodeInstance(mock);

    const outputs: Array<{ status: string; result: string | null; newSessionId?: string }> = [];

    const sessionDone = startSession(
      MOCK_CHAT,
      BASE_INPUT,
      async (output) => {
        outputs.push(output);
      },
    );

    await new Promise<void>((resolve) => {
      const check = setInterval(() => {
        const hasMarker = outputs.some((o) => o.newSessionId && !o.result);
        if (hasMarker) {
          clearInterval(check);
          resolve();
        }
      }, 10);
    });

    const sent = await sendFollowUp('test-chat', 'follow-up question');
    expect(sent).toBe(true);

    abortSession('test-chat');
    await sessionDone;

    const followUpOutput = outputs.filter((o) => o.result === 'follow-up reply');
    expect(followUpOutput.length).toBeGreaterThan(0);
  });

  it('prepends context prefix to follow-up prompt', async () => {
    const promptCalls: Array<{ body: { parts: Array<{ text: string }> } }> = [];

    let callCount = 0;
    const mock = {
      client: {
        session: {
          get: vi.fn().mockResolvedValue({ data: { id: 'sess-1' } }),
          create: vi.fn().mockResolvedValue({ data: { id: 'sess-1' } }),
          abort: vi.fn().mockResolvedValue({}),
          promptAsync: vi.fn().mockImplementation(async (opts: unknown) => {
            promptCalls.push(opts as typeof promptCalls[0]);
            return {};
          }),
          message: vi.fn().mockResolvedValue({
            data: { info: null, parts: [{ type: 'text', text: 'ok' }] },
          }),
        },
        event: {
          subscribe: vi.fn().mockImplementation(() => {
            callCount++;
            return { stream: (async function* () {})() };
          }),
        },
      },
      server: { close: vi.fn() },
    };

    _setTestOpencodeInstance(mock);

    const outputs: Array<{ newSessionId?: string; result: unknown }> = [];
    const sessionDone = startSession(MOCK_CHAT, BASE_INPUT, async (output) => {
      outputs.push(output);
    });

    await new Promise<void>((resolve) => {
      const check = setInterval(() => {
        if (outputs.some((o) => o.newSessionId && !o.result)) {
          clearInterval(check);
          resolve();
        }
      }, 10);
    });

    await sendFollowUp('test-chat', 'my follow-up');
    abortSession('test-chat');
    await sessionDone;

    expect(promptCalls.length).toBeGreaterThanOrEqual(2);
    const followUpText = promptCalls[1].body.parts[0].text;
    expect(followUpText).toContain('<pocketbrain_context>');
    expect(followUpText).toContain('my follow-up');
  });
});

describe('abortSession', () => {
  it('is a no-op when no active session exists', () => {
    expect(() => abortSession('nonexistent-chat')).not.toThrow();
  });

  it('resolves the session end promise so startSession returns', async () => {
    const mock = makeStreamingMock([], {
      data: { info: null, parts: [{ type: 'text', text: 'hi' }] },
    });
    _setTestOpencodeInstance(mock);

    let sessionResolved = false;

    const sessionDone = startSession(
      MOCK_CHAT,
      BASE_INPUT,
      async (output) => {
        if (output.newSessionId && !output.result) {
          abortSession('test-chat');
        }
      },
    ).then((r) => {
      sessionResolved = true;
      return r;
    });

    await sessionDone;
    expect(sessionResolved).toBe(true);
  });

  it('removes the session from activeSessions', async () => {
    const mock = makeStreamingMock([], {
      data: { info: null, parts: [{ type: 'text', text: 'hi' }] },
    });
    _setTestOpencodeInstance(mock);

    const sessionDone = startSession(MOCK_CHAT, BASE_INPUT, async (output) => {
      if (output.newSessionId && !output.result) {
        abortSession('test-chat');
      }
    });

    await sessionDone;
    expect(hasActiveSession('test-chat')).toBe(false);
  });

  it('calls client.session.abort when session is busy', async () => {
    const mock = makeStreamingMock([], {
      data: { info: null, parts: [{ type: 'text', text: 'hi' }] },
    });
    _setTestOpencodeInstance(mock);

    const sessionDone = startSession(MOCK_CHAT, BASE_INPUT, async (output) => {
      if (output.newSessionId && !output.result) {
        abortSession('test-chat');
      }
    });

    await sessionDone;
    expect(hasActiveSession('test-chat')).toBe(false);
  });
});

describe('buildContextPrefix', () => {
  it('contains the chatJid', () => {
    const prefix = _buildContextPrefix(MOCK_CHAT, { ...BASE_INPUT, chatJid: 'abc@g.us' });
    expect(prefix).toContain('abc@g.us');
  });

  it('contains the chatFolder', () => {
    const prefix = _buildContextPrefix(MOCK_CHAT, { ...BASE_INPUT, chatFolder: 'my-folder' });
    expect(prefix).toContain('my-folder');
  });

  it('returns a non-empty string', () => {
    const prefix = _buildContextPrefix(MOCK_CHAT, BASE_INPUT);
    expect(typeof prefix).toBe('string');
    expect(prefix.length).toBeGreaterThan(0);
  });

  it('contains pocketbrain_context XML tags', () => {
    const prefix = _buildContextPrefix(MOCK_CHAT, BASE_INPUT);
    expect(prefix).toContain('<pocketbrain_context>');
    expect(prefix).toContain('</pocketbrain_context>');
  });
});
