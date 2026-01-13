package orchestrator

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTickReschedules(t *testing.T) {
	m := initialModel(nil, false)
	m.ready = true
	m.toolsCheck.State = ToolsCheckStatePassed
	m.state = StateRunning

	// Initial tick count
	initialTick := m.tickCount

	// Send tickMsg
	msg := tickMsg(time.Now())
	newModel, cmd := m.Update(msg)
	m = newModel.(model)

	// Check if tick count incremented
	if m.tickCount != initialTick+1 {
		t.Errorf("Expected tickCount to increment to %d, got %d", initialTick+1, m.tickCount)
	}

	// Check if a command was returned
	if cmd == nil {
		t.Fatal("Expected a command to be returned, got nil")
	}

	// Check if the command is a tick
	// We can't easily check the function value, but we can execute it and see if it returns another tickMsg
	resMsg := cmd()
	if _, ok := resMsg.(tickMsg); !ok {
		// It might be a BatchMsg if multiple commands were returned
		if batch, ok := resMsg.(tea.BatchMsg); ok {
			foundTick := false
			for _, c := range batch {
				if c != nil {
					m2 := c()
					if _, ok := m2.(tickMsg); ok {
						foundTick = true
						break
					}
				}
			}
			if !foundTick {
				t.Errorf("Expected BatchMsg to contain tickMsg, but it didn't")
			}
		} else {
			t.Errorf("Expected tickMsg or BatchMsg, got %T", resMsg)
		}
	}
}
