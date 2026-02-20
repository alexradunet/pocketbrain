package store

import (
	"encoding/json"

	"github.com/ncruces/go-sqlite3"
	"github.com/pocketbrain/pocketbrain/internal/core"
)

// ChannelRepo implements core.ChannelRepository backed by SQLite kv table.
type ChannelRepo struct {
	conn *sqlite3.Conn
}

func NewChannelRepo(db *DB) *ChannelRepo {
	return &ChannelRepo{conn: db.Conn()}
}

func (r *ChannelRepo) SaveLastChannel(channel, userID string) error {
	value, err := json.Marshal(core.LastChannel{Channel: channel, UserID: userID})
	if err != nil {
		return err
	}
	return withStmt(r.conn, "INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", func(stmt *sqlite3.Stmt) error {
		stmt.BindText(1, "last_channel")
		stmt.BindText(2, string(value))
		stmt.Step()
		return nil
	})
}

func (r *ChannelRepo) GetLastChannel() (*core.LastChannel, error) {
	var raw string
	err := withStmt(r.conn, "SELECT value FROM kv WHERE key = ?", func(stmt *sqlite3.Stmt) error {
		stmt.BindText(1, "last_channel")
		if !stmt.Step() {
			return nil
		}
		raw = stmt.ColumnText(0)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}

	var lc core.LastChannel
	if err := json.Unmarshal([]byte(raw), &lc); err != nil {
		return nil, nil // treat corrupt data as missing
	}
	if lc.Channel == "" || lc.UserID == "" {
		return nil, nil
	}
	return &lc, nil
}
