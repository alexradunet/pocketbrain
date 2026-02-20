package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchEnvFilePreservesUnknownLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	initial := strings.Join([]string{
		"# comment",
		"CUSTOM_X=keepme",
		"PROVIDER=kronk",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := PatchEnvFile(path, map[string]string{
		"PROVIDER": "openai",
		"MODEL":    "gpt-4o",
	}); err != nil {
		t.Fatalf("PatchEnvFile: %v", err)
	}

	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(out)
	if !strings.Contains(content, "CUSTOM_X=keepme") {
		t.Fatalf("custom line should be preserved, got:\n%s", content)
	}
	if !strings.Contains(content, "PROVIDER=openai") {
		t.Fatalf("provider not updated, got:\n%s", content)
	}
	if !strings.Contains(content, "MODEL=gpt-4o") {
		t.Fatalf("model should be appended, got:\n%s", content)
	}
}
