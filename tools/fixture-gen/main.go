package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// GenerateFixture creates a temporary git repository with a valid Beads state.
func GenerateFixture(path string) error {
	// 1. Create target directory
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// 2. Run 'git init'
	if err := runCmd(absPath, "git", "init"); err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}

	// 3. Initialize beads with prefix 'bead'
	// Use --no-daemon to ensure direct database access and flushing
	if err := runCmd(absPath, "bd", "--no-daemon", "init", "--prefix", "bead"); err != nil {
		return fmt.Errorf("bd init failed: %w", err)
	}

	// 4. Populate with bead-1 (Open) and bead-2 (Blocked by bead-1)
	if err := runCmd(absPath, "bd", "--no-daemon", "create", "First Bead", "--id", "bead-1"); err != nil {
		return fmt.Errorf("failed to create bead-1: %w", err)
	}
	if err := runCmd(absPath, "bd", "--no-daemon", "create", "Second Bead", "--id", "bead-2", "--deps", "bead-1"); err != nil {
		return fmt.Errorf("failed to create bead-2: %w", err)
	}

	// Ensure .beads/beads.jsonl exists (renaming from issues.jsonl if necessary)
	issuesJSONL := filepath.Join(absPath, ".beads", "issues.jsonl")
	beadsJSONL := filepath.Join(absPath, ".beads", "beads.jsonl")
	if _, err := os.Stat(issuesJSONL); err == nil {
		if err := os.Rename(issuesJSONL, beadsJSONL); err != nil {
			return fmt.Errorf("failed to rename issues.jsonl to beads.jsonl: %w", err)
		}
	} else if _, err := os.Stat(beadsJSONL); os.IsNotExist(err) {
		// If neither exists, something went wrong with bd create
		return fmt.Errorf("no jsonl file found in .beads/")
	}

	// 5. Commit everything to git
	if err := runCmd(absPath, "git", "add", "."); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	commitCmd := exec.Command("git", "commit", "-m", "Initial beads fixture")
	commitCmd.Dir = absPath
	commitCmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Fixture Gen",
		"GIT_AUTHOR_EMAIL=fixture@example.com",
		"GIT_COMMITTER_NAME=Fixture Gen",
		"GIT_COMMITTER_EMAIL=fixture@example.com",
	)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %w, output: %s", err, string(out))
	}

	return nil
}

func runCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("command %s %v failed: %w, output: %s", name, args, err, string(out))
	}
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: fixture-gen <path>")
		os.Exit(1)
	}

	targetPath := os.Args[1]
	if err := GenerateFixture(targetPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully generated fixture at: %s\n", targetPath)
}
