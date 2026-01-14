package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bryantinsley/machinator/backend/internal/beads"
	"github.com/bryantinsley/machinator/backend/internal/quota"
)

// buildLeftContent builds the left pane content (status sidebar).
func (t *TUI) buildLeftContent() string {
	var content string

	// Load beads OUTSIDE of any locks (this does I/O)
	var newTasks []*beads.Task
	shouldRefreshBeads := time.Since(t.cachedTasksTime) > 15*time.Second
	if shouldRefreshBeads {
		newTasks = t.loadTasksWithTimeout(2 * time.Second)
	}

	// Load git log OUTSIDE of any locks
	var newGitLog []CommitInfo
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

	// Helper for full-width underlines
	underline := func(minWidth int) string {
		width := t.leftWidth
		if width < minWidth {
			width = minWidth
		}
		return strings.Repeat("─", width)
	}

	// Quota section - video game style hearts
	// Grid format: columns = models (simple, complex), rows = accounts
	content += "[yellow]Quota[-]\n"
	content += underline(5) + "\n"

	// Get model names from project config
	simpleModel := "gemini-3-flash-preview"
	complexModel := "gemini-3-pro-preview"
	simpleLabel := "flash"
	complexLabel := "pro"
	if t.projCfg != nil {
		if t.projCfg.SimpleModelName != "" {
			simpleModel = t.projCfg.SimpleModelName
			// Extract recognizable label from model name
			if strings.Contains(simpleModel, "flash") {
				simpleLabel = "flash"
			} else if strings.Contains(simpleModel, "pro") {
				simpleLabel = "pro"
			} else {
				simpleLabel = "simple"
			}
		}
		if t.projCfg.ComplexModelName != "" {
			complexModel = t.projCfg.ComplexModelName
			if strings.Contains(complexModel, "pro") {
				complexLabel = "pro"
			} else if strings.Contains(complexModel, "flash") {
				complexLabel = "flash"
			} else {
				complexLabel = "complex"
			}
		}
	}

	if t.quota != nil && len(t.quota.Accounts) > 0 {
		// Sort accounts by name
		accounts := make([]quota.AccountQuota, len(t.quota.Accounts))
		copy(accounts, t.quota.Accounts)
		sort.Slice(accounts, func(i, j int) bool {
			return accounts[i].Name < accounts[j].Name
		})

		// Header row with model names
		content += fmt.Sprintf("%-6s %-12s %-12s\n", "", "[blue]"+simpleLabel+"[-]", "[purple]"+complexLabel+"[-]")

		for _, acc := range accounts {
			// Get quota percentages (0-100)
			simplePct := -1
			complexPct := -1
			if remaining, ok := acc.Models[simpleModel]; ok {
				simplePct = int(remaining * 100)
			}
			if remaining, ok := acc.Models[complexModel]; ok {
				complexPct = int(remaining * 100)
			}

			// Render hearts + percentage for each model
			simpleHearts := renderQuotaHearts(simplePct)
			complexHearts := renderQuotaHearts(complexPct)
			simplePctStr := "[gray] --[-]"
			complexPctStr := "[gray] --[-]"
			if simplePct >= 0 {
				simplePctStr = fmt.Sprintf(" %3d%%", simplePct)
			}
			if complexPct >= 0 {
				complexPctStr = fmt.Sprintf(" %3d%%", complexPct)
			}

			// Truncate account name to fit
			name := acc.Name
			if len(name) > 6 {
				name = name[:5] + "…"
			}
			content += fmt.Sprintf("%-6s %s%s %s%s\n", name, simpleHearts, simplePctStr, complexHearts, complexPctStr)
		}
	} else {
		content += "[gray]No quota data[-]\n"
	}

	// Agents section - state has its own synchronization
	content += "\n[yellow]Agents[-]\n"
	content += underline(6) + "\n"

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
				// Truncate based on left panel width
				// Format: "   shortID: title" = 3 + len(shortID) + 2 + title
				titleWidth := t.leftWidth - 3 - len(shortID) - 2
				if titleWidth < 5 {
					titleWidth = 5
				}
				if len(title) > titleWidth {
					title = title[:titleWidth-1] + "…"
				}
				content += fmt.Sprintf("   [gray]%s: %s[-]\n", shortID, title)
			}
		}
	}

	// Beads section - use the cached copy we made
	content += "\n[yellow]Beads[-]\n"
	content += underline(5) + "\n"

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
	content += "\n[yellow]Git Commits[-]\n"
	content += underline(11) + "\n"
	if len(cachedGitLog) == 0 {
		content += "[gray]No commits[-]\n"
	} else {
		// Format commits with current width for responsive display
		for _, commit := range cachedGitLog {
			suffix := fmt.Sprintf(" (%s)", commit.Age)
			// Overhead: hash (3) + space (1) + suffix
			overhead := 3 + 1 + len(suffix)
			maxMsgLen := t.leftWidth - overhead
			if maxMsgLen < 5 {
				maxMsgLen = 5
			}
			msg := commit.Message
			if len(msg) > maxMsgLen {
				msg = msg[:maxMsgLen-1] + "…"
			}
			// Pad message to right-align the age
			content += fmt.Sprintf("[gray]%s %-*s%s[-]\n", commit.Hash, maxMsgLen, msg, suffix)
		}
	}

	return content
}
