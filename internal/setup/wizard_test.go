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
		"+5511987654321", // whatsapp allowed number
		".data/workspace",
		"y",            // webdav
		"0.0.0.0:6060", // webdav addr
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
		"WEBDAV_ENABLED=true",
		"WEBDAV_ADDR=0.0.0.0:6060",
		"WHATSAPP_WHITELIST_NUMBERS=+5511987654321",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %q in .env:\n%s", want, content)
		}
	}
}

func TestWizardRunRejectsPhoneWithoutPlus(t *testing.T) {
	input := strings.Join([]string{
		"2", // anthropic
		"claude-sonnet-4-20250514",
		"sk-ant-123",
		"y", // whatsapp
		".data/whatsapp-auth",
		"5511987654321", // missing + prefix
		"",
	}, "\n")

	var out bytes.Buffer
	w := NewWizard(strings.NewReader(input), &out)
	envPath := filepath.Join(t.TempDir(), ".env")
	err := w.Run(envPath)
	if err == nil {
		t.Fatal("expected error for phone number without +, got nil")
	}
	if !strings.Contains(err.Error(), "international format") {
		t.Fatalf("expected international format error, got: %v", err)
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
		"n", // webdav
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
		"n", // webdav
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
