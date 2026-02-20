package app

import (
	"context"
	"time"

	"github.com/pocketbrain/pocketbrain/internal/core"
	"github.com/pocketbrain/pocketbrain/internal/tui"
)

// sessionStarterAdapter bridges AssistantCore to the whatsapp.SessionStarter
// interface (drops the returned session ID).
type sessionStarterAdapter struct {
	ctx         context.Context
	a           *core.AssistantCore
	channelRepo core.ChannelRepository
	bus         *tui.EventBus
}

func (s *sessionStarterAdapter) StartNewSession(userID, reason string) error {
	if _, err := s.a.StartNewMainSession(s.ctx, reason); err != nil {
		return err
	}

	version, err := s.a.MainSessionVersion()
	if err != nil {
		version = 0
	}

	if s.channelRepo != nil {
		_ = s.channelRepo.SaveLastChannel("whatsapp", userID)
	}

	if s.bus != nil {
		s.bus.Publish(tui.Event{
			Type: tui.EventSessionChanged,
			Data: tui.SessionChangedEvent{
				Channel: "whatsapp",
				UserID:  userID,
				Reason:  reason,
				Version: version,
			},
			Timestamp: time.Now(),
		})
	}

	return nil
}
