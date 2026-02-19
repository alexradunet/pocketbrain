package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// VaultFolders maps logical folder names to vault-relative paths.
type VaultFolders struct {
	Inbox     string
	Daily     string
	Journal   string
	Projects  string
	Areas     string
	Resources string
	Archive   string
}

// Config holds all application configuration.
type Config struct {
	AppName  string
	LogLevel string
	DataDir  string

	// AI provider (replaces OpenCode)
	Provider string
	Model    string

	// Heartbeat / scheduler
	HeartbeatIntervalMinutes   int
	HeartbeatBaseDelayMs       int
	HeartbeatMaxDelayMs        int
	HeartbeatNotifyAfterFailures int

	// WhatsApp channel
	EnableWhatsApp bool
	WhatsAppAuthDir string

	// Message delivery
	MessageMaxLength    int
	MessageChunkDelayMs int
	MessageRateLimitMs  int

	// Outbox (message queue)
	OutboxIntervalMs      int
	OutboxMaxRetries      int
	OutboxRetryBaseDelayMs int

	// Connection
	ConnectionTimeoutMs      int
	ConnectionReconnectDelayMs int

	// WhatsApp pairing security
	WhitelistPairToken         string
	WhatsAppPairMaxFailures    int
	WhatsAppPairFailureWindowMs  int
	WhatsAppPairBlockDurationMs  int
	WhatsAppWhitelistNumbers   []string

	// Taildrive
	TaildriveEnabled   bool
	TaildriveShareName string
	TaildriveAutoShare bool

	// Vault / PKM
	VaultPath          string
	VaultEnabled       bool
	VaultFolders       VaultFolders
	DailyNoteFormat    string

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
	vaultEnabled := envBool("VAULT_ENABLED", true)
	vaultPathRaw := strings.TrimSpace(os.Getenv("VAULT_PATH"))

	var vaultPath string
	if vaultPathRaw != "" {
		vaultPath = resolvePath(cwd, vaultPathRaw)
	} else {
		vaultPath = filepath.Join(dataDir, "vault")
	}

	pocketBrainHome := filepath.Join(vaultPath, "99-system", "99-pocketbrain")
	if v := strings.TrimSpace(os.Getenv("POCKETBRAIN_HOME")); v != "" {
		pocketBrainHome = resolvePath(cwd, v)
	}

	waAuthDir := strings.TrimSpace(os.Getenv("WHATSAPP_AUTH_DIR"))
	if waAuthDir == "" {
		waAuthDir = filepath.Join(dataDir, "whatsapp-auth")
	} else {
		waAuthDir = resolvePath(cwd, waAuthDir)
	}

	daily := envStr("VAULT_FOLDER_DAILY", "daily")

	cfg := &Config{
		AppName:  envStr("APP_NAME", "pocketbrain"),
		LogLevel: envStr("LOG_LEVEL", "info"),
		DataDir:  dataDir,

		Provider: envStr("PROVIDER", ""),
		Model:    envStr("MODEL", ""),

		HeartbeatIntervalMinutes:     envInt("HEARTBEAT_INTERVAL_MINUTES", 30),
		HeartbeatBaseDelayMs:         60_000,
		HeartbeatMaxDelayMs:          30 * 60 * 1000,
		HeartbeatNotifyAfterFailures: 3,

		EnableWhatsApp:  envBool("ENABLE_WHATSAPP", false),
		WhatsAppAuthDir: waAuthDir,

		MessageMaxLength:    3500,
		MessageChunkDelayMs: 500,
		MessageRateLimitMs:  1000,

		OutboxIntervalMs:       60_000,
		OutboxMaxRetries:       3,
		OutboxRetryBaseDelayMs: 60_000,

		ConnectionTimeoutMs:        20_000,
		ConnectionReconnectDelayMs: 3000,

		WhitelistPairToken:          strings.TrimSpace(os.Getenv("WHITELIST_PAIR_TOKEN")),
		WhatsAppPairMaxFailures:     envInt("WHATSAPP_PAIR_MAX_FAILURES", 5),
		WhatsAppPairFailureWindowMs: envInt("WHATSAPP_PAIR_FAILURE_WINDOW_MS", 5*60*1000),
		WhatsAppPairBlockDurationMs: envInt("WHATSAPP_PAIR_BLOCK_DURATION_MS", 15*60*1000),
		WhatsAppWhitelistNumbers:    parsePhoneWhitelist(os.Getenv("WHATSAPP_WHITELIST_NUMBERS"), os.Getenv("WHATSAPP_WHITELIST_NUMBER")),

		TaildriveEnabled:   envBool("TAILDRIVE_ENABLED", false),
		TaildriveShareName: envStr("TAILDRIVE_SHARE_NAME", "vault"),
		TaildriveAutoShare: envBool("TAILDRIVE_AUTO_SHARE", true),

		VaultPath:       vaultPath,
		VaultEnabled:    vaultEnabled,
		DailyNoteFormat: envStr("DAILY_NOTE_FORMAT", "YYYY-MM-DD"),
		VaultFolders: VaultFolders{
			Inbox:     envStr("VAULT_FOLDER_INBOX", "inbox"),
			Daily:     daily,
			Journal:   daily,
			Projects:  envStr("VAULT_FOLDER_PROJECTS", "projects"),
			Areas:     envStr("VAULT_FOLDER_AREAS", "areas"),
			Resources: envStr("VAULT_FOLDER_RESOURCES", "resources"),
			Archive:   envStr("VAULT_FOLDER_ARCHIVE", "archive"),
		},

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
	if c.VaultEnabled && c.VaultPath == "" {
		return fmt.Errorf("VAULT_PATH cannot be empty when VAULT_ENABLED=true")
	}
	if c.HeartbeatIntervalMinutes < 1 {
		return fmt.Errorf("HEARTBEAT_INTERVAL_MINUTES must be >= 1")
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
