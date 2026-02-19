import { describe, expect, test, mock, beforeEach } from "bun:test"

/**
 * Tests for WhatsApp read receipts and typing indicators.
 *
 * We can't instantiate WhatsAppAdapter easily (needs ConnectionManager, etc.)
 * so we test via the public interface by mocking the pieces that matter.
 */

// ---- Helpers to build a minimal adapter under test --------------------------------

// We need to reach into the adapter's privates, so we import the class and
// drive it through `start()` → message event.

import { WhatsAppAdapter } from "../../../../src/adapters/channels/whatsapp/adapter"

function noop() {}
const noopLogger = {
  info: noop,
  warn: noop,
  error: noop,
  debug: noop,
  child: () => noopLogger,
} as any

function createMockSocket() {
  return {
    readMessages: mock(() => Promise.resolve()),
    sendPresenceUpdate: mock(() => Promise.resolve()),
    sendMessage: mock(() => Promise.resolve()),
    ev: {
      on: mock(() => {}),
    },
  }
}

function createAdapter(overrides: {
  messageHandler?: (jid: string, text: string) => Promise<string>
  socket?: ReturnType<typeof createMockSocket>
  whitelistAll?: boolean
  pairToken?: string
}) {
  const socket = overrides.socket ?? createMockSocket()
  const handler = overrides.messageHandler ?? (async () => "ok")

  const adapter = new WhatsAppAdapter({
    authDir: "/tmp/test-auth",
    logger: noopLogger,
    whitelistRepository: {
      isWhitelisted: () => overrides.whitelistAll !== false,
      addToWhitelist: () => true,
    } as any,
    outboxRepository: {
      listPending: () => [],
    } as any,
    messageSender: {
      send: async (_target: string, text: string, fn: (chunk: string) => Promise<void>) => {
        await fn(text)
      },
    } as any,
    pairToken: overrides.pairToken ?? "test-token",
  })

  // Monkey-patch the connection manager so we don't actually connect
  const connMgr = (adapter as any).connectionManager
  connMgr.isConnected = () => true
  connMgr.getSocket = () => socket
  connMgr.connect = async () => socket

  return { adapter, socket, handler }
}

/** Simulate what `start()` + incoming message does, but skip the real WS connect. */
async function simulateMessage(
  adapter: WhatsAppAdapter,
  handler: (jid: string, text: string) => Promise<string>,
  msg: Record<string, unknown>,
) {
  // Register the handler
  ;(adapter as any).messageHandler = handler

  // Drive handleMessage directly
  await (adapter as any).handleMessage(msg)
}

// ---- Tests -----------------------------------------------------------------------

const JID = "15551234567@s.whatsapp.net"

describe("WhatsApp read receipts and typing indicators", () => {
  let socket: ReturnType<typeof createMockSocket>
  let adapter: WhatsAppAdapter
  let handler: ReturnType<typeof mock>

  beforeEach(() => {
    handler = mock(async () => "AI response")
    const setup = createAdapter({ socket: undefined, messageHandler: handler as any })
    adapter = setup.adapter
    socket = setup.socket
  })

  test("sends read receipt with correct message key", async () => {
    const msgKey = { remoteJid: JID, id: "MSG123", fromMe: false }
    await simulateMessage(adapter, handler as any, {
      message: { conversation: "hello" },
      key: { ...msgKey },
    })

    expect(socket.readMessages).toHaveBeenCalledTimes(1)
    const call = (socket.readMessages as any).mock.calls[0]
    expect(call[0]).toEqual([{ remoteJid: JID, id: "MSG123", fromMe: false }])
  })

  test("sends composing before AI processing and paused after", async () => {
    const order: string[] = []

    socket.sendPresenceUpdate = mock(async (type: string) => {
      order.push(type)
    }) as any

    handler = mock(async () => {
      order.push("handler")
      return "response"
    })

    await simulateMessage(adapter, handler as any, {
      message: { conversation: "hello" },
      key: { remoteJid: JID, id: "MSG1", fromMe: false },
    })

    expect(order).toEqual(["composing", "handler", "paused"])
  })

  test("sends paused even when handler throws", async () => {
    handler = mock(async () => {
      throw new Error("boom")
    })

    await simulateMessage(adapter, handler as any, {
      message: { conversation: "hello" },
      key: { remoteJid: JID, id: "MSG2", fromMe: false },
    })

    // paused should still be called
    const pausedCalls = (socket.sendPresenceUpdate as any).mock.calls.filter(
      (c: any[]) => c[0] === "paused",
    )
    expect(pausedCalls.length).toBe(1)
  })

  test("read receipt failure does not break message processing", async () => {
    socket.readMessages = mock(async () => {
      throw new Error("read receipt failed")
    }) as any

    handler = mock(async () => "still works")

    await simulateMessage(adapter, handler as any, {
      message: { conversation: "hello" },
      key: { remoteJid: JID, id: "MSG3", fromMe: false },
    })

    expect(handler).toHaveBeenCalledTimes(1)
    expect(socket.sendMessage).toHaveBeenCalledTimes(1)
  })

  test("presence failure does not break message processing", async () => {
    socket.sendPresenceUpdate = mock(async () => {
      throw new Error("presence failed")
    }) as any

    handler = mock(async () => "still works")

    await simulateMessage(adapter, handler as any, {
      message: { conversation: "hello" },
      key: { remoteJid: JID, id: "MSG4", fromMe: false },
    })

    expect(handler).toHaveBeenCalledTimes(1)
  })

  test("sends read receipt for command messages", async () => {
    // Use the pair token to trigger the pair command
    const setup = createAdapter({ pairToken: "SECRET" })
    const cmdAdapter = setup.adapter
    const cmdSocket = setup.socket

    // Patch in the same socket
    ;(cmdAdapter as any).connectionManager.getSocket = () => cmdSocket

    // Register a handler (needed for adapter to be started)
    ;(cmdAdapter as any).messageHandler = async () => "ok"

    await (cmdAdapter as any).handleMessage({
      message: { conversation: "/pair SECRET" },
      key: { remoteJid: JID, id: "CMD1", fromMe: false },
    })

    expect(cmdSocket.readMessages).toHaveBeenCalledTimes(1)
    const call = (cmdSocket.readMessages as any).mock.calls[0]
    expect(call[0]).toEqual([{ remoteJid: JID, id: "CMD1", fromMe: false }])
  })
})
