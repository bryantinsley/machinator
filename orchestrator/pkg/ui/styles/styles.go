package styles

import "github.com/charmbracelet/lipgloss"

var (
	ButtonStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 2).
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230"))

	ButtonFocusedStyle = ButtonStyle.Copy().
				Background(lipgloss.Color("205"))

	ListItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	ListItemSelectedStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("205"))
)
