// Package whatsapp implements a WhatsApp channel adapter for PocketBrain.
//
// The adapter uses a WAClient interface to abstract the actual WhatsApp
// connection (e.g. whatsmeow), allowing the business logic to be tested
// without external dependencies.
package whatsapp

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/pocketbrain/pocketbrain/internal/core"
)

// WAClient abstracts the WhatsApp connection (whatsmeow or mock).
type WAClient interface {
	Connect() error
	Disconnect()
	SendText(jid string, text string) error
	IsConnected() bool
}

// SessionStarter abstracts starting a new conversation session.
type SessionStarter interface {
	StartNewSession(userID, reason string) error
}

// Adapter implements core.ChannelAdapter for WhatsApp.
type Adapter struct {
	client  WAClient
	handler core.MessageHandler
	logger  *slog.Logger

	mu      sync.Mutex
	stopped bool
}

// NewAdapter creates a new WhatsApp adapter.
func NewAdapter(client WAClient, logger *slog.Logger) *Adapter {
	return &Adapter{
		client: client,
		logger: logger,
	}
}

// Name returns the channel name for this adapter.
func (a *Adapter) Name() string {
	return "whatsapp"
}

// Start connects the WAClient and stores the message handler for incoming
// messages. The actual message receive loop is provided by the WAClient
// implementation (e.g. whatsmeow event handler).
func (a *Adapter) Start(handler core.MessageHandler) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.stopped {
		return fmt.Errorf("whatsapp adapter already stopped")
	}

	a.handler = handler

	if err := a.client.Connect(); err != nil {
		return fmt.Errorf("whatsapp connect: %w", err)
	}

	a.logger.Info("whatsapp adapter started")
	return nil
}

// Stop disconnects the WAClient. It is safe to call multiple times.
func (a *Adapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.stopped {
		return nil
	}
	a.stopped = true

	a.client.Disconnect()
	a.logger.Info("whatsapp adapter stopped")
	return nil
}

// Send delivers a text message to the given WhatsApp JID.
func (a *Adapter) Send(userID, text string) error {
	if !a.client.IsConnected() {
		return fmt.Errorf("whatsapp client not connected")
	}
	return a.client.SendText(userID, text)
}

// HandleIncoming is called by the WAClient event handler when a message
// arrives. It delegates to the MessageProcessor.
func (a *Adapter) HandleIncoming(userID, text string) (string, error) {
	a.mu.Lock()
	handler := a.handler
	a.mu.Unlock()

	if handler == nil {
		return "", fmt.Errorf("no message handler registered")
	}

	return handler(userID, text)
}

// MessageProcessor routes incoming messages through commands or the
// main handler, enforcing whitelist access control.
type MessageProcessor struct {
	whitelist core.WhitelistRepository
	router    *CommandRouter
	handler   core.MessageHandler
	logger    *slog.Logger
}

// NewMessageProcessor creates a MessageProcessor.
func NewMessageProcessor(
	whitelist core.WhitelistRepository,
	router *CommandRouter,
	handler core.MessageHandler,
	logger *slog.Logger,
) *MessageProcessor {
	return &MessageProcessor{
		whitelist: whitelist,
		router:    router,
		handler:   handler,
		logger:    logger,
	}
}

// Process handles an incoming message. Commands (starting with /) are
// routed first. Non-whitelisted users are rejected. Empty messages are
// silently ignored.
func (p *MessageProcessor) Process(userID, text string) (string, error) {
	text = strings.TrimSpace(text)

	// Ignore empty messages.
	if text == "" {
		return "", nil
	}

	// Route commands first (commands like /pair work even for non-whitelisted users).
	if strings.HasPrefix(text, "/") {
		if resp, handled := p.router.Route(userID, text); handled {
			return resp, nil
		}
	}

	// Check whitelist for regular messages.
	allowed, err := p.whitelist.IsWhitelisted("whatsapp", userID)
	if err != nil {
		p.logger.Error("whitelist check failed", "userID", userID, "error", err)
		return "", fmt.Errorf("whitelist check: %w", err)
	}
	if !allowed {
		p.logger.Warn("non-whitelisted user rejected", "userID", userID)
		return "You are not authorized. Ask the operator to whitelist your number.", nil
	}

	// Delegate to the main handler.
	return p.handler(userID, text)
}
