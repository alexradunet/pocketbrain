package ai

import (
	"fmt"
	"strings"

	"github.com/pocketbrain/pocketbrain/internal/core"
)

// RegisterMemoryTools adds save_memory and delete_memory tools.
func RegisterMemoryTools(reg *Registry, repo core.MemoryRepository) {
	reg.Register(&Tool{
		Name:        "save_memory",
		Description: "Append one durable user fact to memory.",
		Parameters: []ToolParam{
			{Name: "fact", Type: "string", Description: "A short, stable user fact worth remembering", Required: true},
		},
		Execute: func(args map[string]any) (string, error) {
			fact := strings.TrimSpace(argString(args, "fact"))
			if fact == "" {
				return "Skipped: empty memory fact.", nil
			}
			inserted, err := repo.Append(fact, nil)
			if err != nil {
				return fmt.Sprintf("Error saving memory: %v", err), nil
			}
			if inserted {
				return "Saved durable memory.", nil
			}
			return "Skipped: similar fact already exists.", nil
		},
	})

	reg.Register(&Tool{
		Name:        "delete_memory",
		Description: "Delete a memory fact by ID.",
		Parameters: []ToolParam{
			{Name: "id", Type: "number", Description: "The memory ID to delete", Required: true},
		},
		Execute: func(args map[string]any) (string, error) {
			idRaw, ok := args["id"]
			if !ok {
				return "Error: missing id parameter", nil
			}
			var id int64
			switch v := idRaw.(type) {
			case float64:
				id = int64(v)
			case int64:
				id = v
			case int:
				id = int64(v)
			default:
				return "Error: invalid id parameter type", nil
			}

			deleted, err := repo.Delete(id)
			if err != nil {
				return fmt.Sprintf("Error deleting memory: %v", err), nil
			}
			if deleted {
				return fmt.Sprintf("Memory %d deleted.", id), nil
			}
			return fmt.Sprintf("Memory %d not found.", id), nil
		},
	})
}
