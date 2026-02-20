package tui

import (
	"bytes"
	"strings"

	qrterminal "github.com/mdp/qrterminal/v3"
)

// qrModel renders WhatsApp pairing QR in the right panel.
type qrModel struct {
	rawCode string
	ascii   string
	width   int
	height  int
}

func newQRModel() qrModel {
	return qrModel{}
}

func (q *qrModel) setQR(code string) {
	q.rawCode = strings.TrimSpace(code)
	if q.rawCode == "" {
		q.ascii = ""
		return
	}

	var buf bytes.Buffer
	qrterminal.GenerateHalfBlock(q.rawCode, qrterminal.L, &buf)
	q.ascii = strings.TrimRight(buf.String(), "\n")
}

func (q *qrModel) clear() {
	q.rawCode = ""
	q.ascii = ""
}

func (q qrModel) active() bool {
	return q.rawCode != ""
}

func (q qrModel) View() string {
	title := panelTitleStyle.Render("WhatsApp Pairing QR")
	content := title + "\n\n" + q.ascii
	return panelStyle.Width(q.width).Height(q.height).Render(content)
}
