package beads

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ╔══════════════════════════════════════════════════════════════════════════════╗
// ║ DO NOT REMOVE - UPSTREAM SYNCHRONIZATION NOTICE                              ║
// ╠══════════════════════════════════════════════════════════════════════════════╣
// ║ The Task struct below is aligned with the upstream beads Issue struct:       ║
// ║                                                                              ║
// ║ Source:https://github.com/steveyegge/beads/blob/main/internal/types/types.go ║
// ║                                                                              ║
// ║ HOW TO UPDATE:                                                               ║
// ║ 1. Check the upstream types.go for new fields in the Issue struct            ║
// ║ 2. Add any new fields to the Task struct below with matching JSON tags       ║
// ║ 3. Update buildBeadDetailView() in view_beads.go to display new fields       ║
// ║ 4. Test that JSON unmarshaling still works with older JSONL files            ║
// ║                                                                              ║
// ║ IMPORTANT FIELDS TO WATCH FOR:                                               ║
// ║ - Core: ID, Title, Description, Design, AcceptanceCriteria, Notes            ║
// ║ - Workflow: Status, Priority, IssueType                                      ║
// ║ - Assignment: Assignee, Owner, EstimatedMinutes                              ║
// ║ - Timestamps: CreatedAt, UpdatedAt, ClosedAt, DueAt, DeferUntil              ║
// ║ - Relations: Labels, BlockedBy (Dependencies), Comments                      ║
// ║ - Agent fields: AgentState, HookBead, RoleBead, etc. (if needed)             ║
// ║                                                                              ║
// ║ The Comment struct is also from upstream (types.Comment).                    ║
// ╚══════════════════════════════════════════════════════════════════════════════╝

// Task represents a bead task (aligned with steveyegge/beads Issue struct).
type Task struct {
	// Core identification
	ID string `json:"id"`

	// Issue content
	Title              string `json:"title"`
	Description        string `json:"description,omitempty"`
	Design             string `json:"design,omitempty"`
	AcceptanceCriteria string `json:"acceptance_criteria,omitempty"`
	Notes              string `json:"notes,omitempty"`

	// Status & workflow
	Status    string `json:"status,omitempty"`
	Priority  int    `json:"priority"`
	IssueType string `json:"issue_type,omitempty"`

	// Assignment
	Assignee         string `json:"assignee,omitempty"`
	Owner            string `json:"owner,omitempty"`
	EstimatedMinutes *int   `json:"estimated_minutes,omitempty"`

	// Timestamps
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   string     `json:"created_by,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
	CloseReason string     `json:"close_reason,omitempty"`
	DueAt       *time.Time `json:"due_at,omitempty"`
	DeferUntil  *time.Time `json:"defer_until,omitempty"`

	// Relations
	Labels    []string  `json:"labels,omitempty"`
	BlockedBy []string  `json:"blocked_by,omitempty"`
	Comments  []Comment `json:"comments,omitempty"`

	// Derived fields (not in JSON)
	IsComplex bool // Derived from CHALLENGE tag in description
}

// Comment represents a comment on an issue.
type Comment struct {
	ID        string    `json:"id,omitempty"`
	Author    string    `json:"author,omitempty"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// LoadTasks loads tasks from the beads JSONL file.
func LoadTasks(repoDir string) ([]*Task, error) {
	jsonlPath := filepath.Join(repoDir, ".beads", "issues.jsonl")

	file, err := os.Open(jsonlPath)
	if err != nil {
		return nil, fmt.Errorf("open issues.jsonl: %w", err)
	}
	defer file.Close()

	var tasks []*Task
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var task Task
		if err := json.Unmarshal([]byte(line), &task); err != nil {
			continue // Skip malformed lines
		}

		// Derive complexity from description
		task.IsComplex = strings.Contains(task.Description, "CHALLENGE:complex")

		tasks = append(tasks, &task)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan issues.jsonl: %w", err)
	}

	return tasks, nil
}

// ReadyTasks returns tasks that are ready for assignment.
func ReadyTasks(tasks []*Task) []*Task {
	var ready []*Task

	// Build set of closed task IDs
	closedIDs := make(map[string]bool)
	for _, t := range tasks {
		if t.Status == "closed" {
			closedIDs[t.ID] = true
		}
	}

	for _, t := range tasks {
		if t.Status != "open" {
			continue
		}

		// Check if all blockers are closed
		blocked := false
		for _, blockerID := range t.BlockedBy {
			if !closedIDs[blockerID] {
				blocked = true
				break
			}
		}

		if !blocked {
			ready = append(ready, t)
		}
	}

	return ready
}
