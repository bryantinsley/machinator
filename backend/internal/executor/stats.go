package executor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// ExecutionStats tracks cumulative execution statistics across runs.
type ExecutionStats struct {
	TotalExecutions      int           `json:"total_executions"`
	SuccessfulExecutions int           `json:"successful_executions"`
	FailedExecutions     int           `json:"failed_executions"`
	TotalDuration        time.Duration `json:"total_duration"`
	TasksCompleted       []string      `json:"tasks_completed"`
	TasksFailed          []string      `json:"tasks_failed"`
	LastExecution        time.Time     `json:"last_execution"`

	machinatorDir string
}

// LoadStats loads stats from ~/.machinator/stats.json using the provided machinatorDir.
func LoadStats(machinatorDir string) (*ExecutionStats, error) {
	statsFile := filepath.Join(machinatorDir, "stats.json")
	data, err := os.ReadFile(statsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &ExecutionStats{
				TasksCompleted: []string{},
				TasksFailed:    []string{},
				machinatorDir:  machinatorDir,
			}, nil
		}
		return nil, err
	}

	var stats ExecutionStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, err
	}
	stats.machinatorDir = machinatorDir
	return &stats, nil
}

// Save saves stats to ~/.machinator/stats.json using the provided machinatorDir.
func (s *ExecutionStats) Save(machinatorDir string) error {
	dir := machinatorDir
	if dir == "" {
		dir = s.machinatorDir
	}
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".machinator")
	}

	statsFile := filepath.Join(dir, "stats.json")
	
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(statsFile, data, 0644)
}

// Record updates stats based on the provided ExecutionResult and taskID, and then saves them.
func (s *ExecutionStats) Record(result ExecutionResult, taskID string) {
	s.TotalExecutions++
	s.TotalDuration += result.Duration
	s.LastExecution = time.Now()

	if result.Error == nil {
		s.SuccessfulExecutions++
		s.TasksCompleted = append(s.TasksCompleted, taskID)
	} else {
		s.FailedExecutions++
		s.TasksFailed = append(s.TasksFailed, taskID)
	}

	_ = s.Save(s.machinatorDir)
}
