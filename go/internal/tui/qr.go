package tui

// QR code display for WhatsApp pairing.
// Will be implemented in Phase 3 alongside WhatsApp adapter.

type qrModel struct {
	qrText string
	width  int
	height int
}

func newQRModel() qrModel {
	return qrModel{}
}

func (q *qrModel) setQR(text string) {
	q.qrText = text
}

func (q qrModel) View() string {
	if q.qrText == "" {
		return ""
	}
	title := panelTitleStyle.Render("Scan QR Code")
	return panelStyle.Width(q.width).Height(q.height).Render(title + "\n\n" + q.qrText)
}
