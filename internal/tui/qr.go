package tui

import (
	"bytes"
	"strings"

	"github.com/charmbracelet/lipgloss"
	qrterminal "github.com/mdp/qrterminal/v3"
)

type qrStatus int

const (
	qrStatusNone    qrStatus = iota // No QR code yet
	qrStatusActive                  // QR is fresh, ready to scan
	qrStatusTimedOut                // QR timed out, still showing for convenience
	qrStatusPaired                  // Paired successfully
)

// qrModel renders WhatsApp pairing QR in a compact overlay.
type qrModel struct {
	rawCode string
	ascii   string
	status  qrStatus
	width   int
	height  int
}

func newQRModel() qrModel {
	return qrModel{}
}

func (q *qrModel) setQR(code string) {
	q.rawCode = strings.TrimSpace(code)
	if q.rawCode == "" {
		return
	}
	q.status = qrStatusActive

	var buf bytes.Buffer
	qrterminal.GenerateHalfBlock(q.rawCode, qrterminal.L, &buf)
	q.ascii = strings.TrimRight(buf.String(), "\n")
}

func (q *qrModel) setTimedOut() {
	// Keep the QR visible but mark as timed out.
	q.status = qrStatusTimedOut
}

func (q *qrModel) setPaired() {
	q.rawCode = ""
	q.ascii = ""
	q.status = qrStatusPaired
}

func (q qrModel) active() bool {
	return q.rawCode != "" && q.status != qrStatusPaired
}

func (q qrModel) View() string {
	title := panelTitleStyle.Render("WhatsApp Pairing QR")

	var statusLine string
	switch q.status {
	case qrStatusActive:
		statusLine = statusConnected.Render("Scan with your phone")
	case qrStatusTimedOut:
		statusLine = lipgloss.NewStyle().Foreground(colorWarning).Render(
			"QR timed out — restart app to refresh")
	}

	content := title + "\n" + statusLine + "\n\n" + q.ascii
	return panelStyle.Width(q.width).Height(q.height).Render(content)
}

// CompactView renders a smaller QR overlay suitable for a corner position.
func (q qrModel) CompactView(maxW, maxH int) string {
	if !q.active() {
		return ""
	}

	title := panelTitleStyle.Render("WhatsApp QR")

	var statusLine string
	switch q.status {
	case qrStatusActive:
		statusLine = statusConnected.Render("Scan now")
	case qrStatusTimedOut:
		statusLine = lipgloss.NewStyle().Foreground(colorWarning).Render("Timed out")
	}

	// Truncate QR lines to fit
	qrLines := strings.Split(q.ascii, "\n")
	if maxH > 0 {
		available := maxH - 4 // title + status + border padding
		if available < 3 {
			available = 3
		}
		if len(qrLines) > available {
			qrLines = qrLines[:available]
		}
	}

	content := title + "\n" + statusLine + "\n" + strings.Join(qrLines, "\n")

	w := maxW
	if w <= 0 {
		w = 40
	}

	return panelStyle.
		BorderForeground(colorPrimary).
		Width(w).
		Render(content)
}
