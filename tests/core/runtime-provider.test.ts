import { describe, test, expect } from "bun:test"
import pino from "pino"
import { RuntimeProvider } from "../../src/core/runtime-provider"

describe("RuntimeProvider", () => {
  test("buildModelConfig parses provider/model", () => {
    const runtime = new RuntimeProvider({
      model: "openai/gpt-5",
      serverUrl: undefined,
      hostname: "127.0.0.1",
      port: 4096,
      logger: pino({ enabled: false }),
    })

    expect(runtime.buildModelConfig()).toEqual({ providerID: "openai", modelID: "gpt-5" })
  })

  test("buildModelConfig returns undefined for invalid model", () => {
    const runtime = new RuntimeProvider({
      model: "invalid",
      serverUrl: undefined,
      hostname: "127.0.0.1",
      port: 4096,
      logger: pino({ enabled: false }),
    })

    expect(runtime.buildModelConfig()).toBeUndefined()
  })

  test("init with serverUrl creates reusable client", async () => {
    const runtime = new RuntimeProvider({
      model: undefined,
      serverUrl: "http://127.0.0.1:4096",
      hostname: "127.0.0.1",
      port: 4096,
      logger: pino({ enabled: false }),
    })

    const first = await runtime.init()
    const second = await runtime.init()

    expect(first.client).toBeDefined()
    expect(second).toBe(first)
    expect(runtime.getClient()).toBeDefined()

    await runtime.close()
    expect(runtime.getClient()).toBeUndefined()
  })
})
