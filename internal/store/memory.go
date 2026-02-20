package store

import (
	"time"

	"github.com/ncruces/go-sqlite3"
	"github.com/pocketbrain/pocketbrain/internal/core"
)

// MemoryRepo implements core.MemoryRepository backed by SQLite.
type MemoryRepo struct {
	db *DB
}

func NewMemoryRepo(db *DB) *MemoryRepo {
	return &MemoryRepo{db: db}
}

func (r *MemoryRepo) Append(fact string, source *string) (bool, error) {
	normalized := NormalizeMemoryFact(fact)

	// Check for duplicate.
	duplicate := false
	err := r.db.exec(func() error {
		return withStmt(r.db.conn, "SELECT 1 FROM memory WHERE fact_normalized = ?", func(stmt *sqlite3.Stmt) error {
			stmt.BindText(1, normalized)
			duplicate = stmt.Step()
			return nil
		})
	})
	if err != nil {
		return false, err
	}
	if duplicate {
		return false, nil
	}

	err = r.db.exec(func() error {
		return withStmt(r.db.conn, "INSERT INTO memory (fact, fact_normalized, source, created_at) VALUES (?, ?, ?, ?)", func(stmt *sqlite3.Stmt) error {
			stmt.BindText(1, fact)
			stmt.BindText(2, normalized)
			if source != nil {
				stmt.BindText(3, *source)
			} else {
				stmt.BindNull(3)
			}
			stmt.BindText(4, time.Now().UTC().Format(time.RFC3339))
			stmt.Step()
			return stmt.Err()
		})
	})
	return err == nil, err
}

func (r *MemoryRepo) Delete(id int64) (bool, error) {
	var changed bool
	err := r.db.exec(func() error {
		return withStmt(r.db.conn, "DELETE FROM memory WHERE id = ?", func(stmt *sqlite3.Stmt) error {
			stmt.BindInt64(1, id)
			stmt.Step()
			if err := stmt.Err(); err != nil {
				return err
			}
			changed = r.db.conn.Changes() > 0
			return nil
		})
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

func (r *MemoryRepo) Update(id int64, fact string) (bool, error) {
	normalized := NormalizeMemoryFact(fact)

	// Check for duplicate on other row.
	duplicate := false
	err := r.db.exec(func() error {
		return withStmt(r.db.conn, "SELECT 1 FROM memory WHERE fact_normalized = ? AND id != ?", func(stmt *sqlite3.Stmt) error {
			stmt.BindText(1, normalized)
			stmt.BindInt64(2, id)
			duplicate = stmt.Step()
			return nil
		})
	})
	if err != nil {
		return false, err
	}
	if duplicate {
		return false, nil
	}

	var changed bool
	err = r.db.exec(func() error {
		return withStmt(r.db.conn, "UPDATE memory SET fact = ?, fact_normalized = ? WHERE id = ?", func(stmt *sqlite3.Stmt) error {
			stmt.BindText(1, fact)
			stmt.BindText(2, normalized)
			stmt.BindInt64(3, id)
			stmt.Step()
			if err := stmt.Err(); err != nil {
				return err
			}
			changed = r.db.conn.Changes() > 0
			return nil
		})
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

func (r *MemoryRepo) GetAll() ([]core.MemoryEntry, error) {
	var entries []core.MemoryEntry
	err := r.db.exec(func() error {
		return withStmt(r.db.conn, "SELECT id, fact, source FROM memory ORDER BY id", func(stmt *sqlite3.Stmt) error {
			for stmt.Step() {
				e := core.MemoryEntry{
					ID:   stmt.ColumnInt64(0),
					Fact: stmt.ColumnText(1),
				}
				if stmt.ColumnType(2) != sqlite3.NULL {
					s := stmt.ColumnText(2)
					e.Source = &s
				}
				entries = append(entries, e)
			}
			return stmt.Err()
		})
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}
