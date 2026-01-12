package main

import (
	"fmt"
	"os"

	"github.com/bryantinsley/machinator/orchestrator/pkg/ui/agentgrid"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Create mock cards
	cards := []*agentgrid.AgentCard{
		agentgrid.NewAgentCard("Gemini-01", agentgrid.StatusActive, "Fixing bug #123", func() tea.Cmd {
			return tea.Println("Clicked Gemini-01")
		}),
		agentgrid.NewAgentCard("Gemini-02", agentgrid.StatusIdle, "", func() tea.Cmd {
			return tea.Println("Clicked Gemini-02")
		}),
		agentgrid.NewAgentCard("Gemini-03", agentgrid.StatusError, "Quota exceeded", func() tea.Cmd {
			return tea.Println("Clicked Gemini-03")
		}),
		agentgrid.NewAgentCard("Gemini-04", agentgrid.StatusActive, "Implementing feature X", func() tea.Cmd {
			return tea.Println("Clicked Gemini-04")
		}),
	}

	grid := agentgrid.NewAgentGrid(cards, 2)

	p := tea.NewProgram(grid, tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
