package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config holds the global configuration.
type Config struct {
	MachinatorDir string `json:"-"`

	// DefaultAgentCount is the number of agents created on first run
	// (before any agents exist in state). You can add more with + in the TUI.
	DefaultAgentCount int `json:"default_agent_count"`

	Timeouts struct {
		Idle       Duration `json:"idle"`
		MaxRuntime Duration `json:"max_runtime"`
	} `json:"timeouts"`

	Intervals struct {
		Assigner     Duration `json:"assigner"`
		QuotaRefresh Duration `json:"quota_refresh"`
		AgentWatch   Duration `json:"agent_watch"`
	} `json:"intervals"`

	// HideCommitAuthors is a list of author names/emails to hide from commit log
	HideCommitAuthors []string `json:"hide_commit_authors"`
}

// Duration is a time.Duration that can be unmarshaled from JSON strings like "10m", "1s"
type Duration time.Duration

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		// Try as number (nanoseconds)
		var n int64
		if err := json.Unmarshal(b, &n); err != nil {
			return err
		}
		*d = Duration(n)
		return nil
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(dur)
	return nil
}

func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// Load loads configuration from MACHINATOR_DIR/config.json.
func Load() (*Config, error) {
	dir := getMachinatorDir()

	cfg := &Config{
		MachinatorDir: dir,
	}

	// Set defaults
	cfg.DefaultAgentCount = 3
	cfg.Timeouts.Idle = Duration(10 * time.Minute)
	cfg.Timeouts.MaxRuntime = Duration(30 * time.Minute)
	cfg.Intervals.Assigner = Duration(1 * time.Second)
	cfg.Intervals.QuotaRefresh = Duration(60 * time.Second)
	cfg.Intervals.AgentWatch = Duration(100 * time.Millisecond)

	// Load from file if exists
	configPath := filepath.Join(dir, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No config file, use defaults
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	// Strip JSONC comments
	data = StripJSONComments(data)

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}

// Template returns a documented config template.
func Template() string {
	return `{
  // Number of agents created on first run (before any agents exist).
  // You can add more at runtime with + in the TUI.
  "default_agent_count": 3,

  // Timeout settings (use Go duration strings like "10m", "1h")
  "timeouts": {
    "idle": "10m",
    "max_runtime": "30m"
  },

  // Refresh intervals
  "intervals": {
    "assigner": "1s",
    "quota_refresh": "60s",
    "agent_watch": "100ms"
  },

  // Hide commits by these authors from the TUI Commits section.
  // Matches if author name or email contains any of these strings.
  // Example: ["github-actions", "dependabot"]
  "hide_commit_authors": []
}
`
}

// ConfigPath returns the path to the global config file.
func ConfigPath() string {
	return filepath.Join(getMachinatorDir(), "config.json")
}

// EnsureTemplate creates the config file with template if it doesn't exist.
func EnsureTemplate() (string, error) {
	dir := getMachinatorDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create machinator dir: %w", err)
	}

	configPath := ConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := os.WriteFile(configPath, []byte(Template()), 0644); err != nil {
			return "", fmt.Errorf("write template: %w", err)
		}
	}

	return configPath, nil
}

func getMachinatorDir() string {
	if dir := os.Getenv("MACHINATOR_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".machinator")
}
