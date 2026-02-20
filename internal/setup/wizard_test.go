package setup

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWizardRunWritesEnv(t *testing.T) {
	input := strings.Join([]string{
		"2", // anthropic
		"claude-sonnet-4-20250514",
		"sk-ant-123", // API key
		"y",          // whatsapp
		".data/whatsapp-auth",
		"pair-token-1",
		".data/workspace",
		"y", // tailscale
		"tskey-auth-123",
		"pocketbrain",
		".data/tsnet",
		"y", // taildrive
		"workspace",
		"y",
		"",
	}, "\n")

	var out bytes.Buffer
	w := NewWizard(strings.NewReader(input), &out)
	envPath := filepath.Join(t.TempDir(), ".env")
	if err := w.Run(envPath); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		"PROVIDER=anthropic",
		"MODEL=claude-sonnet-4-20250514",
		"API_KEY=sk-ant-123",
		"TAILSCALE_ENABLED=true",
		"TS_AUTHKEY=tskey-auth-123",
		"TAILDRIVE_ENABLED=true",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %q in .env:\n%s", want, content)
		}
	}
}
