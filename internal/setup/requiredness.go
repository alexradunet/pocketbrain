package setup

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var requiredKeys = []string{
	"PROVIDER",
	"MODEL",
	"ENABLE_WHATSAPP",
	"WORKSPACE_PATH",
	"WEBDAV_ENABLED",
}

// NeedSetup returns true when the setup wizard should run.
func NeedSetup(path string) (bool, string, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return true, "missing .env file", nil
		}
		return false, "", fmt.Errorf("stat env: %w", err)
	}

	vals, err := readEnvValues(path)
	if err != nil {
		return false, "", err
	}

	for _, k := range requiredKeys {
		if strings.TrimSpace(vals[k]) == "" {
			return true, "missing required key: " + k, nil
		}
	}

	return false, "", nil
}

func readEnvValues(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open env: %w", err)
	}
	defer f.Close()

	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.TrimPrefix(parts[0], "export "))
		val := strings.TrimSpace(parts[1])
		out[key] = strings.Trim(val, "\"'")
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan env: %w", err)
	}
	return out, nil
}
