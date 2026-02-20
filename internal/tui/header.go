package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

type headerModel struct {
	width          int
	whatsAppConn   bool
	webdavConn     bool
	heartbeatInfo  string
}

func newHeaderModel() headerModel {
	return headerModel{
		heartbeatInfo: "idle",
	}
}

func (h headerModel) View() string {
	if h.width < 60 {
		return h.viewCompact()
	}
	return h.viewFull()
}

func (h headerModel) viewCompact() string {
	title := headerStyle.Render("PocketBrain")

	waIcon := " WA:"
	if h.whatsAppConn {
		waIcon += statusConnected.Render("●")
	} else {
		waIcon += statusDisconnected.Render("●")
	}

	wdIcon := " WD:"
	if h.webdavConn {
		wdIcon += statusConnected.Render("●")
	} else {
		wdIcon += statusDisconnected.Render("●")
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Top, title, waIcon, wdIcon)

	return lipgloss.NewStyle().
		Width(h.width).
		Background(lipgloss.Color("#1F2937")).
		Render(bar)
}

func (h headerModel) viewFull() string {
	title := headerStyle.Render(" PocketBrain v1.0 ")

	waStatus := " WA: "
	if h.whatsAppConn {
		waStatus += statusConnected.Render("● Connected")
	} else {
		waStatus += statusDisconnected.Render("● Disconnected")
	}

	wdStatus := " WD: "
	if h.webdavConn {
		wdStatus += statusConnected.Render("● Online")
	} else {
		wdStatus += statusDisconnected.Render("● Offline")
	}

	hb := fmt.Sprintf(" HB: %s ", h.heartbeatInfo)

	bar := lipgloss.JoinHorizontal(lipgloss.Top, title, waStatus, wdStatus, hb)

	return lipgloss.NewStyle().
		Width(h.width).
		Background(lipgloss.Color("#1F2937")).
		Render(bar)
}
