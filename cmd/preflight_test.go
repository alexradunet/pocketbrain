package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunSetupPreflightHeadlessFailsWhenMissingEnv(t *testing.T) {
	wd, _ := os.Getwd()
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	err := runSetupPreflight(true)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunSetupPreflightNoopWhenEnvComplete(t *testing.T) {
	wd, _ := os.Getwd()
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	content := "" +
		"PROVIDER=openai\n" +
		"MODEL=gpt-4o\n" +
		"ENABLE_WHATSAPP=false\n" +
		"WORKSPACE_PATH=.data/workspace\n" +
		"WEBDAV_ENABLED=false\n"
	if err := os.WriteFile(filepath.Join(tmp, ".env"), []byte(content), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	if err := runSetupPreflight(true); err != nil {
		t.Fatalf("runSetupPreflight: %v", err)
	}
}

func TestReloadEnvFromFileLoadsValues(t *testing.T) {
	tmp := t.TempDir()
	envPath := filepath.Join(tmp, ".env")
	if err := os.WriteFile(envPath, []byte("MODEL=from-dotenv\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	os.Unsetenv("MODEL")
	t.Cleanup(func() { os.Unsetenv("MODEL") })

	if err := reloadEnvFromFile(envPath); err != nil {
		t.Fatalf("reloadEnvFromFile: %v", err)
	}
	if got := os.Getenv("MODEL"); got != "from-dotenv" {
		t.Fatalf("MODEL = %q; want from-dotenv", got)
	}
}
