package ai

import (
	"fmt"
	"strings"

	"github.com/pocketbrain/pocketbrain/internal/core"
)

// RegisterChannelTools adds the send_channel_message tool.
func RegisterChannelTools(reg *Registry, channelRepo core.ChannelRepository, outboxRepo core.OutboxRepository) {
	reg.Register(&Tool{
		Name:        "send_channel_message",
		Description: "Queue a proactive message to the last used chat channel/user.",
		Parameters: []ToolParam{
			{Name: "text", Type: "string", Description: "Plain-text message to send", Required: true},
		},
		Execute: func(args map[string]any) (string, error) {
			text := strings.TrimSpace(argString(args, "text"))
			if text == "" {
				return "Skipped: empty message.", nil
			}

			target, err := channelRepo.GetLastChannel()
			if err != nil || target == nil {
				return "No last-used channel found yet.", nil
			}

			channel := strings.TrimSpace(target.Channel)
			userID := strings.TrimSpace(target.UserID)
			if channel == "" || userID == "" {
				return "Last-used channel data is invalid.", nil
			}

			if err := outboxRepo.Enqueue(channel, userID, text, 0); err != nil {
				return fmt.Sprintf("Error queuing message: %v", err), nil
			}
			return fmt.Sprintf("Queued message for %s:%s", channel, userID), nil
		},
	})
}
