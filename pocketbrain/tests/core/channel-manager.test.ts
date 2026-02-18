import { describe, test, expect, vi } from "bun:test"
import pino from "pino"
import { ChannelManager } from "../../src/core/channel-manager"
import type { ChannelAdapter, MessageHandler } from "../../src/core/ports/channel-adapter"

function createAdapter(name: string): ChannelAdapter {
  return {
    name,
    start: vi.fn(async (_handler: MessageHandler) => {}),
    stop: vi.fn(async () => {}),
    send: vi.fn(async (_userID: string, _text: string) => {}),
  }
}

describe("ChannelManager", () => {
  test("starts and stops all registered adapters", async () => {
    const manager = new ChannelManager(pino({ enabled: false }))
    const whatsapp = createAdapter("whatsapp")
    const slack = createAdapter("slack")

    manager.register(whatsapp)
    manager.register(slack)

    const handler: MessageHandler = async (_userID, text) => text
    await manager.start(handler)

    expect(whatsapp.start).toHaveBeenCalledTimes(1)
    expect(slack.start).toHaveBeenCalledTimes(1)

    await manager.stop()
    expect(whatsapp.stop).toHaveBeenCalledTimes(1)
    expect(slack.stop).toHaveBeenCalledTimes(1)
  })

  test("routes send calls to target channel", async () => {
    const manager = new ChannelManager(pino({ enabled: false }))
    const whatsapp = createAdapter("whatsapp")
    manager.register(whatsapp)

    await manager.send("whatsapp", "123@s.whatsapp.net", "hello")
    expect(whatsapp.send).toHaveBeenCalledWith("123@s.whatsapp.net", "hello")
  })

  test("throws for unknown channel", async () => {
    const manager = new ChannelManager(pino({ enabled: false }))
    await expect(manager.send("unknown", "u", "msg")).rejects.toThrow("Unknown channel")
  })
})
