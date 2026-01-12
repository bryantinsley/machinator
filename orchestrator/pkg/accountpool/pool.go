package accountpool

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

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
