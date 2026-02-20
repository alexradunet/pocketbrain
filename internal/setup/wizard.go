package setup

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ardanlabs/kronk/sdk/tools/catalog"
	"golang.org/x/term"
)

const kronkCatalogURL = "https://raw.githubusercontent.com/ardanlabs/kronk_catalogs/main/CATALOG.md"

var kronkCatalogIDPattern = regexp.MustCompile(`\|\s*\[([^\]]+)\]\(`)

type Wizard struct {
	in  io.Reader
	out io.Writer

	fetchCatalog func() ([]string, error)
	download     func(io.Writer, string) error
}

func NewWizard(in io.Reader, out io.Writer) *Wizard {
	return &Wizard{
		in:           in,
		out:          out,
		fetchCatalog: fetchKronkCatalogModels,
		download:     downloadKronkModelWithSDK,
	}
}

func (w *Wizard) Run(envPath string) error {
	r := bufio.NewReader(w.in)
	fmt.Fprintln(w.out, "PocketBrain Setup Wizard")
	fmt.Fprintln(w.out, "========================")

	provider, err := w.askChoice(r, "LLM provider", []string{"kronk", "anthropic", "openai", "google", "custom"}, 0)
	if err != nil {
		return err
	}

	apiKey := ""
	model := defaultModel(provider)
	if provider == "kronk" {
		entries, err := w.fetchCatalog()
		if err != nil || len(entries) == 0 {
			fmt.Fprintf(w.out, "Warning: unable to fetch Kronk catalog (%v). Falling back to manual model entry.\n", err)
			model, err = w.askText(r, "Model", "Qwen3-8B-Q8_0")
			if err != nil {
				return err
			}
		} else {
			fmt.Fprintln(w.out, "\nKronk catalog models:")
			selected, err := w.askMultiChoice(r, "Choose model(s) to download", entries, 0)
			if err != nil {
				return err
			}
			model = selected[0]

			downloadNow, err := w.askYesNo(r, "Download selected Kronk model(s) now?", true)
			if err != nil {
				return err
			}
			if downloadNow {
				for _, m := range selected {
					fmt.Fprintf(w.out, "Downloading %s via Kronk SDK...\n", m)
					if err := w.download(w.out, m); err != nil {
						fmt.Fprintf(w.out, "Warning: failed to download %s: %v\n", m, err)
					}
				}
			}
		}
	} else {
		model, err = w.askText(r, "Model", defaultModel(provider))
		if err != nil {
			return err
		}
		apiKey, err = w.askSecret(r, "API key")
		if err != nil {
			return err
		}
	}

	enableWhatsApp, err := w.askYesNo(r, "Enable WhatsApp?", true)
	if err != nil {
		return err
	}
	waAuthDir := ".data/whatsapp-auth"
	pairToken := ""
	if enableWhatsApp {
		waAuthDir, err = w.askText(r, "WhatsApp auth dir", waAuthDir)
		if err != nil {
			return err
		}
		pairToken, err = w.askSecret(r, "WhatsApp pair token")
		if err != nil {
			return err
		}
	}

	workspacePath, err := w.askText(r, "Workspace path", ".data/workspace")
	if err != nil {
		return err
	}

	enableTailscale, err := w.askYesNo(r, "Enable embedded Tailscale (tsnet)?", true)
	if err != nil {
		return err
	}
	tsAuthKey := ""
	tsHost := "pocketbrain"
	tsStateDir := ".data/tsnet"
	if enableTailscale {
		tsAuthKey, err = w.askSecret(r, "Tailscale auth key (TS_AUTHKEY)")
		if err != nil {
			return err
		}
		tsHost, err = w.askText(r, "Tailscale hostname", tsHost)
		if err != nil {
			return err
		}
		tsStateDir, err = w.askText(r, "Tailscale state dir", tsStateDir)
		if err != nil {
			return err
		}
	}

	enableTaildrive, err := w.askYesNo(r, "Enable Taildrive workspace share?", true)
	if err != nil {
		return err
	}
	shareName := "workspace"
	autoShare := true
	if enableTaildrive {
		shareName, err = w.askText(r, "Taildrive share name", shareName)
		if err != nil {
			return err
		}
		autoShare, err = w.askYesNo(r, "Auto-create/share on startup?", true)
		if err != nil {
			return err
		}
	}

	values := map[string]string{
		"PROVIDER":             provider,
		"API_KEY":              apiKey,
		"MODEL":                model,
		"ENABLE_WHATSAPP":      fmt.Sprintf("%t", enableWhatsApp),
		"WHATSAPP_AUTH_DIR":    waAuthDir,
		"WHITELIST_PAIR_TOKEN": pairToken,
		"WORKSPACE_ENABLED":    "true",
		"WORKSPACE_PATH":       workspacePath,
		"TAILSCALE_ENABLED":    fmt.Sprintf("%t", enableTailscale),
		"TS_AUTHKEY":           tsAuthKey,
		"TS_HOSTNAME":          tsHost,
		"TS_STATE_DIR":         tsStateDir,
		"TAILDRIVE_ENABLED":    fmt.Sprintf("%t", enableTaildrive),
		"TAILDRIVE_SHARE_NAME": shareName,
		"TAILDRIVE_AUTO_SHARE": fmt.Sprintf("%t", autoShare),
		"DATA_DIR":             ".data",
		"LOG_LEVEL":            "info",
	}

	if err := PatchEnvFile(envPath, values); err != nil {
		return err
	}

	fmt.Fprintln(w.out, "\nSetup complete.")
	if enableWhatsApp {
		fmt.Fprintln(w.out, "Next steps: start PocketBrain, then use /pair <token> and scan QR code in logs/TUI.")
	}
	return nil
}

func (w *Wizard) askText(r *bufio.Reader, label, def string) (string, error) {
	fmt.Fprintf(w.out, "%s [%s]: ", label, def)
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def, nil
	}
	return line, nil
}

func (w *Wizard) askChoice(r *bufio.Reader, label string, options []string, defaultIdx int) (string, error) {
	fmt.Fprintf(w.out, "%s:\n", label)
	for i, opt := range options {
		fmt.Fprintf(w.out, "  %d) %s\n", i+1, opt)
	}
	fmt.Fprintf(w.out, "Choose [%d]: ", defaultIdx+1)
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return options[defaultIdx], nil
	}
	var idx int
	if _, err := fmt.Sscanf(line, "%d", &idx); err != nil || idx < 1 || idx > len(options) {
		return options[defaultIdx], nil
	}
	return options[idx-1], nil
}

func (w *Wizard) askMultiChoice(r *bufio.Reader, label string, options []string, defaultIdx int) ([]string, error) {
	fmt.Fprintf(w.out, "%s:\n", label)
	for i, opt := range options {
		fmt.Fprintf(w.out, "  %d) %s\n", i+1, opt)
	}
	fmt.Fprintf(w.out, "Choose numbers (comma separated) [%d]: ", defaultIdx+1)
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return []string{options[defaultIdx]}, nil
	}

	parts := strings.Split(line, ",")
	selected := make([]string, 0, len(parts))
	seen := make(map[string]struct{})
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > len(options) {
			continue
		}
		val := options[n-1]
		if _, ok := seen[val]; ok {
			continue
		}
		seen[val] = struct{}{}
		selected = append(selected, val)
	}
	if len(selected) == 0 {
		return []string{options[defaultIdx]}, nil
	}
	return selected, nil
}

func (w *Wizard) askYesNo(r *bufio.Reader, label string, def bool) (bool, error) {
	defStr := "y"
	if !def {
		defStr = "n"
	}
	fmt.Fprintf(w.out, "%s [y/n, default=%s]: ", label, defStr)
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return def, nil
	}
	return line == "y" || line == "yes" || line == "1" || line == "true", nil
}

func (w *Wizard) askSecret(r *bufio.Reader, label string) (string, error) {
	fmt.Fprintf(w.out, "%s: ", label)
	if f, ok := w.in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(w.out)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	// Non-terminal fallback (tests/pipes).
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func fetchKronkCatalogModels() ([]string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(kronkCatalogURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("catalog http status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseKronkCatalogModelIDs(body), nil
}

func parseKronkCatalogModelIDs(md []byte) []string {
	lines := bytes.Split(md, []byte{'\n'})
	seen := make(map[string]struct{})
	var ids []string
	for _, line := range lines {
		m := kronkCatalogIDPattern.FindSubmatch(line)
		if len(m) < 2 {
			continue
		}
		id := strings.TrimSpace(string(m[1]))
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func downloadKronkModelWithSDK(out io.Writer, modelID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	ctlg, err := catalog.New()
	if err != nil {
		return fmt.Errorf("catalog init: %w", err)
	}
	logf := func(_ context.Context, msg string, args ...any) {
		_, _ = fmt.Fprintf(out, msg+"\n", args...)
	}
	if err := ctlg.Download(ctx, catalog.WithLogger(logf)); err != nil {
		return fmt.Errorf("catalog update: %w", err)
	}
	if _, err := ctlg.DownloadModel(ctx, logf, modelID); err != nil {
		return fmt.Errorf("model download: %w", err)
	}
	return nil
}

func defaultModel(provider string) string {
	switch provider {
	case "anthropic":
		return "claude-sonnet-4-20250514"
	case "openai":
		return "gpt-4o"
	case "google":
		return "gemini-2.0-flash"
	case "custom":
		return "gpt-4o-mini"
	default:
		return ""
	}
}
