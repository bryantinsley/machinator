package executor

import (
	"fmt"
	"os/exec"
)

// CloseBeadTask runs 'bd close {taskID} --reason "Completed by agent"' in the worktree dir.
func CloseBeadTask(worktreeDir string, taskID string, logger Logger) error {
	cmd := exec.Command("bd", "close", taskID, "--reason", "Completed by agent")
	cmd.Dir = worktreeDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := fmt.Sprintf("failed to close bead task %s: %v, output: %s", taskID, err, string(output))
		if logger != nil {
			logger.Log("beads", msg)
		}
		return fmt.Errorf("%s", msg)
	}

	msg := fmt.Sprintf("successfully closed bead task %s", taskID)
	if logger != nil {
		logger.Log("beads", msg)
	}
	return nil
}

// BlockBeadTask runs 'bd update {taskID} --status=blocked' in the worktree dir.
func BlockBeadTask(worktreeDir string, taskID string, reason string, logger Logger) error {
	// Note: 'bd update' doesn't seem to take a --reason flag based on the specs,
	// but it's good practice to log it if provided.
	cmd := exec.Command("bd", "update", taskID, "--status=blocked")
	cmd.Dir = worktreeDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := fmt.Sprintf("failed to block bead task %s: %v, output: %s", taskID, err, string(output))
		if logger != nil {
			logger.Log("beads", msg)
		}
		return fmt.Errorf("%s", msg)
	}

	msg := fmt.Sprintf("successfully blocked bead task %s: %s", taskID, reason)
	if logger != nil {
		logger.Log("beads", msg)
	}
	return nil
}

// SyncBeads runs 'bd sync' in the worktree dir.
func SyncBeads(worktreeDir string, logger Logger) error {
	cmd := exec.Command("bd", "sync")
	cmd.Dir = worktreeDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := fmt.Sprintf("failed to sync beads: %v, output: %s", err, string(output))
		if logger != nil {
			logger.Log("beads", msg)
		}
		return fmt.Errorf("%s", msg)
	}

	msg := "successfully synced beads"
	if logger != nil {
		logger.Log("beads", msg)
	}
	return nil
}
