package quota

import (
	"bytes"
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
// Builds new data, then atomically swaps to avoid visible reload.
func (q *Quota) Refresh() error {
	accounts, err := q.discoverAccounts()
	if err != nil {
		return fmt.Errorf("discover accounts: %w", err)
	}

	// Build new list first
	var newAccounts []AccountQuota
	for _, homeDir := range accounts {
		name := filepath.Base(homeDir)
		models, err := fetchQuotaForAccount(q.MachinatorDir, homeDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: quota fetch failed for %s: %v\n", name, err)
			continue
		}

		newAccounts = append(newAccounts, AccountQuota{
			Name:    name,
			HomeDir: homeDir,
			Models:  models,
		})
	}

	// Atomic swap
	q.Accounts = newAccounts
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

func fetchQuotaForAccount(machinatorDir, homeDir string) (map[string]float64, error) {
	geminiPath := filepath.Join(machinatorDir, "gemini")

	cmd := exec.Command(geminiPath, "--dump-quota")
	cmd.Env = append(os.Environ(),
		"HOME="+homeDir,
		"GEMINI_CLI_HOME="+homeDir,
		"GEMINI_FORCE_FILE_STORAGE=true",
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gemini --dump-quota: %w", err)
	}

	// Extract JSON block (skip spurious output before/after)
	jsonBytes := extractJSON(output)
	if jsonBytes == nil {
		return nil, fmt.Errorf("no JSON found in quota output")
	}

	// Parse JSON output
	var result struct {
		Buckets []struct {
			ModelId           string  `json:"modelId"`
			RemainingFraction float64 `json:"remainingFraction"`
		} `json:"buckets"`
	}

	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil, fmt.Errorf("parse quota json: %w", err)
	}

	models := make(map[string]float64)
	for _, b := range result.Buckets {
		models[b.ModelId] = b.RemainingFraction
	}

	return models, nil
}

// extractJSON finds the first JSON object in the output
func extractJSON(data []byte) []byte {
	start := bytes.Index(data, []byte("{"))
	if start == -1 {
		return nil
	}

	// Find matching closing brace
	depth := 0
	for i := start; i < len(data); i++ {
		switch data[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return data[start : i+1]
			}
		}
	}
	return nil
}
