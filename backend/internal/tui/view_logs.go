package tui

import "fmt"

// buildLogsView builds the filtered logs view for the right pane.
func (t *TUI) buildLogsView() string {
	var content string

	logFilter := t.logFilter

	t.logMu.Lock()
	logs := make([]LogEntry, len(t.logs))
	copy(logs, t.logs)
	t.logMu.Unlock()

	for _, entry := range logs {
		match := false
		switch logFilter {
		case "all":
			match = true
		case "assign":
			match = entry.Source == "assign" || entry.Source == "quota"
		default:
			match = entry.Source == logFilter
		}

		if match {
			timeStr := entry.Time.Format("15:04:05")
			content += fmt.Sprintf("[gray]%s[-] %s\n", timeStr, entry.Message)
		}
	}

	return content
}
