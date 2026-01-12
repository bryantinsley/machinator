package components

import (
	"fmt"

	"github.com/bryantinsley/machinator/orchestrator/pkg/ui/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Dropdown is a selectable list component
type Dropdown struct {
	Label    string
	Options  []string
	Selected int
	OnChange func(selected int) tea.Cmd

	x, y          int
	width, height int
	focused       bool
	expanded      bool
}

// NewDropdown creates a new dropdown
func NewDropdown(label string, options []string, onChange func(selected int) tea.Cmd) *Dropdown {
	return &Dropdown{
		Label:    label,
		Options:  options,
		OnChange: onChange,
		Selected: 0,
	}
}

// SetFocused sets the focus state
func (d *Dropdown) SetFocused(focused bool) {
	d.focused = focused
}

// Contains checks if the point is within bounds
func (d *Dropdown) Contains(x, y int) bool {
	return x >= d.x && x < d.x+d.width && y >= d.y && y < d.y+d.height
}

// HandleClick processes clicks
func (d *Dropdown) HandleClick(x, y int) tea.Cmd {
	// Relative Y coordinate
	relY := y - d.y

	if !d.expanded {
		// Click anywhere on collapsed dropdown toggles it
		d.Toggle()
		return nil
	}

	// Expanded logic
	// Header is approx 2 lines (border + text) depending on style
	// We need to be careful about matching visual layout to logical hit testing.
	// For this iteration, we assume standard border (1 line top).

	// If clicked on header (approx top 2 lines), toggle closed
	if relY < 2 {
		d.Toggle()
		return nil
	}

	// Click on options
	// Options start after header.
	// The expanded view renders header then options.
	// Each option is 1 line.
	// Offset = relY - 2 (header height)

	optionIdx := relY - 2
	if optionIdx >= 0 && optionIdx < len(d.Options) {
		d.Selected = optionIdx
		d.expanded = false
		if d.OnChange != nil {
			return d.OnChange(d.Selected)
		}
	} else if relY >= d.height-1 {
		// Clicked bottom border or outside valid option range but inside bounds
		d.Toggle()
	}

	return nil
}

// Toggle expands or collapses the dropdown
func (d *Dropdown) Toggle() {
	d.expanded = !d.expanded
}

// Bounds returns the component's screen position
func (d *Dropdown) Bounds() (x, y, width, height int) {
	return d.x, d.y, d.width, d.height
}

// SetBounds sets the component's screen position
func (d *Dropdown) SetBounds(x, y, width, height int) {
	d.x = x
	d.y = y
	d.width = width
	d.height = height
}

// Render renders the component
func (d *Dropdown) Render() string {
	selectedText := ""
	if d.Selected >= 0 && d.Selected < len(d.Options) {
		selectedText = d.Options[d.Selected]
	} else if len(d.Options) > 0 {
		selectedText = d.Options[0]
	} else {
		selectedText = "(none)"
	}

	headerText := fmt.Sprintf("%s: %s ▼", d.Label, selectedText)
	if d.expanded {
		headerText = fmt.Sprintf("%s: %s ▲", d.Label, selectedText)
	}

	// Base style for header
	headerStyle := styles.DropdownStyle
	if d.focused {
		headerStyle = headerStyle.Copy().BorderForeground(lipgloss.Color("205"))
	}

	if !d.expanded {
		rendered := headerStyle.Render(headerText)
		d.width = lipgloss.Width(rendered)
		d.height = lipgloss.Height(rendered)
		return rendered
	}

	// Expanded View
	var options []string
	width := lipgloss.Width(headerStyle.Render(headerText)) - 2 // Subtract borders

	// Ensure width is enough for options
	for _, opt := range d.Options {
		if len(opt)+2 > width {
			width = len(opt) + 2
		}
	}

	for i, opt := range d.Options {
		style := styles.DropdownItemStyle.Copy().Width(width)
		if i == d.Selected {
			style = styles.DropdownItemSelectedStyle.Copy().Width(width)
		}
		options = append(options, style.Render(opt))
	}

	list := lipgloss.JoinVertical(lipgloss.Left, options...)
	listRendered := styles.DropdownExpandedStyle.Copy().Width(width).Render(list)

	// Join header and list
	// We want the list to appear "under" the header, but since we are flow layout,
	// we just render header then list.
	// Actually, a real dropdown replaces the header with an open version attached to the list.

	// For simplicity: Render header, then render list immediately below.

	renderedHeader := headerStyle.Render(headerText)
	fullRender := lipgloss.JoinVertical(lipgloss.Left, renderedHeader, listRendered)

	d.width = lipgloss.Width(fullRender)
	d.height = lipgloss.Height(fullRender)

	return fullRender
}
