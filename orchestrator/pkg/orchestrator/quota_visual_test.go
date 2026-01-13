package orchestrator

import (
	"fmt"
	"testing"
)

func TestVisualQuotaHearts(t *testing.T) {
	m := model{}
	percentages := []int{100, 90, 80, 70, 60, 50, 40, 30, 20, 10, 5, 0}

	fmt.Println("Visual check of renderQuotaHearts:")
	for _, p := range percentages {
		hearts := m.renderQuotaHearts(p, false)
		fmt.Printf("%3d%%: %s\n", p, hearts)
	}

	fmt.Println("\nVisual check of renderQuotaBar:")
	for _, p := range percentages {
		bar := m.renderQuotaBar(p, 10)
		fmt.Printf("%3d%%: %s\n", p, bar)
	}
}
