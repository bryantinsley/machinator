package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bryantinsley/machinator/orchestrator/pkg/accountpool"
)

func TestExecuteTask_GitPreparation(t *testing.T) {
	// Setup temp directory
	tmpDir, err := os.MkdirTemp("", "git-prep-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	projectRoot := filepath.Join(tmpDir, "project")
	os.MkdirAll(projectRoot, 0755)

	repoDir := filepath.Join(projectRoot, "repo")
	os.MkdirAll(repoDir, 0755)

	// Create a mock project.json
	projectConfig := `{"id": 1, "branch": "main"}`
	os.WriteFile(filepath.Join(projectRoot, "project.json"), []byte(projectConfig), 0644)

	// Setup agent dir to skip worktree creation in this test
	agentID := 1
	agentDir := filepath.Join(projectRoot, "agents", "1")
	os.MkdirAll(agentDir, 0755)

	// Setup originalCwd and template for buildDirective
	oldOriginalCwd := originalCwd
	originalCwd = tmpDir
	defer func() { originalCwd = oldOriginalCwd }()

	os.MkdirAll(filepath.Join(tmpDir, "templates"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "templates", "directive_template.txt"), []byte("Mock template for {{.TaskID}}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte("Mock agents"), 0644)

	// Mock execCommand
	var commandsRun []string
	oldExecCommand := execCommand
	defer func() { execCommand = oldExecCommand }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		cmdStr := name + " " + strings.Join(args, " ")
		commandsRun = append(commandsRun, cmdStr)

		// Return a command that does nothing but exit 0
		// For "git branch --show-current", return "main" initially
		if name == "git" && len(args) > 0 && args[0] == "branch" && args[1] == "--show-current" {
			return exec.Command("echo", "main")
		}

		return exec.Command("true")
	}

	taskID := "task-456"
	agentName := "agent-1"
	pool := accountpool.NewPool()
	pool.Add(accountpool.Account{Name: "default", HomeDir: tmpDir})

	// Run executeTask
	cmd := executeTask(agentID, taskID, agentName, projectRoot, repoDir, pool, false)
	_ = cmd()

	// Verify git operations
	expectedOps := []string{
		"git reset --hard",
		"git fetch origin main",
		"git checkout -B machinator/task-456 origin/main",
	}

	for _, op := range expectedOps {
		found := false
		for _, run := range commandsRun {
			if strings.Contains(run, op) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected git operation '%s' not found in commands run: %v", op, commandsRun)
		}
	}
}
