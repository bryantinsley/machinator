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

	// Strip JSONC comments (// style)
	data = stripJSONComments(data)

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

// stripJSONComments removes // comments from JSONC
func stripJSONComments(data []byte) []byte {
	var result []byte
	inString := false
	i := 0
	for i < len(data) {
		// Track string state
		if data[i] == '"' && (i == 0 || data[i-1] != '\\') {
			inString = !inString
		}

		// Check for // comment outside strings
		if !inString && i+1 < len(data) && data[i] == '/' && data[i+1] == '/' {
			// Skip to end of line
			for i < len(data) && data[i] != '\n' {
				i++
			}
			continue
		}

		result = append(result, data[i])
		i++
	}
	return result
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

// ConfigPath returns the path to the project config file.
func ConfigPath(machinatorDir, projectID string) string {
	return filepath.Join(machinatorDir, "projects", projectID, "config.json")
}

// Template returns a documented config template.
func Template() string {
	return `{
  // Git repository URL (required)
  // Example: "git@github.com:user/repo" or "https://github.com/user/repo"
  "repo": "",

  // Branch to track (default: "main")
  "branch": "main",

  // Model for simple/quick tasks (CHALLENGE:simple)
  // Example: "gemini-3-flash-preview", "gemini-2.5-flash"
  "simple_model_name": "gemini-3-flash-preview",

  // Model for complex tasks (CHALLENGE:complex)
  // Example: "gemini-3-pro-preview", "gemini-2.5-pro"  
  "complex_model_name": "gemini-3-pro-preview"
}
`
}

// EnsureTemplate creates the config file with template if it doesn't exist.
func EnsureTemplate(machinatorDir, projectID string) (string, error) {
	projectDir := filepath.Join(machinatorDir, "projects", projectID)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return "", fmt.Errorf("create project dir: %w", err)
	}

	configPath := ConfigPath(machinatorDir, projectID)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := os.WriteFile(configPath, []byte(Template()), 0644); err != nil {
			return "", fmt.Errorf("write template: %w", err)
		}
	}

	return configPath, nil
}
