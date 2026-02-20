package whatsapp

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/pocketbrain/pocketbrain/internal/core"
)

// ---------------------------------------------------------------------------
// Stubs / mocks (no external mocking library)
// ---------------------------------------------------------------------------

// stubWAClient implements WAClient for tests.
type stubWAClient struct {
	mu           sync.Mutex
	connected    bool
	connectErr   error
	sendErr      error
	sentMessages []sentMsg
}

type sentMsg struct {
	JID  string
	Text string
}

func (c *stubWAClient) Connect() error {
	if c.connectErr != nil {
		return c.connectErr
	}
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()
	return nil
}

func (c *stubWAClient) Disconnect() {
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
}

func (c *stubWAClient) SendText(jid, text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sendErr != nil {
		return c.sendErr
	}
	c.sentMessages = append(c.sentMessages, sentMsg{JID: jid, Text: text})
	return nil
}

func (c *stubWAClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

func (c *stubWAClient) sent() []sentMsg {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]sentMsg, len(c.sentMessages))
	copy(cp, c.sentMessages)
	return cp
}

// stubWhitelist implements core.WhitelistRepository.
type stubWhitelist struct {
	mu    sync.Mutex
	users map[string]bool // key = "channel:userID"
}

func newStubWhitelist() *stubWhitelist {
	return &stubWhitelist{users: make(map[string]bool)}
}

func (w *stubWhitelist) key(channel, userID string) string {
	return channel + ":" + userID
}

func (w *stubWhitelist) IsWhitelisted(channel, userID string) (bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.users[w.key(channel, userID)], nil
}

func (w *stubWhitelist) AddToWhitelist(channel, userID string) (bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	k := w.key(channel, userID)
	if w.users[k] {
		return false, nil // already present
	}
	w.users[k] = true
	return true, nil
}

func (w *stubWhitelist) RemoveFromWhitelist(channel, userID string) (bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	k := w.key(channel, userID)
	if !w.users[k] {
		return false, nil
	}
	delete(w.users, k)
	return true, nil
}

// stubMemoryRepo implements core.MemoryRepository.
type stubMemoryRepo struct {
	mu      sync.Mutex
	entries []core.MemoryEntry
	nextID  int64
}

func newStubMemoryRepo() *stubMemoryRepo {
	return &stubMemoryRepo{nextID: 1}
}

func (r *stubMemoryRepo) Append(fact string, source *string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, core.MemoryEntry{ID: r.nextID, Fact: fact, Source: source})
	r.nextID++
	return true, nil
}

func (r *stubMemoryRepo) Delete(id int64) (bool, error) { return true, nil }

func (r *stubMemoryRepo) Update(id int64, fact string) (bool, error) { return true, nil }

func (r *stubMemoryRepo) GetAll() ([]core.MemoryEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]core.MemoryEntry, len(r.entries))
	copy(cp, r.entries)
	return cp, nil
}

// stubSessionStarter implements SessionStarter.
type stubSessionStarter struct {
	mu     sync.Mutex
	called int
}

func (s *stubSessionStarter) StartNewSession(userID, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.called++
	return nil
}

func (s *stubSessionStarter) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.called
}

// stubOutboxRepo implements core.OutboxRepository.
type stubOutboxRepo struct {
	mu       sync.Mutex
	messages []core.OutboxMessage
	nextID   int64
	ackErr   error
}

func newStubOutboxRepo() *stubOutboxRepo {
	return &stubOutboxRepo{nextID: 1}
}

func (r *stubOutboxRepo) Enqueue(channel, userID, text string, maxRetries int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = append(r.messages, core.OutboxMessage{
		ID:         r.nextID,
		Channel:    channel,
		UserID:     userID,
		Text:       text,
		MaxRetries: maxRetries,
	})
	r.nextID++
	return nil
}

func (r *stubOutboxRepo) ListPending(channel string) ([]core.OutboxMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []core.OutboxMessage
	for _, m := range r.messages {
		if m.Channel == channel {
			result = append(result, m)
		}
	}
	return result, nil
}

func (r *stubOutboxRepo) Acknowledge(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ackErr != nil {
		return r.ackErr
	}
	filtered := r.messages[:0]
	for _, m := range r.messages {
		if m.ID != id {
			filtered = append(filtered, m)
		}
	}
	r.messages = filtered
	return nil
}

func (r *stubOutboxRepo) MarkRetry(id int64, retryCount int, nextRetryAt string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, m := range r.messages {
		if m.ID == id {
			r.messages[i].RetryCount = retryCount
			r.messages[i].NextRetryAt = &nextRetryAt
			break
		}
	}
	return nil
}

func (r *stubOutboxRepo) pendingCount(channel string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, m := range r.messages {
		if m.Channel == channel {
			count++
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func testLogger() *slog.Logger {
	return slog.Default()
}

// ---------------------------------------------------------------------------
// Adapter tests
// ---------------------------------------------------------------------------

func TestWhatsAppAdapter_Name_ReturnsWhatsapp(t *testing.T) {
	a := NewAdapter(&stubWAClient{}, testLogger())
	if got := a.Name(); got != "whatsapp" {
		t.Errorf("Name() = %q; want %q", got, "whatsapp")
	}
}

func TestWhatsAppAdapter_Stop_Idempotent(t *testing.T) {
	client := &stubWAClient{}
	a := NewAdapter(client, testLogger())

	handler := func(userID, text string) (string, error) { return "ok", nil }
	if err := a.Start(handler); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// First stop should succeed.
	if err := a.Stop(); err != nil {
		t.Fatalf("first Stop: %v", err)
	}
	// Second stop should also succeed (idempotent).
	if err := a.Stop(); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CommandRouter tests
// ---------------------------------------------------------------------------

func TestCommandRouter_Pair_ValidToken(t *testing.T) {
	wl := newStubWhitelist()
	guard := NewBruteForceGuard(5, 300000, 900000)
	router := &CommandRouter{
		pairToken:  "secret123",
		guard:      guard,
		whitelist:  wl,
		memoryRepo: newStubMemoryRepo(),
		sessionMgr: &stubSessionStarter{},
		logger:     testLogger(),
	}

	resp, handled := router.Route("user@test", "/pair secret123")
	if !handled {
		t.Fatal("expected command to be handled")
	}
	if resp == "" {
		t.Fatal("expected non-empty response")
	}

	// User should now be whitelisted.
	ok, _ := wl.IsWhitelisted("whatsapp", "user@test")
	if !ok {
		t.Error("user should be whitelisted after valid /pair")
	}
}

func TestCommandRouter_Pair_InvalidToken(t *testing.T) {
	wl := newStubWhitelist()
	guard := NewBruteForceGuard(5, 300000, 900000)
	router := &CommandRouter{
		pairToken:  "secret123",
		guard:      guard,
		whitelist:  wl,
		memoryRepo: newStubMemoryRepo(),
		sessionMgr: &stubSessionStarter{},
		logger:     testLogger(),
	}

	resp, handled := router.Route("user@test", "/pair wrongtoken")
	if !handled {
		t.Fatal("expected command to be handled")
	}
	if resp == "" {
		t.Fatal("expected non-empty rejection response")
	}

	// User should NOT be whitelisted.
	ok, _ := wl.IsWhitelisted("whatsapp", "user@test")
	if ok {
		t.Error("user should not be whitelisted after invalid /pair")
	}
}

func TestCommandRouter_Pair_BruteForceProtection(t *testing.T) {
	wl := newStubWhitelist()
	guard := NewBruteForceGuard(3, 300000, 900000) // block after 3 failures
	router := &CommandRouter{
		pairToken:  "secret123",
		guard:      guard,
		whitelist:  wl,
		memoryRepo: newStubMemoryRepo(),
		sessionMgr: &stubSessionStarter{},
		logger:     testLogger(),
	}

	// Exhaust failures.
	for i := 0; i < 3; i++ {
		router.Route("attacker@test", "/pair wrong")
	}

	// Next attempt should be blocked even with the correct token.
	resp, handled := router.Route("attacker@test", "/pair secret123")
	if !handled {
		t.Fatal("expected command to be handled")
	}

	// Should still be blocked.
	ok, _ := wl.IsWhitelisted("whatsapp", "attacker@test")
	if ok {
		t.Error("attacker should not be whitelisted while blocked")
	}
	_ = resp
}

func TestCommandRouter_New_StartsNewSession(t *testing.T) {
	sessionStarter := &stubSessionStarter{}
	router := &CommandRouter{
		pairToken:  "token",
		guard:      NewBruteForceGuard(5, 300000, 900000),
		whitelist:  newStubWhitelist(),
		memoryRepo: newStubMemoryRepo(),
		sessionMgr: sessionStarter,
		logger:     testLogger(),
	}

	resp, handled := router.Route("user@test", "/new")
	if !handled {
		t.Fatal("expected /new to be handled")
	}
	if resp == "" {
		t.Fatal("expected non-empty response")
	}
	if sessionStarter.callCount() != 1 {
		t.Errorf("StartNewSession called %d times; want 1", sessionStarter.callCount())
	}
}

func TestCommandRouter_Remember_SavesMemory(t *testing.T) {
	memRepo := newStubMemoryRepo()
	router := &CommandRouter{
		pairToken:  "token",
		guard:      NewBruteForceGuard(5, 300000, 900000),
		whitelist:  newStubWhitelist(),
		memoryRepo: memRepo,
		sessionMgr: &stubSessionStarter{},
		logger:     testLogger(),
	}

	resp, handled := router.Route("user@test", "/remember The sky is blue")
	if !handled {
		t.Fatal("expected /remember to be handled")
	}
	if resp == "" {
		t.Fatal("expected non-empty response")
	}

	entries, _ := memRepo.GetAll()
	if len(entries) != 1 {
		t.Fatalf("expected 1 memory entry; got %d", len(entries))
	}
	if entries[0].Fact != "The sky is blue" {
		t.Errorf("memory fact = %q; want %q", entries[0].Fact, "The sky is blue")
	}
}

// ---------------------------------------------------------------------------
// MessageProcessor tests
// ---------------------------------------------------------------------------

func TestMessageProcessor_WhitelistedUser_Processes(t *testing.T) {
	wl := newStubWhitelist()
	wl.AddToWhitelist("whatsapp", "user@test")

	var handlerCalled bool
	handler := func(userID, text string) (string, error) {
		handlerCalled = true
		return "reply", nil
	}

	router := &CommandRouter{
		pairToken:  "token",
		guard:      NewBruteForceGuard(5, 300000, 900000),
		whitelist:  wl,
		memoryRepo: newStubMemoryRepo(),
		sessionMgr: &stubSessionStarter{},
		logger:     testLogger(),
	}

	mp := NewMessageProcessor(wl, router, handler, testLogger())
	resp, err := mp.Process("user@test", "hello")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !handlerCalled {
		t.Error("expected handler to be called for whitelisted user")
	}
	if resp != "reply" {
		t.Errorf("response = %q; want %q", resp, "reply")
	}
}

func TestMessageProcessor_NonWhitelistedUser_Rejects(t *testing.T) {
	wl := newStubWhitelist()

	var handlerCalled bool
	handler := func(userID, text string) (string, error) {
		handlerCalled = true
		return "reply", nil
	}

	router := &CommandRouter{
		pairToken:  "token",
		guard:      NewBruteForceGuard(5, 300000, 900000),
		whitelist:  wl,
		memoryRepo: newStubMemoryRepo(),
		sessionMgr: &stubSessionStarter{},
		logger:     testLogger(),
	}

	mp := NewMessageProcessor(wl, router, handler, testLogger())
	resp, err := mp.Process("stranger@test", "hello")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if handlerCalled {
		t.Error("handler should NOT be called for non-whitelisted user")
	}
	// Should get an empty response (ignored) or a rejection.
	_ = resp
}

func TestMessageProcessor_EmptyMessage_Ignored(t *testing.T) {
	wl := newStubWhitelist()
	wl.AddToWhitelist("whatsapp", "user@test")

	var handlerCalled bool
	handler := func(userID, text string) (string, error) {
		handlerCalled = true
		return "reply", nil
	}

	router := &CommandRouter{
		pairToken:  "token",
		guard:      NewBruteForceGuard(5, 300000, 900000),
		whitelist:  wl,
		memoryRepo: newStubMemoryRepo(),
		sessionMgr: &stubSessionStarter{},
		logger:     testLogger(),
	}

	mp := NewMessageProcessor(wl, router, handler, testLogger())
	resp, err := mp.Process("user@test", "")
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if handlerCalled {
		t.Error("handler should NOT be called for empty message")
	}
	if resp != "" {
		t.Errorf("response = %q; want empty", resp)
	}
}

// ---------------------------------------------------------------------------
// OutboxProcessor tests
// ---------------------------------------------------------------------------

func TestOutboxProcessor_DeliversPending(t *testing.T) {
	client := &stubWAClient{connected: true}
	outbox := newStubOutboxRepo()
	outbox.Enqueue("whatsapp", "user@test", "hello", 3)

	proc := NewOutboxProcessor(outbox, client, testLogger())
	if err := proc.ProcessPending(); err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}

	msgs := client.sent()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 sent message; got %d", len(msgs))
	}
	if msgs[0].JID != "user@test" || msgs[0].Text != "hello" {
		t.Errorf("sent = %+v; want JID=user@test Text=hello", msgs[0])
	}

	// Message should be acknowledged (removed).
	if outbox.pendingCount("whatsapp") != 0 {
		t.Error("expected outbox to be empty after delivery")
	}
}

func TestOutboxProcessor_RetriesOnFailure(t *testing.T) {
	client := &stubWAClient{connected: true, sendErr: errors.New("network error")}
	outbox := newStubOutboxRepo()
	outbox.Enqueue("whatsapp", "user@test", "hello", 3)

	proc := NewOutboxProcessor(outbox, client, testLogger())
	err := proc.ProcessPending()
	// ProcessPending should not return an error; it handles failures internally.
	if err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}

	// Message should still be pending (retried, not acknowledged).
	if outbox.pendingCount("whatsapp") != 1 {
		t.Error("expected message to remain pending after send failure")
	}
}

// ---------------------------------------------------------------------------
// Constant-time token comparison tests
// ---------------------------------------------------------------------------

func TestConstantTimeTokenCompare_Equal(t *testing.T) {
	if !constantTimeTokenCompare("secret123", "secret123") {
		t.Error("identical tokens should match")
	}
}

func TestConstantTimeTokenCompare_NotEqual(t *testing.T) {
	if constantTimeTokenCompare("secret123", "wrong") {
		t.Error("different tokens should not match")
	}
}

func TestConstantTimeTokenCompare_EmptyBoth(t *testing.T) {
	if !constantTimeTokenCompare("", "") {
		t.Error("two empty strings should match")
	}
}

func TestConstantTimeTokenCompare_OneEmpty(t *testing.T) {
	if constantTimeTokenCompare("secret", "") {
		t.Error("empty vs non-empty should not match")
	}
}

// ---------------------------------------------------------------------------
// BruteForceGuard tests
// ---------------------------------------------------------------------------

func TestBruteForceGuard_BlocksAfterMaxFailures(t *testing.T) {
	guard := NewBruteForceGuard(3, 300000, 900000) // 3 failures, 5min window, 15min block

	// First 3 attempts should be allowed.
	for i := 0; i < 3; i++ {
		if !guard.Check("user1") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
		guard.RecordFailure("user1")
	}

	// 4th attempt should be blocked.
	if guard.Check("user1") {
		t.Error("user should be blocked after max failures")
	}
}

func TestBruteForceGuard_CleansUpExpiredEntries(t *testing.T) {
	// Use tiny windows so entries expire immediately.
	guard := NewBruteForceGuard(3, 1, 1) // 3 failures, 1ms window, 1ms block

	// Generate failures from many unique users.
	for i := 0; i < 100; i++ {
		userID := fmt.Sprintf("user-%d", i)
		guard.RecordFailure(userID)
	}

	// Wait for all windows and blocks to expire.
	time.Sleep(10 * time.Millisecond)

	// Trigger cleanup by calling Check on any user.
	guard.Check("trigger-cleanup")

	// After cleanup, expired entries should be removed.
	guard.mu.Lock()
	attemptsLen := len(guard.attempts)
	blocksLen := len(guard.blocks)
	guard.mu.Unlock()

	// Should have cleaned up the 100 expired user entries.
	// Allow some slack — the "trigger-cleanup" user may be present.
	if attemptsLen > 5 {
		t.Errorf("attempts map has %d entries; expected most expired entries to be cleaned up", attemptsLen)
	}
	if blocksLen > 5 {
		t.Errorf("blocks map has %d entries; expected most expired entries to be cleaned up", blocksLen)
	}
}

func TestBruteForceGuard_ResetsAfterWindow(t *testing.T) {
	// Use a tiny window (1ms) so it expires immediately.
	guard := NewBruteForceGuard(3, 1, 1) // 3 failures, 1ms window, 1ms block

	for i := 0; i < 3; i++ {
		guard.Check("user1")
		guard.RecordFailure("user1")
	}

	// Wait for the block to expire.
	time.Sleep(5 * time.Millisecond)

	if !guard.Check("user1") {
		t.Error("user should be unblocked after block duration expires")
	}
}
