package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/bryantinsley/machinator/orchestrator/pkg/accountpool"
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

// checkQuota runs gemini --dump-quota for all accounts
func checkQuota(pool *accountpool.Pool) tea.Cmd {
	return func() tea.Msg {
		quotas := make(map[string]int)
		accounts := pool.GetAccounts()

		// Debug log
		logPath := filepath.Join(originalCwd, "machinator", "logs", "tui_debug.log")
		f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			f.WriteString(fmt.Sprintf("[%s] checkQuota() started for %d accounts\n", time.Now().Format("15:04:05"), len(accounts)))
		}

		// If no accounts, return empty or default
		if len(accounts) == 0 {
			if f != nil {
				f.Close()
			}
			return quotaMsg(map[string]int{"default": 100})
		}

		machinatorDir := setup.GetMachinatorDir()
		geminiPath := filepath.Join(machinatorDir, "gemini")

		for _, acc := range accounts {
			cmd := exec.Command(geminiPath, "--dump-quota")
			// Set HOME to the account's home dir so it uses that account's credentials
			cmd.Env = append(os.Environ(), "HOME="+acc.HomeDir)

			output, err := cmd.CombinedOutput()
			if err != nil {
				if f != nil {
					f.WriteString(fmt.Sprintf("[%s] Error checking quota for %s: %v\n", time.Now().Format("15:04:05"), acc.Name, err))
				}
				// Assume 100 if check fails to avoid blocking (or maybe 0?)
				quotas[acc.Name] = 100
				continue
			}

			var response QuotaResponse
			if err := json.Unmarshal(output, &response); err != nil {
				if f != nil {
					f.WriteString(fmt.Sprintf("[%s] JSON error for %s: %v\n", time.Now().Format("15:04:05"), acc.Name, err))
				}
				quotas[acc.Name] = 100
				continue
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
					}
				}
			}

			if foundAny {
				quotas[acc.Name] = minPercent
			} else {
				quotas[acc.Name] = 100
			}

			if f != nil {
				f.WriteString(fmt.Sprintf("[%s] Quota for %s: %d%%\n", time.Now().Format("15:04:05"), acc.Name, quotas[acc.Name]))
			}
		}

		if f != nil {
			f.Close()
		}
		return quotaMsg(quotas)
	}
}
