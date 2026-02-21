package executor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewExecutionID(t *testing.T) {
	taskID := "test-task"
	execID := NewExecutionID(taskID)

	if !strings.HasPrefix(execID, taskID+"-") {
		t.Errorf("Expected prefix %s-, got %s", taskID, execID)
	}

	// Should be taskID-YYYYMMDD-HHMMSS
	// Length should be len(taskID) + 1 (dash) + 8 (date) + 1 (dash) + 6 (time) = len(taskID) + 16
	expectedLen := len(taskID) + 16
	if len(execID) != expectedLen {
		t.Errorf("Expected length %d, got %d (%s)", expectedLen, len(execID), execID)
	}
}

func TestCreateLogDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "machinator-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	execID := "test-exec-id"
	baseDir, err := CreateLogDir(tmpDir, execID)
	if err != nil {
		t.Fatal(err)
	}

	expectedBase := filepath.Join(tmpDir, "logs", "executions", execID)
	if baseDir != expectedBase {
		t.Errorf("Expected base dir %s, got %s", expectedBase, baseDir)
	}

	subdirs := []string{
		"orchestrator.log",
		"gemini.log",
		"events.jsonl",
		"directive.md",
	}

	for _, subdir := range subdirs {
		path := filepath.Join(baseDir, subdir)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("Failed to stat subdir %s: %v", path, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("Path %s is not a directory", path)
		}
	}
}

func TestSetGitAuthor(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "machinator-git-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a git repo
	cmdInit := exec.Command("git", "init")
	cmdInit.Dir = tmpDir
	if err := cmdInit.Run(); err != nil {
		t.Fatalf("Failed to git init: %v", err)
	}

	agentID := 42
	err = SetGitAuthor(tmpDir, agentID)
	if err != nil {
		t.Fatalf("SetGitAuthor failed: %v", err)
	}

	// Verify name
	cmdGetName := exec.Command("git", "config", "user.name")
	cmdGetName.Dir = tmpDir
	nameOut, err := cmdGetName.Output()
	if err != nil {
		t.Fatalf("Failed to get git user.name: %v", err)
	}
	expectedName := "Machinator Agent: 42"
	if strings.TrimSpace(string(nameOut)) != expectedName {
		t.Errorf("Expected name %q, got %q", expectedName, string(nameOut))
	}

	// Verify email
	cmdGetEmail := exec.Command("git", "config", "user.email")
	cmdGetEmail.Dir = tmpDir
	emailOut, err := cmdGetEmail.Output()
	if err != nil {
		t.Fatalf("Failed to get git user.email: %v", err)
	}
	expectedEmail := "agent-42.local"
	if strings.TrimSpace(string(emailOut)) != expectedEmail {
		t.Errorf("Expected email %q, got %q", expectedEmail, string(emailOut))
	}
}
