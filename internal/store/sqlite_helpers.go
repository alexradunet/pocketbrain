package store

import "github.com/ncruces/go-sqlite3"

func withStmt(conn *sqlite3.Conn, query string, fn func(stmt *sqlite3.Stmt) error) (err error) {
	stmt, _, err := conn.Prepare(query)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := stmt.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	return fn(stmt)
}
