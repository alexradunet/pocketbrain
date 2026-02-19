/**
 * Database Module
 * 
 * Raw database instance and schema setup.
 * All persistence operations go through repository implementations.
 */

import { Database } from "bun:sqlite"
import { mkdirSync } from "node:fs"
import { isAbsolute, join } from "node:path"

const configuredDataDir = Bun.env.DATA_DIR?.trim() || ".data"
const DATA_DIR = isAbsolute(configuredDataDir)
  ? configuredDataDir
  : join(process.cwd(), configuredDataDir)
mkdirSync(DATA_DIR, { recursive: true })

export const db = new Database(join(DATA_DIR, "state.db"))
db.run("PRAGMA journal_mode = WAL")
db.run("PRAGMA foreign_keys = ON")

export function normalizeMemoryFact(fact: string): string {
  return fact.toLowerCase().replace(/\s+/g, " ").trim()
}

interface TableInfoRow {
  name: string
}

interface MemorySeedRow {
  id: number
  fact: string
}

function hasColumn(database: Database, tableName: string, columnName: string): boolean {
  const columns = database.query<TableInfoRow, []>(`PRAGMA table_info(${tableName})`).all() as TableInfoRow[]
  return columns.some((column) => column.name === columnName)
}

export function migrateMemoryTable(database: Database): void {
  if (!hasColumn(database, "memory", "fact_normalized")) {
    database.run("ALTER TABLE memory ADD COLUMN fact_normalized TEXT")
  }

  const rows = database
    .query<MemorySeedRow, []>(
      "SELECT id, fact FROM memory WHERE fact_normalized IS NULL OR TRIM(fact_normalized) = ''",
    )
    .all() as MemorySeedRow[]

  if (rows.length === 0) {
    return
  }

  const update = database.prepare("UPDATE memory SET fact_normalized = ? WHERE id = ?")
  database.run("BEGIN")
  try {
    for (const row of rows) {
      update.run(normalizeMemoryFact(row.fact), row.id)
    }
    database.run("COMMIT")
  } catch (error) {
    database.run("ROLLBACK")
    throw error
  } finally {
    update.finalize()
  }
}

// Schema setup
db.run(`
  CREATE TABLE IF NOT EXISTS kv (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
  )
`)

db.run(`
  CREATE TABLE IF NOT EXISTS whitelist (
    channel TEXT NOT NULL,
    user_id TEXT NOT NULL,
    PRIMARY KEY (channel, user_id)
  )
`)

db.run(`
  CREATE TABLE IF NOT EXISTS outbox (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    channel    TEXT NOT NULL,
    user_id    TEXT NOT NULL,
    text       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    next_retry_at TEXT
  )
`)

db.run(`
  CREATE TABLE IF NOT EXISTS memory (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    fact       TEXT NOT NULL,
    fact_normalized TEXT NOT NULL,
    source     TEXT,
    created_at TEXT NOT NULL
  )
`)
migrateMemoryTable(db)
db.run(`CREATE INDEX IF NOT EXISTS idx_memory_normalized ON memory(fact_normalized)`)

db.run(`
  CREATE TABLE IF NOT EXISTS heartbeat_tasks (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    task    TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1
  )
`)
