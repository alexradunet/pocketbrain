import { describe, test, expect, vi } from "bun:test"
import type { Logger } from "pino"
import { MessageChunker } from "../../src/core/services/message-chunker"
import { MessageSender } from "../../src/core/services/message-sender"
import type { RateLimiter } from "../../src/adapters/channels/rate-limiter"

function createLoggerMock(): Logger {
  return {
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
    debug: vi.fn(),
  } as unknown as Logger
}

describe("MessageSender", () => {
  test("sends chunked message in order after throttling", async () => {
    const logger = createLoggerMock()
    const rateLimiter = {
      throttle: vi.fn(async (_userID: string) => {}),
    } as unknown as RateLimiter

    const sender = new MessageSender({
      chunker: new MessageChunker({ maxLength: 5 }),
      rateLimiter,
      chunkDelayMs: 0,
      logger,
    })

    const sent: string[] = []
    await sender.send("user-1", "hello world", async (chunk) => {
      sent.push(chunk)
    })

    expect(rateLimiter.throttle).toHaveBeenCalledWith("user-1")
    expect(sent).toEqual(["hello", "world"])
  })

  test("warns and skips empty text payload", async () => {
    const logger = createLoggerMock()
    const rateLimiter = {
      throttle: vi.fn(async (_userID: string) => {}),
    } as unknown as RateLimiter

    const sender = new MessageSender({
      chunker: new MessageChunker({ maxLength: 10 }),
      rateLimiter,
      chunkDelayMs: 0,
      logger,
    })

    const sendFn = vi.fn(async (_chunk: string) => {})
    await sender.send("user-2", "   ", sendFn)

    expect(sendFn).toHaveBeenCalledTimes(0)
    expect(logger.warn).toHaveBeenCalledTimes(1)
  })
})
