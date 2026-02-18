/**
 * SQLite Whitelist Repository Implementation
 * Implements WhitelistRepository port using bun:sqlite
 */

import type { WhitelistRepository } from "../../../core/ports/whitelist-repository"
import { db } from "../../../store/db"

export class SQLiteWhitelistRepository implements WhitelistRepository {
  private readonly stmtCheck: ReturnType<typeof db.prepare>
  private readonly stmtInsert: ReturnType<typeof db.prepare>
  private readonly stmtDelete: ReturnType<typeof db.prepare>

  constructor() {
    this.stmtCheck = db.prepare<{ n: number }, [string, string]>(
      "SELECT 1 AS n FROM whitelist WHERE channel = ? AND user_id = ?"
    )
    this.stmtInsert = db.prepare(
      "INSERT OR IGNORE INTO whitelist (channel, user_id) VALUES (?, ?)"
    )
    this.stmtDelete = db.prepare("DELETE FROM whitelist WHERE channel = ? AND user_id = ?")
  }

  isWhitelisted(channel: string, userID: string): boolean {
    return !!this.stmtCheck.get(channel, userID)
  }

  addToWhitelist(channel: string, userID: string): boolean {
    const before = this.stmtCheck.get(channel, userID)
    if (before) return false
    this.stmtInsert.run(channel, userID)
    return true
  }

  removeFromWhitelist(channel: string, userID: string): boolean {
    const result = this.stmtDelete.run(channel, userID)
    return result.changes > 0
  }

  /**
   * Finalize all prepared statements
   * Call this when shutting down to release resources
   */
  close(): void {
    this.stmtCheck.finalize()
    this.stmtInsert.finalize()
    this.stmtDelete.finalize()
  }
}
