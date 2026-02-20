package tui

import (
	"strings"
	"testing"
	"time"
)

func TestEventBus_BroadcastsToAllSubscribers(t *testing.T) {
	bus := NewEventBus(8)
	sub1 := bus.Subscribe()
	sub2 := bus.Subscribe()

	want := Event{Type: EventLog, Data: LogEvent{Level: "INFO", Message: "hello"}}
	bus.Publish(want)

	select {
	case got := <-sub1:
		if got.Type != want.Type {
			t.Fatalf("sub1 type = %v, want %v", got.Type, want.Type)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for sub1 event")
	}

	select {
	case got := <-sub2:
		if got.Type != want.Type {
			t.Fatalf("sub2 type = %v, want %v", got.Type, want.Type)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for sub2 event")
	}
}

func TestModel_HandleSessionChangedEvent_AddsMessage(t *testing.T) {
	m := New(NewEventBus(8))
	e := Event{
		Type:      EventSessionChanged,
		Timestamp: time.Now(),
		Data: SessionChangedEvent{
			Channel: "whatsapp",
			UserID:  "user@s.whatsapp.net",
			Reason:  "whatsapp /new command",
			Version: 2,
		},
	}

	m.handleEvent(e)

	if len(m.messages.messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(m.messages.messages))
	}
	got := m.messages.messages[0].Text
	if !strings.Contains(got, "Context changed") {
		t.Fatalf("message text = %q, expected context changed notice", got)
	}
	if !strings.Contains(got, "[v2]") {
		t.Fatalf("message text = %q, expected session version marker", got)
	}
}
