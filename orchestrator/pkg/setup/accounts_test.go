package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitAccountsDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "machinator-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	err = InitAccountsDir(tempDir)
	if err != nil {
		t.Fatalf("InitAccountsDir failed: %v", err)
	}

	// Should only create the accounts directory, NOT a default account
	accountsDir := filepath.Join(tempDir, "accounts")
	if _, err := os.Stat(accountsDir); os.IsNotExist(err) {
		t.Errorf("accounts directory not created")
	}

	// Default account should NOT be created
	defaultAccountDir := filepath.Join(tempDir, "accounts", "default")
	if _, err := os.Stat(defaultAccountDir); err == nil {
		t.Errorf("default account dir should NOT be created automatically")
	}
}

func TestGetAccounts(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "machinator-test-get")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	if err := InitAccountsDir(tempDir); err != nil {
		t.Fatal(err)
	}

	accounts, err := GetAccounts(tempDir)
	if err != nil {
		t.Fatalf("GetAccounts failed: %v", err)
	}

	// With no accounts created, should return empty list
	if len(accounts) != 0 {
		t.Errorf("expected empty list, got %v", accounts)
	}
}

func TestSetupDefaultAccount(t *testing.T) {
	// Mock home dir
	tempHome, err := os.MkdirTemp("", "machinator-home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempHome)

	// Create .gemini in temp home
	geminiDir := filepath.Join(tempHome, ".gemini")
	os.Mkdir(geminiDir, 0755)
	os.WriteFile(filepath.Join(geminiDir, "config.yaml"), []byte("test: config"), 0644)

	// Set HOME env var (this might be tricky in tests if os.UserHomeDir uses syscalls directly, but usually it respects HOME)
	t.Setenv("HOME", tempHome)

	tempMachinator, err := os.MkdirTemp("", "machinator-root")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempMachinator)

	// Init dirs first
	InitAccountsDir(tempMachinator)

	err = SetupDefaultAccount(tempMachinator)
	if err != nil {
		t.Fatalf("SetupDefaultAccount failed: %v", err)
	}

	destConfig := filepath.Join(tempMachinator, "accounts", "default", ".gemini", "config.yaml")
	if _, err := os.Stat(destConfig); os.IsNotExist(err) {
		t.Errorf("Config file not copied to default account")
	}
}
