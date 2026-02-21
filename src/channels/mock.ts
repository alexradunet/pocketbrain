/**
 * MockChannel — HTTP-based channel for e2e testing.
 *
 * Exposes a local HTTP server so test runners can inject messages and read
 * outbound responses without a real WhatsApp connection:
 *
 *   POST /inbox   { content, sender?, senderName?, jid? }  → 201
 *   GET  /outbox                                           → { messages: [] }
 *   DELETE /outbox                                         → 204
 *   GET  /health                                           → { status: "ok" }
 *
 * Usage: set CHANNEL=mock in the environment. PocketBrain's main() will
 * instantiate MockChannel instead of WhatsAppChannel.
 */
import { logger } from '../logger.js';
import type { Channel, ChatConfig, NewMessage, OnInboundMessage } from '../types.js';

/** JID used for the mock test group. Must match the seeded chat config. */
export const MOCK_CHANNEL_JID = 'e2e-test@mock.test';

const DEFAULT_PORT = parseInt(process.env.MOCK_CHANNEL_PORT || '3456', 10);

export interface MockChannelOpts {
  onMessage: OnInboundMessage;
  chats: () => Record<string, ChatConfig>;
  port?: number;
}

export interface OutboxMessage {
  jid: string;
  text: string;
  timestamp: string;
}

export class MockChannel implements Channel {
  name = 'mock';

  private _connected = false;
  private outbox: OutboxMessage[] = [];
  private opts: MockChannelOpts;
  private port: number;
  private server: ReturnType<typeof Bun.serve> | null = null;

  constructor(opts: MockChannelOpts) {
    this.opts = opts;
    this.port = opts.port ?? DEFAULT_PORT;
  }

  async connect(): Promise<void> {
    this._connected = true;

    this.server = Bun.serve({
      port: this.port,
      fetch: (req) => this.handleRequest(req),
    });

    logger.info({ port: this.port }, 'MockChannel HTTP server started');
  }

  private async handleRequest(req: Request): Promise<Response> {
    const url = new URL(req.url);

    if (url.pathname === '/health' && req.method === 'GET') {
      return Response.json({ status: 'ok', connected: this._connected });
    }

    if (url.pathname === '/inbox' && req.method === 'POST') {
      try {
        const body = (await req.json()) as {
          content: string;
          sender?: string;
          senderName?: string;
          jid?: string;
        };

        const msg: NewMessage = {
          id: crypto.randomUUID(),
          chat_jid: body.jid ?? MOCK_CHANNEL_JID,
          sender: body.sender ?? 'test-user@s.whatsapp.net',
          sender_name: body.senderName ?? 'TestUser',
          content: body.content,
          timestamp: new Date().toISOString(),
          is_from_me: false,
          is_bot_message: false,
        };

        // Deliver to PocketBrain's message pipeline
        this.opts.onMessage(msg.chat_jid, msg);
        logger.debug({ jid: msg.chat_jid, content: body.content }, 'MockChannel: message injected');

        return Response.json({ ok: true, id: msg.id }, { status: 201 });
      } catch (err) {
        return Response.json({ error: String(err) }, { status: 400 });
      }
    }

    if (url.pathname === '/outbox' && req.method === 'GET') {
      return Response.json({ messages: this.outbox });
    }

    if (url.pathname === '/outbox' && req.method === 'DELETE') {
      this.outbox = [];
      return new Response(null, { status: 204 });
    }

    return new Response('Not Found', { status: 404 });
  }

  async sendMessage(jid: string, text: string): Promise<void> {
    this.outbox.push({ jid, text, timestamp: new Date().toISOString() });
    logger.debug({ jid, preview: text.slice(0, 100) }, 'MockChannel: outbound message captured');
  }

  isConnected(): boolean {
    return this._connected;
  }

  ownsJid(jid: string): boolean {
    return jid === MOCK_CHANNEL_JID || jid.endsWith('@mock.test');
  }

  async disconnect(): Promise<void> {
    this._connected = false;
    this.server?.stop();
    logger.info('MockChannel disconnected');
  }

  async setTyping(_jid: string, _isTyping: boolean): Promise<void> {
    // no-op for mock
  }
}
