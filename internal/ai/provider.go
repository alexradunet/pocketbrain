package ai

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
	"charm.land/fantasy/providers/google"
	"charm.land/fantasy/providers/openai"
	"charm.land/fantasy/providers/openaicompat"

	"github.com/pocketbrain/pocketbrain/internal/core"
)

// Compile-time checks.
var (
	_ core.Provider = (*FantasyProvider)(nil)
	_ core.Provider = (*StubProvider)(nil)
)

const maxSessionHistory = 200

// FantasyProviderConfig holds settings for the unified Fantasy-based provider.
type FantasyProviderConfig struct {
	ProviderName string // "anthropic", "openai", "google", or any openai-compat name
	APIKey       string
	Model        string
	Tools        []fantasy.AgentTool
	Logger       *slog.Logger
}

// FantasyProvider is a unified AI provider that uses charm.land/fantasy to
// support Anthropic, OpenAI, Google, and any OpenAI-compatible endpoint.
// It stores conversation history in memory per session and delegates tool
// execution to fantasy.Agent.
type FantasyProvider struct {
	model  fantasy.LanguageModel
	tools  []fantasy.AgentTool
	logger *slog.Logger

	mu       sync.Mutex
	sessions map[string][]fantasy.Message
}

// NewFantasyProvider creates a unified provider backed by charm.land/fantasy.
func NewFantasyProvider(ctx context.Context, cfg FantasyProviderConfig) (*FantasyProvider, error) {
	var provider fantasy.Provider
	var err error

	switch cfg.ProviderName {
	case "anthropic":
		provider, err = anthropic.New(
			anthropic.WithAPIKey(cfg.APIKey),
		)
	case "google":
		provider, err = google.New(
			google.WithGeminiAPIKey(cfg.APIKey),
		)
	case "openai":
		provider, err = openai.New(
			openai.WithAPIKey(cfg.APIKey),
		)
	default:
		// Any other provider name: use openai-compatible endpoint.
		provider, err = openaicompat.New(
			openaicompat.WithAPIKey(cfg.APIKey),
			openaicompat.WithName(cfg.ProviderName),
		)
	}
	if err != nil {
		return nil, fmt.Errorf("fantasy: create provider %q: %w", cfg.ProviderName, err)
	}

	model, err := provider.LanguageModel(ctx, cfg.Model)
	if err != nil {
		return nil, fmt.Errorf("fantasy: create model %q: %w", cfg.Model, err)
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &FantasyProvider{
		model:    model,
		tools:    cfg.Tools,
		logger:   logger,
		sessions: make(map[string][]fantasy.Message),
	}, nil
}

// newFantasyProviderWithModel creates a FantasyProvider from an existing model.
// Used for testing with mock models.
func newFantasyProviderWithModel(model fantasy.LanguageModel, tools []fantasy.AgentTool) *FantasyProvider {
	return &FantasyProvider{
		model:    model,
		tools:    tools,
		logger:   slog.Default(),
		sessions: make(map[string][]fantasy.Message),
	}
}

// CreateSession generates a new unique session ID.
func (p *FantasyProvider) CreateSession(_ context.Context, title string) (string, error) {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	id := fmt.Sprintf("fantasy-%s-%s", sanitizeTitle(title), hex.EncodeToString(b))

	p.mu.Lock()
	p.sessions[id] = nil
	p.mu.Unlock()

	return id, nil
}

// SendMessage sends userText to the model within the given session and returns
// the assistant reply. The system prompt is provided per-call to support
// dynamic content (e.g. memory injection). Fantasy's Agent handles the tool
// execution loop internally.
func (p *FantasyProvider) SendMessage(ctx context.Context, sessionID, system, userText string) (string, error) {
	// Get current conversation history (before this message).
	history := p.getHistory(sessionID)

	// Create agent per-call with the dynamic system prompt.
	// Agent creation is cheap (just a struct allocation, no connections).
	opts := []fantasy.AgentOption{
		fantasy.WithMaxRetries(3),
		fantasy.WithStopConditions(fantasy.StepCountIs(10)),
	}
	if system != "" {
		opts = append(opts, fantasy.WithSystemPrompt(system))
	}
	if len(p.tools) > 0 {
		opts = append(opts, fantasy.WithTools(p.tools...))
	}
	agent := fantasy.NewAgent(p.model, opts...)

	result, err := agent.Generate(ctx, fantasy.AgentCall{
		Prompt:   userText,
		Messages: history,
	})
	if err != nil {
		return "", fmt.Errorf("fantasy: generate: %w", err)
	}

	text := result.Response.Content.Text()

	// Append user message and assistant reply to session history.
	p.appendMessage(sessionID, fantasy.NewUserMessage(userText))
	p.appendMessage(sessionID, fantasy.Message{
		Role: fantasy.MessageRoleAssistant,
		Content: []fantasy.MessagePart{
			fantasy.TextPart{Text: text},
		},
	})

	return text, nil
}

// SendMessageNoReply injects userText into the session history by sending it
// to the model. The reply is stored in history but not returned.
func (p *FantasyProvider) SendMessageNoReply(ctx context.Context, sessionID, userText string) error {
	_, err := p.SendMessage(ctx, sessionID, "", userText)
	return err
}

// RecentContext returns a condensed string of the last few messages in the
// session, suitable for context injection.
func (p *FantasyProvider) RecentContext(_ context.Context, sessionID string) (string, error) {
	const maxMessages = 10

	p.mu.Lock()
	hist, ok := p.sessions[sessionID]
	p.mu.Unlock()

	if !ok || len(hist) == 0 {
		return "", nil
	}

	start := 0
	if len(hist) > maxMessages {
		start = len(hist) - maxMessages
	}
	recent := hist[start:]

	var sb strings.Builder
	for _, m := range recent {
		// Skip tool-related messages for context summary.
		if m.Role == fantasy.MessageRoleTool {
			continue
		}
		sb.WriteString(string(m.Role))
		sb.WriteString(": ")
		for _, part := range m.Content {
			if tp, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok {
				sb.WriteString(tp.Text)
			}
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// Close is a no-op for the Fantasy provider.
func (p *FantasyProvider) Close() error {
	return nil
}

// ---------------------------------------------------------------------------
// Internal: session history helpers
// ---------------------------------------------------------------------------

func (p *FantasyProvider) appendMessage(sessionID string, msg fantasy.Message) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sessions[sessionID] = append(p.sessions[sessionID], msg)
	if len(p.sessions[sessionID]) > maxSessionHistory {
		p.sessions[sessionID] = p.sessions[sessionID][len(p.sessions[sessionID])-maxSessionHistory:]
	}
}

func (p *FantasyProvider) getHistory(sessionID string) []fantasy.Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	hist := p.sessions[sessionID]
	cp := make([]fantasy.Message, len(hist))
	copy(cp, hist)
	return cp
}

// ---------------------------------------------------------------------------
// StubProvider: placeholder when no API key is configured
// ---------------------------------------------------------------------------

// StubProvider is a placeholder that satisfies core.Provider.
type StubProvider struct {
	logger *slog.Logger
}

// NewStubProvider creates a stub provider for development/testing.
func NewStubProvider(logger *slog.Logger) *StubProvider {
	return &StubProvider{logger: logger}
}

func (p *StubProvider) SendMessage(_ context.Context, sessionID, _, userText string) (string, error) {
	p.logger.Info("stub provider called",
		"sessionID", sessionID,
		"userTextLen", len(userText),
	)
	return fmt.Sprintf("PocketBrain AI provider not yet configured. Received: %s", truncateForLog(userText, 100)), nil
}

func (p *StubProvider) SendMessageNoReply(_ context.Context, sessionID, userText string) error {
	p.logger.Debug("stub provider no-reply",
		"sessionID", sessionID,
		"userTextLen", len(userText),
	)
	return nil
}

func (p *StubProvider) CreateSession(_ context.Context, title string) (string, error) {
	p.logger.Info("stub provider create session", "title", title)
	return fmt.Sprintf("stub-session-%s", title), nil
}

func (p *StubProvider) RecentContext(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (p *StubProvider) Close() error {
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func sanitizeTitle(title string) string {
	r := strings.NewReplacer(" ", "-", "/", "-", "\\", "-")
	s := r.Replace(strings.ToLower(strings.TrimSpace(title)))
	if len(s) > 32 {
		s = s[:32]
	}
	return s
}

func truncateForLog(s string, max int) string {
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "..."
	}
	return s
}
