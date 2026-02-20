package app

import (
	"context"
	"log/slog"
	"strings"

	"charm.land/fantasy"

	"github.com/pocketbrain/pocketbrain/internal/ai"
	"github.com/pocketbrain/pocketbrain/internal/config"
	"github.com/pocketbrain/pocketbrain/internal/core"
	"github.com/pocketbrain/pocketbrain/internal/skills"
	"github.com/pocketbrain/pocketbrain/internal/workspace"
)

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
