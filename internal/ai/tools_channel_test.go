package ai

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"charm.land/fantasy"

	"github.com/pocketbrain/pocketbrain/internal/core"
)

type stubChannelRepo struct {
	last *core.LastChannel
}

func (s *stubChannelRepo) SaveLastChannel(channel, userID string) error {
	s.last = &core.LastChannel{Channel: channel, UserID: userID}
	return nil
}

func (s *stubChannelRepo) GetLastChannel() (*core.LastChannel, error) {
	return s.last, nil
}

type enqueueCall struct {
	channel    string
	userID     string
	text       string
	maxRetries int
}

type stubOutboxRepo struct {
	enqueued []enqueueCall
}

func (s *stubOutboxRepo) Enqueue(channel, userID, text string, maxRetries int) error {
	s.enqueued = append(s.enqueued, enqueueCall{
		channel:    channel,
		userID:     userID,
		text:       text,
		maxRetries: maxRetries,
	})
	return nil
}

func (s *stubOutboxRepo) ListPending(channel string) ([]core.OutboxMessage, error) { return nil, nil }
func (s *stubOutboxRepo) Acknowledge(id int64) error                               { return nil }
func (s *stubOutboxRepo) MarkRetry(id int64, retryCount int, nextRetryAt string) error {
	return nil
}

func runChannelTool(t *testing.T, tool fantasy.AgentTool, input any) string {
	t.Helper()
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "test",
		Name:  tool.Info().Name,
		Input: string(data),
	})
	if err != nil {
		t.Fatal(err)
	}
	return resp.Content
}

func TestChannelTool_QueuesForWhatsApp(t *testing.T) {
	channelRepo := &stubChannelRepo{last: &core.LastChannel{Channel: "whatsapp", UserID: "user@s.whatsapp.net"}}
	outboxRepo := &stubOutboxRepo{}
	tools := ChannelTools(channelRepo, outboxRepo, slog.Default())

	result := runChannelTool(t, tools[0], sendChannelMessageInput{Text: "hello"})
	if !strings.Contains(result, "Queued message") {
		t.Fatalf("unexpected result: %q", result)
	}
	if len(outboxRepo.enqueued) != 1 {
		t.Fatalf("enqueued calls = %d, want 1", len(outboxRepo.enqueued))
	}
}

func TestChannelTool_SkipsUnsupportedChannel(t *testing.T) {
	channelRepo := &stubChannelRepo{last: &core.LastChannel{Channel: "tui", UserID: "local"}}
	outboxRepo := &stubOutboxRepo{}
	tools := ChannelTools(channelRepo, outboxRepo, slog.Default())

	result := runChannelTool(t, tools[0], sendChannelMessageInput{Text: "hello"})
	if !strings.Contains(result, "not currently supported") {
		t.Fatalf("unexpected result: %q", result)
	}
	if len(outboxRepo.enqueued) != 0 {
		t.Fatalf("enqueued calls = %d, want 0", len(outboxRepo.enqueued))
	}
}
