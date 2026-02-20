package config

import (
	"os"
	"path/filepath"
	"testing"
)

func clearEnv() {
	for _, key := range []string{
		"APP_NAME", "LOG_LEVEL", "DATA_DIR", "PROVIDER", "MODEL",
		"HEARTBEAT_INTERVAL_MINUTES", "ENABLE_WHATSAPP", "WHATSAPP_AUTH_DIR",
		"WHITELIST_PAIR_TOKEN", "WHATSAPP_PAIR_MAX_FAILURES",
		"WHATSAPP_PAIR_FAILURE_WINDOW_MS", "WHATSAPP_PAIR_BLOCK_DURATION_MS",
		"WHATSAPP_WHITELIST_NUMBERS", "WHATSAPP_WHITELIST_NUMBER",
		"TAILDRIVE_ENABLED", "TAILDRIVE_SHARE_NAME", "TAILDRIVE_AUTO_SHARE",
		"WORKSPACE_PATH", "WORKSPACE_ENABLED", "VAULT_PATH", "VAULT_ENABLED",
		"POCKETBRAIN_HOME",
	} {
		os.Unsetenv(key)
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.AppName != "pocketbrain" {
		t.Errorf("AppName = %q, want %q", cfg.AppName, "pocketbrain")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.Provider != "kronk" {
		t.Errorf("Provider = %q, want %q", cfg.Provider, "kronk")
	}
	if cfg.HeartbeatIntervalMinutes != 30 {
		t.Errorf("HeartbeatIntervalMinutes = %d, want 30", cfg.HeartbeatIntervalMinutes)
	}
	if cfg.EnableWhatsApp != false {
		t.Error("EnableWhatsApp should default to false")
	}
	if cfg.WorkspaceEnabled != true {
		t.Error("WorkspaceEnabled should default to true")
	}
	if cfg.TaildriveEnabled != false {
		t.Error("TaildriveEnabled should default to false")
	}
	if cfg.TaildriveShareName != "workspace" {
		t.Errorf("TaildriveShareName = %q, want %q", cfg.TaildriveShareName, "workspace")
	}
	if cfg.OutboxIntervalMs != 60_000 {
		t.Errorf("OutboxIntervalMs = %d, want 60000", cfg.OutboxIntervalMs)
	}
	if cfg.OutboxMaxRetries != 3 {
		t.Errorf("OutboxMaxRetries = %d, want 3", cfg.OutboxMaxRetries)
	}
}

func TestLoadFromEnv(t *testing.T) {
	clearEnv()
	t.Setenv("APP_NAME", "testbrain")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("HEARTBEAT_INTERVAL_MINUTES", "15")
	t.Setenv("WORKSPACE_ENABLED", "false")
	t.Setenv("ENABLE_WHATSAPP", "false")
	t.Setenv("TAILDRIVE_ENABLED", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.AppName != "testbrain" {
		t.Errorf("AppName = %q, want %q", cfg.AppName, "testbrain")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.HeartbeatIntervalMinutes != 15 {
		t.Errorf("HeartbeatIntervalMinutes = %d, want 15", cfg.HeartbeatIntervalMinutes)
	}
	if cfg.WorkspaceEnabled != false {
		t.Error("WorkspaceEnabled should be false")
	}
	if cfg.TaildriveEnabled != true {
		t.Error("TaildriveEnabled should be true")
	}
}

func TestDataDirResolution(t *testing.T) {
	clearEnv()
	cwd, _ := os.Getwd()

	// Relative path
	t.Setenv("DATA_DIR", "testdata")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(cwd, "testdata")
	if cfg.DataDir != expected {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, expected)
	}

	// Absolute path
	clearEnv()
	t.Setenv("DATA_DIR", "/tmp/pb-test-data")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DataDir != "/tmp/pb-test-data" {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, "/tmp/pb-test-data")
	}
}

func TestParsePhoneWhitelist(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		expect []string
	}{
		{"empty", []string{""}, nil},
		{"single", []string{"15551234567"}, []string{"15551234567"}},
		{"comma separated", []string{"111,222,333"}, []string{"111", "222", "333"}},
		{"with formatting", []string{"+1 (555) 123-4567"}, []string{"15551234567"}},
		{"dedup", []string{"111,111,222"}, []string{"111", "222"}},
		{"multi values", []string{"111", "222"}, []string{"111", "222"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePhoneWhitelist(tt.input...)
			if len(result) != len(tt.expect) {
				t.Errorf("len = %d, want %d (got %v)", len(result), len(tt.expect), result)
				return
			}
			for i, v := range result {
				if v != tt.expect[i] {
					t.Errorf("result[%d] = %q, want %q", i, v, tt.expect[i])
				}
			}
		})
	}
}

func TestEnvBool(t *testing.T) {
	tests := []struct {
		input    string
		fallback bool
		expect   bool
	}{
		{"", false, false},
		{"", true, true},
		{"1", false, true},
		{"true", false, true},
		{"TRUE", false, true},
		{"yes", false, true},
		{"on", false, true},
		{"0", true, false},
		{"false", true, false},
		{"no", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			key := "TEST_BOOL_VAR"
			if tt.input != "" {
				os.Setenv(key, tt.input)
			} else {
				os.Unsetenv(key)
			}
			defer os.Unsetenv(key)

			got := envBool(key, tt.fallback)
			if got != tt.expect {
				t.Errorf("envBool(%q, %v) = %v, want %v", tt.input, tt.fallback, got, tt.expect)
			}
		})
	}
}

func TestValidation(t *testing.T) {
	clearEnv()
	t.Setenv("ENABLE_WHATSAPP", "true")
	t.Setenv("WHATSAPP_AUTH_DIR", "")
	// WhatsApp enabled but no auth dir should still work since it defaults to DATA_DIR/whatsapp-auth.

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.WhatsAppAuthDir == "" {
		t.Error("WhatsAppAuthDir should have a default value")
	}
}
