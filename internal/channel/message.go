package channel

import (
	"log/slog"
	"strings"
	"sync"
	"time"
)

// MessageChunker splits long messages into sendable chunks.
type MessageChunker struct {
	MaxLength        int
	NewlineThreshold float64 // 0.0–1.0, default 0.5
}

// NewMessageChunker creates a chunker with the given max length.
func NewMessageChunker(maxLength int) *MessageChunker {
	if maxLength <= 0 {
		maxLength = 3500
	}
	return &MessageChunker{
		MaxLength:        maxLength,
		NewlineThreshold: 0.5,
	}
}

// Split breaks text into chunks no longer than MaxLength.
func (c *MessageChunker) Split(text string) []string {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return nil
	}
	if len(text) <= c.MaxLength {
		return []string{text}
	}

	var chunks []string
	remaining := text

	for len(remaining) > 0 {
		if len(remaining) <= c.MaxLength {
			trimmed := strings.TrimSpace(remaining)
			if trimmed != "" {
				chunks = append(chunks, trimmed)
			}
			break
		}

		chunk := remaining[:c.MaxLength]
		cutAt := -1

		// Try to split at a newline within the threshold zone.
		threshold := int(float64(c.MaxLength) * c.NewlineThreshold)
		nlIdx := strings.LastIndex(chunk[threshold:], "\n")
		if nlIdx >= 0 {
			cutAt = threshold + nlIdx
		}

		// Fallback: split at last space.
		if cutAt < 0 {
			spIdx := strings.LastIndex(chunk, " ")
			if spIdx > 0 {
				cutAt = spIdx
			}
		}

		// Hard cut if no good break point.
		if cutAt < 0 {
			cutAt = c.MaxLength
		}

		trimmed := strings.TrimSpace(remaining[:cutAt])
		if trimmed != "" {
			chunks = append(chunks, trimmed)
		}
		remaining = strings.TrimSpace(remaining[cutAt:])
	}

	return chunks
}

// SendFunc is a callback that sends a single text message.
type SendFunc func(text string) error

// MessageSender handles chunking, rate limiting, and sending.
type MessageSender struct {
	chunker      *MessageChunker
	rateLimiter  *RateLimiter
	chunkDelayMs int
	logger       *slog.Logger
}

// NewMessageSender creates a sender with the given options.
func NewMessageSender(chunker *MessageChunker, rateLimiter *RateLimiter, chunkDelayMs int, logger *slog.Logger) *MessageSender {
	return &MessageSender{
		chunker:      chunker,
		rateLimiter:  rateLimiter,
		chunkDelayMs: chunkDelayMs,
		logger:       logger,
	}
}

// Send chunks the text, applies rate limiting, and sends each chunk.
func (s *MessageSender) Send(userID, text string, sendFn SendFunc) error {
	s.rateLimiter.Throttle(userID)

	chunks := s.chunker.Split(text)
	for i, chunk := range chunks {
		if err := sendFn(chunk); err != nil {
			return err
		}
		// Delay between chunks, but not after the last one.
		if i < len(chunks)-1 && s.chunkDelayMs > 0 {
			time.Sleep(time.Duration(s.chunkDelayMs) * time.Millisecond)
		}
	}
	return nil
}

// RateLimiter enforces per-user minimum send intervals.
type RateLimiter struct {
	minInterval time.Duration
	mu          sync.Mutex
	lastSend    map[string]time.Time
}

// NewRateLimiter creates a rate limiter with the given minimum interval.
func NewRateLimiter(minIntervalMs int) *RateLimiter {
	return &RateLimiter{
		minInterval: time.Duration(minIntervalMs) * time.Millisecond,
		lastSend:    make(map[string]time.Time),
	}
}

// Throttle blocks until the minimum interval has passed since the last send for this user.
func (r *RateLimiter) Throttle(userID string) {
	r.mu.Lock()
	now := time.Now()
	var sleepDur time.Duration

	if last, ok := r.lastSend[userID]; ok {
		nextAllowed := last.Add(r.minInterval)
		if now.Before(nextAllowed) {
			sleepDur = nextAllowed.Sub(now)
			r.lastSend[userID] = nextAllowed
		} else {
			r.lastSend[userID] = now
		}
	} else {
		r.lastSend[userID] = now
	}
	r.mu.Unlock()

	if sleepDur > 0 {
		time.Sleep(sleepDur)
	}
}
