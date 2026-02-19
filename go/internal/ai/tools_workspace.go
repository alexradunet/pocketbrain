package ai

import (
	"fmt"

	"github.com/pocketbrain/pocketbrain/internal/workspace"
)

// RegisterWorkspaceTools adds the 7 workspace tools to the registry.
func RegisterWorkspaceTools(reg *Registry, ws *workspace.Workspace) {
	reg.Register(&Tool{
		Name:        "workspace_read",
		Description: "Read the contents of a file from the workspace. Path is relative to workspace root.",
		Parameters: []ToolParam{
			{Name: "path", Type: "string", Description: "Path to the file, relative to workspace root", Required: true},
		},
		Execute: func(args map[string]any) (string, error) {
			path := argString(args, "path")
			content, ok := ws.ReadFile(path)
			if !ok {
				return fmt.Sprintf("Error: File not found: %s", path), nil
			}
			return content, nil
		},
	})

	reg.Register(&Tool{
		Name:        "workspace_write",
		Description: "Write content to a file in the workspace. Creates the file if it doesn't exist, overwrites if it does.",
		Parameters: []ToolParam{
			{Name: "path", Type: "string", Description: "Path to the file, relative to workspace root", Required: true},
			{Name: "content", Type: "string", Description: "Content to write to the file", Required: true},
		},
		Execute: func(args map[string]any) (string, error) {
			path := argString(args, "path")
			content := argString(args, "content")
			if ws.WriteFile(path, content) {
				return fmt.Sprintf("Successfully wrote to %s", path), nil
			}
			return fmt.Sprintf("Error: Failed to write to %s", path), nil
		},
	})

	reg.Register(&Tool{
		Name:        "workspace_append",
		Description: "Append content to a file in the workspace. Creates the file if it doesn't exist.",
		Parameters: []ToolParam{
			{Name: "path", Type: "string", Description: "Path to the file, relative to workspace root", Required: true},
			{Name: "content", Type: "string", Description: "Content to append to the file", Required: true},
		},
		Execute: func(args map[string]any) (string, error) {
			path := argString(args, "path")
			content := argString(args, "content")
			if ws.AppendToFile(path, content) {
				return fmt.Sprintf("Successfully appended to %s", path), nil
			}
			return fmt.Sprintf("Error: Failed to append to %s", path), nil
		},
	})

	reg.Register(&Tool{
		Name:        "workspace_list",
		Description: "List files and folders in a workspace directory.",
		Parameters: []ToolParam{
			{Name: "folder", Type: "string", Description: "Folder path relative to workspace root (default: root)", Required: false},
		},
		Execute: func(args map[string]any) (string, error) {
			folder := argString(args, "folder")
			files, _ := ws.ListFiles(folder)
			if len(files) == 0 {
				if folder == "" {
					return "Folder is empty: root", nil
				}
				return fmt.Sprintf("Folder is empty: %s", folder), nil
			}
			result := fmt.Sprintf("Contents of %s:\n", displayFolder(folder))
			for _, f := range files {
				if f.IsDirectory {
					result += fmt.Sprintf("DIR  %s\n", f.Name)
				} else {
					result += fmt.Sprintf("FILE %s (%s)\n", f.Name, formatBytes(f.Size))
				}
			}
			return result, nil
		},
	})

	reg.Register(&Tool{
		Name:        "workspace_search",
		Description: "Search for files in the workspace by name, content, or both.",
		Parameters: []ToolParam{
			{Name: "query", Type: "string", Description: "Search query", Required: true},
			{Name: "folder", Type: "string", Description: "Folder to search in (default: entire workspace)", Required: false},
			{Name: "mode", Type: "string", Description: "Search mode: name | content | both (default: name)", Required: false},
		},
		Execute: func(args map[string]any) (string, error) {
			query := argString(args, "query")
			folder := argString(args, "folder")
			mode := argString(args, "mode")
			if mode == "" {
				mode = "name"
			}
			if mode != "name" && mode != "content" && mode != "both" {
				return "Error: Invalid search mode. Use one of: name, content, both", nil
			}
			files, _ := ws.SearchFiles(query, folder, workspace.SearchMode(mode))
			if len(files) == 0 {
				return fmt.Sprintf("No files found matching %q in %s mode", query, mode), nil
			}
			result := fmt.Sprintf("Found %d file(s) matching %q in %s mode:\n", len(files), query, mode)
			for _, f := range files {
				result += fmt.Sprintf("- %s\n", f.Path)
			}
			return result, nil
		},
	})

	reg.Register(&Tool{
		Name:        "workspace_move",
		Description: "Move or rename a file in the workspace.",
		Parameters: []ToolParam{
			{Name: "from", Type: "string", Description: "Source path relative to workspace root", Required: true},
			{Name: "to", Type: "string", Description: "Destination path relative to workspace root", Required: true},
		},
		Execute: func(args map[string]any) (string, error) {
			from := argString(args, "from")
			to := argString(args, "to")
			if ws.MoveFile(from, to) {
				return fmt.Sprintf("Successfully moved %s to %s", from, to), nil
			}
			return fmt.Sprintf("Error: Failed to move %s", from), nil
		},
	})

	reg.Register(&Tool{
		Name:        "workspace_stats",
		Description: "Get statistics about the workspace: total files, total size, last modified date.",
		Parameters:  []ToolParam{},
		Execute: func(args map[string]any) (string, error) {
			stats, err := ws.GetStats()
			if err != nil {
				return fmt.Sprintf("Error getting workspace stats: %v", err), nil
			}
			lastMod := "N/A"
			if stats.LastModified != nil && !stats.LastModified.IsZero() {
				lastMod = stats.LastModified.Format("2006-01-02T15:04:05Z")
			}
			return fmt.Sprintf("Workspace Statistics:\n- Total files: %d\n- Total size: %s\n- Last modified: %s",
				stats.TotalFiles, formatBytes(stats.TotalSize), lastMod), nil
		},
	})
}
