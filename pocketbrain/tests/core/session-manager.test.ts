import { describe, test, expect, vi } from "bun:test"
import pino from "pino"
import type { createOpencodeClient } from "@opencode-ai/sdk"
import { SessionManager } from "../../src/core/session-manager"
import type { SessionRepository } from "../../src/core/ports/session-repository"

function createRepository(initial: Record<string, string> = {}): SessionRepository {
  const store = new Map<string, string>(Object.entries(initial))
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

describe("SessionManager", () => {
  test("returns existing session without creating a new one", async () => {
    const repository = createRepository({ "session:main": "existing-main" })
    const createSpy = vi.fn().mockResolvedValue({ data: { id: "new-id" } })
    const client = {
      session: {
        create: createSpy,
      },
    } as unknown as ReturnType<typeof createOpencodeClient>

    const manager = new SessionManager({ repository, logger: pino({ enabled: false }) })
    const id = await manager.getOrCreateMainSession(client)

    expect(id).toBe("existing-main")
    expect(createSpy).toHaveBeenCalledTimes(0)
  })

  test("creates and persists new session id when missing", async () => {
    const repository = createRepository()
    const createSpy = vi.fn().mockResolvedValue({ data: { id: "created-main" } })
    const client = {
      session: {
        create: createSpy,
      },
    } as unknown as ReturnType<typeof createOpencodeClient>

    const manager = new SessionManager({ repository, logger: pino({ enabled: false }) })
    const id = await manager.getOrCreateMainSession(client)

    expect(id).toBe("created-main")
    expect(repository.getSessionId("session:main")).toBe("created-main")
    expect(createSpy).toHaveBeenCalledTimes(1)
  })

  test("throws on invalid create response", async () => {
    const repository = createRepository()
    const client = {
      session: {
        create: vi.fn().mockResolvedValue({ data: {} }),
      },
    } as unknown as ReturnType<typeof createOpencodeClient>

    const manager = new SessionManager({ repository, logger: pino({ enabled: false }) })

    await expect(manager.getOrCreateHeartbeatSession(client)).rejects.toThrow("missing id")
  })
})
