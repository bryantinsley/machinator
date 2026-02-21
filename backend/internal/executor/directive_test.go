package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildDirective_Basic(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer os.Chdir(oldDir)

	// Setup mock files
	if err := os.MkdirAll("bootstrap", 0755); err != nil {
		t.Fatal(err)
	}
	template := "You are AGENT_NAME_VAR, execute task TASK_ID_VAR: TASK_CONTEXT_VAR. Full context: PROJECT_CONTEXT_VAR"
	if err := os.WriteFile("bootstrap/directive_template.txt", []byte(template), 0644); err != nil {
		t.Fatal(err)
	}
	agentsContent := "Line 1\nLine 2\nLine 3"
	if err := os.WriteFile("AGENTS.md", []byte(agentsContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Test inputs
	taskID := "task-123"
	taskDesc := "Create a new feature"
	agentName := "Bryan"
	
	// Call BuildDirective
	path, err := BuildDirective(taskID, taskDesc, "context ignored", agentName, "main")
	if err != nil {
		t.Fatalf("BuildDirective returned error: %v", err)
	}

	// Verify file path
	expectedPath := filepath.Join(tmpDir, ".machinator", "directive.md")
	evalExpected, _ := filepath.EvalSymlinks(expectedPath)
	evalActual, _ := filepath.EvalSymlinks(path)
	if evalExpected != evalActual {
		t.Errorf("expected path %s, got %s", evalExpected, evalActual)
	}

	// Verify contents
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read generated directive: %v", err)
	}
	content := string(contentBytes)

	if !strings.Contains(content, agentName) {
		t.Errorf("content missing agent name %s: %s", agentName, content)
	}
	if !strings.Contains(content, taskID) {
		t.Errorf("content missing task ID %s: %s", taskID, content)
	}
	if !strings.Contains(content, taskDesc) {
		t.Errorf("content missing task description %s: %s", taskDesc, content)
	}
	// BuildDirective appends a newline to each line it reads from AGENTS.md
	expectedProjectContext := "Line 1\nLine 2\nLine 3\n"
	if !strings.Contains(content, expectedProjectContext) {
		t.Errorf("content missing project context %q: %s", expectedProjectContext, content)
	}
}

func TestBuildDirective_SpecialChars(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer os.Chdir(oldDir)

	// Setup mock files
	if err := os.MkdirAll("bootstrap", 0755); err != nil {
		t.Fatal(err)
	}
	template := "TASK_CONTEXT_VAR"
	if err := os.WriteFile("bootstrap/directive_template.txt", []byte(template), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("AGENTS.md", []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	specialChars := "Quotes: \"'\", Newlines: \n, Backticks: ``, Symbols: !@#$%^&*()"
	path, err := BuildDirective("task-456", specialChars, "", "Agent", "")
	if err != nil {
		t.Fatalf("BuildDirective failed: %v", err)
	}

	contentBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contentBytes) != specialChars {
		t.Errorf("expected %q, got %q", specialChars, string(contentBytes))
	}
}

func TestBuildDirective_EmptyContext(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}
	defer os.Chdir(oldDir)

	// Setup mock files
	if err := os.MkdirAll("bootstrap", 0755); err != nil {
		t.Fatal(err)
	}
	template := "AGENT_NAME_VAR"
	if err := os.WriteFile("bootstrap/directive_template.txt", []byte(template), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("AGENTS.md", []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// empty agentsContext should not crash
	_, err = BuildDirective("task", "desc", "", "agent", "")
	if err != nil {
		t.Errorf("BuildDirective failed with empty context: %v", err)
	}
}