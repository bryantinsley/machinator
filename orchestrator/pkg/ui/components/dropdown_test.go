package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDropdown_Toggle(t *testing.T) {
	d := NewDropdown("Test", []string{"A", "B"}, nil)

	if d.expanded {
		t.Error("Expected dropdown to start collapsed")
	}

	d.Toggle()
	if !d.expanded {
		t.Error("Expected dropdown to be expanded after Toggle")
	}

	d.Toggle()
	if d.expanded {
		t.Error("Expected dropdown to be collapsed after second Toggle")
	}
}

func TestDropdown_Render(t *testing.T) {
	d := NewDropdown("Test", []string{"Option A", "Option B"}, nil)
	d.Selected = 0

	// Collapsed render
	out := d.Render()
	if !strings.Contains(out, "Test: Option A ▼") {
		t.Errorf("Expected collapsed render to contain label and arrow, got: %q", out)
	}

	// Expanded render
	d.Toggle()
	out = d.Render()
	if !strings.Contains(out, "Test: Option A ▲") {
		t.Errorf("Expected expanded header to contain label and arrow, got: %q", out)
	}
	if !strings.Contains(out, "Option B") {
		t.Errorf("Expected expanded render to contain options, got: %q", out)
	}
}

func TestDropdown_HandleClick(t *testing.T) {
	selected := -1
	onChange := func(idx int) tea.Cmd {
		selected = idx
		return nil
	}

	d := NewDropdown("Test", []string{"A", "B"}, onChange)
	d.x, d.y = 10, 10

	// Initial render to set dimensions (collapsed)
	d.Render()

	// 1. Click to expand
	d.HandleClick(12, 11) // Inside bounds
	if !d.expanded {
		t.Error("Click should expand dropdown")
	}

	// Re-render to update dimensions for expanded state
	d.Render()

	// 2. Click Option B (index 1)
	// Header is ~2 lines (y=10,11), List starts at y=12
	// Option A is y=12, Option B is y=13

	// Note: HandleClick expects absolute coordinates, but internally calculates relative
	// My implementation: relY := y - d.y
	// Header height assumed 2.
	// Click on Option B: y = 10 + 2 + 1 = 13.

	d.HandleClick(15, 13)

	if d.expanded {
		t.Error("Selection should collapse dropdown")
	}
	if selected != 1 {
		t.Errorf("Expected selected index 1, got %d", selected)
	}
	if d.Selected != 1 {
		t.Errorf("Expected internal selection 1, got %d", d.Selected)
	}
}
