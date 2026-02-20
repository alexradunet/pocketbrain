package ai

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pocketbrain/pocketbrain/internal/workspace"
)

func newTestWorkspaceRegistry(t *testing.T) (*Registry, string) {
	t.Helper()
	root := t.TempDir()
	ws := workspace.New(root, slog.Default())
	reg := NewRegistry()
	RegisterWorkspaceTools(reg, ws)
	return reg, root
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
	reg, _ := newTestWorkspaceRegistry(t)
	names := reg.Names()
	if len(names) != 7 {
		t.Fatalf("expected 7 workspace tools, got %d: %v", len(names), names)
	}
}

// ---------------------------------------------------------------------------
// workspace_read
// ---------------------------------------------------------------------------

func TestWorkspaceTool_Read_Success(t *testing.T) {
	reg, root := newTestWorkspaceRegistry(t)
	seedTestFile(t, root, "hello.md", "hello content")

	tool, _ := reg.Get("workspace_read")
	result, err := tool.Execute(map[string]any{"path": "hello.md"})
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello content" {
		t.Fatalf("unexpected result: %q", result)
	}
}

func TestWorkspaceTool_Read_NotFound(t *testing.T) {
	reg, _ := newTestWorkspaceRegistry(t)

	tool, _ := reg.Get("workspace_read")
	result, err := tool.Execute(map[string]any{"path": "nonexistent.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Error") {
		t.Fatalf("expected error message, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// workspace_write
// ---------------------------------------------------------------------------

func TestWorkspaceTool_Write_Success(t *testing.T) {
	reg, root := newTestWorkspaceRegistry(t)

	tool, _ := reg.Get("workspace_write")
	result, err := tool.Execute(map[string]any{"path": "new.md", "content": "new content"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Successfully") {
		t.Fatalf("expected success, got: %q", result)
	}

	data, _ := os.ReadFile(filepath.Join(root, "new.md"))
	if string(data) != "new content" {
		t.Fatalf("file content = %q, want %q", string(data), "new content")
	}
}

func TestWorkspaceTool_Write_Traversal(t *testing.T) {
	reg, _ := newTestWorkspaceRegistry(t)

	tool, _ := reg.Get("workspace_write")
	result, err := tool.Execute(map[string]any{"path": "../escape.md", "content": "bad"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Error") {
		t.Fatalf("expected error for traversal, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// workspace_append
// ---------------------------------------------------------------------------

func TestWorkspaceTool_Append_Success(t *testing.T) {
	reg, root := newTestWorkspaceRegistry(t)
	seedTestFile(t, root, "log.md", "line1\n")

	tool, _ := reg.Get("workspace_append")
	result, err := tool.Execute(map[string]any{"path": "log.md", "content": "line2\n"})
	if err != nil {
		t.Fatal(err)
	}
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
	reg, root := newTestWorkspaceRegistry(t)
	seedTestFile(t, root, "a.md", "a")
	seedTestFile(t, root, "b.md", "b")

	tool, _ := reg.Get("workspace_list")
	result, err := tool.Execute(map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "a.md") || !strings.Contains(result, "b.md") {
		t.Fatalf("expected file listings, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// workspace_search
// ---------------------------------------------------------------------------

func TestWorkspaceTool_Search_Success(t *testing.T) {
	reg, root := newTestWorkspaceRegistry(t)
	seedTestFile(t, root, "meeting.md", "notes")
	seedTestFile(t, root, "todo.md", "tasks")

	tool, _ := reg.Get("workspace_search")
	result, err := tool.Execute(map[string]any{"query": "meeting"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "meeting.md") {
		t.Fatalf("expected meeting.md in results, got: %q", result)
	}
}

func TestWorkspaceTool_Search_NoResults(t *testing.T) {
	reg, root := newTestWorkspaceRegistry(t)
	seedTestFile(t, root, "a.md", "hello")

	tool, _ := reg.Get("workspace_search")
	result, err := tool.Execute(map[string]any{"query": "zzzzz"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "No files found") {
		t.Fatalf("expected no results message, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// workspace_move
// ---------------------------------------------------------------------------

func TestWorkspaceTool_Move_Success(t *testing.T) {
	reg, root := newTestWorkspaceRegistry(t)
	seedTestFile(t, root, "old.md", "data")

	tool, _ := reg.Get("workspace_move")
	result, err := tool.Execute(map[string]any{"from": "old.md", "to": "new.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Successfully") {
		t.Fatalf("expected success, got: %q", result)
	}
}

func TestWorkspaceTool_Move_NotFound(t *testing.T) {
	reg, _ := newTestWorkspaceRegistry(t)

	tool, _ := reg.Get("workspace_move")
	result, err := tool.Execute(map[string]any{"from": "nope.md", "to": "dest.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Error") {
		t.Fatalf("expected error, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// workspace_stats
// ---------------------------------------------------------------------------

func TestWorkspaceTool_Stats_Success(t *testing.T) {
	reg, root := newTestWorkspaceRegistry(t)
	seedTestFile(t, root, "a.md", "hello")

	tool, _ := reg.Get("workspace_stats")
	result, err := tool.Execute(map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Workspace Statistics") {
		t.Fatalf("expected stats output, got: %q", result)
	}
}
