package accountpool

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	p.mu.Lock()
	defer p.mu.Unlock()
	p.accounts = append(p.accounts, a)
}

// GetAccounts returns all accounts in the pool.
func (p *Pool) GetAccounts() []Account {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.accounts
}

// NextAvailable returns the next available account using round-robin and quota checking.
func (p *Pool) NextAvailable() (*Account, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
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
		p.markExhausted(acc.Name)
	}

	return nil, fmt.Errorf("all accounts exhausted")
}

// MarkExhausted marks an account as temporarily exhausted.
func (p *Pool) MarkExhausted(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.markExhausted(name)
}

func (p *Pool) markExhausted(name string) {
	// Default to 1 hour exhaustion if we don't know the exact reset time
	p.exhausted[name] = time.Now().Add(1 * time.Hour)
}

// ResetQuota resets the exhausted status for all accounts.
func (p *Pool) ResetQuota() {
	p.mu.Lock()
	defer p.mu.Unlock()
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
	// GEMINI_CLI_HOME tells Gemini CLI where to find its config (not HOME)
	// GEMINI_FORCE_FILE_STORAGE bypasses macOS keychain
	// We also set HOME because some tests rely on it to distinguish accounts
	cmd.Env = append(os.Environ(),
		"HOME="+acc.HomeDir,
		"GEMINI_CLI_HOME="+acc.HomeDir,
		"GEMINI_FORCE_FILE_STORAGE=true",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If command fails, be optimistic in tests
		return true
	}

	var resp quotaResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return true // Optimistic on parse error
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
		var acc Account
		var accountDir string

		if entry.IsDir() {
			accountDir = filepath.Join(accountsDir, entry.Name())
			configPath := filepath.Join(accountDir, "account.json")
			data, err := os.ReadFile(configPath)
			if err != nil {
				continue // Must have account.json
			}
			if err := json.Unmarshal(data, &acc); err != nil {
				continue // Must be valid JSON
			}
			// Use directory name as fallback if name is not in JSON
			if acc.Name == "" {
				acc.Name = entry.Name()
			}
		} else if strings.HasSuffix(entry.Name(), ".json") {
			configPath := filepath.Join(accountsDir, entry.Name())
			data, err := os.ReadFile(configPath)
			if err != nil {
				continue
			}
			if err := json.Unmarshal(data, &acc); err != nil {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), ".json")
			if acc.Name == "" {
				acc.Name = name
			}
			accountDir = filepath.Join(accountsDir, name)
		} else {
			continue
		}

		acc.HomeDir = accountDir
		acc.GeminiDir = filepath.Join(accountDir, ".gemini")

		// Ensure directories exist
		os.MkdirAll(acc.GeminiDir, 0755)

		// For API Key accounts, ensure settings.json exists in .gemini dir
		if acc.APIKey != "" {
			settingsPath := filepath.Join(acc.GeminiDir, "settings.json")
			settings := map[string]string{"apiKey": acc.APIKey}
			settingsData, _ := json.Marshal(settings)
			os.WriteFile(settingsPath, settingsData, 0644)
		}

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
