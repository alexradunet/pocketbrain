package whatsapp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWhatsmeowClient_NewClient_CreatesDB(t *testing.T) {
	dir := t.TempDir()
	authDir := filepath.Join(dir, "wa-auth")

	client, err := NewWhatsmeowClient(WhatsmeowConfig{
		AuthDir: authDir,
		Logger:  testLogger(),
	})
	if err != nil {
		t.Fatalf("NewWhatsmeowClient: %v", err)
	}
	defer client.Close()

	dbPath := filepath.Join(authDir, "whatsapp.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("expected DB file at %s", dbPath)
	}
}

func TestWhatsmeowClient_NewClient_InvalidAuthDir(t *testing.T) {
	_, err := NewWhatsmeowClient(WhatsmeowConfig{
		AuthDir: "/dev/null/nope",
		Logger:  testLogger(),
	})
	if err == nil {
		t.Fatal("expected error for invalid auth dir")
	}
}

func TestWhatsmeowClient_IsConnected_InitiallyFalse(t *testing.T) {
	dir := t.TempDir()
	client, err := NewWhatsmeowClient(WhatsmeowConfig{
		AuthDir: dir,
		Logger:  testLogger(),
	})
	if err != nil {
		t.Fatalf("NewWhatsmeowClient: %v", err)
	}
	defer client.Close()

	if client.IsConnected() {
		t.Error("expected IsConnected() == false before Connect()")
	}
}

func TestWhatsmeowClient_Close_Idempotent(t *testing.T) {
	dir := t.TempDir()
	client, err := NewWhatsmeowClient(WhatsmeowConfig{
		AuthDir: dir,
		Logger:  testLogger(),
	})
	if err != nil {
		t.Fatalf("NewWhatsmeowClient: %v", err)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second close should not panic.
	_ = client.Close()
}
