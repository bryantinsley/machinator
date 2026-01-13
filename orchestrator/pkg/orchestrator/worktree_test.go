package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bryantinsley/machinator/orchestrator/pkg/accountpool"
)

func TestExecuteTask_WorktreeCreation(t *testing.T) {
	// Setup temp directory
	tmpDir, err := os.MkdirTemp("", "worktree-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	projectRoot := filepath.Join(tmpDir, "project")
	os.MkdirAll(projectRoot, 0755)

	repoDir := filepath.Join(projectRoot, "repo")
	os.MkdirAll(repoDir, 0755)

	// Create a mock project.json
	projectConfig := `{"id": 1, "branch": "develop"}`
	os.WriteFile(filepath.Join(projectRoot, "project.json"), []byte(projectConfig), 0644)

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

		if name == "git" && len(args) > 5 && args[2] == "worktree" && args[3] == "add" {
			// Actually create the directory to simulate git worktree add
			os.MkdirAll(args[5], 0755)
		}

		// Return a command that does nothing but exit 0
		return exec.Command("true")
	}

	// 1. Case: Agent dir MISSING -> should create worktree
	agentID := 1
	taskID := "task-123"
	agentName := "agent-1"

	pool := accountpool.NewPool()
	pool.Add(accountpool.Account{Name: "default", HomeDir: tmpDir})

	cmd := executeTask(agentID, taskID, agentName, projectRoot, repoDir, pool, false)
	msg := cmd()

	// Verify it started successfully
	if fMsg, ok := msg.(taskFailedMsg); ok {
		t.Errorf("Expected taskStartedMsg, got taskFailedMsg: %s", fMsg.reason)
	} else if _, ok := msg.(taskStartedMsg); !ok {
		t.Errorf("Expected taskStartedMsg, got %T", msg)
	}

	// Verify worktree creation was attempted
	foundWorktreeAdd := false
	for _, c := range commandsRun {
		if strings.Contains(c, "worktree add") && strings.Contains(c, "agents/1") && strings.Contains(c, "develop") {
			foundWorktreeAdd = true
			break
		}
	}
	if !foundWorktreeAdd {
		t.Errorf("Expected git worktree add command with branch 'develop', but none found in: %v", commandsRun)
	}

	// 2. Case: Agent dir EXISTS -> should NOT create worktree
	commandsRun = nil
	agentDir := filepath.Join(projectRoot, "agents", "1")
	os.MkdirAll(agentDir, 0755)

	cmd = executeTask(agentID, taskID, agentName, projectRoot, repoDir, pool, false)
	_ = cmd()

	foundWorktreeAdd = false
	for _, c := range commandsRun {
		if strings.Contains(c, "worktree add") {
			foundWorktreeAdd = true
			break
		}
	}
	if foundWorktreeAdd {
		t.Errorf("Did not expect git worktree add when agent dir exists")
	}

	// 3. Verify bd init --from-jsonl is called
	// (Happens before task execution in all cases)
	foundBdInit := false
	for _, c := range commandsRun {
		if strings.Contains(c, "bd") && strings.Contains(c, "init") && strings.Contains(c, "--from-jsonl") {
			foundBdInit = true
			break
		}
	}
	if !foundBdInit {
		t.Errorf("Expected bd init --from-jsonl, but none found in: %v", commandsRun)
	}

	// 4. Failure to create worktree returns taskFailedMsg
	os.RemoveAll(agentDir)
	commandsRun = nil
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 3 && args[2] == "worktree" && args[3] == "add" {
			// Return a command that fails
			return exec.Command("false")
		}
		return exec.Command("true")
	}

	cmd = executeTask(agentID, taskID, agentName, projectRoot, repoDir, pool, false)
	msg = cmd()

	if _, ok := msg.(taskFailedMsg); !ok {
		t.Errorf("Expected taskFailedMsg on worktree creation failure, got %T", msg)
	}
}
