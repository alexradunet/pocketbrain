package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Quit     key.Binding
	Messages key.Binding
	Vault    key.Binding
	Logs     key.Binding
	Tab      key.Binding
}

var keys = keyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Messages: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "messages"),
	),
	Vault: key.NewBinding(
		key.WithKeys("v"),
		key.WithHelp("v", "vault"),
	),
	Logs: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "logs"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "focus"),
	),
}
