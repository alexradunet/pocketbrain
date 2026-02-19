package channel

import (
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// MessageChunker.Split
// ---------------------------------------------------------------------------

func TestSplit_EmptyText(t *testing.T) {
	c := NewMessageChunker(100)

	for _, input := range []string{"", "   ", "\n\n"} {
		got := c.Split(input)
		if got != nil {
			t.Errorf("Split(%q) = %v; want nil", input, got)
		}
	}
}

func TestSplit_ShortText_NoSplit(t *testing.T) {
	c := NewMessageChunker(100)
	text := "Hello, world!"

	got := c.Split(text)
	if len(got) != 1 || got[0] != text {
		t.Fatalf("Split(%q) = %v; want [%q]", text, got, text)
	}
}

func TestSplit_ExactMaxLength(t *testing.T) {
	c := NewMessageChunker(10)
	text := "0123456789" // exactly 10 chars

	got := c.Split(text)
	if len(got) != 1 || got[0] != text {
		t.Fatalf("Split(%q) = %v; want [%q]", text, got, text)
	}
}

func TestSplit_NewlineBreak(t *testing.T) {
	c := NewMessageChunker(20)
	// The newline threshold is 0.5, so the chunker looks for newlines in
	// chunk[10:] (the upper half of a 20-char window). Place the newline at
	// index 14 so it falls inside that search zone.
	// "aaaaaaaaaaaaaa\nbbbbbbbbbbbbbb" = 14 + 1 + 14 = 29 chars
	text := "aaaaaaaaaaaaaa\nbbbbbbbbbbbbbb"

	got := c.Split(text)
	if len(got) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d: %v", len(got), got)
	}
	// The first chunk should break at the newline (index 14).
	if got[0] != "aaaaaaaaaaaaaa" {
		t.Errorf("first chunk = %q; want %q", got[0], "aaaaaaaaaaaaaa")
	}
	// Verify second chunk contains the remainder.
	if got[1] != "bbbbbbbbbbbbbb" {
		t.Errorf("second chunk = %q; want %q", got[1], "bbbbbbbbbbbbbb")
	}
}

func TestSplit_SpaceBreak(t *testing.T) {
	c := NewMessageChunker(20)
	c.NewlineThreshold = 0.99 // push newline threshold very high so spaces win

	// "hello world this is a test" — no newlines, spaces exist
	text := "hello world this is a test message here"

	got := c.Split(text)
	if len(got) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d: %v", len(got), got)
	}
	for i, chunk := range got {
		if len(chunk) > c.MaxLength {
			t.Errorf("chunk[%d] len=%d exceeds MaxLength=%d: %q", i, len(chunk), c.MaxLength, chunk)
		}
	}
}

func TestSplit_HardBreak_NoSpacesOrNewlines(t *testing.T) {
	c := NewMessageChunker(10)
	text := "abcdefghijklmnopqrstuvwxyz" // 26 chars, no spaces/newlines

	got := c.Split(text)
	if len(got) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d: %v", len(got), got)
	}
	for i, chunk := range got {
		if len(chunk) > c.MaxLength {
			t.Errorf("chunk[%d] len=%d exceeds MaxLength=%d: %q", i, len(chunk), c.MaxLength, chunk)
		}
	}
	// Reassemble.
	reassembled := strings.Join(got, "")
	if reassembled != text {
		t.Errorf("reassembled = %q; want %q", reassembled, text)
	}
}

func TestSplit_DefaultMaxLength(t *testing.T) {
	c := NewMessageChunker(0) // should default to 3500
	if c.MaxLength != 3500 {
		t.Fatalf("MaxLength = %d; want 3500", c.MaxLength)
	}
}

// ---------------------------------------------------------------------------
// RateLimiter.Throttle
// ---------------------------------------------------------------------------

func TestThrottle_FirstCallDoesNotBlock(t *testing.T) {
	rl := NewRateLimiter(500) // 500ms interval

	start := time.Now()
	rl.Throttle("user1")
	elapsed := time.Since(start)

	if elapsed > 50*time.Millisecond {
		t.Errorf("first call blocked for %v; expected near-zero", elapsed)
	}
}

func TestThrottle_SecondCallBlocks(t *testing.T) {
	rl := NewRateLimiter(200) // 200ms interval

	rl.Throttle("user1")

	start := time.Now()
	rl.Throttle("user1")
	elapsed := time.Since(start)

	// Should have blocked for at least ~150ms (allowing some tolerance).
	if elapsed < 100*time.Millisecond {
		t.Errorf("second call elapsed %v; expected at least ~150ms", elapsed)
	}
}

func TestThrottle_DifferentUsersIndependent(t *testing.T) {
	rl := NewRateLimiter(500)

	rl.Throttle("user1")

	start := time.Now()
	rl.Throttle("user2") // different user — should not block
	elapsed := time.Since(start)

	if elapsed > 50*time.Millisecond {
		t.Errorf("different user blocked for %v; expected near-zero", elapsed)
	}
}

// ---------------------------------------------------------------------------
// MessageSender.Send
// ---------------------------------------------------------------------------

func TestSend_ChunksAndRateLimits(t *testing.T) {
	chunker := NewMessageChunker(20)
	rl := NewRateLimiter(0) // no rate limit delay for speed
	logger := slog.Default()
	sender := NewMessageSender(chunker, rl, 0, logger)

	text := "hello world this is a longer message"
	var mu sync.Mutex
	var sent []string

	err := sender.Send("user1", text, func(chunk string) error {
		mu.Lock()
		defer mu.Unlock()
		sent = append(sent, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}

	if len(sent) < 2 {
		t.Fatalf("expected at least 2 chunks sent, got %d: %v", len(sent), sent)
	}

	for i, chunk := range sent {
		if len(chunk) > chunker.MaxLength {
			t.Errorf("sent[%d] len=%d exceeds MaxLength=%d", i, len(chunk), chunker.MaxLength)
		}
	}
}

func TestSend_EmptyText(t *testing.T) {
	chunker := NewMessageChunker(100)
	rl := NewRateLimiter(0)
	logger := slog.Default()
	sender := NewMessageSender(chunker, rl, 0, logger)

	called := false
	err := sender.Send("user1", "", func(chunk string) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	if called {
		t.Error("sendFn should not have been called for empty text")
	}
}

func TestSend_SendFuncError(t *testing.T) {
	chunker := NewMessageChunker(10)
	rl := NewRateLimiter(0)
	logger := slog.Default()
	sender := NewMessageSender(chunker, rl, 0, logger)

	text := "abcdefghijklmnopqrstuvwxyz" // will chunk

	sendErr := strings.NewReader("") // just need a non-nil error
	_ = sendErr
	err := sender.Send("user1", text, func(chunk string) error {
		return &testError{"send failed"}
	})

	if err == nil {
		t.Fatal("expected error from Send, got nil")
	}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
