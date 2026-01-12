package setup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// InitAccountsDir ensures the accounts directory structure exists
func InitAccountsDir(machinatorDir string) error {
	accountsDir := filepath.Join(machinatorDir, "accounts")
	defaultAccountDir := filepath.Join(accountsDir, "default")
	return os.MkdirAll(defaultAccountDir, 0755)
}

// SetupDefaultAccount copies the user's ~/.gemini to ~/.machinator/accounts/default/.gemini
func SetupDefaultAccount(machinatorDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}

	srcGemini := filepath.Join(home, ".gemini")

	if _, err := os.Stat(srcGemini); os.IsNotExist(err) {
		return nil
	}

	destDir := filepath.Join(machinatorDir, "accounts", "default", ".gemini")

	// Check if already exists
	if _, err := os.Stat(destDir); err == nil {
		return nil // Already exists, don't overwrite
	}

	return copyDir(srcGemini, destDir)
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	info, err := os.Stat(src)
	if err == nil {
		os.Chmod(dst, info.Mode())
	}

	return nil
}

// GetAccounts returns a list of configured accounts
func GetAccounts(machinatorDir string) ([]string, error) {
	accountsDir := filepath.Join(machinatorDir, "accounts")
	entries, err := os.ReadDir(accountsDir)
	if err != nil {
		return nil, err
	}

	var accounts []string
	for _, entry := range entries {
		if entry.IsDir() {
			accounts = append(accounts, entry.Name())
		}
	}
	return accounts, nil
}
