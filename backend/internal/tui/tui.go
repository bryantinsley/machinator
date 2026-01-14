package tui

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/bryantinsley/machinator/backend/internal/beads"
	"github.com/bryantinsley/machinator/backend/internal/quota"
	"github.com/bryantinsley/machinator/backend/internal/state"
)

const maxLogLines = 500

// LogEntry represents a log line with context.
type LogEntry struct {
	Time    time.Time
	Source  string // "assign", "agent-1", "quota", etc.
	Message string
}

// TUI is the terminal user interface.
type TUI struct {
	app       *tview.Application
	leftPane  *tview.TextView
	rightPane *tview.TextView
	helpBar   *tview.TextView

	state   *state.State
	quota   *quota.Quota
	repoDir string

	logs        []LogEntry
	logMu       sync.Mutex
	logFilter   string // "all", "assign", "agent-N", "beads"
	confirmQuit bool

	// Cached beads (refresh every 15s)
	cachedTasks     []*beads.Task
	cachedTasksTime time.Time

	mu sync.RWMutex
}

// New creates a new TUI.
func New(st *state.State, q *quota.Quota, repoDir string) *TUI {
	// Start paused by default
	st.AssignmentPaused = true

	t := &TUI{
		state:     st,
		quota:     q,
		repoDir:   repoDir,
		logFilter: "assign",
	}

	t.app = tview.NewApplication()

	// Left pane: quota and agents
	t.leftPane = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)
	t.leftPane.SetBorder(true).SetTitle(" Status ")

	// Right pane: logs
	t.rightPane = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetChangedFunc(func() {
			t.app.Draw()
		})
	t.rightPane.SetBorder(true).SetTitle(" [a] Assignment Log ")

	// Help bar (starts paused)
	t.helpBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	t.helpBar.SetText("[yellow]PAUSED[-]  [0-9]Agent [a]Assign [b]Beads [s]Start [q]Quit")

	// Layout
	mainFlex := tview.NewFlex().
		AddItem(t.leftPane, 0, 1, false).
		AddItem(t.rightPane, 0, 2, true)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(mainFlex, 0, 1, true).
		AddItem(t.helpBar, 1, 0, false)

	t.app.SetRoot(root, true)
	t.app.SetInputCapture(t.handleInput)

	return t
}

// Run starts the TUI.
func (t *TUI) Run() error {
	// Start refresh goroutine
	go t.refreshLoop()
	return t.app.Run()
}

// Stop stops the TUI.
func (t *TUI) Stop() {
	t.app.Stop()
}

// Log adds a log entry.
func (t *TUI) Log(source, message string) {
	t.logMu.Lock()
	defer t.logMu.Unlock()

	entry := LogEntry{
		Time:    time.Now(),
		Source:  source,
		Message: message,
	}
	t.logs = append(t.logs, entry)
	if len(t.logs) > maxLogLines {
		t.logs = t.logs[len(t.logs)-maxLogLines:]
	}
}

func (t *TUI) handleInput(event *tcell.EventKey) *tcell.EventKey {
	// If in confirm mode, handle y/n
	if t.confirmQuit {
		switch event.Rune() {
		case 'y', 'Y':
			t.app.Stop()
			return nil
		case 'n', 'N':
			t.confirmQuit = false
			t.updateHelpBar()
		}
		return nil
	}

	switch event.Rune() {
	case 'q', 'Q', 'x', 'X':
		t.confirmQuit = true
		t.updateHelpBar()
		return nil
	case 'p', 'P':
		t.state.SetPaused(true)
		t.Log("assign", "[yellow]PAUSED[-] - press 's' to resume")
		t.updateHelpBar()
	case 's', 'S':
		t.state.SetPaused(false)
		t.Log("assign", "[green]RESUMED[-]")
		t.updateHelpBar()
	case 'a', 'A':
		t.logFilter = "assign"
		t.rightPane.SetTitle(" [a] Assignment Log ")
	case 'b', 'B':
		t.logFilter = "beads"
		t.rightPane.SetTitle(" [b] Beads Status ")
	case '+', '=':
		agent := t.state.AddAgent()
		t.Log("main", fmt.Sprintf("[blue]Added agent %d (pending)[-]", agent.ID))
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		agentNum := int(event.Rune() - '0')
		t.logFilter = fmt.Sprintf("agent-%d", agentNum)
		t.rightPane.SetTitle(fmt.Sprintf(" [%d] Agent %d Log ", agentNum, agentNum))
	}
	t.updateRightPane()
	return event
}

func (t *TUI) updateHelpBar() {
	var text string
	if t.confirmQuit {
		text = "[red]Quit? (y/n)[-]"
	} else if t.state.AssignmentPaused {
		text = "[yellow]PAUSED[-]  [0-9]Agent [a]Assign [b]Beads [+]Add [s]Start [q]Quit"
	} else {
		text = "[green]RUNNING[-] [0-9]Agent [a]Assign [b]Beads [+]Add [p]Pause [q]Quit"
	}
	t.app.QueueUpdateDraw(func() {
		t.helpBar.SetText(text)
	})
}

func (t *TUI) refreshLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		t.updateLeftPane()
		t.updateRightPane()
	}
}

func (t *TUI) updateLeftPane() {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var content string

	// Quota section (sorted by account name)
	content += "[yellow]Quota[-]\n"
	content += "─────\n"
	if t.quota != nil && len(t.quota.Accounts) > 0 {
		// Sort accounts by name
		accounts := make([]quota.AccountQuota, len(t.quota.Accounts))
		copy(accounts, t.quota.Accounts)
		sort.Slice(accounts, func(i, j int) bool {
			return accounts[i].Name < accounts[j].Name
		})

		for _, acc := range accounts {
			content += fmt.Sprintf("[white]%s[-]\n", acc.Name)
			// Show flash then pro (sorted order)
			for _, model := range []string{"gemini-3-flash-preview", "gemini-3-pro-preview"} {
				if remaining, ok := acc.Models[model]; ok {
					shortName := "flash"
					if model == "gemini-3-pro-preview" {
						shortName = "pro"
					}
					color := "green"
					if remaining < 0.3 {
						color = "red"
					} else if remaining < 0.7 {
						color = "yellow"
					}
					content += fmt.Sprintf("  %s: [%s]%.0f%%[-]\n", shortName, color, remaining*100)
				}
			}
		}
	} else {
		content += "[gray]No quota data[-]\n"
	}

	// Agents section
	content += "\n[yellow]Agents[-]\n"
	content += "──────\n"
	if t.state != nil {
		for _, agent := range t.state.Agents {
			stateColor := "green"
			if agent.State == "assigned" {
				stateColor = "blue"
			} else if agent.State == "pending" {
				stateColor = "yellow"
			}
			content += fmt.Sprintf("[white]%d:[-] [%s]%s[-]\n", agent.ID, stateColor, agent.State)
			if agent.TaskID != "" {
				elapsed := ""
				if !agent.StartedAt.IsZero() {
					elapsed = fmt.Sprintf(" (%s)", time.Since(agent.StartedAt).Round(time.Second))
				}
				content += fmt.Sprintf("   [gray]%s%s[-]\n", agent.TaskID, elapsed)
			}
		}
	}

	// Beads section (cached, refresh every 15s)
	content += "\n[yellow]Beads[-]\n"
	content += "─────\n"

	// Refresh cache if stale
	if time.Since(t.cachedTasksTime) > 15*time.Second {
		tasks, err := beads.LoadTasks(t.repoDir)
		if err == nil {
			t.cachedTasks = tasks
			t.cachedTasksTime = time.Now()
		}
	}

	if len(t.cachedTasks) == 0 {
		content += "[gray]No tasks[-]\n"
	} else {
		// Build closed set for blocking check
		closedIDs := make(map[string]bool)
		for _, task := range t.cachedTasks {
			if task.Status == "closed" {
				closedIDs[task.ID] = true
			}
		}

		ready, assigned, blocked, closed := 0, 0, 0, 0
		for _, task := range t.cachedTasks {
			switch task.Status {
			case "open":
				// Check if actually blocked (has unclosed blockers)
				isBlocked := false
				for _, blockerID := range task.BlockedBy {
					if !closedIDs[blockerID] {
						isBlocked = true
						break
					}
				}
				if isBlocked {
					blocked++
				} else {
					ready++
				}
			case "in_progress":
				assigned++
			case "closed":
				closed++
			}
		}
		content += fmt.Sprintf("ready:[green]%d[-] assigned:[blue]%d[-]\n", ready, assigned)
		content += fmt.Sprintf("blocked:[yellow]%d[-] closed:[gray]%d[-]\n", blocked, closed)
	}

	t.app.QueueUpdateDraw(func() {
		t.leftPane.SetText(content)
	})
}

func (t *TUI) updateRightPane() {
	t.logMu.Lock()
	defer t.logMu.Unlock()

	var content string

	if t.logFilter == "beads" {
		// Show beads status
		tasks, err := beads.LoadTasks(t.repoDir)
		if err != nil {
			content = fmt.Sprintf("[red]Error: %v[-]", err)
		} else {
			ready := beads.ReadyTasks(tasks)

			open := 0
			closed := 0
			inProgress := 0
			for _, task := range tasks {
				switch task.Status {
				case "open":
					open++
				case "closed":
					closed++
				case "in_progress":
					inProgress++
				}
			}

			content += fmt.Sprintf("[yellow]Task Summary[-]\n")
			content += fmt.Sprintf("Total: %d\n", len(tasks))
			content += fmt.Sprintf("Open: [green]%d[-]  In Progress: [blue]%d[-]  Closed: [gray]%d[-]\n", open, inProgress, closed)
			content += fmt.Sprintf("Ready for assignment: [white]%d[-]\n\n", len(ready))

			content += "[yellow]Ready Tasks[-]\n"
			for _, task := range ready {
				complexity := "simple"
				if task.IsComplex {
					complexity = "complex"
				}
				content += fmt.Sprintf("  [white]%s[-] [gray](%s)[-] %s\n", task.ID, complexity, task.Title)
			}
		}
	} else {
		// Show filtered logs
		for _, entry := range t.logs {
			match := false
			switch t.logFilter {
			case "all":
				match = true
			case "assign":
				match = entry.Source == "assign" || entry.Source == "quota"
			default:
				match = entry.Source == t.logFilter
			}

			if match {
				timeStr := entry.Time.Format("15:04:05")
				content += fmt.Sprintf("[gray]%s[-] %s\n", timeStr, entry.Message)
			}
		}
	}

	t.app.QueueUpdateDraw(func() {
		t.rightPane.SetText(content)
		t.rightPane.ScrollToEnd()
	})
}
