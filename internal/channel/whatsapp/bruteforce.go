package whatsapp

import (
	"sync"
	"time"
)

// BruteForceGuard provides rate-limiting protection against brute-force
// pairing attempts. It tracks failed attempts per user and blocks users
// who exceed the maximum failures within a time window.
type BruteForceGuard struct {
	maxFailures int
	windowMs    int
	blockMs     int

	mu       sync.Mutex
	attempts map[string][]time.Time
	blocks   map[string]time.Time
}

// NewBruteForceGuard creates a BruteForceGuard.
//   - maxFailures: number of failed attempts before blocking.
//   - windowMs: time window (in ms) within which failures are counted.
//   - blockMs: how long (in ms) a user is blocked after exceeding max failures.
func NewBruteForceGuard(maxFailures, windowMs, blockMs int) *BruteForceGuard {
	return &BruteForceGuard{
		maxFailures: maxFailures,
		windowMs:    windowMs,
		blockMs:     blockMs,
		attempts:    make(map[string][]time.Time),
		blocks:      make(map[string]time.Time),
	}
}

// Check returns true if the user is allowed to attempt, false if blocked.
func (g *BruteForceGuard) Check(userID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()

	// Periodic sweep of expired entries.
	g.sweepExpiredLocked(now)

	// Check if user is currently blocked.
	if blockedUntil, ok := g.blocks[userID]; ok {
		if now.Before(blockedUntil) {
			return false
		}
		// Block expired, clean up.
		delete(g.blocks, userID)
		delete(g.attempts, userID)
	}

	return true
}

// RecordFailure records a failed attempt for the user. If the number of
// failures within the window exceeds maxFailures, the user is blocked.
func (g *BruteForceGuard) RecordFailure(userID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-time.Duration(g.windowMs) * time.Millisecond)

	// Prune old attempts outside the window.
	existing := g.attempts[userID]
	pruned := existing[:0]
	for _, t := range existing {
		if t.After(windowStart) {
			pruned = append(pruned, t)
		}
	}

	pruned = append(pruned, now)
	g.attempts[userID] = pruned

	// Block if threshold exceeded.
	if len(pruned) >= g.maxFailures {
		g.blocks[userID] = now.Add(time.Duration(g.blockMs) * time.Millisecond)
	}

	// Periodic sweep: clean up expired entries from other users.
	g.sweepExpiredLocked(now)
}

// sweepExpiredLocked removes expired blocks and empty attempt entries.
// Must be called with g.mu held.
func (g *BruteForceGuard) sweepExpiredLocked(now time.Time) {
	windowStart := now.Add(-time.Duration(g.windowMs) * time.Millisecond)

	for uid, blockedUntil := range g.blocks {
		if now.After(blockedUntil) {
			delete(g.blocks, uid)
		}
	}

	for uid, attempts := range g.attempts {
		// Remove entries with all attempts expired.
		hasRecent := false
		for _, t := range attempts {
			if t.After(windowStart) {
				hasRecent = true
				break
			}
		}
		if !hasRecent {
			delete(g.attempts, uid)
		}
	}
}

// RecordSuccess clears the failure history for the user.
func (g *BruteForceGuard) RecordSuccess(userID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.attempts, userID)
	delete(g.blocks, userID)
}
