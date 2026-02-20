package tui

import (
	"sync"
	"time"
)

// Event types for the service event bus.
// Backend goroutines publish events via channels; the TUI subscribes.

// EventType classifies service events.
type EventType int

const (
	EventLog EventType = iota
	EventMessageIn
	EventMessageOut
	EventSessionChanged
	EventWhatsAppStatus
	EventWebDAVStatus
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

// SessionChangedEvent signals that the active conversation context/session changed.
type SessionChangedEvent struct {
	Channel string
	UserID  string
	Reason  string
	Version int64
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
	mu          sync.RWMutex
	nextID      int
	bufSize     int
	subscribers map[int]chan Event
}

// NewEventBus creates a buffered event bus.
func NewEventBus(bufSize int) *EventBus {
	if bufSize <= 0 {
		bufSize = 256
	}
	return &EventBus{
		bufSize:     bufSize,
		subscribers: make(map[int]chan Event),
	}
}

// Publish broadcasts an event to all subscribers, non-blocking per subscriber.
// Slow subscribers may drop events.
func (b *EventBus) Publish(e Event) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.subscribers {
		select {
		case ch <- e:
		default:
		}
	}
}

// Subscribe returns a dedicated read channel for consuming events.
func (b *EventBus) Subscribe() <-chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++
	ch := make(chan Event, b.bufSize)
	b.subscribers[id] = ch
	return ch
}

// PublishLog is a convenience for logging events.
func (b *EventBus) PublishLog(level, msg string, fields map[string]any) {
	b.Publish(Event{
		Type: EventLog,
		Data: LogEvent{Level: level, Message: msg, Fields: fields},
	})
}
