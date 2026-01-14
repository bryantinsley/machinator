package tui

import (
	"fmt"
	"strings"

	"github.com/bryantinsley/machinator/backend/internal/beads"
)

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

	// Core info - all labels gray, aligned at column 12
	content += pad + fmt.Sprintf("[gray]ID:[-]         %s\n", task.ID)
	content += pad + "[gray]Title:[-]      " + wrapText(task.Title, pad+"               ", t.rightWidth)[len(pad)+15:] + "\n"
	content += pad + fmt.Sprintf("[gray]Status:[-]     %s\n", task.Status)

	if task.IssueType != "" {
		content += pad + fmt.Sprintf("[gray]Type:[-]       %s\n", task.IssueType)
	}

	content += pad + fmt.Sprintf("[gray]Priority:[-]   %d\n", task.Priority)

	complexity := "simple"
	if task.IsComplex {
		complexity = "complex"
	}
	content += pad + fmt.Sprintf("[gray]Challenge:[-]  %s\n", complexity)

	// Assignment
	if task.Assignee != "" {
		content += pad + fmt.Sprintf("[gray]Assignee:[-]   %s\n", task.Assignee)
	}
	if task.Owner != "" {
		content += pad + fmt.Sprintf("[gray]Owner:[-]      %s\n", task.Owner)
	}
	if task.EstimatedMinutes != nil {
		content += pad + fmt.Sprintf("[gray]Estimate:[-]   %dm\n", *task.EstimatedMinutes)
	}

	// Timestamps (inline with other fields, no heading)
	if !task.CreatedAt.IsZero() {
		createdStr := task.CreatedAt.Format("2006-01-02 15:04")
		if task.CreatedBy != "" {
			createdStr += " by " + task.CreatedBy
		}
		content += pad + fmt.Sprintf("[gray]Created:[-]    %s\n", createdStr)
	}
	if !task.UpdatedAt.IsZero() {
		content += pad + fmt.Sprintf("[gray]Updated:[-]    %s\n", task.UpdatedAt.Format("2006-01-02 15:04"))
	}
	if task.ClosedAt != nil {
		content += pad + fmt.Sprintf("[gray]Closed:[-]     %s\n", task.ClosedAt.Format("2006-01-02 15:04"))
	}
	// Skip redundant close reasons
	skipReasons := task.CloseReason == "" ||
		strings.EqualFold(task.CloseReason, "Closed") ||
		strings.EqualFold(task.CloseReason, "Completed") ||
		strings.EqualFold(task.CloseReason, "Done") ||
		strings.EqualFold(task.CloseReason, "Finished") ||
		strings.EqualFold(task.CloseReason, "Resolved") ||
		strings.EqualFold(task.CloseReason, "Fixed") ||
		strings.EqualFold(task.CloseReason, "Complete") ||
		strings.EqualFold(task.CloseReason, "Shipped") ||
		strings.EqualFold(task.CloseReason, "Merged")
	if !skipReasons {
		content += pad + "[gray]Close Reason:[-] " + wrapText(task.CloseReason, pad+"                 ", t.rightWidth)[len(pad)+17:] + "\n"
	}
	if task.DueAt != nil {
		content += pad + fmt.Sprintf("[gray]Due:[-]        %s\n", task.DueAt.Format("2006-01-02 15:04"))
	}
	if task.DeferUntil != nil {
		content += pad + fmt.Sprintf("[gray]Deferred:[-]   until %s\n", task.DeferUntil.Format("2006-01-02 15:04"))
	}

	// Relations
	if len(task.Labels) > 0 {
		content += pad + fmt.Sprintf("[gray]Labels:[-]     %s\n", strings.Join(task.Labels, ", "))
	}
	if len(task.BlockedBy) > 0 {
		content += pad + fmt.Sprintf("[gray]Blocked by:[-] %s\n", strings.Join(task.BlockedBy, ", "))
	}

	// Description
	content += "\n" + pad + "[cyan]Description[-]\n"
	content += pad + strings.Repeat("─", 11) + "\n"
	if task.Description != "" {
		content += wrapText(task.Description, pad, t.rightWidth) + "\n"
	} else {
		content += pad + "[gray]No description[-]\n"
	}

	// Design
	if task.Design != "" {
		content += "\n" + pad + "[#CC99FF]Design[-]\n"
		content += pad + strings.Repeat("─", 6) + "\n"
		content += wrapText(task.Design, pad, t.rightWidth) + "\n"
	}

	// Acceptance Criteria
	if task.AcceptanceCriteria != "" {
		content += "\n" + pad + "[#66CCFF]Acceptance Criteria[-]\n"
		content += pad + strings.Repeat("─", 19) + "\n"
		content += wrapText(task.AcceptanceCriteria, pad, t.rightWidth) + "\n"
	}

	// Notes
	if task.Notes != "" {
		content += "\n" + pad + "[#99FFCC]Notes[-]\n"
		content += pad + strings.Repeat("─", 5) + "\n"
		content += wrapText(task.Notes, pad, t.rightWidth) + "\n"
	}

	// Comments
	if len(task.Comments) > 0 {
		content += "\n" + pad + fmt.Sprintf("[#FFCC66]Comments (%d)[-]\n", len(task.Comments))
		content += pad + strings.Repeat("─", 12) + "\n"
		for _, comment := range task.Comments {
			author := comment.Author
			if author == "" {
				author = "anonymous"
			}
			content += pad + fmt.Sprintf("[blue]%s[-] [gray]%s[-]\n", author, comment.CreatedAt.Format("2006-01-02 15:04"))
			content += wrapText(comment.Content, pad+"  ", t.rightWidth) + "\n\n"
		}
	}

	return content
}
