package accountpool

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestPool_NextAvailable_ExpiredExhaustion(t *testing.T) {
	pool := NewPool()
	acc1 := Account{Name: "acc1", HomeDir: "/tmp/acc1"}
	pool.Add(acc1)

	// Manually set an expired exhaustion
	pool.mu.Lock()
	pool.exhausted["acc1"] = time.Now().Add(-1 * time.Minute)
	pool.mu.Unlock()

	// Should be available now
	a, err := pool.NextAvailable()
	if err != nil {
		t.Fatalf("NextAvailable failed: %v", err)
	}
	if a.Name != "acc1" {
		t.Errorf("expected acc1, got %s", a.Name)
	}

	// Should have been removed from exhausted map
	pool.mu.Lock()
	_, exists := pool.exhausted["acc1"]
	pool.mu.Unlock()
	if exists {
		t.Error("expected acc1 to be removed from exhausted map")
	}
}

func TestPool_ConcurrentAccess(t *testing.T) {
	pool := NewPool()
	for i := 0; i < 10; i++ {
		pool.Add(Account{Name: "acc", HomeDir: "/tmp/acc"})
	}

	const goroutines = 20
	const iterations = 100
	done := make(chan bool)

	for i := 0; i < goroutines; i++ {
		go func() {
			for j := 0; j < iterations; j++ {
				_, _ = pool.NextAvailable()
				pool.GetAccounts()
				pool.ResetQuota()
			}
			done <- true
		}()
	}

	for i := 0; i < goroutines; i++ {
		<-done
	}
}

func TestPool_NextAvailable_Empty(t *testing.T) {
	pool := NewPool()
	_, err := pool.NextAvailable()
	if err == nil {
		t.Fatal("expected error for empty pool, got nil")
	}
}

func TestPool_HasQuota(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "machinator-hasquota-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldMachinatorDir := os.Getenv("MACHINATOR_DIR")
	os.Setenv("MACHINATOR_DIR", tmpDir)
	defer os.Setenv("MACHINATOR_DIR", oldMachinatorDir)

	geminiPath := filepath.Join(tmpDir, "gemini")

	pool := NewPool()
	acc := Account{Name: "acc1", HomeDir: "/tmp/acc1"}
	pool.Add(acc)

	t.Run("gemini missing", func(t *testing.T) {
		if !pool.hasQuota(acc) {
			t.Error("expected true when gemini binary is missing")
		}
	})

	t.Run("gemini has quota", func(t *testing.T) {
		script := `#!/bin/bash
echo '{"buckets": [{"modelId": "gemini-3-flash-preview", "remainingFraction": 0.5}]}'
`
		os.WriteFile(geminiPath, []byte(script), 0755)
		if !pool.hasQuota(acc) {
			t.Error("expected true when quota is 50%")
		}
	})

	t.Run("gemini no quota", func(t *testing.T) {
		script := `#!/bin/bash
echo '{"buckets": [{"modelId": "gemini-3-flash-preview", "remainingFraction": 0.0}]}'
`
		os.WriteFile(geminiPath, []byte(script), 0755)
		if pool.hasQuota(acc) {
			t.Error("expected false when quota is 0%")
		}
	})

	t.Run("gemini invalid output", func(t *testing.T) {
		script := `#!/bin/bash
echo 'invalid json'
`
		os.WriteFile(geminiPath, []byte(script), 0755)
		if !pool.hasQuota(acc) {
			t.Error("expected true when gemini output is invalid")
		}
	})

	t.Run("gemini command fails", func(t *testing.T) {
		script := `#!/bin/bash
exit 1
`
		os.WriteFile(geminiPath, []byte(script), 0755)
		if !pool.hasQuota(acc) {
			t.Error("expected true when gemini command fails")
		}
	})

	t.Run("gemini multiple models, one exhausted", func(t *testing.T) {
		script := `#!/bin/bash
echo '{"buckets": [{"modelId": "gemini-3-flash-preview", "remainingFraction": 0.0}, {"modelId": "gemini-3-pro-preview", "remainingFraction": 0.5}]}'
`
		os.WriteFile(geminiPath, []byte(script), 0755)
		if pool.hasQuota(acc) {
			t.Error("expected false when one model is exhausted")
		}
	})

	t.Run("gemini no matching models", func(t *testing.T) {
		script := `#!/bin/bash
echo '{"buckets": [{"modelId": "other-model", "remainingFraction": 0.0}]}'
`
		os.WriteFile(geminiPath, []byte(script), 0755)
		if !pool.hasQuota(acc) {
			t.Error("expected true when no matching models found")
		}
	})
}

func TestGetMachinatorDir(t *testing.T) {
	oldMachinatorDir := os.Getenv("MACHINATOR_DIR")
	os.Unsetenv("MACHINATOR_DIR")
	defer os.Setenv("MACHINATOR_DIR", oldMachinatorDir)

	dir := getMachinatorDir()
	if dir == "" {
		t.Error("expected non-empty machinator dir")
	}

	homeDir, _ := os.UserHomeDir()
	expected := filepath.Join(homeDir, ".machinator")
	if dir != expected {
		t.Errorf("expected %s, got %s", expected, dir)
	}
}

func TestPool_NextAvailable_ExhaustionThroughHasQuota(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "machinator-extra-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldMachinatorDir := os.Getenv("MACHINATOR_DIR")
	os.Setenv("MACHINATOR_DIR", tmpDir)
	defer os.Setenv("MACHINATOR_DIR", oldMachinatorDir)

	geminiPath := filepath.Join(tmpDir, "gemini")
	// Script that returns no quota
	script := `#!/bin/bash
echo '{"buckets": [{"modelId": "gemini-3-flash-preview", "remainingFraction": 0.0}]}'
`
	os.WriteFile(geminiPath, []byte(script), 0755)

	pool := NewPool()
	acc1 := Account{Name: "acc1", HomeDir: "/tmp/acc1"}
	pool.Add(acc1)

	_, err = pool.NextAvailable()
	if err == nil {
		t.Fatal("expected error when acc1 has no quota, got nil")
	}

	if err.Error() != "all accounts exhausted" {
		t.Errorf("expected 'all accounts exhausted', got '%v'", err)
	}

	// Verify it was marked as exhausted
	pool.mu.Lock()
	_, exists := pool.exhausted["acc1"]
	pool.mu.Unlock()
	if !exists {
		t.Error("expected acc1 to be marked as exhausted")
	}
}

func TestPool_NextAvailable_MultipleAccounts_SomeExhaustedThroughHasQuota(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "machinator-extra-test-2-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	oldMachinatorDir := os.Getenv("MACHINATOR_DIR")
	os.Setenv("MACHINATOR_DIR", tmpDir)
	defer os.Setenv("MACHINATOR_DIR", oldMachinatorDir)

	geminiPath := filepath.Join(tmpDir, "gemini")
	// Script that returns quota only for acc2
	// We need to distinguish between accounts. We can use HOME env var in the script.
	script := `#!/bin/bash
if [[ "$HOME" == "/tmp/acc1" ]]; then
  echo '{"buckets": [{"modelId": "gemini-3-flash-preview", "remainingFraction": 0.0}]}'
else
  echo '{"buckets": [{"modelId": "gemini-3-flash-preview", "remainingFraction": 0.5}]}'
fi
`
	os.WriteFile(geminiPath, []byte(script), 0755)

	pool := NewPool()
	acc1 := Account{Name: "acc1", HomeDir: "/tmp/acc1"}
	acc2 := Account{Name: "acc2", HomeDir: "/tmp/acc2"}
	pool.Add(acc1)
	pool.Add(acc2)

	// Should try acc1, find it exhausted, mark it, then return acc2
	a, err := pool.NextAvailable()
	if err != nil {
		t.Fatalf("NextAvailable failed: %v", err)
	}
	if a.Name != "acc2" {
		t.Errorf("expected acc2, got %s", a.Name)
	}

	// Verify acc1 was marked as exhausted
	pool.mu.Lock()
	_, exists := pool.exhausted["acc1"]
	pool.mu.Unlock()
	if !exists {
		t.Error("expected acc1 to be marked as exhausted")
	}
}
