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
	stmt, _, err := r.conn.Prepare("SELECT value FROM kv WHERE key = ?")
	if err != nil {
		return "", false, err
	}
	defer stmt.Close()
	stmt.BindText(1, key)
	if !stmt.Step() {
		return "", false, nil
	}
	return stmt.ColumnText(0), true, nil
}

func (r *SessionRepo) SaveSessionID(key, sessionID string) error {
	stmt, _, err := r.conn.Prepare("INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value")
	if err != nil {
		return err
	}
	defer stmt.Close()
	stmt.BindText(1, key)
	stmt.BindText(2, sessionID)
	stmt.Step()
	return stmt.Close()
}

func (r *SessionRepo) DeleteSession(key string) error {
	stmt, _, err := r.conn.Prepare("DELETE FROM kv WHERE key = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()
	stmt.BindText(1, key)
	stmt.Step()
	return stmt.Close()
}
