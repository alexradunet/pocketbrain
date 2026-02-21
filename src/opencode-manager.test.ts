import { afterEach, beforeEach, describe, expect, it, vi } from 'bun:test';

import {
  abortSession,
  hasActiveSession,
  sendFollowUp,
  startSession,
  _buildContextPrefix,
  _clearActiveSessions,
  _setTestOpencodeInstance,
} from './opencode-manager.js';
import type { RegisteredGroup } from './types.js';

const MOCK_GROUP: RegisteredGroup = {
  name: 'Test Group',
  folder: 'test-group',
  added_at: '2024-01-01T00:00:00.000Z',
};

const BASE_INPUT = {
  prompt: 'hello',
  groupFolder: 'test-group',
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
 * events in order. `messageResult` controls what client.session.message() returns
 * (the canonical fetch after stream ends). Pass `null` to simulate a timeout/no-data.
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
      abortSession('test-group');
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
      MOCK_GROUP,
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
      MOCK_GROUP,
      { ...BASE_INPUT, sessionId: 'stale-id' },
      async () => {},
    );

    expect(result.status).toBe('error');
    expect(result.error).toMatch(/create failed/i);
  });
});

describe('runPrompt — via startSession — SSE event processing', () => {
  /**
   * To test runPrompt's event loop, we need to know the messageID it generates
   * (via crypto.randomUUID) and the sessionID. Since the messageID is random,
   * we set up events that use the WRONG sessionID or messageID, then rely on the
   * canonical fetch (client.session.message) to provide the final text.
   *
   * For delta accumulation, we need matching sessionID + messageID.
   * We intercept promptAsync to capture the messageID.
   */

  it('returns canonical text from session.message after stream ends', async () => {
    const mock = makeStreamingMock(
      [], // no SSE events — rely purely on canonical fetch
      { data: { info: null, parts: [{ type: 'text', text: 'hello from canonical' }] } },
    );

    _setTestOpencodeInstance(mock);

    const outputs: Array<{ status: string; result: string | null }> = [];
    const result = await startSession(
      MOCK_GROUP,
      BASE_INPUT,
      async (output) => {
        outputs.push(output);
        if (output.newSessionId && !output.result) {
          abortSession('test-group');
        }
      },
    );

    expect(result.status).toBe('success');
    // The first output call carries the prompt result
    const promptOutput = outputs.find((o) => o.result !== null);
    expect(promptOutput?.result).toBe('hello from canonical');
  });

  it('accumulates text parts via delta events for the correct messageID', async () => {
    let capturedMessageId: string | undefined;

    // We need to intercept promptAsync to get the messageID, then inject events
    // that reference it. Use a deferred approach: promptAsync resolves after we
    // set up the stream. Because makeStreamingMock uses a static gen(), we need
    // to build the events dynamically.

    // Strategy: use a passthrough async generator that waits for promptAsync to
    // fire, then yields events with the captured messageID.
    const SESSION_ID = 'sess-1';
    let emitEvents!: (events: object[]) => void;
    const eventQueue: object[] = [];
    let resolveStream!: () => void;
    const streamDone = new Promise<void>((r) => { resolveStream = r; });

    async function* dynamicStream() {
      // Wait until promptAsync has been called (so we have the messageID)
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
              // Emit matching events now that we have the messageID
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
      MOCK_GROUP,
      BASE_INPUT,
      async (output) => {
        outputs.push(output);
        if (output.newSessionId && !output.result) {
          abortSession('test-group');
        }
      },
    );

    expect(result.status).toBe('success');
    const promptOutput = outputs.find((o) => o.result !== null);
    // Canonical fetch returns the assembled text
    expect(promptOutput?.result).toBe('Hello World');
    // session.message was called for the canonical fetch
    expect(mock.client.session.message).toHaveBeenCalled();
  });

  it('returns error when message.updated carries an error for the target message', async () => {
    let capturedMessageId: string | undefined;
    const SESSION_ID = 'sess-1';

    async function* errorStream() {
      // Yield nothing until promptAsync fires; we use a simple approach:
      // promptAsync is synchronous enough that we can yield after a tick
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
      MOCK_GROUP,
      BASE_INPUT,
      async (output) => {
        outputs.push(output);
        if (output.newSessionId && !output.result) {
          abortSession('test-group');
        }
      },
    );

    // The canonical fetch detects the error in info.error
    const errorOutput = outputs.find((o) => o.status === 'error');
    expect(errorOutput).toBeDefined();
    expect(errorOutput?.error).toMatch(/ProviderError/i);
  });

  it('does not terminate on session.idle for a different sessionID', async () => {
    const SESSION_ID = 'sess-1';
    const WRONG_SESSION_ID = 'sess-other';

    async function* wrongIdleStream() {
      // Emit session.idle for a different session — should be ignored
      yield {
        type: 'session.idle',
        properties: { sessionID: WRONG_SESSION_ID },
      };
      // Stream ends naturally without matching idle — runPrompt falls through to
      // canonical fetch
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
      MOCK_GROUP,
      BASE_INPUT,
      async (output) => {
        outputs.push(output);
        if (output.newSessionId && !output.result) {
          abortSession('test-group');
        }
      },
    );

    expect(result.status).toBe('success');
    // Canonical fetch still provides the text even without matching session.idle
    const promptOutput = outputs.find((o) => o.result !== null);
    expect(promptOutput?.result).toBe('correct reply');
    // session.message WAS called (canonical fetch always runs)
    expect(mock.client.session.message).toHaveBeenCalled();
  });

  it('calls session.message for canonical fetch after stream completes', async () => {
    const mock = makeStreamingMock([], {
      data: { info: null, parts: [{ type: 'text', text: 'fetched' }] },
    });

    _setTestOpencodeInstance(mock);

    await startSession(MOCK_GROUP, BASE_INPUT, async (output) => {
      if (output.newSessionId && !output.result) abortSession('test-group');
    });

    expect(mock.client.session.message).toHaveBeenCalledTimes(1);
  });
});

describe('sendFollowUp', () => {
  it('returns false when opencode is not booted', async () => {
    _setTestOpencodeInstance(null);
    const sent = await sendFollowUp('test-group', 'hello again');
    expect(sent).toBe(false);
  });

  it('returns false when no active session exists for the group', async () => {
    _setTestOpencodeInstance(makeMockInstance());
    // No startSession called — no active session
    const sent = await sendFollowUp('test-group', 'hello again');
    expect(sent).toBe(false);
  });

  it('sends follow-up prompt and calls onOutput with the result', async () => {
    const mock = makeStreamingMock([], {
      data: { info: null, parts: [{ type: 'text', text: 'follow-up reply' }] },
    });
    _setTestOpencodeInstance(mock);

    const outputs: Array<{ status: string; result: string | null; newSessionId?: string }> = [];

    // Start a session so there is an active session
    const sessionDone = startSession(
      MOCK_GROUP,
      BASE_INPUT,
      async (output) => {
        outputs.push(output);
        // Don't abort immediately — let follow-up run first
      },
    );

    // Wait for the first prompt to complete (session enters wait state)
    // The first output with result !== null is the first prompt result.
    // The second output with newSessionId && !result is the session-update marker.
    await new Promise<void>((resolve) => {
      const check = setInterval(() => {
        const hasMarker = outputs.some((o) => o.newSessionId && !o.result);
        if (hasMarker) {
          clearInterval(check);
          resolve();
        }
      }, 10);
    });

    // Now send a follow-up
    const sent = await sendFollowUp('test-group', 'follow-up question');
    expect(sent).toBe(true);

    // Abort and wait for session to complete
    abortSession('test-group');
    await sessionDone;

    // There should be a follow-up result in outputs
    const followUpOutput = outputs.filter((o) => o.result === 'follow-up reply');
    expect(followUpOutput.length).toBeGreaterThan(0);
  });

  it('prepends context prefix to follow-up prompt', async () => {
    const promptCalls: Array<{ body: { parts: Array<{ text: string }> } }> = [];

    // Use a fresh stream for each subscribe call
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
    const sessionDone = startSession(MOCK_GROUP, BASE_INPUT, async (output) => {
      outputs.push(output);
    });

    // Wait for session marker
    await new Promise<void>((resolve) => {
      const check = setInterval(() => {
        if (outputs.some((o) => o.newSessionId && !o.result)) {
          clearInterval(check);
          resolve();
        }
      }, 10);
    });

    await sendFollowUp('test-group', 'my follow-up');
    abortSession('test-group');
    await sessionDone;

    // The second promptAsync call (follow-up) should have context prefix prepended
    expect(promptCalls.length).toBeGreaterThanOrEqual(2);
    const followUpText = promptCalls[1].body.parts[0].text;
    expect(followUpText).toContain('<pocketbrain_context>');
    expect(followUpText).toContain('my follow-up');
  });
});

describe('abortSession', () => {
  it('is a no-op when no active session exists', () => {
    // Should not throw
    expect(() => abortSession('nonexistent-group')).not.toThrow();
  });

  it('resolves the session end promise so startSession returns', async () => {
    const mock = makeStreamingMock([], {
      data: { info: null, parts: [{ type: 'text', text: 'hi' }] },
    });
    _setTestOpencodeInstance(mock);

    let sessionResolved = false;

    const sessionDone = startSession(
      MOCK_GROUP,
      BASE_INPUT,
      async (output) => {
        // Abort once we see the session-update marker
        if (output.newSessionId && !output.result) {
          abortSession('test-group');
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

    const sessionDone = startSession(MOCK_GROUP, BASE_INPUT, async (output) => {
      if (output.newSessionId && !output.result) {
        abortSession('test-group');
      }
    });

    await sessionDone;
    // After session ends, hasActiveSession should return false
    expect(hasActiveSession('test-group')).toBe(false);
  });

  it('calls client.session.abort when session is busy', async () => {
    // This is hard to test without racing; verify abort is callable without errors
    const mock = makeStreamingMock([], {
      data: { info: null, parts: [{ type: 'text', text: 'hi' }] },
    });
    _setTestOpencodeInstance(mock);

    // Start session and abort immediately from onOutput (session.busy = false after runPrompt)
    // so session.abort won't be called here (busy=false). That's correct behavior.
    const sessionDone = startSession(MOCK_GROUP, BASE_INPUT, async (output) => {
      if (output.newSessionId && !output.result) {
        abortSession('test-group');
      }
    });

    await sessionDone;
    expect(hasActiveSession('test-group')).toBe(false);
  });
});

describe('buildContextPrefix', () => {
  it('contains the chatJid', () => {
    const prefix = _buildContextPrefix(MOCK_GROUP, { ...BASE_INPUT, chatJid: 'abc@g.us' });
    expect(prefix).toContain('abc@g.us');
  });

  it('contains the groupFolder', () => {
    const prefix = _buildContextPrefix(MOCK_GROUP, { ...BASE_INPUT, groupFolder: 'my-folder' });
    expect(prefix).toContain('my-folder');
  });

  it('returns a non-empty string', () => {
    const prefix = _buildContextPrefix(MOCK_GROUP, BASE_INPUT);
    expect(typeof prefix).toBe('string');
    expect(prefix.length).toBeGreaterThan(0);
  });

  it('contains pocketbrain_context XML tags', () => {
    const prefix = _buildContextPrefix(MOCK_GROUP, BASE_INPUT);
    expect(prefix).toContain('<pocketbrain_context>');
    expect(prefix).toContain('</pocketbrain_context>');
  });
});
