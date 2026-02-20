package app

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/pocketbrain/pocketbrain/internal/channel/whatsapp"
	"github.com/pocketbrain/pocketbrain/internal/config"
	"github.com/pocketbrain/pocketbrain/internal/core"
	"github.com/pocketbrain/pocketbrain/internal/tui"
)

func applyWhatsAppWhitelistFromEnv(logger *slog.Logger, whitelistRepo core.WhitelistRepository, phones []string) {
	if len(phones) == 0 {
		return
	}

	var added int
	for _, phone := range phones {
		directJID := phone + "@s.whatsapp.net"
		lidJID := phone + "@lid"
		if ok, err := whitelistRepo.AddToWhitelist("whatsapp", directJID); err != nil {
			logger.Warn("failed to apply WhatsApp whitelist entry", "jid", directJID, "error", err)
		} else if ok {
			added++
		}
		if ok, err := whitelistRepo.AddToWhitelist("whatsapp", lidJID); err != nil {
			logger.Warn("failed to apply WhatsApp whitelist entry", "jid", lidJID, "error", err)
		} else if ok {
			added++
		}
	}

	logger.Info("applied WhatsApp whitelist from environment",
		"configuredCount", len(phones),
		"addedCount", added,
	)
}

func startWhatsApp(
	ctx context.Context,
	cfg *config.Config,
	assistant *core.AssistantCore,
	logger *slog.Logger,
	bus *tui.EventBus,
	whitelistRepo core.WhitelistRepository,
	memoryRepo core.MemoryRepository,
	channelRepo core.ChannelRepository,
	outboxRepo core.OutboxRepository,
	shutdown *shutdown,
) error {
	waClient, err := whatsapp.NewWhatsmeowClient(whatsapp.WhatsmeowConfig{
		AuthDir: cfg.WhatsAppAuthDir,
		Logger:  logger,
	})
	if err != nil {
		return fmt.Errorf("whatsapp client: %w", err)
	}

	cmdRouter := whatsapp.NewCommandRouter(
		whitelistRepo,
		memoryRepo,
		&sessionStarterAdapter{
			ctx:         ctx,
			a:           assistant,
			channelRepo: channelRepo,
			bus:         bus,
		},
		logger,
	)

	waAdapter := whatsapp.NewAdapter(waClient, logger)
	processor := whatsapp.NewMessageProcessor(whitelistRepo, cmdRouter,
		func(userID, text string) (string, error) {
			return assistant.Ask(ctx, core.AssistantInput{
				Channel: "whatsapp", UserID: userID, Text: text,
			})
		}, logger)

	waClient.SetOnMessage(func(userID, text string) {
		reply, err := processor.Process(userID, text)
		if err != nil {
			logger.Error("whatsapp message processing failed", "error", err)
			return
		}
		if reply != "" {
			if err := waAdapter.Send(userID, reply); err != nil {
				logger.Error("whatsapp send failed", "error", err)
			}
		}
	})

	if err := waAdapter.Start(func(userID, text string) (string, error) {
		return processor.Process(userID, text)
	}); err != nil {
		_ = waClient.Close()
		return fmt.Errorf("whatsapp start: %w", err)
	}

	outboxProcessor := whatsapp.NewOutboxProcessor(outboxRepo, waClient, logger)
	if err := outboxProcessor.ProcessPending(); err != nil {
		logger.Error("initial outbox processing failed", "error", err)
	}

	outboxInterval := time.Duration(cfg.OutboxIntervalMs) * time.Millisecond
	if outboxInterval <= 0 {
		outboxInterval = time.Minute
	}

	outboxStop := make(chan struct{})
	var outboxStopOnce sync.Once
	go func() {
		ticker := time.NewTicker(outboxInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-outboxStop:
				return
			case <-ticker.C:
				if err := outboxProcessor.ProcessPending(); err != nil {
					logger.Error("outbox processing failed", "error", err)
				}
			}
		}
	}()

	shutdown.addCloser(func() {
		outboxStopOnce.Do(func() { close(outboxStop) })
	})
	shutdown.addCloser(func() { _ = waAdapter.Stop(); _ = waClient.Close() })

	logger.Info("whatsapp adapter ready")
	return nil
}
