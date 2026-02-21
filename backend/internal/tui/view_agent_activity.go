package tui

import (
	"fmt"
	"strings"
)

// buildAgentActivityContent builds the agent activity view for the right pane.
// It shows the most recent 50 log entries with source icons.
func (t *TUI) buildAgentActivityContent() string {
	t.logMu.Lock()
	logs := make([]LogEntry, len(t.logs))
	copy(logs, t.logs)
	t.logMu.Unlock()

	// Most recent 50 entries
	if len(logs) > 50 {
		logs = logs[len(logs)-50:]
	}

	var content string
	for _, entry := range logs {
		icon := ""
		if strings.HasPrefix(entry.Source, "agent-") {
			icon = "🤖"
		} else {
			switch entry.Source {
			case "assign":
				icon = "⚡"
			case "setup":
				icon = "🔧"
			case "monitor":
				icon = "👁️"
			case "quota":
				icon = "📊"
			default:
				// Default icon if source doesn't match specific ones
				icon = "📝"
			}
		}

		timeStr := entry.Time.Format("15:04:05")
		// Each line should be: [gray]HH:MM:SS[-] {icon} {message}
		content += fmt.Sprintf("[gray]%s[-] %s %s\n", timeStr, icon, entry.Message)
	}

	return content
}
