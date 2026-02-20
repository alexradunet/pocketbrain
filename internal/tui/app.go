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
	eventSub <-chan Event
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

	// computed layout — set in Update, read in View
	msgW    int
	msgH    int
	logW    int
	logH    int
	statusW int
	statusH int
}

// New creates a new TUI model.
func New(bus *EventBus) Model {
	var sub <-chan Event
	if bus != nil {
		sub = bus.Subscribe()
	}

	return Model{
		eventBus: bus,
		eventSub: sub,
		header:   newHeaderModel(),
		messages: newMessagesModel(),
		qr:       newQRModel(),
		logs:     newLogsModel(),
		focus:    PanelMessages,
	}
}

// SetSize sets the terminal dimensions for the model.
// Used by SSH sessions to provide initial dimensions before the program starts.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.ready = w > 0 && h > 0
	if m.ready {
		m.updateLayout()
	}
}

func (m *Model) updateLayout() {
	mode := layoutMode(m.width)
	m.header.width = m.width
	if mode == LayoutCompact {
		mainH := m.height - 4
		if mainH < 6 {
			mainH = 6
		}
		msgH := mainH * 40 / 100
		if msgH < 3 {
			msgH = 3
		}
		statusH := mainH * 30 / 100
		if statusH < 3 {
			statusH = 3
		}
		logH := mainH - msgH - statusH
		if logH < 3 {
			logH = 3
		}
		contentW := m.width - 4
		if contentW < 10 {
			contentW = 10
		}
		m.msgW = contentW
		m.msgH = msgH - 2
		m.logW = contentW
		m.logH = logH - 2
		m.statusW = contentW
		m.statusH = statusH
	} else {
		leftW, rightW := layoutColumns(m.width, mode)
		mainH := m.height - 4
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
		m.msgW = leftW - 4
		m.msgH = msgH - 2
		m.logW = m.width - 4
		m.logH = logH - 2
		m.statusW = rightW - 2
		m.statusH = msgH
	}
	m.messages.width = m.msgW
	m.messages.height = m.msgH
	m.logs.width = m.logW
	m.logs.height = m.logH
}

func (m Model) Init() tea.Cmd {
	return listenForEvents(m.eventSub)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.updateLayout()
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
		m = m.handleEvent(Event(msg))
		return m, listenForEvents(m.eventSub)
	}

	return m, nil
}

func (m Model) handleEvent(e Event) Model {
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
			case "whatsapp pairing successful":
				m.qr.setPaired()
			case "whatsapp QR code timed out":
				m.qr.setTimedOut()
			case "whatsapp pairing error":
				m.qr.setTimedOut()
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
	case EventSessionChanged:
		if se, ok := e.Data.(SessionChangedEvent); ok {
			reason := se.Reason
			if reason == "" {
				reason = "session changed"
			}
			versionText := ""
			if se.Version > 0 {
				versionText = fmt.Sprintf(" [v%d]", se.Version)
			}
			m.messages.addMessage(MessageEvent{
				UserID:    se.UserID,
				Text:      fmt.Sprintf("Context changed%s (%s) on %s.", versionText, reason, se.Channel),
				Outgoing:  true,
				Timestamp: e.Timestamp,
			})
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
	case EventSSHStatus:
		if se, ok := e.Data.(StatusEvent); ok {
			m.header.sshConn = se.Connected
		}
	case EventWebStatus:
		if se, ok := e.Data.(StatusEvent); ok {
			m.header.webConn = se.Connected
		}
	}
	return m
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing PocketBrain..."
	}

	mode := layoutMode(m.width)
	if mode == LayoutCompact {
		return m.viewCompact()
	}
	return m.viewColumns()
}

func (m Model) viewCompact() string {
	header := m.header.View()

	// Status / QR panel
	var statusPanel string
	if m.qr.active() {
		qrH := m.statusH * 2 / 3
		if qrH < 6 {
			qrH = 6
		}
		sH := m.statusH - qrH
		if sH < 2 {
			sH = 2
		}
		m.qr.width = m.statusW
		qrView := m.qr.CompactView(m.statusW, qrH-2)
		statusContent := m.renderStatusPanel(m.statusW-4, sH-2)
		statusView := panelStyle.Width(m.statusW).Height(sH - 2).Render(statusContent)
		statusPanel = lipgloss.JoinVertical(lipgloss.Left, qrView, statusView)
	} else {
		statusContent := m.renderStatusPanel(m.statusW-4, m.statusH-2)
		statusPanel = panelStyle.Width(m.statusW).Height(m.statusH - 2).Render(statusContent)
	}

	help := helpStyle.Render("[q] [m] [l] [tab]")

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		m.messages.View(),
		statusPanel,
		m.logs.View(),
		help,
	)
}

func (m Model) viewColumns() string {
	header := m.header.View()

	// Right panel: status info, with QR overlay when active.
	var statusPanel string
	if m.qr.active() {
		qrH := m.statusH * 2 / 3
		if qrH < 8 {
			qrH = 8
		}
		statusH := m.statusH - qrH
		if statusH < 3 {
			statusH = 3
		}

		m.qr.width = m.statusW
		qrView := m.qr.CompactView(m.statusW, qrH-2)
		statusContent := m.renderStatusPanel(m.statusW-4, statusH-2)
		statusView := panelStyle.Width(m.statusW).Height(statusH - 2).Render(statusContent)
		statusPanel = lipgloss.JoinVertical(lipgloss.Left, qrView, statusView)
	} else {
		statusContent := m.renderStatusPanel(m.statusW-4, m.statusH-2)
		statusPanel = panelStyle.Width(m.statusW).Height(m.statusH - 2).Render(statusContent)
	}

	topRow := lipgloss.JoinHorizontal(lipgloss.Top,
		m.messages.View(),
		statusPanel,
	)

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

func listenForEvents(sub <-chan Event) tea.Cmd {
	return func() tea.Msg {
		if sub == nil {
			return nil
		}
		e := <-sub
		return eventMsg(e)
	}
}
