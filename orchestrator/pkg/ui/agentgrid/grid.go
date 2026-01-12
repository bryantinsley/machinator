package agentgrid

import (
	"github.com/bryantinsley/machinator/orchestrator/pkg/ui/components"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type AgentGrid struct {
	Cards      []*AgentCard
	Cols       int
	FocusIndex int
	Dispatcher *components.ClickDispatcher

	width, height int
}

func NewAgentGrid(cards []*AgentCard, cols int) *AgentGrid {
	grid := &AgentGrid{
		Cards:      cards,
		Cols:       cols,
		FocusIndex: 0,
	}

	// Create dispatcher with cards as clickables
	clickables := make([]components.Clickable, len(cards))
	for i, c := range cards {
		clickables[i] = c
	}
	grid.Dispatcher = components.NewClickDispatcher(clickables)

	// Set initial focus
	if len(cards) > 0 {
		cards[0].SetFocused(true)
	}

	return grid
}

func (g *AgentGrid) AddCard(card *AgentCard) {
	g.Cards = append(g.Cards, card)
	// Update dispatcher
	g.updateDispatcher()
}

func (g *AgentGrid) RemoveCard(index int) {
	if index >= 0 && index < len(g.Cards) {
		g.Cards = append(g.Cards[:index], g.Cards[index+1:]...)
		g.updateDispatcher()
		// Adjust focus if needed
		if g.FocusIndex >= len(g.Cards) && len(g.Cards) > 0 {
			g.FocusIndex = len(g.Cards) - 1
			g.Cards[g.FocusIndex].SetFocused(true)
		} else if len(g.Cards) == 0 {
			g.FocusIndex = 0
		}
	}
}

func (g *AgentGrid) updateDispatcher() {
	clickables := make([]components.Clickable, len(g.Cards))
	for i, c := range g.Cards {
		clickables[i] = c
	}
	g.Dispatcher = components.NewClickDispatcher(clickables)
}

func (g *AgentGrid) SetSize(width, height int) {
	g.width = width
	g.height = height
	// Recalculate layout if needed (flexible cols?)
	// For now fixed cols
}

func (g *AgentGrid) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "right", "l":
			g.MoveFocus(1)
		case "left", "h":
			g.MoveFocus(-1)
		case "down", "j":
			g.MoveFocus(g.Cols)
		case "up", "k":
			g.MoveFocus(-g.Cols)
		case "enter":
			if g.FocusIndex >= 0 && g.FocusIndex < len(g.Cards) {
				return g, g.Cards[g.FocusIndex].HandleClick(0, 0) // Simulate click
			}
		}
	case tea.MouseMsg:
		return g, g.Dispatcher.HandleMouse(msg)
	}
	return g, nil
}

func (g *AgentGrid) MoveFocus(delta int) {
	if len(g.Cards) == 0 {
		return
	}

	newIndex := g.FocusIndex + delta
	if newIndex >= 0 && newIndex < len(g.Cards) {
		g.Cards[g.FocusIndex].SetFocused(false)
		g.FocusIndex = newIndex
		g.Cards[g.FocusIndex].SetFocused(true)
	}
}

func (g *AgentGrid) View() string {
	if len(g.Cards) == 0 {
		return "No agents"
	}

	// Layout cards
	// We need to render them to know their size?
	// Or we assume fixed size from style?
	// CardStyle has Width(30), Height(10).
	// + padding/border.

	var views []string
	currentRow := []string{}

	for i, card := range g.Cards {
		// Update card position for click detection
		// This is tricky because lipgloss joins strings, it doesn't place them at absolute coords.
		// We need to calculate where they WILL be.
		// Assuming fixed grid:
		// x = (i % cols) * (cardWidth + gap)
		// y = (i / cols) * (cardHeight + gap)
		// But render sizes include borders etc.
		// Let's assume approx size for now or render and measure.

		rendered := card.Render()
		currentRow = append(currentRow, rendered)

		if (i+1)%g.Cols == 0 || i == len(g.Cards)-1 {
			views = append(views, lipgloss.JoinHorizontal(lipgloss.Top, currentRow...))
			currentRow = []string{}
		}

		// Note: We are NOT updating card.x/y here accurately for click detection
		// because we don't know the parent offset or exact layout without measuring.
		// For TUI click handling usually requires passing X/Y offset down or doing absolute layout.
		// For this task (Design/Prototype), we might skip precise mouse click coords unless we do absolute layout.
		// Let's rely on keyboard navigation primarily for V1.
	}

	return lipgloss.JoinVertical(lipgloss.Left, views...)
}

func (g *AgentGrid) Init() tea.Cmd {
	return nil
}
