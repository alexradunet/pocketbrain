package store

import (
	"time"

	"github.com/ncruces/go-sqlite3"
	"github.com/pocketbrain/pocketbrain/internal/core"
)

// OutboxRepo implements core.OutboxRepository backed by SQLite.
type OutboxRepo struct {
	conn           *sqlite3.Conn
	defaultMaxRetries int
}

func NewOutboxRepo(db *DB, defaultMaxRetries int) *OutboxRepo {
	if defaultMaxRetries <= 0 {
		defaultMaxRetries = 3
	}
	return &OutboxRepo{conn: db.Conn(), defaultMaxRetries: defaultMaxRetries}
}

func (r *OutboxRepo) Enqueue(channel, userID, text string, maxRetries int) error {
	if maxRetries <= 0 {
		maxRetries = r.defaultMaxRetries
	}
	stmt, _, err := r.conn.Prepare(
		"INSERT INTO outbox (channel, user_id, text, created_at, retry_count, max_retries, next_retry_at) VALUES (?, ?, ?, ?, 0, ?, NULL)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	stmt.BindText(1, channel)
	stmt.BindText(2, userID)
	stmt.BindText(3, text)
	stmt.BindText(4, time.Now().UTC().Format(time.RFC3339))
	stmt.BindInt(5, maxRetries)
	stmt.Step()
	return stmt.Close()
}

func (r *OutboxRepo) ListPending(channel string) ([]core.OutboxMessage, error) {
	stmt, _, err := r.conn.Prepare(
		"SELECT id, channel, user_id, text, retry_count, max_retries, next_retry_at FROM outbox WHERE channel = ? AND (next_retry_at IS NULL OR next_retry_at <= ?) ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	stmt.BindText(1, channel)
	stmt.BindText(2, time.Now().UTC().Format(time.RFC3339))

	var msgs []core.OutboxMessage
	for stmt.Step() {
		m := core.OutboxMessage{
			ID:         stmt.ColumnInt64(0),
			Channel:    stmt.ColumnText(1),
			UserID:     stmt.ColumnText(2),
			Text:       stmt.ColumnText(3),
			RetryCount: stmt.ColumnInt(4),
			MaxRetries: stmt.ColumnInt(5),
		}
		if stmt.ColumnType(6) != sqlite3.NULL {
			s := stmt.ColumnText(6)
			m.NextRetryAt = &s
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func (r *OutboxRepo) Acknowledge(id int64) error {
	stmt, _, err := r.conn.Prepare("DELETE FROM outbox WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()
	stmt.BindInt64(1, id)
	stmt.Step()
	return stmt.Close()
}

func (r *OutboxRepo) MarkRetry(id int64, retryCount int, nextRetryAt string) error {
	stmt, _, err := r.conn.Prepare("UPDATE outbox SET retry_count = ?, next_retry_at = ? WHERE id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()
	stmt.BindInt(1, retryCount)
	stmt.BindText(2, nextRetryAt)
	stmt.BindInt64(3, id)
	stmt.Step()
	return stmt.Close()
}
