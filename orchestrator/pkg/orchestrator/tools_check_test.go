package orchestrator

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestToolsCheckModel_Update(t *testing.T) {
	m := InitialToolsCheckModel()

	// Initial state
	if m.State != ToolsCheckStateChecking {
		t.Errorf("expected state Checking, got %v", m.State)
	}

	// Simulate failure
	failMsg := toolsCheckResultMsg{err: fmt.Errorf("not found")}
	newM, cmd := m.Update(failMsg)

	if newM.State != ToolsCheckStateFailed {
		t.Errorf("expected state Failed, got %v", newM.State)
	}
	if cmd != nil {
		t.Errorf("expected no cmd on failure, got %v", cmd)
	}

	// Simulate success
	successMsg := toolsCheckResultMsg{version: "1.0.0"}
	newM, cmd = m.Update(successMsg)

	if newM.State != ToolsCheckStatePassed {
		t.Errorf("expected state Passed, got %v", newM.State)
	}
	if newM.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", newM.Version)
	}

	// Simulate install key press
	m.State = ToolsCheckStateFailed
	// Key 'i'
	newM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})

	if newM.State != ToolsCheckStateInstalling {
		t.Errorf("expected state Installing, got %v", newM.State)
	}
	if cmd == nil {
		t.Errorf("expected install cmd, got nil")
	}
}
