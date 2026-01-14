package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds project-specific configuration.
type Config struct {
	Repo             string `json:"repo"`
	Branch           string `json:"branch"`
	SimpleModelName  string `json:"simple_model_name"`
	ComplexModelName string `json:"complex_model_name"`
}

// Load loads project config from disk.
func Load(machinatorDir string, projectID string) (*Config, error) {
	configPath := filepath.Join(machinatorDir, "projects", projectID, "config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("project %s not found", projectID)
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{
		// Defaults
		Branch:           "main",
		SimpleModelName:  "gemini-3-flash-preview",
		ComplexModelName: "gemini-3-pro-preview",
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Repo == "" {
		return nil, fmt.Errorf("project config missing 'repo'")
	}

	return cfg, nil
}

// Save saves project config to disk.
func Save(machinatorDir string, projectID string, cfg *Config) error {
	projectDir := filepath.Join(machinatorDir, "projects", projectID)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("create project dir: %w", err)
	}

	configPath := filepath.Join(projectDir, "config.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// RepoDir returns the path to the cloned repo.
func RepoDir(machinatorDir, projectID string) string {
	return filepath.Join(machinatorDir, "projects", projectID, "repo")
}

// AgentDir returns the path to an agent's worktree.
func AgentDir(machinatorDir, projectID string, agentID int) string {
	return filepath.Join(machinatorDir, "projects", projectID, "agents", fmt.Sprintf("%d", agentID))
}
