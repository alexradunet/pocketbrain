/**
 * SQLite Heartbeat Repository Implementation
 * Implements HeartbeatRepository port using bun:sqlite
 */

import type { HeartbeatRepository } from "../../../core/ports/heartbeat-repository"
import { db } from "../../../store/db"

interface TaskRow {
  task: string
}

interface CountRow {
  count: number
}

export class SQLiteHeartbeatRepository implements HeartbeatRepository {
  private readonly stmtSelectAll: ReturnType<typeof db.prepare>
  private readonly stmtCount: ReturnType<typeof db.prepare>

  constructor() {
    this.stmtSelectAll = db.prepare<TaskRow, []>(
      "SELECT task FROM heartbeat_tasks WHERE enabled = 1 ORDER BY id"
    )
    this.stmtCount = db.prepare<CountRow, []>(
      "SELECT COUNT(*) as count FROM heartbeat_tasks WHERE enabled = 1"
    )
  }

  getTasks(): string[] {
    const rows = this.stmtSelectAll.all() as TaskRow[]
    return rows.map((r) => r.task)
  }

  getTaskCount(): number {
    const row = this.stmtCount.get()
    if (!row) return 0
    return (row as CountRow).count
  }

  /**
   * Finalize all prepared statements
   * Call this when shutting down to release resources
   */
  close(): void {
    this.stmtSelectAll.finalize()
    this.stmtCount.finalize()
  }
}
