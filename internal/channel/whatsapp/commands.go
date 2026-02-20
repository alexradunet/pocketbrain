package whatsapp

import (
	"crypto/subtle"
	"fmt"
	"log/slog"
	"strings"

	"github.com/pocketbrain/pocketbrain/internal/core"
)

// constantTimeTokenCompare compares two tokens in constant time to prevent
// timing side-channel attacks.
func constantTimeTokenCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// CommandRouter handles slash-commands from WhatsApp users.
type CommandRouter struct {
	pairToken  string
	guard      *BruteForceGuard
	whitelist  core.WhitelistRepository
	memoryRepo core.MemoryRepository
	sessionMgr SessionStarter
	logger     *slog.Logger
}

// NewCommandRouter creates a CommandRouter with the given dependencies.
func NewCommandRouter(
	pairToken string,
	guard *BruteForceGuard,
	whitelist core.WhitelistRepository,
	memoryRepo core.MemoryRepository,
	sessionMgr SessionStarter,
	logger *slog.Logger,
) *CommandRouter {
	return &CommandRouter{
		pairToken:  pairToken,
		guard:      guard,
		whitelist:  whitelist,
		memoryRepo: memoryRepo,
		sessionMgr: sessionMgr,
		logger:     logger,
	}
}

// Route checks if text is a /command and handles it.
// Returns (response, handled). If handled is false, the message should
// be processed as a regular message.
func (r *CommandRouter) Route(userID, text string) (string, bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return "", false
	}

	parts := strings.SplitN(text, " ", 2)
	cmd := strings.ToLower(parts[0])
	var arg string
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}

	switch cmd {
	case "/pair":
		return r.handlePair(userID, arg), true
	case "/new":
		return r.handleNew(userID), true
	case "/remember":
		return r.handleRemember(userID, arg), true
	default:
		return "", false
	}
}

// handlePair validates a pairing token and whitelists the user on success.
func (r *CommandRouter) handlePair(userID, token string) string {
	// Check brute-force guard first.
	if !r.guard.Check(userID) {
		r.logger.Warn("pair attempt blocked by brute-force guard", "userID", userID)
		return "Too many failed attempts. Please try again later."
	}

	// Empty pair token means tokenless onboarding mode.
	if r.pairToken == "" {
		added, err := r.whitelist.AddToWhitelist("whatsapp", userID)
		if err != nil {
			r.logger.Error("failed to whitelist user in tokenless mode", "userID", userID, "error", err)
			return fmt.Sprintf("Pairing failed: %v", err)
		}
		if !added {
			return "You are already paired."
		}
		return "Paired successfully! Pair token is not required on this server."
	}

	if !constantTimeTokenCompare(token, r.pairToken) {
		r.guard.RecordFailure(userID)
		r.logger.Warn("invalid pair token", "userID", userID)
		return "Invalid pairing token."
	}

	// Success: whitelist the user.
	r.guard.RecordSuccess(userID)

	added, err := r.whitelist.AddToWhitelist("whatsapp", userID)
	if err != nil {
		r.logger.Error("failed to whitelist user", "userID", userID, "error", err)
		return fmt.Sprintf("Pairing failed: %v", err)
	}

	if !added {
		return "You are already paired."
	}

	r.logger.Info("user paired successfully", "userID", userID)
	return "Paired successfully! You can now send messages."
}

// TokenlessPairingEnabled reports whether users should be auto-onboarded
// without requiring /pair token.
func (r *CommandRouter) TokenlessPairingEnabled() bool {
	return strings.TrimSpace(r.pairToken) == ""
}

// handleNew starts a fresh conversation session.
func (r *CommandRouter) handleNew(userID string) string {
	if err := r.sessionMgr.StartNewSession(userID, "whatsapp /new command"); err != nil {
		r.logger.Error("failed to start new session", "userID", userID, "error", err)
		return fmt.Sprintf("Failed to start new session: %v", err)
	}

	r.logger.Info("new session started via /new", "userID", userID)
	return "New conversation started."
}

// handleRemember saves a fact to the memory repository.
func (r *CommandRouter) handleRemember(userID, fact string) string {
	if fact == "" {
		return "Usage: /remember <fact>"
	}

	source := "whatsapp:" + userID
	ok, err := r.memoryRepo.Append(fact, &source)
	if err != nil {
		r.logger.Error("failed to save memory", "userID", userID, "error", err)
		return fmt.Sprintf("Failed to save: %v", err)
	}

	if !ok {
		return "Memory entry already exists."
	}

	r.logger.Info("memory saved via /remember", "userID", userID, "fact", fact)
	return "Remembered!"
}
