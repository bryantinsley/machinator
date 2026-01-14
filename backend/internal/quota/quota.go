package quota

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Quota holds quota information for all accounts.
type Quota struct {
	MachinatorDir string
	Accounts      []AccountQuota
	UpdatedAt     time.Time
}

// AccountQuota holds quota for a single account.
type AccountQuota struct {
	Name    string
	HomeDir string
	Models  map[string]float64 // model name -> remaining fraction (0.0 to 1.0)
}

// New creates a new Quota instance.
func New(machinatorDir string) *Quota {
	return &Quota{
		MachinatorDir: machinatorDir,
	}
}

// Refresh fetches quota for all discovered accounts.
func (q *Quota) Refresh() error {
	accounts, err := q.discoverAccounts()
	if err != nil {
		return fmt.Errorf("discover accounts: %w", err)
	}

	q.Accounts = nil
	for _, homeDir := range accounts {
		name := filepath.Base(homeDir)
		models, err := fetchQuotaForAccount(homeDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: quota fetch failed for %s: %v\n", name, err)
			continue
		}

		q.Accounts = append(q.Accounts, AccountQuota{
			Name:    name,
			HomeDir: homeDir,
			Models:  models,
		})
	}

	q.UpdatedAt = time.Now()
	return nil
}

// TotalFor returns aggregate quota across all accounts for a model.
func (q *Quota) TotalFor(model string) float64 {
	total := 0.0
	for _, acc := range q.Accounts {
		total += acc.Models[model]
	}
	return total
}

// BestAccountFor returns the account with the most quota for a model.
func (q *Quota) BestAccountFor(model string) (string, error) {
	best := ""
	bestVal := 0.0
	for _, acc := range q.Accounts {
		if v := acc.Models[model]; v > bestVal {
			best = acc.Name
			bestVal = v
		}
	}
	if best == "" {
		return "", fmt.Errorf("no account with quota for %s", model)
	}
	return best, nil
}

func (q *Quota) discoverAccounts() ([]string, error) {
	pattern := filepath.Join(q.MachinatorDir, "accounts", "*")
	dirs, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	var accounts []string
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err == nil && info.IsDir() {
			accounts = append(accounts, dir)
		}
	}
	return accounts, nil
}

func fetchQuotaForAccount(homeDir string) (map[string]float64, error) {
	cmd := exec.Command("gemini", "--dump-quota")
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"GEMINI_CLI_HOME="+homeDir,
		"GEMINI_FORCE_FILE_STORAGE=true",
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gemini --dump-quota: %w", err)
	}

	// Parse JSON output
	var result struct {
		Models map[string]struct {
			Remaining float64 `json:"remaining"`
		} `json:"models"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse quota json: %w", err)
	}

	models := make(map[string]float64)
	for name, m := range result.Models {
		models[name] = m.Remaining
	}

	return models, nil
}
