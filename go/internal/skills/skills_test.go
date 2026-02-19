package skills

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pocketbrain/pocketbrain/internal/workspace"
)

func newTestService(t *testing.T) (*Service, string) {
	t.Helper()
	root := t.TempDir()
	ws := workspace.New(root, slog.Default())
	if err := ws.Initialize(); err != nil {
		t.Fatal(err)
	}
	svc := New(ws, slog.Default())
	return svc, root
}

func seedSkill(t *testing.T, root, name, content string) {
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
// List
// ---------------------------------------------------------------------------

func TestList_EmptyDirectory(t *testing.T) {
	svc, _ := newTestService(t)
	skills, err := svc.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills, got %d", len(skills))
	}
}

func TestList_FindsSkillFiles(t *testing.T) {
	svc, root := newTestService(t)
	seedSkill(t, root, "greeting.md", "---\nname: greeting\n---\nHello!")
	seedSkill(t, root, "farewell.md", "---\nname: farewell\n---\nBye!")

	skills, err := svc.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
}

func TestList_IgnoresNonMarkdownFiles(t *testing.T) {
	svc, root := newTestService(t)
	seedSkill(t, root, "greeting.md", "---\nname: greeting\n---\nHello!")
	seedSkill(t, root, "notes.txt", "not a skill")
	seedSkill(t, root, "data.json", `{"not": "a skill"}`)

	skills, err := svc.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "greeting" {
		t.Fatalf("expected skill name 'greeting', got %q", skills[0].Name)
	}
}

func TestList_ReadsManifestFields(t *testing.T) {
	svc, root := newTestService(t)
	content := "---\nname: deploy\ndescription: Deploy the app\ntrigger: when user says deploy\n---\nDeploy steps here."
	seedSkill(t, root, "deploy.md", content)

	skills, err := svc.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	s := skills[0]
	if s.Name != "deploy" {
		t.Errorf("name = %q; want %q", s.Name, "deploy")
	}
	if s.Description != "Deploy the app" {
		t.Errorf("description = %q; want %q", s.Description, "Deploy the app")
	}
	if s.Trigger != "when user says deploy" {
		t.Errorf("trigger = %q; want %q", s.Trigger, "when user says deploy")
	}
}

// ---------------------------------------------------------------------------
// Load
// ---------------------------------------------------------------------------

func TestLoad_ExistingSkill(t *testing.T) {
	svc, root := newTestService(t)
	seedSkill(t, root, "greeting.md", "---\nname: greeting\ndescription: A greeting skill\n---\nHello world!")

	skill, err := svc.Load("greeting")
	if err != nil {
		t.Fatal(err)
	}
	if skill.Name != "greeting" {
		t.Errorf("name = %q; want %q", skill.Name, "greeting")
	}
	if !strings.Contains(skill.Content, "Hello world!") {
		t.Errorf("content missing expected text, got: %q", skill.Content)
	}
}

func TestLoad_NonExistentSkill(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Load("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent skill")
	}
}

func TestLoad_PathTraversalBlocked(t *testing.T) {
	svc, _ := newTestService(t)
	_, err := svc.Load("../../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
	if !strings.Contains(err.Error(), "invalid skill name") {
		t.Errorf("expected 'invalid skill name' error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestCreate_NewSkill(t *testing.T) {
	svc, root := newTestService(t)
	content := "---\nname: test-skill\ndescription: A test\n---\nBody here."
	err := svc.Create("test-skill", content)
	if err != nil {
		t.Fatal(err)
	}

	data, fileErr := os.ReadFile(filepath.Join(root, "skills", "test-skill.md"))
	if fileErr != nil {
		t.Fatalf("skill file not created: %v", fileErr)
	}
	if string(data) != content {
		t.Errorf("file content = %q; want %q", string(data), content)
	}
}

func TestCreate_WritesManifest(t *testing.T) {
	svc, _ := newTestService(t)
	content := "---\nname: my-skill\ndescription: desc\ntrigger: on request\n---\nContent."
	err := svc.Create("my-skill", content)
	if err != nil {
		t.Fatal(err)
	}

	skill, loadErr := svc.Load("my-skill")
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if skill.Name != "my-skill" {
		t.Errorf("name = %q; want %q", skill.Name, "my-skill")
	}
	if skill.Description != "desc" {
		t.Errorf("description = %q; want %q", skill.Description, "desc")
	}
}

func TestCreate_InvalidName(t *testing.T) {
	svc, _ := newTestService(t)
	for _, name := range []string{"", "../../evil", "has spaces", "semi;colon", "back\\slash"} {
		err := svc.Create(name, "content")
		if err == nil {
			t.Errorf("expected error for invalid name %q", name)
		}
	}
}

// ---------------------------------------------------------------------------
// ParseManifest
// ---------------------------------------------------------------------------

func TestParseManifest_ValidFrontmatter(t *testing.T) {
	input := "---\nname: deploy\ndescription: Deploy the app\ntrigger: on deploy command\n---\nBody content."
	m := ParseManifest(input)
	if m.Name != "deploy" {
		t.Errorf("name = %q; want %q", m.Name, "deploy")
	}
	if m.Description != "Deploy the app" {
		t.Errorf("description = %q; want %q", m.Description, "Deploy the app")
	}
	if m.Trigger != "on deploy command" {
		t.Errorf("trigger = %q; want %q", m.Trigger, "on deploy command")
	}
}

func TestParseManifest_MissingFrontmatter(t *testing.T) {
	input := "Just some content without frontmatter."
	m := ParseManifest(input)
	if m.Name != "" || m.Description != "" || m.Trigger != "" {
		t.Errorf("expected empty manifest, got: %+v", m)
	}
}

func TestParseManifest_PartialFields(t *testing.T) {
	input := "---\nname: partial\n---\nSome content."
	m := ParseManifest(input)
	if m.Name != "partial" {
		t.Errorf("name = %q; want %q", m.Name, "partial")
	}
	if m.Description != "" {
		t.Errorf("description = %q; want empty", m.Description)
	}
	if m.Trigger != "" {
		t.Errorf("trigger = %q; want empty", m.Trigger)
	}
}
