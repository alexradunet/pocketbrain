package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"charm.land/fantasy"

	"github.com/pocketbrain/pocketbrain/internal/ai"
	"github.com/pocketbrain/pocketbrain/internal/channel/whatsapp"
	"github.com/pocketbrain/pocketbrain/internal/config"
	"github.com/pocketbrain/pocketbrain/internal/core"
	"github.com/pocketbrain/pocketbrain/internal/scheduler"
	"github.com/pocketbrain/pocketbrain/internal/skills"
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

func buildAgentTools(
	workspaceService *workspace.Workspace,
	memoryRepo core.MemoryRepository,
	channelRepo core.ChannelRepository,
	outboxRepo core.OutboxRepository,
	logger *slog.Logger,
) ([]fantasy.AgentTool, []string) {
	var tools []fantasy.AgentTool
	var toolNames []string

	appendTools := func(list []fantasy.AgentTool) {
		tools = append(tools, list...)
		for _, t := range list {
			toolNames = append(toolNames, t.Info().Name)
		}
	}

	if workspaceService != nil {
		appendTools(ai.WorkspaceTools(workspaceService, logger))
		skillsService := skills.New(workspaceService, logger)
		appendTools(ai.SkillsTools(skillsService, logger))
	}

	appendTools(ai.MemoryTools(memoryRepo, logger))
	appendTools(ai.ChannelTools(channelRepo, outboxRepo, logger))
	return tools, toolNames
}

func buildProvider(ctx context.Context, cfg *config.Config, tools []fantasy.AgentTool, logger *slog.Logger) (core.Provider, string, error) {
	providerName := strings.TrimSpace(cfg.Provider)
	if providerName == "" {
		providerName = "kronk"
	}

	if providerName != "kronk" && cfg.APIKey == "" {
		logger.Warn("no API_KEY configured for provider; using stub provider", "provider", providerName)
		return ai.NewStubProvider(logger), "", nil
	}

	fp, err := ai.NewFantasyProvider(ctx, ai.FantasyProviderConfig{
		ProviderName: providerName,
		APIKey:       cfg.APIKey,
		Model:        cfg.Model,
		Tools:        tools,
		Logger:       logger,
	})
	if err != nil {
		return nil, "", err
	}
	return fp, providerName, nil
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
