package ai

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"charm.land/fantasy"
)

// ---------------------------------------------------------------------------
// StubProvider
// ---------------------------------------------------------------------------

func TestStubProvider_SendMessage(t *testing.T) {
	p := NewStubProvider(slog.Default())
	reply, err := p.SendMessage(context.Background(), "s1", "", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reply, "not yet configured") {
		t.Fatalf("unexpected reply: %q", reply)
	}
	if !strings.Contains(reply, "hello") {
		t.Fatalf("expected user text echoed, got: %q", reply)
	}
}

func TestStubProvider_CreateSession(t *testing.T) {
	p := NewStubProvider(slog.Default())
	id, err := p.CreateSession(context.Background(), "test session")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(id, "stub-session") {
		t.Fatalf("unexpected session ID: %q", id)
	}
}

func TestStubProvider_RecentContext(t *testing.T) {
	p := NewStubProvider(slog.Default())
	ctx, err := p.RecentContext(context.Background(), "s1")
	if err != nil {
		t.Fatal(err)
	}
	if ctx != "" {
		t.Fatalf("expected empty context, got: %q", ctx)
	}
}

func TestStubProvider_Close(t *testing.T) {
	p := NewStubProvider(slog.Default())
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// FantasyProvider: session management (no real model needed)
// ---------------------------------------------------------------------------

func TestFantasyProvider_CreateSession(t *testing.T) {
	p := newFantasyProviderWithModel(nil, nil)

	id, err := p.CreateSession(context.Background(), "My Test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(id, "fantasy-") {
		t.Fatalf("expected fantasy- prefix, got: %q", id)
	}
	if !strings.Contains(id, "my-test") {
		t.Fatalf("expected sanitized title in ID, got: %q", id)
	}
}

func TestFantasyProvider_RecentContext_Empty(t *testing.T) {
	p := newFantasyProviderWithModel(nil, nil)
	ctx, err := p.RecentContext(context.Background(), "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if ctx != "" {
		t.Fatalf("expected empty context, got: %q", ctx)
	}
}

func TestFantasyProvider_SessionHistoryTrimming(t *testing.T) {
	p := newFantasyProviderWithModel(nil, nil)
	sid := "trim-test"

	p.mu.Lock()
	p.sessions[sid] = nil
	p.mu.Unlock()

	// Append more than maxSessionHistory messages.
	for i := 0; i < maxSessionHistory+50; i++ {
		p.appendMessage(sid, fantasy.NewUserMessage("msg"))
	}

	hist := p.getHistory(sid)
	if len(hist) != maxSessionHistory {
		t.Fatalf("expected %d messages after trimming, got %d", maxSessionHistory, len(hist))
	}
}

func TestFantasyProvider_Close(t *testing.T) {
	p := newFantasyProviderWithModel(nil, nil)
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func TestSanitizeTitle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"My/Session\\Name", "my-session-name"},
		{"  spaces  ", "spaces"},
		{"", ""},
	}
	for _, tt := range tests {
		got := sanitizeTitle(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeTitle(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeTitle_LongInput(t *testing.T) {
	long := strings.Repeat("a", 100)
	got := sanitizeTitle(long)
	if len(got) > 32 {
		t.Fatalf("expected max 32 chars, got %d", len(got))
	}
}

func TestTruncateForLog(t *testing.T) {
	if got := truncateForLog("short", 100); got != "short" {
		t.Fatalf("unexpected: %q", got)
	}
	if got := truncateForLog("hello world", 5); got != "hello..." {
		t.Fatalf("unexpected: %q", got)
	}
}
