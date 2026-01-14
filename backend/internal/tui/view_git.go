package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// handleGitKey handles all key events for the git view.
// Currently passes all keys through since git view is read-only.
func (t *TUI) handleGitKey(event *tcell.EventKey) *tcell.EventKey {
	// Git view is currently read-only, no custom key handling
	// Future: could add selection, commit detail view, etc.
	return event
}

// CommitInfo holds raw commit data for responsive formatting.
type CommitInfo struct {
	Hash    string // 3-char short hash
	Message string // First line of commit message
	Age     string // "7h", "2d", etc.
}

// buildGitView builds the extended git commits view for the right pane.
func (t *TUI) buildGitView() string {
	// Load 100 commits - tview handles scrolling
	commits := t.loadGitLogExtended(100)
	if len(commits) == 0 {
		return "[gray]No commits[-]"
	}

	var content string

	// Calculate available width for message
	// Format: "date (10) + space + hash (7) + space + msg"
	dateWidth := 10
	hashWidth := 7
	overhead := dateWidth + 1 + hashWidth + 1
	msgWidth := t.rightWidth - overhead
	if msgWidth < 10 {
		msgWidth = 10
	}

	for _, c := range commits {
		msg := c.msg
		if len(msg) > msgWidth {
			msg = msg[:msgWidth-1] + "…"
		}
		content += fmt.Sprintf("[blue]%-*s[-] [gray]%s[-] %s\n", dateWidth, c.date, c.hash, msg)
	}
	return content
}

// selectGitItem handles Enter key on git commits list (placeholder for future detail view)
func (t *TUI) selectGitItem() {
	// TODO: Implement git commit detail view if needed
	// For now, do nothing - git view is informational
}

// loadGitLog returns the last N commits as raw data using go-git (for sidebar).
func (t *TUI) loadGitLog(n int) []CommitInfo {
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

	var commits []CommitInfo
	count := 0
	iter.ForEach(func(c *object.Commit) error {
		if count >= n {
			return fmt.Errorf("done")
		}

		// Skip hidden authors
		for _, hide := range t.cfg.HideCommitAuthors {
			if strings.Contains(c.Author.Name, hide) || strings.Contains(c.Author.Email, hide) {
				return nil
			}
		}

		// Store raw data
		commits = append(commits, CommitInfo{
			Hash:    c.Hash.String()[4:7], // 3 chars from middle for variety
			Message: strings.Split(c.Message, "\n")[0],
			Age:     formatAge(time.Since(c.Author.When)),
		})
		count++
		return nil
	})

	return commits
}

// commitExtended holds formatted commit data for the right pane git view.
type commitExtended struct {
	date string
	hash string
	msg  string
}

// loadGitLogExtended returns commits with human-readable dates (for git view).
func (t *TUI) loadGitLogExtended(n int) []commitExtended {
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

	var commits []commitExtended
	count := 0
	iter.ForEach(func(c *object.Commit) error {
		if count >= n {
			return fmt.Errorf("done")
		}

		// Skip hidden authors
		for _, hide := range t.cfg.HideCommitAuthors {
			if strings.Contains(c.Author.Name, hide) || strings.Contains(c.Author.Email, hide) {
				return nil
			}
		}

		commits = append(commits, commitExtended{
			date: formatCommitDate(c.Author.When),
			hash: c.Hash.String()[:7],
			msg:  strings.Split(c.Message, "\n")[0],
		})
		count++
		return nil
	})

	return commits
}

// formatAge returns a short age string like "30s", "5m", "4h", "2d".
func formatAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 48*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// formatCommitDate returns human-readable date.
// Within 24h: "5s ago", "10m ago", "17h ago"
// Within week: "yesterday", "Monday", etc.
// Older: "8d ago", "2mo ago", "1yr ago"
func formatCommitDate(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	// Within last 24 hours: use time-based format
	if diff < 24*time.Hour {
		if diff < time.Minute {
			return fmt.Sprintf("%ds ago", int(diff.Seconds()))
		}
		if diff < time.Hour {
			return fmt.Sprintf("%dm ago", int(diff.Minutes()))
		}
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	weekAgo := today.AddDate(0, 0, -7)

	if t.After(yesterday) {
		return "yesterday"
	}
	if t.After(weekAgo) {
		return t.Weekday().String()[:3] // Mon, Tue, etc
	}

	days := int(diff.Hours() / 24)
	if days < 30 {
		return fmt.Sprintf("%dd ago", days)
	}
	months := days / 30
	if months < 12 {
		return fmt.Sprintf("%dmo ago", months)
	}
	years := months / 12
	return fmt.Sprintf("%dyr ago", years)
}
