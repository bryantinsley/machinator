package tui

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
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

	// Cached git log (refresh every 30s)
	cachedGitLog     []string
	cachedGitLogTime time.Time

	mu sync.Mutex
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

	// Don't block on beads - refresh loop will load them

	t.app = tview.NewApplication()

	// Left pane: quota and agents
	t.leftPane = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)
	t.leftPane.SetBorder(true).SetTitle(" Status ")
	t.leftPane.SetText("[gray]Loading...[-]")

	// Right pane: logs
	t.rightPane = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	t.rightPane.SetBorder(true).SetTitle(" [a] Assignment Log ")

	// Help bar (starts paused)
	t.helpBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	t.helpBar.SetText("[yellow]PAUSED[-]  [[0-9]] Agent  [[A]]ssign  [[B]]eads  [[+]] Add  [[S]]tart  [[Q]]uit")

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
	// Start refresh goroutine - it will populate content immediately
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
	// CRITICAL: This runs on the main tview goroutine.
	// Do NOT call any function that acquires a lock or does I/O.
	// Do NOT use QueueUpdate - we're already on the main goroutine.

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
		// Set paused in goroutine to avoid blocking
		go t.state.SetPaused(true)
		t.updateHelpBar()
	case 's', 'S':
		go t.state.SetPaused(false)
		t.updateHelpBar()
	case 'a', 'A':
		t.logFilter = "assign"
		t.rightPane.SetTitle(" [a] Assignment Log ")
	case 'b', 'B':
		t.logFilter = "beads"
		t.rightPane.SetTitle(" [b] Beads Status ")
	case '+', '=':
		go t.state.AddAgent()
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		agentNum := int(event.Rune() - '0')
		t.logFilter = fmt.Sprintf("agent-%d", agentNum)
		t.rightPane.SetTitle(fmt.Sprintf(" [%d] Agent %d Log ", agentNum, agentNum))
	}
	return event
}

func (t *TUI) updateHelpBar() {
	var text string
	if t.confirmQuit {
		text = "[red]Quit? (y/n)[-]"
	} else if t.state.AssignmentPaused {
		text = "[yellow]PAUSED[-]  [[0-9]] Agent  [[A]]ssign  [[B]]eads  [[+]] Add  [[S]]tart  [[Q]]uit"
	} else {
		text = "[green]RUNNING[-]  [[0-9]] Agent  [[A]]ssign  [[B]]eads  [[+]] Add  [[P]]ause  [[Q]]uit"
	}
	t.helpBar.SetText(text)
}

func (t *TUI) refreshLoop() {
	// Do initial refresh immediately
	t.doRefresh()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		t.doRefresh()
	}
}

func (t *TUI) doRefresh() {
	// Build content outside of any locks
	leftContent := t.buildLeftContent()
	rightContent := t.buildRightContent()

	// QueueUpdateDraw is non-blocking
	t.app.QueueUpdateDraw(func() {
		t.leftPane.SetText(leftContent)
		t.rightPane.SetText(rightContent)
	})
}

func (t *TUI) buildLeftContent() string {
	var content string

	// Load beads OUTSIDE of any locks (this does I/O)
	var newTasks []*beads.Task
	shouldRefreshBeads := time.Since(t.cachedTasksTime) > 15*time.Second
	if shouldRefreshBeads {
		newTasks = t.loadTasksWithTimeout(2 * time.Second)
	}

	// Load git log OUTSIDE of any locks
	var newGitLog []string
	shouldRefreshGit := time.Since(t.cachedGitLogTime) > 30*time.Second
	if shouldRefreshGit {
		newGitLog = t.loadGitLog(5)
	}

	// Now acquire lock briefly to update cache and read state
	t.mu.Lock()
	if newTasks != nil {
		t.cachedTasks = newTasks
		t.cachedTasksTime = time.Now()
	}
	if newGitLog != nil {
		t.cachedGitLog = newGitLog
		t.cachedGitLogTime = time.Now()
	}
	// Copy data we need while holding lock
	cachedTasks := t.cachedTasks
	cachedGitLog := t.cachedGitLog
	t.mu.Unlock()

	// Quota section (sorted by account name) - quota has its own synchronization
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

	// Agents section - state has its own synchronization
	content += "\n[yellow]Agents[-]\n"
	content += "──────\n"

	// Build task lookup for titles
	taskTitles := make(map[string]string)
	for _, task := range cachedTasks {
		taskTitles[task.ID] = task.Title
	}

	if t.state != nil {
		for _, agent := range t.state.Agents {
			stateColor := "green"
			if agent.State == "assigned" {
				stateColor = "blue"
			} else if agent.State == "pending" {
				stateColor = "yellow"
			}
			// Show elapsed time next to state if assigned
			elapsed := ""
			if agent.State == "assigned" && !agent.StartedAt.IsZero() {
				elapsed = fmt.Sprintf(" %s", time.Since(agent.StartedAt).Round(time.Second))
			}
			content += fmt.Sprintf("[white]%d:[-] [%s]%s[-]%s\n", agent.ID, stateColor, agent.State, elapsed)
			if agent.TaskID != "" {
				// Show short ID (part after last hyphen) and truncated title
				shortID := agent.TaskID
				if idx := strings.LastIndex(agent.TaskID, "-"); idx >= 0 {
					shortID = agent.TaskID[idx+1:]
				}
				title := taskTitles[agent.TaskID]
				if len(title) > 25 {
					title = title[:25] + "…"
				}
				content += fmt.Sprintf("   [gray]%s: %s[-]\n", shortID, title)
			}
		}
	}

	// Beads section - use the cached copy we made
	content += "\n[yellow]Beads[-]\n"
	content += "─────\n"

	if len(cachedTasks) == 0 {
		content += "[gray]No tasks[-]\n"
	} else {
		// Build closed set for blocking check
		closedIDs := make(map[string]bool)
		for _, task := range cachedTasks {
			if task.Status == "closed" {
				closedIDs[task.ID] = true
			}
		}

		ready, assigned, blocked, closed := 0, 0, 0, 0
		for _, task := range cachedTasks {
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

	// Recent commits section
	content += "\n[yellow]Commits[-]\n"
	content += "───────\n"
	if len(cachedGitLog) == 0 {
		content += "[gray]No commits[-]\n"
	} else {
		for _, line := range cachedGitLog {
			content += fmt.Sprintf("[gray]%s[-]\n", line)
		}
	}

	return content
}

// loadGitLog returns the last N commit summaries using go-git
func (t *TUI) loadGitLog(n int) []string {
	repo, err := git.PlainOpen(t.repoDir)
	if err != nil {
		return nil
	}

	ref, err := repo.Head()
	if err != nil {
		return nil
	}

	iter, err := repo.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return nil
	}

	var commits []string
	count := 0
	iter.ForEach(func(c *object.Commit) error {
		if count >= n {
			return fmt.Errorf("done")
		}
		// Format: short hash + truncated message
		hash := c.Hash.String()[:7]
		msg := strings.Split(c.Message, "\n")[0]
		if len(msg) > 30 {
			msg = msg[:30] + "…"
		}
		commits = append(commits, fmt.Sprintf("%s %s", hash, msg))
		count++
		return nil
	})

	return commits
}

// loadTasksWithTimeout loads beads with a timeout to prevent blocking the UI
func (t *TUI) loadTasksWithTimeout(timeout time.Duration) []*beads.Task {
	type result struct {
		tasks []*beads.Task
		err   error
	}
	ch := make(chan result, 1)

	go func() {
		tasks, err := beads.LoadTasks(t.repoDir)
		ch <- result{tasks, err}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			return nil
		}
		return r.tasks
	case <-time.After(timeout):
		// Timeout - return nil, will use stale cache
		return nil
	}
}

func (t *TUI) buildRightContent() string {
	var content string

	// Check filter type BEFORE acquiring any locks
	logFilter := t.logFilter

	if logFilter == "beads" {
		// For beads view, use cached tasks (loaded by buildLeftContent)
		// This avoids doing I/O while holding locks
		t.mu.Lock()
		cachedTasks := t.cachedTasks
		t.mu.Unlock()

		if len(cachedTasks) == 0 {
			content = "[gray]No tasks loaded[-]"
		} else {
			ready := beads.ReadyTasks(cachedTasks)

			open := 0
			closed := 0
			inProgress := 0
			for _, task := range cachedTasks {
				switch task.Status {
				case "open":
					open++
				case "closed":
					closed++
				case "in_progress":
					inProgress++
				}
			}

			content += "[yellow]Task Summary[-]\n"
			content += fmt.Sprintf("Total: %d\n", len(cachedTasks))
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
		// Show filtered logs - need logMu for this
		t.logMu.Lock()
		logs := make([]LogEntry, len(t.logs))
		copy(logs, t.logs)
		t.logMu.Unlock()

		for _, entry := range logs {
			match := false
			switch logFilter {
			case "all":
				match = true
			case "assign":
				match = entry.Source == "assign" || entry.Source == "quota"
			default:
				match = entry.Source == logFilter
			}

			if match {
				timeStr := entry.Time.Format("15:04:05")
				content += fmt.Sprintf("[gray]%s[-] %s\n", timeStr, entry.Message)
			}
		}
	}

	return content
}
