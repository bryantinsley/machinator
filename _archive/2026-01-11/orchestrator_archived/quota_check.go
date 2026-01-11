package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// checkQuota runs check_quota.sh and parses the percentage
func checkQuota() tea.Cmd {
	return func() tea.Msg {
		// Debug log
		logPath := filepath.Join(originalCwd, "machinator", "logs", "tui_debug.log")
		f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			f.WriteString(fmt.Sprintf("[%s] checkQuota() started\n", time.Now().Format("15:04:05")))
		}

		// Use originalCwd (set at startup)
		scriptPath := filepath.Join(originalCwd, "machinator", "check_quota.sh")

		if f != nil {
			f.WriteString(fmt.Sprintf("[%s] Running script: %s\n", time.Now().Format("15:04:05"), scriptPath))
		}

		cmd := exec.Command(scriptPath)
		output, err := cmd.CombinedOutput()

		outputStr := strings.TrimSpace(string(output))

		if f != nil {
			if err != nil {
				f.WriteString(fmt.Sprintf("[%s] Script error: %v\n", time.Now().Format("15:04:05"), err))
			}
			f.WriteString(fmt.Sprintf("[%s] Script output: %s\n", time.Now().Format("15:04:05"), outputStr))
		}

		// Extract percentage using regex - matches any number followed by %
		re := regexp.MustCompile(`(\d+)%`)
		matches := re.FindStringSubmatch(outputStr)

		if len(matches) > 1 {
			if percent, err := strconv.Atoi(matches[1]); err == nil && percent > 0 {
				if f != nil {
					f.WriteString(fmt.Sprintf("[%s] Returning quotaMsg(%d)\n", time.Now().Format("15:04:05"), percent))
					f.Close()
				}
				return quotaMsg(percent)
			}
		}

		// Default to 100 if can't parse
		if f != nil {
			f.WriteString(fmt.Sprintf("[%s] Returning default quotaMsg(100)\n", time.Now().Format("15:04:05")))
			f.Close()
		}
		return quotaMsg(100)
	}
}
