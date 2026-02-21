package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// NewExecutionID returns a string in the format '{taskID}-{YYYYMMDD}-{HHMMSS}'
func NewExecutionID(taskID string) string {
	now := time.Now()
	return fmt.Sprintf("%s-%s", taskID, now.Format("20060102-150405"))
}

// CreateLogDir creates ~/.machinator/logs/executions/{execID}/ with subdirs for orchestrator.log,
// gemini.log, events.jsonl, directive.md.
func CreateLogDir(machinatorDir string, execID string) (string, error) {
	baseDir := filepath.Join(machinatorDir, "logs", "executions", execID)

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", baseDir, err)
	}

	return baseDir, nil
}

// SetGitAuthor sets the git author name and email in the specified worktree.
func SetGitAuthor(worktreeDir string, agentID int) error {
	name := fmt.Sprintf("Machinator Agent: %d", agentID)
	email := fmt.Sprintf("agent-%d.local", agentID)

	cmdName := exec.Command("git", "config", "user.name", name)
	cmdName.Dir = worktreeDir
	if err := cmdName.Run(); err != nil {
		return fmt.Errorf("failed to set git user.name: %w", err)
	}

	cmdEmail := exec.Command("git", "config", "user.email", email)
	cmdEmail.Dir = worktreeDir
	if err := cmdEmail.Run(); err != nil {
		return fmt.Errorf("failed to set git user.email: %w", err)
	}

	return nil
}
