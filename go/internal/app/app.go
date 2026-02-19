package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/pocketbrain/pocketbrain/internal/ai"
	"github.com/pocketbrain/pocketbrain/internal/channel"
	"github.com/pocketbrain/pocketbrain/internal/config"
	"github.com/pocketbrain/pocketbrain/internal/core"
	"github.com/pocketbrain/pocketbrain/internal/scheduler"
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

	// --- Phase 2: Workspace + AI + Assistant ---

	// Initialize workspace service.
	var workspaceService *workspace.Workspace
	if cfg.WorkspaceEnabled {
		workspaceService = workspace.New(cfg.WorkspacePath, logger)
		if err := workspaceService.Initialize(); err != nil {
			return fmt.Errorf("workspace: %w", err)
		}
		logger.Info("workspace initialized", "path", cfg.WorkspacePath)
	}

	// Create AI provider (stub until Fantasy is wired).
	provider := ai.NewStubProvider(logger)

	// Register tool registry.
	toolRegistry := ai.NewRegistry()
	if workspaceService != nil {
		ai.RegisterWorkspaceTools(toolRegistry, workspaceService)
	}
	ai.RegisterMemoryTools(toolRegistry, memoryRepo)
	ai.RegisterChannelTools(toolRegistry, channelRepo, outboxRepo)
	logger.Info("tool registry ready", "toolCount", len(toolRegistry.Names()))

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

	// Create channel manager.
	channelMgr := channel.NewManager(logger)

	// Setup graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	shutdown := newShutdown(logger, cancel, db)

	if workspaceService != nil {
		shutdown.addCloser(func() { _ = workspaceService.Stop() })
	}

	// --- Phase 5: Taildrive file server ---
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

	// Placeholder for channel adapters (Phase 3: WhatsApp).
	_ = channelMgr

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
