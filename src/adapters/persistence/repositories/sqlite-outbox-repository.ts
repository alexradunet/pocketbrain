/**
 * SQLite Outbox Repository Implementation
 * Implements OutboxRepository port using bun:sqlite
 */

import type { OutboxRepository, OutboxMessage } from "../../../core/ports/outbox-repository"
import { db } from "../../../store/db"

interface OutboxRow {
  id: number
  channel: string
  user_id: string
  text: string
  retry_count: number
  max_retries: number
  next_retry_at: string | null
}

export class SQLiteOutboxRepository implements OutboxRepository {
  private readonly defaultMaxRetries: number
  private readonly stmtInsert: ReturnType<typeof db.prepare>
  private readonly stmtList: ReturnType<typeof db.prepare>
  private readonly stmtDelete: ReturnType<typeof db.prepare>
  private readonly stmtUpdateRetry: ReturnType<typeof db.prepare>

  constructor(defaultMaxRetries = 3) {
    this.defaultMaxRetries = defaultMaxRetries
    this.stmtInsert = db.prepare(
      "INSERT INTO outbox (channel, user_id, text, created_at, retry_count, max_retries, next_retry_at) VALUES (?, ?, ?, ?, 0, ?, NULL)"
    )
    this.stmtList = db.prepare<OutboxRow, [string, string]>(
      "SELECT id, channel, user_id, text, retry_count, max_retries, next_retry_at FROM outbox WHERE channel = ? AND (next_retry_at IS NULL OR next_retry_at <= ?) ORDER BY id"
    )
    this.stmtDelete = db.prepare("DELETE FROM outbox WHERE id = ?")
    this.stmtUpdateRetry = db.prepare("UPDATE outbox SET retry_count = ?, next_retry_at = ? WHERE id = ?")
  }

  enqueue(channel: string, userID: string, text: string, maxRetries = this.defaultMaxRetries): void {
    this.stmtInsert.run(channel, userID, text, new Date().toISOString(), maxRetries)
  }

  listPending(channel: string): OutboxMessage[] {
    const rows = this.stmtList.all(channel, new Date().toISOString()) as OutboxRow[]
    return rows.map((r) => ({
      id: r.id,
      channel: r.channel,
      userID: r.user_id,
      text: r.text,
      retryCount: r.retry_count,
      maxRetries: r.max_retries,
      nextRetryAt: r.next_retry_at,
    }))
  }

  acknowledge(id: number): void {
    this.stmtDelete.run(id)
  }

  markRetry(id: number, retryCount: number, nextRetryAt: string): void {
    this.stmtUpdateRetry.run(retryCount, nextRetryAt, id)
  }

  /**
   * Finalize all prepared statements
   * Call this when shutting down to release resources
   */
  close(): void {
    this.stmtInsert.finalize()
    this.stmtList.finalize()
    this.stmtDelete.finalize()
    this.stmtUpdateRetry.finalize()
  }
}
