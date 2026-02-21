package executor

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// TriageWorktree assesses the state of the worktree after an agent session.
// Possible return actions: 'clean', 'committed', 'minor_discard', 'salvage'.
func TriageWorktree(worktreeDir string, branch string, logger Logger) (string, error) {
	// 1. Run 'git -C worktreeDir status --porcelain' to check for uncommitted changes
	statusOutput, err := runGit(worktreeDir, "status", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("git status: %w", err)
	}

	if statusOutput == "" {
		// 2. If no output: check 'git -C worktreeDir log origin/{branch}..HEAD --oneline' for commits.
		logOutput, err := runGit(worktreeDir, "log", fmt.Sprintf("origin/%s..HEAD", branch), "--oneline")
		if err != nil {
			return "", fmt.Errorf("git log: %w", err)
		}
		if logOutput != "" {
			return "committed", nil
		}
		return "clean", nil
	}

	// 3. If output exists: count lines.
	statusLines := strings.Split(strings.TrimSpace(statusOutput), "\n")
	numStatusLines := len(statusLines)

	diffStat, err := runGit(worktreeDir, "diff", "--stat")
	if err != nil {
		return "", fmt.Errorf("git diff --stat: %w", err)
	}

	// Parse diff stat to get total changes.
	totalChanges := parseDiffStat(diffStat)

	// If <= 2 lines and total diff < 20 lines (git diff --stat), return 'minor_discard'.
	if numStatusLines <= 2 && totalChanges < 20 {
		return "minor_discard", nil
	}

	return "salvage", nil
}

// ResetWorktree resets the worktree to the origin branch state.
func ResetWorktree(worktreeDir string, branch string) error {
	if _, err := runGit(worktreeDir, "fetch", "origin"); err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}
	if _, err := runGit(worktreeDir, "reset", "--hard", fmt.Sprintf("origin/%s", branch)); err != nil {
		return fmt.Errorf("git reset: %w", err)
	}
	if _, err := runGit(worktreeDir, "clean", "-fd"); err != nil {
		return fmt.Errorf("git clean: %w", err)
	}
	return nil
}

// SalvageWorktree commits uncommitted changes to a salvage branch and pushes to origin.
func SalvageWorktree(worktreeDir string, taskID string, logger Logger) error {
	timestamp := time.Now().Format("20060102-150405")
	salvageBranch := fmt.Sprintf("salvage/%s-%s", taskID, timestamp)

	if _, err := runGit(worktreeDir, "checkout", "-b", salvageBranch); err != nil {
		return fmt.Errorf("git checkout -b: %w", err)
	}
	if _, err := runGit(worktreeDir, "add", "-A"); err != nil {
		return fmt.Errorf("git add -A: %w", err)
	}
	// We use --allow-empty just in case, though triage should have caught it.
	if _, err := runGit(worktreeDir, "commit", "-m", fmt.Sprintf("salvage: uncommitted changes from task %s", taskID)); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	if _, err := runGit(worktreeDir, "push", "origin", salvageBranch); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	if logger != nil {
		logger.Log("triage", fmt.Sprintf("Salvaged worktree to branch %s", salvageBranch))
	}
	return nil
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git %v failed: %w (stderr: %s)", args, err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

func parseDiffStat(output string) int {
	if output == "" {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return 0
	}
	lastLine := lines[len(lines)-1]

	// Typical last line format:
	// " 1 file changed, 1 insertion(+)"
	// " 2 files changed, 10 insertions(+), 5 deletions(-)"
	parts := strings.Split(lastLine, ",")
	total := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "insertion") || strings.Contains(part, "deletion") {
			var n int
			fmt.Sscanf(part, "%d", &n)
			total += n
		}
	}
	return total
}
