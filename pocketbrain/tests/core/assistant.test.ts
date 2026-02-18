import { describe, test, expect } from "bun:test"
import pino from "pino"
import type { createOpencodeClient } from "@opencode-ai/sdk"
import { AssistantCore } from "../../src/core/assistant"
import { PromptBuilder } from "../../src/core/prompt-builder"
import { SessionManager } from "../../src/core/session-manager"
import type { MemoryRepository } from "../../src/core/ports/memory-repository"
import type { ChannelRepository } from "../../src/core/ports/channel-repository"
import type { HeartbeatRepository } from "../../src/core/ports/heartbeat-repository"
import type { SessionRepository } from "../../src/core/ports/session-repository"
import { RuntimeProvider } from "../../src/core/runtime-provider"

function createSessionManager(): SessionManager {
  const repository: SessionRepository = {
    getSessionId: () => undefined,
    saveSessionId: () => {},
    deleteSession: () => {},
  }

  return new SessionManager({ repository, logger: pino({ enabled: false }) })
}

describe("AssistantCore", () => {
  test("ask returns assistant text and tracks whatsapp last channel", async () => {
    let saved: { channel: string; userID: string } | null = null

    const fakeClient = {
      session: {
        prompt: async () => ({ data: { parts: [{ type: "text", text: "hello" }] } }),
        messages: async () => ({ data: [] }),
        create: async () => ({ data: { id: "ignored" } }),
      },
    } as unknown as ReturnType<typeof createOpencodeClient>

    const runtimeProvider = new RuntimeProvider({
      model: "provider/model",
      serverUrl: "http://127.0.0.1:4096",
      hostname: "127.0.0.1",
      port: 4096,
      logger: pino({ enabled: false }),
    })
    runtimeProvider.getClient = () => fakeClient

    const sessionManager = createSessionManager()
    sessionManager.getOrCreateMainSession = async () => "main-session"

    const memoryRepository: MemoryRepository = {
      append: () => true,
      readAll: () => "# Memory\n- prefers concise replies\n",
      delete: () => true,
      update: () => true,
      getAll: () => [],
    }

    const channelRepository: ChannelRepository = {
      saveLastChannel: (channel, userID) => {
        saved = { channel, userID }
      },
      getLastChannel: () => null,
    }

    const heartbeatRepository: HeartbeatRepository = {
      getTasks: () => [],
      addTask: () => {},
      removeTask: () => {},
      getTaskCount: () => 0,
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

    const response = await assistant.ask({
      channel: "whatsapp",
      userID: "123@s.whatsapp.net",
      text: "hi",
    })

    expect(response).toBe("hello")
    expect(saved).toEqual({ channel: "whatsapp", userID: "123@s.whatsapp.net" })
  })

  test("runHeartbeatTasks skips when no tasks exist", async () => {
    const fakeClient = {
      session: {
        prompt: async () => ({ data: { parts: [{ type: "text", text: "ok" }] } }),
        messages: async () => ({ data: [] }),
        create: async () => ({ data: { id: "ignored" } }),
      },
    } as unknown as ReturnType<typeof createOpencodeClient>

    const runtimeProvider = new RuntimeProvider({
      model: undefined,
      serverUrl: "http://127.0.0.1:4096",
      hostname: "127.0.0.1",
      port: 4096,
      logger: pino({ enabled: false }),
    })
    runtimeProvider.getClient = () => fakeClient

    const heartbeatRepository: HeartbeatRepository = {
      getTasks: () => [],
      addTask: () => {},
      removeTask: () => {},
      getTaskCount: () => 0,
    }

    const assistant = new AssistantCore({
      runtimeProvider,
      sessionManager: createSessionManager(),
      promptBuilder: new PromptBuilder({ heartbeatIntervalMinutes: 30 }),
      memoryRepository: {
        append: () => true,
        readAll: () => "",
        delete: () => true,
        update: () => true,
        getAll: () => [],
      },
      channelRepository: {
        saveLastChannel: () => {},
        getLastChannel: () => null,
      },
      heartbeatRepository,
      logger: pino({ enabled: false }),
    })

    const result = await assistant.runHeartbeatTasks()
    expect(result).toContain("no tasks found")
  })
})
