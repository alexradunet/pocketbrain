package store

import (
	"time"

	"github.com/ncruces/go-sqlite3"
	"github.com/pocketbrain/pocketbrain/internal/core"
)

// MemoryRepo implements core.MemoryRepository backed by SQLite.
type MemoryRepo struct {
	conn *sqlite3.Conn
}

func NewMemoryRepo(db *DB) *MemoryRepo {
	return &MemoryRepo{conn: db.Conn()}
}

func (r *MemoryRepo) Append(fact string, source *string) (bool, error) {
	normalized := NormalizeMemoryFact(fact)

	// Check for duplicate.
	check, _, err := r.conn.Prepare("SELECT 1 FROM memory WHERE fact_normalized = ?")
	if err != nil {
		return false, err
	}
	defer check.Close()
	check.BindText(1, normalized)
	if check.Step() {
		return false, nil // duplicate
	}

	ins, _, err := r.conn.Prepare("INSERT INTO memory (fact, fact_normalized, source, created_at) VALUES (?, ?, ?, ?)")
	if err != nil {
		return false, err
	}
	defer ins.Close()

	ins.BindText(1, fact)
	ins.BindText(2, normalized)
	if source != nil {
		ins.BindText(3, *source)
	} else {
		ins.BindNull(3)
	}
	ins.BindText(4, time.Now().UTC().Format(time.RFC3339))
	ins.Step()
	return true, ins.Close()
}

func (r *MemoryRepo) Delete(id int64) (bool, error) {
	stmt, _, err := r.conn.Prepare("DELETE FROM memory WHERE id = ?")
	if err != nil {
		return false, err
	}
	defer stmt.Close()
	stmt.BindInt64(1, id)
	stmt.Step()
	if err := stmt.Close(); err != nil {
		return false, err
	}
	return r.conn.Changes() > 0, nil
}

func (r *MemoryRepo) Update(id int64, fact string) (bool, error) {
	normalized := NormalizeMemoryFact(fact)

	// Check for duplicate on other row.
	check, _, err := r.conn.Prepare("SELECT 1 FROM memory WHERE fact_normalized = ? AND id != ?")
	if err != nil {
		return false, err
	}
	defer check.Close()
	check.BindText(1, normalized)
	check.BindInt64(2, id)
	if check.Step() {
		return false, nil // duplicate
	}

	upd, _, err := r.conn.Prepare("UPDATE memory SET fact = ?, fact_normalized = ? WHERE id = ?")
	if err != nil {
		return false, err
	}
	defer upd.Close()
	upd.BindText(1, fact)
	upd.BindText(2, normalized)
	upd.BindInt64(3, id)
	upd.Step()
	if err := upd.Close(); err != nil {
		return false, err
	}
	return r.conn.Changes() > 0, nil
}

func (r *MemoryRepo) GetAll() ([]core.MemoryEntry, error) {
	stmt, _, err := r.conn.Prepare("SELECT id, fact, source FROM memory ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var entries []core.MemoryEntry
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
	return entries, nil
}
