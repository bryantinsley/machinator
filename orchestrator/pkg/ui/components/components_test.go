package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestButton_Click(t *testing.T) {
	clicked := false
	btn := NewButton("Test", func() tea.Cmd {
		clicked = true
		return nil
	})

	btn.SetBounds(10, 10, 20, 3)

	// Test click inside
	if !btn.Contains(15, 11) {
		t.Error("Button should contain point inside bounds")
	}
	btn.HandleClick(15, 11)
	if !clicked {
		t.Error("Button should have been clicked")
	}

	// Test click outside
	clicked = false
	if btn.Contains(5, 5) {
		t.Error("Button should not contain point outside bounds")
	}
	// Typically HandleClick wouldn't be called if Contains is false,
	// but the dispatcher handles that check.
}

func TestListItem_Click(t *testing.T) {
	selected := false
	item := NewListItem("Item 1", func() tea.Cmd {
		selected = true
		return nil
	})

	item.SetBounds(0, 5, 20, 1)

	// Test click inside
	if !item.Contains(10, 5) {
		t.Error("ListItem should contain point inside bounds")
	}
	item.HandleClick(10, 5)
	if !selected {
		t.Error("ListItem should have been selected")
	}
}

func TestClickDispatcher(t *testing.T) {
	btnClicked := false
	btn := NewButton("Btn", func() tea.Cmd {
		btnClicked = true
		return nil
	})
	btn.SetBounds(10, 10, 10, 3)

	listClicked := false
	list := NewListItem("List", func() tea.Cmd {
		listClicked = true
		return nil
	})
	list.SetBounds(0, 0, 10, 1)

	dispatcher := NewClickDispatcher([]Clickable{btn, list})

	// Click button
	dispatcher.HandleMouse(tea.MouseMsg{
		X:      15,
		Y:      11,
		Action: tea.MouseActionRelease,
	})

	if !btnClicked {
		t.Error("Button should have been clicked via dispatcher")
	}
	if listClicked {
		t.Error("List should NOT have been clicked")
	}

	// Reset
	btnClicked = false

	// Click list
	dispatcher.HandleMouse(tea.MouseMsg{
		X:      5,
		Y:      0,
		Action: tea.MouseActionRelease,
	})

	if btnClicked {
		t.Error("Button should NOT have been clicked")
	}
	if !listClicked {
		t.Error("List should have been clicked via dispatcher")
	}

	// Click nothing
	listClicked = false
	dispatcher.HandleMouse(tea.MouseMsg{
		X:      100,
		Y:      100,
		Action: tea.MouseActionRelease,
	})

	if btnClicked || listClicked {
		t.Error("Nothing should have been clicked")
	}
}
