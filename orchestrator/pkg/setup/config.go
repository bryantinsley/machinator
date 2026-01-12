package setup

import (
	"encoding/json"
	"fmt"
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

// LoadProject loads a project configuration by ID.
func LoadProject(projectID int) (*ProjectConfig, error) {
	machinatorDir := GetMachinatorDir()
	projectsDir := filepath.Join(machinatorDir, "projects")

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read projects directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			configPath := filepath.Join(projectsDir, entry.Name(), "project.json")
			data, err := os.ReadFile(configPath)
			if err != nil {
				continue
			}

			var config ProjectConfig
			if err := json.Unmarshal(data, &config); err != nil {
				continue
			}

			if config.ID == projectID {
				config.DirName = entry.Name()
				if err := config.Validate(); err != nil {
					return nil, fmt.Errorf("invalid project configuration: %w", err)
				}
				return &config, nil
			}
		}
	}

	return nil, fmt.Errorf("project with ID %d not found", projectID)
}
