import { afterEach, describe, expect, test, vi } from "bun:test"
import type { Logger } from "pino"
import { ConnectionManager } from "../../../../src/adapters/channels/whatsapp/connection-manager"

function createLoggerMock(): Logger {
  return {
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
    debug: vi.fn(),
  } as unknown as Logger
}

describe("ConnectionManager", () => {
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  test("clears pending reconnect timer on stop and allows a fresh reconnect schedule", async () => {
    vi.useFakeTimers()

    const manager = new ConnectionManager({
      authDir: ".test-auth",
      logger: createLoggerMock(),
    })

    const connectSpy = vi.spyOn(manager, "connect").mockResolvedValue({} as never)

    manager.scheduleReconnect(1000, "first schedule")
    manager.stop()

    const internalState = manager as unknown as { stopping: boolean }
    internalState.stopping = false

    manager.scheduleReconnect(50, "after restart")

    vi.advanceTimersByTime(60)
    await Promise.resolve()

    expect(connectSpy).toHaveBeenCalledTimes(1)

    vi.advanceTimersByTime(1000)
    await Promise.resolve()

    expect(connectSpy).toHaveBeenCalledTimes(1)
  })
})
