package orchestrator

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTUI_StateTransitions(t *testing.T) {
	// Setup initial model
	m := initialModel(nil, false)
	m.ready = true
	m.state = StateRunning

	// Test Pause (p)
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(model)

	if m.state != StatePaused {
		t.Errorf("Expected state to be Paused after 'p', got %v", m.state)
	}

	// Test Resume (s)
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(model)

	if m.state != StateRunning {
		t.Errorf("Expected state to be Running after 's', got %v", m.state)
	}

	// Test Stop (x)
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(model)

	if m.state != StateStopped {
		t.Errorf("Expected state to be Stopped after 'x', got %v", m.state)
	}

	// Test Start from Stopped (s)
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(model)

	if m.state != StateRunning {
		t.Errorf("Expected state to be Running after 's' from Stopped, got %v", m.state)
	}
}

func TestTUI_TickRespecsState(t *testing.T) {
	// Setup initial model in Paused state
	m := initialModel(nil, false)
	m.ready = true
	m.state = StatePaused
	m.tickCount = 4 // 5-second cooldown -> next tick is 5
	m.config.CooldownPeriod = 5 * time.Second

	// Simulate tick
	m.tickCount++ // Now 5
	// We can't easily check side effects (commands) without mocking,
	// but we can check if it stays paused and doesn't crash.
	// Real logic check relies on inspection of the code:
	// "if ... && m.state == StateRunning ..."

	// Just verify state consistency
	msg := tickMsg(time.Now())
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(model)

	if m.state != StatePaused {
		t.Errorf("Tick changed state unexpectedly, got %v", m.state)
	}
}

func TestTUI_ClickHandling(t *testing.T) {
	m := initialModel(nil, false)
	m.ready = true
	m.toolsCheck.State = ToolsCheckStatePassed
	m.width = 100
	m.height = 40
	m.state = StatePaused

	// Add some tasks
	m.tasks = []Task{
		{ID: "task-1", Status: "ready"},
	}

	// Render the view to register components with dispatcher
	_ = m.View()

	// Test clicking the status bar "resume" button
	// In StatePaused, "s: resume" is the first button at (0, 39)
	msg := tea.MouseMsg{
		X:      2,
		Y:      39,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	}

	_, cmd := m.Update(msg)

	if cmd == nil {
		t.Error("Expected command when clicking resume button, got nil")
	}
}
