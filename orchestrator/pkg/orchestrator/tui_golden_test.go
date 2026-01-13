package orchestrator

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bryantinsley/machinator/orchestrator/pkg/ui/agentgrid"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func TestTUI_Golden(t *testing.T) {
	update := false
	if f := flag.Lookup("update"); f != nil {
		update = f.Value.(flag.Getter).Get().(bool)
	}

	tests := []struct {
		name  string
		setup func(m *model)
	}{
		{
			name: "InitialState",
			setup: func(m *model) {
				m.ready = true
				m.state = StatePaused
			},
		},
		{
			name: "RunningWithAgents",
			setup: func(m *model) {
				m.ready = true
				m.state = StateRunning
				m.quotaLoaded = true
				m.quotas = map[string]AccountQuota{
					"flash": {Flash: 80, Pro: 50},
					"pro":   {Flash: 100, Pro: 20},
				}
				// Add tasks
				m.tasks = []Task{
					{ID: "task-1", Title: "Fix bug", Status: "in_progress"},
					{ID: "task-2", Title: "Add feature", Status: "ready"},
				}
				// Activity
				m.addActivity("🚀 Orchestrator started")
				m.addActivity("⚡ Starting task-1")
			},
		},
		{
			name: "QuotaLevels",
			setup: func(m *model) {
				m.ready = true
				m.quotaLoaded = true
				m.quotas = map[string]AccountQuota{
					"high":   {Flash: 100, Pro: 100},
					"medium": {Flash: 50, Pro: 50},
					"low":    {Flash: 5, Pro: 5},
					"error":  {Flash: -1, Pro: -1},
				}
			},
		},
		{
			name: "AgentGrid1",
			setup: func(m *model) {
				m.ready = true
				// 1 agent is default
			},
		},
		{
			name: "AgentGrid2",
			setup: func(m *model) {
				m.ready = true
				m.agentGrid.AddCard(agentgrid.NewAgentCard("CoderAgent-2", agentgrid.StatusIdle, "", nil))
			},
		},
		{
			name: "AgentGrid3",
			setup: func(m *model) {
				m.ready = true
				for i := 2; i <= 3; i++ {
					m.agentGrid.AddCard(agentgrid.NewAgentCard(fmt.Sprintf("CoderAgent-%d", i), agentgrid.StatusIdle, "", nil))
				}
			},
		},
		{
			name: "EmptyTasks",
			setup: func(m *model) {
				m.ready = true
				m.tasks = []Task{}
			},
		},
		{
			name: "StatePaused",
			setup: func(m *model) {
				m.ready = true
				m.state = StatePaused
			},
		},
		{
			name: "StateRunning",
			setup: func(m *model) {
				m.ready = true
				m.state = StateRunning
			},
		},
		{
			name: "StateStopped",
			setup: func(m *model) {
				m.ready = true
				m.state = StateStopped
			},
		},
		{
			name: "Quota0",
			setup: func(m *model) {
				m.ready = true
				m.quotaLoaded = true
				m.quotas = map[string]AccountQuota{
					"exhausted": {Flash: 0, Pro: 0},
				}
			},
		},
		{
			name: "Quota50",
			setup: func(m *model) {
				m.ready = true
				m.quotaLoaded = true
				m.quotas = map[string]AccountQuota{
					"half": {Flash: 50, Pro: 50},
				}
			},
		},
		{
			name: "Quota100",
			setup: func(m *model) {
				m.ready = true
				m.quotaLoaded = true
				m.quotas = map[string]AccountQuota{
					"full": {Flash: 100, Pro: 100},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := initialModel(nil, false)
			m.width = 120
			m.height = 40

			if tt.setup != nil {
				tt.setup(&m)
			}

			tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(m.width, m.height))
			tm.Send(tea.Quit())
			tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
			out, err := io.ReadAll(tm.FinalOutput(t))
			if err != nil {
				t.Fatal(err)
			}

			goldenPath := filepath.Join("testdata", "golden", tt.name+".golden")
			if update {
				err := os.WriteFile(goldenPath, out, 0644)
				if err != nil {
					t.Fatalf("failed to update golden file: %v", err)
				}
			}

			expected, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("failed to read golden file %s: %v", goldenPath, err)
			}

			if string(out) != string(expected) {
				t.Errorf("output does not match golden file %s", goldenPath)
			}
		})
	}
}
