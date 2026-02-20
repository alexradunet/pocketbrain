package store

import (
	"time"

	"github.com/ncruces/go-sqlite3"
)

// ChatMessage represents a stored conversation message for Fantasy session history.
type ChatMessage struct {
	ID        int64
	SessionID string
	Role      string
	Content   string
	CreatedAt string
}

// MessageRepo stores conversation history in SQLite (Fantasy doesn't manage sessions).
type MessageRepo struct {
	conn *sqlite3.Conn
}

func NewMessageRepo(db *DB) *MessageRepo {
	return &MessageRepo{conn: db.Conn()}
}

// Append stores a new message in the conversation history.
func (r *MessageRepo) Append(sessionID, role, content string) error {
	return withStmt(r.conn, "INSERT INTO messages (session_id, role, content, created_at) VALUES (?, ?, ?, ?)", func(stmt *sqlite3.Stmt) error {
		stmt.BindText(1, sessionID)
		stmt.BindText(2, role)
		stmt.BindText(3, content)
		stmt.BindText(4, time.Now().UTC().Format(time.RFC3339))
		stmt.Step()
		return nil
	})
}

// GetBySession returns messages for a session, ordered by ID.
func (r *MessageRepo) GetBySession(sessionID string) ([]ChatMessage, error) {
	var msgs []ChatMessage
	err := withStmt(r.conn, "SELECT id, session_id, role, content, created_at FROM messages WHERE session_id = ? ORDER BY id", func(stmt *sqlite3.Stmt) error {
		stmt.BindText(1, sessionID)
		for stmt.Step() {
			msgs = append(msgs, ChatMessage{
				ID:        stmt.ColumnInt64(0),
				SessionID: stmt.ColumnText(1),
				Role:      stmt.ColumnText(2),
				Content:   stmt.ColumnText(3),
				CreatedAt: stmt.ColumnText(4),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return msgs, nil
}

// GetRecentBySession returns the last N messages for a session.
func (r *MessageRepo) GetRecentBySession(sessionID string, limit int) ([]ChatMessage, error) {
	var msgs []ChatMessage
	err := withStmt(r.conn, "SELECT id, session_id, role, content, created_at FROM messages WHERE session_id = ? ORDER BY id DESC LIMIT ?", func(stmt *sqlite3.Stmt) error {
		stmt.BindText(1, sessionID)
		stmt.BindInt(2, limit)
		for stmt.Step() {
			msgs = append(msgs, ChatMessage{
				ID:        stmt.ColumnInt64(0),
				SessionID: stmt.ColumnText(1),
				Role:      stmt.ColumnText(2),
				Content:   stmt.ColumnText(3),
				CreatedAt: stmt.ColumnText(4),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Reverse to chronological order.
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

// DeleteBySession removes all messages for a session.
func (r *MessageRepo) DeleteBySession(sessionID string) error {
	return withStmt(r.conn, "DELETE FROM messages WHERE session_id = ?", func(stmt *sqlite3.Stmt) error {
		stmt.BindText(1, sessionID)
		stmt.Step()
		return nil
	})
}
