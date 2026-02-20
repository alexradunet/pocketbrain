package whatsapp

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/pocketbrain/pocketbrain/internal/core"
)

// OutboxProcessor delivers pending outbox messages via the WAClient.
type OutboxProcessor struct {
	outboxRepo core.OutboxRepository
	client     WAClient
	logger     *slog.Logger
}

// NewOutboxProcessor creates an OutboxProcessor.
func NewOutboxProcessor(repo core.OutboxRepository, client WAClient, logger *slog.Logger) *OutboxProcessor {
	return &OutboxProcessor{
		outboxRepo: repo,
		client:     client,
		logger:     logger,
	}
}

// ProcessPending fetches all pending WhatsApp outbox messages and attempts
// to deliver them. Successfully delivered messages are acknowledged.
// Failed messages are marked for retry with exponential backoff.
func (p *OutboxProcessor) ProcessPending() error {
	messages, err := p.outboxRepo.ListPending("whatsapp")
	if err != nil {
		return fmt.Errorf("list pending outbox: %w", err)
	}

	if len(messages) == 0 {
		return nil
	}

	if !p.client.IsConnected() {
		p.logger.Warn("skipping outbox processing: client not connected",
			"pendingCount", len(messages))
		return nil
	}

	for _, msg := range messages {
		if err := p.deliver(msg); err != nil {
			p.logger.Error("outbox delivery failed",
				"messageID", msg.ID,
				"userID", msg.UserID,
				"retryCount", msg.RetryCount,
				"error", err,
			)
			p.scheduleRetry(msg)
			continue
		}

		if err := p.outboxRepo.Acknowledge(msg.ID); err != nil {
			p.logger.Error("outbox acknowledge failed",
				"messageID", msg.ID,
				"error", err,
			)
		} else {
			p.logger.Info("outbox message delivered",
				"messageID", msg.ID,
				"userID", msg.UserID,
			)
		}
	}

	return nil
}

// deliver sends a single outbox message via the WAClient.
func (p *OutboxProcessor) deliver(msg core.OutboxMessage) error {
	return p.client.SendText(msg.UserID, msg.Text)
}

// scheduleRetry marks a failed message for retry with exponential backoff.
func (p *OutboxProcessor) scheduleRetry(msg core.OutboxMessage) {
	nextRetry := msg.RetryCount + 1
	if nextRetry > msg.MaxRetries {
		p.logger.Warn("outbox message exceeded max retries, giving up",
			"messageID", msg.ID,
			"userID", msg.UserID,
			"retryCount", msg.RetryCount,
		)
		// Acknowledge to remove from queue (dead-letter).
		_ = p.outboxRepo.Acknowledge(msg.ID)
		return
	}

	// Exponential backoff: 60s, 120s, 240s, ...
	backoffMs := 60_000 * (1 << (nextRetry - 1))
	nextRetryAt := time.Now().Add(time.Duration(backoffMs) * time.Millisecond).
		Format(time.RFC3339)

	if err := p.outboxRepo.MarkRetry(msg.ID, nextRetry, nextRetryAt); err != nil {
		p.logger.Error("failed to mark retry",
			"messageID", msg.ID,
			"error", err,
		)
	}
}
