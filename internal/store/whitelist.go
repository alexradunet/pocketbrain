package store

import "github.com/ncruces/go-sqlite3"

// WhitelistRepo implements core.WhitelistRepository backed by SQLite.
type WhitelistRepo struct {
	db *DB
}

func NewWhitelistRepo(db *DB) *WhitelistRepo {
	return &WhitelistRepo{db: db}
}

func (r *WhitelistRepo) IsWhitelisted(channel, userID string) (bool, error) {
	var whitelisted bool
	err := r.db.exec(func() error {
		return withStmt(r.db.conn, "SELECT 1 FROM whitelist WHERE channel = ? AND user_id = ?", func(stmt *sqlite3.Stmt) error {
			stmt.BindText(1, channel)
			stmt.BindText(2, userID)
			whitelisted = stmt.Step()
			return nil
		})
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

	err = r.db.exec(func() error {
		return withStmt(r.db.conn, "INSERT OR IGNORE INTO whitelist (channel, user_id) VALUES (?, ?)", func(stmt *sqlite3.Stmt) error {
			stmt.BindText(1, channel)
			stmt.BindText(2, userID)
			stmt.Step()
			return stmt.Err()
		})
	})
	return err == nil, err
}

func (r *WhitelistRepo) RemoveFromWhitelist(channel, userID string) (bool, error) {
	var changed bool
	err := r.db.exec(func() error {
		return withStmt(r.db.conn, "DELETE FROM whitelist WHERE channel = ? AND user_id = ?", func(stmt *sqlite3.Stmt) error {
			stmt.BindText(1, channel)
			stmt.BindText(2, userID)
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
