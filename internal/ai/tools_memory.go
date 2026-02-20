package ai

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"charm.land/fantasy"

	"github.com/pocketbrain/pocketbrain/internal/core"
)

// Memory tool input types.

type saveMemoryInput struct {
	Fact string `json:"fact" description:"A short, stable user fact worth remembering"`
}

type deleteMemoryInput struct {
	ID float64 `json:"id" description:"The memory ID to delete"`
}

// MemoryTools returns save_memory and delete_memory as Fantasy AgentTools.
func MemoryTools(repo core.MemoryRepository, logger *slog.Logger) []fantasy.AgentTool {
	return []fantasy.AgentTool{
		fantasy.NewAgentTool(
			"save_memory",
			"Append one durable user fact to memory.",
			func(_ context.Context, input saveMemoryInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
				fact := strings.TrimSpace(input.Fact)
				if fact == "" {
					logger.Info("tool result", "op", "tool.execute", "tool", "save_memory", "result", "empty")
					return fantasy.NewTextResponse("Skipped: empty memory fact."), nil
				}
				logger.Info("tool execute", "op", "tool.execute", "tool", "save_memory", "factLen", len(fact))
				inserted, err := repo.Append(fact, nil)
				if err != nil {
					logger.Info("tool result", "op", "tool.execute", "tool", "save_memory", "result", "error", "error", err)
					return fantasy.NewTextResponse(fmt.Sprintf("Error saving memory: %v", err)), nil
				}
				if inserted {
					logger.Info("tool result", "op", "tool.execute", "tool", "save_memory", "result", "inserted")
					return fantasy.NewTextResponse("Saved durable memory."), nil
				}
				logger.Info("tool result", "op", "tool.execute", "tool", "save_memory", "result", "duplicate")
				return fantasy.NewTextResponse("Skipped: similar fact already exists."), nil
			},
		),

		fantasy.NewAgentTool(
			"delete_memory",
			"Delete a memory fact by ID.",
			func(_ context.Context, input deleteMemoryInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
				id := int64(input.ID)
				logger.Info("tool execute", "op", "tool.execute", "tool", "delete_memory", "memoryID", id)
				deleted, err := repo.Delete(id)
				if err != nil {
					logger.Info("tool result", "op", "tool.execute", "tool", "delete_memory", "result", "error", "error", err)
					return fantasy.NewTextResponse(fmt.Sprintf("Error deleting memory: %v", err)), nil
				}
				if deleted {
					logger.Info("tool result", "op", "tool.execute", "tool", "delete_memory", "result", "deleted", "memoryID", id)
					return fantasy.NewTextResponse(fmt.Sprintf("Memory %d deleted.", id)), nil
				}
				logger.Info("tool result", "op", "tool.execute", "tool", "delete_memory", "result", "not_found", "memoryID", id)
				return fantasy.NewTextResponse(fmt.Sprintf("Memory %d not found.", id)), nil
			},
		),
	}
}
