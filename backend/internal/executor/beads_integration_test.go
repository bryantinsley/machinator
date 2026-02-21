package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBeadsIntegration(t *testing.T) {
	// Create a temporary directory for the mock bd script
	tmpDir := t.TempDir()
	mockBD := filepath.Join(tmpDir, "bd")
	
	// Create a dummy bd script that records its arguments
	recordFile := filepath.Join(tmpDir, "args.txt")
	content := "#!/bin/bash\necho \"$@\" >> " + recordFile + "\nexit 0\n"
	if err := os.WriteFile(mockBD, []byte(content), 0755); err != nil {
		t.Fatalf("Failed to write mock bd: %v", err)
	}
	
	// Update PATH to include the mock bd
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	defer os.Setenv("PATH", oldPath)
	
	worktreeDir := t.TempDir()
	taskID := "task-123"
	
t.Run("CloseBeadTask", func(t *testing.T) {
		logger := &mockLogger{}
		os.Remove(recordFile)
		err := CloseBeadTask(worktreeDir, taskID, logger)
		if err != nil {
			t.Errorf("CloseBeadTask failed: %v", err)
		}
		
		// Verify logs
		found := false
		for _, log := range logger.logs {
			if strings.Contains(log, "successfully closed bead task task-123") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected success log, got: %v", logger.logs)
		}
		
		// Verify arguments
		args, _ := os.ReadFile(recordFile)
		expected := "close task-123 --reason Completed by agent\n"
		if string(args) != expected {
			t.Errorf("Expected args %q, got %q", expected, string(args))
		}
	})
	
t.Run("BlockBeadTask", func(t *testing.T) {
		logger := &mockLogger{}
		os.Remove(recordFile)
		err := BlockBeadTask(worktreeDir, taskID, "some reason", logger)
		if err != nil {
			t.Errorf("BlockBeadTask failed: %v", err)
		}
		
		// Verify logs
		found := false
		for _, log := range logger.logs {
			if strings.Contains(log, "successfully blocked bead task task-123: some reason") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected success log, got: %v", logger.logs)
		}
		
		// Verify arguments
		args, _ := os.ReadFile(recordFile)
		expected := "update task-123 --status=blocked\n"
		if string(args) != expected {
			t.Errorf("Expected args %q, got %q", expected, string(args))
		}
	})
	
t.Run("SyncBeads", func(t *testing.T) {
		logger := &mockLogger{}
		os.Remove(recordFile)
		err := SyncBeads(worktreeDir, logger)
		if err != nil {
			t.Errorf("SyncBeads failed: %v", err)
		}
		
		// Verify logs
		found := false
		for _, log := range logger.logs {
			if strings.Contains(log, "successfully synced beads") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected success log, got: %v", logger.logs)
		}
		
		// Verify arguments
		args, _ := os.ReadFile(recordFile)
		expected := "sync\n"
		if string(args) != expected {
			t.Errorf("Expected args %q, got %q", expected, string(args))
		}
	})
}
