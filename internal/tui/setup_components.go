package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// choiceModel is a reusable arrow-key navigable selector.
type choiceModel struct {
	label       string
	options     []string
	cursor      int
	selected    map[int]struct{} // for multi-select
	multi       bool
	done        bool
	width       int
}

func newChoiceModel(label string, options []string, multi bool) choiceModel {
	return choiceModel{
		label:    label,
		options:  options,
		selected: map[int]struct{}{},
		multi:    multi,
	}
}

func (m choiceModel) Init() tea.Cmd { return nil }

func (m choiceModel) Update(msg tea.Msg) (choiceModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case " ":
			if m.multi {
				if _, ok := m.selected[m.cursor]; ok {
					delete(m.selected, m.cursor)
				} else {
					m.selected[m.cursor] = struct{}{}
				}
			}
		case "enter":
			if m.multi {
				if len(m.selected) == 0 {
					m.selected[m.cursor] = struct{}{}
				}
			} else {
				m.selected = map[int]struct{}{m.cursor: {}}
			}
			m.done = true
		}
	}
	return m, nil
}

func (m choiceModel) View() string {
	var b strings.Builder
	titleStyle := lipgloss.NewStyle().Foreground(colorSecondary).Bold(true)
	b.WriteString(titleStyle.Render(m.label))
	b.WriteString("\n\n")

	for i, opt := range m.options {
		cursor := "  "
		if i == m.cursor {
			cursor = setupCursorStyle.Render("> ")
		}

		_, isSel := m.selected[i]
		line := opt
		if m.multi {
			check := "[ ]"
			if isSel {
				check = setupCheckSelectedStyle.Render("[x]")
			}
			line = fmt.Sprintf("%s %s", check, opt)
		}

		if i == m.cursor {
			line = setupHighlightStyle.Render(line)
		}

		b.WriteString(cursor + line + "\n")
	}

	if m.multi {
		b.WriteString("\n" + setupHintStyle.Render("space: toggle  enter: confirm"))
	} else {
		b.WriteString("\n" + setupHintStyle.Render("enter: select"))
	}
	return b.String()
}

// Value returns the single selected value (for single-select).
func (m choiceModel) Value() string {
	for i := range m.selected {
		if i < len(m.options) {
			return m.options[i]
		}
	}
	if len(m.options) > 0 {
		return m.options[0]
	}
	return ""
}

// Values returns all selected values (for multi-select).
func (m choiceModel) Values() []string {
	var out []string
	for i := 0; i < len(m.options); i++ {
		if _, ok := m.selected[i]; ok {
			out = append(out, m.options[i])
		}
	}
	return out
}
