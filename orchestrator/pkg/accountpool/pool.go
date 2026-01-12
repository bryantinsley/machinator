package accountpool

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type quotaBucket struct {
	ModelID           string  `json:"modelId"`
	RemainingFraction float64 `json:"remainingFraction"`
	ResetTime         string  `json:"resetTime"`
}

type quotaResponse struct {
	Buckets []quotaBucket `json:"buckets"`
}

// NewPool creates a new account pool.
func NewPool() *Pool {
	return &Pool{
		accounts:  []Account{},
		exhausted: make(map[string]time.Time),
	}
}

// Add adds an account to the pool.
func (p *Pool) Add(a Account) {
	p.accounts = append(p.accounts, a)
}

// GetAccounts returns all accounts in the pool.
func (p *Pool) GetAccounts() []Account {
	return p.accounts
}

// NextAvailable returns the next available account using round-robin and quota checking.
func (p *Pool) NextAvailable() (*Account, error) {
	if len(p.accounts) == 0 {
		return nil, fmt.Errorf("no accounts in pool")
	}

	for i := 0; i < len(p.accounts); i++ {
		idx := (p.lastIdx + i) % len(p.accounts)
		acc := p.accounts[idx]

		// Skip if exhausted
		if resetTime, exists := p.exhausted[acc.Name]; exists {
			if time.Now().Before(resetTime) {
				continue
			}
			// Reset time passed, remove from exhausted
			delete(p.exhausted, acc.Name)
		}

		if p.hasQuota(acc) {
			p.lastIdx = (idx + 1) % len(p.accounts)
			return &acc, nil
		}

		// Mark as exhausted if no quota
		p.MarkExhausted(acc.Name)
	}

	return nil, fmt.Errorf("all accounts exhausted")
}

// MarkExhausted marks an account as temporarily exhausted.
func (p *Pool) MarkExhausted(name string) {
	// Default to 1 hour exhaustion if we don't know the exact reset time
	p.exhausted[name] = time.Now().Add(1 * time.Hour)
}

// ResetQuota resets the exhausted status for all accounts.
func (p *Pool) ResetQuota() {
	p.exhausted = make(map[string]time.Time)
}

func (p *Pool) hasQuota(acc Account) bool {
	machinatorDir := getMachinatorDir()
	geminiPath := filepath.Join(machinatorDir, "gemini")

	// If gemini binary doesn't exist, assume it has quota (for testing/development)
	if _, err := os.Stat(geminiPath); os.IsNotExist(err) {
		return true
	}

	cmd := exec.Command(geminiPath, "--dump-quota")
	// Set HOME to the account's home dir so it uses that account's credentials
	cmd.Env = append(os.Environ(), "HOME="+acc.HomeDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If command fails, assume it has quota to avoid blocking
		return true
	}

	var resp quotaResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return true
	}

	modelsToCheck := []string{"gemini-3-flash-preview", "gemini-3-pro-preview"}
	foundAny := false
	minPercent := 100

	for _, bucket := range resp.Buckets {
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

	if !foundAny {
		return true // Default to true if no matching models found
	}

	return minPercent > 0
}

func getMachinatorDir() string {
	if dir := os.Getenv("MACHINATOR_DIR"); dir != "" {
		return dir
	}
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".machinator")
}

// LoadAccounts loads accounts from the specified machinator directory.
func LoadAccounts(machinatorDir string) ([]Account, error) {
	accountsDir := filepath.Join(machinatorDir, "accounts")
	entries, err := os.ReadDir(accountsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Account{}, nil
		}
		return nil, err
	}

	var accounts []Account
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		accountDir := filepath.Join(accountsDir, entry.Name())
		configPath := filepath.Join(accountDir, "account.json")

		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			continue
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			continue
		}

		var acc Account
		if err := json.Unmarshal(data, &acc); err != nil {
			continue
		}

		if acc.Name == "" {
			acc.Name = entry.Name()
		}
		acc.HomeDir = accountDir

		accounts = append(accounts, acc)
	}
	return accounts, nil
}

// LoadFromDir loads accounts from the specified directory into the pool.
func (p *Pool) LoadFromDir(machinatorDir string) error {
	accounts, err := LoadAccounts(machinatorDir)
	if err != nil {
		return err
	}
	for _, acc := range accounts {
		p.Add(acc)
	}
	return nil
}
