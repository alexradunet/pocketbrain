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
  private readonly stmtInsert: ReturnType<typeof db.prepare>
  private readonly stmtDelete: ReturnType<typeof db.prepare>
  private readonly stmtCount: ReturnType<typeof db.prepare>

  constructor() {
    this.stmtSelectAll = db.prepare<TaskRow, []>(
      "SELECT task FROM heartbeat_tasks WHERE enabled = 1 ORDER BY id"
    )
    this.stmtInsert = db.prepare(
      "INSERT INTO heartbeat_tasks (task, enabled) VALUES (?, 1)"
    )
    this.stmtDelete = db.prepare("DELETE FROM heartbeat_tasks WHERE id = ?")
    this.stmtCount = db.prepare<CountRow, []>(
      "SELECT COUNT(*) as count FROM heartbeat_tasks WHERE enabled = 1"
    )
  }

  getTasks(): string[] {
    const rows = this.stmtSelectAll.all() as TaskRow[]
    return rows.map((r) => r.task)
  }

  addTask(task: string): void {
    this.stmtInsert.run(task)
  }

  removeTask(id: number): void {
    this.stmtDelete.run(id)
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
    this.stmtInsert.finalize()
    this.stmtDelete.finalize()
    this.stmtCount.finalize()
  }
}
