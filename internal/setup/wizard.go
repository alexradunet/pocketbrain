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

	fetchCatalog      func() ([]string, error)
	resolveModelValue func(string) (string, error)
	download          func(io.Writer, string) error
}

func NewWizard(in io.Reader, out io.Writer) *Wizard {
	return &Wizard{
		in:                in,
		out:               out,
		fetchCatalog:      FetchKronkCatalogModels,
		resolveModelValue: ResolveKronkModelURLWithSDK,
		download:          DownloadKronkModelWithSDK,
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
	model := DefaultModel(provider)
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
			model, err = w.resolveModelValue(selected[0])
			if err != nil || strings.TrimSpace(model) == "" {
				fmt.Fprintf(w.out, "Warning: unable to resolve Kronk model URL for %s (%v). Using model ID.\n", selected[0], err)
				model = selected[0]
			}

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
		model, err = w.askText(r, "Model", DefaultModel(provider))
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
	whitelistNumber := ""
	if enableWhatsApp {
		waAuthDir, err = w.askText(r, "WhatsApp auth dir", waAuthDir)
		if err != nil {
			return err
		}

		whitelistNumber, err = w.askText(r, "WhatsApp allowed number (e.g. +5511987654321)", "")
		if err != nil {
			return err
		}
		if whitelistNumber != "" && !strings.HasPrefix(whitelistNumber, "+") {
			return fmt.Errorf("phone number must start with + (international format)")
		}
	}

	workspacePath, err := w.askText(r, "Workspace path", ".data/workspace")
	if err != nil {
		return err
	}

	enableWebDAV, err := w.askYesNo(r, "Enable WebDAV workspace server?", true)
	if err != nil {
		return err
	}
	webdavAddr := "0.0.0.0:6060"
	if enableWebDAV {
		webdavAddr, err = w.askText(r, "WebDAV listen address", webdavAddr)
		if err != nil {
			return err
		}
	}

	values := map[string]string{
		"PROVIDER":             provider,
		"API_KEY":              apiKey,
		"MODEL":                model,
		"ENABLE_WHATSAPP":   fmt.Sprintf("%t", enableWhatsApp),
		"WHATSAPP_AUTH_DIR": waAuthDir,
		"WORKSPACE_ENABLED": "true",
		"WORKSPACE_PATH":       workspacePath,
		"WEBDAV_ENABLED": fmt.Sprintf("%t", enableWebDAV),
		"WEBDAV_ADDR":    webdavAddr,
		"DATA_DIR":             ".data",
		"LOG_LEVEL":            "info",
	}

	if whitelistNumber != "" {
		values["WHATSAPP_WHITELIST_NUMBERS"] = whitelistNumber
	}

	if err := PatchEnvFile(envPath, values); err != nil {
		return err
	}

	fmt.Fprintln(w.out, "\nSetup complete.")
	if enableWhatsApp {
		fmt.Fprintln(w.out, "Next steps: start PocketBrain and scan the QR code in logs/TUI.")
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

func FetchKronkCatalogModels() ([]string, error) {
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

func DownloadKronkModelWithSDK(out io.Writer, modelID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	ctlg, err := catalog.New()
	if err != nil {
		return fmt.Errorf("catalog init: %w", err)
	}
	logf := func(_ context.Context, msg string, args ...any) {
		if len(args) == 0 {
			_, _ = fmt.Fprintln(out, msg)
			return
		}
		_, _ = fmt.Fprint(out, msg)
		for i := 0; i < len(args); i += 2 {
			if i+1 < len(args) {
				_, _ = fmt.Fprintf(out, " %v=%v", args[i], args[i+1])
			} else {
				_, _ = fmt.Fprintf(out, " %v", args[i])
			}
		}
		_, _ = fmt.Fprintln(out)
	}
	if err := ctlg.Download(ctx, catalog.WithLogger(logf)); err != nil {
		return fmt.Errorf("catalog update: %w", err)
	}
	if _, err := ctlg.DownloadModel(ctx, logf, modelID); err != nil {
		return fmt.Errorf("model download: %w", err)
	}
	return nil
}

func ResolveKronkModelURLWithSDK(modelID string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	ctlg, err := catalog.New()
	if err != nil {
		return "", fmt.Errorf("catalog init: %w", err)
	}
	if err := ctlg.Download(ctx); err != nil {
		return "", fmt.Errorf("catalog update: %w", err)
	}
	details, err := ctlg.Details(modelID)
	if err != nil {
		return "", fmt.Errorf("catalog details: %w", err)
	}
	if len(details.Files.Models) == 0 || strings.TrimSpace(details.Files.Models[0].URL) == "" {
		return "", fmt.Errorf("catalog model has no download URL")
	}
	return details.Files.Models[0].URL, nil
}

func DefaultModel(provider string) string {
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

