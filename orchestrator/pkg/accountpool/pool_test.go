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

func TestPool_NextAvailable_RoundRobin(t *testing.T) {
	pool := NewPool()
	acc1 := Account{Name: "acc1", HomeDir: "/tmp/acc1"}
	acc2 := Account{Name: "acc2", HomeDir: "/tmp/acc2"}
	pool.Add(acc1)
	pool.Add(acc2)

	// First call
	a, err := pool.NextAvailable()
	if err != nil {
		t.Fatalf("NextAvailable failed: %v", err)
	}
	if a.Name != "acc1" {
		t.Errorf("expected acc1, got %s", a.Name)
	}

	// Second call
	a, err = pool.NextAvailable()
	if err != nil {
		t.Fatalf("NextAvailable failed: %v", err)
	}
	if a.Name != "acc2" {
		t.Errorf("expected acc2, got %s", a.Name)
	}

	// Third call (round-robin)
	a, err = pool.NextAvailable()
	if err != nil {
		t.Fatalf("NextAvailable failed: %v", err)
	}
	if a.Name != "acc1" {
		t.Errorf("expected acc1, got %s", a.Name)
	}
}

func TestPool_NextAvailable_Exhaustion(t *testing.T) {
	pool := NewPool()
	acc1 := Account{Name: "acc1", HomeDir: "/tmp/acc1"}
	acc2 := Account{Name: "acc2", HomeDir: "/tmp/acc2"}
	pool.Add(acc1)
	pool.Add(acc2)

	pool.MarkExhausted("acc1")

	// Should skip acc1 and return acc2
	a, err := pool.NextAvailable()
	if err != nil {
		t.Fatalf("NextAvailable failed: %v", err)
	}
	if a.Name != "acc2" {
		t.Errorf("expected acc2, got %s", a.Name)
	}

	// Next call should still skip acc1 and return acc2 (since only one available)
	a, err = pool.NextAvailable()
	if err != nil {
		t.Fatalf("NextAvailable failed: %v", err)
	}
	if a.Name != "acc2" {
		t.Errorf("expected acc2, got %s", a.Name)
	}

	pool.MarkExhausted("acc2")

	// Both exhausted
	_, err = pool.NextAvailable()
	if err == nil {
		t.Fatal("expected error when all accounts exhausted, got nil")
	}

	pool.ResetQuota()
	a, err = pool.NextAvailable()
	if err != nil {
		t.Fatalf("NextAvailable failed after ResetQuota: %v", err)
	}
}
