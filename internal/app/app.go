package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/pocketbrain/pocketbrain/internal/config"
	"github.com/pocketbrain/pocketbrain/internal/core"
	"github.com/pocketbrain/pocketbrain/internal/scheduler"
	"github.com/pocketbrain/pocketbrain/internal/store"
	"github.com/pocketbrain/pocketbrain/internal/tui"
	"github.com/pocketbrain/pocketbrain/internal/webdav"
	"github.com/pocketbrain/pocketbrain/internal/workspace"
)

// StartBackend wires all backend services using the given event bus.
// Returns a cleanup function. The caller owns the TUI lifecycle.
func StartBackend(bus *tui.EventBus) (func(), error) {
	return startBackendInternal(bus, false)
}

// Run starts PocketBrain in headless mode. It creates its own event bus and
// blocks until interrupted.
func Run(headless bool) error {
	bus := tui.NewEventBus(512)

	cleanup, err := startBackendInternal(bus, headless)
	if err != nil {
		return err
	}

	if headless {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
		<-sigCh
		cleanup()
		return nil
	}

	_ = cleanup
	return nil
}

func startBackendInternal(bus *tui.EventBus, headless bool) (func(), error) {
	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

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
			return nil, fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	// Open database.
	db, err := store.Open(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}

	// Create repositories.
	memoryRepo := store.NewMemoryRepo(db)
	channelRepo := store.NewChannelRepo(db)
	sessionRepo := store.NewSessionRepo(db)
	whitelistRepo := store.NewWhitelistRepo(db)
	outboxRepo := store.NewOutboxRepo(db, cfg.OutboxMaxRetries)
	heartbeatRepo := store.NewHeartbeatRepo(db)

	// Apply env-based WhatsApp phone whitelist.
	applyWhatsAppWhitelistFromEnv(logger, whitelistRepo, cfg.WhatsAppWhitelistNumbers)

	// Log repository readiness.
	taskCount, err := heartbeatRepo.GetTaskCount()
	if err != nil {
		logger.Warn("failed to read heartbeat task count", "error", err)
		taskCount = 0
	}
	memories, err := memoryRepo.GetAll()
	if err != nil {
		logger.Warn("failed to read memory entries", "error", err)
		memories = nil
	}
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
			return nil, fmt.Errorf("workspace: %w", err)
		}
		logger.Info("workspace initialized", "path", cfg.WorkspacePath)
	}

	tools, toolNames := buildAgentTools(workspaceService, memoryRepo, channelRepo, outboxRepo, logger)
	logger.Info("tools ready", "toolCount", len(tools), "toolNames", toolNames)

	// Create AI provider based on configuration.
	ctx, cancel := context.WithCancel(context.Background())
	shutdown := newShutdown(logger, cancel, db)

	provider, providerName, err := buildProvider(ctx, cfg, tools, logger)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("ai provider: %w", err)
	}
	if providerName != "" {
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

	// --- WebDAV workspace file server ---
	if cfg.WebDAVEnabled && workspaceService != nil {
		wdSvc, err := webdav.New(webdav.Config{
			Enabled: true,
			Addr:    cfg.WebDAVAddr,
			RootDir: workspaceService.RootPath(),
			Logger:  logger,
		})
		if err != nil {
			return nil, fmt.Errorf("webdav: %w", err)
		}
		if err := wdSvc.Start(); err != nil {
			return nil, fmt.Errorf("webdav start: %w", err)
		}
		shutdown.addCloser(func() { _ = wdSvc.Stop() })
		bus.Publish(tui.Event{
			Type: tui.EventWebDAVStatus,
			Data: tui.StatusEvent{Connected: true, Detail: "listening on " + wdSvc.Addr()},
		})
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
		if err := startWhatsApp(ctx, cfg, assistant, logger, bus, whitelistRepo, memoryRepo, channelRepo, outboxRepo, shutdown); err != nil {
			return nil, err
		}
	}

	// Register signal handlers.
	shutdown.handleSignals()

	return shutdown.run, nil
}
