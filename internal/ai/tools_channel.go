package ai

import (
	"context"
	"fmt"
	"strings"

	"charm.land/fantasy"

	"github.com/pocketbrain/pocketbrain/internal/core"
)

// Channel tool input types.

type sendChannelMessageInput struct {
	Text string `json:"text" description:"Plain-text message to send"`
}

// ChannelTools returns the send_channel_message tool as a Fantasy AgentTool.
func ChannelTools(channelRepo core.ChannelRepository, outboxRepo core.OutboxRepository) []fantasy.AgentTool {
	return []fantasy.AgentTool{
		fantasy.NewAgentTool(
			"send_channel_message",
			"Queue a proactive message to the last used chat channel/user.",
			func(_ context.Context, input sendChannelMessageInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
				text := strings.TrimSpace(input.Text)
				if text == "" {
					return fantasy.NewTextResponse("Skipped: empty message."), nil
				}

				target, err := channelRepo.GetLastChannel()
				if err != nil || target == nil {
					return fantasy.NewTextResponse("No last-used channel found yet."), nil
				}

				channel := strings.TrimSpace(target.Channel)
				userID := strings.TrimSpace(target.UserID)
				if channel == "" || userID == "" {
					return fantasy.NewTextResponse("Last-used channel data is invalid."), nil
				}

				if err := outboxRepo.Enqueue(channel, userID, text, 0); err != nil {
					return fantasy.NewTextResponse(fmt.Sprintf("Error queuing message: %v", err)), nil
				}
				return fantasy.NewTextResponse(fmt.Sprintf("Queued message for %s:%s", channel, userID)), nil
			},
		),
	}
}
