package agentgrid

import (
	"fmt"

	"github.com/bryantinsley/machinator/orchestrator/pkg/ui/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type AgentStatus string

const (
	StatusActive AgentStatus = "Active"
	StatusIdle   AgentStatus = "Idle"
	StatusError  AgentStatus = "Error"
)

// AgentCard represents a single agent in the grid
type AgentCard struct {
	Name    string
	Status  AgentStatus
	Task    string
	Elapsed string // Time elapsed on current task
	OnClick func() tea.Cmd

	x, y          int
	width, height int
	focused       bool
}

// NewAgentCard creates a new agent card
func NewAgentCard(name string, status AgentStatus, task string, onClick func() tea.Cmd) *AgentCard {
	return &AgentCard{
		Name:    name,
		Status:  status,
		Task:    task,
		OnClick: onClick,
	}
}

// SetFocused sets the focus state
func (c *AgentCard) SetFocused(focused bool) {
	c.focused = focused
}

// Clickable Implementation

func (c *AgentCard) Contains(x, y int) bool {
	return x >= c.x && x < c.x+c.width && y >= c.y && y < c.y+c.height
}

func (c *AgentCard) HandleClick(x, y int) tea.Cmd {
	if c.OnClick != nil {
		return c.OnClick()
	}
	return nil
}

func (c *AgentCard) Bounds() (x, y, width, height int) {
	return c.x, c.y, c.width, c.height
}

func (c *AgentCard) SetBounds(x, y, width, height int) {
	c.x = x
	c.y = y
	c.width = width
	c.height = height
}

// Render renders the card
func (c *AgentCard) Render() string {
	var style lipgloss.Style
	if c.focused {
		style = styles.CardFocusedStyle
	} else {
		style = styles.CardStyle
	}

	// Status Icon/Color
	var statusStyle lipgloss.Style
	icon := "●"
	switch c.Status {
	case StatusActive:
		statusStyle = styles.StatusActiveStyle
		icon = "⚡"
	case StatusError:
		statusStyle = styles.StatusErrorStyle
		icon = "✖"
	default:
		statusStyle = styles.StatusIdleStyle
		icon = "zzz"
	}

	header := fmt.Sprintf("%s %s", statusStyle.Render(icon), c.Name)

	// Handle task and elapsed time display
	task := c.Task
	if task == "" {
		task = "(no task)"
	} else {
		task = "Task: " + task
	}

	elapsed := ""
	if c.Elapsed != "" {
		elapsed = fmt.Sprintf(" (%s)", c.Elapsed)
	}

	// Total limit for the task line
	const maxLen = 36
	if len(task)+len(elapsed) > maxLen {
		// Truncate task to fit elapsed time
		availableForTask := maxLen - len(elapsed)
		if availableForTask < 10 {
			availableForTask = 10 // Minimum task visibility
		}
		if len(task) > availableForTask {
			task = task[:availableForTask-3] + "..."
		}
	}
	content := fmt.Sprintf("%s\n\n%s%s", header, task, elapsed)

	rendered := style.Render(content)
	c.width = lipgloss.Width(rendered)
	c.height = lipgloss.Height(rendered)

	return rendered
}
