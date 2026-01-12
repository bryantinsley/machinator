package accountpool

import "time"

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
