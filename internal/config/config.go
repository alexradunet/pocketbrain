package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds all application configuration.
type Config struct {
	AppName  string
	LogLevel string
	DataDir  string

	// AI provider
	Provider string
	Model    string
	APIKey   string

	// Heartbeat / scheduler
	HeartbeatIntervalMinutes     int
	HeartbeatBaseDelayMs         int
	HeartbeatMaxDelayMs          int
	HeartbeatNotifyAfterFailures int

	// WhatsApp channel
	EnableWhatsApp  bool
	WhatsAppAuthDir string

	// Outbox (message queue)
	OutboxIntervalMs int
	OutboxMaxRetries int

	// WhatsApp whitelist
	WhatsAppWhitelistNumbers []string

	// WebDAV (workspace file sharing)
	WebDAVEnabled bool
	WebDAVAddr    string

	// Workspace (files exposed via WebDAV)
	WorkspacePath    string
	WorkspaceEnabled bool

	// Derived paths
	PocketBrainHome string
}

// Load reads environment variables and returns a validated Config.
func Load() (*Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}

	dataDir := resolvePath(cwd, envStr("DATA_DIR", ".data"))

	// Workspace path: WORKSPACE_PATH > default
	workspaceEnabled := envBool("WORKSPACE_ENABLED", true)
	workspacePathRaw := strings.TrimSpace(os.Getenv("WORKSPACE_PATH"))

	var workspacePath string
	if workspacePathRaw != "" {
		workspacePath = resolvePath(cwd, workspacePathRaw)
	} else {
		workspacePath = filepath.Join(dataDir, "workspace")
	}

	pocketBrainHome := filepath.Join(workspacePath, "99-system", "99-pocketbrain")
	if v := strings.TrimSpace(os.Getenv("POCKETBRAIN_HOME")); v != "" {
		pocketBrainHome = resolvePath(cwd, v)
	}

	waAuthDir := strings.TrimSpace(os.Getenv("WHATSAPP_AUTH_DIR"))
	if waAuthDir == "" {
		waAuthDir = filepath.Join(dataDir, "whatsapp-auth")
	} else {
		waAuthDir = resolvePath(cwd, waAuthDir)
	}

	cfg := &Config{
		AppName:  envStr("APP_NAME", "pocketbrain"),
		LogLevel: envStr("LOG_LEVEL", "info"),
		DataDir:  dataDir,

		Provider: envStr("PROVIDER", "kronk"),
		Model:    envStr("MODEL", ""),
		APIKey:   envStr("API_KEY", ""),

		HeartbeatIntervalMinutes:     envInt("HEARTBEAT_INTERVAL_MINUTES", 30),
		HeartbeatBaseDelayMs:         60_000,
		HeartbeatMaxDelayMs:          30 * 60 * 1000,
		HeartbeatNotifyAfterFailures: 3,

		EnableWhatsApp:  envBool("ENABLE_WHATSAPP", false),
		WhatsAppAuthDir: waAuthDir,

		OutboxIntervalMs: 60_000,
		OutboxMaxRetries: 3,

		WhatsAppWhitelistNumbers: parsePhoneWhitelist(os.Getenv("WHATSAPP_WHITELIST_NUMBERS"), os.Getenv("WHATSAPP_WHITELIST_NUMBER")),

		WebDAVEnabled: envBool("WEBDAV_ENABLED", false),
		WebDAVAddr:    envStr("WEBDAV_ADDR", "0.0.0.0:6060"),

		WorkspacePath:    workspacePath,
		WorkspaceEnabled: workspaceEnabled,

		PocketBrainHome: pocketBrainHome,
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.EnableWhatsApp && c.WhatsAppAuthDir == "" {
		return fmt.Errorf("WHATSAPP_AUTH_DIR cannot be empty when ENABLE_WHATSAPP=true")
	}
	if c.WorkspaceEnabled && c.WorkspacePath == "" {
		return fmt.Errorf("WORKSPACE_PATH cannot be empty when WORKSPACE_ENABLED=true")
	}
	if c.HeartbeatIntervalMinutes < 1 {
		return fmt.Errorf("HEARTBEAT_INTERVAL_MINUTES must be >= 1")
	}
	return nil
}

// LoadDotEnvFile loads KEY=VALUE pairs from a dotenv file into the process
// environment only for keys that are not already set.
func LoadDotEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open dotenv: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}
		value := strings.TrimSpace(parts[1])
		if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") && len(value) >= 2 {
			value = value[1 : len(value)-1]
		}
		if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") && len(value) >= 2 {
			value = value[1 : len(value)-1]
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("setenv %s: %w", key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan dotenv: %w", err)
	}
	return nil
}

// --- helpers ---

func envStr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func resolvePath(cwd, value string) string {
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(cwd, value)
}

func parsePhoneWhitelist(values ...string) []string {
	seen := make(map[string]struct{})
	var result []string

	for _, value := range values {
		if value == "" {
			continue
		}
		for _, item := range strings.Split(value, ",") {
			var digits strings.Builder
			for _, r := range item {
				if r >= '0' && r <= '9' {
					digits.WriteRune(r)
				}
			}
			normalized := digits.String()
			if normalized == "" {
				continue
			}
			if _, ok := seen[normalized]; !ok {
				seen[normalized] = struct{}{}
				result = append(result, normalized)
			}
		}
	}
	return result
}
