package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func stubStartBackend(bus *EventBus) (func(), error) {
	return func() {}, nil
}

func TestAppModelStartsOnLoadingScreen(t *testing.T) {
	m := NewApp(".env", false, stubStartBackend)
	if m.screen != ScreenLoading {
		t.Fatalf("expected ScreenLoading, got %d", m.screen)
	}
}

func TestAppModelForceSetupGoesToSetup(t *testing.T) {
	m := NewApp(".env", true, stubStartBackend)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected non-nil cmd from Init")
	}
	msg := cmd()
	result, ok := msg.(setupCheckResultMsg)
	if !ok {
		t.Fatalf("expected setupCheckResultMsg, got %T", msg)
	}
	if !result.needed {
		t.Fatal("expected needed=true for force setup")
	}
}

func TestAppModelSetupCheckNeededTransitionsToSetup(t *testing.T) {
	m := NewApp(".env", false, stubStartBackend)
	m.width = 80
	m.height = 24

	var model tea.Model = m
	model, _ = model.Update(setupCheckResultMsg{needed: true, reason: "test"})
	app := model.(AppModel)
	if app.screen != ScreenSetup {
		t.Fatalf("expected ScreenSetup, got %d", app.screen)
	}
}

func TestAppModelSetupCheckNotNeededStartsBackend(t *testing.T) {
	m := NewApp(".env", false, stubStartBackend)
	var model tea.Model = m
	_, cmd := model.Update(setupCheckResultMsg{needed: false})
	if cmd == nil {
		t.Fatal("expected backend start cmd")
	}
}

func TestAppModelBackendStartedTransitionsToDashboard(t *testing.T) {
	m := NewApp(".env", false, stubStartBackend)
	m.width = 80
	m.height = 24
	bus := NewEventBus(16)
	cleaned := false

	var model tea.Model = m
	model, _ = model.Update(backendStartedMsg{
		bus:     bus,
		cleanup: func() { cleaned = true },
	})
	app := model.(AppModel)
	if app.screen != ScreenDashboard {
		t.Fatalf("expected ScreenDashboard, got %d", app.screen)
	}
	if app.cleanup == nil {
		t.Fatal("expected cleanup to be set")
	}
	app.cleanup()
	if !cleaned {
		t.Fatal("cleanup was not called")
	}
}

func TestAppModelBackendErrorShowsError(t *testing.T) {
	m := NewApp(".env", false, stubStartBackend)
	var model tea.Model = m
	model, _ = model.Update(backendStartedMsg{
		err: fmt.Errorf("backend failed"),
	})
	app := model.(AppModel)
	if app.err == nil {
		t.Fatal("expected error to be set")
	}
	if app.err.Error() != "backend failed" {
		t.Fatalf("expected 'backend failed', got %q", app.err.Error())
	}
}

func TestAppModelWindowSizeForwarded(t *testing.T) {
	m := NewApp(".env", false, stubStartBackend)
	var model tea.Model = m
	model, _ = model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app := model.(AppModel)
	if app.width != 120 || app.height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", app.width, app.height)
	}
}

func TestAppModelDashboardReadyAfterBackendStarted(t *testing.T) {
	m := NewApp(".env", false, stubStartBackend)
	m.width = 80
	m.height = 24
	bus := NewEventBus(16)

	var model tea.Model = m
	model, _ = model.Update(backendStartedMsg{
		bus:     bus,
		cleanup: func() {},
	})
	app := model.(AppModel)
	if !app.dashboard.ready {
		t.Fatal("expected dashboard.ready=true after backendStartedMsg with pre-set size")
	}
}

func TestAppModelDashboardNotReadyWithZeroSize(t *testing.T) {
	m := NewApp(".env", false, stubStartBackend)
	// width and height are 0 (default)
	bus := NewEventBus(16)

	var model tea.Model = m
	model, _ = model.Update(backendStartedMsg{
		bus:     bus,
		cleanup: func() {},
	})
	app := model.(AppModel)
	if app.dashboard.ready {
		t.Fatal("expected dashboard.ready=false when width/height are zero")
	}
}
