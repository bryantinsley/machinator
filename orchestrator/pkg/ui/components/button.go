package components

import (
	"github.com/bryantinsley/machinator/orchestrator/pkg/ui/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Button is a clickable action trigger
type Button struct {
	Shortcut string
	Label    string
	OnClick  func() tea.Cmd

	x, y          int
	width, height int
	focused       bool
	Dimmed        bool
	Active        bool
}

// NewButton creates a new button with the given label and click handler
func NewButton(label string, onClick func() tea.Cmd) *Button {
	return &Button{
		Label:   label,
		OnClick: onClick,
	}
}

// NewButtonWithShortcut creates a new button with a shortcut and label
func NewButtonWithShortcut(shortcut, label string, onClick func() tea.Cmd) *Button {
	return &Button{
		Shortcut: shortcut,
		Label:    label,
		OnClick:  onClick,
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
	var keyStyle lipgloss.Style

	if b.Dimmed {
		style = styles.ButtonDimmedStyle
		keyStyle = styles.KeyDimmedStyle
	} else if b.focused {
		style = styles.ButtonFocusedStyle
		keyStyle = styles.KeyStyle // Keep key bright even if focused? Or maybe another style
	} else {
		style = styles.ButtonStyle
		keyStyle = styles.KeyStyle
	}

	if b.Active {
		// Highlight active state (e.g. "Running" mode)
		style = style.Copy().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230"))
	}

	content := b.Label
	if b.Shortcut != "" {
		key := keyStyle.Render(b.Shortcut)
		content = key + " " + b.Label
	}

	rendered := style.Render(content)
	b.width = lipgloss.Width(rendered)
	b.height = lipgloss.Height(rendered)

	return rendered
}
