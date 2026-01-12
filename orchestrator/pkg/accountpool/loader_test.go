package accountpool

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAccounts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "machinator-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	accountsDir := filepath.Join(tmpDir, "accounts")
	if err := os.MkdirAll(accountsDir, 0755); err != nil {
		t.Fatalf("failed to create accounts dir: %v", err)
	}

	// Create a valid account
	acc1Dir := filepath.Join(accountsDir, "acc1")
	if err := os.MkdirAll(acc1Dir, 0755); err != nil {
		t.Fatalf("failed to create acc1 dir: %v", err)
	}
	acc1Config := Account{
		Name:     "Account 1",
		AuthType: AuthTypeAPIKey,
	}
	acc1Data, _ := json.Marshal(acc1Config)
	if err := os.WriteFile(filepath.Join(acc1Dir, "account.json"), acc1Data, 0644); err != nil {
		t.Fatalf("failed to write acc1 config: %v", err)
	}

	// Create an account with missing name (should use dir name)
	acc2Dir := filepath.Join(accountsDir, "acc2")
	if err := os.MkdirAll(acc2Dir, 0755); err != nil {
		t.Fatalf("failed to create acc2 dir: %v", err)
	}
	acc2Config := Account{
		AuthType: AuthTypeGoogle,
	}
	acc2Data, _ := json.Marshal(acc2Config)
	if err := os.WriteFile(filepath.Join(acc2Dir, "account.json"), acc2Data, 0644); err != nil {
		t.Fatalf("failed to write acc2 config: %v", err)
	}

	// Create an invalid account (no account.json)
	acc3Dir := filepath.Join(accountsDir, "acc3")
	if err := os.MkdirAll(acc3Dir, 0755); err != nil {
		t.Fatalf("failed to create acc3 dir: %v", err)
	}

	// Create a non-directory entry
	if err := os.WriteFile(filepath.Join(accountsDir, "notadir"), []byte("foo"), 0644); err != nil {
		t.Fatalf("failed to create notadir: %v", err)
	}

	accounts, err := LoadAccounts(tmpDir)
	if err != nil {
		t.Errorf("LoadAccounts failed: %v", err)
	}

	if len(accounts) != 2 {
		t.Errorf("expected 2 accounts, got %d", len(accounts))
	}

	foundAcc1 := false
	foundAcc2 := false

	for _, acc := range accounts {
		if acc.Name == "Account 1" {
			foundAcc1 = true
			if acc.AuthType != AuthTypeAPIKey {
				t.Errorf("acc1: expected AuthTypeAPIKey, got %s", acc.AuthType)
			}
			if acc.HomeDir != acc1Dir {
				t.Errorf("acc1: expected HomeDir %s, got %s", acc1Dir, acc.HomeDir)
			}
		} else if acc.Name == "acc2" {
			foundAcc2 = true
			if acc.AuthType != AuthTypeGoogle {
				t.Errorf("acc2: expected AuthTypeGoogle, got %s", acc.AuthType)
			}
			if acc.HomeDir != acc2Dir {
				t.Errorf("acc2: expected HomeDir %s, got %s", acc2Dir, acc.HomeDir)
			}
		}
	}

	if !foundAcc1 {
		t.Errorf("Account 1 not found")
	}
	if !foundAcc2 {
		t.Errorf("acc2 not found")
	}
}

func TestLoadAccounts_NonExistentDir(t *testing.T) {
	accounts, err := LoadAccounts("/non/existent/dir")
	if err != nil {
		t.Fatalf("expected no error for non-existent dir, got %v", err)
	}
	if len(accounts) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(accounts))
	}
}

func TestLoadAccounts_InvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "machinator-test-invalid-json-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	accountsDir := filepath.Join(tmpDir, "accounts")
	accDir := filepath.Join(accountsDir, "invalid")
	os.MkdirAll(accDir, 0755)
	os.WriteFile(filepath.Join(accDir, "account.json"), []byte("invalid json"), 0644)

	accounts, err := LoadAccounts(tmpDir)
	if err != nil {
		t.Fatalf("LoadAccounts failed: %v", err)
	}

	if len(accounts) != 0 {
		t.Errorf("expected 0 accounts due to invalid JSON, got %d", len(accounts))
	}
}

func TestLoadAccounts_UnreadableJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "machinator-test-unreadable-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	accountsDir := filepath.Join(tmpDir, "accounts")
	accDir := filepath.Join(accountsDir, "unreadable")
	os.MkdirAll(accDir, 0755)
	configPath := filepath.Join(accDir, "account.json")
	os.WriteFile(configPath, []byte("{}"), 0000) // Unreadable

	accounts, err := LoadAccounts(tmpDir)
	if err != nil {
		t.Fatalf("LoadAccounts failed: %v", err)
	}

	// It should skip the unreadable file
	if len(accounts) != 0 {
		t.Errorf("expected 0 accounts due to unreadable file, got %d", len(accounts))
	}
}

func TestPool_LoadFromDir_Error(t *testing.T) {
	pool := NewPool()
	// This will fail because LoadAccounts doesn't return error for non-existent dir anymore,
	// but let's test a case where it might fail if we could trigger one.
	// Actually, LoadAccounts only returns error if os.ReadDir fails and it's not os.IsNotExist.
	// We can try to trigger this with a file where a directory is expected.

	tmpDir, _ := os.MkdirTemp("", "machinator-test-loaderr-*")
	defer os.RemoveAll(tmpDir)
	os.WriteFile(filepath.Join(tmpDir, "accounts"), []byte("not a dir"), 0644)

	err := pool.LoadFromDir(tmpDir)
	if err == nil {
		t.Error("expected error when accounts is a file, got nil")
	}
}

func TestPool_LoadFromDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "machinator-test-pool-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	accountsDir := filepath.Join(tmpDir, "accounts")
	acc1Dir := filepath.Join(accountsDir, "acc1")
	os.MkdirAll(acc1Dir, 0755)
	acc1Config := Account{Name: "acc1", AuthType: AuthTypeAPIKey}
	acc1Data, _ := json.Marshal(acc1Config)
	os.WriteFile(filepath.Join(acc1Dir, "account.json"), acc1Data, 0644)

	pool := NewPool()
	if err := pool.LoadFromDir(tmpDir); err != nil {
		t.Errorf("LoadFromDir failed: %v", err)
	}

	if len(pool.GetAccounts()) != 1 {
		t.Errorf("expected 1 account in pool, got %d", len(pool.GetAccounts()))
	}
}
