package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ncruces/go-sqlite3"
	_ "github.com/ncruces/go-sqlite3/embed"
)

// DB wraps a SQLite connection with prepared-statement management.
type DB struct {
	conn *sqlite3.Conn
}

// Open creates the data directory if needed, opens state.db, enables WAL
// and foreign keys, and runs schema migrations.
func Open(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dataDir, err)
	}

	dbPath := filepath.Join(dataDir, "state.db")
	conn, err := sqlite3.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dbPath, err)
	}

	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
	}
	for _, p := range pragmas {
		if err := conn.Exec(p); err != nil {
			conn.Close()
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

// Conn returns the underlying sqlite3 connection.
func (db *DB) Conn() *sqlite3.Conn {
	return db.conn
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	ddl := []string{
		`CREATE TABLE IF NOT EXISTS kv (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS whitelist (
			channel TEXT NOT NULL,
			user_id TEXT NOT NULL,
			PRIMARY KEY (channel, user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS outbox (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			channel     TEXT NOT NULL,
			user_id     TEXT NOT NULL,
			text        TEXT NOT NULL,
			created_at  TEXT NOT NULL,
			retry_count INTEGER NOT NULL DEFAULT 0,
			max_retries INTEGER NOT NULL DEFAULT 3,
			next_retry_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS memory (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			fact            TEXT NOT NULL,
			fact_normalized TEXT NOT NULL,
			source          TEXT,
			created_at      TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_memory_normalized ON memory(fact_normalized)`,
		`CREATE TABLE IF NOT EXISTS heartbeat_tasks (
			id      INTEGER PRIMARY KEY AUTOINCREMENT,
			task    TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			role       TEXT NOT NULL,
			content    TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, id)`,
	}

	for _, stmt := range ddl {
		if err := db.conn.Exec(stmt); err != nil {
			return fmt.Errorf("exec ddl: %w", err)
		}
	}

	// Migrate: add fact_normalized column if missing (compat with TS version).
	if err := db.migrateMemoryNormalized(); err != nil {
		return err
	}

	return nil
}

func (db *DB) migrateMemoryNormalized() error {
	hasCol, err := db.hasColumn("memory", "fact_normalized")
	if err != nil {
		return err
	}
	if hasCol {
		return nil
	}

	if err := db.conn.Exec("ALTER TABLE memory ADD COLUMN fact_normalized TEXT"); err != nil {
		return err
	}

	// Backfill existing rows.
	stmt, _, err := db.conn.Prepare("SELECT id, fact FROM memory WHERE fact_normalized IS NULL OR TRIM(fact_normalized) = ''")
	if err != nil {
		return err
	}
	defer stmt.Close()

	type row struct {
		id   int64
		fact string
	}
	var rows []row
	for stmt.Step() {
		rows = append(rows, row{id: stmt.ColumnInt64(0), fact: stmt.ColumnText(1)})
	}
	if err := stmt.Close(); err != nil {
		return err
	}

	if len(rows) == 0 {
		return nil
	}

	if err := db.conn.Exec("BEGIN"); err != nil {
		return err
	}

	update, _, err := db.conn.Prepare("UPDATE memory SET fact_normalized = ? WHERE id = ?")
	if err != nil {
		db.conn.Exec("ROLLBACK")
		return err
	}
	defer update.Close()

	for _, r := range rows {
		update.BindText(1, NormalizeMemoryFact(r.fact))
		update.BindInt64(2, r.id)
		update.Step()
		if err := update.Reset(); err != nil {
			db.conn.Exec("ROLLBACK")
			return err
		}
	}

	return db.conn.Exec("COMMIT")
}

func (db *DB) hasColumn(table, column string) (bool, error) {
	if !isValidIdentifier(table) {
		return false, fmt.Errorf("invalid table name: %q", table)
	}
	stmt, _, err := db.conn.Prepare(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer stmt.Close()

	for stmt.Step() {
		name := stmt.ColumnText(1) // column index 1 is the name
		if name == column {
			return true, nil
		}
	}
	return false, nil
}

// isValidIdentifier returns true if s is a safe SQL identifier
// (alphanumeric + underscore, non-empty).
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// NormalizeMemoryFact lowercases and collapses whitespace.
func NormalizeMemoryFact(fact string) string {
	s := strings.ToLower(fact)
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}
