package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bryantinsley/machinator/orchestrator/pkg/setup"
	tea "github.com/charmbracelet/bubbletea"
)

type QuotaBucket struct {
	ModelID           string  `json:"modelId"`
	RemainingFraction float64 `json:"remainingFraction"`
	ResetTime         string  `json:"resetTime"`
}

type QuotaResponse struct {
	Buckets []QuotaBucket `json:"buckets"`
}

// checkQuota runs gemini --dump-quota using the managed CLI
func checkQuota() tea.Cmd {
	return func() tea.Msg {
		// Debug log
		logPath := filepath.Join(originalCwd, "machinator", "logs", "tui_debug.log")
		f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			f.WriteString(fmt.Sprintf("[%s] checkQuota() started\n", time.Now().Format("15:04:05")))
		}

		machinatorDir := setup.GetMachinatorDir()
		geminiPath := filepath.Join(machinatorDir, "gemini")

		// If debug mode, log what we're doing
		if f != nil {
			f.WriteString(fmt.Sprintf("[%s] Running: %s --dump-quota\n", time.Now().Format("15:04:05"), geminiPath))
		}

		cmd := exec.Command(geminiPath, "--dump-quota")
		output, err := cmd.CombinedOutput()
		outputStr := strings.TrimSpace(string(output))

		if f != nil {
			if err != nil {
				f.WriteString(fmt.Sprintf("[%s] Command error: %v\n", time.Now().Format("15:04:05"), err))
				// Log first 200 chars of output on error
				logOut := outputStr
				if len(logOut) > 200 {
					logOut = logOut[:200] + "..."
				}
				f.WriteString(fmt.Sprintf("[%s] Output (partial): %s\n", time.Now().Format("15:04:05"), logOut))
			}
		}

		if err != nil {
			if f != nil {
				f.Close()
			}
			// If managed CLI fails, fallback to assuming full quota but log error
			// This prevents TUI from crashing or showing error state if CLI is broken
			return quotaMsg(100)
		}

		var response QuotaResponse
		if err := json.Unmarshal([]byte(outputStr), &response); err != nil {
			if f != nil {
				f.WriteString(fmt.Sprintf("[%s] JSON parse error: %v\n", time.Now().Format("15:04:05"), err))
				f.Close()
			}
			return quotaMsg(100)
		}

		minPercent := 100
		modelsToCheck := []string{"gemini-3-flash-preview", "gemini-3-pro-preview"}

		foundAny := false
		for _, bucket := range response.Buckets {
			for _, model := range modelsToCheck {
				if bucket.ModelID == model {
					foundAny = true
					percent := int(bucket.RemainingFraction * 100)
					if percent < minPercent {
						minPercent = percent
					}
					if f != nil {
						f.WriteString(fmt.Sprintf("[%s] Model %s: %d%%\n", time.Now().Format("15:04:05"), model, percent))
					}
				}
			}
		}

		if !foundAny && f != nil {
			f.WriteString(fmt.Sprintf("[%s] Warning: No matching models found in quota response\n", time.Now().Format("15:04:05")))
		}

		if f != nil {
			f.WriteString(fmt.Sprintf("[%s] Returning quotaMsg(%d)\n", time.Now().Format("15:04:05"), minPercent))
			f.Close()
		}
		return quotaMsg(minPercent)
	}
}
