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
	stmt, _, err := r.conn.Prepare("SELECT 1 FROM whitelist WHERE channel = ? AND user_id = ?")
	if err != nil {
		return false, err
	}
	defer stmt.Close()
	stmt.BindText(1, channel)
	stmt.BindText(2, userID)
	return stmt.Step(), nil
}

func (r *WhitelistRepo) AddToWhitelist(channel, userID string) (bool, error) {
	ok, err := r.IsWhitelisted(channel, userID)
	if err != nil {
		return false, err
	}
	if ok {
		return false, nil
	}

	stmt, _, err := r.conn.Prepare("INSERT OR IGNORE INTO whitelist (channel, user_id) VALUES (?, ?)")
	if err != nil {
		return false, err
	}
	defer stmt.Close()
	stmt.BindText(1, channel)
	stmt.BindText(2, userID)
	stmt.Step()
	return true, stmt.Close()
}

func (r *WhitelistRepo) RemoveFromWhitelist(channel, userID string) (bool, error) {
	stmt, _, err := r.conn.Prepare("DELETE FROM whitelist WHERE channel = ? AND user_id = ?")
	if err != nil {
		return false, err
	}
	defer stmt.Close()
	stmt.BindText(1, channel)
	stmt.BindText(2, userID)
	stmt.Step()
	if err := stmt.Close(); err != nil {
		return false, err
	}
	return r.conn.Changes() > 0, nil
}
