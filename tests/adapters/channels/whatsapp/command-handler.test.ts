import { afterEach, describe, test, expect, vi } from "bun:test"
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
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

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

  test("shows pairing-disabled guidance for unwhitelisted users when token is not configured", () => {
    const handler = new CommandHandler({
      pairToken: undefined,
      logger: createLoggerMock(),
    })

    const result = handler.handle({
      jid: "123@s.whatsapp.net",
      text: "hello",
      isWhitelisted: false,
    })

    expect(result.handled).toBe(true)
    expect(result.action).toBeUndefined()
    expect(result.response).toContain("Pairing is disabled")
    expect(result.response).not.toContain("/pair <token>")
  })

  test("treats /pairing as a normal message for whitelisted users", () => {
    const handler = new CommandHandler({
      pairToken: "secret",
      logger: createLoggerMock(),
    })

    const result = handler.handle({
      jid: "123@s.whatsapp.net",
      text: "/pairing secret",
      isWhitelisted: true,
    })

    expect(result.handled).toBe(false)
    expect(result.action).toBeUndefined()
    expect(result.response).toBeUndefined()
  })

  test("temporarily blocks repeated invalid /pair attempts", () => {
    let now = 0
    vi.spyOn(Date, "now").mockImplementation(() => now)

    const handler = new CommandHandler({
      pairToken: "secret",
      logger: createLoggerMock(),
      pairMaxFailures: 3,
      pairFailureWindowMs: 60_000,
      pairBlockDurationMs: 120_000,
    })

    const jid = "123@s.whatsapp.net"

    const first = handler.handle({ jid, text: "/pair wrong", isWhitelisted: false })
    const second = handler.handle({ jid, text: "/pair wrong", isWhitelisted: false })
    const third = handler.handle({ jid, text: "/pair wrong", isWhitelisted: false })

    expect(first.response).toBe("Invalid pairing token.")
    expect(second.response).toBe("Invalid pairing token.")
    expect(third.response).toContain("Too many failed pairing attempts")

    const blocked = handler.handle({ jid, text: "/pair secret", isWhitelisted: false })
    expect(blocked.response).toContain("Too many failed pairing attempts")

    now = 121_000
    const afterCooldown = handler.handle({ jid, text: "/pair secret", isWhitelisted: false })
    expect(afterCooldown.action).toBe("pair")
  })

  test("resets invalid attempt window after configured interval", () => {
    let now = 0
    vi.spyOn(Date, "now").mockImplementation(() => now)

    const handler = new CommandHandler({
      pairToken: "secret",
      logger: createLoggerMock(),
      pairMaxFailures: 3,
      pairFailureWindowMs: 60_000,
      pairBlockDurationMs: 300_000,
    })

    const jid = "123@s.whatsapp.net"
    const first = handler.handle({ jid, text: "/pair wrong", isWhitelisted: false })
    expect(first.response).toBe("Invalid pairing token.")

    now = 61_000
    const second = handler.handle({ jid, text: "/pair wrong", isWhitelisted: false })
    const third = handler.handle({ jid, text: "/pair wrong", isWhitelisted: false })

    expect(second.response).toBe("Invalid pairing token.")
    expect(third.response).toBe("Invalid pairing token.")
  })
})
