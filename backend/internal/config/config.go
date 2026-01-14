package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the global configuration.
type Config struct {
	MachinatorDir string

	Agents struct {
		Count int `yaml:"count"`
	} `yaml:"agents"`

	Timeouts struct {
		Idle       time.Duration `yaml:"idle"`
		MaxRuntime time.Duration `yaml:"max_runtime"`
	} `yaml:"timeouts"`

	Intervals struct {
		Assigner     time.Duration `yaml:"assigner"`
		QuotaRefresh time.Duration `yaml:"quota_refresh"`
		AgentWatch   time.Duration `yaml:"agent_watch"`
	} `yaml:"intervals"`
}

// Load loads configuration from MACHINATOR_DIR/config.yaml.
func Load() (*Config, error) {
	dir := getMachinatorDir()

	cfg := &Config{
		MachinatorDir: dir,
	}

	// Set defaults
	cfg.Agents.Count = 3
	cfg.Timeouts.Idle = 10 * time.Minute
	cfg.Timeouts.MaxRuntime = 30 * time.Minute
	cfg.Intervals.Assigner = 1 * time.Second
	cfg.Intervals.QuotaRefresh = 60 * time.Second
	cfg.Intervals.AgentWatch = 100 * time.Millisecond

	// Load from file if exists
	configPath := filepath.Join(dir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No config file, use defaults
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}

func getMachinatorDir() string {
	if dir := os.Getenv("MACHINATOR_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".machinator")
}
