package orchestrator

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/bryantinsley/machinator/orchestrator/pkg/ui/agentgrid"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func TestTUI_Golden(t *testing.T) {
	tests := []struct {
		name  string
		setup func(m *model)
	}{
		{
			name: "TestInitialStateGolden",
			setup: func(m *model) {
				m.ready = true
				m.state = StatePaused
			},
		},
		{
			name: "TestRunningWithAgentsGolden",
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
			name: "TestQuotaLevelsGolden",
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
			name: "TestAgentGridMultiGolden",
			setup: func(m *model) {
				m.ready = true
				// Add 3 agents
				for i := 2; i <= 3; i++ {
					m.agentGrid.AddCard(agentgrid.NewAgentCard(fmt.Sprintf("CoderAgent-%d", i), agentgrid.StatusIdle, "", nil))
				}
			},
		},
		{
			name: "TestEmptyTasksGolden",
			setup: func(m *model) {
				m.ready = true
				m.tasks = []Task{}
			},
		},
		{
			name: "TestStatePausedGolden",
			setup: func(m *model) {
				m.ready = true
				m.state = StatePaused
			},
		},
		{
			name: "TestStateRunningGolden",
			setup: func(m *model) {
				m.ready = true
				m.state = StateRunning
			},
		},
		{
			name: "TestStateStoppedGolden",
			setup: func(m *model) {
				m.ready = true
				m.state = StateStopped
			},
		},
		{
			name: "TestQuota0Golden",
			setup: func(m *model) {
				m.ready = true
				m.quotaLoaded = true
				m.quotas = map[string]AccountQuota{
					"exhausted": {Flash: 0, Pro: 0},
				}
			},
		},
		{
			name: "TestQuota50Golden",
			setup: func(m *model) {
				m.ready = true
				m.quotaLoaded = true
				m.quotas = map[string]AccountQuota{
					"half": {Flash: 50, Pro: 50},
				}
			},
		},
		{
			name: "TestQuota100Golden",
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
			// Mock dimensions for stable output
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
			teatest.RequireEqualOutput(t, out)
		})
	}
}
