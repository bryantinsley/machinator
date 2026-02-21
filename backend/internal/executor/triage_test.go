package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestParseDiffStat(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected int
	}{
		{
			name:     "empty output",
			output:   "",
			expected: 0,
		},
		{
			name:     "single file changed",
			output:   " file1.txt | 10 ++++++++++\n 1 file changed, 10 insertions(+)",
			expected: 10,
		},
		{
			name:     "multiple files changed",
			output:   " file1.txt | 10 ++++++++++\n file2.txt | 5 +++++\n 2 files changed, 10 insertions(+), 5 deletions(-)",
			expected: 15,
		},
		{
			name:     "summary with only insertions",
			output:   " 1 file changed, 5 insertions(+)",
			expected: 5,
		},
		{
			name:     "summary with only deletions",
			output:   " 1 file changed, 3 deletions(-)",
			expected: 3,
		},
		{
			name:     "complex summary line",
			output:   " 3 files changed, 10 insertions(+), 5 deletions(-), 2 modifications(!)",
			expected: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDiffStat(tt.output)
			if got != tt.expected {
				t.Errorf("parseDiffStat() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func setupGitRepo(t *testing.T, dir string) {
	if _, err := runGit(dir, "init"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	if _, err := runGit(dir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("git config user.email failed: %v", err)
	}
	if _, err := runGit(dir, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("git config user.name failed: %v", err)
	}
	if _, err := runGit(dir, "checkout", "-b", "main"); err != nil {
		t.Fatalf("git checkout -b main failed: %v", err)
	}
	if _, err := runGit(dir, "commit", "--allow-empty", "-m", "initial commit"); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}
	// Create origin branch reference.
	if _, err := runGit(dir, "branch", "origin/main"); err != nil {
		t.Fatalf("git branch origin/main failed: %v", err)
	}
}

func TestTriageWorktree_Clean(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	got, err := TriageWorktree(dir, "main", nil)
	if err != nil {
		t.Fatalf("TriageWorktree failed: %v", err)
	}
	if got != "clean" {
		t.Errorf("expected clean, got %s", got)
	}
}

func TestTriageWorktree_Committed(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	// Add an extra commit.
	if _, err := runGit(dir, "commit", "--allow-empty", "-m", "extra commit"); err != nil {
		t.Fatalf("git commit extra failed: %v", err)
	}

	got, err := TriageWorktree(dir, "main", nil)
	if err != nil {
		t.Fatalf("TriageWorktree failed: %v", err)
	}
	if got != "committed" {
		t.Errorf("expected committed, got %s", got)
	}
}

func TestTriageWorktree_MinorDiscard(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	// Create 1 small file (untracked).
	import_path := "test.txt"
	if err := os.WriteFile(filepath.Join(dir, import_path), []byte("hello"), 0644); err != nil {
		t.Fatalf("os.WriteFile failed: %v", err)
	}

	got, err := TriageWorktree(dir, "main", nil)
	if err != nil {
		t.Fatalf("TriageWorktree failed: %v", err)
	}
	if got != "minor_discard" {
		t.Errorf("expected minor_discard, got %s", got)
	}
}

func TestTriageWorktree_Salvage(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	// Create 5 untracked files.
	for i := 1; i <= 5; i++ {
		fname := fmt.Sprintf("file%d.txt", i)
		if err := os.WriteFile(filepath.Join(dir, fname), []byte("content"), 0644); err != nil {
			t.Fatalf("os.WriteFile failed: %v", err)
		}
	}

	got, err := TriageWorktree(dir, "main", nil)
	if err != nil {
		t.Fatalf("TriageWorktree failed: %v", err)
	}
	if got != "salvage" {
		t.Errorf("expected salvage, got %s", got)
	}
}
