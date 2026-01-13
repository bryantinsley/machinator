package agentgrid

import (
	"strings"
	"testing"
)

func TestAgentCard_Render(t *testing.T) {
	tests := []struct {
		name     string
		card     *AgentCard
		contains []string
	}{
		{
			name: "Idle card",
			card: &AgentCard{
				Name:   "Agent-1",
				Status: StatusIdle,
			},
			contains: []string{"Agent-1", "zzz", "(no task)"},
		},
		{
			name: "Active card with task and elapsed time",
			card: &AgentCard{
				Name:    "Agent-1",
				Status:  StatusActive,
				Task:    "machinator-123",
				Elapsed: "2m 34s",
			},
			contains: []string{"Agent-1", "⚡", "Task: machinator-123 (2m 34s)"},
		},
		{
			name: "Error card",
			card: &AgentCard{
				Name:   "Agent-1",
				Status: StatusError,
				Task:   "machinator-123",
			},
			contains: []string{"Agent-1", "✖", "Task: machinator-123"},
		},
		{
			name: "Long task ID should not truncate elapsed time",
			card: &AgentCard{
				Name:    "Agent-1",
				Status:  StatusActive,
				Task:    "machinator-very-long-task-id-that-definitely-exceeds-the-limit",
				Elapsed: "2m 34s",
			},
			contains: []string{"2m 34s"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := tt.card.Render()
			// Remove lipgloss styling for easier matching
			plain := stripStyles(rendered)
			for _, c := range tt.contains {
				if !strings.Contains(plain, c) {
					t.Errorf("Expected rendered card to contain %q, but it didn't.\nGot:\n%s", c, plain)
				}
			}
		})
	}
}

// Simple helper to strip some lipgloss/ANSI escape codes if needed,
// but lipgloss.Render might not include them in tests if no terminal is detected.
// Actually, it's better to just check the string.
func stripStyles(s string) string {
	// This is a very crude stripper, might need improvement if tests fail due to colors
	return s
}
