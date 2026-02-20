package ai

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pocketbrain/pocketbrain/internal/skills"
	"github.com/pocketbrain/pocketbrain/internal/workspace"
)

func newTestSkillsRegistry(t *testing.T) (*Registry, string) {
	t.Helper()
	root := t.TempDir()
	ws := workspace.New(root, slog.Default())
	if err := ws.Initialize(); err != nil {
		t.Fatal(err)
	}
	svc := skills.New(ws, slog.Default())
	reg := NewRegistry()
	RegisterSkillsTools(reg, svc)
	return reg, root
}

func seedSkillFile(t *testing.T, root, name, content string) {
	t.Helper()
	dir := filepath.Join(root, "skills")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func TestSkillsTools_RegistrationCount(t *testing.T) {
	reg, _ := newTestSkillsRegistry(t)
	names := reg.Names()
	if len(names) != 4 {
		t.Fatalf("expected 4 skills tools, got %d: %v", len(names), names)
	}
}

// ---------------------------------------------------------------------------
// skill_list
// ---------------------------------------------------------------------------

func TestSkillsTool_List_Empty(t *testing.T) {
	reg, _ := newTestSkillsRegistry(t)
	tool, _ := reg.Get("skill_list")
	result, err := tool.Execute(map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "No skills found") {
		t.Fatalf("expected no-skills message, got: %q", result)
	}
}

func TestSkillsTool_List_WithSkills(t *testing.T) {
	reg, root := newTestSkillsRegistry(t)
	seedSkillFile(t, root, "greeting.md", "---\nname: greeting\ndescription: Greets people\n---\nHello!")

	tool, _ := reg.Get("skill_list")
	result, err := tool.Execute(map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "greeting") {
		t.Fatalf("expected skill listing, got: %q", result)
	}
	if !strings.Contains(result, "Greets people") {
		t.Fatalf("expected description in listing, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// skill_load
// ---------------------------------------------------------------------------

func TestSkillsTool_Load_Success(t *testing.T) {
	reg, root := newTestSkillsRegistry(t)
	seedSkillFile(t, root, "deploy.md", "---\nname: deploy\n---\nDeploy steps here.")

	tool, _ := reg.Get("skill_load")
	result, err := tool.Execute(map[string]any{"name": "deploy"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Deploy steps here") {
		t.Fatalf("expected skill content, got: %q", result)
	}
}

func TestSkillsTool_Load_NotFound(t *testing.T) {
	reg, _ := newTestSkillsRegistry(t)
	tool, _ := reg.Get("skill_load")
	result, err := tool.Execute(map[string]any{"name": "nonexistent"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Error") {
		t.Fatalf("expected error message, got: %q", result)
	}
}

func TestSkillsTool_Load_EmptyName(t *testing.T) {
	reg, _ := newTestSkillsRegistry(t)
	tool, _ := reg.Get("skill_load")
	result, err := tool.Execute(map[string]any{"name": ""})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Error") {
		t.Fatalf("expected error message, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// skill_create
// ---------------------------------------------------------------------------

func TestSkillsTool_Create_Success(t *testing.T) {
	reg, root := newTestSkillsRegistry(t)
	tool, _ := reg.Get("skill_create")
	result, err := tool.Execute(map[string]any{
		"name":    "my-skill",
		"content": "---\nname: my-skill\n---\nContent here.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "created successfully") {
		t.Fatalf("expected success message, got: %q", result)
	}

	// Verify file exists.
	data, readErr := os.ReadFile(filepath.Join(root, "skills", "my-skill.md"))
	if readErr != nil {
		t.Fatalf("skill file not created: %v", readErr)
	}
	if !strings.Contains(string(data), "Content here") {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}

func TestSkillsTool_Create_InvalidName(t *testing.T) {
	reg, _ := newTestSkillsRegistry(t)
	tool, _ := reg.Get("skill_create")
	result, err := tool.Execute(map[string]any{
		"name":    "../bad-name",
		"content": "content",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Error") {
		t.Fatalf("expected error for invalid name, got: %q", result)
	}
}

func TestSkillsTool_Create_EmptyContent(t *testing.T) {
	reg, _ := newTestSkillsRegistry(t)
	tool, _ := reg.Get("skill_create")
	result, err := tool.Execute(map[string]any{
		"name":    "test",
		"content": "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Error") {
		t.Fatalf("expected error for empty content, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// install_skill
// ---------------------------------------------------------------------------

func TestSkillsTool_Install_InvalidURL(t *testing.T) {
	reg, _ := newTestSkillsRegistry(t)
	tool, _ := reg.Get("install_skill")
	result, err := tool.Execute(map[string]any{"url": "not-a-url"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Error") {
		t.Fatalf("expected error for invalid URL, got: %q", result)
	}
}

func TestSkillsTool_Install_EmptyURL(t *testing.T) {
	reg, _ := newTestSkillsRegistry(t)
	tool, _ := reg.Get("install_skill")
	result, err := tool.Execute(map[string]any{"url": ""})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Error") {
		t.Fatalf("expected error for empty URL, got: %q", result)
	}
}
