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
        baseDelayMs: 1,
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

  test("keeps normal cadence after a failed run", async () => {
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
        baseDelayMs: 1,
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
    await Promise.resolve()
    expect(assistant.runHeartbeatTasks).toHaveBeenCalled()

    vi.advanceTimersByTime(ONE_MINUTE_MS + 10)
    await Promise.resolve()
    await Promise.resolve()
    expect(assistant.runHeartbeatTasks).toHaveBeenCalledTimes(3)

    scheduler.stop()
  })

  test("queues user notification after configured consecutive failures", async () => {
    vi.useFakeTimers()

    const assistant = {
      runHeartbeatTasks: vi
        .fn()
        .mockRejectedValue(new Error("fail")),
    }
    const outboxRepository = { enqueue: vi.fn() }
    const channelRepository = {
      getLastChannel: vi.fn().mockReturnValue({ channel: "whatsapp", userID: "123@s.whatsapp.net" }),
    }

    const scheduler = new HeartbeatScheduler(
      {
        intervalMinutes: 1,
        baseDelayMs: 1,
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
    await Promise.resolve()
    vi.advanceTimersByTime(ONE_MINUTE_MS + 10)
    await Promise.resolve()
    await Promise.resolve()
    vi.advanceTimersByTime(ONE_MINUTE_MS + 10)
    await Promise.resolve()
    await Promise.resolve()

    expect(assistant.runHeartbeatTasks.mock.calls.length).toBeGreaterThanOrEqual(6)
    expect(outboxRepository.enqueue.mock.calls.length).toBeGreaterThanOrEqual(1)

    scheduler.stop()
  })

  test("sends only one notification per failure incident until recovery", async () => {
    vi.useFakeTimers()

    const assistant = {
      runHeartbeatTasks: vi.fn().mockRejectedValue(new Error("always fail")),
    }
    const outboxRepository = { enqueue: vi.fn() }
    const channelRepository = {
      getLastChannel: vi.fn().mockReturnValue({ channel: "whatsapp", userID: "123@s.whatsapp.net" }),
    }

    const scheduler = new HeartbeatScheduler(
      {
        intervalMinutes: 1,
        baseDelayMs: 1,
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
    vi.advanceTimersByTime(10_000)
    await Promise.resolve()
    await Promise.resolve()

    vi.advanceTimersByTime(70_000)
    await Promise.resolve()
    await Promise.resolve()

    vi.advanceTimersByTime(70_000)
    await Promise.resolve()
    await Promise.resolve()

    expect(outboxRepository.enqueue).toHaveBeenCalledTimes(1)

    scheduler.stop()
  })

  test("sends a new notification after recovery and another failure streak", async () => {
    vi.useFakeTimers()

    let call = 0
    const assistant = {
      runHeartbeatTasks: vi.fn().mockImplementation(async () => {
        call += 1
        if (call <= 6 || (call >= 8 && call <= 13)) {
          throw new Error("fail")
        }
        return "ok"
      }),
    }

    const outboxRepository = { enqueue: vi.fn() }
    const channelRepository = {
      getLastChannel: vi.fn().mockReturnValue({ channel: "whatsapp", userID: "123@s.whatsapp.net" }),
    }

    const scheduler = new HeartbeatScheduler(
      {
        intervalMinutes: 1,
        baseDelayMs: 1,
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

    vi.advanceTimersByTime(10_000)
    await Promise.resolve()
    await Promise.resolve()

    vi.advanceTimersByTime(70_000)
    await Promise.resolve()
    await Promise.resolve()

    vi.advanceTimersByTime(70_000)
    await Promise.resolve()
    await Promise.resolve()

    vi.advanceTimersByTime(70_000)
    await Promise.resolve()
    await Promise.resolve()

    vi.advanceTimersByTime(70_000)
    await Promise.resolve()
    await Promise.resolve()

    expect(outboxRepository.enqueue).toHaveBeenCalledTimes(2)

    scheduler.stop()
  })
})
