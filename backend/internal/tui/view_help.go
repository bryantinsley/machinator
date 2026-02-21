package tui

import (
	"fmt"
	"strings"
)

// buildHelpContent returns a formatted help screen with keyboard shortcuts.
func (t *TUI) buildHelpContent() string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString(" [yellow]Keyboard Shortcuts[-]\n")
	sb.WriteString(" [#333333]" + strings.Repeat("─", t.rightWidth-2) + "[-]\n\n")

	// Navigation
	sb.WriteString(" [cyan]Navigation[-]\n")
	sb.WriteString("   [white]Tab[-]         Cycle through views\n")
	sb.WriteString("   [white]j/k[-]         Scroll up/down (in scrollable views)\n")
	sb.WriteString("   [white]Enter[-]       View details of selected item\n")
	sb.WriteString("   [white]Esc[-]         Go back from detail view\n")
	sb.WriteString("   [white]?[-]           Toggle this help screen\n\n")

	// Views
	sb.WriteString(" [cyan]View Selection[-]\n")
	sb.WriteString("   [white]a[-]           Assignment Log\n")
	sb.WriteString("   [white]b[-]           Beads (Tasks)\n")
	sb.WriteString("   [white]g[-]           Git Commits\n")
	sb.WriteString("   [white]e[-]           Execution Logs\n")
	sb.WriteString("   [white]c[-]           Configuration\n")
	sb.WriteString("   [white]1-9[-]         Directly view Agent N Log\n\n")

	// Actions
	sb.WriteString(" [cyan]Actions[-]\n")
	sb.WriteString("   [white]p[-]           Pause Orchestrator\n")
	sb.WriteString("   [white]s[-]           Start/Resume Orchestrator\n")
	sb.WriteString("   [white]r[-]           Force refresh state\n")
	sb.WriteString("   [white]+/-[-]         Scale agents up/down\n")
	sb.WriteString("   [white]q[-]           Quit Machinator\n\n")

	// Footer
	sb.WriteString("\n [#333333]" + strings.Repeat("─", t.rightWidth-2) + "[-]\n")
	sb.WriteString(fmt.Sprintf(" [gray]Machinator v2  |  %s  |  Press Esc or ? to close help[-]", t.repoDir))

	return sb.String()
}
