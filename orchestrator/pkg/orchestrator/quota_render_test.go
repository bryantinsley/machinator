package orchestrator

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderQuotaBar(t *testing.T) {
	m := model{}

	tests := []struct {
		name     string
		percent  int
		width    int
		contains string
	}{
		{"100%", 100, 10, "█" + strings.Repeat("█", 9)},
		{"0%", 0, 10, strings.Repeat("░", 10)},
		{"50%", 50, 10, "█" + strings.Repeat("█", 4) + strings.Repeat("░", 5)},
		{"Negative/Error", -1, 10, strings.Repeat("░", 10)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := m.renderQuotaBar(tt.percent, tt.width)
			// Strip ANSI codes to check content
			plain := ansi.Strip(rendered)
			if !strings.Contains(plain, tt.contains) {
				t.Errorf("renderQuotaBar(%d, %d) = %q, want it to contain %q", tt.percent, tt.width, plain, tt.contains)
			}
		})
	}
}

func TestGetQuotaStyle(t *testing.T) {
	m := model{}

	// 100% should be Green
	s100 := m.getQuotaStyle(100)
	if s100.GetForeground() == nil {
		t.Errorf("Expected foreground color for 100%%")
	}

	// 0% should be Red
	s0 := m.getQuotaStyle(0)
	if s0.GetForeground() == nil {
		t.Errorf("Expected foreground color for 0%%")
	}
}
