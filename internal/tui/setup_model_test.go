package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSetupModelStartsAtProvider(t *testing.T) {
	m := NewSetupModel(".env")
	if m.step != stepProvider {
		t.Fatalf("expected stepProvider, got %d", m.step)
	}
}

func TestNextStepProviderKronk(t *testing.T) {
	values := map[string]string{"PROVIDER": "kronk"}
	next := nextStep(stepProvider, values)
	if next != stepKronkCatalog {
		t.Fatalf("expected stepKronkCatalog, got %d", next)
	}
}

func TestNextStepProviderNonKronk(t *testing.T) {
	values := map[string]string{"PROVIDER": "openai"}
	next := nextStep(stepProvider, values)
	if next != stepModel {
		t.Fatalf("expected stepModel, got %d", next)
	}
}

func TestNextStepModelToAPIKey(t *testing.T) {
	next := nextStep(stepModel, nil)
	if next != stepAPIKey {
		t.Fatalf("expected stepAPIKey, got %d", next)
	}
}

func TestNextStepAPIKeyToWhatsApp(t *testing.T) {
	next := nextStep(stepAPIKey, nil)
	if next != stepWhatsAppEnable {
		t.Fatalf("expected stepWhatsAppEnable, got %d", next)
	}
}

func TestNextStepWhatsAppEnabledGoesToAuthDir(t *testing.T) {
	values := map[string]string{"ENABLE_WHATSAPP": "true"}
	next := nextStep(stepWhatsAppEnable, values)
	if next != stepWhatsAppAuthDir {
		t.Fatalf("expected stepWhatsAppAuthDir, got %d", next)
	}
}

func TestNextStepWhatsAppDisabledGoesToWorkspace(t *testing.T) {
	values := map[string]string{"ENABLE_WHATSAPP": "false"}
	next := nextStep(stepWhatsAppEnable, values)
	if next != stepWorkspacePath {
		t.Fatalf("expected stepWorkspacePath, got %d", next)
	}
}

func TestNextStepWorkspaceToWebDAV(t *testing.T) {
	next := nextStep(stepWorkspacePath, nil)
	if next != stepWebDAVEnable {
		t.Fatalf("expected stepWebDAVEnable, got %d", next)
	}
}

func TestNextStepWebDAVEnabledGoesToAddr(t *testing.T) {
	values := map[string]string{"WEBDAV_ENABLED": "true"}
	next := nextStep(stepWebDAVEnable, values)
	if next != stepWebDAVAddr {
		t.Fatalf("expected stepWebDAVAddr, got %d", next)
	}
}

func TestNextStepWebDAVDisabledGoesToSaving(t *testing.T) {
	values := map[string]string{"WEBDAV_ENABLED": "false"}
	next := nextStep(stepWebDAVEnable, values)
	if next != stepSaving {
		t.Fatalf("expected stepSaving, got %d", next)
	}
}

func TestNextStepSavingToDone(t *testing.T) {
	next := nextStep(stepSaving, nil)
	if next != stepDone {
		t.Fatalf("expected stepDone, got %d", next)
	}
}

func TestSetupModelCatalogFetchedError(t *testing.T) {
	m := NewSetupModel(".env")
	m.step = stepKronkCatalog

	m, _ = m.Update(catalogFetchedMsg{err: errTest})
	if m.step != stepModel {
		t.Fatalf("expected fallback to stepModel on catalog error, got %d", m.step)
	}
}

func TestSetupModelCatalogFetchedSuccess(t *testing.T) {
	m := NewSetupModel(".env")
	m.step = stepKronkCatalog

	entries := []string{"model-a", "model-b"}
	m, _ = m.Update(catalogFetchedMsg{entries: entries})
	if m.step != stepKronkModelSelect {
		t.Fatalf("expected stepKronkModelSelect, got %d", m.step)
	}
	if len(m.catalogEntries) != 2 {
		t.Fatalf("expected 2 catalog entries, got %d", len(m.catalogEntries))
	}
}

func TestChoiceModelSingleSelect(t *testing.T) {
	c := newChoiceModel("Pick one", []string{"a", "b", "c"}, false)
	// Move down, select
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if c.cursor != 1 {
		t.Fatalf("expected cursor=1, got %d", c.cursor)
	}
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !c.done {
		t.Fatal("expected done=true after enter")
	}
	if c.Value() != "b" {
		t.Fatalf("expected 'b', got %q", c.Value())
	}
}

func TestChoiceModelMultiSelect(t *testing.T) {
	c := newChoiceModel("Pick many", []string{"a", "b", "c"}, true)
	// Toggle first
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	// Move down, toggle second
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	// Confirm
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !c.done {
		t.Fatal("expected done=true")
	}
	vals := c.Values()
	if len(vals) != 2 || vals[0] != "a" || vals[1] != "b" {
		t.Fatalf("expected [a b], got %v", vals)
	}
}

var errTest = fmt.Errorf("test error")
