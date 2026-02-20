package core

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// mockProvider implements Provider for tests.
type mockProvider struct {
	sendMessageFn   func(ctx context.Context, sessionID, system, userText string) (string, error)
	sendNoReplyFn   func(ctx context.Context, sessionID, userText string) error
	createSessionFn func(ctx context.Context, title string) (string, error)
	recentContextFn func(ctx context.Context, sessionID string) (string, error)
}

func (m *mockProvider) SendMessage(ctx context.Context, sessionID, system, userText string) (string, error) {
	if m.sendMessageFn != nil {
		return m.sendMessageFn(ctx, sessionID, system, userText)
	}
	return "default reply", nil
}

func (m *mockProvider) SendMessageNoReply(ctx context.Context, sessionID, userText string) error {
	if m.sendNoReplyFn != nil {
		return m.sendNoReplyFn(ctx, sessionID, userText)
	}
	return nil
}

func (m *mockProvider) CreateSession(ctx context.Context, title string) (string, error) {
	if m.createSessionFn != nil {
		return m.createSessionFn(ctx, title)
	}
	return "mock-session-id", nil
}

func (m *mockProvider) RecentContext(ctx context.Context, sessionID string) (string, error) {
	if m.recentContextFn != nil {
		return m.recentContextFn(ctx, sessionID)
	}
	return "", nil
}

// mockMemoryRepo implements MemoryRepository for tests.
type mockMemoryRepo struct {
	entries []MemoryEntry
	err     error
}

func (m *mockMemoryRepo) Append(fact string, source *string) (bool, error) { return true, nil }
func (m *mockMemoryRepo) Delete(id int64) (bool, error)                    { return true, nil }
func (m *mockMemoryRepo) Update(id int64, fact string) (bool, error)       { return true, nil }
func (m *mockMemoryRepo) GetAll() ([]MemoryEntry, error) {
	return m.entries, m.err
}

// mockChannelRepo implements ChannelRepository for tests.
type mockChannelRepo struct {
	savedChannel string
	savedUserID  string
	saveErr      error
}

func (m *mockChannelRepo) SaveLastChannel(channel, userID string) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.savedChannel = channel
	m.savedUserID = userID
	return nil
}

func (m *mockChannelRepo) GetLastChannel() (*LastChannel, error) {
	return &LastChannel{Channel: m.savedChannel, UserID: m.savedUserID}, nil
}

// mockHeartbeatRepo implements HeartbeatRepository for tests.
type mockHeartbeatRepo struct {
	tasks    []string
	tasksErr error
}

func (m *mockHeartbeatRepo) GetTasks() ([]string, error) {
	return m.tasks, m.tasksErr
}

func (m *mockHeartbeatRepo) GetTaskCount() (int, error) {
	return len(m.tasks), m.tasksErr
}

// buildTestCore builds an AssistantCore with sensible test defaults. Callers
// may override individual fields via the options struct after calling this.
func buildTestCore(provider Provider, memRepo MemoryRepository, chanRepo ChannelRepository, hbRepo HeartbeatRepository) *AssistantCore {
	sessionRepo := newMockSessionRepo()
	sessionMgr := NewSessionManager(sessionRepo, testLogger())
	pb := NewPromptBuilder(PromptBuilderOptions{HeartbeatIntervalMinutes: 30})
	return NewAssistantCore(AssistantCoreOptions{
		Provider:      provider,
		SessionMgr:    sessionMgr,
		PromptBuilder: pb,
		MemoryRepo:    memRepo,
		ChannelRepo:   chanRepo,
		HeartbeatRepo: hbRepo,
		Logger:        testLogger(),
	})
}

func defaultMocks() (*mockProvider, *mockMemoryRepo, *mockChannelRepo, *mockHeartbeatRepo) {
	return &mockProvider{}, &mockMemoryRepo{}, &mockChannelRepo{}, &mockHeartbeatRepo{}
}

func TestAssistantCore_Ask_Success(t *testing.T) {
	prov, mem, ch, hb := defaultMocks()
	prov.sendMessageFn = func(_ context.Context, _, _, _ string) (string, error) {
		return "  hello world  ", nil
	}
	core := buildTestCore(prov, mem, ch, hb)

	reply, err := core.Ask(context.Background(), AssistantInput{Channel: "slack", UserID: "u1", Text: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "hello world" {
		t.Errorf("expected trimmed reply 'hello world', got %q", reply)
	}
}

func TestAssistantCore_Ask_CreatesSession(t *testing.T) {
	prov, mem, ch, hb := defaultMocks()
	createCalled := false
	prov.createSessionFn = func(_ context.Context, title string) (string, error) {
		createCalled = true
		return "new-sess", nil
	}
	core := buildTestCore(prov, mem, ch, hb)

	_, err := core.Ask(context.Background(), AssistantInput{Channel: "slack", UserID: "u1", Text: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !createCalled {
		t.Error("expected CreateSession to be called for new session")
	}
}

func TestAssistantCore_Ask_ProviderError(t *testing.T) {
	prov, mem, ch, hb := defaultMocks()
	prov.sendMessageFn = func(_ context.Context, _, _, _ string) (string, error) {
		return "", errors.New("network error")
	}
	core := buildTestCore(prov, mem, ch, hb)

	reply, err := core.Ask(context.Background(), AssistantInput{Channel: "slack", UserID: "u1", Text: "hi"})
	if err != nil {
		t.Fatalf("expected nil error (friendly message), got: %v", err)
	}
	if reply == "" {
		t.Error("expected friendly error message, got empty string")
	}
	if strings.Contains(reply, "network error") {
		t.Error("expected friendly message, not raw error")
	}
}

func TestAssistantCore_Ask_EmptyReply(t *testing.T) {
	prov, mem, ch, hb := defaultMocks()
	prov.sendMessageFn = func(_ context.Context, _, _, _ string) (string, error) {
		return "   ", nil // whitespace only -> trimmed to ""
	}
	core := buildTestCore(prov, mem, ch, hb)

	reply, err := core.Ask(context.Background(), AssistantInput{Channel: "slack", UserID: "u1", Text: "hi"})
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if reply == "" {
		t.Error("expected friendly message for empty reply, got empty string")
	}
}

func TestAssistantCore_Ask_SavesLastChannel(t *testing.T) {
	prov, mem, ch, hb := defaultMocks()
	core := buildTestCore(prov, mem, ch, hb)

	_, err := core.Ask(context.Background(), AssistantInput{Channel: "telegram", UserID: "u42", Text: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch.savedChannel != "telegram" {
		t.Errorf("expected saved channel 'telegram', got %q", ch.savedChannel)
	}
	if ch.savedUserID != "u42" {
		t.Errorf("expected saved userID 'u42', got %q", ch.savedUserID)
	}
}

func TestAssistantCore_Ask_IncludesMemoryInSystem(t *testing.T) {
	prov, mem, ch, hb := defaultMocks()
	mem.entries = []MemoryEntry{
		{ID: 1, Fact: "user loves coffee"},
	}
	capturedSystem := ""
	prov.sendMessageFn = func(_ context.Context, _, system, _ string) (string, error) {
		capturedSystem = system
		return "ok", nil
	}
	core := buildTestCore(prov, mem, ch, hb)

	_, err := core.Ask(context.Background(), AssistantInput{Channel: "slack", UserID: "u1", Text: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(capturedSystem, "user loves coffee") {
		t.Error("expected memory entry in system prompt")
	}
}

func TestAssistantCore_RunHeartbeatTasks_Success(t *testing.T) {
	prov, mem, ch, hb := defaultMocks()
	hb.tasks = []string{"check metrics", "review alerts"}
	callCount := 0
	prov.sendMessageFn = func(_ context.Context, _, _, _ string) (string, error) {
		callCount++
		return "all clear", nil
	}
	core := buildTestCore(prov, mem, ch, hb)

	result, err := core.RunHeartbeatTasks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "2") {
		t.Errorf("expected task count in result, got %q", result)
	}
	if callCount < 1 {
		t.Error("expected provider SendMessage to be called at least once")
	}
}

func TestAssistantCore_RunHeartbeatTasks_NoTasks(t *testing.T) {
	prov, mem, ch, hb := defaultMocks()
	hb.tasks = []string{}
	core := buildTestCore(prov, mem, ch, hb)

	result, err := core.RunHeartbeatTasks(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(strings.ToLower(result), "skipped") {
		t.Errorf("expected skip message, got %q", result)
	}
}

func TestAssistantCore_RunHeartbeatTasks_ProviderError(t *testing.T) {
	prov, mem, ch, hb := defaultMocks()
	hb.tasks = []string{"task-a"}
	prov.sendMessageFn = func(_ context.Context, _, _, _ string) (string, error) {
		return "", errors.New("provider down")
	}
	core := buildTestCore(prov, mem, ch, hb)

	result, err := core.RunHeartbeatTasks(context.Background())
	if err != nil {
		t.Fatalf("expected nil error (friendly message), got: %v", err)
	}
	if result == "" {
		t.Error("expected friendly result message, got empty string")
	}
}

func TestAssistantCore_StartNewMainSession_Success(t *testing.T) {
	prov, mem, ch, hb := defaultMocks()
	prov.createSessionFn = func(_ context.Context, title string) (string, error) {
		return "fresh-session", nil
	}
	core := buildTestCore(prov, mem, ch, hb)

	id, err := core.StartNewMainSession(context.Background(), "test reset")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "fresh-session" {
		t.Errorf("expected fresh-session, got %q", id)
	}
}
