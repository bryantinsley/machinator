package setup

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initTestRepo(t *testing.T) string {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test Repo"), 0644)
	if err != nil {
		t.Fatalf("failed to write README: %v", err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial commit")
	runGit(t, dir, "branch", "-m", "main")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func TestNew(t *testing.T) {
	baseDir := "/tmp/machinator"
	s := New(baseDir)
	if s.MachinatorDir != baseDir {
		t.Errorf("expected MachinatorDir %s, got %s", baseDir, s.MachinatorDir)
	}
}

func TestCloneRepo(t *testing.T) {
	baseDir := t.TempDir()
	s := New(baseDir)

	// Create a bare repo to clone from
	remoteDir := t.TempDir()
	runGit(t, remoteDir, "init", "--bare")

	// Create a repo, push to the bare repo
	tempDir := initTestRepo(t)
	runGit(t, tempDir, "remote", "add", "origin", remoteDir)
	runGit(t, tempDir, "push", "origin", "main")

	projectID := 1
	repoURL := remoteDir
	branch := "main"

	repoDir, err := s.CloneRepo(projectID, repoURL, branch)
	if err != nil {
		t.Fatalf("CloneRepo failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err != nil {
		t.Errorf("repo .git dir does not exist: %v", err)
	}
}

func TestCreateWorktree(t *testing.T) {
	baseDir := t.TempDir()
	s := New(baseDir)

	// Setup: Clone a repo first so we have a repo with origin
	remoteDir := t.TempDir()
	runGit(t, remoteDir, "init", "--bare")

	tempDir := initTestRepo(t)
	runGit(t, tempDir, "remote", "add", "origin", remoteDir)
	runGit(t, tempDir, "push", "origin", "main")

	projectID := 1
	branch := "main"
	_, err := s.CloneRepo(projectID, remoteDir, branch)
	if err != nil {
		t.Fatalf("setup CloneRepo failed: %v", err)
	}

	agentID := 101
	agentDir, err := s.CreateWorktree(projectID, agentID, branch)
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	if _, err := os.Stat(agentDir); err != nil {
		t.Errorf("agent worktree dir does not exist: %v", err)
	}
}

func TestResetWorktree(t *testing.T) {
	baseDir := t.TempDir()
	s := New(baseDir)

	// Setup: Clone a repo first
	remoteDir := t.TempDir()
	runGit(t, remoteDir, "init", "--bare")

	tempDir := initTestRepo(t)
	runGit(t, tempDir, "remote", "add", "origin", remoteDir)
	runGit(t, tempDir, "push", "origin", "main")

	projectID := 1
	branch := "main"
	_, err := s.CloneRepo(projectID, remoteDir, branch)
	if err != nil {
		t.Fatalf("setup CloneRepo failed: %v", err)
	}

	agentID := 101
	agentDir, err := s.CreateWorktree(projectID, agentID, branch)
	if err != nil {
		t.Fatalf("setup CreateWorktree failed: %v", err)
	}

	// Make changes in worktree
	dirtyFile := filepath.Join(agentDir, "dirty.txt")
	os.WriteFile(dirtyFile, []byte("dirty content"), 0644)

	// Also modify existing file
	os.WriteFile(filepath.Join(agentDir, "README.md"), []byte("modified"), 0644)

	// Reset worktree
	err = s.ResetWorktree(agentDir, branch)
	if err != nil {
		t.Fatalf("ResetWorktree failed: %v", err)
	}

	// Verify clean state
	if _, err := os.Stat(dirtyFile); err == nil {
		t.Errorf("dirty file still exists after reset")
	}

	content, err := os.ReadFile(filepath.Join(agentDir, "README.md"))
	if err != nil {
		t.Fatalf("failed to read README.md: %v", err)
	}
	if string(content) != "# Test Repo" {
		t.Errorf("expected content '# Test Repo', got '%s'", string(content))
	}
}
