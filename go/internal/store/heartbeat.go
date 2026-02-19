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
	stmt, _, err := r.conn.Prepare("SELECT task FROM heartbeat_tasks WHERE enabled = 1 ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	var tasks []string
	for stmt.Step() {
		tasks = append(tasks, stmt.ColumnText(0))
	}
	return tasks, nil
}

func (r *HeartbeatRepo) GetTaskCount() (int, error) {
	stmt, _, err := r.conn.Prepare("SELECT COUNT(*) FROM heartbeat_tasks WHERE enabled = 1")
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	if stmt.Step() {
		return stmt.ColumnInt(0), nil
	}
	return 0, nil
}
