package orchestrator

import (
	"os"
	"os/exec"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// mockExecCommandDone for testing
func mockExecCommandDone(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcessDone", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS_DONE=1"}
	return cmd
}

// TestHelperProcessDone is used to mock exec.Command
func TestHelperProcessDone(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS_DONE") != "1" {
		return
	}
	// Simple mock: return success with no output for git status
	os.Exit(0)
}

func TestDoneRouting(t *testing.T) {
	// Reset shared state
	claimedTasksMu.Lock()
	claimedTasks = make(map[string]int)
	completedTasks = make(map[string]time.Time)
	claimedTasksMu.Unlock()

	// Override execCommand
	oldExecCommand := execCommand
	execCommand = mockExecCommandDone
	defer func() { execCommand = oldExecCommand }()

	// Setup model
	m := initialModel(nil, false)
	m.ready = true
	m.state = StateRunning
	m.toolsCheck.State = ToolsCheckStatePassed

	// Initialize agents
	agentID1 := 1
	agentID2 := 2

	m.agents[agentID1] = &AgentState{
		ID:            agentID1,
		Name:          "Agent-1",
		Running:       true,
		CurrentTaskID: "task-1",
	}
	m.agents[agentID2] = &AgentState{
		ID:            agentID2,
		Name:          "Agent-2",
		Running:       true,
		CurrentTaskID: "task-2",
	}

	// Claim tasks
	claimTask("task-1", agentID1)
	claimTask("task-2", agentID2)

	// 1. Test geminiDoneMsg from agent 1
	msg1 := geminiDoneMsg{
		AgentID: agentID1,
		TaskID:  "task-1",
	}

	updatedModel, _ := m.Update(msg1)
	m = updatedModel.(model)

	if m.agents[agentID1].Running {
		t.Errorf("Expected Agent 1 to be not running after geminiDoneMsg")
	}
	if isTaskClaimed("task-1") {
		t.Errorf("Expected task-1 to be released after geminiDoneMsg")
	}

	// 2. Test geminiDoneMsg from agent 2
	msg2 := geminiDoneMsg{
		AgentID: agentID2,
		TaskID:  "task-2",
	}

	updatedModel, _ = m.Update(msg2)
	m = updatedModel.(model)

	if m.agents[agentID2].Running {
		t.Errorf("Expected Agent 2 to be not running after geminiDoneMsg")
	}
	if isTaskClaimed("task-2") {
		t.Errorf("Expected task-2 to be released after geminiDoneMsg")
	}

	// 3. Test exit-once mode
	m.exitOnce = true
	msg3 := geminiDoneMsg{
		AgentID: agentID1,
		TaskID:  "task-3",
	}

	_, cmd := m.Update(msg3)
	if cmd == nil {
		t.Errorf("Expected tea.Quit command in exit-once mode")
	} else {
		// Verify it's tea.Quit
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Errorf("Expected tea.QuitMsg, got %T", msg)
		}
	}
}
