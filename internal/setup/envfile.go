package setup

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ManagedKeys = []string{
	"PROVIDER",
	"API_KEY",
	"MODEL",
	"ENABLE_WHATSAPP",
	"WHATSAPP_AUTH_DIR",
	"WHATSAPP_WHITELIST_NUMBERS",
	"WORKSPACE_ENABLED",
	"WORKSPACE_PATH",
	"WEBDAV_ENABLED",
	"WEBDAV_ADDR",
	"DATA_DIR",
	"LOG_LEVEL",
}

// PatchEnvFile updates only managed keys, preserving all other lines.
func PatchEnvFile(path string, values map[string]string) error {
	existing, err := readLines(path)
	if err != nil {
		return err
	}

	indexByKey := map[string]int{}
	for i, line := range existing {
		key, ok := parseKey(line)
		if !ok {
			continue
		}
		if _, managed := values[key]; managed {
			if _, already := indexByKey[key]; !already {
				indexByKey[key] = i
			}
		}
	}

	for k, idx := range indexByKey {
		existing[idx] = formatEnvLine(k, values[k])
	}

	for _, k := range ManagedKeys {
		if _, managed := values[k]; !managed {
			continue
		}
		if _, found := indexByKey[k]; found {
			continue
		}
		existing = append(existing, formatEnvLine(k, values[k]))
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir env dir: %w", err)
	}
	content := strings.Join(existing, "\n")
	if len(existing) > 0 {
		content += "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write env: %w", err)
	}
	return nil
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open env: %w", err)
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan env: %w", err)
	}
	return lines, nil
}

func parseKey(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	parts := strings.SplitN(trimmed, "=", 2)
	if len(parts) != 2 {
		return "", false
	}
	key := strings.TrimSpace(strings.TrimPrefix(parts[0], "export "))
	if key == "" {
		return "", false
	}
	return key, true
}

func formatEnvLine(k, v string) string {
	if strings.ContainsAny(v, " #\t") {
		return fmt.Sprintf("%s=%q", k, v)
	}
	return fmt.Sprintf("%s=%s", k, v)
}
