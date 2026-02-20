package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"charm.land/fantasy"

	"github.com/pocketbrain/pocketbrain/internal/ai"
	"github.com/pocketbrain/pocketbrain/internal/channel/whatsapp"
	"github.com/pocketbrain/pocketbrain/internal/config"
	"github.com/pocketbrain/pocketbrain/internal/core"
	"github.com/pocketbrain/pocketbrain/internal/scheduler"
	"github.com/pocketbrain/pocketbrain/internal/skills"
	"github.com/pocketbrain/pocketbrain/internal/store"
	"github.com/pocketbrain/pocketbrain/internal/taildrive"
	"github.com/pocketbrain/pocketbrain/internal/tui"
	"github.com/pocketbrain/pocketbrain/internal/workspace"
)

// Run is the composition root. It wires all dependencies and starts the app.
func Run(headless bool) error {
	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// Create event bus for TUI <-> backend communication.
	bus := tui.NewEventBus(512)

	// Setup structured logging.
	var logLevel slog.Level
	switch cfg.LogLevel {
	case "debug", "trace":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error", "fatal":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	var handler slog.Handler
	if headless {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	} else {
		handler = NewBusHandler(bus, logLevel)
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)

	logger.Info("starting PocketBrain",
		"appName", cfg.AppName,
		"dataDir", cfg.DataDir,
		"headless", headless,
	)

	// Ensure data directories exist.
	for _, dir := range []string{cfg.DataDir, cfg.PocketBrainHome} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	// Open database.
	db, err := store.Open(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}

	// Create repositories.
	memoryRepo := store.NewMemoryRepo(db)
	channelRepo := store.NewChannelRepo(db)
	sessionRepo := store.NewSessionRepo(db)
	whitelistRepo := store.NewWhitelistRepo(db)
	outboxRepo := store.NewOutboxRepo(db, cfg.OutboxMaxRetries)
	heartbeatRepo := store.NewHeartbeatRepo(db)

	// Apply env-based WhatsApp phone whitelist.
	if len(cfg.WhatsAppWhitelistNumbers) > 0 {
		var added int
		for _, phone := range cfg.WhatsAppWhitelistNumbers {
			directJID := phone + "@s.whatsapp.net"
			lidJID := phone + "@lid"
			if ok, _ := whitelistRepo.AddToWhitelist("whatsapp", directJID); ok {
				added++
			}
			if ok, _ := whitelistRepo.AddToWhitelist("whatsapp", lidJID); ok {
				added++
			}
		}
		logger.Info("applied WhatsApp whitelist from environment",
			"configuredCount", len(cfg.WhatsAppWhitelistNumbers),
			"addedCount", added,
		)
	}

	// Log repository readiness.
	taskCount, _ := heartbeatRepo.GetTaskCount()
	memories, _ := memoryRepo.GetAll()
	logger.Info("repositories ready",
		"heartbeatTasks", taskCount,
		"memoryEntries", len(memories),
	)

	// Publish initial stats to TUI.
	bus.Publish(tui.Event{Type: tui.EventMemoryStats, Data: tui.StatsEvent{Label: "memory", Count: len(memories)}})

	if taskCount == 0 {
		logger.Warn("no heartbeat tasks configured; add tasks via SQL: INSERT INTO heartbeat_tasks (task) VALUES ('your task')")
	}

	// --- Workspace + AI + Assistant ---

	// Initialize workspace service.
	var workspaceService *workspace.Workspace
	if cfg.WorkspaceEnabled {
		workspaceService = workspace.New(cfg.WorkspacePath, logger)
		if err := workspaceService.Initialize(); err != nil {
			return fmt.Errorf("workspace: %w", err)
		}
		logger.Info("workspace initialized", "path", cfg.WorkspacePath)
	}

	// Build Fantasy agent tools.
	var tools []fantasy.AgentTool
	if workspaceService != nil {
		tools = append(tools, ai.WorkspaceTools(workspaceService)...)

		// Skills tools (skills live inside workspace).
		skillsService := skills.New(workspaceService, logger)
		tools = append(tools, ai.SkillsTools(skillsService)...)
	}
	tools = append(tools, ai.MemoryTools(memoryRepo)...)
	tools = append(tools, ai.ChannelTools(channelRepo, outboxRepo)...)
	logger.Info("tools ready", "toolCount", len(tools))

	// Create AI provider based on configuration.
	ctx, cancel := context.WithCancel(context.Background())
	shutdown := newShutdown(logger, cancel, db)

	var provider core.Provider
	switch {
	case cfg.APIKey == "":
		provider = ai.NewStubProvider(logger)
		logger.Warn("no API_KEY configured; using stub provider")
	default:
		providerName := cfg.Provider
		if providerName == "" {
			providerName = "openai"
		}
		fp, err := ai.NewFantasyProvider(ctx, ai.FantasyProviderConfig{
			ProviderName: providerName,
			APIKey:       cfg.APIKey,
			Model:        cfg.Model,
			Tools:        tools,
			Logger:       logger,
		})
		if err != nil {
			cancel()
			return fmt.Errorf("ai provider: %w", err)
		}
		provider = fp
		logger.Info("AI provider ready", "provider", providerName, "model", cfg.Model)
	}

	// Create session manager.
	sessionMgr := core.NewSessionManager(sessionRepo, logger)

	// Create prompt builder.
	promptBuilder := core.NewPromptBuilder(core.PromptBuilderOptions{
		HeartbeatIntervalMinutes: cfg.HeartbeatIntervalMinutes,
		WorkspaceEnabled:         cfg.WorkspaceEnabled,
	})

	// Create assistant core.
	assistant := core.NewAssistantCore(core.AssistantCoreOptions{
		Provider:      provider,
		SessionMgr:    sessionMgr,
		PromptBuilder: promptBuilder,
		MemoryRepo:    memoryRepo,
		ChannelRepo:   channelRepo,
		HeartbeatRepo: heartbeatRepo,
		Logger:        logger,
	})

	if workspaceService != nil {
		shutdown.addCloser(func() { _ = workspaceService.Stop() })
	}

	// --- Taildrive file server ---
	if cfg.TaildriveEnabled && workspaceService != nil {
		tdSvc, err := taildrive.New(taildrive.Config{
			Enabled:   true,
			ShareName: cfg.TaildriveShareName,
			AutoShare: cfg.TaildriveAutoShare,
			RootDir:   workspaceService.RootPath(),
			Logger:    logger,
		})
		if err != nil {
			return fmt.Errorf("taildrive: %w", err)
		}
		if err := tdSvc.Start(); err != nil {
			return fmt.Errorf("taildrive start: %w", err)
		}
		shutdown.addCloser(func() { _ = tdSvc.Stop() })
		logger.Info("taildrive file server ready",
			"addr", tdSvc.Addr(),
			"shareName", cfg.TaildriveShareName,
		)
	}

	// Wire and start the heartbeat scheduler (assistant implements HeartbeatRunner).
	sched := scheduler.NewHeartbeatScheduler(
		scheduler.HeartbeatConfig{
			IntervalMinutes:     cfg.HeartbeatIntervalMinutes,
			BaseDelayMs:         cfg.HeartbeatBaseDelayMs,
			MaxDelayMs:          cfg.HeartbeatMaxDelayMs,
			NotifyAfterFailures: cfg.HeartbeatNotifyAfterFailures,
		},
		assistant,
		outboxRepo,
		channelRepo,
		logger,
	)
	sched.Start(ctx)
	shutdown.addCloser(sched.Stop)

	if cfg.EnableWhatsApp {
		waClient, err := whatsapp.NewWhatsmeowClient(whatsapp.WhatsmeowConfig{
			AuthDir: cfg.WhatsAppAuthDir,
			Logger:  logger,
		})
		if err != nil {
			return fmt.Errorf("whatsapp client: %w", err)
		}

		guard := whatsapp.NewBruteForceGuard(
			cfg.WhatsAppPairMaxFailures,
			cfg.WhatsAppPairFailureWindowMs,
			cfg.WhatsAppPairBlockDurationMs,
		)

		cmdRouter := whatsapp.NewCommandRouter(
			cfg.WhitelistPairToken,
			guard,
			whitelistRepo,
			memoryRepo,
			&sessionStarterAdapter{ctx: ctx, a: assistant},
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
	}

	// Register signal handlers.
	shutdown.handleSignals()

	if headless {
		logger.Info("running in headless mode (Ctrl+C to stop)")
		<-ctx.Done()
		shutdown.run()
		return nil
	}

	// Start TUI.
	logger.Info("starting TUI")
	go func() {
		<-ctx.Done()
	}()

	if err := tui.Run(bus); err != nil {
		shutdown.run()
		return fmt.Errorf("tui: %w", err)
	}

	shutdown.run()
	return nil
}

// sessionStarterAdapter bridges AssistantCore to the whatsapp.SessionStarter
// interface (drops the returned session ID).
type sessionStarterAdapter struct {
	ctx context.Context
	a   *core.AssistantCore
}

func (s *sessionStarterAdapter) StartNewSession(_, reason string) error {
	_, err := s.a.StartNewMainSession(s.ctx, reason)
	return err
}
