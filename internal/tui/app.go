package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Panel identifies which panel has focus.
type Panel int

const (
	PanelMessages Panel = iota
	PanelStatus
	PanelLogs
)

// eventMsg wraps an Event for the Bubble Tea message loop.
type eventMsg Event

// tickMsg triggers periodic refresh.
type tickMsg time.Time

// Model is the root Bubble Tea model.
type Model struct {
	eventBus *EventBus
	header   headerModel
	messages messagesModel
	qr       qrModel
	logs     logsModel
	focus    Panel
	width    int
	height   int
	ready    bool

	// Status panel counters
	vaultFiles  int
	memoryCount int
	outboxCount int
	activeTasks int
}

// New creates a new TUI model.
func New(bus *EventBus) Model {
	return Model{
		eventBus: bus,
		header:   newHeaderModel(),
		messages: newMessagesModel(),
		qr:       newQRModel(),
		logs:     newLogsModel(),
		focus:    PanelMessages,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		listenForEvents(m.eventBus),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Tab):
			m.focus = (m.focus + 1) % 3
		case key.Matches(msg, keys.Messages):
			m.focus = PanelMessages
		case key.Matches(msg, keys.Logs):
			m.focus = PanelLogs
		}
		return m, nil

	case eventMsg:
		m.handleEvent(Event(msg))
		return m, listenForEvents(m.eventBus)
	}

	return m, nil
}

func (m *Model) handleEvent(e Event) {
	switch e.Type {
	case EventLog:
		if le, ok := e.Data.(LogEvent); ok {
			m.logs.addEntry(le)
			// Drive QR panel from WhatsApp pairing logs.
			switch le.Message {
			case "whatsapp QR code ready - scan with your phone":
				if raw, ok := le.Fields["qr"]; ok {
					if qrText, ok := raw.(string); ok {
						m.qr.setQR(qrText)
					}
				}
			case "whatsapp pairing successful", "whatsapp QR code timed out", "whatsapp pairing error":
				m.qr.clear()
			}
		}
	case EventMessageIn:
		if me, ok := e.Data.(MessageEvent); ok {
			m.messages.addMessage(me)
		}
	case EventMessageOut:
		if me, ok := e.Data.(MessageEvent); ok {
			m.messages.addMessage(me)
		}
	case EventWhatsAppStatus:
		if se, ok := e.Data.(StatusEvent); ok {
			m.header.whatsAppConn = se.Connected
		}
	case EventWebDAVStatus:
		if se, ok := e.Data.(StatusEvent); ok {
			m.header.webdavConn = se.Connected
		}
	case EventHeartbeatStatus:
		if se, ok := e.Data.(StatusEvent); ok {
			m.header.heartbeatInfo = se.Detail
		}
	case EventVaultStats:
		if se, ok := e.Data.(StatsEvent); ok {
			m.vaultFiles = se.Count
		}
	case EventMemoryStats:
		if se, ok := e.Data.(StatsEvent); ok {
			m.memoryCount = se.Count
		}
	case EventOutboxStats:
		if se, ok := e.Data.(StatsEvent); ok {
			m.outboxCount = se.Count
		}
	}
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing PocketBrain..."
	}

	header := m.header.View()

	// Layout calculations.
	leftW := m.width / 2
	rightW := m.width - leftW
	mainH := m.height - 4 // header + help + borders
	if mainH < 4 {
		mainH = 4
	}

	msgH := mainH * 2 / 3
	if msgH < 3 {
		msgH = 3
	}
	logH := mainH - msgH
	if logH < 3 {
		logH = 3
	}

	// Left panel: messages
	m.messages.width = leftW - 4
	m.messages.height = msgH - 2

	// Right panel: pairing QR or status.
	var statusPanel string
	if m.qr.active() {
		m.qr.width = rightW - 2
		m.qr.height = msgH - 2
		statusPanel = m.qr.View()
	} else {
		statusContent := m.renderStatusPanel(rightW-6, msgH-2)
		statusPanel = panelStyle.Width(rightW - 2).Height(msgH - 2).Render(statusContent)
	}

	topRow := lipgloss.JoinHorizontal(lipgloss.Top,
		m.messages.View(),
		statusPanel,
	)

	// Bottom panel: logs
	m.logs.width = m.width - 4
	m.logs.height = logH - 2

	help := helpStyle.Render("[q]uit  [m]essages  [v]ault  [l]ogs  [tab] focus")

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		topRow,
		m.logs.View(),
		help,
	)
}

func (m Model) renderStatusPanel(w, h int) string {
	title := panelTitleStyle.Render("Status / Vault")
	lines := []string{
		title,
		"",
		fmt.Sprintf("  Heartbeat: %s", m.header.heartbeatInfo),
		fmt.Sprintf("  Tasks:     %d active", m.activeTasks),
		fmt.Sprintf("  Memory:    %d facts", m.memoryCount),
		fmt.Sprintf("  Vault:     %d files", m.vaultFiles),
		fmt.Sprintf("  Outbox:    %d pending", m.outboxCount),
	}

	content := ""
	for i, l := range lines {
		if i >= h {
			break
		}
		content += l + "\n"
	}
	return content
}

// --- tea.Cmd helpers ---

func listenForEvents(bus *EventBus) tea.Cmd {
	return func() tea.Msg {
		e := <-bus.Subscribe()
		return eventMsg(e)
	}
}

