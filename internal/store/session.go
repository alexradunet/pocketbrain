package store

import "github.com/ncruces/go-sqlite3"

// SessionRepo implements core.SessionRepository backed by SQLite kv table.
type SessionRepo struct {
	conn *sqlite3.Conn
}

func NewSessionRepo(db *DB) *SessionRepo {
	return &SessionRepo{conn: db.Conn()}
}

func (r *SessionRepo) GetSessionID(key string) (string, bool, error) {
	var value string
	found := false
	err := withStmt(r.conn, "SELECT value FROM kv WHERE key = ?", func(stmt *sqlite3.Stmt) error {
		stmt.BindText(1, key)
		if !stmt.Step() {
			return nil
		}
		value = stmt.ColumnText(0)
		found = true
		return nil
	})
	if err != nil {
		return "", false, err
	}
	return value, found, nil
}

func (r *SessionRepo) SaveSessionID(key, sessionID string) error {
	return withStmt(r.conn, "INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", func(stmt *sqlite3.Stmt) error {
		stmt.BindText(1, key)
		stmt.BindText(2, sessionID)
		stmt.Step()
		return stmt.Err()
	})
}

func (r *SessionRepo) DeleteSession(key string) error {
	return withStmt(r.conn, "DELETE FROM kv WHERE key = ?", func(stmt *sqlite3.Stmt) error {
		stmt.BindText(1, key)
		stmt.Step()
		return stmt.Err()
	})
}
