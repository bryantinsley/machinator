package agentgrid

import (
	"testing"
)

func TestAgentGrid_AddRemoveCard(t *testing.T) {
	// Initialize with one card
	card1 := NewAgentCard("Agent-1", StatusIdle, "", nil)
	grid := NewAgentGrid([]*AgentCard{card1}, 2)

	if len(grid.Cards) != 1 {
		t.Errorf("Expected 1 card, got %d", len(grid.Cards))
	}

	// Add a card
	card2 := NewAgentCard("Agent-2", StatusIdle, "", nil)
	grid.AddCard(card2)

	if len(grid.Cards) != 2 {
		t.Errorf("Expected 2 cards, got %d", len(grid.Cards))
	}

	if grid.Cards[1].Name != "Agent-2" {
		t.Errorf("Expected second card to be Agent-2, got %s", grid.Cards[1].Name)
	}

	// Remove a card
	grid.RemoveCard(1)

	if len(grid.Cards) != 1 {
		t.Errorf("Expected 1 card after removal, got %d", len(grid.Cards))
	}

	if grid.Cards[0].Name != "Agent-1" {
		t.Errorf("Expected remaining card to be Agent-1, got %s", grid.Cards[0].Name)
	}

	// Remove last card
	grid.RemoveCard(0)
	if len(grid.Cards) != 0 {
		t.Errorf("Expected 0 cards, got %d", len(grid.Cards))
	}
}

func TestAgentGrid_RemoveInvalidIndex(t *testing.T) {
	card1 := NewAgentCard("Agent-1", StatusIdle, "", nil)
	grid := NewAgentGrid([]*AgentCard{card1}, 2)

	grid.RemoveCard(5) // Invalid index

	if len(grid.Cards) != 1 {
		t.Errorf("Expected 1 card after invalid removal, got %d", len(grid.Cards))
	}

	grid.RemoveCard(-1) // Invalid index

	if len(grid.Cards) != 1 {
		t.Errorf("Expected 1 card after invalid removal, got %d", len(grid.Cards))
	}
}
