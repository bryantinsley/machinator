package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// State holds the persistent orchestrator state.
type State struct {
	mu            sync.RWMutex
	MachinatorDir string `json:"-"`

	Agents           []*Agent `json:"agents"`
	AssignmentPaused bool     `json:"assignment_paused"`
	LaunchesPaused   bool     `json:"launches_paused"`
	BarredTasks      []string `json:"barred_tasks"`
}

// Agent represents an agent slot.
type Agent struct {
	ID               int       `json:"id"`
	State            string    `json:"state"` // pending, ready, assigned
	PID              int       `json:"pid,omitempty"`
	TaskID           string    `json:"task_id,omitempty"`
	StartedAt        time.Time `json:"started_at,omitempty"`
	LastActivity     time.Time `json:"last_activity,omitempty"`
	LogOffset        int64     `json:"log_offset,omitempty"`
	MarkedForRemoval bool      `json:"marked_for_removal,omitempty"`
}

// New creates a new State instance.
func New(machinatorDir string) *State {
	return &State{
		MachinatorDir: machinatorDir,
		Agents:        make([]*Agent, 0),
		BarredTasks:   make([]string, 0),
	}
}

// Load loads state from disk.
func Load(machinatorDir string) (*State, error) {
	s := New(machinatorDir)
	path := filepath.Join(machinatorDir, "state.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil // No state file, start fresh
		}
		return nil, fmt.Errorf("read state: %w", err)
	}

	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}

	s.MachinatorDir = machinatorDir
	return s, nil
}

// Save persists state to disk.
func (s *State) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.MachinatorDir, "state.json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}

	return nil
}

// GetAgent returns an agent by ID.
func (s *State) GetAgent(id int) *Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, a := range s.Agents {
		if a.ID == id {
			return a
		}
	}
	return nil
}

// ReadyAgents returns agents in ready state.
func (s *State) ReadyAgents() []*Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ready []*Agent
	for _, a := range s.Agents {
		if a.State == "ready" && !a.MarkedForRemoval {
			ready = append(ready, a)
		}
	}
	return ready
}

// AssignedAgents returns agents in assigned state.
func (s *State) AssignedAgents() []*Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var assigned []*Agent
	for _, a := range s.Agents {
		if a.State == "assigned" {
			assigned = append(assigned, a)
		}
	}
	return assigned
}

// AddAgent adds a new agent slot.
func (s *State) AddAgent() *Agent {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find next ID
	maxID := 0
	for _, a := range s.Agents {
		if a.ID > maxID {
			maxID = a.ID
		}
	}

	agent := &Agent{
		ID:    maxID + 1,
		State: "ready",
	}
	s.Agents = append(s.Agents, agent)
	return agent
}

// IsTaskBarred checks if a task is barred from assignment.
func (s *State) IsTaskBarred(taskID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, t := range s.BarredTasks {
		if t == taskID {
			return true
		}
	}
	return false
}

// BarTask adds a task to the barred list.
func (s *State) BarTask(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, t := range s.BarredTasks {
		if t == taskID {
			return // Already barred
		}
	}
	s.BarredTasks = append(s.BarredTasks, taskID)
}

// IsTaskAssigned checks if a task is currently assigned to any agent.
func (s *State) IsTaskAssigned(taskID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, a := range s.Agents {
		if a.TaskID == taskID && a.State == "assigned" {
			return true
		}
	}
	return false
}
