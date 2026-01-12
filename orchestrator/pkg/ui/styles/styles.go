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

	// Agent Card Styles
	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(1).
			Width(30).
			Height(10)

	CardFocusedStyle = CardStyle.Copy().
				BorderForeground(lipgloss.Color("205"))

	StatusActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))  // Green
	StatusIdleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Grey
	StatusErrorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
)
