package beads

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Task represents a bead task.
type Task struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    int      `json:"priority"`
	BlockedBy   []string `json:"blocked_by"`
	IsComplex   bool     // Derived from CHALLENGE tag
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
