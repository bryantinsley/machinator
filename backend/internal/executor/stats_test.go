package executor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStats(t *testing.T) {
	tmpDir := t.TempDir()
	machinatorDir := filepath.Join(tmpDir, ".machinator")

	// Test LoadStats for non-existent file
	stats, err := LoadStats(machinatorDir)
	if err != nil {
		t.Fatalf("LoadStats failed: %v", err)
	}
	if stats.TotalExecutions != 0 {
		t.Errorf("expected 0 total executions, got %d", stats.TotalExecutions)
	}

	// Test Record
	result := ExecutionResult{
		ExitCode: 0,
		Duration: 10 * time.Second,
		Error:    nil,
	}
	stats.Record(result, "task-1")

	if stats.TotalExecutions != 1 {
		t.Errorf("expected 1 total execution, got %d", stats.TotalExecutions)
	}
	if stats.SuccessfulExecutions != 1 {
		t.Errorf("expected 1 successful execution, got %d", stats.SuccessfulExecutions)
	}
	if stats.TotalDuration != 10*time.Second {
		t.Errorf("expected 10s duration, got %v", stats.TotalDuration)
	}
	if len(stats.TasksCompleted) != 1 || stats.TasksCompleted[0] != "task-1" {
		t.Errorf("expected [task-1] completed tasks, got %v", stats.TasksCompleted)
	}

	// Test Save/Load
	stats2, err := LoadStats(machinatorDir)
	if err != nil {
		t.Fatalf("LoadStats failed: %v", err)
	}
	if stats2.TotalExecutions != 1 {
		t.Errorf("expected 1 total execution after load, got %d", stats2.TotalExecutions)
	}
	if stats2.SuccessfulExecutions != 1 {
		t.Errorf("expected 1 successful execution after load, got %d", stats2.SuccessfulExecutions)
	}

	// Test Failed Record
	resultFail := ExecutionResult{
		ExitCode: 1,
		Duration: 5 * time.Second,
		Error:    os.ErrNotExist,
	}
	stats2.Record(resultFail, "task-2")

	if stats2.TotalExecutions != 2 {
		t.Errorf("expected 2 total executions, got %d", stats2.TotalExecutions)
	}
	if stats2.FailedExecutions != 1 {
		t.Errorf("expected 1 failed execution, got %d", stats2.FailedExecutions)
	}
	if len(stats2.TasksFailed) != 1 || stats2.TasksFailed[0] != "task-2" {
		t.Errorf("expected [task-2] failed tasks, got %v", stats2.TasksFailed)
	}
}
