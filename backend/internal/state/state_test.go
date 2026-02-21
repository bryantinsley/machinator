package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewState(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)

	if len(s.Agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(s.Agents))
	}
	if len(s.BarredTasks) != 0 {
		t.Errorf("expected 0 barred tasks, got %d", len(s.BarredTasks))
	}
}

func TestAddAgent(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)

	agent := s.AddAgent()
	if agent.ID != 1 {
		t.Errorf("expected agent ID 1, got %d", agent.ID)
	}
	if agent.State != "pending" {
		t.Errorf("expected agent state 'pending', got '%s'", agent.State)
	}
	if len(s.Agents) != 1 {
		t.Errorf("expected 1 agent in state, got %d", len(s.Agents))
	}
}

func TestSetAgentReady(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)

	agent := s.AddAgent()
	s.SetAgentReady(agent.ID)

	updatedAgent := s.GetAgent(agent.ID)
	if updatedAgent.State != "ready" {
		t.Errorf("expected agent state 'ready', got '%s'", updatedAgent.State)
	}
}

func TestAssignTask(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)

	agent := s.AddAgent()
	s.SetAgentReady(agent.ID)

	taskID := "task-123"
	success := s.AssignTask(agent.ID, taskID)
	if !success {
		t.Errorf("expected AssignTask to succeed")
	}

	updatedAgent := s.GetAgent(agent.ID)
	if updatedAgent.State != "assigned" {
		t.Errorf("expected agent state 'assigned', got '%s'", updatedAgent.State)
	}
	if updatedAgent.TaskID != taskID {
		t.Errorf("expected agent TaskID '%s', got '%s'", taskID, updatedAgent.TaskID)
	}
}

func TestCompleteTask(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)

	agent := s.AddAgent()
	s.SetAgentReady(agent.ID)
	s.AssignTask(agent.ID, "task-123")

	s.CompleteTask(agent.ID)

	updatedAgent := s.GetAgent(agent.ID)
	if updatedAgent.State != "ready" {
		t.Errorf("expected agent state 'ready' after completion, got '%s'", updatedAgent.State)
	}
	if updatedAgent.TaskID != "" {
		t.Errorf("expected empty TaskID after completion, got '%s'", updatedAgent.TaskID)
	}
}

func TestBarTask(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)

	taskID := "barred-task"
	s.BarTask(taskID)

	if !s.IsTaskBarred(taskID) {
		t.Errorf("expected task '%s' to be barred", taskID)
	}

	if s.IsTaskBarred("other-task") {
		t.Errorf("expected 'other-task' not to be barred")
	}
}

func TestIsTaskAssigned(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)

	agent := s.AddAgent()
	s.SetAgentReady(agent.ID)
	taskID := "task-123"

	if s.IsTaskAssigned(taskID) {
		t.Errorf("expected task '%s' not to be assigned yet", taskID)
	}

	s.AssignTask(agent.ID, taskID)

	if !s.IsTaskAssigned(taskID) {
		t.Errorf("expected task '%s' to be assigned", taskID)
	}
}

func TestReadyAgents(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)

	a1 := s.AddAgent() // pending
	a2 := s.AddAgent() // pending
	s.SetAgentReady(a2.ID)

	ready := s.ReadyAgents()
	if len(ready) != 1 {
		t.Errorf("expected 1 ready agent, got %d", len(ready))
	}
	if ready[0].ID != a2.ID {
		t.Errorf("expected agent %d to be ready, got %d", a2.ID, ready[0].ID)
	}

	s.SetAgentReady(a1.ID)
	ready = s.ReadyAgents()
	if len(ready) != 2 {
		t.Errorf("expected 2 ready agents, got %d", len(ready))
	}
}

func TestPendingAgents(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)

	a1 := s.AddAgent() // pending
	a2 := s.AddAgent() // pending
	s.SetAgentReady(a2.ID)

	pending := s.PendingAgents()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending agent, got %d", len(pending))
	}
	if pending[0].ID != a1.ID {
		t.Errorf("expected agent %d to be pending, got %d", a1.ID, pending[0].ID)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	s := New(tmpDir)

	a1 := s.AddAgent()
	s.SetAgentReady(a1.ID)
	s.BarTaskAndSave("task-barred")

	// Verify file exists
	path := filepath.Join(tmpDir, "state.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected state file to exist at %s", path)
	}

	// Load into new state instance
	s2, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if len(s2.Agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(s2.Agents))
	}
	if s2.Agents[0].ID != a1.ID || s2.Agents[0].State != "ready" {
		t.Errorf("agent state mismatch")
	}
	if !s2.IsTaskBarred("task-barred") {
		t.Errorf("expected task to be barred in loaded state")
	}
}
