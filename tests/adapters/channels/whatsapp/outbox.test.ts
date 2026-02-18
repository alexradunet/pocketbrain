import { describe, test, expect, vi } from "bun:test"
import type { Logger } from "pino"
import type { WhitelistRepository } from "../../../../src/core/ports/whitelist-repository"
import type { OutboxRepository } from "../../../../src/core/ports/outbox-repository"
import type { MessageSender } from "../../../../src/core/services/message-sender"
import { WhatsAppAdapter } from "../../../../src/adapters/channels/whatsapp/adapter"

function createLoggerMock(): Logger {
  return {
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
    debug: vi.fn(),
  } as unknown as Logger
}

function createAdapterDeps(sendImpl?: (userID: string, text: string) => Promise<void>) {
  const whitelistRepository: WhitelistRepository = {
    isWhitelisted: () => true,
    addToWhitelist: () => true,
    removeFromWhitelist: () => true,
  }

  const outboxRepository = {
    enqueue: vi.fn(),
    listPending: vi.fn(() => []),
    acknowledge: vi.fn(),
    markRetry: vi.fn(),
  } as unknown as OutboxRepository

  const messageSender = {
    send: vi.fn(async (userID: string, text: string) => {
      if (sendImpl) {
        await sendImpl(userID, text)
      }
    }),
  } as unknown as MessageSender

  const adapter = new WhatsAppAdapter({
    authDir: ".data/test-whatsapp",
    logger: createLoggerMock(),
    whitelistRepository,
    outboxRepository,
    messageSender,
    pairToken: "secret",
    outboxRetryBaseDelayMs: 1,
  })

  return { adapter, outboxRepository, messageSender }
}

describe("WhatsAppAdapter outbox handling", () => {
  test("acknowledges outbox item with invalid jid", async () => {
    const { adapter, outboxRepository } = createAdapterDeps()
    ;(adapter as any).connectionManager = { isConnected: () => true, getSocket: () => ({}) }

    await (adapter as any).processOutboxItem({
      id: 1,
      userID: "bad jid",
      text: "hello",
      retryCount: 0,
      maxRetries: 3,
    })

    expect((outboxRepository.acknowledge as unknown as ReturnType<typeof vi.fn>)).toHaveBeenCalledWith(1)
  })

  test("acknowledges outbox item when target is non-direct chat", async () => {
    const { adapter, outboxRepository } = createAdapterDeps()
    ;(adapter as any).connectionManager = { isConnected: () => true, getSocket: () => ({}) }

    await (adapter as any).processOutboxItem({
      id: 2,
      userID: "12345@g.us",
      text: "hello",
      retryCount: 0,
      maxRetries: 3,
    })

    expect((outboxRepository.acknowledge as unknown as ReturnType<typeof vi.fn>)).toHaveBeenCalledWith(2)
  })

  test("sends and acknowledges outbox item on success", async () => {
    const { adapter, outboxRepository, messageSender } = createAdapterDeps()
    const socket = {
      sendMessage: vi.fn(async () => {}),
    }
    ;(adapter as any).connectionManager = {
      isConnected: () => true,
      getSocket: () => socket,
    }

    await (adapter as any).processOutboxItem({
      id: 3,
      userID: "123456789@s.whatsapp.net",
      text: "hello",
      retryCount: 0,
      maxRetries: 3,
    })

    expect((messageSender.send as unknown as ReturnType<typeof vi.fn>)).toHaveBeenCalled()
    expect((outboxRepository.acknowledge as unknown as ReturnType<typeof vi.fn>)).toHaveBeenCalledWith(3)
  })

  test("marks retry when send fails and retries remain", async () => {
    const { adapter, outboxRepository } = createAdapterDeps(async () => {
      throw new Error("send failed")
    })
    const socket = {
      sendMessage: vi.fn(async () => {}),
    }
    ;(adapter as any).connectionManager = {
      isConnected: () => true,
      getSocket: () => socket,
    }

    await (adapter as any).processOutboxItem({
      id: 4,
      userID: "123456789@s.whatsapp.net",
      text: "hello",
      retryCount: 0,
      maxRetries: 3,
    })

    expect((outboxRepository.markRetry as unknown as ReturnType<typeof vi.fn>)).toHaveBeenCalledTimes(1)
    expect((outboxRepository.acknowledge as unknown as ReturnType<typeof vi.fn>)).not.toHaveBeenCalled()
  })

  test("acknowledges when max retries are exceeded", async () => {
    const { adapter, outboxRepository } = createAdapterDeps(async () => {
      throw new Error("send failed")
    })
    const socket = {
      sendMessage: vi.fn(async () => {}),
    }
    ;(adapter as any).connectionManager = {
      isConnected: () => true,
      getSocket: () => socket,
    }

    await (adapter as any).processOutboxItem({
      id: 5,
      userID: "123456789@s.whatsapp.net",
      text: "hello",
      retryCount: 2,
      maxRetries: 3,
    })

    expect((outboxRepository.acknowledge as unknown as ReturnType<typeof vi.fn>)).toHaveBeenCalledWith(5)
  })

  test("marks retry when socket is unavailable", async () => {
    const { adapter, outboxRepository } = createAdapterDeps()
    ;(adapter as any).connectionManager = {
      isConnected: () => true,
      getSocket: () => undefined,
    }

    await (adapter as any).processOutboxItem({
      id: 6,
      userID: "123456789@s.whatsapp.net",
      text: "hello",
      retryCount: 0,
      maxRetries: 3,
    })

    expect((outboxRepository.markRetry as unknown as ReturnType<typeof vi.fn>)).toHaveBeenCalledTimes(1)
  })
})
