package orchestrator

import (
	"testing"
	"time"
)

func TestFindReadyTask_StaleInProgress(t *testing.T) {
	// Reset global state
	claimedTasksMu.Lock()
	claimedTasks = make(map[string]int)
	completedTasks = make(map[string]time.Time)
	claimedTasksMu.Unlock()

	// Setup stale tasks list
	tasks := []Task{
		{ID: "task-1", Status: "in_progress", Assignee: "Agent-1"},
		{ID: "task-2", Status: "ready"},
	}

	agentName := "Agent-1"
	failedTasks := make(map[string]time.Time)
	repoPath := "."

	// Simulate Agent-1 just finished task-1
	markTaskCompleted("task-1")
	// task-1 is released (not in claimedTasks)

	// Call findReadyTask
	// We expect it to SKIP task-1 because it is completed, even though it is "in_progress" in tasks list.
	// But currently it lacks the check.

	// mock execCommand to avoid running bd
	oldExecCommand := execCommand
	defer func() { execCommand = oldExecCommand }()
	execCommand = mockMultiAgentExec // reusing mock from multi_agent_test.go

	nextTask := findReadyTask(tasks, agentName, failedTasks, repoPath)

	if nextTask == "task-1" {
		t.Fatal("findReadyTask returned task-1 which is marked completed! Bug reproduced.")
	}

	if nextTask != "task-2" && nextTask != "task-10" { // task-10 comes from mock bd ready
		// If it skips task-1, it might fall through to bd ready (mock returns task-10)
		// Or if we implemented logic to look for other tasks in 'tasks', it might find task-2.
		// But findReadyTask implementation only looks for "in_progress" in 'tasks',
		// then falls back to 'bd ready'.
		// So we expect it to NOT return task-1.
		t.Logf("Returned task: %s", nextTask)
	}
}
