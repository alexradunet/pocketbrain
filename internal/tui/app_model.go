package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/pocketbrain/pocketbrain/internal/config"
	"github.com/pocketbrain/pocketbrain/internal/setup"
)

// Screen identifies which top-level screen is active.
type Screen int

const (
	ScreenLoading   Screen = iota // Checking if setup is needed
	ScreenSetup                   // Setup wizard
	ScreenDashboard               // Existing dashboard
)

// setupCheckResultMsg carries the result of the NeedSetup check.
type setupCheckResultMsg struct {
	needed bool
	reason string
	err    error
}

// backendStartedMsg carries the result of starting the backend.
type backendStartedMsg struct {
	bus     *EventBus
	cleanup func()
	err     error
}

// StartBackendFunc is the signature for the function that starts backend services.
type StartBackendFunc func(bus *EventBus) (func(), error)

// AppModel is the root Bubble Tea model that routes between screens.
type AppModel struct {
	screen       Screen
	setup        SetupModel
	dashboard    Model
	eventBus     *EventBus
	envPath      string
	forceSetup   bool
	startBackend StartBackendFunc
	cleanup      func()
	width        int
	height       int
	err          error
}

// NewApp creates the root application model.
func NewApp(envPath string, forceSetup bool, startBackend StartBackendFunc) AppModel {
	return AppModel{
		screen:       ScreenLoading,
		envPath:      envPath,
		forceSetup:   forceSetup,
		startBackend: startBackend,
	}
}

func (m AppModel) Init() tea.Cmd {
	if m.forceSetup {
		return func() tea.Msg {
			return setupCheckResultMsg{needed: true, reason: "forced via --setup"}
		}
	}
	envPath := m.envPath
	return func() tea.Msg {
		need, reason, err := setup.NeedSetup(envPath)
		return setupCheckResultMsg{needed: need, reason: reason, err: err}
	}
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Forward to active child
		switch m.screen {
		case ScreenSetup:
			var cmd tea.Cmd
			m.setup, cmd = m.setup.Update(msg)
			return m, cmd
		case ScreenDashboard:
			dm, cmd := m.dashboard.Update(msg)
			m.dashboard = dm.(Model)
			return m, cmd
		}
		return m, nil

	case setupCheckResultMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		if msg.needed {
			m.screen = ScreenSetup
			m.setup = NewSetupModel(m.envPath)
			m.setup.width = m.width
			m.setup.height = m.height
			return m, m.setup.Init()
		}
		// No setup needed → start backend
		return m, m.startBackendCmd()

	case setupCompleteMsg:
		// Reload env after setup, then start backend
		_ = config.LoadDotEnvFile(m.envPath)
		return m, m.startBackendCmd()

	case backendStartedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.cleanup = msg.cleanup
		m.eventBus = msg.bus
		m.screen = ScreenDashboard
		m.dashboard = New(msg.bus)
		m.dashboard.width = m.width
		m.dashboard.height = m.height
		return m, m.dashboard.Init()

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			if m.cleanup != nil {
				m.cleanup()
			}
			return m, tea.Quit
		}
		// Forward to active child
		switch m.screen {
		case ScreenSetup:
			var cmd tea.Cmd
			m.setup, cmd = m.setup.Update(msg)
			return m, cmd
		case ScreenDashboard:
			dm, cmd := m.dashboard.Update(msg)
			m.dashboard = dm.(Model)
			return m, cmd
		}
		return m, nil

	default:
		// Forward all other messages to active child
		switch m.screen {
		case ScreenSetup:
			var cmd tea.Cmd
			m.setup, cmd = m.setup.Update(msg)
			return m, cmd
		case ScreenDashboard:
			dm, cmd := m.dashboard.Update(msg)
			m.dashboard = dm.(Model)
			return m, cmd
		}
	}

	return m, nil
}

func (m AppModel) View() string {
	if m.err != nil {
		return setupErrorStyle.Render("Error: "+m.err.Error()) + "\n\nPress Ctrl+C to exit."
	}

	switch m.screen {
	case ScreenLoading:
		return "\n  Checking configuration..."
	case ScreenSetup:
		return m.setup.View()
	case ScreenDashboard:
		return m.dashboard.View()
	}

	return ""
}

func (m AppModel) startBackendCmd() tea.Cmd {
	startFn := m.startBackend
	return func() tea.Msg {
		bus := NewEventBus(512)
		cleanup, err := startFn(bus)
		return backendStartedMsg{bus: bus, cleanup: cleanup, err: err}
	}
}
