package accountpool

import (
	"testing"
)

func TestPool_Add(t *testing.T) {
	pool := NewPool()
	acc := Account{
		Name:     "default",
		AuthType: AuthTypeAPIKey,
		HomeDir:  "/tmp/fakehome/default",
	}

	pool.Add(acc)

	accounts := pool.GetAccounts()
	if len(accounts) != 1 {
		t.Errorf("expected 1 account, got %d", len(accounts))
	}

	if accounts[0].Name != "default" {
		t.Errorf("expected account name 'default', got '%s'", accounts[0].Name)
	}

	if accounts[0].HomeDir != "/tmp/fakehome/default" {
		t.Errorf("expected HomeDir '/tmp/fakehome/default', got '%s'", accounts[0].HomeDir)
	}
}
