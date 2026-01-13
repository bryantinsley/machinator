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

// AccountQuota holds per-model quota information for an account
type AccountQuota struct {
	Flash int // gemini-3-flash-preview quota (0-100, -1 for error)
	Pro   int // gemini-3-pro-preview quota (0-100, -1 for error)
}

// quotaDetailMsg is sent when quota check completes with per-model data
type quotaDetailMsg map[string]AccountQuota

// checkQuota runs gemini --dump-quota for all accounts
func checkQuota(pool *accountpool.Pool) tea.Cmd {
	return func() tea.Msg {
		quotas := make(map[string]AccountQuota)
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
			return quotaDetailMsg(map[string]AccountQuota{"default": {Flash: 100, Pro: 100}})
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
					f.WriteString(fmt.Sprintf("[%s] Error checking quota for %s: %v (output: %s)\n", time.Now().Format("15:04:05"), acc.Name, err, string(output)))
				}
				// Use -1 to indicate error
				quotas[acc.Name] = AccountQuota{Flash: -1, Pro: -1}
				continue
			}

			var response QuotaResponse
			if err := json.Unmarshal(output, &response); err != nil {
				if f != nil {
					f.WriteString(fmt.Sprintf("[%s] JSON error for %s: %v (output: %s)\n", time.Now().Format("15:04:05"), acc.Name, err, string(output)))
				}
				quotas[acc.Name] = AccountQuota{Flash: -1, Pro: -1}
				continue
			}

			// Extract per-model quotas
			accQuota := AccountQuota{Flash: -1, Pro: -1} // Default to -1 (not found)

			for _, bucket := range response.Buckets {
				percent := int(bucket.RemainingFraction * 100)
				switch bucket.ModelID {
				case "gemini-3-flash-preview":
					accQuota.Flash = percent
				case "gemini-3-pro-preview":
					accQuota.Pro = percent
				}
			}

			quotas[acc.Name] = accQuota

			if f != nil {
				f.WriteString(fmt.Sprintf("[%s] Quota for %s: Flash=%d%%, Pro=%d%%\n",
					time.Now().Format("15:04:05"), acc.Name, accQuota.Flash, accQuota.Pro))
			}
		}

		if f != nil {
			f.Close()
		}
		return quotaDetailMsg(quotas)
	}
}
