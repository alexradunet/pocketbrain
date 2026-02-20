package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pocketbrain/pocketbrain/internal/setup"
)

// --- Bubble Tea messages for async setup operations ---

type catalogFetchedMsg struct {
	entries []string
	err     error
}

type kronkModelResolvedMsg struct {
	modelURL string
	err      error
}

type downloadProgressMsg struct {
	line string
}

type downloadCompleteMsg struct {
	err error
}

type saveEnvMsg struct {
	err error
}

// setupCompleteMsg signals the AppModel that setup finished successfully.
type setupCompleteMsg struct{}

// progressWriter sends each Write call as a line through a channel.
type progressWriter struct {
	ch chan<- string
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	text := strings.TrimRight(string(p), "\n\r")
	if text != "" {
		select {
		case pw.ch <- text:
		default:
		}
	}
	return len(p), nil
}

// SetupModel is the Bubble Tea model for the interactive setup wizard.
type SetupModel struct {
	step       setupStep
	values     map[string]string
	envPath    string
	width      int
	height     int
	err        error
	statusText string

	// Input components
	textInput textinput.Model
	choice    choiceModel
	spinner   spinner.Model

	// Kronk catalog state
	catalogEntries []string
	selectedModels []string

	// Download progress
	progressCh   <-chan string
	doneCh       <-chan downloadCompleteMsg
	downloadLines []string
}

// NewSetupModel creates a new setup wizard model.
func NewSetupModel(envPath string) SetupModel {
	ti := textinput.New()
	ti.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorPrimary)

	return SetupModel{
		step:    stepProvider,
		values:  map[string]string{},
		envPath: envPath,
		choice: newChoiceModel("LLM Provider",
			[]string{"kronk", "anthropic", "openai", "google", "custom"}, false),
		textInput: ti,
		spinner:   sp,
	}
}

func (m SetupModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m SetupModel) Update(msg tea.Msg) (SetupModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case catalogFetchedMsg:
		if msg.err != nil || len(msg.entries) == 0 {
			m.statusText = fmt.Sprintf("Warning: unable to fetch catalog (%v). Using manual entry.", msg.err)
			m.step = stepModel
			m.initStepInput()
			return m, nil
		}
		m.catalogEntries = msg.entries
		m.step = stepKronkModelSelect
		m.choice = newChoiceModel("Select Kronk model(s) to download", msg.entries, true)
		return m, nil

	case kronkModelResolvedMsg:
		if msg.err == nil && strings.TrimSpace(msg.modelURL) != "" {
			m.values["MODEL"] = msg.modelURL
		}
		m.step = stepKronkDownloadConfirm
		m.choice = newChoiceModel("Download selected model(s) now?", []string{"Yes", "No"}, false)
		return m, nil

	case downloadProgressMsg:
		m.downloadLines = append(m.downloadLines, msg.line)
		// Keep only the last 12 lines to avoid unbounded growth
		if len(m.downloadLines) > 12 {
			m.downloadLines = m.downloadLines[len(m.downloadLines)-12:]
		}
		return m, listenForDownloadProgress(m.progressCh, m.doneCh)

	case downloadCompleteMsg:
		m.progressCh = nil
		m.doneCh = nil
		if msg.err != nil {
			m.statusText = fmt.Sprintf("Warning: download failed: %v", msg.err)
		} else {
			m.statusText = "Download complete."
		}
		m.step = stepWhatsAppEnable
		m.choice = newChoiceModel("Enable WhatsApp?", []string{"Yes", "No"}, false)
		return m, nil

	case saveEnvMsg:
		if msg.err != nil {
			m.err = msg.err
			m.statusText = fmt.Sprintf("Error saving: %v", msg.err)
			return m, nil
		}
		m.step = stepDone
		return m, func() tea.Msg { return setupCompleteMsg{} }

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m.handleKeyInput(msg)
	}

	// Forward to active input
	if m.isTextStep() {
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m SetupModel) handleKeyInput(msg tea.KeyMsg) (SetupModel, tea.Cmd) {
	switch m.step {
	case stepProvider:
		var cmd tea.Cmd
		m.choice, cmd = m.choice.Update(msg)
		if m.choice.done {
			m.values["PROVIDER"] = m.choice.Value()
			next := nextStep(m.step, m.values)
			m.step = next
			if next == stepKronkCatalog {
				m.statusText = "Fetching Kronk catalog..."
				return m, tea.Batch(m.spinner.Tick, fetchCatalogCmd)
			}
			m.initStepInput()
		}
		return m, cmd

	case stepKronkModelSelect:
		var cmd tea.Cmd
		m.choice, cmd = m.choice.Update(msg)
		if m.choice.done {
			m.selectedModels = m.choice.Values()
			if len(m.selectedModels) > 0 {
				m.values["MODEL"] = m.selectedModels[0]
				m.statusText = "Resolving model URL..."
				return m, tea.Batch(m.spinner.Tick, resolveKronkModelCmd(m.selectedModels[0]))
			}
			m.step = stepKronkDownloadConfirm
			m.choice = newChoiceModel("Download selected model(s) now?", []string{"Yes", "No"}, false)
		}
		return m, cmd

	case stepKronkDownloadConfirm:
		var cmd tea.Cmd
		m.choice, cmd = m.choice.Update(msg)
		if m.choice.done {
			if m.choice.Value() == "Yes" {
				m.values["_download_now"] = "true"
				m.step = stepKronkDownload
				m.statusText = ""
				m.downloadLines = nil
				return m, tea.Batch(m.spinner.Tick, m.startDownloadCmd())
			}
			m.values["_download_now"] = "false"
			m.step = stepWhatsAppEnable
			m.choice = newChoiceModel("Enable WhatsApp?", []string{"Yes", "No"}, false)
		}
		return m, cmd

	case stepWhatsAppEnable:
		var cmd tea.Cmd
		m.choice, cmd = m.choice.Update(msg)
		if m.choice.done {
			m.values["ENABLE_WHATSAPP"] = fmt.Sprintf("%t", m.choice.Value() == "Yes")
			m.step = nextStep(m.step, m.values)
			m.initStepInput()
		}
		return m, cmd

	case stepWebDAVEnable:
		var cmd tea.Cmd
		m.choice, cmd = m.choice.Update(msg)
		if m.choice.done {
			m.values["WEBDAV_ENABLED"] = fmt.Sprintf("%t", m.choice.Value() == "Yes")
			next := nextStep(m.step, m.values)
			m.step = next
			if next == stepSaving {
				return m, tea.Batch(m.spinner.Tick, m.saveCmd())
			}
			m.initStepInput()
		}
		return m, cmd

	default:
		// Text input steps
		if m.isTextStep() {
			if msg.String() == "enter" {
				val := strings.TrimSpace(m.textInput.Value())
				m.applyTextValue(val)
				next := nextStep(m.step, m.values)
				m.step = next
				if next == stepSaving {
					return m, tea.Batch(m.spinner.Tick, m.saveCmd())
				}
				m.initStepInput()
				return m, nil
			}
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m *SetupModel) applyTextValue(val string) {
	switch m.step {
	case stepModel:
		if val == "" {
			val = setup.DefaultModel(m.values["PROVIDER"])
		}
		m.values["MODEL"] = val
	case stepAPIKey:
		m.values["API_KEY"] = val
	case stepWhatsAppAuthDir:
		if val == "" {
			val = ".data/whatsapp-auth"
		}
		m.values["WHATSAPP_AUTH_DIR"] = val
	case stepWhatsAppNumber:
		if val != "" && !strings.HasPrefix(val, "+") {
			m.err = fmt.Errorf("phone number must start with + (international format)")
			return
		}
		if val != "" {
			m.values["WHATSAPP_WHITELIST_NUMBERS"] = val
		}
	case stepWorkspacePath:
		if val == "" {
			val = ".data/workspace"
		}
		m.values["WORKSPACE_PATH"] = val
	case stepWebDAVAddr:
		if val == "" {
			val = "0.0.0.0:6060"
		}
		m.values["WEBDAV_ADDR"] = val
	}
}

func (m *SetupModel) initStepInput() {
	m.statusText = ""
	m.err = nil

	switch m.step {
	case stepModel:
		m.textInput = textinput.New()
		m.textInput.Placeholder = setup.DefaultModel(m.values["PROVIDER"])
		m.textInput.Focus()
	case stepAPIKey:
		m.textInput = textinput.New()
		m.textInput.Placeholder = "sk-..."
		m.textInput.EchoMode = textinput.EchoPassword
		m.textInput.Focus()
	case stepWhatsAppAuthDir:
		m.textInput = textinput.New()
		m.textInput.Placeholder = ".data/whatsapp-auth"
		m.textInput.Focus()
	case stepWhatsAppNumber:
		m.textInput = textinput.New()
		m.textInput.Placeholder = "+5511987654321"
		m.textInput.Focus()
	case stepWorkspacePath:
		m.textInput = textinput.New()
		m.textInput.Placeholder = ".data/workspace"
		m.textInput.Focus()
	case stepWebDAVAddr:
		m.textInput = textinput.New()
		m.textInput.Placeholder = "0.0.0.0:6060"
		m.textInput.Focus()
	case stepWhatsAppEnable:
		m.choice = newChoiceModel("Enable WhatsApp?", []string{"Yes", "No"}, false)
	case stepWebDAVEnable:
		m.choice = newChoiceModel("Enable WebDAV workspace server?", []string{"Yes", "No"}, false)
	}
}

func (m SetupModel) isTextStep() bool {
	switch m.step {
	case stepModel, stepAPIKey, stepWhatsAppAuthDir, stepWhatsAppNumber,
		stepWorkspacePath, stepWebDAVAddr:
		return true
	}
	return false
}

func (m SetupModel) isAsyncStep() bool {
	switch m.step {
	case stepKronkCatalog, stepKronkDownload, stepSaving:
		return true
	}
	return false
}

func (m SetupModel) saveCmd() tea.Cmd {
	values := map[string]string{
		"PROVIDER":        m.values["PROVIDER"],
		"API_KEY":         m.values["API_KEY"],
		"MODEL":           m.values["MODEL"],
		"ENABLE_WHATSAPP": m.values["ENABLE_WHATSAPP"],
		"WHATSAPP_AUTH_DIR": func() string {
			if v, ok := m.values["WHATSAPP_AUTH_DIR"]; ok {
				return v
			}
			return ".data/whatsapp-auth"
		}(),
		"WORKSPACE_ENABLED": "true",
		"WORKSPACE_PATH":    m.values["WORKSPACE_PATH"],
		"WEBDAV_ENABLED":    m.values["WEBDAV_ENABLED"],
		"WEBDAV_ADDR": func() string {
			if v, ok := m.values["WEBDAV_ADDR"]; ok {
				return v
			}
			return "0.0.0.0:6060"
		}(),
		"DATA_DIR":  ".data",
		"LOG_LEVEL": "info",
	}
	if v, ok := m.values["WHATSAPP_WHITELIST_NUMBERS"]; ok && v != "" {
		values["WHATSAPP_WHITELIST_NUMBERS"] = v
	}
	envPath := m.envPath
	return func() tea.Msg {
		err := setup.PatchEnvFile(envPath, values)
		if err != nil {
			return saveEnvMsg{err: err}
		}
		return saveEnvMsg{}
	}
}

// startDownloadCmd launches the download in a goroutine and returns a cmd
// that starts listening for progress. Progress flows via channels.
func (m *SetupModel) startDownloadCmd() tea.Cmd {
	progressCh := make(chan string, 64)
	doneCh := make(chan downloadCompleteMsg, 1)
	models := m.selectedModels

	go func() {
		defer close(progressCh)
		pw := &progressWriter{ch: progressCh}
		for _, model := range models {
			progressCh <- fmt.Sprintf("Starting download: %s", model)
			if err := setup.DownloadKronkModelWithSDK(pw, model); err != nil {
				doneCh <- downloadCompleteMsg{err: fmt.Errorf("download %s: %w", model, err)}
				return
			}
		}
		doneCh <- downloadCompleteMsg{}
	}()

	m.progressCh = progressCh
	m.doneCh = doneCh

	return listenForDownloadProgress(progressCh, doneCh)
}

func (m SetupModel) View() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Foreground(colorPrimary).
		Bold(true).
		MarginBottom(1).
		Render("PocketBrain Setup Wizard")

	b.WriteString(title + "\n")
	divW := m.width - 4
	if divW < 10 {
		divW = 40
	} else if divW > 50 {
		divW = 50
	}
	b.WriteString(setupDividerStyle.Render(strings.Repeat("─", divW)) + "\n\n")

	// Progress indicator
	progress := m.renderProgress()
	b.WriteString(progress + "\n\n")

	// Step title with spinner for async steps
	if m.isAsyncStep() {
		b.WriteString(m.spinner.View() + " " + setupStepTitleStyle.Render(stepTitle(m.step)) + "\n\n")
	} else {
		b.WriteString(setupStepTitleStyle.Render(stepTitle(m.step)) + "\n\n")
	}

	// Error
	if m.err != nil {
		b.WriteString(setupErrorStyle.Render("Error: "+m.err.Error()) + "\n\n")
	}

	// Status text
	if m.statusText != "" {
		b.WriteString(setupStatusStyle.Render(m.statusText) + "\n\n")
	}

	// Step content
	switch m.step {
	case stepProvider, stepKronkModelSelect, stepKronkDownloadConfirm,
		stepWhatsAppEnable, stepWebDAVEnable:
		b.WriteString(m.choice.View())

	case stepKronkCatalog, stepSaving:
		// spinner is shown in the step title above

	case stepKronkDownload:
		// Show download log with scrolling
		if len(m.downloadLines) > 0 {
			maxLines := 10
			if m.height > 0 {
				maxLines = m.height/2 - 8
				if maxLines < 4 {
					maxLines = 4
				}
			}
			lines := m.downloadLines
			if len(lines) > maxLines {
				lines = lines[len(lines)-maxLines:]
			}
			logW := m.width - 10
			if logW < 30 {
				logW = 50
			} else if logW > 70 {
				logW = 70
			}
			for _, line := range lines {
				// Truncate long lines to fit panel
				display := line
				if len(display) > logW {
					display = display[:logW-3] + "..."
				}
				b.WriteString(setupLogLineStyle.Render(display) + "\n")
			}
		}

	case stepDone:
		b.WriteString(setupSuccessStyle.Render("Configuration saved to "+m.envPath) + "\n\n")
		if m.values["ENABLE_WHATSAPP"] == "true" {
			b.WriteString("Next: start PocketBrain and scan the QR code.\n")
		}
		b.WriteString("\n" + setupHintStyle.Render("Starting PocketBrain..."))

	default:
		// Text input
		b.WriteString(m.textInput.View() + "\n\n")
		b.WriteString(setupHintStyle.Render("enter: confirm"))
	}

	content := b.String()
	panelW := m.width - 4
	if panelW < 40 {
		panelW = 60
	} else if panelW > 60 {
		panelW = 60
	}
	return setupPanelStyle.
		Width(panelW).
		Render(content)
}

func (m SetupModel) renderProgress() string {
	steps := []setupStep{
		stepProvider, stepModel, stepWhatsAppEnable,
		stepWorkspacePath, stepWebDAVEnable, stepSaving,
	}
	labels := []string{"Provider", "Model", "WhatsApp", "Workspace", "WebDAV", "Save"}

	var parts []string
	for i, s := range steps {
		style := setupProgressInactive
		if m.step == s || (m.step > s && i < len(steps)-1 && m.step < steps[i+1]) {
			style = setupProgressActive
		} else if m.step > s {
			style = setupProgressDone
		}
		parts = append(parts, style.Render(labels[i]))
	}
	return strings.Join(parts, setupProgressSep.Render(" > "))
}

// --- Async command helpers ---

func fetchCatalogCmd() tea.Msg {
	entries, err := setup.FetchKronkCatalogModels()
	return catalogFetchedMsg{entries: entries, err: err}
}

func resolveKronkModelCmd(modelID string) tea.Cmd {
	return func() tea.Msg {
		url, err := setup.ResolveKronkModelURLWithSDK(modelID)
		return kronkModelResolvedMsg{modelURL: url, err: err}
	}
}

// listenForDownloadProgress reads one message from the progress or done channel.
// When it receives progress, the model re-subscribes; on done, the loop ends.
func listenForDownloadProgress(progressCh <-chan string, doneCh <-chan downloadCompleteMsg) tea.Cmd {
	return func() tea.Msg {
		select {
		case line, ok := <-progressCh:
			if !ok {
				// Progress channel closed, wait for completion signal
				return <-doneCh
			}
			return downloadProgressMsg{line: line}
		case done := <-doneCh:
			return done
		}
	}
}
