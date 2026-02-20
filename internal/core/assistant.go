package core

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// Compile-time check that AssistantCore satisfies HeartbeatRunner.
var _ HeartbeatRunner = (*AssistantCore)(nil)

// Provider is the interface that the AI layer must implement.  It abstracts
// the underlying model provider (e.g. Fantasy / OpenCode) from core logic.
type Provider interface {
	// SendMessage sends userText to the given session and returns the model
	// reply.  system overrides the system prompt when non-empty.
	SendMessage(ctx context.Context, sessionID string, system string, userText string) (string, error)

	// SendMessageNoReply sends userText to the given session without waiting
	// for or returning a model reply (fire-and-forget injection).
	SendMessageNoReply(ctx context.Context, sessionID string, userText string) error

	// CreateSession asks the provider to create a new named session and returns
	// its ID.
	CreateSession(ctx context.Context, title string) (string, error)

	// RecentContext returns a condensed string of the last few messages in the
	// given session, suitable for heartbeat context injection.  Returns "" on
	// error or if the session has no history.
	RecentContext(ctx context.Context, sessionID string) (string, error)
}

// AssistantInput carries a single user message received from a channel.
type AssistantInput struct {
	Channel string
	UserID  string
	Text    string
}

// AssistantCore orchestrates ask / heartbeat flows by delegating to injected
// dependencies.
type AssistantCore struct {
	provider      Provider
	sessionMgr    *SessionManager
	promptBuilder *PromptBuilder
	memoryRepo    MemoryRepository
	channelRepo   ChannelRepository
	heartbeatRepo HeartbeatRepository
	logger        *slog.Logger
}

// AssistantCoreOptions groups all required dependencies for AssistantCore.
type AssistantCoreOptions struct {
	Provider      Provider
	SessionMgr    *SessionManager
	PromptBuilder *PromptBuilder
	MemoryRepo    MemoryRepository
	ChannelRepo   ChannelRepository
	HeartbeatRepo HeartbeatRepository
	Logger        *slog.Logger
}

// NewAssistantCore creates an AssistantCore with the supplied options.
func NewAssistantCore(opts AssistantCoreOptions) *AssistantCore {
	return &AssistantCore{
		provider:      opts.Provider,
		sessionMgr:    opts.SessionMgr,
		promptBuilder: opts.PromptBuilder,
		memoryRepo:    opts.MemoryRepo,
		channelRepo:   opts.ChannelRepo,
		heartbeatRepo: opts.HeartbeatRepo,
		logger:        opts.Logger,
	}
}

// Ask processes a user message and returns the model reply.
func (a *AssistantCore) Ask(ctx context.Context, input AssistantInput) (string, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
	}

	opID := operationID("ask")

	sessionID, err := a.sessionMgr.GetOrCreateMainSession(ctx, a.provider.CreateSession)
	if err != nil {
		return "", fmt.Errorf("ask: get main session: %w", err)
	}

	if err := a.channelRepo.SaveLastChannel(input.Channel, input.UserID); err != nil {
		a.logger.WarnContext(ctx, "failed to save last channel", "error", err)
	}

	memoryEntries, err := a.memoryRepo.GetAll()
	if err != nil {
		a.logger.WarnContext(ctx, "failed to load memory", "error", err)
		memoryEntries = nil
	}

	systemPrompt := a.promptBuilder.BuildAgentSystemPrompt(memoryEntries)

	a.logger.InfoContext(ctx, "assistant request started",
		"operationID", opID,
		"channel", input.Channel,
		"userID", input.UserID,
		"sessionID", sessionID,
		"textLength", len(input.Text),
		"textPreview", truncateForLog(input.Text, 100),
		"memoryContextLength", len(memoryEntries),
	)

	reply, err := a.provider.SendMessage(ctx, sessionID, systemPrompt, input.Text)
	if err != nil {
		a.logger.ErrorContext(ctx, "provider SendMessage failed", "operationID", opID, "sessionID", sessionID, "error", err)
		return "I did not receive a valid model reply. Please check provider auth/model setup.", nil //nolint:nilerr
	}

	reply = strings.TrimSpace(reply)
	if reply == "" {
		a.logger.ErrorContext(ctx, "provider returned empty reply", "operationID", opID, "sessionID", sessionID)
		return "I did not receive a model reply. Please check provider auth/model setup.", nil
	}

	a.logger.InfoContext(ctx, "assistant request completed",
		"operationID", opID,
		"channel", input.Channel,
		"userID", input.UserID,
		"sessionID", sessionID,
		"answerLength", len(reply),
		"answerPreview", truncateForLog(reply, 100),
	)

	return reply, nil
}

// RunHeartbeatTasks executes enabled heartbeat tasks in the heartbeat session,
// then injects the summary into the main session and triggers a proactive
// notification decision.
func (a *AssistantCore) RunHeartbeatTasks(ctx context.Context) (string, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
	}

	opID := operationID("heartbeat")

	tasks, err := a.heartbeatRepo.GetTasks()
	if err != nil {
		return "", fmt.Errorf("heartbeat: get tasks: %w", err)
	}
	if len(tasks) == 0 {
		return "Heartbeat skipped: no tasks found in heartbeat_tasks table.", nil
	}

	heartbeatSessionID, err := a.sessionMgr.GetOrCreateHeartbeatSession(ctx, a.provider.CreateSession)
	if err != nil {
		return "", fmt.Errorf("heartbeat: get heartbeat session: %w", err)
	}

	mainSessionID, err := a.sessionMgr.GetOrCreateMainSession(ctx, a.provider.CreateSession)
	if err != nil {
		return "", fmt.Errorf("heartbeat: get main session: %w", err)
	}

	a.logger.InfoContext(ctx, "heartbeat sessions ready",
		"operationID", opID,
		"heartbeatSessionID", heartbeatSessionID,
		"mainSessionID", mainSessionID,
		"taskCount", len(tasks),
	)

	memoryEntries, err := a.memoryRepo.GetAll()
	if err != nil {
		a.logger.WarnContext(ctx, "failed to load memory for heartbeat", "error", err)
		memoryEntries = nil
	}

	systemPrompt := a.promptBuilder.BuildAgentSystemPrompt(memoryEntries)

	recentContext, err := a.provider.RecentContext(ctx, mainSessionID)
	if err != nil {
		a.logger.WarnContext(ctx, "failed to load recent context", "error", err, "sessionID", mainSessionID)
		recentContext = ""
	}

	heartbeatPrompt := a.promptBuilder.BuildHeartbeatPrompt(tasks, recentContext)

	summary, err := a.provider.SendMessage(ctx, heartbeatSessionID, systemPrompt, heartbeatPrompt)
	if err != nil {
		a.logger.ErrorContext(ctx, "heartbeat provider call failed", "operationID", opID, "heartbeatSessionID", heartbeatSessionID, "error", err)
		return "Heartbeat failed: invalid response format from model.", nil //nolint:nilerr
	}

	summary = strings.TrimSpace(summary)
	if summary == "" {
		a.logger.ErrorContext(ctx, "heartbeat empty summary", "operationID", opID, "heartbeatSessionID", heartbeatSessionID)
		return "Heartbeat failed: no summary reply from model.", nil
	}

	// Inject summary into main session (no reply needed).
	if err := a.provider.SendMessageNoReply(ctx, mainSessionID, "[Heartbeat summary]\n"+summary); err != nil {
		a.logger.WarnContext(ctx, "failed to inject heartbeat summary into main session", "error", err, "mainSessionID", mainSessionID)
	}

	// Trigger proactive notification decision.
	notifyPrompt := a.promptBuilder.BuildProactiveNotificationPrompt()
	if _, err := a.provider.SendMessage(ctx, mainSessionID, "", notifyPrompt); err != nil {
		a.logger.WarnContext(ctx, "proactive notification decision failed", "error", err, "mainSessionID", mainSessionID)
	}

	a.logger.InfoContext(ctx, "heartbeat task run complete",
		"operationID", opID,
		"heartbeatSessionID", heartbeatSessionID,
		"mainSessionID", mainSessionID,
		"taskCount", len(tasks),
	)

	return fmt.Sprintf("Heartbeat completed with %d tasks.", len(tasks)), nil
}

// StartNewMainSession replaces the current main session with a fresh one.
func (a *AssistantCore) StartNewMainSession(ctx context.Context, reason string) (string, error) {
	return a.sessionMgr.StartNewMainSession(ctx, reason, a.provider.CreateSession)
}

// MainSessionVersion returns the persisted version counter for the active main session.
func (a *AssistantCore) MainSessionVersion() (int64, error) {
	return a.sessionMgr.GetMainSessionVersion()
}

// operationID generates a short unique identifier for log correlation.
func operationID(prefix string) string {
	return prefix + "-" + newUUID()
}

// truncateForLog returns the first max runes of s, appending "..." if truncated.
func truncateForLog(s string, max int) string {
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "..."
	}
	return s
}
