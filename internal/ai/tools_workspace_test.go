package ai

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/fantasy"

	"github.com/pocketbrain/pocketbrain/internal/workspace"
)

func newTestWorkspaceTools(t *testing.T) ([]fantasy.AgentTool, string) {
	t.Helper()
	root := t.TempDir()
	ws := workspace.New(root, slog.Default())
	return WorkspaceTools(ws), root
}

func runTool(t *testing.T, tool fantasy.AgentTool, input any) string {
	t.Helper()
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "test",
		Name:  tool.Info().Name,
		Input: string(data),
	})
	if err != nil {
		t.Fatal(err)
	}
	return resp.Content
}

func findTool(tools []fantasy.AgentTool, name string) fantasy.AgentTool {
	for _, t := range tools {
		if t.Info().Name == name {
			return t
		}
	}
	return nil
}

func seedTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func TestWorkspaceTools_RegistrationCount(t *testing.T) {
	tools, _ := newTestWorkspaceTools(t)
	if len(tools) != 7 {
		names := make([]string, len(tools))
		for i, tool := range tools {
			names[i] = tool.Info().Name
		}
		t.Fatalf("expected 7 workspace tools, got %d: %v", len(tools), names)
	}
}

// ---------------------------------------------------------------------------
// workspace_read
// ---------------------------------------------------------------------------

func TestWorkspaceTool_Read_Success(t *testing.T) {
	tools, root := newTestWorkspaceTools(t)
	seedTestFile(t, root, "hello.md", "hello content")

	result := runTool(t, findTool(tools, "workspace_read"), workspaceReadInput{Path: "hello.md"})
	if result != "hello content" {
		t.Fatalf("unexpected result: %q", result)
	}
}

func TestWorkspaceTool_Read_NotFound(t *testing.T) {
	tools, _ := newTestWorkspaceTools(t)

	result := runTool(t, findTool(tools, "workspace_read"), workspaceReadInput{Path: "nonexistent.md"})
	if !strings.Contains(result, "Error") {
		t.Fatalf("expected error message, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// workspace_write
// ---------------------------------------------------------------------------

func TestWorkspaceTool_Write_Success(t *testing.T) {
	tools, root := newTestWorkspaceTools(t)

	result := runTool(t, findTool(tools, "workspace_write"), workspaceWriteInput{Path: "new.md", Content: "new content"})
	if !strings.Contains(result, "Successfully") {
		t.Fatalf("expected success, got: %q", result)
	}

	data, _ := os.ReadFile(filepath.Join(root, "new.md"))
	if string(data) != "new content" {
		t.Fatalf("file content = %q, want %q", string(data), "new content")
	}
}

func TestWorkspaceTool_Write_Traversal(t *testing.T) {
	tools, _ := newTestWorkspaceTools(t)

	result := runTool(t, findTool(tools, "workspace_write"), workspaceWriteInput{Path: "../escape.md", Content: "bad"})
	if !strings.Contains(result, "Error") {
		t.Fatalf("expected error for traversal, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// workspace_append
// ---------------------------------------------------------------------------

func TestWorkspaceTool_Append_Success(t *testing.T) {
	tools, root := newTestWorkspaceTools(t)
	seedTestFile(t, root, "log.md", "line1\n")

	result := runTool(t, findTool(tools, "workspace_append"), workspaceAppendInput{Path: "log.md", Content: "line2\n"})
	if !strings.Contains(result, "Successfully") {
		t.Fatalf("expected success, got: %q", result)
	}

	data, _ := os.ReadFile(filepath.Join(root, "log.md"))
	if string(data) != "line1\nline2\n" {
		t.Fatalf("file content = %q", string(data))
	}
}

// ---------------------------------------------------------------------------
// workspace_list
// ---------------------------------------------------------------------------

func TestWorkspaceTool_List_Success(t *testing.T) {
	tools, root := newTestWorkspaceTools(t)
	seedTestFile(t, root, "a.md", "a")
	seedTestFile(t, root, "b.md", "b")

	result := runTool(t, findTool(tools, "workspace_list"), workspaceListInput{})
	if !strings.Contains(result, "a.md") || !strings.Contains(result, "b.md") {
		t.Fatalf("expected file listings, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// workspace_search
// ---------------------------------------------------------------------------

func TestWorkspaceTool_Search_Success(t *testing.T) {
	tools, root := newTestWorkspaceTools(t)
	seedTestFile(t, root, "meeting.md", "notes")
	seedTestFile(t, root, "todo.md", "tasks")

	result := runTool(t, findTool(tools, "workspace_search"), workspaceSearchInput{Query: "meeting"})
	if !strings.Contains(result, "meeting.md") {
		t.Fatalf("expected meeting.md in results, got: %q", result)
	}
}

func TestWorkspaceTool_Search_NoResults(t *testing.T) {
	tools, root := newTestWorkspaceTools(t)
	seedTestFile(t, root, "a.md", "hello")

	result := runTool(t, findTool(tools, "workspace_search"), workspaceSearchInput{Query: "zzzzz"})
	if !strings.Contains(result, "No files found") {
		t.Fatalf("expected no results message, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// workspace_move
// ---------------------------------------------------------------------------

func TestWorkspaceTool_Move_Success(t *testing.T) {
	tools, root := newTestWorkspaceTools(t)
	seedTestFile(t, root, "old.md", "data")

	result := runTool(t, findTool(tools, "workspace_move"), workspaceMoveInput{From: "old.md", To: "new.md"})
	if !strings.Contains(result, "Successfully") {
		t.Fatalf("expected success, got: %q", result)
	}

	if _, err := os.Stat(filepath.Join(root, "new.md")); err != nil {
		t.Fatalf("new.md should exist: %v", err)
	}
}

func TestWorkspaceTool_Move_NotFound(t *testing.T) {
	tools, _ := newTestWorkspaceTools(t)

	result := runTool(t, findTool(tools, "workspace_move"), workspaceMoveInput{From: "nope.md", To: "dest.md"})
	if !strings.Contains(result, "Error") {
		t.Fatalf("expected error, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// workspace_stats
// ---------------------------------------------------------------------------

func TestWorkspaceTool_Stats_Success(t *testing.T) {
	tools, root := newTestWorkspaceTools(t)
	seedTestFile(t, root, "a.md", "hello")

	result := runTool(t, findTool(tools, "workspace_stats"), workspaceStatsInput{})
	if !strings.Contains(result, "Workspace Statistics") {
		t.Fatalf("expected stats output, got: %q", result)
	}
}
