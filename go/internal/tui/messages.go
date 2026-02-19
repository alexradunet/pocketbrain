package tui

import (
	"fmt"
	"strings"
	"time"
)

const maxMessages = 200

type messagesModel struct {
	messages []MessageEvent
	width    int
	height   int
	offset   int
}

func newMessagesModel() messagesModel {
	return messagesModel{}
}

func (m *messagesModel) addMessage(msg MessageEvent) {
	m.messages = append(m.messages, msg)
	if len(m.messages) > maxMessages {
		m.messages = m.messages[len(m.messages)-maxMessages:]
	}
	// Auto-scroll to bottom.
	visible := m.height - 2
	if visible < 1 {
		visible = 1
	}
	if len(m.messages) > visible {
		m.offset = len(m.messages) - visible
	}
}

func (m messagesModel) View() string {
	title := panelTitleStyle.Render("Messages")

	visible := m.height - 2
	if visible < 1 {
		visible = 1
	}

	start := m.offset
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > len(m.messages) {
		end = len(m.messages)
	}

	var lines []string
	for _, msg := range m.messages[start:end] {
		ts := msg.Timestamp.Format("15:04")
		var line string
		if msg.Outgoing {
			line = fmt.Sprintf("%s %s %s",
				logTimestamp.Render("["+ts+"]"),
				msgBot.Render("bot:"),
				truncate(msg.Text, m.width-20))
		} else {
			line = fmt.Sprintf("%s %s %s",
				logTimestamp.Render("["+ts+"]"),
				msgUser.Render("user:"),
				truncate(msg.Text, m.width-20))
		}
		lines = append(lines, line)
	}

	if len(lines) == 0 {
		lines = append(lines, logTimestamp.Render("  No messages yet"))
	}

	content := title + "\n" + strings.Join(lines, "\n")
	return panelStyle.Width(m.width).Height(m.height).Render(content)
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) > max {
		return string(r[:max-1]) + "…"
	}
	return s
}

func fmtDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
