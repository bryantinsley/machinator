package orchestrator

import (
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"
)

func TestClaimingSystem(t *testing.T) {
	// Reset the claimed tasks before starting
	claimedTasksMu.Lock()
	claimedTasks = make(map[string]int)
	claimedTasksMu.Unlock()

	taskID := "task-1"
	agentID := 1

	// 1. claimTask() returns true for unclaimed tasks
	if !claimTask(taskID, agentID) {
		t.Errorf("Expected claimTask to return true for unclaimed task")
	}

	// 2. claimTask() returns false for already-claimed tasks
	if claimTask(taskID, 2) {
		t.Errorf("Expected claimTask to return false for already-claimed task")
	}

	// 3. releaseTask() frees the claim
	releaseTask(taskID)
	if isTaskClaimed(taskID) {
		t.Errorf("Expected task to be unclaimed after releaseTask")
	}

	if !claimTask(taskID, agentID) {
		t.Errorf("Expected claimTask to return true after releasing the claim")
	}
}

func TestSimultaneousClaim(t *testing.T) {
	// Reset the claimed tasks before starting
	claimedTasksMu.Lock()
	claimedTasks = make(map[string]int)
	claimedTasksMu.Unlock()

	taskID := "race-task"
	numAgents := 100
	var wg sync.WaitGroup
	claims := make(chan bool, numAgents)

	for i := 0; i < numAgents; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if claimTask(taskID, id) {
				claims <- true
			}
		}(i)
	}

	wg.Wait()
	close(claims)

	count := 0
	for range claims {
		count++
	}

	// 4. Two agents cannot claim the same task simultaneously
	if count != 1 {
		t.Errorf("Expected exactly 1 agent to successfully claim the task, but got %d", count)
	}
}

func TestFindReadyTaskSkipsClaimed(t *testing.T) {
	// Reset the claimed tasks before starting
	claimedTasksMu.Lock()
	claimedTasks = make(map[string]int)
	completedTasks = make(map[string]time.Time)
	claimedTasksMu.Unlock()

	// Mock execCommand to return a specific task when 'bd ready' is called
	oldExecCommand := execCommand
	defer func() { execCommand = oldExecCommand }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		isReady := false
		for _, a := range arg {
			if a == "ready" {
				isReady = true
				break
			}
		}
		if name == "bd" && isReady {
			// Use a helper process or just a simple command that works in sandbox
			// 'echo' should work. Let's use /bin/echo to be sure.
			return exec.Command("/bin/echo", `[{"id": "task-ready", "status": "ready"}]`)
		}
		return exec.Command("/usr/bin/true")
	}

	tasks := []Task{
		{ID: "task-assigned", Status: "in_progress", Assignee: "agent-1"},
	}
	failedTasks := make(map[string]time.Time)

	// Case 1: Assigned task is claimed by another agent
	claimTask("task-assigned", 2)
	got := findReadyTask(tasks, "agent-1", failedTasks, ".")
	if got != "task-ready" {
		t.Errorf("Expected findReadyTask to skip claimed assigned task and find task-ready, but got '%s'", got)
	}

	// Case 2: Both assigned and ready tasks are claimed
	claimTask("task-ready", 2)
	got = findReadyTask(tasks, "agent-1", failedTasks, ".")
	if got != "" {
		t.Errorf("Expected findReadyTask to return empty string when all tasks are claimed, but got %s", got)
	}

	// Case 3: Release task-ready and it should be found
	releaseTask("task-ready")
	got = findReadyTask(tasks, "agent-1", failedTasks, ".")
	if got != "task-ready" {
		t.Errorf("Expected findReadyTask to find task-ready after release, but got %s", got)
	}
}

// TestMain provides a way to run helper commands for mocking exec.Command if needed,
// but for simple 'echo' it might not be necessary.
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
