import { describe, test, expect, vi } from "bun:test"
import type { Logger } from "pino"
import { CommandHandler } from "../../../../src/adapters/channels/whatsapp/command-handler"

function createLoggerMock(): Logger {
  return {
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
    debug: vi.fn(),
  } as unknown as Logger
}

describe("CommandHandler", () => {
  test("returns new_session action for /new", () => {
    const handler = new CommandHandler({
      pairToken: "secret",
      logger: createLoggerMock(),
    })

    const result = handler.handle({
      jid: "123@s.whatsapp.net",
      text: "/new",
      isWhitelisted: true,
    })

    expect(result.handled).toBe(true)
    expect(result.action).toBe("new_session")
    expect(result.response).toBe("Starting a new conversation session...")
  })

  test("blocks non-pair commands when user is not whitelisted", () => {
    const handler = new CommandHandler({
      pairToken: "secret",
      logger: createLoggerMock(),
    })

    const result = handler.handle({
      jid: "123@s.whatsapp.net",
      text: "/new",
      isWhitelisted: false,
    })

    expect(result.handled).toBe(true)
    expect(result.action).toBeUndefined()
    expect(result.response).toContain("Access restricted")
  })

  test("returns remember usage when payload is missing", () => {
    const handler = new CommandHandler({
      pairToken: "secret",
      logger: createLoggerMock(),
    })

    const result = handler.handle({
      jid: "123@s.whatsapp.net",
      text: "/remember",
      isWhitelisted: true,
    })

    expect(result.handled).toBe(true)
    expect(result.response).toBe("Usage: /remember <text>")
    expect(result.action).toBeUndefined()
  })
})
