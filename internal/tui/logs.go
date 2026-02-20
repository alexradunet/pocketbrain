package tui

import (
	"fmt"
	"strings"
)

const maxLogEntries = 500

type logsModel struct {
	entries []LogEvent
	width   int
	height  int
	offset  int
}

func newLogsModel() logsModel {
	return logsModel{}
}

func (l *logsModel) addEntry(entry LogEvent) {
	l.entries = append(l.entries, entry)
	if len(l.entries) > maxLogEntries {
		l.entries = l.entries[len(l.entries)-maxLogEntries:]
	}
	visible := l.height - 2
	if visible < 1 {
		visible = 1
	}
	if len(l.entries) > visible {
		l.offset = len(l.entries) - visible
	}
}

func (l logsModel) View() string {
	title := panelTitleStyle.Render("Logs")

	visible := l.height - 2
	if visible < 1 {
		visible = 1
	}

	start := l.offset
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > len(l.entries) {
		end = len(l.entries)
	}

	var lines []string
	for _, e := range l.entries[start:end] {
		var levelStr string
		switch e.Level {
		case "info", "INFO":
			levelStr = logInfo.Render("INFO ")
		case "warn", "WARN":
			levelStr = logWarn.Render("WARN ")
		case "error", "ERROR":
			levelStr = logError.Render("ERROR")
		default:
			levelStr = logDebug.Render("DEBUG")
		}
		line := fmt.Sprintf(" %s %s", levelStr, truncate(e.Message, l.width-12))
		lines = append(lines, line)
	}

	if len(lines) == 0 {
		lines = append(lines, logTimestamp.Render("  Waiting for events..."))
	}

	content := title + "\n" + strings.Join(lines, "\n")
	return panelStyle.Width(l.width).Height(l.height).Render(content)
}
