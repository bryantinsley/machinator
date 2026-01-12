package components

import (
	"github.com/bryantinsley/machinator/orchestrator/pkg/ui/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ListItem is a selectable item in a list
type ListItem struct {
	Label    string
	OnSelect func() tea.Cmd

	x, y          int
	width, height int
	selected      bool
}

// NewListItem creates a new list item
func NewListItem(label string, onSelect func() tea.Cmd) *ListItem {
	return &ListItem{
		Label:    label,
		OnSelect: onSelect,
	}
}

// SetSelected sets the selected state
func (l *ListItem) SetSelected(selected bool) {
	l.selected = selected
}

// Contains returns true if the point is within the component's bounds
func (l *ListItem) Contains(x, y int) bool {
	return x >= l.x && x < l.x+l.width && y >= l.y && y < l.y+l.height
}

// HandleClick processes a click at the given position
func (l *ListItem) HandleClick(x, y int) tea.Cmd {
	// If clicked, we typically want to select it
	if l.OnSelect != nil {
		return l.OnSelect()
	}
	return nil
}

// Bounds returns the component's screen position
func (l *ListItem) Bounds() (x, y, width, height int) {
	return l.x, l.y, l.width, l.height
}

// SetBounds sets the component's screen position
func (l *ListItem) SetBounds(x, y, width, height int) {
	l.x = x
	l.y = y
	l.width = width
	l.height = height
}

// Render renders the list item
func (l *ListItem) Render() string {
	var style lipgloss.Style
	if l.selected {
		style = styles.ListItemSelectedStyle
	} else {
		style = styles.ListItemStyle
	}

	rendered := style.Render(l.Label)
	l.width = lipgloss.Width(rendered)
	l.height = lipgloss.Height(rendered)

	return rendered
}
