package executor

import (
	"os"
	"strings"
	"testing"
)

func TestBuildDirective(t *testing.T) {
	// In Bazel go_test, the working directory is the package directory.
	// We need to move up to the runfiles root to access data files.
	// The runfiles root is 3 levels up from backend/internal/executor.
	if err := os.Chdir("../../.."); err != nil {
		t.Fatalf("Failed to change directory to runfiles root: %v", err)
	}

	taskID := "test-task"
	taskDesc := "This is a test goal."
	agentName := "TestAgent"
	agentsCtx := "Extra context"
	branchProt := "main:protected"

	directivePath, err := BuildDirective(taskID, taskDesc, agentsCtx, agentName, branchProt)
	if err != nil {
		t.Fatalf("BuildDirective failed: %v", err)
	}

	// Verify the file exists
	if _, err := os.Stat(directivePath); os.IsNotExist(err) {
		t.Errorf("Directive file not created at %s", directivePath)
	}

	// Read and verify content
	content, err := os.ReadFile(directivePath)
	if err != nil {
		t.Fatalf("Failed to read created directive: %v", err)
	}

	strContent := string(content)
	if !strings.Contains(strContent, agentName) {
		t.Errorf("Directive missing agent name: %s", agentName)
	}
	if !strings.Contains(strContent, taskID) {
		t.Errorf("Directive missing task ID: %s", taskID)
	}
	if !strings.Contains(strContent, taskDesc) {
		t.Errorf("Directive missing task description: %s", taskDesc)
	}
	if !strings.Contains(strContent, "Agent Instructions") {
		t.Errorf("Directive missing project context (from AGENTS.md)")
	}

	// Cleanup
	os.Remove(directivePath)
}
