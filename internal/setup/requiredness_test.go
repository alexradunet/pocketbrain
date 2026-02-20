package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNeedSetupWhenMissingEnv(t *testing.T) {
	need, reason, err := NeedSetup(filepath.Join(t.TempDir(), ".env"))
	if err != nil {
		t.Fatalf("NeedSetup error: %v", err)
	}
	if !need {
		t.Fatal("expected need setup")
	}
	if reason == "" {
		t.Fatal("expected reason")
	}
}

func TestNeedSetupFalseWhenComplete(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "" +
		"PROVIDER=openai\n" +
		"MODEL=gpt-4o\n" +
		"ENABLE_WHATSAPP=true\n" +
		"WORKSPACE_PATH=.data/workspace\n" +
		"WEBDAV_ENABLED=true\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	need, reason, err := NeedSetup(path)
	if err != nil {
		t.Fatalf("NeedSetup error: %v", err)
	}
	if need {
		t.Fatalf("expected no setup needed, got reason=%q", reason)
	}
}
