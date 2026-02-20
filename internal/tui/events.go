package tui

import "time"

// Event types for the service event bus.
// Backend goroutines publish events via channels; the TUI subscribes.

// EventType classifies service events.
type EventType int

const (
	EventLog EventType = iota
	EventMessageIn
	EventMessageOut
	EventWhatsAppStatus
	EventTailscaleStatus
	EventHeartbeatStatus
	EventVaultStats
	EventMemoryStats
	EventOutboxStats
)

// Event carries data from backend services to the TUI.
type Event struct {
	Type      EventType
	Timestamp time.Time
	Data      any
}

// LogEvent carries a log entry.
type LogEvent struct {
	Level   string
	Message string
	Fields  map[string]any
}

// MessageEvent carries an incoming or outgoing chat message.
type MessageEvent struct {
	UserID    string
	Text      string
	Outgoing  bool
	Timestamp time.Time
}

// StatusEvent carries connection status.
type StatusEvent struct {
	Connected bool
	Detail    string
}

// StatsEvent carries numeric stats for a subsystem.
type StatsEvent struct {
	Label string
	Count int
}

// EventBus fans out events to subscribers.
type EventBus struct {
	ch chan Event
}

// NewEventBus creates a buffered event bus.
func NewEventBus(bufSize int) *EventBus {
	if bufSize <= 0 {
		bufSize = 256
	}
	return &EventBus{ch: make(chan Event, bufSize)}
}

// Publish sends an event non-blocking (drops if buffer full).
func (b *EventBus) Publish(e Event) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	select {
	case b.ch <- e:
	default:
	}
}

// Subscribe returns the read channel for consuming events.
func (b *EventBus) Subscribe() <-chan Event {
	return b.ch
}

// PublishLog is a convenience for logging events.
func (b *EventBus) PublishLog(level, msg string, fields map[string]any) {
	b.Publish(Event{
		Type: EventLog,
		Data: LogEvent{Level: level, Message: msg, Fields: fields},
	})
}
