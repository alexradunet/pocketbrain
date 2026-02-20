package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

type headerModel struct {
	width          int
	whatsAppConn   bool
	webdavConn     bool
	sshConn        bool
	webConn        bool
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

	sshIcon := " SSH:"
	if h.sshConn {
		sshIcon += statusConnected.Render("●")
	} else {
		sshIcon += statusDisconnected.Render("●")
	}

	webIcon := " WEB:"
	if h.webConn {
		webIcon += statusConnected.Render("●")
	} else {
		webIcon += statusDisconnected.Render("●")
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Top, title, waIcon, wdIcon, sshIcon, webIcon)

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

	sshStatus := " SSH: "
	if h.sshConn {
		sshStatus += statusConnected.Render("● Listening")
	} else {
		sshStatus += statusDisconnected.Render("● Off")
	}

	webStatus := " WEB: "
	if h.webConn {
		webStatus += statusConnected.Render("● Listening")
	} else {
		webStatus += statusDisconnected.Render("● Off")
	}

	hb := fmt.Sprintf(" HB: %s ", h.heartbeatInfo)

	bar := lipgloss.JoinHorizontal(lipgloss.Top, title, waStatus, wdStatus, sshStatus, webStatus, hb)

	return lipgloss.NewStyle().
		Width(h.width).
		Background(lipgloss.Color("#1F2937")).
		Render(bar)
}
