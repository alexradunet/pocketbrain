package store

import "github.com/ncruces/go-sqlite3"

// WhitelistRepo implements core.WhitelistRepository backed by SQLite.
type WhitelistRepo struct {
	conn *sqlite3.Conn
}

func NewWhitelistRepo(db *DB) *WhitelistRepo {
	return &WhitelistRepo{conn: db.Conn()}
}

func (r *WhitelistRepo) IsWhitelisted(channel, userID string) (bool, error) {
	whitelisted := false
	err := withStmt(r.conn, "SELECT 1 FROM whitelist WHERE channel = ? AND user_id = ?", func(stmt *sqlite3.Stmt) error {
		stmt.BindText(1, channel)
		stmt.BindText(2, userID)
		whitelisted = stmt.Step()
		return nil
	})
	return whitelisted, err
}

func (r *WhitelistRepo) AddToWhitelist(channel, userID string) (bool, error) {
	ok, err := r.IsWhitelisted(channel, userID)
	if err != nil {
		return false, err
	}
	if ok {
		return false, nil
	}

	err = withStmt(r.conn, "INSERT OR IGNORE INTO whitelist (channel, user_id) VALUES (?, ?)", func(stmt *sqlite3.Stmt) error {
		stmt.BindText(1, channel)
		stmt.BindText(2, userID)
		stmt.Step()
		return stmt.Err()
	})
	return err == nil, err
}

func (r *WhitelistRepo) RemoveFromWhitelist(channel, userID string) (bool, error) {
	if err := withStmt(r.conn, "DELETE FROM whitelist WHERE channel = ? AND user_id = ?", func(stmt *sqlite3.Stmt) error {
		stmt.BindText(1, channel)
		stmt.BindText(2, userID)
		stmt.Step()
		return stmt.Err()
	}); err != nil {
		return false, err
	}
	return r.conn.Changes() > 0, nil
}
