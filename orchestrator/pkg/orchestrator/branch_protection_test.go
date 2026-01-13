package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildDirective_BranchProtection(t *testing.T) {
	// Setup temp directory
	tmpDir, err := os.MkdirTemp("", "bp-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Mock originalCwd to point to tmpDir
	oldOriginalCwd := originalCwd
	originalCwd = tmpDir
	defer func() { originalCwd = oldOriginalCwd }()

	// Create templates directory and mock template
	os.MkdirAll(filepath.Join(tmpDir, "templates"), 0755)

	// Create a mock template that uses the BranchProtection variable
	// We use the actual logic snippet we want to test
	mockTemplate := `
{{if eq .BranchProtection "pr-required"}}
PR REQUIRED
{{else}}
DIRECT PUSH
{{end}}
`
	os.WriteFile(filepath.Join(tmpDir, "templates", "directive_template.txt"), []byte(mockTemplate), 0644)

	// Mock AGENTS.md
	os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte("Mock agents"), 0644)

	// Mock execCommand to avoid real bd calls
	oldExecCommand := execCommand
	defer func() { execCommand = oldExecCommand }()
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	// Test Case 1: "none" (default)
	output, err := buildDirective("agent-1", "task-1", tmpDir, "none")
	if err != nil {
		t.Fatalf("buildDirective failed: %v", err)
	}
	if !strings.Contains(output, "DIRECT PUSH") {
		t.Errorf("Expected 'DIRECT PUSH' for none, got: %s", output)
	}
	if strings.Contains(output, "PR REQUIRED") {
		t.Errorf("Did not expect 'PR REQUIRED' for none")
	}

	// Test Case 2: "pr-required"
	output, err = buildDirective("agent-1", "task-1", tmpDir, "pr-required")
	if err != nil {
		t.Fatalf("buildDirective failed: %v", err)
	}
	if !strings.Contains(output, "PR REQUIRED") {
		t.Errorf("Expected 'PR REQUIRED' for pr-required, got: %s", output)
	}
	if strings.Contains(output, "DIRECT PUSH") {
		t.Errorf("Did not expect 'DIRECT PUSH' for pr-required")
	}

	// Test Case 3: Empty (should be same as none/else)
	output, err = buildDirective("agent-1", "task-1", tmpDir, "")
	if err != nil {
		t.Fatalf("buildDirective failed: %v", err)
	}
	if !strings.Contains(output, "DIRECT PUSH") {
		t.Errorf("Expected 'DIRECT PUSH' for empty, got: %s", output)
	}
}
