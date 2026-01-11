package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetMachinatorDir(t *testing.T) {
	// Save original env
	original := os.Getenv("MACHINATOR_DIR")
	defer os.Setenv("MACHINATOR_DIR", original)

	t.Run("Default", func(t *testing.T) {
		os.Unsetenv("MACHINATOR_DIR")
		homeDir, _ := os.UserHomeDir()
		expected := filepath.Join(homeDir, ".machinator")
		actual := GetMachinatorDir()
		if actual != expected {
			t.Errorf("expected %s, got %s", expected, actual)
		}
	})

	t.Run("Custom", func(t *testing.T) {
		customDir := "/tmp/custom-machinator"
		os.Setenv("MACHINATOR_DIR", customDir)
		actual := GetMachinatorDir()
		if actual != customDir {
			t.Errorf("expected %s, got %s", customDir, actual)
		}
	})
}
