/**
 * SQLite Session Repository Implementation
 * Implements SessionRepository port using bun:sqlite
 */

import type { SessionRepository } from "../../../core/ports/session-repository"
import { db } from "../../../store/db"

interface KvRow {
  value: string
}

export class SQLiteSessionRepository implements SessionRepository {
  private readonly stmtKvGet: ReturnType<typeof db.prepare>
  private readonly stmtKvSet: ReturnType<typeof db.prepare>
  private readonly stmtKvDelete: ReturnType<typeof db.prepare>

  constructor() {
    this.stmtKvGet = db.prepare<KvRow, [string]>("SELECT value FROM kv WHERE key = ?")
    this.stmtKvSet = db.prepare(
      "INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value"
    )
    this.stmtKvDelete = db.prepare("DELETE FROM kv WHERE key = ?")
  }

  getSessionId(key: string): string | undefined {
    const row = this.stmtKvGet.get(key)
    if (!row) return undefined
    return (row as KvRow).value
  }

  saveSessionId(key: string, sessionId: string): void {
    this.stmtKvSet.run(key, sessionId)
  }

  deleteSession(key: string): void {
    this.stmtKvDelete.run(key)
  }

  /**
   * Finalize all prepared statements
   * Call this when shutting down to release resources
   */
  close(): void {
    this.stmtKvGet.finalize()
    this.stmtKvSet.finalize()
    this.stmtKvDelete.finalize()
  }
}
