package core

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"strconv"
)

// SessionManager manages named OpenCode sessions stored in a SessionRepository.
// It maps logical keys ("main", "heartbeat") to provider-assigned session IDs.
type SessionManager struct {
	repo   SessionRepository
	logger *slog.Logger
}

const (
	mainSessionKey        = "session:main"
	mainSessionVersionKey = "session:main:version"
)

// NewSessionManager creates a SessionManager backed by the given repository.
func NewSessionManager(repo SessionRepository, logger *slog.Logger) *SessionManager {
	return &SessionManager{repo: repo, logger: logger}
}

// GetOrCreateMainSession returns the current main session ID, creating one if
// none exists.
func (m *SessionManager) GetOrCreateMainSession(ctx context.Context, createFn func(ctx context.Context, title string) (string, error)) (string, error) {
	return m.getOrCreate(ctx, "main", createFn)
}

// GetOrCreateHeartbeatSession returns the current heartbeat session ID,
// creating one if none exists.
func (m *SessionManager) GetOrCreateHeartbeatSession(ctx context.Context, createFn func(ctx context.Context, title string) (string, error)) (string, error) {
	return m.getOrCreate(ctx, "heartbeat", createFn)
}

// StartNewMainSession discards any stored main session ID and creates a fresh
// one.  reason is only used for logging.
func (m *SessionManager) StartNewMainSession(ctx context.Context, reason string, createFn func(ctx context.Context, title string) (string, error)) (string, error) {
	if reason == "" {
		reason = "manual"
	}

	sessionID, err := m.createSession(ctx, fmt.Sprintf("main:%s", reason), createFn)
	if err != nil {
		return "", err
	}

	if err := m.repo.SaveSessionID(mainSessionKey, sessionID); err != nil {
		return "", fmt.Errorf("save new main session: %w", err)
	}

	version, err := m.bumpMainSessionVersion()
	if err != nil {
		return "", fmt.Errorf("bump main session version: %w", err)
	}

	m.logger.InfoContext(ctx, "created new main session", "sessionID", sessionID, "reason", reason, "version", version)
	return sessionID, nil
}

// GetMainSessionVersion returns the persisted main-session version counter.
// Returns 0 when no version has been recorded yet.
func (m *SessionManager) GetMainSessionVersion() (int64, error) {
	raw, ok, err := m.repo.GetSessionID(mainSessionVersionKey)
	if err != nil {
		return 0, fmt.Errorf("get main session version: %w", err)
	}
	if !ok {
		return 0, nil
	}

	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse main session version: %w", err)
	}
	return n, nil
}

// getOrCreate returns the stored session ID for key, or creates a new one.
func (m *SessionManager) getOrCreate(ctx context.Context, key string, createFn func(ctx context.Context, title string) (string, error)) (string, error) {
	storeKey := "session:" + key

	existing, ok, err := m.repo.GetSessionID(storeKey)
	if err != nil {
		return "", fmt.Errorf("get session %q: %w", key, err)
	}
	if ok {
		return existing, nil
	}

	sessionID, err := m.createSession(ctx, key, createFn)
	if err != nil {
		return "", err
	}

	if err := m.repo.SaveSessionID(storeKey, sessionID); err != nil {
		return "", fmt.Errorf("save session %q: %w", key, err)
	}
	if key == "main" {
		if err := m.ensureMainSessionVersionInitialized(); err != nil {
			return "", fmt.Errorf("init main session version: %w", err)
		}
	}

	return sessionID, nil
}

// createSession calls createFn to obtain a new session ID.  If createFn is nil
// (no provider wired yet) a local UUID is generated as a fallback.
func (m *SessionManager) createSession(ctx context.Context, key string, createFn func(ctx context.Context, title string) (string, error)) (string, error) {
	title := "chat:" + key

	if createFn == nil {
		// No provider available yet – generate a local UUID placeholder.
		id := newUUID()
		m.logger.WarnContext(ctx, "no provider createFn; using local UUID", "key", key, "sessionID", id)
		return id, nil
	}

	id, err := createFn(ctx, title)
	if err != nil {
		return "", fmt.Errorf("create session %q: %w", key, err)
	}
	if id == "" {
		return "", fmt.Errorf("create session %q: provider returned empty ID", key)
	}

	m.logger.InfoContext(ctx, "created session", "key", key, "sessionID", id)
	return id, nil
}

// newUUID generates a random UUID v4 string without external dependencies.
func newUUID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

func (m *SessionManager) ensureMainSessionVersionInitialized() error {
	raw, ok, err := m.repo.GetSessionID(mainSessionVersionKey)
	if err != nil {
		return err
	}
	if ok && raw != "" {
		return nil
	}
	return m.repo.SaveSessionID(mainSessionVersionKey, "1")
}

func (m *SessionManager) bumpMainSessionVersion() (int64, error) {
	current, err := m.GetMainSessionVersion()
	if err != nil {
		return 0, err
	}
	next := current + 1
	if next <= 0 {
		next = 1
	}
	if err := m.repo.SaveSessionID(mainSessionVersionKey, strconv.FormatInt(next, 10)); err != nil {
		return 0, err
	}
	return next, nil
}
