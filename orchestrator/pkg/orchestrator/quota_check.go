package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
// It reloads accounts from disk each time to pick up any changes
func checkQuota(pool *accountpool.Pool) tea.Cmd {
	return func() tea.Msg {
		quotas := make(map[string]AccountQuota)

		// Reload accounts from disk each time (not from pool cache)
		machinatorDir := setup.GetMachinatorDir()
		accounts, err := accountpool.LoadAccounts(machinatorDir)

		// Debug log
		logPath := filepath.Join(originalCwd, "machinator", "logs", "tui_debug.log")
		f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			if err != nil {
				f.WriteString(fmt.Sprintf("[%s] checkQuota() error loading accounts: %v\n", time.Now().Format("15:04:05"), err))
			} else {
				f.WriteString(fmt.Sprintf("[%s] checkQuota() started for %d accounts\n", time.Now().Format("15:04:05"), len(accounts)))
			}
		}

		// If no accounts or error, return empty
		if err != nil || len(accounts) == 0 {
			if f != nil {
				f.Close()
			}
			return quotaDetailMsg(quotas)
		}

		geminiPath := filepath.Join(machinatorDir, "gemini")

		// Check all accounts in parallel
		type quotaResult struct {
			name  string
			quota AccountQuota
		}
		resultChan := make(chan quotaResult, len(accounts))

		for _, acc := range accounts {
			go func(acc accountpool.Account) {
				cmd := exec.Command(geminiPath, "--dump-quota")
				// GEMINI_CLI_HOME tells Gemini CLI where to find its config (not HOME)
				// GEMINI_FORCE_FILE_STORAGE bypasses macOS keychain
				cmd.Env = append(os.Environ(),
					"GEMINI_CLI_HOME="+acc.HomeDir,
					"GEMINI_FORCE_FILE_STORAGE=true",
				)

				output, err := cmd.CombinedOutput()
				if err != nil {
					resultChan <- quotaResult{acc.Name, AccountQuota{Flash: -1, Pro: -1}}
					return
				}

				// Extract JSON from output - npm may print noise before the actual JSON
				outputStr := string(output)
				jsonStart := strings.Index(outputStr, "{")
				jsonEnd := strings.LastIndex(outputStr, "}")
				if jsonStart == -1 || jsonEnd == -1 || jsonEnd < jsonStart {
					resultChan <- quotaResult{acc.Name, AccountQuota{Flash: -1, Pro: -1}}
					return
				}
				jsonStr := outputStr[jsonStart : jsonEnd+1]

				var response QuotaResponse
				if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
					resultChan <- quotaResult{acc.Name, AccountQuota{Flash: -1, Pro: -1}}
					return
				}

				// Extract per-model quotas
				accQuota := AccountQuota{Flash: -1, Pro: -1}
				for _, bucket := range response.Buckets {
					percent := int(bucket.RemainingFraction * 100)
					switch bucket.ModelID {
					case "gemini-3-flash-preview":
						accQuota.Flash = percent
					case "gemini-3-pro-preview":
						accQuota.Pro = percent
					}
				}

				resultChan <- quotaResult{acc.Name, accQuota}
			}(acc)
		}

		// Collect results
		for i := 0; i < len(accounts); i++ {
			result := <-resultChan
			quotas[result.name] = result.quota
			if f != nil {
				f.WriteString(fmt.Sprintf("[%s] Quota for %s: Flash=%d%%, Pro=%d%%\n",
					time.Now().Format("15:04:05"), result.name, result.quota.Flash, result.quota.Pro))
			}
		}

		if f != nil {
			f.Close()
		}
		return quotaDetailMsg(quotas)
	}
}
