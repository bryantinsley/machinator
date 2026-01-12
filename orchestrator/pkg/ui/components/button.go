package components

import (
	"github.com/bryantinsley/machinator/orchestrator/pkg/ui/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Button is a clickable action trigger
type Button struct {
	Label   string
	OnClick func() tea.Cmd

	x, y          int
	width, height int
	focused       bool
}

// NewButton creates a new button with the given label and click handler
func NewButton(label string, onClick func() tea.Cmd) *Button {
	return &Button{
		Label:   label,
		OnClick: onClick,
	}
}

// SetFocused sets the focus state of the button
func (b *Button) SetFocused(focused bool) {
	b.focused = focused
}

// Contains returns true if the point is within the component's bounds
func (b *Button) Contains(x, y int) bool {
	return x >= b.x && x < b.x+b.width && y >= b.y && y < b.y+b.height
}

// HandleClick processes a click at the given position
func (b *Button) HandleClick(x, y int) tea.Cmd {
	if b.OnClick != nil {
		return b.OnClick()
	}
	return nil
}

// Bounds returns the component's screen position
func (b *Button) Bounds() (x, y, width, height int) {
	return b.x, b.y, b.width, b.height
}

// SetBounds sets the component's screen position
func (b *Button) SetBounds(x, y, width, height int) {
	b.x = x
	b.y = y
	b.width = width
	b.height = height
}

// Render renders the button
func (b *Button) Render() string {
	var style lipgloss.Style
	if b.focused {
		style = styles.ButtonFocusedStyle
	} else {
		style = styles.ButtonStyle
	}

	rendered := style.Render(b.Label)
	b.width = lipgloss.Width(rendered)
	b.height = lipgloss.Height(rendered)

	return rendered
}
