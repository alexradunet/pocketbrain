package ai

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/fantasy"

	"github.com/pocketbrain/pocketbrain/internal/skills"
	"github.com/pocketbrain/pocketbrain/internal/workspace"
)

func newTestSkillsTools(t *testing.T) ([]fantasy.AgentTool, string) {
	t.Helper()
	root := t.TempDir()
	ws := workspace.New(root, slog.Default())
	if err := ws.Initialize(); err != nil {
		t.Fatal(err)
	}
	svc := skills.New(ws, slog.Default())
	return SkillsTools(svc, slog.Default()), root
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
	tools, _ := newTestSkillsTools(t)
	if len(tools) != 4 {
		names := make([]string, len(tools))
		for i, tool := range tools {
			names[i] = tool.Info().Name
		}
		t.Fatalf("expected 4 skills tools, got %d: %v", len(tools), names)
	}
}

// ---------------------------------------------------------------------------
// skill_list
// ---------------------------------------------------------------------------

func TestSkillsTool_List_Empty(t *testing.T) {
	tools, _ := newTestSkillsTools(t)
	result := runTool(t, findTool(tools, "skill_list"), skillListInput{})
	if !strings.Contains(result, "No skills found") {
		t.Fatalf("expected no-skills message, got: %q", result)
	}
}

func TestSkillsTool_List_WithSkills(t *testing.T) {
	tools, root := newTestSkillsTools(t)
	seedSkillFile(t, root, "greeting.md", "---\nname: greeting\ndescription: Greets people\n---\nHello!")

	result := runTool(t, findTool(tools, "skill_list"), skillListInput{})
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
	tools, root := newTestSkillsTools(t)
	seedSkillFile(t, root, "deploy.md", "---\nname: deploy\n---\nDeploy steps here.")

	result := runTool(t, findTool(tools, "skill_load"), skillLoadInput{Name: "deploy"})
	if !strings.Contains(result, "Deploy steps here") {
		t.Fatalf("expected skill content, got: %q", result)
	}
}

func TestSkillsTool_Load_NotFound(t *testing.T) {
	tools, _ := newTestSkillsTools(t)
	result := runTool(t, findTool(tools, "skill_load"), skillLoadInput{Name: "nonexistent"})
	if !strings.Contains(result, "Error") {
		t.Fatalf("expected error message, got: %q", result)
	}
}

func TestSkillsTool_Load_EmptyName(t *testing.T) {
	tools, _ := newTestSkillsTools(t)
	result := runTool(t, findTool(tools, "skill_load"), skillLoadInput{Name: ""})
	if !strings.Contains(result, "Error") {
		t.Fatalf("expected error message, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// skill_create
// ---------------------------------------------------------------------------

func TestSkillsTool_Create_Success(t *testing.T) {
	tools, root := newTestSkillsTools(t)
	result := runTool(t, findTool(tools, "skill_create"), skillCreateInput{
		Name:    "my-skill",
		Content: "---\nname: my-skill\n---\nContent here.",
	})
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
	tools, _ := newTestSkillsTools(t)
	result := runTool(t, findTool(tools, "skill_create"), skillCreateInput{
		Name:    "../bad-name",
		Content: "content",
	})
	if !strings.Contains(result, "Error") {
		t.Fatalf("expected error for invalid name, got: %q", result)
	}
}

func TestSkillsTool_Create_EmptyContent(t *testing.T) {
	tools, _ := newTestSkillsTools(t)
	result := runTool(t, findTool(tools, "skill_create"), skillCreateInput{
		Name:    "test",
		Content: "",
	})
	if !strings.Contains(result, "Error") {
		t.Fatalf("expected error for empty content, got: %q", result)
	}
}

// ---------------------------------------------------------------------------
// install_skill
// ---------------------------------------------------------------------------

func TestSkillsTool_Install_InvalidURL(t *testing.T) {
	tools, _ := newTestSkillsTools(t)
	result := runTool(t, findTool(tools, "install_skill"), installSkillInput{URL: "not-a-url"})
	if !strings.Contains(result, "Error") {
		t.Fatalf("expected error for invalid URL, got: %q", result)
	}
}

func TestSkillsTool_Install_EmptyURL(t *testing.T) {
	tools, _ := newTestSkillsTools(t)
	result := runTool(t, findTool(tools, "install_skill"), installSkillInput{URL: ""})
	if !strings.Contains(result, "Error") {
		t.Fatalf("expected error for empty URL, got: %q", result)
	}
}
