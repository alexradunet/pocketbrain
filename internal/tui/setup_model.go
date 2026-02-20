package tui

import (
	"fmt"
	"io"
	"strings"

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

type downloadCompleteMsg struct {
	err error
}

type saveEnvMsg struct {
	err error
}

// setupCompleteMsg signals the AppModel that setup finished successfully.
type setupCompleteMsg struct{}

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

	// Kronk catalog state
	catalogEntries  []string
	selectedModels  []string
	downloadLog     strings.Builder
}

// NewSetupModel creates a new setup wizard model.
func NewSetupModel(envPath string) SetupModel {
	ti := textinput.New()
	ti.Focus()

	return SetupModel{
		step:    stepProvider,
		values:  map[string]string{},
		envPath: envPath,
		choice: newChoiceModel("LLM Provider",
			[]string{"kronk", "anthropic", "openai", "google", "custom"}, false),
		textInput: ti,
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
		// proceed to download confirm or next
		m.step = stepKronkDownloadConfirm
		m.choice = newChoiceModel("Download selected model(s) now?", []string{"Yes", "No"}, false)
		return m, nil

	case downloadCompleteMsg:
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
				return m, fetchCatalogCmd
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
				// Use first selected as MODEL, try to resolve URL
				m.values["MODEL"] = m.selectedModels[0]
				m.statusText = "Resolving model URL..."
				return m, resolveKronkModelCmd(m.selectedModels[0])
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
				m.statusText = "Downloading model(s)..."
				return m, downloadModelsCmd(m.selectedModels)
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
				return m, m.saveCmd()
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
					return m, m.saveCmd()
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

func (m SetupModel) saveCmd() tea.Cmd {
	values := map[string]string{
		"PROVIDER":         m.values["PROVIDER"],
		"API_KEY":          m.values["API_KEY"],
		"MODEL":            m.values["MODEL"],
		"ENABLE_WHATSAPP":  m.values["ENABLE_WHATSAPP"],
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

	// Step title
	b.WriteString(setupStepTitleStyle.Render(stepTitle(m.step)) + "\n\n")

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

	case stepKronkCatalog, stepKronkDownload, stepSaving:
		b.WriteString(setupSpinnerStyle.Render("Please wait...") + "\n")
		if m.downloadLog.Len() > 0 {
			b.WriteString("\n" + m.downloadLog.String())
		}

	case stepDone:
		b.WriteString(setupSuccessStyle.Render("Configuration saved to " + m.envPath) + "\n\n")
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

func downloadModelsCmd(models []string) tea.Cmd {
	return func() tea.Msg {
		for _, m := range models {
			if err := setup.DownloadKronkModelWithSDK(io.Discard, m); err != nil {
				return downloadCompleteMsg{err: fmt.Errorf("download %s: %w", m, err)}
			}
		}
		return downloadCompleteMsg{}
	}
}
