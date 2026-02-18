import { describe, test, expect, vi } from "bun:test"
import pino from "pino"
import type { createOpencodeClient } from "@opencode-ai/sdk"
import { AssistantCore } from "../../src/core/assistant"
import { RuntimeProvider } from "../../src/core/runtime-provider"
import { SessionManager } from "../../src/core/session-manager"
import { PromptBuilder } from "../../src/core/prompt-builder"
import type { SessionRepository } from "../../src/core/ports/session-repository"
import type { MemoryRepository } from "../../src/core/ports/memory-repository"
import type { ChannelRepository } from "../../src/core/ports/channel-repository"
import type { HeartbeatRepository } from "../../src/core/ports/heartbeat-repository"

function createSessionRepository(): SessionRepository {
  const store = new Map<string, string>()
  return {
    getSessionId: (key) => store.get(key),
    saveSessionId: (key, value) => {
      store.set(key, value)
    },
    deleteSession: (key) => {
      store.delete(key)
    },
  }
}

describe("AssistantCore heartbeat process", () => {
  test("runs heartbeat tasks and injects summary into main session", async () => {
    const promptSpy = vi
      .fn()
      .mockResolvedValueOnce({ data: { parts: [{ type: "text", text: "summary output" }] } })
      .mockResolvedValueOnce({ data: { parts: [] } })
      .mockResolvedValueOnce({ data: { parts: [] } })

    const createSpy = vi
      .fn()
      .mockResolvedValueOnce({ data: { id: "heartbeat-session" } })
      .mockResolvedValueOnce({ data: { id: "main-session" } })

    const messagesSpy = vi.fn().mockResolvedValue({ data: [] })

    const fakeClient = {
      session: {
        prompt: promptSpy,
        create: createSpy,
        messages: messagesSpy,
      },
    } as unknown as ReturnType<typeof createOpencodeClient>

    const runtimeProvider = new RuntimeProvider({
      model: "openai/gpt-5",
      serverUrl: "http://127.0.0.1:4096",
      hostname: "127.0.0.1",
      port: 4096,
      logger: pino({ enabled: false }),
    })
    runtimeProvider.getClient = () => fakeClient

    const sessionManager = new SessionManager({
      repository: createSessionRepository(),
      logger: pino({ enabled: false }),
    })

    const memoryRepository: MemoryRepository = {
      append: () => true,
      readAll: () => "# Memory\n- likes concise updates\n",
      delete: () => true,
      update: () => true,
      getAll: () => [],
    }

    const channelRepository: ChannelRepository = {
      saveLastChannel: () => {},
      getLastChannel: () => null,
    }

    const heartbeatRepository: HeartbeatRepository = {
      getTasks: () => ["Check inbox"],
      addTask: () => {},
      removeTask: () => {},
      getTaskCount: () => 1,
    }

    const assistant = new AssistantCore({
      runtimeProvider,
      sessionManager,
      promptBuilder: new PromptBuilder({ heartbeatIntervalMinutes: 30 }),
      memoryRepository,
      channelRepository,
      heartbeatRepository,
      logger: pino({ enabled: false }),
    })

    const result = await assistant.runHeartbeatTasks()

    expect(result).toContain("Heartbeat completed")
    expect(createSpy).toHaveBeenCalledTimes(2)
    expect(messagesSpy).toHaveBeenCalledTimes(1)
    expect(promptSpy).toHaveBeenCalledTimes(3)

    const firstPrompt = promptSpy.mock.calls[0]?.[0]
    const secondPrompt = promptSpy.mock.calls[1]?.[0]
    const thirdPrompt = promptSpy.mock.calls[2]?.[0]

    expect(firstPrompt?.body?.parts?.[0]?.text).toContain("Task list")
    expect(secondPrompt?.body?.parts?.[0]?.text).toContain("Heartbeat summary")
    expect(thirdPrompt?.body?.parts?.[0]?.text).toContain("proactively informed")
  })
})
