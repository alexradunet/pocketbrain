package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvFileMissingIsNoop(t *testing.T) {
	clearEnv()
	err := LoadDotEnvFile(filepath.Join(t.TempDir(), ".env"))
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
}

func TestLoadDotEnvFileSetsOnlyUnsetKeys(t *testing.T) {
	clearEnv()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "PROVIDER=openai\nMODEL=gpt-4o-mini\nAPI_KEY=from-file\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Setenv("MODEL", "from-os")

	if err := LoadDotEnvFile(path); err != nil {
		t.Fatalf("LoadDotEnvFile: %v", err)
	}

	if got := os.Getenv("PROVIDER"); got != "openai" {
		t.Fatalf("PROVIDER = %q, want openai", got)
	}
	if got := os.Getenv("MODEL"); got != "from-os" {
		t.Fatalf("MODEL = %q, want from-os", got)
	}
	if got := os.Getenv("API_KEY"); got != "from-file" {
		t.Fatalf("API_KEY = %q, want from-file", got)
	}
}
