// Package scheduler provides periodic task execution with retry and failure notification.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pocketbrain/pocketbrain/internal/core"
)

// HeartbeatConfig holds tuning parameters for the heartbeat scheduler.
type HeartbeatConfig struct {
	// IntervalMinutes is the period between heartbeat runs. Minimum 1.
	IntervalMinutes int
	// BaseDelayMs is the starting backoff delay between retries (milliseconds).
	BaseDelayMs int
	// MaxDelayMs is the ceiling on backoff delay (milliseconds).
	MaxDelayMs int
	// NotifyAfterFailures is the consecutive-failure count that triggers an
	// outbox notification. Zero disables notifications.
	NotifyAfterFailures int
}

// HeartbeatScheduler runs heartbeat tasks at a fixed interval.
//
// It uses a time.Ticker so the interval is measured from the start of each
// tick, not from the end of the previous run. If a run is still in progress
// when the next tick fires the tick is silently skipped.
type HeartbeatScheduler struct {
	cfg     HeartbeatConfig
	runner  core.HeartbeatRunner
	outbox  core.OutboxRepository
	channel core.ChannelRepository
	log     *slog.Logger

	// running is 1 while a run goroutine is active.
	running atomic.Int32
	// stopCh is closed by Stop to signal the ticker loop to exit.
	stopCh chan struct{}
}

// NewHeartbeatScheduler creates a scheduler. Call Start to begin execution.
func NewHeartbeatScheduler(
	cfg HeartbeatConfig,
	runner core.HeartbeatRunner,
	outbox core.OutboxRepository,
	channel core.ChannelRepository,
	log *slog.Logger,
) *HeartbeatScheduler {
	return &HeartbeatScheduler{
		cfg:     cfg,
		runner:  runner,
		outbox:  outbox,
		channel: channel,
		log:     log,
		stopCh:  make(chan struct{}),
	}
}

// Start launches the scheduler loop in its own goroutine. The loop exits when
// ctx is cancelled or Stop is called.
func (s *HeartbeatScheduler) Start(ctx context.Context) {
	intervalMinutes := s.cfg.IntervalMinutes
	if intervalMinutes < 1 {
		intervalMinutes = 1
	}
	interval := time.Duration(intervalMinutes) * time.Minute

	s.log.Info("heartbeat scheduler started", "intervalMinutes", intervalMinutes)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Track failure state in the loop goroutine — no mutex needed because
		// the loop is the only writer. The run goroutine communicates back via
		// the resultCh channel.
		consecutiveFailures := 0
		notifiedForIncident := false

		for {
			select {
			case <-s.stopCh:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !s.running.CompareAndSwap(0, 1) {
					s.log.Warn("heartbeat tick skipped: previous run still active")
					continue
				}

				result, err := s.executeWithRetry(ctx)

				// running guard released inside executeWithRetry after the run
				// completes; reset it here instead so we can inspect err cleanly.
				s.running.Store(0)

				if err != nil {
					consecutiveFailures++
					s.log.Error("heartbeat run failed",
						"consecutiveFailures", consecutiveFailures,
						"error", err,
					)

					if s.cfg.NotifyAfterFailures > 0 &&
						consecutiveFailures >= s.cfg.NotifyAfterFailures &&
						!notifiedForIncident {
						if sent := s.notifyFailure(consecutiveFailures); sent {
							notifiedForIncident = true
						}
					}
				} else {
					s.log.Info("heartbeat run completed", "result", result)
					consecutiveFailures = 0
					notifiedForIncident = false
				}
			}
		}
	}()
}

// Stop signals the scheduler loop to exit. It is safe to call multiple times.
func (s *HeartbeatScheduler) Stop() {
	select {
	case <-s.stopCh:
		// already closed
	default:
		close(s.stopCh)
	}
	s.log.Info("heartbeat scheduler stopped")
}

// executeWithRetry runs the heartbeat task with up to 2 retries using
// exponential backoff. It releases the running guard before returning.
//
// Retry schedule (with defaults base=60s, max=1800s, factor=2):
//
//	attempt 1 → fail → sleep min(base*2^0, max) = base
//	attempt 2 → fail → sleep min(base*2^1, max) = base*2
//	attempt 3 → fail → return error
func (s *HeartbeatScheduler) executeWithRetry(ctx context.Context) (string, error) {
	const maxAttempts = 3 // 1 initial + 2 retries

	baseDelay := time.Duration(s.cfg.BaseDelayMs) * time.Millisecond
	if baseDelay <= 0 {
		baseDelay = 60 * time.Second
	}
	maxDelay := time.Duration(s.cfg.MaxDelayMs) * time.Millisecond
	if maxDelay <= 0 {
		maxDelay = 30 * time.Minute
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		s.log.Debug("heartbeat attempt", "attempt", attempt, "maxAttempts", maxAttempts)

		result, err := s.runner.RunHeartbeatTasks(ctx)
		if err == nil {
			return result, nil
		}

		lastErr = err
		retriesLeft := maxAttempts - attempt

		s.log.Warn("heartbeat attempt failed",
			"attempt", attempt,
			"retriesLeft", retriesLeft,
			"error", err,
		)

		if retriesLeft == 0 {
			break
		}

		// Exponential backoff: base * 2^(attempt-1), capped at maxDelay.
		delay := baseDelay * (1 << uint(attempt-1))
		if delay > maxDelay {
			delay = maxDelay
		}

		select {
		case <-time.After(delay):
		case <-s.stopCh:
			return "", fmt.Errorf("scheduler stopped during retry backoff")
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	return "", lastErr
}

// notifyFailure enqueues a failure notification to the last active channel.
// Returns true if the notification was successfully enqueued.
func (s *HeartbeatScheduler) notifyFailure(failureCount int) bool {
	target := s.resolveNotificationTarget()
	if target == nil {
		s.log.Warn("heartbeat consecutive failures but no valid notification target",
			"consecutiveFailures", failureCount,
		)
		return false
	}

	text := fmt.Sprintf(
		"Heartbeat has failed %d times in a row. Check logs for details.",
		failureCount,
	)

	if err := s.outbox.Enqueue(target.Channel, target.UserID, text, 0); err != nil {
		s.log.Error("heartbeat notification enqueue failed",
			"failureCount", failureCount,
			"error", err,
		)
		return false
	}

	s.log.Warn("heartbeat notification sent",
		"failureCount", failureCount,
		"channel", target.Channel,
	)
	return true
}

// resolveNotificationTarget returns a trimmed, non-empty LastChannel or nil.
func (s *HeartbeatScheduler) resolveNotificationTarget() *core.LastChannel {
	lc, err := s.channel.GetLastChannel()
	if err != nil {
		s.log.Warn("heartbeat: failed to read last channel", "error", err)
		return nil
	}
	if lc == nil {
		return nil
	}

	channel := strings.TrimSpace(lc.Channel)
	userID := strings.TrimSpace(lc.UserID)
	if channel == "" || userID == "" {
		return nil
	}

	return &core.LastChannel{Channel: channel, UserID: userID}
}
