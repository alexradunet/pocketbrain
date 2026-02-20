package setup

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
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
		"n", // do not auto-generate token
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

func TestWizardRunGeneratesWhatsAppPairToken(t *testing.T) {
	input := strings.Join([]string{
		"2", // anthropic
		"claude-sonnet-4-20250514",
		"sk-ant-123",
		"y", // whatsapp
		".data/whatsapp-auth",
		"y", // auto-generate token
		".data/workspace",
		"n", // tailscale
		"n", // taildrive
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
	if !strings.Contains(content, "WHITELIST_PAIR_TOKEN=pb_") {
		t.Fatalf("expected generated WHITELIST_PAIR_TOKEN, got:\n%s", content)
	}
	if !strings.Contains(out.String(), "Generated WhatsApp pair token: pb_") {
		t.Fatalf("expected generated token message, got:\n%s", out.String())
	}
}

func TestParseKronkCatalogModelIDs(t *testing.T) {
	md := []byte(`
| [Qwen3-8B-Q8_0](https://example.com/a) | Text |
| [gpt-oss-20b-Q8_0](https://example.com/b) | Text |
`)
	got := parseKronkCatalogModelIDs(md)
	want := []string{"Qwen3-8B-Q8_0", "gpt-oss-20b-Q8_0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseKronkCatalogModelIDs = %v, want %v", got, want)
	}
}

func TestWizardRunKronkCatalogSelectionAndDownload(t *testing.T) {
	input := strings.Join([]string{
		"1", // kronk provider
		"2,1",
		"y", // download selected models
		"n", // enable whatsapp
		".data/workspace",
		"n", // tailscale
		"n", // taildrive
		"",
	}, "\n")

	var out bytes.Buffer
	w := NewWizard(strings.NewReader(input), &out)

	var downloaded []string
	w.fetchCatalog = func() ([]string, error) {
		return []string{"Qwen3-8B-Q8_0", "gpt-oss-20b-Q8_0"}, nil
	}
	w.resolveModelValue = func(id string) (string, error) {
		return "https://huggingface.co/repo/resolve/main/" + id + ".gguf", nil
	}
	w.download = func(_ io.Writer, modelID string) error {
		downloaded = append(downloaded, modelID)
		return nil
	}

	envPath := filepath.Join(t.TempDir(), ".env")
	if err := w.Run(envPath); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "PROVIDER=kronk") {
		t.Fatalf("missing PROVIDER in env: %s", content)
	}
	// First selected model is resolved to runtime MODEL URL.
	if !strings.Contains(content, "MODEL=https://huggingface.co/repo/resolve/main/gpt-oss-20b-Q8_0.gguf") {
		t.Fatalf("missing selected MODEL in env: %s", content)
	}
	if !reflect.DeepEqual(downloaded, []string{"gpt-oss-20b-Q8_0", "Qwen3-8B-Q8_0"}) {
		t.Fatalf("downloaded = %v", downloaded)
	}
}

func TestWizardRunKronkSkipsDownloadWhenUserChoosesNo(t *testing.T) {
	input := strings.Join([]string{
		"1", // kronk provider
		"1",
		"n", // download selected models
		"n", // enable whatsapp
		".data/workspace",
		"n", // tailscale
		"n", // taildrive
		"",
	}, "\n")

	var out bytes.Buffer
	w := NewWizard(strings.NewReader(input), &out)
	w.fetchCatalog = func() ([]string, error) { return []string{"Qwen3-8B-Q8_0"}, nil }
	w.resolveModelValue = func(id string) (string, error) {
		return "https://huggingface.co/repo/resolve/main/" + id + ".gguf", nil
	}
	w.download = func(_ io.Writer, modelID string) error {
		t.Fatalf("download should not be called, got %s", modelID)
		return nil
	}

	envPath := filepath.Join(t.TempDir(), ".env")
	if err := w.Run(envPath); err != nil {
		t.Fatalf("Run: %v", err)
	}
}
