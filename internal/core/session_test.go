package core

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"testing"
)

// mockSessionRepo implements SessionRepository for tests.
type mockSessionRepo struct {
	sessions map[string]string
	saveErr  error
	getErr   error
}

func newMockSessionRepo() *mockSessionRepo {
	return &mockSessionRepo{sessions: make(map[string]string)}
}

func (m *mockSessionRepo) GetSessionID(key string) (string, bool, error) {
	if m.getErr != nil {
		return "", false, m.getErr
	}
	v, ok := m.sessions[key]
	return v, ok, nil
}

func (m *mockSessionRepo) SaveSessionID(key, sessionID string) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.sessions[key] = sessionID
	return nil
}

func (m *mockSessionRepo) DeleteSession(key string) error {
	delete(m.sessions, key)
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestSessionManager_GetOrCreateMainSession_CreatesNew(t *testing.T) {
	repo := newMockSessionRepo()
	mgr := NewSessionManager(repo, testLogger())

	createCalled := false
	createFn := func(_ context.Context, title string) (string, error) {
		createCalled = true
		return "session-abc", nil
	}

	id, err := mgr.GetOrCreateMainSession(context.Background(), createFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "session-abc" {
		t.Errorf("expected session-abc, got %q", id)
	}
	if !createCalled {
		t.Error("expected createFn to be called")
	}
}

func TestSessionManager_GetOrCreateMainSession_ReturnsCached(t *testing.T) {
	repo := newMockSessionRepo()
	mgr := NewSessionManager(repo, testLogger())

	callCount := 0
	createFn := func(_ context.Context, title string) (string, error) {
		callCount++
		return "session-abc", nil
	}

	ctx := context.Background()
	first, err := mgr.GetOrCreateMainSession(ctx, createFn)
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	second, err := mgr.GetOrCreateMainSession(ctx, createFn)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if first != second {
		t.Errorf("expected same session ID, got %q and %q", first, second)
	}
	if callCount != 1 {
		t.Errorf("expected createFn called once, got %d", callCount)
	}
}

func TestSessionManager_GetOrCreateHeartbeatSession_CreatesNew(t *testing.T) {
	repo := newMockSessionRepo()
	mgr := NewSessionManager(repo, testLogger())

	createFn := func(_ context.Context, title string) (string, error) {
		return "hb-session-xyz", nil
	}

	id, err := mgr.GetOrCreateHeartbeatSession(context.Background(), createFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "hb-session-xyz" {
		t.Errorf("expected hb-session-xyz, got %q", id)
	}
	if _, ok := repo.sessions["session:heartbeat"]; !ok {
		t.Error("expected session:heartbeat to be stored in repo")
	}
}

func TestSessionManager_StartNewMainSession_ReplacesExisting(t *testing.T) {
	repo := newMockSessionRepo()
	repo.sessions["session:main"] = "old-session"
	mgr := NewSessionManager(repo, testLogger())

	callCount := 0
	createFn := func(_ context.Context, title string) (string, error) {
		callCount++
		return "new-session", nil
	}

	id, err := mgr.StartNewMainSession(context.Background(), "test", createFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "new-session" {
		t.Errorf("expected new-session, got %q", id)
	}
	if callCount != 1 {
		t.Errorf("expected createFn called once, got %d", callCount)
	}
	if repo.sessions["session:main"] != "new-session" {
		t.Errorf("expected session:main to be new-session, got %q", repo.sessions["session:main"])
	}
}

func TestSessionManager_GetOrCreateMainSession_NilCreateFn(t *testing.T) {
	repo := newMockSessionRepo()
	mgr := NewSessionManager(repo, testLogger())

	id, err := mgr.GetOrCreateMainSession(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty UUID fallback")
	}
}

func TestSessionManager_GetOrCreateMainSession_CreateFnError(t *testing.T) {
	repo := newMockSessionRepo()
	mgr := NewSessionManager(repo, testLogger())

	createFn := func(_ context.Context, title string) (string, error) {
		return "", errors.New("provider unavailable")
	}

	_, err := mgr.GetOrCreateMainSession(context.Background(), createFn)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSessionManager_GetOrCreateMainSession_SaveError(t *testing.T) {
	repo := newMockSessionRepo()
	repo.saveErr = errors.New("disk full")
	mgr := NewSessionManager(repo, testLogger())

	createFn := func(_ context.Context, title string) (string, error) {
		return "session-ok", nil
	}

	_, err := mgr.GetOrCreateMainSession(context.Background(), createFn)
	if err == nil {
		t.Fatal("expected error from save, got nil")
	}
}

func TestSessionManager_GetOrCreateMainSession_InitializesVersion(t *testing.T) {
	repo := newMockSessionRepo()
	mgr := NewSessionManager(repo, testLogger())

	createFn := func(_ context.Context, title string) (string, error) {
		return "session-abc", nil
	}

	if _, err := mgr.GetOrCreateMainSession(context.Background(), createFn); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	v, ok := repo.sessions["session:main:version"]
	if !ok {
		t.Fatal("expected session:main:version to be stored")
	}
	if v != "1" {
		t.Fatalf("version = %q, want 1", v)
	}
}

func TestSessionManager_StartNewMainSession_BumpsVersion(t *testing.T) {
	repo := newMockSessionRepo()
	repo.sessions["session:main"] = "old-session"
	repo.sessions["session:main:version"] = "7"
	mgr := NewSessionManager(repo, testLogger())

	createFn := func(_ context.Context, title string) (string, error) {
		return "new-session", nil
	}

	if _, err := mgr.StartNewMainSession(context.Background(), "test", createFn); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := mgr.GetMainSessionVersion()
	if err != nil {
		t.Fatalf("GetMainSessionVersion error: %v", err)
	}
	if got != 8 {
		t.Fatalf("version = %d, want 8", got)
	}
	if raw := repo.sessions["session:main:version"]; raw != strconv.FormatInt(got, 10) {
		t.Fatalf("stored version = %q, want %d", raw, got)
	}
}
