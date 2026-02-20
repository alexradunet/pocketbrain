package store

import (
	"os"
	"path/filepath"
	"testing"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")
	db, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(filepath.Join(dir, "state.db")); err != nil {
		t.Errorf("state.db should exist: %v", err)
	}
}

// --- Memory Repository ---

func TestMemoryAppendAndGetAll(t *testing.T) {
	db := testDB(t)
	repo := NewMemoryRepo(db)

	ok, err := repo.Append("user likes coffee", nil)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if !ok {
		t.Error("first insert should return true")
	}

	// Duplicate (case insensitive).
	ok, err = repo.Append("USER LIKES COFFEE", nil)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if ok {
		t.Error("duplicate should return false")
	}

	entries, err := repo.GetAll()
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len = %d, want 1", len(entries))
	}
	if entries[0].Fact != "user likes coffee" {
		t.Errorf("fact = %q", entries[0].Fact)
	}
}

func TestMemoryAppendWithSource(t *testing.T) {
	db := testDB(t)
	repo := NewMemoryRepo(db)

	src := "whatsapp"
	ok, err := repo.Append("test fact", &src)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if !ok {
		t.Error("should return true")
	}

	entries, err := repo.GetAll()
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len = %d, want 1", len(entries))
	}
	if entries[0].Source == nil || *entries[0].Source != "whatsapp" {
		t.Errorf("source = %v, want 'whatsapp'", entries[0].Source)
	}
}

func TestMemoryDelete(t *testing.T) {
	db := testDB(t)
	repo := NewMemoryRepo(db)

	repo.Append("fact to delete", nil)
	entries, _ := repo.GetAll()
	id := entries[0].ID

	ok, err := repo.Delete(id)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !ok {
		t.Error("delete should return true")
	}

	ok, err = repo.Delete(9999)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if ok {
		t.Error("delete non-existent should return false")
	}
}

func TestMemoryUpdate(t *testing.T) {
	db := testDB(t)
	repo := NewMemoryRepo(db)

	repo.Append("original fact", nil)
	entries, _ := repo.GetAll()
	id := entries[0].ID

	ok, err := repo.Update(id, "updated fact")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !ok {
		t.Error("update should return true")
	}

	entries, _ = repo.GetAll()
	if entries[0].Fact != "updated fact" {
		t.Errorf("fact = %q, want 'updated fact'", entries[0].Fact)
	}
}

func TestMemoryUpdateRejectsDuplicate(t *testing.T) {
	db := testDB(t)
	repo := NewMemoryRepo(db)

	repo.Append("fact one", nil)
	repo.Append("fact two", nil)
	entries, _ := repo.GetAll()

	// Try to update "fact two" to match "fact one".
	ok, err := repo.Update(entries[1].ID, "Fact One")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if ok {
		t.Error("update to duplicate should return false")
	}
}

// --- Session Repository ---

func TestSessionCRUD(t *testing.T) {
	db := testDB(t)
	repo := NewSessionRepo(db)

	// Get missing key.
	_, found, err := repo.GetSessionID("session:main")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if found {
		t.Error("should not find missing key")
	}

	// Save and get.
	if err := repo.SaveSessionID("session:main", "abc-123"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	val, found, err := repo.GetSessionID("session:main")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Error("should find saved key")
	}
	if val != "abc-123" {
		t.Errorf("val = %q, want 'abc-123'", val)
	}

	// Upsert.
	if err := repo.SaveSessionID("session:main", "def-456"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	val, _, _ = repo.GetSessionID("session:main")
	if val != "def-456" {
		t.Errorf("val = %q, want 'def-456'", val)
	}

	// Delete.
	if err := repo.DeleteSession("session:main"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, found, _ = repo.GetSessionID("session:main")
	if found {
		t.Error("should not find deleted key")
	}
}

// --- Whitelist Repository ---

func TestWhitelistCRUD(t *testing.T) {
	db := testDB(t)
	repo := NewWhitelistRepo(db)

	ok, _ := repo.IsWhitelisted("whatsapp", "123@s.whatsapp.net")
	if ok {
		t.Error("should not be whitelisted initially")
	}

	added, _ := repo.AddToWhitelist("whatsapp", "123@s.whatsapp.net")
	if !added {
		t.Error("first add should return true")
	}

	added, _ = repo.AddToWhitelist("whatsapp", "123@s.whatsapp.net")
	if added {
		t.Error("second add should return false (duplicate)")
	}

	ok, _ = repo.IsWhitelisted("whatsapp", "123@s.whatsapp.net")
	if !ok {
		t.Error("should be whitelisted")
	}

	removed, _ := repo.RemoveFromWhitelist("whatsapp", "123@s.whatsapp.net")
	if !removed {
		t.Error("remove should return true")
	}

	removed, _ = repo.RemoveFromWhitelist("whatsapp", "123@s.whatsapp.net")
	if removed {
		t.Error("remove non-existent should return false")
	}
}

// --- Outbox Repository ---

func TestOutboxEnqueueAndList(t *testing.T) {
	db := testDB(t)
	repo := NewOutboxRepo(db, 3)

	if err := repo.Enqueue("whatsapp", "user1", "hello", 0); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if err := repo.Enqueue("whatsapp", "user1", "world", 0); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	msgs, err := repo.ListPending("whatsapp")
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2", len(msgs))
	}
	if msgs[0].Text != "hello" || msgs[1].Text != "world" {
		t.Error("messages out of order")
	}
	if msgs[0].MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", msgs[0].MaxRetries)
	}
}

func TestOutboxAcknowledge(t *testing.T) {
	db := testDB(t)
	repo := NewOutboxRepo(db, 3)

	repo.Enqueue("whatsapp", "user1", "test", 0)
	msgs, _ := repo.ListPending("whatsapp")

	if err := repo.Acknowledge(msgs[0].ID); err != nil {
		t.Fatalf("Acknowledge: %v", err)
	}

	msgs, _ = repo.ListPending("whatsapp")
	if len(msgs) != 0 {
		t.Errorf("len = %d, want 0", len(msgs))
	}
}

func TestOutboxMarkRetry(t *testing.T) {
	db := testDB(t)
	repo := NewOutboxRepo(db, 3)

	repo.Enqueue("whatsapp", "user1", "test", 0)
	msgs, _ := repo.ListPending("whatsapp")

	// Set next_retry_at far in the future.
	if err := repo.MarkRetry(msgs[0].ID, 1, "2099-01-01T00:00:00Z"); err != nil {
		t.Fatalf("MarkRetry: %v", err)
	}

	// Should not appear in pending since next_retry_at > now.
	msgs, _ = repo.ListPending("whatsapp")
	if len(msgs) != 0 {
		t.Errorf("len = %d, want 0 (retry not yet due)", len(msgs))
	}
}

// --- Heartbeat Repository ---

func TestHeartbeatGetTasks(t *testing.T) {
	db := testDB(t)
	repo := NewHeartbeatRepo(db)

	count, _ := repo.GetTaskCount()
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}

	// Insert tasks directly.
	db.Conn().Exec("INSERT INTO heartbeat_tasks (task, enabled) VALUES ('check email', 1)")
	db.Conn().Exec("INSERT INTO heartbeat_tasks (task, enabled) VALUES ('disabled task', 0)")
	db.Conn().Exec("INSERT INTO heartbeat_tasks (task, enabled) VALUES ('review notes', 1)")

	tasks, err := repo.GetTasks()
	if err != nil {
		t.Fatalf("GetTasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("len = %d, want 2", len(tasks))
	}
	if tasks[0] != "check email" || tasks[1] != "review notes" {
		t.Errorf("tasks = %v", tasks)
	}

	count, _ = repo.GetTaskCount()
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

// --- Channel Repository ---

func TestChannelSaveAndGet(t *testing.T) {
	db := testDB(t)
	repo := NewChannelRepo(db)

	lc, err := repo.GetLastChannel()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if lc != nil {
		t.Error("should be nil initially")
	}

	if err := repo.SaveLastChannel("whatsapp", "user123"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	lc, err = repo.GetLastChannel()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if lc == nil {
		t.Fatal("should not be nil")
	}
	if lc.Channel != "whatsapp" || lc.UserID != "user123" {
		t.Errorf("got %+v", lc)
	}
}

// --- hasColumn validation ---

func TestHasColumn_RejectsInvalidTableName(t *testing.T) {
	db := testDB(t)

	// SQL injection attempt: should return error, not execute.
	invalidNames := []string{
		"memory; DROP TABLE memory--",
		"memory); DROP TABLE memory--",
		"table name with spaces",
		"robert');DROP TABLE students;--",
		"",
	}

	for _, name := range invalidNames {
		_, err := db.hasColumn(name, "fact")
		if err == nil {
			t.Errorf("hasColumn(%q, \"fact\") should return error for invalid table name", name)
		}
	}
}

func TestHasColumn_AcceptsValidTableName(t *testing.T) {
	db := testDB(t)

	// Valid table name should work normally.
	has, err := db.hasColumn("memory", "fact")
	if err != nil {
		t.Fatalf("hasColumn(\"memory\", \"fact\") error: %v", err)
	}
	if !has {
		t.Error("memory table should have 'fact' column")
	}

	has, err = db.hasColumn("memory", "nonexistent_col")
	if err != nil {
		t.Fatalf("hasColumn(\"memory\", \"nonexistent_col\") error: %v", err)
	}
	if has {
		t.Error("memory table should not have 'nonexistent_col' column")
	}
}

// --- Normalize ---

func TestNormalizeMemoryFact(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"User likes Coffee", "user likes coffee"},
		{"  extra   spaces  ", "extra spaces"},
		{"UPPER CASE", "upper case"},
	}

	for _, tt := range tests {
		got := NormalizeMemoryFact(tt.input)
		if got != tt.expect {
			t.Errorf("NormalizeMemoryFact(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}
