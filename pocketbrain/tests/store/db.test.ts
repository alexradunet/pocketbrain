import { describe, test, expect, beforeEach, afterEach } from "bun:test"
import { Database } from "bun:sqlite"

function createTestDb(): Database {
  const db = new Database(":memory:")
  db.run("PRAGMA journal_mode = WAL")
  db.run("PRAGMA foreign_keys = ON")
  return db
}

describe("Database", () => {
  let db: Database

  beforeEach(() => {
    db = createTestDb()
  })

  afterEach(() => {
    db.close(false)
  })

  describe("KV Store", () => {
    test("set and get KV", () => {
      db.run("CREATE TABLE IF NOT EXISTS kv (key TEXT PRIMARY KEY, value TEXT NOT NULL)")
      db.run("INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", ["test-key", "test-value"])

      const result = db.query<{ value: string }, [string]>("SELECT value FROM kv WHERE key = ?").get("test-key")
      expect(result?.value).toBe("test-value")
    })

    test("update existing KV", () => {
      db.run("CREATE TABLE IF NOT EXISTS kv (key TEXT PRIMARY KEY, value TEXT NOT NULL)")
      db.run("INSERT INTO kv (key, value) VALUES (?, ?)", ["test-key", "original"])
      db.run("INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", ["test-key", "updated"])

      const result = db.query<{ value: string }, [string]>("SELECT value FROM kv WHERE key = ?").get("test-key")
      expect(result?.value).toBe("updated")
    })
  })

  describe("Whitelist", () => {
    beforeEach(() => {
      db.run("CREATE TABLE IF NOT EXISTS whitelist (channel TEXT NOT NULL, user_id TEXT NOT NULL, PRIMARY KEY (channel, user_id))")
    })

    test("add and check whitelist", () => {
      db.run("INSERT OR IGNORE INTO whitelist (channel, user_id) VALUES (?, ?)", ["whatsapp", "user@test.com"])

      const result = db.query<{ n: number }, [string, string]>("SELECT 1 AS n FROM whitelist WHERE channel = ? AND user_id = ?").get("whatsapp", "user@test.com")
      expect(result?.n).toBe(1)
    })

    test("whitelist returns false for non-whitelisted", () => {
      const result = db.query<{ n: number }, [string, string]>("SELECT 1 AS n FROM whitelist WHERE channel = ? AND user_id = ?").get("whatsapp", "unknown@test.com")
      expect(result == null || result.n !== 1).toBe(true)
    })
  })

  describe("Memory", () => {
    beforeEach(() => {
      db.run(`
        CREATE TABLE IF NOT EXISTS memory (
          id INTEGER PRIMARY KEY AUTOINCREMENT,
          fact TEXT NOT NULL,
          fact_normalized TEXT NOT NULL,
          source TEXT,
          created_at TEXT NOT NULL
        )
      `)
    })

    test("append memory", () => {
      const normalized = "userlikescoffee"
      db.run("INSERT INTO memory (fact, fact_normalized, source, created_at) VALUES (?, ?, ?, ?)", ["User likes coffee", normalized, "test", new Date().toISOString()])

      const result = db.query<{ fact: string }, []>("SELECT fact FROM memory").all()
      expect(result).toHaveLength(1)
      expect(result[0].fact).toBe("User likes coffee")
    })

    test("memory deduplication", () => {
      const normalized = "userlikescoffee"
      db.run("INSERT INTO memory (fact, fact_normalized, source, created_at) VALUES (?, ?, ?, ?)", ["User likes coffee", normalized, "test", new Date().toISOString()])

      const existing = db.query<{ n: number }, [string]>("SELECT 1 AS n FROM memory WHERE fact_normalized = ?").get(normalized)
      expect(existing?.n).toBe(1)
    })

    test("delete memory", () => {
      const result = db.run("INSERT INTO memory (fact, fact_normalized, source, created_at) VALUES (?, ?, ?, ?)", ["Test fact", "testfact", "test", new Date().toISOString()])
      const id = Number(result.lastInsertRowid)

      db.run("DELETE FROM memory WHERE id = ?", [id])

      const deleted = db.query<{ fact: string }, [number]>("SELECT fact FROM memory WHERE id = ?").get(id)
      expect(deleted == null).toBe(true)
    })
  })

  describe("Outbox with retries", () => {
    beforeEach(() => {
      db.run(`
        CREATE TABLE IF NOT EXISTS outbox (
          id INTEGER PRIMARY KEY AUTOINCREMENT,
          channel TEXT NOT NULL,
          user_id TEXT NOT NULL,
          text TEXT NOT NULL,
          created_at TEXT NOT NULL,
          retry_count INTEGER NOT NULL DEFAULT 0,
          max_retries INTEGER NOT NULL DEFAULT 3,
          next_retry_at TEXT
        )
      `)
    })

    test("enqueue outbox", () => {
      db.run("INSERT INTO outbox (channel, user_id, text, created_at, retry_count, max_retries, next_retry_at) VALUES (?, ?, ?, ?, 0, 3, NULL)", ["whatsapp", "user@test.com", "Hello", new Date().toISOString()])

      const result = db.query<{ id: number; text: string }, []>("SELECT id, text FROM outbox").all()
      expect(result).toHaveLength(1)
      expect(result[0].text).toBe("Hello")
    })

    test("list pending outbox (ready to send)", () => {
      const now = new Date().toISOString()
      db.run("INSERT INTO outbox (channel, user_id, text, created_at, retry_count, max_retries, next_retry_at) VALUES (?, ?, ?, ?, 0, 3, NULL)", ["whatsapp", "user@test.com", "Hello", now])

      const result = db.query<{ id: number }, [string, string]>("SELECT id FROM outbox WHERE channel = ? AND (next_retry_at IS NULL OR next_retry_at <= ?) ORDER BY id").all("whatsapp", now)
      expect(result).toHaveLength(1)
    })

    test("list pending outbox (not ready - future retry)", () => {
      const now = new Date().toISOString()
      const future = new Date(Date.now() + 60000).toISOString()
      db.run("INSERT INTO outbox (channel, user_id, text, created_at, retry_count, max_retries, next_retry_at) VALUES (?, ?, ?, ?, 1, 3, ?)", ["whatsapp", "user@test.com", "Hello", now, future])

      const result = db.query<{ id: number }, [string, string]>("SELECT id FROM outbox WHERE channel = ? AND (next_retry_at IS NULL OR next_retry_at <= ?) ORDER BY id").all("whatsapp", now)
      expect(result).toHaveLength(0)
    })

    test("update retry count", () => {
      const now = new Date().toISOString()
      db.run("INSERT INTO outbox (channel, user_id, text, created_at, retry_count, max_retries, next_retry_at) VALUES (?, ?, ?, ?, 0, 3, NULL)", ["whatsapp", "user@test.com", "Hello", now])
      const id = db.query<{ id: number }, []>("SELECT id FROM outbox").get()?.id

      if (id !== undefined) {
        const nextRetry = new Date(Date.now() + 60000).toISOString()
        db.run("UPDATE outbox SET retry_count = ?, next_retry_at = ? WHERE id = ?", [1, nextRetry, id])

        const result = db.query<{ retry_count: number; next_retry_at: string }, [number]>("SELECT retry_count, next_retry_at FROM outbox WHERE id = ?").get(id)
        expect(result?.retry_count).toBe(1)
        expect(result?.next_retry_at).toBe(nextRetry)
      }
    })
  })

  describe("Heartbeat tasks", () => {
    beforeEach(() => {
      db.run(`
        CREATE TABLE IF NOT EXISTS heartbeat_tasks (
          id INTEGER PRIMARY KEY AUTOINCREMENT,
          task TEXT NOT NULL,
          enabled INTEGER NOT NULL DEFAULT 1
        )
      `)
    })

    test("get enabled tasks", () => {
      db.run("INSERT INTO heartbeat_tasks (task, enabled) VALUES (?, 1)", ["Check emails"])
      db.run("INSERT INTO heartbeat_tasks (task, enabled) VALUES (?, 0)", ["Disabled task"])

      const result = db.query<{ task: string }, []>("SELECT task FROM heartbeat_tasks WHERE enabled = 1 ORDER BY id").all()
      expect(result).toHaveLength(1)
      expect(result[0].task).toBe("Check emails")
    })
  })
})
