package ai

import (
	"context"
	"fmt"

	"charm.land/fantasy"

	"github.com/pocketbrain/pocketbrain/internal/workspace"
)

// Workspace tool input types.

type workspaceReadInput struct {
	Path string `json:"path" description:"Path to the file, relative to workspace root"`
}

type workspaceWriteInput struct {
	Path    string `json:"path" description:"Path to the file, relative to workspace root"`
	Content string `json:"content" description:"Content to write to the file"`
}

type workspaceAppendInput struct {
	Path    string `json:"path" description:"Path to the file, relative to workspace root"`
	Content string `json:"content" description:"Content to append to the file"`
}

type workspaceListInput struct {
	Folder string `json:"folder" description:"Folder path relative to workspace root (default: root)"`
}

type workspaceSearchInput struct {
	Query  string `json:"query" description:"Search query"`
	Folder string `json:"folder" description:"Folder to search in (default: entire workspace)"`
	Mode   string `json:"mode" description:"Search mode: name | content | both (default: name)"`
}

type workspaceMoveInput struct {
	From string `json:"from" description:"Source path relative to workspace root"`
	To   string `json:"to" description:"Destination path relative to workspace root"`
}

type workspaceStatsInput struct{}

// WorkspaceTools returns the 7 workspace tools as Fantasy AgentTools.
func WorkspaceTools(ws *workspace.Workspace) []fantasy.AgentTool {
	return []fantasy.AgentTool{
		fantasy.NewAgentTool(
			"workspace_read",
			"Read the contents of a file from the workspace. Path is relative to workspace root.",
			func(_ context.Context, input workspaceReadInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
				content, ok := ws.ReadFile(input.Path)
				if !ok {
					return fantasy.NewTextResponse(fmt.Sprintf("Error: File not found: %s", input.Path)), nil
				}
				return fantasy.NewTextResponse(content), nil
			},
		),

		fantasy.NewAgentTool(
			"workspace_write",
			"Write content to a file in the workspace. Creates the file if it doesn't exist, overwrites if it does.",
			func(_ context.Context, input workspaceWriteInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
				if ws.WriteFile(input.Path, input.Content) {
					return fantasy.NewTextResponse(fmt.Sprintf("Successfully wrote to %s", input.Path)), nil
				}
				return fantasy.NewTextResponse(fmt.Sprintf("Error: Failed to write to %s", input.Path)), nil
			},
		),

		fantasy.NewAgentTool(
			"workspace_append",
			"Append content to a file in the workspace. Creates the file if it doesn't exist.",
			func(_ context.Context, input workspaceAppendInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
				if ws.AppendToFile(input.Path, input.Content) {
					return fantasy.NewTextResponse(fmt.Sprintf("Successfully appended to %s", input.Path)), nil
				}
				return fantasy.NewTextResponse(fmt.Sprintf("Error: Failed to append to %s", input.Path)), nil
			},
		),

		fantasy.NewAgentTool(
			"workspace_list",
			"List files and folders in a workspace directory.",
			func(_ context.Context, input workspaceListInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
				files, _ := ws.ListFiles(input.Folder)
				if len(files) == 0 {
					return fantasy.NewTextResponse(fmt.Sprintf("Folder is empty: %s", displayFolder(input.Folder))), nil
				}
				result := fmt.Sprintf("Contents of %s:\n", displayFolder(input.Folder))
				for _, f := range files {
					if f.IsDirectory {
						result += fmt.Sprintf("DIR  %s\n", f.Name)
					} else {
						result += fmt.Sprintf("FILE %s (%s)\n", f.Name, formatBytes(f.Size))
					}
				}
				return fantasy.NewTextResponse(result), nil
			},
		),

		fantasy.NewAgentTool(
			"workspace_search",
			"Search for files in the workspace by name, content, or both.",
			func(_ context.Context, input workspaceSearchInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
				mode := input.Mode
				if mode == "" {
					mode = "name"
				}
				if mode != "name" && mode != "content" && mode != "both" {
					return fantasy.NewTextResponse("Error: Invalid search mode. Use one of: name, content, both"), nil
				}
				files, _ := ws.SearchFiles(input.Query, input.Folder, workspace.SearchMode(mode))
				if len(files) == 0 {
					return fantasy.NewTextResponse(fmt.Sprintf("No files found matching %q in %s mode", input.Query, mode)), nil
				}
				result := fmt.Sprintf("Found %d file(s) matching %q in %s mode:\n", len(files), input.Query, mode)
				for _, f := range files {
					result += fmt.Sprintf("- %s\n", f.Path)
				}
				return fantasy.NewTextResponse(result), nil
			},
		),

		fantasy.NewAgentTool(
			"workspace_move",
			"Move or rename a file in the workspace.",
			func(_ context.Context, input workspaceMoveInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
				if ws.MoveFile(input.From, input.To) {
					return fantasy.NewTextResponse(fmt.Sprintf("Successfully moved %s to %s", input.From, input.To)), nil
				}
				return fantasy.NewTextResponse(fmt.Sprintf("Error: Failed to move %s", input.From)), nil
			},
		),

		fantasy.NewAgentTool(
			"workspace_stats",
			"Get statistics about the workspace: total files, total size, last modified date.",
			func(_ context.Context, _ workspaceStatsInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
				stats, err := ws.GetStats()
				if err != nil {
					return fantasy.NewTextResponse(fmt.Sprintf("Error getting workspace stats: %v", err)), nil
				}
				lastMod := "N/A"
				if stats.LastModified != nil && !stats.LastModified.IsZero() {
					lastMod = stats.LastModified.Format("2006-01-02T15:04:05Z")
				}
				return fantasy.NewTextResponse(fmt.Sprintf(
					"Workspace Statistics:\n- Total files: %d\n- Total size: %s\n- Last modified: %s",
					stats.TotalFiles, formatBytes(stats.TotalSize), lastMod,
				)), nil
			},
		),
	}
}
