package tui

import (
	"fmt"
	"strings"

	"github.com/bryantinsley/machinator/backend/internal/beads"
)

// buildBeadsView builds the beads status view for the right pane.
func (t *TUI) buildBeadsView() string {
	// Check if showing detail view
	if strings.HasPrefix(t.logFilter, "beads:") {
		taskID := strings.TrimPrefix(t.logFilter, "beads:")
		return t.buildBeadDetailView(taskID)
	}

	var content string

	t.mu.Lock()
	cachedTasks := t.cachedTasks
	t.mu.Unlock()

	if len(cachedTasks) == 0 {
		return "[gray]No tasks loaded[-]"
	}

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

	// Extract short IDs and find max width
	type readyTask struct {
		fullID     string
		shortID    string
		complexity string
		title      string
	}
	var tasks []readyTask
	maxIDLen := 0
	for _, task := range ready {
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
		tasks = append(tasks, readyTask{task.ID, shortID, complexity, task.Title})
	}

	content += "[yellow]Ready Tasks[-] (↑↓ navigate, Enter select)\n"

	// Clamp selectedIdx
	if t.selectedIdx >= len(tasks) {
		t.selectedIdx = len(tasks) - 1
	}
	if t.selectedIdx < 0 {
		t.selectedIdx = 0
	}

	// Calculate available width for title
	overhead := 4 + maxIDLen + 1 + 9 + 1 // ">> " or "   " + ID + space + (complexity) + space
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

// selectBeadItem handles Enter key on beads list
func (t *TUI) selectBeadItem() {
	t.mu.Lock()
	cachedTasks := t.cachedTasks
	t.mu.Unlock()

	ready := beads.ReadyTasks(cachedTasks)
	if t.selectedIdx >= 0 && t.selectedIdx < len(ready) {
		task := ready[t.selectedIdx]
		t.logFilter = "beads:" + task.ID
	}
}

// navigateBeadDetail moves to prev/next bead in detail view
func (t *TUI) navigateBeadDetail(delta int) {
	currentID := strings.TrimPrefix(t.logFilter, "beads:")

	t.mu.Lock()
	cachedTasks := t.cachedTasks
	t.mu.Unlock()

	ready := beads.ReadyTasks(cachedTasks)
	if len(ready) == 0 {
		return
	}

	// Find current index
	currentIdx := -1
	for i, task := range ready {
		if task.ID == currentID {
			currentIdx = i
			break
		}
	}

	if currentIdx < 0 {
		return
	}

	// Calculate new index
	newIdx := currentIdx + delta
	if newIdx < 0 {
		newIdx = 0
	}
	if newIdx >= len(ready) {
		newIdx = len(ready) - 1
	}

	// Navigate to new task
	if newIdx != currentIdx {
		t.logFilter = "beads:" + ready[newIdx].ID
		t.selectedIdx = newIdx
	}
}

// buildBeadDetailView shows details for a specific task
func (t *TUI) buildBeadDetailView(taskID string) string {
	t.mu.Lock()
	cachedTasks := t.cachedTasks
	t.mu.Unlock()

	// Find the task
	var task *beads.Task
	for _, tt := range cachedTasks {
		if tt.ID == taskID {
			task = tt
			break
		}
	}

	if task == nil {
		return fmt.Sprintf("[red]Task not found: %s[-]\n\n[gray]Press Escape to go back[-]", taskID)
	}

	var content string
	content += "[yellow]Task Details[-] (Escape to go back)\n"
	content += strings.Repeat("─", t.rightWidth) + "\n\n"

	content += fmt.Sprintf("[white]ID:[-]        %s\n", task.ID)
	content += fmt.Sprintf("[white]Title:[-]     %s\n", task.Title)
	content += fmt.Sprintf("[white]Status:[-]    %s\n", task.Status)

	complexity := "simple"
	if task.IsComplex {
		complexity = "complex"
	}
	content += fmt.Sprintf("[white]Complexity:[-] %s\n", complexity)

	if len(task.BlockedBy) > 0 {
		content += fmt.Sprintf("[white]Blocked by:[-] %s\n", strings.Join(task.BlockedBy, ", "))
	}

	content += "\n[yellow]Description[-]\n"
	content += strings.Repeat("─", 11) + "\n"
	if task.Description != "" {
		// Word wrap description
		content += task.Description + "\n"
	} else {
		content += "[gray]No description[-]\n"
	}

	return content
}
