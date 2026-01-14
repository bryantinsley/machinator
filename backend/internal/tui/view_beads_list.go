package tui

import (
	"fmt"
	"strings"

	"github.com/bryantinsley/machinator/backend/internal/beads"
	"github.com/gdamore/tcell/v2"
)

// handleBeadsKey handles all key events for the beads view.
// Returns nil to indicate the key was handled, or returns event to pass through.
func (t *TUI) handleBeadsKey(event *tcell.EventKey) *tcell.EventKey {
	inDetailView := strings.Contains(t.logFilter, ":")

	switch event.Key() {
	case tcell.KeyUp:
		if inDetailView {
			// Let tview handle scrolling
			return event
		}
		if t.selectedIdx > 0 {
			t.selectedIdx--
		}
		return nil

	case tcell.KeyDown:
		if inDetailView {
			// Let tview handle scrolling
			return event
		}
		t.selectedIdx++
		return nil

	case tcell.KeyLeft:
		if inDetailView {
			// Navigate to previous bead
			t.navigateBeadDetail(-1)
		} else {
			// Cycle to previous list type
			t.beadsListType = (t.beadsListType + 3) % 4
			t.selectedIdx = 0
		}
		return nil

	case tcell.KeyRight:
		if inDetailView {
			// Navigate to next bead
			t.navigateBeadDetail(1)
		} else {
			// Cycle to next list type
			t.beadsListType = (t.beadsListType + 1) % 4
			t.selectedIdx = 0
		}
		return nil

	case tcell.KeyEnter:
		if !inDetailView {
			t.selectBeadItem()
		}
		return nil

	case tcell.KeyEscape:
		if inDetailView {
			// Go back to list view
			t.logFilter = "beads"
			// Keep selectedIdx so we're on the same item
		}
		return nil
	}

	return event // Pass through unhandled keys
}

// buildBeadsView builds the beads status view for the right pane.
func (t *TUI) buildBeadsView() string {
	// Check if showing detail view
	if strings.HasPrefix(t.logFilter, "beads:") {
		taskID := strings.TrimPrefix(t.logFilter, "beads:")
		return t.buildBeadDetailView(taskID)
	}

	// List view
	var content string

	t.mu.Lock()
	cachedTasks := t.cachedTasks
	t.mu.Unlock()

	if len(cachedTasks) == 0 {
		return "[gray]No tasks loaded[-]"
	}

	// Build categorized lists
	closedIDs := make(map[string]bool)
	for _, task := range cachedTasks {
		if task.Status == "closed" {
			closedIDs[task.ID] = true
		}
	}

	var readyTasks, blockedTasks, assignedTasks, closedTasks []*beads.Task
	for _, task := range cachedTasks {
		switch task.Status {
		case "open":
			// Check if blocked
			isBlocked := false
			for _, blockerID := range task.BlockedBy {
				if !closedIDs[blockerID] {
					isBlocked = true
					break
				}
			}
			if isBlocked {
				blockedTasks = append(blockedTasks, task)
			} else {
				readyTasks = append(readyTasks, task)
			}
		case "in_progress":
			assignedTasks = append(assignedTasks, task)
		case "closed":
			closedTasks = append(closedTasks, task)
		}
	}

	// Tab header with counts
	listTypes := []struct {
		name  string
		count int
		tasks []*beads.Task
	}{
		{"Ready", len(readyTasks), readyTasks},
		{"Blocked", len(blockedTasks), blockedTasks},
		{"Assigned", len(assignedTasks), assignedTasks},
		{"Closed", len(closedTasks), closedTasks},
	}

	// Get selected list
	selectedTasks := listTypes[t.beadsListType].tasks
	listName := listTypes[t.beadsListType].name

	if len(selectedTasks) == 0 {
		content += fmt.Sprintf("[gray]No %s tasks[-]\n", strings.ToLower(listName))
		return content
	}

	// Extract short IDs and find max width
	type taskEntry struct {
		fullID     string
		shortID    string
		complexity string
		title      string
	}
	var tasks []taskEntry
	maxIDLen := 0
	for _, task := range selectedTasks {
		shortID := task.ID
		if idx := strings.LastIndex(task.ID, "-"); idx >= 0 {
			shortID = task.ID[idx+1:]
		}
		if len(shortID) > maxIDLen {
			maxIDLen = len(shortID)
		}
		complexity := "simple"
		if task.IsComplex {
			complexity = "complex"
		}
		tasks = append(tasks, taskEntry{task.ID, shortID, complexity, task.Title})
	}

	// Clamp selectedIdx
	if t.selectedIdx >= len(tasks) {
		t.selectedIdx = len(tasks) - 1
	}
	if t.selectedIdx < 0 {
		t.selectedIdx = 0
	}

	// Calculate available width for title
	overhead := 4 + maxIDLen + 1 + 9 + 1
	titleWidth := t.rightWidth - overhead
	if titleWidth < 10 {
		titleWidth = 10
	}

	for i, task := range tasks {
		title := task.title
		if len(title) > titleWidth {
			title = title[:titleWidth-1] + "…"
		}

		// Highlight selected item
		prefix := "  "
		if i == t.selectedIdx {
			prefix = "[white::r]> "
			content += fmt.Sprintf("%s%-*s[-:-:-] [gray](%s)[-] %s\n", prefix, maxIDLen, task.shortID, task.complexity, title)
		} else {
			content += fmt.Sprintf("%s[white]%-*s[-] [gray](%s)[-] %s\n", prefix, maxIDLen, task.shortID, task.complexity, title)
		}
	}

	return content
}

// getBeadsListTasks returns the tasks for the current beadsListType
func (t *TUI) getBeadsListTasks() []*beads.Task {
	t.mu.Lock()
	cachedTasks := t.cachedTasks
	t.mu.Unlock()

	// Build closed set
	closedIDs := make(map[string]bool)
	for _, task := range cachedTasks {
		if task.Status == "closed" {
			closedIDs[task.ID] = true
		}
	}

	var readyTasks, blockedTasks, assignedTasks, closedTasks []*beads.Task
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
				blockedTasks = append(blockedTasks, task)
			} else {
				readyTasks = append(readyTasks, task)
			}
		case "in_progress":
			assignedTasks = append(assignedTasks, task)
		case "closed":
			closedTasks = append(closedTasks, task)
		}
	}

	switch t.beadsListType {
	case 0:
		return readyTasks
	case 1:
		return blockedTasks
	case 2:
		return assignedTasks
	case 3:
		return closedTasks
	default:
		return readyTasks
	}
}

// selectBeadItem handles Enter key on beads list
func (t *TUI) selectBeadItem() {
	tasks := t.getBeadsListTasks()
	if t.selectedIdx >= 0 && t.selectedIdx < len(tasks) {
		task := tasks[t.selectedIdx]
		t.logFilter = "beads:" + task.ID
	}
}
