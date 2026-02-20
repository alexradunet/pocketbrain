package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Color palette
	colorPrimary   = lipgloss.Color("#7C3AED") // Purple
	colorSecondary = lipgloss.Color("#06B6D4") // Cyan
	colorSuccess   = lipgloss.Color("#10B981") // Green
	colorWarning   = lipgloss.Color("#F59E0B") // Amber
	colorError     = lipgloss.Color("#EF4444") // Red
	colorMuted     = lipgloss.Color("#6B7280") // Gray
	colorBorder    = lipgloss.Color("#374151") // Border gray

	// Header bar style
	headerStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1F2937")).
			Foreground(lipgloss.Color("#F9FAFB")).
			Bold(true).
			Padding(0, 1)

	statusConnected = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	statusDisconnected = lipgloss.NewStyle().
				Foreground(colorError).
				Bold(true)

	// Panel styles
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	panelTitleStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true)

	// Log styles
	logTimestamp = lipgloss.NewStyle().
			Foreground(colorMuted)

	logInfo = lipgloss.NewStyle().
		Foreground(colorSuccess)

	logWarn = lipgloss.NewStyle().
		Foreground(colorWarning)

	logError = lipgloss.NewStyle().
			Foreground(colorError)

	logDebug = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Message styles
	msgUser = lipgloss.NewStyle().
		Foreground(colorSecondary)

	msgBot = lipgloss.NewStyle().
		Foreground(colorPrimary)

	// Help bar
	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Padding(0, 1)

	// Setup wizard styles
	setupPanelStyle = lipgloss.NewStyle().
			Padding(1, 2)

	setupStepTitleStyle = lipgloss.NewStyle().
				Foreground(colorSecondary).
				Bold(true)

	setupDividerStyle = lipgloss.NewStyle().
				Foreground(colorBorder)

	setupCursorStyle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true)

	setupHighlightStyle = lipgloss.NewStyle().
				Foreground(colorPrimary)

	setupCheckSelectedStyle = lipgloss.NewStyle().
				Foreground(colorSuccess)

	setupHintStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	setupErrorStyle = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)

	setupSuccessStyle = lipgloss.NewStyle().
				Foreground(colorSuccess).
				Bold(true)

	setupStatusStyle = lipgloss.NewStyle().
				Foreground(colorWarning)

	setupSpinnerStyle = lipgloss.NewStyle().
				Foreground(colorMuted)

	setupProgressActive = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true)

	setupProgressDone = lipgloss.NewStyle().
				Foreground(colorSuccess)

	setupProgressInactive = lipgloss.NewStyle().
				Foreground(colorMuted)

	setupProgressSep = lipgloss.NewStyle().
				Foreground(colorBorder)

	// Suppress unused warnings
	_ = colorWarning
	_ = msgBot
)
