package core

import "context"

// MemoryEntry represents a single stored fact.
type MemoryEntry struct {
	ID     int64
	Fact   string
	Source *string
}

// MemoryRepository defines the interface for memory persistence.
type MemoryRepository interface {
	Append(fact string, source *string) (bool, error)
	Delete(id int64) (bool, error)
	Update(id int64, fact string) (bool, error)
	GetAll() ([]MemoryEntry, error)
}

// LastChannel records the most recently active channel and user.
type LastChannel struct {
	Channel string
	UserID  string
}

// ChannelRepository persists the last-used channel info.
type ChannelRepository interface {
	SaveLastChannel(channel, userID string) error
	GetLastChannel() (*LastChannel, error)
}

// SessionRepository stores opaque session IDs in a key-value table.
type SessionRepository interface {
	GetSessionID(key string) (string, bool, error)
	SaveSessionID(key, sessionID string) error
	DeleteSession(key string) error
}

// WhitelistRepository gates per-channel user access.
type WhitelistRepository interface {
	IsWhitelisted(channel, userID string) (bool, error)
	AddToWhitelist(channel, userID string) (bool, error)
	RemoveFromWhitelist(channel, userID string) (bool, error)
}

// OutboxMessage is a queued proactive message with retry metadata.
type OutboxMessage struct {
	ID          int64
	Channel     string
	UserID      string
	Text        string
	RetryCount  int
	MaxRetries  int
	NextRetryAt *string
}

// OutboxRepository manages the proactive message queue.
type OutboxRepository interface {
	Enqueue(channel, userID, text string, maxRetries int) error
	ListPending(channel string) ([]OutboxMessage, error)
	Acknowledge(id int64) error
	MarkRetry(id int64, retryCount int, nextRetryAt string) error
}

// HeartbeatRepository reads enabled heartbeat tasks.
type HeartbeatRepository interface {
	GetTasks() ([]string, error)
	GetTaskCount() (int, error)
}

// HeartbeatRunner executes heartbeat tasks.
type HeartbeatRunner interface {
	RunHeartbeatTasks(ctx context.Context) (string, error)
}

// MessageHandler processes an incoming user message and returns a reply.
type MessageHandler func(userID, text string) (string, error)

// ChannelAdapter is the interface for messaging channel implementations.
type ChannelAdapter interface {
	Name() string
	Start(handler MessageHandler) error
	Stop() error
	Send(userID, text string) error
}

// ThrottlePort controls per-user send rate limiting.
type ThrottlePort interface {
	Throttle(userID string) error
}
