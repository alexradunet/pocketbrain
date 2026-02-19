package ai

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pocketbrain/pocketbrain/internal/core"
)

// Compile-time check that StubProvider satisfies core.Provider.
var _ core.Provider = (*StubProvider)(nil)

// ProviderConfig holds the configuration for AI provider setup.
type ProviderConfig struct {
	ProviderName string // e.g., "anthropic", "openai", "google"
	ModelName    string // e.g., "claude-sonnet-4-20250514"
	APIKey       string // from environment
	Logger       *slog.Logger
}

// StubProvider is a placeholder that satisfies core.Provider.
// Replace with Fantasy implementation in Phase 2+.
type StubProvider struct {
	logger *slog.Logger
}

func NewStubProvider(logger *slog.Logger) *StubProvider {
	return &StubProvider{logger: logger}
}

func (p *StubProvider) SendMessage(ctx context.Context, sessionID, system, userText string) (string, error) {
	p.logger.Info("stub provider called",
		"sessionID", sessionID,
		"userTextLen", len(userText),
	)
	return fmt.Sprintf("PocketBrain AI provider not yet configured. Received: %s", truncateForLog(userText, 100)), nil
}

func (p *StubProvider) SendMessageNoReply(ctx context.Context, sessionID, userText string) error {
	p.logger.Debug("stub provider no-reply",
		"sessionID", sessionID,
		"userTextLen", len(userText),
	)
	return nil
}

func (p *StubProvider) CreateSession(ctx context.Context, title string) (string, error) {
	p.logger.Info("stub provider create session", "title", title)
	return fmt.Sprintf("stub-session-%s", title), nil
}

func (p *StubProvider) RecentContext(ctx context.Context, sessionID string) (string, error) {
	return "", nil
}

func (p *StubProvider) Close() error {
	return nil
}

func truncateForLog(s string, max int) string {
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "..."
	}
	return s
}
