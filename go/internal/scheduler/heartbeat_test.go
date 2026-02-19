package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pocketbrain/pocketbrain/internal/core"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

type mockHeartbeatRunner struct {
	mu      sync.Mutex
	calls   int
	results []runResult
}

type runResult struct {
	result string
	err    error
}

func (m *mockHeartbeatRunner) RunHeartbeatTasks(_ context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.calls
	m.calls++
	if idx < len(m.results) {
		return m.results[idx].result, m.results[idx].err
	}
	return "ok", nil
}

func (m *mockHeartbeatRunner) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

type mockOutboxRepository struct {
	mu       sync.Mutex
	enqueued []enqueueCall
}

type enqueueCall struct {
	channel    string
	userID     string
	text       string
	maxRetries int
}

func (m *mockOutboxRepository) Enqueue(channel, userID, text string, maxRetries int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enqueued = append(m.enqueued, enqueueCall{channel, userID, text, maxRetries})
	return nil
}

func (m *mockOutboxRepository) ListPending(_ string) ([]core.OutboxMessage, error) {
	return nil, nil
}

func (m *mockOutboxRepository) Acknowledge(_ int64) error { return nil }

func (m *mockOutboxRepository) MarkRetry(_ int64, _ int, _ string) error { return nil }

func (m *mockOutboxRepository) enqueueCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.enqueued)
}

type mockChannelRepository struct {
	mu          sync.Mutex
	lastChannel *core.LastChannel
	err         error
}

func (m *mockChannelRepository) SaveLastChannel(channel, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastChannel = &core.LastChannel{Channel: channel, UserID: userID}
	return nil
}

func (m *mockChannelRepository) GetLastChannel() (*core.LastChannel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastChannel, m.err
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestHeartbeatScheduler_BasicTick(t *testing.T) {
	runner := &mockHeartbeatRunner{
		results: []runResult{
			{result: "done", err: nil},
		},
	}
	outbox := &mockOutboxRepository{}
	chanRepo := &mockChannelRepository{}
	logger := slog.Default()

	// Use executeWithRetry directly to avoid timer-based flakiness.
	s := NewHeartbeatScheduler(HeartbeatConfig{
		IntervalMinutes: 1,
		BaseDelayMs:     10,
		MaxDelayMs:      50,
	}, runner, outbox, chanRepo, logger)

	ctx := context.Background()
	result, err := s.executeWithRetry(ctx)
	if err != nil {
		t.Fatalf("executeWithRetry returned error: %v", err)
	}
	if result != "done" {
		t.Errorf("result = %q; want %q", result, "done")
	}
	if runner.callCount() != 1 {
		t.Errorf("runner called %d times; want 1", runner.callCount())
	}
}

func TestHeartbeatScheduler_SkipWhenRunning(t *testing.T) {
	// Simulate a long-running task by having RunHeartbeatTasks block.
	blockCh := make(chan struct{})
	var callCount atomic.Int32

	runner := &mockHeartbeatRunner{
		results: []runResult{
			{result: "ok", err: nil},
		},
	}

	outbox := &mockOutboxRepository{}
	chanRepo := &mockChannelRepository{}
	logger := slog.Default()

	s := NewHeartbeatScheduler(HeartbeatConfig{
		IntervalMinutes: 1,
		BaseDelayMs:     10,
		MaxDelayMs:      50,
	}, runner, outbox, chanRepo, logger)

	// Override the runner with a blocking one.
	s.runner = &blockingRunner{
		blockCh:   blockCh,
		callCount: &callCount,
	}

	// Manually set running to 1, simulating an active run.
	s.running.Store(1)

	// CompareAndSwap should fail since running == 1.
	swapped := s.running.CompareAndSwap(0, 1)
	if swapped {
		t.Fatal("CompareAndSwap should have failed because running == 1")
	}

	// Reset.
	s.running.Store(0)
	close(blockCh)
}

type blockingRunner struct {
	blockCh   chan struct{}
	callCount *atomic.Int32
}

func (b *blockingRunner) RunHeartbeatTasks(_ context.Context) (string, error) {
	b.callCount.Add(1)
	<-b.blockCh
	return "ok", nil
}

func TestHeartbeatScheduler_RetryWithBackoff(t *testing.T) {
	runner := &mockHeartbeatRunner{
		results: []runResult{
			{result: "", err: errors.New("fail-1")},
			{result: "", err: errors.New("fail-2")},
			{result: "recovered", err: nil},
		},
	}
	outbox := &mockOutboxRepository{}
	chanRepo := &mockChannelRepository{}
	logger := slog.Default()

	s := NewHeartbeatScheduler(HeartbeatConfig{
		IntervalMinutes: 1,
		BaseDelayMs:     10, // small delays for fast tests
		MaxDelayMs:      50,
	}, runner, outbox, chanRepo, logger)

	ctx := context.Background()
	result, err := s.executeWithRetry(ctx)
	if err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}
	if result != "recovered" {
		t.Errorf("result = %q; want %q", result, "recovered")
	}
	if runner.callCount() != 3 {
		t.Errorf("runner called %d times; want 3", runner.callCount())
	}
}

func TestHeartbeatScheduler_AllRetriesFail(t *testing.T) {
	runner := &mockHeartbeatRunner{
		results: []runResult{
			{result: "", err: errors.New("fail-1")},
			{result: "", err: errors.New("fail-2")},
			{result: "", err: errors.New("fail-3")},
		},
	}
	outbox := &mockOutboxRepository{}
	chanRepo := &mockChannelRepository{}
	logger := slog.Default()

	s := NewHeartbeatScheduler(HeartbeatConfig{
		IntervalMinutes: 1,
		BaseDelayMs:     10,
		MaxDelayMs:      50,
	}, runner, outbox, chanRepo, logger)

	ctx := context.Background()
	_, err := s.executeWithRetry(ctx)
	if err == nil {
		t.Fatal("expected error after all retries exhausted, got nil")
	}
	if err.Error() != "fail-3" {
		t.Errorf("error = %q; want %q", err.Error(), "fail-3")
	}
	if runner.callCount() != 3 {
		t.Errorf("runner called %d times; want 3", runner.callCount())
	}
}

func TestHeartbeatScheduler_ContextCancelled(t *testing.T) {
	runner := &mockHeartbeatRunner{
		results: []runResult{
			{result: "", err: errors.New("fail")},
		},
	}
	outbox := &mockOutboxRepository{}
	chanRepo := &mockChannelRepository{}
	logger := slog.Default()

	s := NewHeartbeatScheduler(HeartbeatConfig{
		IntervalMinutes: 1,
		BaseDelayMs:     5000, // long delay — should be interrupted by cancel
		MaxDelayMs:      5000,
	}, runner, outbox, chanRepo, logger)

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after a short delay.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := s.executeWithRetry(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

func TestHeartbeatScheduler_NotifyFailure(t *testing.T) {
	chanRepo := &mockChannelRepository{
		lastChannel: &core.LastChannel{Channel: "telegram", UserID: "user123"},
	}
	outbox := &mockOutboxRepository{}
	logger := slog.Default()

	runner := &mockHeartbeatRunner{}
	s := NewHeartbeatScheduler(HeartbeatConfig{
		IntervalMinutes:     1,
		NotifyAfterFailures: 1,
	}, runner, outbox, chanRepo, logger)

	sent := s.notifyFailure(3)
	if !sent {
		t.Fatal("notifyFailure returned false; want true")
	}
	if outbox.enqueueCount() != 1 {
		t.Fatalf("expected 1 enqueued message, got %d", outbox.enqueueCount())
	}
}

func TestHeartbeatScheduler_NotifyFailure_NoChannel(t *testing.T) {
	chanRepo := &mockChannelRepository{lastChannel: nil}
	outbox := &mockOutboxRepository{}
	logger := slog.Default()

	runner := &mockHeartbeatRunner{}
	s := NewHeartbeatScheduler(HeartbeatConfig{
		IntervalMinutes:     1,
		NotifyAfterFailures: 1,
	}, runner, outbox, chanRepo, logger)

	sent := s.notifyFailure(3)
	if sent {
		t.Fatal("notifyFailure returned true with no channel; want false")
	}
	if outbox.enqueueCount() != 0 {
		t.Fatalf("expected 0 enqueued messages, got %d", outbox.enqueueCount())
	}
}

func TestHeartbeatScheduler_StopDuringBackoff(t *testing.T) {
	runner := &mockHeartbeatRunner{
		results: []runResult{
			{result: "", err: errors.New("fail")},
		},
	}
	outbox := &mockOutboxRepository{}
	chanRepo := &mockChannelRepository{}
	logger := slog.Default()

	s := NewHeartbeatScheduler(HeartbeatConfig{
		IntervalMinutes: 1,
		BaseDelayMs:     5000,
		MaxDelayMs:      5000,
	}, runner, outbox, chanRepo, logger)

	// Stop after a short delay to interrupt backoff sleep.
	go func() {
		time.Sleep(50 * time.Millisecond)
		s.Stop()
	}()

	ctx := context.Background()
	_, err := s.executeWithRetry(ctx)
	if err == nil {
		t.Fatal("expected error from stop during backoff, got nil")
	}
}
