package tui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/bryantinsley/machinator/backend/internal/beads"
	"github.com/bryantinsley/machinator/backend/internal/config"
	"github.com/bryantinsley/machinator/backend/internal/project"
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
	app          *tview.Application
	leftPane     *tview.TextView
	rightFlex    *tview.Flex
	rightHeader  *tview.TextView
	rightContent *tview.TextView
	helpBar      *tview.TextView

	state   *state.State
	quota   *quota.Quota
	repoDir string
	paused  bool // Orchestrator paused state

	logs          []LogEntry
	logMu         sync.Mutex
	logFilter     string // "assign", "beads", "beads:task-id", "git", "git:hash", "config"
	selectedIdx   int    // Current selection index in list views
	beadsListType int    // 0=ready, 1=blocked, 2=assigned, 3=closed
	confirmQuit   bool

	// Cached beads (refresh every 15s)
	cachedTasks     []*beads.Task
	cachedTasksTime time.Time

	// Cached git log (refresh every 30s) - stores raw data for responsive formatting
	cachedGitLog     []CommitInfo
	cachedGitLogTime time.Time

	// Config for displaying settings
	cfg               *config.Config
	projCfg           *project.Config
	projectConfigPath string

	// Cached panel dimensions for responsive truncation
	leftWidth   int
	rightWidth  int
	rightHeight int

	mu sync.Mutex
}

// New creates a new TUI.
func New(st *state.State, q *quota.Quota, repoDir string, cfg *config.Config, projCfg *project.Config, projectConfigPath string) *TUI {
	// Start paused by default
	st.AssignmentPaused = true

	t := &TUI{
		state:             st,
		quota:             q,
		repoDir:           repoDir,
		logFilter:         "assign",
		cfg:               cfg,
		projCfg:           projCfg,
		projectConfigPath: projectConfigPath,
	}

	// Don't block on beads - refresh loop will load them

	t.app = tview.NewApplication()

	// Left pane: quota and agents
	t.leftPane = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)
	t.leftPane.SetBorder(true).SetTitle(" Status ")
	t.leftPane.SetText("[gray]Loading...[-]")

	// Right pane: split into fixed header and scrollable content
	t.rightHeader = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)
	t.rightHeader.SetText("[yellow]Assignment Log[-]")

	t.rightContent = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)

	// Wrap right header + content in a flex
	t.rightFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(t.rightHeader, 2, 0, false). // 2 rows: title + separator
		AddItem(t.rightContent, 0, 1, true)
	t.rightFlex.SetBorder(true).SetTitle(" (A)ssignment Log ")

	// Help bar (starts paused)
	t.helpBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	t.helpBar.SetText("(A)ssign (B)eads (G)it (C)onfig  (+)Add (S)tart (Q)uit")

	// Layout
	mainFlex := tview.NewFlex().
		AddItem(t.leftPane, 0, 1, false).
		AddItem(t.rightFlex, 0, 2, true)

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(mainFlex, 0, 1, true).
		AddItem(t.helpBar, 1, 0, false)

	// Set dark blue-green tinted background on all elements
	bgColor := tcell.NewRGBColor(22, 26, 28) // #161a1c - very dark with blue-green tint
	t.leftPane.SetBackgroundColor(bgColor)
	t.rightHeader.SetBackgroundColor(bgColor)
	t.rightContent.SetBackgroundColor(bgColor)
	t.rightFlex.SetBackgroundColor(bgColor)
	t.helpBar.SetBackgroundColor(bgColor)
	mainFlex.SetBackgroundColor(bgColor)
	root.SetBackgroundColor(bgColor)

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

	t.logs = append(t.logs, LogEntry{
		Time:    time.Now(),
		Source:  source,
		Message: message,
	})

	// Trim if too long
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

	// Delegate screen-specific key handling
	// If handler returns nil, the key was handled - return nil
	// If handler returns event, key was NOT handled - continue to global handlers
	switch {
	case strings.HasPrefix(t.logFilter, "beads"):
		if handled := t.handleBeadsKey(event); handled == nil {
			return nil // Key was handled
		}
		// Key not handled by beads, fall through to global handlers
	case strings.HasPrefix(t.logFilter, "git"):
		if handled := t.handleGitKey(event); handled == nil {
			return nil // Key was handled
		}
		// Key not handled by git, fall through to global handlers
	}

	// Default key handling for views without custom handlers
	switch event.Key() {
	case tcell.KeyEnter:
		t.handleEnter()
		return nil
	case tcell.KeyEscape:
		t.handleEscape()
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
		t.selectedIdx = 0
		t.rightFlex.SetTitle(" (A)ssignment Log ")
	case 'b', 'B':
		t.logFilter = "beads"
		t.selectedIdx = 0
		t.rightFlex.SetTitle(" Beads! ")
	case 'g', 'G':
		t.logFilter = "git"
		t.selectedIdx = 0
		t.rightFlex.SetTitle(" (G)it Commits ")
	case 'c', 'C':
		t.logFilter = "config"
		t.selectedIdx = 0
		t.rightFlex.SetTitle(" (C)onfig ")
	case '+', '=':
		go t.state.AddAgent()
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		agentNum := int(event.Rune() - '0')
		t.logFilter = fmt.Sprintf("agent-%d", agentNum)
		t.selectedIdx = 0
		t.rightFlex.SetTitle(fmt.Sprintf(" [%d] Agent %d Log ", agentNum, agentNum))
	}
	return event
}

// handleEnter processes Enter key for list selection
func (t *TUI) handleEnter() {
	// Dispatch to appropriate view's selection handler
	switch {
	case t.logFilter == "beads":
		t.selectBeadItem()
	case t.logFilter == "git":
		t.selectGitItem()
	}
}

// handleEscape processes Escape key for back navigation
func (t *TUI) handleEscape() {
	// If in a detail view (contains ":"), go back to list
	if strings.Contains(t.logFilter, ":") {
		parts := strings.SplitN(t.logFilter, ":", 2)
		t.logFilter = parts[0]
		t.selectedIdx = 0
	}
}

// navigateDetail moves to prev/next item in detail view
func (t *TUI) navigateDetail(delta int) {
	if strings.HasPrefix(t.logFilter, "beads:") {
		t.navigateBeadDetail(delta)
	}
	// Add other detail navigations here as needed
}

func (t *TUI) updateHelpBar() {
	var text string
	if t.confirmQuit {
		text = "[red]Quit? (y/n)[-]"
	} else if t.state.AssignmentPaused {
		text = "(A)ssign (B)eads (G)it (C)onfig  (+)Add (S)tart (Q)uit"
	} else {
		text = "(A)ssign (B)eads (G)it (C)onfig  (+)Add (P)ause (Q)uit"
	}
	t.helpBar.SetText(text)
}

func (t *TUI) refreshLoop() {
	// Do initial refresh immediately
	t.doRefresh()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		t.doRefresh()
	}
}

func (t *TUI) doRefresh() {
	// Capture widths inside QueueUpdateDraw (must be on main goroutine)
	// Then build content with cached widths
	t.app.QueueUpdateDraw(func() {
		// Update cached dimensions
		_, _, lw, _ := t.leftPane.GetInnerRect()
		_, _, rw, rh := t.rightContent.GetInnerRect()
		t.leftWidth = lw
		t.rightWidth = rw
		t.rightHeight = rh
	})

	// Build content outside of main goroutine using cached widths
	leftContent := t.buildLeftContent()
	rightHeader := t.getRightHeader()
	rightContent := t.buildRightContent()

	// QueueUpdateDraw is non-blocking
	t.app.QueueUpdateDraw(func() {
		t.leftPane.SetText(leftContent)
		t.rightHeader.SetText(rightHeader)
		t.rightContent.SetText(rightContent)
		t.updateHelpBar()
	})
}

func (t *TUI) getRightHeader() string {
	switch {
	case strings.HasPrefix(t.logFilter, "beads"):
		if strings.Contains(t.logFilter, ":") {
			// Bead Detail - show short ID + title
			taskID := strings.TrimPrefix(t.logFilter, "beads:")
			t.mu.Lock()
			cachedTasks := t.cachedTasks
			t.mu.Unlock()

			// Find task for title
			var taskTitle string
			shortID := taskID
			if idx := strings.LastIndex(taskID, "-"); idx >= 0 {
				shortID = taskID[idx+1:]
			}
			for _, task := range cachedTasks {
				if task.ID == taskID {
					taskTitle = task.Title
					break
				}
			}

			title := fmt.Sprintf(" [yellow]%s[-] -- %s", shortID, taskTitle)
			titleLen := 1 + len(shortID) + 4 + len(taskTitle)
			hint := "[white]<esc>[gray] back [white]←[gray] prev [white]→[gray] next[-]"
			hintLen := 27
			padding := t.rightWidth - titleLen - hintLen
			if padding < 1 {
				padding = 1
			}
			return title + strings.Repeat(" ", padding) + hint + "\n[#333333]" + strings.Repeat("─", t.rightWidth) + "[-]"
		}
		// Beads list - show tabs with counts in header
		t.mu.Lock()
		cachedTasks := t.cachedTasks
		t.mu.Unlock()

		// Count tasks by category
		closedIDs := make(map[string]bool)
		for _, task := range cachedTasks {
			if task.Status == "closed" {
				closedIDs[task.ID] = true
			}
		}

		ready, blocked, assigned, closed := 0, 0, 0, 0
		for _, task := range cachedTasks {
			switch task.Status {
			case "open":
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

		counts := []struct {
			name  string
			count int
		}{
			{"Ready", ready},
			{"Blocked", blocked},
			{"Assigned", assigned},
			{"Closed", closed},
		}

		// Build tab bar
		var tabs string
		tabsLen := 0
		for i, c := range counts {
			if i == t.beadsListType {
				tabs += fmt.Sprintf(" [#AAAAAA::r] %s (%d) [-:-:-]", c.name, c.count)
				tabsLen += 3 + len(c.name) + 4 + len(fmt.Sprintf("%d", c.count)) // " X (%d) "
			} else {
				tabs += fmt.Sprintf(" [gray]%s (%d)[-]", c.name, c.count)
				tabsLen += 1 + len(c.name) + 3 + len(fmt.Sprintf("%d", c.count)) // " X (%d)"
			}
		}

		hint := "[white]←/→[gray] list [white]↑↓[gray] nav [white]⏎[gray] view[-]"
		hintLen := 26 // visual length of hint
		padding := t.rightWidth - tabsLen - hintLen
		if padding < 1 {
			padding = 1
		}
		return tabs + strings.Repeat(" ", padding) + hint + "\n[#333333]" + strings.Repeat("─", t.rightWidth) + "[-]"
	case strings.HasPrefix(t.logFilter, "git"):
		return "[yellow]Recent Commits[-]"
	case t.logFilter == "config":
		return "[yellow]Configuration[-]"
	case strings.HasPrefix(t.logFilter, "agent-"):
		return fmt.Sprintf("[yellow]Agent %s Log[-]", strings.TrimPrefix(t.logFilter, "agent-"))
	default:
		return "[yellow]Assignment Log[-]"
	}
}

func (t *TUI) buildRightContent() string {
	switch {
	case strings.HasPrefix(t.logFilter, "beads"):
		return t.buildBeadsView()
	case strings.HasPrefix(t.logFilter, "git"):
		return t.buildGitView()
	case t.logFilter == "config":
		return t.buildConfigView()
	default:
		return t.buildLogsView()
	}
}

// loadTasksWithTimeout loads beads with a timeout to prevent blocking the UI.
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
		return nil
	}
}
