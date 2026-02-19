package ai

import (
	"fmt"
	"math"

	"github.com/pocketbrain/pocketbrain/internal/vault"
)

// RegisterVaultTools adds all 12 vault tools to the registry.
func RegisterVaultTools(reg *Registry, vs *vault.Service) {
	reg.Register(&Tool{
		Name:        "vault_read",
		Description: "Read the contents of a file from the vault. Path is relative to vault root.",
		Parameters: []ToolParam{
			{Name: "path", Type: "string", Description: "Path to the file, relative to vault root", Required: true},
		},
		Execute: func(args map[string]any) (string, error) {
			path := argString(args, "path")
			content, ok := vs.ReadFile(path)
			if !ok {
				return fmt.Sprintf("Error: File not found: %s", path), nil
			}
			return content, nil
		},
	})

	reg.Register(&Tool{
		Name:        "vault_write",
		Description: "Write content to a file in the vault. Creates the file if it doesn't exist, overwrites if it does.",
		Parameters: []ToolParam{
			{Name: "path", Type: "string", Description: "Path to the file, relative to vault root", Required: true},
			{Name: "content", Type: "string", Description: "Content to write to the file", Required: true},
		},
		Execute: func(args map[string]any) (string, error) {
			path := argString(args, "path")
			content := argString(args, "content")
			if vs.WriteFile(path, content) {
				return fmt.Sprintf("Successfully wrote to %s", path), nil
			}
			return fmt.Sprintf("Error: Failed to write to %s", path), nil
		},
	})

	reg.Register(&Tool{
		Name:        "vault_append",
		Description: "Append content to a file in the vault. Creates the file if it doesn't exist.",
		Parameters: []ToolParam{
			{Name: "path", Type: "string", Description: "Path to the file, relative to vault root", Required: true},
			{Name: "content", Type: "string", Description: "Content to append to the file", Required: true},
		},
		Execute: func(args map[string]any) (string, error) {
			path := argString(args, "path")
			content := argString(args, "content")
			if vs.AppendToFile(path, content) {
				return fmt.Sprintf("Successfully appended to %s", path), nil
			}
			return fmt.Sprintf("Error: Failed to append to %s", path), nil
		},
	})

	reg.Register(&Tool{
		Name:        "vault_list",
		Description: "List files and folders in a vault directory.",
		Parameters: []ToolParam{
			{Name: "folder", Type: "string", Description: "Folder path relative to vault root (default: root)", Required: false},
		},
		Execute: func(args map[string]any) (string, error) {
			folder := argString(args, "folder")
			files, _ := vs.ListFiles(folder)
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
		Name:        "vault_search",
		Description: "Search for files in the vault by name, content, or both.",
		Parameters: []ToolParam{
			{Name: "query", Type: "string", Description: "Search query", Required: true},
			{Name: "folder", Type: "string", Description: "Folder to search in (default: entire vault)", Required: false},
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
			files, _ := vs.SearchFiles(query, folder, vault.VaultSearchMode(mode))
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
		Name:        "vault_move",
		Description: "Move or rename a file in the vault.",
		Parameters: []ToolParam{
			{Name: "from", Type: "string", Description: "Source path relative to vault root", Required: true},
			{Name: "to", Type: "string", Description: "Destination path relative to vault root", Required: true},
		},
		Execute: func(args map[string]any) (string, error) {
			from := argString(args, "from")
			to := argString(args, "to")
			if vs.MoveFile(from, to) {
				return fmt.Sprintf("Successfully moved %s to %s", from, to), nil
			}
			return fmt.Sprintf("Error: Failed to move %s", from), nil
		},
	})

	reg.Register(&Tool{
		Name:        "vault_backlinks",
		Description: "Find notes that link to a wiki link target (e.g., 'Project Plan' for [[Project Plan]]).",
		Parameters: []ToolParam{
			{Name: "target", Type: "string", Description: "Wiki link target to find backlinks for", Required: true},
			{Name: "folder", Type: "string", Description: "Folder to search in (default: entire vault)", Required: false},
		},
		Execute: func(args map[string]any) (string, error) {
			target := argString(args, "target")
			folder := argString(args, "folder")
			files, _ := vs.FindBacklinks(target, folder)
			if len(files) == 0 {
				return fmt.Sprintf("No backlinks found for %q", target), nil
			}
			result := fmt.Sprintf("Found %d backlink file(s) for %q:\n", len(files), target)
			for _, f := range files {
				result += fmt.Sprintf("- %s\n", f.Path)
			}
			return result, nil
		},
	})

	reg.Register(&Tool{
		Name:        "vault_tag_search",
		Description: "Find notes containing a tag (supports nested tags like #life/os).",
		Parameters: []ToolParam{
			{Name: "tag", Type: "string", Description: "Tag to search for, with or without # prefix", Required: true},
			{Name: "folder", Type: "string", Description: "Folder to search in (default: entire vault)", Required: false},
		},
		Execute: func(args map[string]any) (string, error) {
			tag := argString(args, "tag")
			folder := argString(args, "folder")
			files, _ := vs.SearchByTag(tag, folder)
			if len(files) == 0 {
				return fmt.Sprintf("No files found with tag %q", tag), nil
			}
			result := fmt.Sprintf("Found %d file(s) with tag %q:\n", len(files), tag)
			for _, f := range files {
				result += fmt.Sprintf("- %s\n", f.Path)
			}
			return result, nil
		},
	})

	reg.Register(&Tool{
		Name:        "vault_daily",
		Description: "Get today's daily note path or append a timestamped entry to it.",
		Parameters: []ToolParam{
			{Name: "content", Type: "string", Description: "Content to append to today's daily note (if not provided, returns the path)", Required: false},
		},
		Execute: func(args map[string]any) (string, error) {
			content := argString(args, "content")
			dailyPath, err := vs.GetTodayDailyNotePath()
			if err != nil {
				return fmt.Sprintf("Error: Could not resolve daily note path: %v", err), nil
			}
			if content == "" {
				existing, ok := vs.ReadFile(dailyPath)
				if !ok || existing == "" {
					return fmt.Sprintf("Today's daily note: %s (doesn't exist yet)", dailyPath), nil
				}
				return fmt.Sprintf("Today's daily note: %s", dailyPath), nil
			}
			if vs.AppendToDaily(content) {
				return fmt.Sprintf("Successfully added timestamped entry to today's daily note (%s)", dailyPath), nil
			}
			return "Error: Failed to update daily note", nil
		},
	})

	reg.Register(&Tool{
		Name:        "vault_daily_track",
		Description: "Set or update a metric in today's daily tracking section (mood, sleep, energy, focus).",
		Parameters: []ToolParam{
			{Name: "metric", Type: "string", Description: "Tracking metric name, for example mood or sleep", Required: true},
			{Name: "value", Type: "string", Description: "Metric value, for example 8/10 or 7h", Required: true},
		},
		Execute: func(args map[string]any) (string, error) {
			metric := argString(args, "metric")
			value := argString(args, "value")
			dailyPath, err := vs.GetTodayDailyNotePath()
			if err != nil {
				dailyPath = "(unknown)"
			}
			if vs.UpsertDailyTracking(metric, value) {
				return fmt.Sprintf("Updated daily tracking (%s) in %s", metric, dailyPath), nil
			}
			return "Error: Failed to update daily tracking. metric and value must both be non-empty.", nil
		},
	})

	reg.Register(&Tool{
		Name:        "vault_obsidian_config",
		Description: "Read .obsidian configuration and summarize where daily notes, new notes, and attachments are saved.",
		Parameters: []ToolParam{
			{Name: "refresh", Type: "boolean", Description: "Force refresh and bypass cached vault fingerprint check (default: false)", Required: false},
		},
		Execute: func(args map[string]any) (string, error) {
			refresh := argBool(args, "refresh")
			summary, err := vs.GetObsidianConfigSummary(refresh)
			if err != nil {
				return fmt.Sprintf("Error reading Obsidian config: %v", err), nil
			}
			if !summary.ObsidianConfigFound {
				return "No .obsidian config found. Ask the user to confirm daily notes folder, new note destination, and attachment folder before creating notes.", nil
			}
			result := "Obsidian config summary:\n"
			result += fmt.Sprintf("- Daily notes: folder=%s, format=%s, template=%s, pluginEnabled=%v\n",
				summary.DailyNotes.Folder, summary.DailyNotes.Format, summary.DailyNotes.TemplateFile, summary.DailyNotes.PluginEnabled)
			result += fmt.Sprintf("- New notes: location=%s, folder=%s\n", summary.NewNotes.Location, summary.NewNotes.Folder)
			result += fmt.Sprintf("- Attachments: folder=%s\n", summary.Attachments.Folder)
			result += fmt.Sprintf("- Link style: %s\n", summary.Links.Style)
			result += fmt.Sprintf("- Templates folder: %s\n", summary.Templates.Folder)
			if len(summary.Warnings) == 0 {
				result += "- Validation: no config warnings detected\n"
			} else {
				result += fmt.Sprintf("- Validation: %d warning(s)\n", len(summary.Warnings))
				for _, w := range summary.Warnings {
					result += fmt.Sprintf("  - %s\n", w)
				}
			}
			return result, nil
		},
	})

	reg.Register(&Tool{
		Name:        "vault_stats",
		Description: "Get statistics about the vault: total files, total size, last modified date.",
		Parameters:  []ToolParam{},
		Execute: func(args map[string]any) (string, error) {
			stats, err := vs.GetStats()
			if err != nil {
				return fmt.Sprintf("Error getting vault stats: %v", err), nil
			}
			lastMod := "N/A"
			if stats.LastModified != nil && !stats.LastModified.IsZero() {
				lastMod = stats.LastModified.Format("2006-01-02T15:04:05Z")
			}
			return fmt.Sprintf("Vault Statistics:\n- Total files: %d\n- Total size: %s\n- Last modified: %s",
				stats.TotalFiles, formatBytes(stats.TotalSize), lastMod), nil
		},
	})
}

// --- helpers ---

func argString(args map[string]any, key string) string {
	v, ok := args[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return s
}

func argBool(args map[string]any, key string) bool {
	v, ok := args[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func displayFolder(folder string) string {
	if folder == "" {
		return "vault root"
	}
	return folder
}

func formatBytes(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}
	k := float64(1024)
	sizes := []string{"B", "KB", "MB", "GB"}
	i := int(math.Floor(math.Log(float64(bytes)) / math.Log(k)))
	if i >= len(sizes) {
		i = len(sizes) - 1
	}
	val := float64(bytes) / math.Pow(k, float64(i))
	return fmt.Sprintf("%.1f %s", val, sizes[i])
}
