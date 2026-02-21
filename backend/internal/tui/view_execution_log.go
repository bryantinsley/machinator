package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

type executionInfo struct {
	ID        string
	Timestamp time.Time
	ExitCode  int
	HasResult bool
}

func (t *TUI) handleExecutionsKey(event *tcell.EventKey) *tcell.EventKey {
	inDetailView := strings.Contains(t.logFilter, ":")

	switch event.Key() {
	case tcell.KeyUp:
		if inDetailView {
			return event
		}
		if t.selectedIdx > 0 {
			t.selectedIdx--
		}
		return nil

	case tcell.KeyDown:
		if inDetailView {
			return event
		}
		t.selectedIdx++
		return nil

	case tcell.KeyEnter:
		if !inDetailView {
			t.selectExecutionItem()
		}
		return nil

	case tcell.KeyEscape:
		if inDetailView {
			t.logFilter = "executions"
		}
		return nil
	}

	return event
}

func (t *TUI) selectExecutionItem() {
	executions := t.loadExecutions()
	if t.selectedIdx >= 0 && t.selectedIdx < len(executions) {
		t.logFilter = "executions:" + executions[t.selectedIdx].ID
	}
}

func (t *TUI) buildExecutionLogContent() string {
	if strings.HasPrefix(t.logFilter, "executions:") {
		id := strings.TrimPrefix(t.logFilter, "executions:")
		return t.buildExecutionDetailView(id)
	}

	executions := t.loadExecutions()
	if len(executions) == 0 {
		return "[gray]No executions found in ~/.machinator/logs/executions/[-]"
	}

	// Clamp selectedIdx
	if t.selectedIdx >= len(executions) {
		t.selectedIdx = len(executions) - 1
	}
	if t.selectedIdx < 0 {
		t.selectedIdx = 0
	}

	var content string
	for i, exec := range executions {
		prefix := "  "
		if i == t.selectedIdx {
			prefix = "[white::r]> "
		}

		status := "[gray]??[-]"
		if exec.HasResult {
			if exec.ExitCode == 0 {
				status = "[green]OK[-]"
			} else {
				status = fmt.Sprintf("[red]ERR(%d)[-]", exec.ExitCode)
			}
		}

		timestamp := exec.Timestamp.Format("2006-01-02 15:04:05")
		
		if i == t.selectedIdx {
			content += fmt.Sprintf("%s%-20s %s %s[-:-:-]\n", prefix, timestamp, status, exec.ID)
		} else {
			content += fmt.Sprintf("%s[white]%-20s[-] %s [gray]%s[-]\n", prefix, timestamp, status, exec.ID)
		}
	}

	return content
}

func (t *TUI) loadExecutions() []executionInfo {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	execDir := filepath.Join(home, ".machinator", "logs", "executions")
	entries, err := os.ReadDir(execDir)
	if err != nil {
		return nil
	}

	var executions []executionInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		info := executionInfo{
			ID: entry.Name(),
		}

		// Try to read result.json
		resultPath := filepath.Join(execDir, entry.Name(), "result.json")
		data, err := os.ReadFile(resultPath)
		if err == nil {
			var result struct {
				ExitCode  int       `json:"exit_code"`
				Timestamp time.Time `json:"timestamp"`
			}
			if err := json.Unmarshal(data, &result); err == nil {
				info.ExitCode = result.ExitCode
				info.Timestamp = result.Timestamp
				info.HasResult = true
			}
		}

		// If no result.json or timestamp is zero, use directory mod time
		if info.Timestamp.IsZero() {
			if fileInfo, err := entry.Info(); err == nil {
				info.Timestamp = fileInfo.ModTime()
			}
		}

		executions = append(executions, info)
	}

	// Sort by timestamp descending
	sort.Slice(executions, func(i, j int) bool {
		return executions[i].Timestamp.After(executions[j].Timestamp)
	})

	// Limit to last 20
	if len(executions) > 20 {
		executions = executions[:20]
	}

	return executions
}

func (t *TUI) buildExecutionDetailView(id string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "[red]Error: could not find home directory[-]"
	}

	logPath := filepath.Join(home, ".machinator", "logs", "executions", id, "orchestrator.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		return fmt.Sprintf("[red]Error reading log: %v[-]", err)
	}

	// Simple sanitization for tview colors if any
	content := string(data)
	content = strings.ReplaceAll(content, "[", "[[")
	
	return content
}
