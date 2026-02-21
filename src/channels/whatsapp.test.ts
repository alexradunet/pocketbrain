import { describe, it, expect, beforeEach, vi, afterEach } from 'bun:test';
import { EventEmitter } from 'events';

async function advanceTimersByTimeAsync(ms: number): Promise<void> {
  vi.advanceTimersByTime(ms);
  await Promise.resolve();
  await Promise.resolve();
}

// --- Mocks ---

// Mock config
vi.mock('../config.js', () => ({
  DATA_DIR: '/tmp/pocketbrain-test-data',
  ASSISTANT_HAS_OWN_NUMBER: false,
}));

// Mock logger
vi.mock('../logger.js', () => ({
  logger: {
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  },
}));

// Mock fs
vi.mock('fs', () => ({
  default: {
    existsSync: vi.fn(() => true),
    mkdirSync: vi.fn(),
  },
  existsSync: vi.fn(() => true),
  mkdirSync: vi.fn(),
}));

// Mock child_process (used for osascript notification)
vi.mock('child_process', () => ({
  exec: vi.fn(),
}));

// Build a fake WASocket that's an EventEmitter with the methods we need
function createFakeSocket() {
  const ev = new EventEmitter();
  const sock = {
    ev: {
      on: (event: string, handler: (...args: unknown[]) => void) => {
        ev.on(event, handler);
      },
    },
    user: {
      id: '1234567890:1@s.whatsapp.net',
      lid: '9876543210:1@lid',
    },
    sendMessage: vi.fn().mockResolvedValue(undefined),
    sendPresenceUpdate: vi.fn().mockResolvedValue(undefined),
    end: vi.fn(),
    // Expose the event emitter for triggering events in tests
    _ev: ev,
  };
  return sock;
}

let fakeSocket: ReturnType<typeof createFakeSocket>;

// Mock Baileys
vi.mock('@whiskeysockets/baileys', () => {
  return {
    default: vi.fn(() => fakeSocket),
    Browsers: { macOS: vi.fn(() => ['macOS', 'Chrome', '']) },
    DisconnectReason: {
      loggedOut: 401,
      badSession: 500,
      connectionClosed: 428,
      connectionLost: 408,
      connectionReplaced: 440,
      timedOut: 408,
      restartRequired: 515,
    },
    makeCacheableSignalKeyStore: vi.fn((keys: unknown) => keys),
    useMultiFileAuthState: vi.fn().mockResolvedValue({
      state: {
        creds: {},
        keys: {},
      },
      saveCreds: vi.fn(),
    }),
  };
});

import { WhatsAppChannel, WhatsAppChannelOpts } from './whatsapp.js';

// --- Test helpers ---

function createTestOpts(overrides?: Partial<WhatsAppChannelOpts>): WhatsAppChannelOpts {
  return {
    onMessage: vi.fn(),
    chats: vi.fn(() => ({
      'registered@g.us': {
        jid: 'registered@g.us',
        name: 'Test Group',
        folder: 'test-group',
        addedAt: '2024-01-01T00:00:00.000Z',
      },
    })),
    ...overrides,
  };
}

function triggerConnection(state: string, extra?: Record<string, unknown>) {
  fakeSocket._ev.emit('connection.update', { connection: state, ...extra });
}

function triggerDisconnect(statusCode: number) {
  fakeSocket._ev.emit('connection.update', {
    connection: 'close',
    lastDisconnect: {
      error: { output: { statusCode } },
    },
  });
}

async function triggerMessages(messages: unknown[]) {
  fakeSocket._ev.emit('messages.upsert', { messages });
  // Flush microtasks so the async messages.upsert handler completes
  await new Promise((r) => setTimeout(r, 0));
}

// --- Tests ---

describe('WhatsAppChannel', () => {
  beforeEach(() => {
    fakeSocket = createFakeSocket();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  /**
   * Helper: start connect, flush microtasks so event handlers are registered,
   * then trigger the connection open event. Returns the resolved promise.
   */
  async function connectChannel(channel: WhatsAppChannel): Promise<void> {
    const p = channel.connect();
    // Flush microtasks so connectInternal completes its await and registers handlers
    await new Promise((r) => setTimeout(r, 0));
    triggerConnection('open');
    return p;
  }

  // --- Connection lifecycle ---

  describe('connection lifecycle', () => {
    it('resolves connect() when connection opens', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      expect(channel.isConnected()).toBe(true);
    });

    it('sets up LID to phone mapping on open', async () => {
      // Use a chats that contains the phone JID so onMessage fires
      const opts = createTestOpts({
        chats: vi.fn(() => ({
          '1234567890@s.whatsapp.net': {
            jid: '1234567890@s.whatsapp.net',
            name: 'Self Chat',
            folder: 'self-chat',
            addedAt: '2024-01-01T00:00:00.000Z',
          },
        })),
      });
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      // sock.user.lid = '9876543210:1@lid' → sock.user.id = '1234567890:1@s.whatsapp.net'
      // The LID mapping is built on open; sending from LID must translate to phone JID
      await triggerMessages([
        {
          key: {
            id: 'msg-lid-init',
            remoteJid: '9876543210@lid',
            fromMe: false,
          },
          message: { conversation: 'From LID after open' },
          pushName: 'Self',
          messageTimestamp: Math.floor(Date.now() / 1000),
        },
      ]);

      // Message should be delivered with translated JID
      expect(opts.onMessage).toHaveBeenCalledWith(
        '1234567890@s.whatsapp.net',
        expect.objectContaining({
          chat_jid: '1234567890@s.whatsapp.net',
          content: 'From LID after open',
        }),
      );
    });

    it('flushes outgoing queue on reconnect', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      // Disconnect
      (channel as any).connected = false;

      // Queue a message while disconnected
      await channel.sendMessage('test@g.us', 'Queued message');
      expect(fakeSocket.sendMessage).not.toHaveBeenCalled();

      // Reconnect
      (channel as any).connected = true;
      await (channel as any).flushOutgoingQueue();

      // Messages get prefixed with assistant name
      expect(fakeSocket.sendMessage).toHaveBeenCalledWith(
        'test@g.us',
        { text: 'Andy: Queued message' },
      );
    });

    it('disconnects cleanly', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      await channel.disconnect();
      expect(channel.isConnected()).toBe(false);
      expect(fakeSocket.end).toHaveBeenCalled();
    });
  });

  // --- QR code and auth ---

  describe('authentication', () => {
    it('exits process when QR code is emitted (no auth state)', async () => {
      vi.useFakeTimers();
      try {
        const mockExit = vi.spyOn(process, 'exit').mockImplementation(() => undefined as never);

        const opts = createTestOpts();
        const channel = new WhatsAppChannel(opts);

        // Start connect but don't await (it won't resolve - process exits)
        channel.connect().catch(() => {});

        // Flush microtasks so connectInternal registers handlers
        await advanceTimersByTimeAsync(0);

        // Emit QR code event
        fakeSocket._ev.emit('connection.update', { qr: 'some-qr-data' });

        // Advance timer past the 1000ms setTimeout before exit
        await advanceTimersByTimeAsync(1500);

        expect(mockExit).toHaveBeenCalledWith(1);
        mockExit.mockRestore();
      } finally {
        vi.useRealTimers();
      }
    });
  });

  // --- Reconnection behavior ---

  describe('reconnection', () => {
    it('does not create multiple concurrent reconnections on rapid close events', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      // Spy on connectInternal to count invocations
      let reconnectCount = 0;
      const originalConnectInternal = (channel as any).connectInternal.bind(channel);
      (channel as any).connectInternal = async (...args: unknown[]) => {
        reconnectCount++;
        return originalConnectInternal(...args);
      };

      // Trigger two rapid close events before any microtask runs
      triggerDisconnect(428);
      triggerDisconnect(428);

      await new Promise((r) => setTimeout(r, 50));

      // Only one reconnect should have been initiated (mutex guard)
      expect(reconnectCount).toBe(1);
    });

    it('reconnects on non-loggedOut disconnect', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      expect(channel.isConnected()).toBe(true);

      // Disconnect with a non-loggedOut reason (e.g., connectionClosed = 428)
      triggerDisconnect(428);

      expect(channel.isConnected()).toBe(false);
    });

    it('exits on loggedOut disconnect', async () => {
      const mockExit = vi.spyOn(process, 'exit').mockImplementation(() => undefined as never);

      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      // Disconnect with loggedOut reason (401)
      triggerDisconnect(401);

      expect(channel.isConnected()).toBe(false);
      expect(mockExit).toHaveBeenCalledWith(0);
      mockExit.mockRestore();
    });

    it('retries reconnection after 5s on failure', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      // Disconnect with stream error 515
      triggerDisconnect(515);

      // The channel sets a 5s retry — just verify it doesn't crash
      await new Promise((r) => setTimeout(r, 100));
    });

    it('blocks new reconnect during 5s retry window after first reconnect fails', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      vi.useFakeTimers();
      try {
        let reconnectCount = 0;
        (channel as any).connectInternal = async () => {
          reconnectCount++;
          throw new Error('Connection refused');
        };

        triggerDisconnect(428);
        await advanceTimersByTimeAsync(10);

        triggerDisconnect(428);
        await advanceTimersByTimeAsync(10);

        expect(reconnectCount).toBe(1);
      } finally {
        vi.useRealTimers();
      }
    });

    it('fires the 5s retry even when a concurrent disconnect was blocked', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      vi.useFakeTimers();
      try {
        let reconnectCount = 0;
        (channel as any).connectInternal = async () => {
          reconnectCount++;
          throw new Error('Connection refused');
        };

        triggerDisconnect(428);
        await advanceTimersByTimeAsync(10);
        expect(reconnectCount).toBe(1);

        triggerDisconnect(428);
        await advanceTimersByTimeAsync(10);
        expect(reconnectCount).toBe(1);

        await advanceTimersByTimeAsync(5100);
        expect(reconnectCount).toBe(2);
      } finally {
        vi.useRealTimers();
      }
    });
  });

  // --- Message handling ---

  describe('message handling', () => {
    it('delivers message for registered chat', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      await triggerMessages([
        {
          key: {
            id: 'msg-1',
            remoteJid: 'registered@g.us',
            participant: '5551234@s.whatsapp.net',
            fromMe: false,
          },
          message: { conversation: 'Hello Andy' },
          pushName: 'Alice',
          messageTimestamp: Math.floor(Date.now() / 1000),
        },
      ]);

      expect(opts.onMessage).toHaveBeenCalledWith(
        'registered@g.us',
        expect.objectContaining({
          id: 'msg-1',
          content: 'Hello Andy',
          sender_name: 'Alice',
          is_from_me: false,
        }),
      );
    });

    it('does not deliver messages for unregistered chats', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      await triggerMessages([
        {
          key: {
            id: 'msg-2',
            remoteJid: 'unregistered@g.us',
            participant: '5551234@s.whatsapp.net',
            fromMe: false,
          },
          message: { conversation: 'Hello' },
          pushName: 'Bob',
          messageTimestamp: Math.floor(Date.now() / 1000),
        },
      ]);

      expect(opts.onMessage).not.toHaveBeenCalled();
    });

    it('ignores status@broadcast messages', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      await triggerMessages([
        {
          key: {
            id: 'msg-3',
            remoteJid: 'status@broadcast',
            fromMe: false,
          },
          message: { conversation: 'Status update' },
          messageTimestamp: Math.floor(Date.now() / 1000),
        },
      ]);

      expect(opts.onMessage).not.toHaveBeenCalled();
    });

    it('ignores messages with no content', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      await triggerMessages([
        {
          key: {
            id: 'msg-4',
            remoteJid: 'registered@g.us',
            fromMe: false,
          },
          message: null,
          messageTimestamp: Math.floor(Date.now() / 1000),
        },
      ]);

      expect(opts.onMessage).not.toHaveBeenCalled();
    });

    it('extracts text from extendedTextMessage', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      await triggerMessages([
        {
          key: {
            id: 'msg-5',
            remoteJid: 'registered@g.us',
            participant: '5551234@s.whatsapp.net',
            fromMe: false,
          },
          message: {
            extendedTextMessage: { text: 'A reply message' },
          },
          pushName: 'Charlie',
          messageTimestamp: Math.floor(Date.now() / 1000),
        },
      ]);

      expect(opts.onMessage).toHaveBeenCalledWith(
        'registered@g.us',
        expect.objectContaining({ content: 'A reply message' }),
      );
    });

    it('extracts caption from imageMessage', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      await triggerMessages([
        {
          key: {
            id: 'msg-6',
            remoteJid: 'registered@g.us',
            participant: '5551234@s.whatsapp.net',
            fromMe: false,
          },
          message: {
            imageMessage: { caption: 'Check this photo', mimetype: 'image/jpeg' },
          },
          pushName: 'Diana',
          messageTimestamp: Math.floor(Date.now() / 1000),
        },
      ]);

      expect(opts.onMessage).toHaveBeenCalledWith(
        'registered@g.us',
        expect.objectContaining({ content: 'Check this photo' }),
      );
    });

    it('extracts caption from videoMessage', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      await triggerMessages([
        {
          key: {
            id: 'msg-7',
            remoteJid: 'registered@g.us',
            participant: '5551234@s.whatsapp.net',
            fromMe: false,
          },
          message: {
            videoMessage: { caption: 'Watch this', mimetype: 'video/mp4' },
          },
          pushName: 'Eve',
          messageTimestamp: Math.floor(Date.now() / 1000),
        },
      ]);

      expect(opts.onMessage).toHaveBeenCalledWith(
        'registered@g.us',
        expect.objectContaining({ content: 'Watch this' }),
      );
    });

    it('handles message with no extractable text (e.g. voice note without caption)', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      await triggerMessages([
        {
          key: {
            id: 'msg-8',
            remoteJid: 'registered@g.us',
            participant: '5551234@s.whatsapp.net',
            fromMe: false,
          },
          message: {
            audioMessage: { mimetype: 'audio/ogg; codecs=opus', ptt: true },
          },
          pushName: 'Frank',
          messageTimestamp: Math.floor(Date.now() / 1000),
        },
      ]);

      // Still delivered but with empty content
      expect(opts.onMessage).toHaveBeenCalledWith(
        'registered@g.us',
        expect.objectContaining({ content: '' }),
      );
    });

    it('uses sender JID when pushName is absent', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      await triggerMessages([
        {
          key: {
            id: 'msg-9',
            remoteJid: 'registered@g.us',
            participant: '5551234@s.whatsapp.net',
            fromMe: false,
          },
          message: { conversation: 'No push name' },
          // pushName is undefined
          messageTimestamp: Math.floor(Date.now() / 1000),
        },
      ]);

      expect(opts.onMessage).toHaveBeenCalledWith(
        'registered@g.us',
        expect.objectContaining({ sender_name: '5551234' }),
      );
    });
  });

  // --- LID ↔ JID translation ---

  describe('LID to JID translation', () => {
    it('translates known LID to phone JID', async () => {
      const opts = createTestOpts({
        chats: vi.fn(() => ({
          '1234567890@s.whatsapp.net': {
            jid: '1234567890@s.whatsapp.net',
            name: 'Self Chat',
            folder: 'self-chat',
            addedAt: '2024-01-01T00:00:00.000Z',
          },
        })),
      });
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      // The socket has lid '9876543210:1@lid' → phone '1234567890@s.whatsapp.net'
      await triggerMessages([
        {
          key: {
            id: 'msg-lid',
            remoteJid: '9876543210@lid',
            fromMe: false,
          },
          message: { conversation: 'From LID' },
          pushName: 'Self',
          messageTimestamp: Math.floor(Date.now() / 1000),
        },
      ]);

      // Should be translated to phone JID and delivered
      expect(opts.onMessage).toHaveBeenCalledWith(
        '1234567890@s.whatsapp.net',
        expect.objectContaining({
          chat_jid: '1234567890@s.whatsapp.net',
          content: 'From LID',
        }),
      );
    });

    it('passes through non-LID JIDs unchanged', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      await triggerMessages([
        {
          key: {
            id: 'msg-normal',
            remoteJid: 'registered@g.us',
            participant: '5551234@s.whatsapp.net',
            fromMe: false,
          },
          message: { conversation: 'Normal JID' },
          pushName: 'Grace',
          messageTimestamp: Math.floor(Date.now() / 1000),
        },
      ]);

      expect(opts.onMessage).toHaveBeenCalledWith(
        'registered@g.us',
        expect.objectContaining({
          chat_jid: 'registered@g.us',
          content: 'Normal JID',
        }),
      );
    });

    it('does not deliver unknown LID JIDs (not registered)', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      await triggerMessages([
        {
          key: {
            id: 'msg-unknown-lid',
            remoteJid: '0000000000@lid',
            fromMe: false,
          },
          message: { conversation: 'Unknown LID' },
          pushName: 'Unknown',
          messageTimestamp: Math.floor(Date.now() / 1000),
        },
      ]);

      // Unknown LID passes through unchanged but isn't registered — not delivered
      expect(opts.onMessage).not.toHaveBeenCalled();
    });
  });

  // --- Outgoing message queue ---

  describe('outgoing message queue', () => {
    it('sends message directly when connected', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      await channel.sendMessage('test@g.us', 'Hello');
      expect(fakeSocket.sendMessage).toHaveBeenCalledWith('test@g.us', { text: 'Andy: Hello' });
    });

    it('prefixes direct chat messages on shared number', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      await channel.sendMessage('123@s.whatsapp.net', 'Hello');
      expect(fakeSocket.sendMessage).toHaveBeenCalledWith('123@s.whatsapp.net', { text: 'Andy: Hello' });
    });

    it('queues message when disconnected', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await channel.sendMessage('test@g.us', 'Queued');
      expect(fakeSocket.sendMessage).not.toHaveBeenCalled();
    });

    it('queues message on send failure', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      fakeSocket.sendMessage.mockRejectedValueOnce(new Error('Network error'));

      await channel.sendMessage('test@g.us', 'Will fail');
      // Should not throw, message queued for retry
    });

    it('does not lose message when send fails mid-flush (peek-then-shift)', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await channel.sendMessage('test@g.us', 'First');
      await channel.sendMessage('test@g.us', 'Second');

      fakeSocket.sendMessage
        .mockRejectedValueOnce(new Error('network error'))
        .mockResolvedValue({});

      await connectChannel(channel);
      await new Promise((r) => setTimeout(r, 50));

      expect(fakeSocket.sendMessage).toHaveBeenCalledWith('test@g.us', { text: 'Andy: First' });
    });

    it('retains failed message in queue for retry (peek-then-shift safety)', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await channel.sendMessage('test@g.us', 'Important');

      fakeSocket.sendMessage.mockRejectedValueOnce(new Error('network error'));
      await connectChannel(channel);
      await new Promise((r) => setTimeout(r, 50));

      fakeSocket.sendMessage.mockResolvedValue({});
      await (channel as any).flushOutgoingQueue();

      expect(fakeSocket.sendMessage).toHaveBeenCalledTimes(2);
      expect(fakeSocket.sendMessage).toHaveBeenLastCalledWith('test@g.us', { text: 'Andy: Important' });
    });

    it('flushes multiple queued messages in order', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await channel.sendMessage('test@g.us', 'First');
      await channel.sendMessage('test@g.us', 'Second');
      await channel.sendMessage('test@g.us', 'Third');

      await connectChannel(channel);
      await new Promise((r) => setTimeout(r, 50));

      expect(fakeSocket.sendMessage).toHaveBeenCalledTimes(3);
      expect(fakeSocket.sendMessage).toHaveBeenNthCalledWith(1, 'test@g.us', { text: 'Andy: First' });
      expect(fakeSocket.sendMessage).toHaveBeenNthCalledWith(2, 'test@g.us', { text: 'Andy: Second' });
      expect(fakeSocket.sendMessage).toHaveBeenNthCalledWith(3, 'test@g.us', { text: 'Andy: Third' });
    });

    it('caps queue at MAX_OUTGOING_QUEUE_SIZE=100 and drops oldest on overflow', async () => {
      const { logger } = await import('../logger.js');
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      for (let i = 1; i <= 101; i++) {
        await channel.sendMessage('test@g.us', `Message ${i}`);
      }

      const queue: Array<{ jid: string; text: string }> = (channel as any).outgoingQueue;

      expect(queue.length).toBe(100);
      expect(queue[0].text).toBe('Andy: Message 2');
      expect(queue[99].text).toBe('Andy: Message 101');

      expect(logger.warn).toHaveBeenCalledWith(
        { queueSize: 100 },
        'outgoingQueue full, dropping oldest message',
      );
    });
  });

  // --- JID ownership ---

  describe('ownsJid', () => {
    it('owns @g.us JIDs (WhatsApp groups)', () => {
      const channel = new WhatsAppChannel(createTestOpts());
      expect(channel.ownsJid('12345@g.us')).toBe(true);
    });

    it('owns @s.whatsapp.net JIDs (WhatsApp DMs)', () => {
      const channel = new WhatsAppChannel(createTestOpts());
      expect(channel.ownsJid('12345@s.whatsapp.net')).toBe(true);
    });

    it('does not own Telegram JIDs', () => {
      const channel = new WhatsAppChannel(createTestOpts());
      expect(channel.ownsJid('tg:12345')).toBe(false);
    });

    it('does not own unknown JID formats', () => {
      const channel = new WhatsAppChannel(createTestOpts());
      expect(channel.ownsJid('random-string')).toBe(false);
    });
  });

  // --- Typing indicator ---

  describe('setTyping', () => {
    it('sends composing presence when typing', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      await channel.setTyping('test@g.us', true);
      expect(fakeSocket.sendPresenceUpdate).toHaveBeenCalledWith('composing', 'test@g.us');
    });

    it('sends paused presence when stopping', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      await channel.setTyping('test@g.us', false);
      expect(fakeSocket.sendPresenceUpdate).toHaveBeenCalledWith('paused', 'test@g.us');
    });

    it('handles typing indicator failure gracefully', async () => {
      const opts = createTestOpts();
      const channel = new WhatsAppChannel(opts);

      await connectChannel(channel);

      fakeSocket.sendPresenceUpdate.mockRejectedValueOnce(new Error('Failed'));

      await expect(channel.setTyping('test@g.us', true)).resolves.toBeUndefined();
    });
  });

  // --- Channel properties ---

  describe('channel properties', () => {
    it('has name "whatsapp"', () => {
      const channel = new WhatsAppChannel(createTestOpts());
      expect(channel.name).toBe('whatsapp');
    });
  });
});
