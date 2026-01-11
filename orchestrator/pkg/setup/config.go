package setup

import (
	"os"
	"path/filepath"
)

// GetMachinatorDir returns the directory where machinator configuration and data are stored.
// It uses the MACHINATOR_DIR environment variable if set, otherwise it defaults to ~/.machinator.
func GetMachinatorDir() string {
	if dir := os.Getenv("MACHINATOR_DIR"); dir != "" {
		return dir
	}
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".machinator")
}
