package accountpool

import (
	"sync"
	"time"
)

type AuthType string

const (
	AuthTypeAPIKey AuthType = "api_key"
	AuthTypeGoogle AuthType = "google"
)

type Account struct {
	Name     string   `json:"name"`
	AuthType AuthType `json:"auth_type"`
	HomeDir  string   // Full path to fake HOME (e.g., ~/.machinator/accounts/default)
}

type Pool struct {
	mu        sync.Mutex
	accounts  []Account
	exhausted map[string]time.Time // Track exhausted accounts
	lastIdx   int                  // Last used account index for round-robin
}
