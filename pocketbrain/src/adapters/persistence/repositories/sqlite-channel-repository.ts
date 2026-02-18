/**
 * SQLite Channel Repository Implementation
 * Implements ChannelRepository port using bun:sqlite
 */

import type { ChannelRepository, LastChannel, ChannelType } from "../../../core/ports/channel-repository"
import { db } from "../../../store/db"

interface KvRow {
  value: string
}

export class SQLiteChannelRepository implements ChannelRepository {
  private readonly stmtKvGet: ReturnType<typeof db.prepare>
  private readonly stmtKvSet: ReturnType<typeof db.prepare>

  constructor() {
    this.stmtKvGet = db.prepare<KvRow, [string]>("SELECT value FROM kv WHERE key = ?")
    this.stmtKvSet = db.prepare(
      "INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value"
    )
  }

  saveLastChannel(channel: ChannelType, userID: string): void {
    const value = JSON.stringify({ channel, userID })
    this.stmtKvSet.run("last_channel", value)
  }

  getLastChannel(): LastChannel | null {
    const row = this.stmtKvGet.get("last_channel")
    if (!row) return null
    
    const kvRow = row as KvRow
    const value = kvRow.value
    if (!value) return null
    
    try {
      const parsed = JSON.parse(value) as Partial<LastChannel>
      if (parsed.channel === "whatsapp" && typeof parsed.userID === "string") {
        return { channel: parsed.channel, userID: parsed.userID }
      }
    } catch (error) {
      // Invalid stored value, ignore but log in debug mode
      if (Bun.env.DEBUG) {
        console.debug("[channel-repo] Failed to parse last channel:", error)
      }
    }
    return null
  }

  /**
   * Finalize all prepared statements
   * Call this when shutting down to release resources
   */
  close(): void {
    this.stmtKvGet.finalize()
    this.stmtKvSet.finalize()
  }
}
