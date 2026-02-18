/**
 * SQLite Memory Repository Implementation
 * Implements MemoryRepository port using bun:sqlite
 */

import type { MemoryRepository, MemoryEntry } from "../../../core/ports/memory-repository"
import { db } from "../../../store/db"

interface MemoryRow {
  id: number
  fact: string
  fact_normalized: string
  source: string | null
}

export class SQLiteMemoryRepository implements MemoryRepository {
  private readonly stmtInsert: ReturnType<typeof db.prepare>
  private readonly stmtSelectAll: ReturnType<typeof db.prepare>
  private readonly stmtCheck: ReturnType<typeof db.prepare>
  private readonly stmtDelete: ReturnType<typeof db.prepare>
  private readonly stmtUpdate: ReturnType<typeof db.prepare>

  constructor() {
    this.stmtInsert = db.prepare(
      "INSERT INTO memory (fact, fact_normalized, source, created_at) VALUES (?, ?, ?, ?)"
    )
    this.stmtSelectAll = db.prepare<MemoryRow, []>(
      "SELECT id, fact, fact_normalized, source FROM memory ORDER BY id"
    )
    this.stmtCheck = db.prepare<{ n: number }, [string]>(
      "SELECT 1 AS n FROM memory WHERE fact_normalized = ?"
    )
    this.stmtDelete = db.prepare("DELETE FROM memory WHERE id = ?")
    this.stmtUpdate = db.prepare("UPDATE memory SET fact = ?, fact_normalized = ? WHERE id = ?")
  }

  append(fact: string, source?: string): boolean {
    const normalized = this.normalizeFact(fact)
    const existing = this.stmtCheck.get(normalized)
    if (existing) {
      return false
    }
    this.stmtInsert.run(fact, normalized, source ?? null, new Date().toISOString())
    return true
  }

  delete(id: number): boolean {
    const result = this.stmtDelete.run(id)
    return result.changes > 0
  }

  update(id: number, fact: string): boolean {
    const normalized = this.normalizeFact(fact)
    const existing = this.stmtCheck.get(normalized)
    if (existing) {
      return false
    }
    const result = this.stmtUpdate.run(fact, normalized, id)
    return result.changes > 0
  }

  getAll(): MemoryEntry[] {
    const rows = this.stmtSelectAll.all() as MemoryRow[]
    return rows.map((r) => ({
      id: r.id,
      fact: r.fact,
      source: r.source,
    }))
  }

  /**
   * Finalize all prepared statements
   * Call this when shutting down to release resources
   */
  close(): void {
    this.stmtInsert.finalize()
    this.stmtSelectAll.finalize()
    this.stmtCheck.finalize()
    this.stmtDelete.finalize()
    this.stmtUpdate.finalize()
  }

  private normalizeFact(fact: string): string {
    return fact.toLowerCase().replace(/\s+/g, " ").trim()
  }
}
