import { describe, test, expect, beforeEach } from "bun:test"
import { db } from "../../../src/store/db"
import {
  SQLiteMemoryRepository,
  SQLiteChannelRepository,
  SQLiteSessionRepository,
  SQLiteWhitelistRepository,
  SQLiteOutboxRepository,
  SQLiteHeartbeatRepository,
} from "../../../src/adapters/persistence/repositories"

function resetTables(): void {
  db.run("DELETE FROM heartbeat_tasks")
  db.run("DELETE FROM outbox")
  db.run("DELETE FROM whitelist")
  db.run("DELETE FROM memory")
  db.run("DELETE FROM kv")
}

describe("SQLite repositories", () => {
  beforeEach(() => {
    resetTables()
  })

  test("session repository stores and reads session ids", () => {
    const repo = new SQLiteSessionRepository()
    repo.saveSessionId("session:main", "abc")
    expect(repo.getSessionId("session:main")).toBe("abc")
    repo.deleteSession("session:main")
    expect(repo.getSessionId("session:main")).toBeUndefined()
    repo.close()
  })

  test("channel repository stores and parses last channel", () => {
    const repo = new SQLiteChannelRepository()
    repo.saveLastChannel("whatsapp", "123@s.whatsapp.net")
    expect(repo.getLastChannel()).toEqual({ channel: "whatsapp", userID: "123@s.whatsapp.net" })
    repo.saveLastChannel("telegram", "999")
    expect(repo.getLastChannel()).toEqual({ channel: "telegram", userID: "999" })
    repo.close()
  })

  test("memory repository deduplicates normalized facts", () => {
    const repo = new SQLiteMemoryRepository()
    expect(repo.append("User likes coffee", "test")).toBe(true)
    expect(repo.append("user   likes coffee", "test")).toBe(false)
    const all = repo.getAll()
    expect(all).toHaveLength(1)
    expect(repo.delete(all[0]!.id)).toBe(true)
    repo.close()
  })

  test("whitelist repository add/check/remove", () => {
    const repo = new SQLiteWhitelistRepository()
    expect(repo.addToWhitelist("whatsapp", "123@s.whatsapp.net")).toBe(true)
    expect(repo.addToWhitelist("whatsapp", "123@s.whatsapp.net")).toBe(false)
    expect(repo.isWhitelisted("whatsapp", "123@s.whatsapp.net")).toBe(true)
    expect(repo.removeFromWhitelist("whatsapp", "123@s.whatsapp.net")).toBe(true)
    repo.close()
  })

  test("outbox repository uses configured default max retries", () => {
    const repo = new SQLiteOutboxRepository(5)
    repo.enqueue("whatsapp", "123@s.whatsapp.net", "hello")

    const pending = repo.listPending("whatsapp")
    expect(pending).toHaveLength(1)
    expect(pending[0]!.maxRetries).toBe(5)

    const firstID = pending[0]!.id
    repo.markRetry(firstID, 1, new Date(Date.now() + 60_000).toISOString())
    repo.acknowledge(firstID)
    expect(repo.listPending("whatsapp")).toHaveLength(0)
    repo.close()
  })

  test("heartbeat repository lists and counts enabled tasks", () => {
    const repo = new SQLiteHeartbeatRepository()
    db.run("INSERT INTO heartbeat_tasks (task, enabled) VALUES (?, 1)", ["check inbox"])
    db.run("INSERT INTO heartbeat_tasks (task, enabled) VALUES (?, 0)", ["disabled task"])
    expect(repo.getTaskCount()).toBe(1)
    expect(repo.getTasks()).toEqual(["check inbox"])
    repo.close()
  })
})
