import { describe, test, expect, vi } from "bun:test"
import type { Logger } from "pino"
import type { OutboxRepository } from "../../../../src/core/ports/outbox-repository"
import { OutboxProcessor } from "../../../../src/adapters/channels/whatsapp/outbox-processor"

function createLoggerMock(): Logger {
  return {
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
    debug: vi.fn(),
  } as unknown as Logger
}

function createOutboxRepositoryMock(): OutboxRepository {
  return {
    enqueue: vi.fn(),
    listPending: vi.fn(() => []),
    acknowledge: vi.fn(),
    markRetry: vi.fn(),
  }
}

describe("OutboxProcessor", () => {
  test("acknowledges invalid JID errors", async () => {
    const repository = createOutboxRepositoryMock()
    const processor = new OutboxProcessor({
      outboxRepository: repository,
      logger: createLoggerMock(),
      retryBaseDelayMs: 1,
      sendMessage: async () => {
        throw new Error("Invalid WhatsApp ID format")
      },
    })

    await processor.process({
      id: 1,
      channel: "whatsapp",
      userID: "bad jid",
      text: "hello",
      retryCount: 0,
      maxRetries: 3,
      nextRetryAt: null,
    })

    expect(repository.acknowledge).toHaveBeenCalledWith(1)
    expect(repository.markRetry).not.toHaveBeenCalled()
  })

  test("acknowledges non-direct recipient errors", async () => {
    const repository = createOutboxRepositoryMock()
    const processor = new OutboxProcessor({
      outboxRepository: repository,
      logger: createLoggerMock(),
      retryBaseDelayMs: 1,
      sendMessage: async () => {
        throw new Error("Cannot send to group chats")
      },
    })

    await processor.process({
      id: 2,
      channel: "whatsapp",
      userID: "12345@g.us",
      text: "hello",
      retryCount: 0,
      maxRetries: 3,
      nextRetryAt: null,
    })

    expect(repository.acknowledge).toHaveBeenCalledWith(2)
    expect(repository.markRetry).not.toHaveBeenCalled()
  })

  test("acknowledges successful send", async () => {
    const repository = createOutboxRepositoryMock()
    const sendMessage = vi.fn(async () => {})
    const processor = new OutboxProcessor({
      outboxRepository: repository,
      logger: createLoggerMock(),
      retryBaseDelayMs: 1,
      sendMessage,
    })

    await processor.process({
      id: 3,
      channel: "whatsapp",
      userID: "123456789@s.whatsapp.net",
      text: "hello",
      retryCount: 0,
      maxRetries: 3,
      nextRetryAt: null,
    })

    expect(sendMessage).toHaveBeenCalledWith("123456789@s.whatsapp.net", "hello")
    expect(repository.acknowledge).toHaveBeenCalledWith(3)
  })

  test("marks retry when send fails and retries remain", async () => {
    const repository = createOutboxRepositoryMock()
    const processor = new OutboxProcessor({
      outboxRepository: repository,
      logger: createLoggerMock(),
      retryBaseDelayMs: 1,
      sendMessage: async () => {
        throw new Error("whatsapp socket unavailable")
      },
    })

    await processor.process({
      id: 4,
      channel: "whatsapp",
      userID: "123456789@s.whatsapp.net",
      text: "hello",
      retryCount: 0,
      maxRetries: 3,
      nextRetryAt: null,
    })

    expect(repository.markRetry).toHaveBeenCalledTimes(1)
    expect(repository.acknowledge).not.toHaveBeenCalled()
  })

  test("acknowledges when max retries are exceeded", async () => {
    const repository = createOutboxRepositoryMock()
    const processor = new OutboxProcessor({
      outboxRepository: repository,
      logger: createLoggerMock(),
      retryBaseDelayMs: 1,
      sendMessage: async () => {
        throw new Error("send failed")
      },
    })

    await processor.process({
      id: 5,
      channel: "whatsapp",
      userID: "123456789@s.whatsapp.net",
      text: "hello",
      retryCount: 2,
      maxRetries: 3,
      nextRetryAt: null,
    })

    expect(repository.acknowledge).toHaveBeenCalledWith(5)
    expect(repository.markRetry).not.toHaveBeenCalled()
  })
})
