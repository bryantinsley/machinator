package executor

import (
	"fmt"
	"time"
)

// ExecuteWithRetry wraps ExecuteTask with retry logic.
// It will retry the task up to maxRetries times if ExecuteTask returns an error.
// Between attempts, it waits for (30s * attempt number) and resets the worktree.
func ExecuteWithRetry(cfg ExecutionConfig, taskID string, taskDescription string, agentsContext string, machinatorDir string, logger Logger, maxRetries int) ExecutionResult {
	var result ExecutionResult

	for attempt := 0; attempt <= maxRetries; attempt++ {
		result = ExecuteTask(cfg, taskID, taskDescription, agentsContext, machinatorDir, logger)

		// If success, return immediately
		if result.Error == nil {
			return result
		}

		// If failure and we have retries left
		if attempt < maxRetries {
			waitTime := time.Duration(30*(attempt+1)) * time.Second
			msg := fmt.Sprintf("Task %s failed (attempt %d/%d): %v. Retrying in %v...", taskID, attempt+1, maxRetries+1, result.Error, waitTime)
			if logger != nil {
				logger.Log("executor", msg)
			}

			// Wait before retry
			time.Sleep(waitTime)

			// Reset worktree before retry
			if logger != nil {
				logger.Log("executor", fmt.Sprintf("Resetting worktree for task %s before retry", taskID))
			}
			if err := ResetWorktree(cfg.WorktreeDir, taskID); err != nil {
				if logger != nil {
					logger.Log("executor", fmt.Sprintf("Failed to reset worktree for task %s: %v", taskID, err))
				}
				// We still retry even if reset fails, though it might fail again.
			}
		} else {
			if logger != nil {
				logger.Log("executor", fmt.Sprintf("Task %s failed after %d attempts. Giving up.", taskID, maxRetries+1))
			}
		}
	}

	return result
}
