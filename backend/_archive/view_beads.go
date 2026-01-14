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
		// Still show tab bar even if empty
		for i, lt := range listTypes {
			if i == t.beadsListType {
				content += fmt.Sprintf(" [white::r] %s (%d) [-:-:-]", lt.name, lt.count)
			} else {
				content += fmt.Sprintf(" [gray]%s (%d)[-]", lt.name, lt.count)
			}
		}
		content += fmt.Sprintf("\n\n[gray]No %s tasks[-]\n", strings.ToLower(listName))
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

	// Tab bar showing list types
	for i, lt := range listTypes {
		if i == t.beadsListType {
			content += fmt.Sprintf(" [white::r] %s (%d) [-:-:-]", lt.name, lt.count)
		} else {
			content += fmt.Sprintf(" [gray]%s (%d)[-]", lt.name, lt.count)
		}
	}
	content += "\n\n"

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

// navigateBeadDetail moves to prev/next bead in detail view
func (t *TUI) navigateBeadDetail(delta int) {
	currentID := strings.TrimPrefix(t.logFilter, "beads:")
	tasks := t.getBeadsListTasks()
	if len(tasks) == 0 {
		return
	}

	// Find current index
	currentIdx := -1
	for i, task := range tasks {
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
	if newIdx >= len(tasks) {
		newIdx = len(tasks) - 1
	}

	// Navigate to new task
	if newIdx != currentIdx {
		t.logFilter = "beads:" + tasks[newIdx].ID
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
		return fmt.Sprintf(" [red]Task not found: %s[-]", taskID)
	}

	var content string
	pad := " " // 1 char left padding

	// Core info
	content += pad + fmt.Sprintf("[white]ID:[-]       %s\n", task.ID)
	content += pad + fmt.Sprintf("[white]Title:[-]    %s\n", task.Title)
	content += pad + fmt.Sprintf("[white]Status:[-]   %s\n", task.Status)

	if task.IssueType != "" {
		content += pad + fmt.Sprintf("[white]Type:[-]     %s\n", task.IssueType)
	}

	content += pad + fmt.Sprintf("[white]Priority:[-] %d\n", task.Priority)

	complexity := "simple"
	if task.IsComplex {
		complexity = "complex"
	}
	content += pad + fmt.Sprintf("[white]Challenge:[-] %s\n", complexity)

	// Assignment
	if task.Assignee != "" {
		content += pad + fmt.Sprintf("[white]Assignee:[-] %s\n", task.Assignee)
	}
	if task.Owner != "" {
		content += pad + fmt.Sprintf("[white]Owner:[-]    %s\n", task.Owner)
	}
	if task.EstimatedMinutes != nil {
		content += pad + fmt.Sprintf("[white]Estimate:[-] %dm\n", *task.EstimatedMinutes)
	}

	// Timestamps
	content += "\n" + pad + "[yellow]Timestamps[-]\n"
	if !task.CreatedAt.IsZero() {
		content += pad + fmt.Sprintf("  [gray]Created:[-]  %s", task.CreatedAt.Format("2006-01-02 15:04"))
		if task.CreatedBy != "" {
			content += fmt.Sprintf(" by %s", task.CreatedBy)
		}
		content += "\n"
	}
	if !task.UpdatedAt.IsZero() {
		content += pad + fmt.Sprintf("  [gray]Updated:[-]  %s\n", task.UpdatedAt.Format("2006-01-02 15:04"))
	}
	if task.ClosedAt != nil {
		content += pad + fmt.Sprintf("  [gray]Closed:[-]   %s", task.ClosedAt.Format("2006-01-02 15:04"))
		if task.CloseReason != "" {
			content += fmt.Sprintf(" (%s)", task.CloseReason)
		}
		content += "\n"
	}
	if task.DueAt != nil {
		content += pad + fmt.Sprintf("  [gray]Due:[-]      %s\n", task.DueAt.Format("2006-01-02 15:04"))
	}
	if task.DeferUntil != nil {
		content += pad + fmt.Sprintf("  [gray]Deferred:[-] until %s\n", task.DeferUntil.Format("2006-01-02 15:04"))
	}

	// Relations
	if len(task.Labels) > 0 {
		content += "\n" + pad + fmt.Sprintf("[white]Labels:[-] %s\n", strings.Join(task.Labels, ", "))
	}
	if len(task.BlockedBy) > 0 {
		content += pad + fmt.Sprintf("[white]Blocked by:[-] %s\n", strings.Join(task.BlockedBy, ", "))
	}

	// Description
	content += "\n" + pad + "[yellow]Description[-]\n"
	content += pad + strings.Repeat("─", 11) + "\n"
	if task.Description != "" {
		content += pad + strings.ReplaceAll(task.Description, "\n", "\n"+pad) + "\n"
	} else {
		content += pad + "[gray]No description[-]\n"
	}

	// Design
	if task.Design != "" {
		content += "\n" + pad + "[yellow]Design[-]\n"
		content += pad + strings.Repeat("─", 6) + "\n"
		content += pad + strings.ReplaceAll(task.Design, "\n", "\n"+pad) + "\n"
	}

	// Acceptance Criteria
	if task.AcceptanceCriteria != "" {
		content += "\n" + pad + "[yellow]Acceptance Criteria[-]\n"
		content += pad + strings.Repeat("─", 19) + "\n"
		content += pad + strings.ReplaceAll(task.AcceptanceCriteria, "\n", "\n"+pad) + "\n"
	}

	// Notes
	if task.Notes != "" {
		content += "\n" + pad + "[yellow]Notes[-]\n"
		content += pad + strings.Repeat("─", 5) + "\n"
		content += pad + strings.ReplaceAll(task.Notes, "\n", "\n"+pad) + "\n"
	}

	// Comments
	if len(task.Comments) > 0 {
		content += "\n" + pad + fmt.Sprintf("[yellow]Comments (%d)[-]\n", len(task.Comments))
		content += pad + strings.Repeat("─", 12) + "\n"
		for _, comment := range task.Comments {
			author := comment.Author
			if author == "" {
				author = "anonymous"
			}
			content += pad + fmt.Sprintf("[blue]%s[-] [gray]%s[-]\n", author, comment.CreatedAt.Format("2006-01-02 15:04"))
			content += pad + "  " + strings.ReplaceAll(comment.Content, "\n", "\n"+pad+"  ") + "\n\n"
		}
	}

	return content
}
