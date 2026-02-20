package store

import "github.com/ncruces/go-sqlite3"

// HeartbeatRepo implements core.HeartbeatRepository backed by SQLite.
type HeartbeatRepo struct {
	conn *sqlite3.Conn
}

func NewHeartbeatRepo(db *DB) *HeartbeatRepo {
	return &HeartbeatRepo{conn: db.Conn()}
}

func (r *HeartbeatRepo) GetTasks() ([]string, error) {
	var tasks []string
	err := withStmt(r.conn, "SELECT task FROM heartbeat_tasks WHERE enabled = 1 ORDER BY id", func(stmt *sqlite3.Stmt) error {
		for stmt.Step() {
			tasks = append(tasks, stmt.ColumnText(0))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

func (r *HeartbeatRepo) GetTaskCount() (int, error) {
	count := 0
	err := withStmt(r.conn, "SELECT COUNT(*) FROM heartbeat_tasks WHERE enabled = 1", func(stmt *sqlite3.Stmt) error {
		if stmt.Step() {
			count = stmt.ColumnInt(0)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}
