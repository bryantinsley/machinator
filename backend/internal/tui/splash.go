package tui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// StartupChecker displays a full-screen preflight check sequence.
type StartupChecker struct {
	app      *tview.Application
	textView *tview.TextView
	lines    []string
	bgColor  tcell.Color
}

// NewStartupChecker creates a new startup checker splash screen.
func NewStartupChecker() *StartupChecker {
	app := tview.NewApplication()
	bgColor := tcell.NewRGBColor(22, 26, 28) // Dark blue-green

	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	textView.SetBackgroundColor(bgColor)

	// Create layout with padding
	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 2, 0, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 4, 0, false).
			AddItem(textView, 0, 1, false).
			AddItem(nil, 4, 0, false), 0, 1, false).
		AddItem(nil, 2, 0, false)
	flex.SetBackgroundColor(bgColor)

	app.SetRoot(flex, true)

	return &StartupChecker{
		app:      app,
		textView: textView,
		lines:    []string{},
		bgColor:  bgColor,
	}
}

// Start begins the startup checker in a goroutine and returns immediately.
// Call the returned channel when startup is complete to close the splash.
func (s *StartupChecker) Start() chan struct{} {
	done := make(chan struct{})

	go func() {
		<-done
		time.Sleep(500 * time.Millisecond) // Brief pause to show final state
		s.app.Stop()
	}()

	go s.app.Run()
	time.Sleep(50 * time.Millisecond) // Let app initialize

	return done
}

// Check adds a checking message for a step.
func (s *StartupChecker) Check(step string) {
	s.lines = append(s.lines, fmt.Sprintf("[gray]▪ %s...[-]", step))
	s.updateDisplay()
}

// OK marks the current step as successful.
func (s *StartupChecker) OK() {
	if len(s.lines) > 0 {
		last := s.lines[len(s.lines)-1]
		// Replace gray bullet with green check
		s.lines[len(s.lines)-1] = "[green]✓[-]" + last[len("[gray]▪"):]
		s.lines[len(s.lines)-1] = s.lines[len(s.lines)-1][:len(s.lines[len(s.lines)-1])-len("[-]")-3] + "[-]"
		s.updateDisplay()
	}
}

// Fixed marks the current step as fixed (was wrong, now corrected).
func (s *StartupChecker) Fixed(action string) {
	if len(s.lines) > 0 {
		last := s.lines[len(s.lines)-1]
		// Replace with yellow fix indicator
		base := last[len("[gray]▪") : len(last)-len("...[-]")]
		s.lines[len(s.lines)-1] = fmt.Sprintf("[yellow]↻[-] %s [gray](%s)[-]", base, action)
		s.updateDisplay()
	}
}

// Error marks the current step as failed.
func (s *StartupChecker) Error(err string) {
	if len(s.lines) > 0 {
		last := s.lines[len(s.lines)-1]
		base := last[len("[gray]▪") : len(last)-len("...[-]")]
		s.lines[len(s.lines)-1] = fmt.Sprintf("[red]✗[-] %s [red](%s)[-]", base, err)
		s.updateDisplay()
	}
}

// Message adds an info message (not a check step).
func (s *StartupChecker) Message(msg string) {
	s.lines = append(s.lines, fmt.Sprintf("[cyan]  %s[-]", msg))
	s.updateDisplay()
}

func (s *StartupChecker) updateDisplay() {
	content := "\n[yellow::b]Machinator Preflight[-:-:-]\n\n"
	for _, line := range s.lines {
		content += line + "\n"
	}
	s.app.QueueUpdateDraw(func() {
		s.textView.SetText(content)
	})
}
