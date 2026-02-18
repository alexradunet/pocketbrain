import { describe, test, expect, vi, afterEach } from "bun:test"
import type { Logger } from "pino"
import type { AssistantCore } from "../../src/core/assistant"
import type { OutboxRepository } from "../../src/core/ports/outbox-repository"
import type { ChannelRepository } from "../../src/core/ports/channel-repository"
import { HeartbeatScheduler } from "../../src/scheduler/heartbeat"

const ONE_MINUTE_MS = 60_000

function createLoggerMock(): Logger {
  return {
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
    debug: vi.fn(),
  } as unknown as Logger
}

describe("HeartbeatScheduler", () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  test("schedules next run at base interval after success", async () => {
    vi.useFakeTimers()

    const assistant = {
      runHeartbeatTasks: vi.fn().mockResolvedValue("ok"),
    }
    const outboxRepository = { enqueue: vi.fn() }
    const channelRepository = { getLastChannel: vi.fn().mockReturnValue(null) }

    const scheduler = new HeartbeatScheduler(
      {
        intervalMinutes: 1,
        baseDelayMs: ONE_MINUTE_MS,
        maxDelayMs: 30 * ONE_MINUTE_MS,
        notifyAfterFailures: 3,
      },
      {
        assistant: assistant as unknown as AssistantCore,
        outboxRepository: outboxRepository as unknown as OutboxRepository,
        channelRepository: channelRepository as unknown as ChannelRepository,
        logger: createLoggerMock(),
      }
    )

    scheduler.start()
    vi.advanceTimersByTime(0)
    await Promise.resolve()
    expect(assistant.runHeartbeatTasks).toHaveBeenCalledTimes(1)

    vi.advanceTimersByTime(ONE_MINUTE_MS - 1)
    await Promise.resolve()
    expect(assistant.runHeartbeatTasks).toHaveBeenCalledTimes(1)

    vi.advanceTimersByTime(1)
    await Promise.resolve()
    expect(assistant.runHeartbeatTasks).toHaveBeenCalledTimes(2)

    scheduler.stop()
  })

  test("applies exponential backoff delay after failure", async () => {
    vi.useFakeTimers()

    const assistant = {
      runHeartbeatTasks: vi
        .fn()
        .mockRejectedValueOnce(new Error("fail once"))
        .mockResolvedValue("ok"),
    }
    const outboxRepository = { enqueue: vi.fn() }
    const channelRepository = { getLastChannel: vi.fn().mockReturnValue(null) }

    const scheduler = new HeartbeatScheduler(
      {
        intervalMinutes: 1,
        baseDelayMs: 120_000,
        maxDelayMs: 30 * ONE_MINUTE_MS,
        notifyAfterFailures: 3,
      },
      {
        assistant: assistant as unknown as AssistantCore,
        outboxRepository: outboxRepository as unknown as OutboxRepository,
        channelRepository: channelRepository as unknown as ChannelRepository,
        logger: createLoggerMock(),
      }
    )

    scheduler.start()
    vi.advanceTimersByTime(0)
    await Promise.resolve()
    expect(assistant.runHeartbeatTasks).toHaveBeenCalledTimes(1)

    vi.advanceTimersByTime(239_999)
    await Promise.resolve()
    expect(assistant.runHeartbeatTasks).toHaveBeenCalledTimes(1)

    vi.advanceTimersByTime(1)
    await Promise.resolve()
    expect(assistant.runHeartbeatTasks).toHaveBeenCalledTimes(2)

    scheduler.stop()
  })

  test("queues user notification after configured consecutive failures", async () => {
    vi.useFakeTimers()

    const assistant = {
      runHeartbeatTasks: vi
        .fn()
        .mockRejectedValueOnce(new Error("fail 1"))
        .mockRejectedValueOnce(new Error("fail 2")),
    }
    const outboxRepository = { enqueue: vi.fn() }
    const channelRepository = {
      getLastChannel: vi.fn().mockReturnValue({ channel: "whatsapp", userID: "123@s.whatsapp.net" }),
    }

    const scheduler = new HeartbeatScheduler(
      {
        intervalMinutes: 1,
        baseDelayMs: 60_000,
        maxDelayMs: 30 * ONE_MINUTE_MS,
        notifyAfterFailures: 2,
      },
      {
        assistant: assistant as unknown as AssistantCore,
        outboxRepository: outboxRepository as unknown as OutboxRepository,
        channelRepository: channelRepository as unknown as ChannelRepository,
        logger: createLoggerMock(),
      }
    )

    scheduler.start()
    vi.advanceTimersByTime(0)
    await Promise.resolve()
    vi.advanceTimersByTime(120_000)
    await Promise.resolve()

    expect(assistant.runHeartbeatTasks).toHaveBeenCalledTimes(2)
    expect(outboxRepository.enqueue).toHaveBeenCalledTimes(1)

    scheduler.stop()
  })
})
