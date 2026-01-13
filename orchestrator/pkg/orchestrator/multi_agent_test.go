package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// mockMultiAgentExec handles bd ready and git status
func mockMultiAgentExec(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcessMulti", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS_MULTI=1"}
	// Pass args to helper to decide output
	allArgs := append([]string{command}, args...)
	cmd.Env = append(cmd.Env, fmt.Sprintf("MOCK_ARGS=%s", strings.Join(allArgs, " ")))
	return cmd
}

func TestHelperProcessMulti(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS_MULTI") != "1" {
		return
	}
	args := os.Getenv("MOCK_ARGS")

	if strings.Contains(args, "git status --porcelain") {
		// No changes
		os.Exit(0)
	}

	if strings.Contains(args, "bd --sandbox ready --json") {
		// Return some tasks
		tasks := []Task{
			{ID: "task-10", Status: "ready"},
			{ID: "task-11", Status: "ready"},
			{ID: "task-12", Status: "ready"},
			{ID: "task-13", Status: "ready"},
		}
		json.NewEncoder(os.Stdout).Encode(tasks)
		os.Exit(0)
	}

	os.Exit(0)
}

func TestMultiAgentCycling(t *testing.T) {
	// Reset shared state
	claimedTasksMu.Lock()
	claimedTasks = make(map[string]int)
	completedTasks = make(map[string]time.Time)
	claimedTasksMu.Unlock()

	// Override execCommand
	oldExecCommand := execCommand
	execCommand = mockMultiAgentExec
	defer func() { execCommand = oldExecCommand }()

	// Setup model
	m := initialModel(nil, false)
	m.ready = true
	m.state = StateRunning
	m.toolsCheck.State = ToolsCheckStatePassed
	// Simulate 3 agents
	// m.config = &setup.ProjectConfig{AgentCount: 3} // Removed invalid assignment

	// Initialize 3 agents
	for i := 1; i <= 3; i++ {
		m.agents[i] = &AgentState{
			ID:            i,
			Name:          fmt.Sprintf("Agent-%d", i),
			Running:       true,
			CurrentTaskID: fmt.Sprintf("task-%d", i),
		}
		claimTask(fmt.Sprintf("task-%d", i), i)
	}

	// Process done message for Agent 2
	msg := geminiDoneMsg{
		AgentID: 2,
		TaskID:  "task-2",
	}

	updatedModel, cmd := m.Update(msg)
	m = updatedModel.(model)

	// We expect a command (executeTask) to be returned to start the next task
	if cmd == nil {
		t.Fatal("Expected a command to be returned after agent finished, but got nil")
	}

	// Since tea.Cmd is a function, we can't inspect it directly easily without running it.
	// But getting a non-nil cmd is a good sign that executeTask was called.
	// However, if the logic failed to find a task, cmd would be nil.

	// Verify that Agent 2 is NOT running (it will be set to running only when executeTask starts... wait)
	// Actually, executeTask sets agent.Running? No, tui.go sets it?

	// tui.go Update:
	/*
			if agent, exists := m.agents[msg.AgentID]; exists {
				agent.Running = false
	            ...
	            if taskID != "" && claimTask(taskID, msg.AgentID) {
	                ...
	                cmds = append(cmds, executeTask(...))
	            }
	        }
	*/

	// So the agent.Running is set to false initially.
	// The executeTask cmd returns taskStartedMsg, which updates the model to set Running=true.
	// So at this point, Agent 2 should be Running=false.

	if m.agents[2].Running {
		t.Errorf("Agent 2 should be marked not running until taskStartedMsg received")
	}

	// Verify a task was claimed
	// We expect one of task-10, 11, 12, 13 to be claimed by agent 2
	foundClaim := false
	claimedTasksMu.Lock()
	for taskID, agentID := range claimedTasks {
		if agentID == 2 && strings.HasPrefix(taskID, "task-1") {
			foundClaim = true
			break
		}
	}
	claimedTasksMu.Unlock()

	if !foundClaim {
		t.Errorf("Agent 2 did not claim a new task")
	}
}

func TestTickPreservesLoop(t *testing.T) {
	// Drain channel
	for len(geminiDoneChan) > 0 {
		<-geminiDoneChan
	}

	m := initialModel(nil, false)

	// Inject done message
	geminiDoneChan <- geminiDoneMsg{AgentID: 1, TaskID: "t1"}

	// Trigger tick
	_, cmd := m.Update(tickMsg(time.Now()))

	if cmd == nil {
		t.Fatal("Expected cmd")
	}

	msg := cmd()

	// Check type
	switch msg.(type) {
	case geminiDoneMsg:
		t.Fatalf("Bug reproduction: Update returned single geminiDoneMsg, creating a loop break (tick lost)")
	case tea.BatchMsg:
		// BatchMsg implies multiple commands (hopefully including tick)
		t.Log("Got BatchMsg, assuming it includes tick")
	default:
		t.Logf("Got %T", msg)
	}
}
