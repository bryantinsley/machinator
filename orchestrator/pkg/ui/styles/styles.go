package styles

import "github.com/charmbracelet/lipgloss"

var (
	ButtonStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230"))

	ButtonFocusedStyle = ButtonStyle.Copy().
				BorderForeground(lipgloss.Color("205")).
				Foreground(lipgloss.Color("205"))

	ButtonDimmedStyle = ButtonStyle.Copy().
				BorderForeground(lipgloss.Color("240")).
				Foreground(lipgloss.Color("245"))

	ButtonActiveStyle = ButtonStyle.Copy().
				BorderForeground(lipgloss.Color("46")).
				Foreground(lipgloss.Color("46")).
				Bold(true)

	KeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	KeyDimmedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("245")).
			Background(lipgloss.Color("240")).
			Padding(0, 1)

	ListItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	ListItemSelectedStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("205"))

	// Agent Card Styles
	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")). // Brighter border
			Padding(1).
			Width(40).
			Height(6) // Reduced height - cards were too tall

	CardFocusedStyle = CardStyle.Copy().
				BorderForeground(lipgloss.Color("205"))

	StatusActiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))  // Green
	StatusIdleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("249")) // Brighter grey
	StatusErrorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red

	// Icon Styles
	IconResumeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))  // Green
	IconPauseStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("226")) // Yellow
	IconStopStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
	IconExecuteStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange/Yellow
	IconAgentStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))  // Blue
	IconQuitStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
	IconHelpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("212")) // Pink

	// Dropdown Styles
	DropdownStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1).
			Width(20)

	DropdownExpandedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(0).
				Background(lipgloss.Color("235"))

	DropdownItemStyle = lipgloss.NewStyle().
				Padding(0, 1)

	DropdownItemSelectedStyle = lipgloss.NewStyle().
					Padding(0, 1).
					Background(lipgloss.Color("62")).
					Foreground(lipgloss.Color("230"))
)
