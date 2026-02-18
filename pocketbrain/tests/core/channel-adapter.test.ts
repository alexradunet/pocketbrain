import { describe, test, expect } from "bun:test"
import type { ChannelAdapter, ChannelAuthOptions, MessageHandler } from "../../src/core/ports/channel-adapter"

class MockAdapter implements ChannelAdapter {
  readonly name = "mock"
  private handler?: MessageHandler
  private started = false
  private stopped = false
  private sentMessages: Array<{ userID: string; text: string }> = []

  async start(handler: MessageHandler): Promise<void> {
    this.handler = handler
    this.started = true
  }

  async stop(): Promise<void> {
    this.stopped = true
  }

  async send(userID: string, text: string): Promise<void> {
    this.sentMessages.push({ userID, text })
  }

  isStarted(): boolean {
    return this.started
  }

  isStopped(): boolean {
    return this.stopped
  }

  getSentMessages(): Array<{ userID: string; text: string }> {
    return this.sentMessages
  }

  getHandler(): MessageHandler | undefined {
    return this.handler
  }
}

describe("ChannelAdapter Interface", () => {
  test("adapter should have name property", () => {
    const adapter = new MockAdapter()
    expect(adapter.name).toBe("mock")
  })

  test("adapter should start and stop", async () => {
    const adapter = new MockAdapter()
    const handler: MessageHandler = async (userID, text) => `Echo: ${text}`

    await adapter.start(handler)
    expect(adapter.isStarted()).toBe(true)
    expect(adapter.getHandler()).toBeDefined()

    await adapter.stop()
    expect(adapter.isStopped()).toBe(true)
  })

  test("adapter should send messages", async () => {
    const adapter = new MockAdapter()
    await adapter.start(async (userID, text) => "response")

    await adapter.send("user@test.com", "Hello")
    const messages = adapter.getSentMessages()

    expect(messages).toHaveLength(1)
    expect(messages[0].userID).toBe("user@test.com")
    expect(messages[0].text).toBe("Hello")
  })

  test("handler should be callable", async () => {
    const adapter = new MockAdapter()
    const handler: MessageHandler = async (userID, text) => `Response to: ${text}`

    await adapter.start(handler)

    const response = await adapter.getHandler()!("user@test.com", "Hello")
    expect(response).toBe("Response to: Hello")
  })
})

describe("ChannelAuthOptions", () => {
  test("should accept required options", () => {
    const options: ChannelAuthOptions = {
      authDir: "/tmp/auth",
      logger: {
        info: () => {},
        warn: () => {},
        error: () => {},
      } as any,
    }

    expect(options.authDir).toBe("/tmp/auth")
  })
})
