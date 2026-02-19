import { afterEach, beforeEach, describe, expect, test } from "bun:test"
import { Database } from "bun:sqlite"
import { migrateMemoryTable, normalizeMemoryFact } from "../../src/store/db"

interface TableInfoRow {
  name: string
}

describe("memory migration", () => {
  let testDb: Database

  beforeEach(() => {
    testDb = new Database(":memory:")
  })

  afterEach(() => {
    testDb.close(false)
  })

  test("adds fact_normalized column and backfills existing rows", () => {
    testDb.run(`
      CREATE TABLE memory (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        fact TEXT NOT NULL,
        source TEXT,
        created_at TEXT NOT NULL
      )
    `)
    testDb.run(
      "INSERT INTO memory (fact, source, created_at) VALUES (?, ?, ?)",
      ["User   likes   Coffee", "test", new Date().toISOString()],
    )

    migrateMemoryTable(testDb)

    const columns = testDb.query<TableInfoRow, []>("PRAGMA table_info(memory)").all() as TableInfoRow[]
    const row = testDb
      .query<{ fact_normalized: string }, []>("SELECT fact_normalized FROM memory LIMIT 1")
      .get()

    expect(columns.some((column) => column.name === "fact_normalized")).toBe(true)
    expect(row?.fact_normalized).toBe("user likes coffee")
  })

  test("fills only missing normalized values", () => {
    testDb.run(`
      CREATE TABLE memory (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        fact TEXT NOT NULL,
        fact_normalized TEXT,
        source TEXT,
        created_at TEXT NOT NULL
      )
    `)

    testDb.run(
      "INSERT INTO memory (fact, fact_normalized, source, created_at) VALUES (?, ?, ?, ?)",
      ["Keep Existing", "custom-normalized", "test", new Date().toISOString()],
    )
    testDb.run(
      "INSERT INTO memory (fact, fact_normalized, source, created_at) VALUES (?, ?, ?, ?)",
      ["Needs Normalize", null, "test", new Date().toISOString()],
    )

    migrateMemoryTable(testDb)

    const rows = testDb
      .query<{ fact: string; fact_normalized: string }, []>(
        "SELECT fact, fact_normalized FROM memory ORDER BY id",
      )
      .all()

    expect(rows[0]?.fact_normalized).toBe("custom-normalized")
    expect(rows[1]?.fact_normalized).toBe("needs normalize")
  })

  test("normalization helper collapses whitespace and lowercases", () => {
    expect(normalizeMemoryFact("  Multi\n  SPACE   Value  ")).toBe("multi space value")
  })
})
