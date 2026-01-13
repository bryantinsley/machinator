package orchestrator

import (
	"os/exec"
	"testing"
	"time"
)

func TestStalenessDetection(t *testing.T) {
	// Reset shared state
	claimedTasksMu.Lock()
	claimedTasks = make(map[string]int)
	completedTasks = make(map[string]time.Time)
	claimedTasksMu.Unlock()

	// Override execCommand with a dummy that starts a process we can kill
	oldExecCommand := execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		// Just run sleep so it stays alive until killed
		return exec.Command("sleep", "60")
	}
	defer func() { execCommand = oldExecCommand }()

	// Setup model
	m := initialModel(nil, false)
	m.geminiRunning = true
	m.failedTasks = make(map[string]time.Time)

	taskID := "stale-task"
	agentID := 1

	// Start a dummy process to be killed
	cmd := execCommand("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start dummy process: %v", err)
	}
	// Ensure we kill it even if test fails
	defer cmd.Process.Kill()

	agent := &AgentState{
		ID:            agentID,
		Name:          "TestAgent",
		Running:       true,
		CurrentTaskID: taskID,
		LastEventTime: time.Now().Add(-20 * time.Minute), // Older than 10m timeout
		Cmd:           cmd,
		FailedTasks:   make(map[string]time.Time),
	}
	m.agents[agentID] = agent
	claimTask(taskID, agentID)

	if !isTaskClaimed(taskID) {
		t.Fatal("Task should be claimed initially")
	}

	// Trigger timeout check
	// Note: tickMsg is time.Time
	updatedModel, _ := m.Update(tickMsg(time.Now()))
	m = updatedModel.(model)

	// Verify agent is no longer running
	if agent.Running {
		t.Error("Agent should not be running after timeout")
	}

	// Verify task is released
	if isTaskClaimed(taskID) {
		t.Error("Task should be released after timeout")
	}

	// Verify task is marked as failed for the agent
	if _, failed := agent.FailedTasks[taskID]; !failed {
		t.Error("Task should be marked as failed in agent state")
	}

	// Verify task is marked as failed in global state
	if _, failed := m.failedTasks[taskID]; !failed {
		t.Error("Task should be marked as failed in global model state")
	}

	// Verify process is killed
	// Wait a bit for process to be killed
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("Process should have been killed (exit code non-zero)")
		}
	case <-time.After(2 * time.Second):
		t.Error("Process was not killed within 2 seconds")
	}
}
