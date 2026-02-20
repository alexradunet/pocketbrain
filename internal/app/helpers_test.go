package app

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/pocketbrain/pocketbrain/internal/config"
	"github.com/pocketbrain/pocketbrain/internal/core"
)

type whitelistCall struct {
	channel string
	userID  string
}

type stubWhitelistRepo struct {
	calls    []whitelistCall
	failFor  map[string]error
	allowMap map[string]bool
}

func (s *stubWhitelistRepo) IsWhitelisted(channel, userID string) (bool, error) {
	return s.allowMap[channel+":"+userID], nil
}

func (s *stubWhitelistRepo) AddToWhitelist(channel, userID string) (bool, error) {
	s.calls = append(s.calls, whitelistCall{channel: channel, userID: userID})
	if err := s.failFor[userID]; err != nil {
		return false, err
	}
	return true, nil
}

func (s *stubWhitelistRepo) RemoveFromWhitelist(channel, userID string) (bool, error) {
	return true, nil
}

type stubMemoryRepo struct{}

func (s stubMemoryRepo) Append(fact string, source *string) (bool, error) { return true, nil }
func (s stubMemoryRepo) Delete(id int64) (bool, error)                    { return true, nil }
func (s stubMemoryRepo) Update(id int64, fact string) (bool, error)       { return true, nil }
func (s stubMemoryRepo) GetAll() ([]core.MemoryEntry, error)              { return nil, nil }

type stubChannelRepo struct{}

func (s stubChannelRepo) SaveLastChannel(channel, userID string) error { return nil }
func (s stubChannelRepo) GetLastChannel() (*core.LastChannel, error) {
	return &core.LastChannel{Channel: "whatsapp", UserID: "u1"}, nil
}

type stubOutboxRepo struct{}

func (s stubOutboxRepo) Enqueue(channel, userID, text string, maxRetries int) error { return nil }
func (s stubOutboxRepo) ListPending(channel string) ([]core.OutboxMessage, error) {
	return nil, nil
}
func (s stubOutboxRepo) Acknowledge(id int64) error                         { return nil }
func (s stubOutboxRepo) MarkRetry(id int64, retryCount int, nextRetryAt string) error { return nil }

func TestApplyWhatsAppWhitelistFromEnv_ContinuesOnErrors(t *testing.T) {
	logger := slog.Default()
	repo := &stubWhitelistRepo{
		failFor: map[string]error{
			"15550001111@lid": errors.New("db write failed"),
		},
		allowMap: map[string]bool{},
	}

	applyWhatsAppWhitelistFromEnv(logger, repo, []string{"15550001111"})
	if len(repo.calls) != 2 {
		t.Fatalf("AddToWhitelist calls = %d, want 2", len(repo.calls))
	}
}

func TestBuildAgentTools_MinimalSetWithoutWorkspace(t *testing.T) {
	tools, names := buildAgentTools(nil, stubMemoryRepo{}, stubChannelRepo{}, stubOutboxRepo{}, slog.Default())
	if len(tools) != 3 {
		t.Fatalf("tool count = %d, want 3", len(tools))
	}

	joined := strings.Join(names, ",")
	for _, want := range []string{"save_memory", "delete_memory", "send_channel_message"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing tool %q in names %v", want, names)
		}
	}
}

func TestBuildProvider_UsesStubWhenAPIKeyMissing(t *testing.T) {
	cfg := &config.Config{
		Provider: "openai",
		APIKey:   "",
		Model:    "gpt-4o",
	}

	provider, providerName, err := buildProvider(context.Background(), cfg, nil, slog.Default())
	if err != nil {
		t.Fatalf("buildProvider returned error: %v", err)
	}
	if providerName != "" {
		t.Fatalf("providerName = %q, want empty for stub path", providerName)
	}

	reply, sendErr := provider.SendMessage(context.Background(), "s1", "", "hello")
	if sendErr != nil {
		t.Fatalf("stub provider SendMessage error: %v", sendErr)
	}
	if !strings.Contains(reply, "not yet configured") {
		t.Fatalf("unexpected stub reply: %q", reply)
	}
}
