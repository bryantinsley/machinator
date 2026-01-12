package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

type Task struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Title  string `json:"title"`
}

type Harness struct {
	RootDir       string
	TmpDir        string
	BinDir        string
	RepoDir       string
	MachinatorDir string
	MachinatorBin string
	GeminiBin     string
}

func setupHarness(t *testing.T) *Harness {
	h := &Harness{}
	h.RootDir = findProjectRoot(t)
	h.TmpDir = t.TempDir()
	h.BinDir = filepath.Join(h.TmpDir, "bin")
	h.RepoDir = filepath.Join(h.TmpDir, "repo")
	h.MachinatorDir = filepath.Join(h.TmpDir, ".machinator")

	if err := os.MkdirAll(h.BinDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(h.MachinatorDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Use runfiles if available
	rf, err := runfiles.New()
	var bdSrc string
	if err == nil {
		// Potential paths for Machinator
		machinatorPaths := []string{
			"_main/orchestrator/cmd/machinator/machinator_/machinator",
			"_main/orchestrator/cmd/machinator/machinator",
		}
		for _, p := range machinatorPaths {
			if loc, _ := rf.Rlocation(p); loc != "" {
				// Try to run it to ensure it's compatible with host arch
				cmd := exec.Command(loc, "--help")
				err := cmd.Run()
				if err == nil || (err != nil && !strings.Contains(err.Error(), "exec format error")) {
					h.MachinatorBin = loc
					break
				}
			}
		}

		// Potential paths for Gemini
		geminiPaths := []string{
			"_main/tools/dummy-gemini/dummy-gemini_/dummy-gemini",
			"_main/tools/dummy-gemini/dummy-gemini",
		}
		for _, p := range geminiPaths {
			if loc, _ := rf.Rlocation(p); loc != "" {
				cmd := exec.Command(loc)
				err := cmd.Run()
				if err == nil || (err != nil && !strings.Contains(err.Error(), "exec format error")) {
					h.GeminiBin = loc
					break
				}
			}
		}

		bdSrc, _ = rf.Rlocation("_main/bd")
	}

	if h.GeminiBin == "" || h.MachinatorBin == "" {
		t.Fatalf("Runfiles NOT found or incomplete: GeminiBin=%q, MachinatorBin=%q. Manual build is forbidden in E2E.", h.GeminiBin, h.MachinatorBin)
	}

	// Copy bd if found
	if bdSrc != "" {
		if err := copyFile(bdSrc, filepath.Join(h.BinDir, "bd")); err != nil {
			t.Logf("Warning: failed to copy bd: %v", err)
		} else {
			os.Chmod(filepath.Join(h.BinDir, "bd"), 0755)
		}
	}

	// Copy dummy-gemini to ~/.machinator/gemini
	if err := copyFile(h.GeminiBin, filepath.Join(h.MachinatorDir, "gemini")); err != nil {
		t.Fatal(err)
	}
	os.Chmod(filepath.Join(h.MachinatorDir, "gemini"), 0755)

	return h
}

func (h *Harness) setupFixture(t *testing.T) {
	// Use runfiles if available
	rf, err := runfiles.New()
	var fixtureSrc, templatesSrc, agentsMd, checkQuotaSrc string
	if err == nil {
		// Better way to find directory: find a known file and get its parent
		if p, _ := rf.Rlocation("_main/testdata/fixture-repo/AGENTS.md"); p != "" {
			fixtureSrc = filepath.Dir(p)
		}
		templatesSrc, _ = rf.Rlocation("_main/templates")
		agentsMd, _ = rf.Rlocation("_main/AGENTS.md")
		checkQuotaSrc, _ = rf.Rlocation("_main/templates/check_quota.sh")
	}

	if fixtureSrc == "" {
		fixtureSrc = filepath.Join(h.RootDir, "testdata", "fixture-repo")
		templatesSrc = filepath.Join(h.RootDir, "templates")
		agentsMd = filepath.Join(h.RootDir, "AGENTS.md")
		checkQuotaSrc = filepath.Join(h.RootDir, "templates", "check_quota.sh")
	}

	if err := copyDir(fixtureSrc, h.RepoDir); err != nil {
		t.Fatalf("Failed to copy fixture repo: %v", err)
	}

	// Initialize git repo (bd needs it)
	exec.Command("git", "-C", h.RepoDir, "init").Run()
	exec.Command("git", "-C", h.RepoDir, "add", ".").Run()
	exec.Command("git", "-C", h.RepoDir, "commit", "-m", "initial commit").Run()

	// Copy templates
	copyDir(templatesSrc, filepath.Join(h.RepoDir, "templates"))

	// Copy AGENTS.md
	copyFile(agentsMd, filepath.Join(h.RepoDir, "AGENTS.md"))

	// Copy check_quota.sh
	os.MkdirAll(filepath.Join(h.RepoDir, "machinator"), 0755)
	copyFile(checkQuotaSrc, filepath.Join(h.RepoDir, "machinator", "check_quota.sh"))
	os.Chmod(filepath.Join(h.RepoDir, "machinator", "check_quota.sh"), 0755)

	// Create agents/1 directory for execution
	os.MkdirAll(filepath.Join(h.RepoDir, "agents", "1"), 0755)
}

func (h *Harness) runMachinator(t *testing.T, envVars []string, args []string, timeout time.Duration) {
	// Sync DB first
	h.syncDB(t)

	defaultArgs := []string{"--once", "--headless", "--run"}
	finalArgs := append(defaultArgs, args...)

	cmd := exec.Command(h.MachinatorBin, finalArgs...)
	cmd.Dir = h.RepoDir

	// Setup Environment
	env := os.Environ()
	// Filter out existing HOME/PATH to isolate
	var newEnv []string
	for _, e := range env {
		if !strings.HasPrefix(e, "HOME=") && !strings.HasPrefix(e, "PATH=") {
			newEnv = append(newEnv, e)
		}
	}
	env = newEnv

	// Add binDir to PATH
	pathVal := h.BinDir + string(os.PathListSeparator) + os.Getenv("PATH")
	env = append(env, "PATH="+pathVal)

	// Set HOME to tmpDir
	env = append(env, "HOME="+h.TmpDir)

	// Default Env
	env = append(env,
		"BD_AGENT_NAME=E2E-Agent",
		"BUILD_WORKING_DIRECTORY="+h.RepoDir,
	)

	// Add custom vars
	env = append(env, envVars...)
	cmd.Env = env

	// Capture output
	stdoutFile, _ := os.Create(filepath.Join(h.RepoDir, "machinator.stdout"))
	stderrFile, _ := os.Create(filepath.Join(h.RepoDir, "machinator.stderr"))
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	t.Logf("Starting machinator with timeout %s...", timeout)
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start machinator: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Logf("Machinator exited with error: %v", err)
		} else {
			t.Log("Machinator exited successfully")
		}
	case <-time.After(timeout):
		cmd.Process.Kill()
		h.dumpLogs(t)
		t.Fatal("Test timed out waiting for machinator")
	}
}

func (h *Harness) syncDB(t *testing.T) {
	// Run bd import to ensure DB is in sync
	bdPath := filepath.Join(h.BinDir, "bd")
	cmd := exec.Command(bdPath, "import", "-i", ".beads/issues.jsonl")
	cmd.Dir = h.RepoDir
	// Need HOME set for bd
	cmd.Env = append(os.Environ(), "HOME="+h.TmpDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bd import failed: %v\nOutput: %s", err, string(out))
	}
}

func (h *Harness) dumpLogs(t *testing.T) {
	t.Log("--- DEBUG LOGS ---")
	if content, err := os.ReadFile(filepath.Join(h.RepoDir, "machinator.stdout")); err == nil {
		t.Logf("STDOUT:\n%s", string(content))
	}
	if content, err := os.ReadFile(filepath.Join(h.RepoDir, "machinator.stderr")); err == nil {
		t.Logf("STDERR:\n%s", string(content))
	}
	logPath := filepath.Join(h.RepoDir, "machinator", "logs", "tui_debug.log")
	if content, err := os.ReadFile(logPath); err == nil {
		t.Logf("TUI LOG:\n%s", string(content))
	}
}

func (h *Harness) getTask(t *testing.T, id string) Task {
	bdPath := filepath.Join(h.BinDir, "bd")
	cmd := exec.Command(bdPath, "list", "--json", "--no-db")
	cmd.Dir = h.RepoDir
	cmd.Env = append(os.Environ(), "HOME="+h.TmpDir)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to verify with bd: %v", err)
	}

	var tasks []Task
	if err := json.Unmarshal(out, &tasks); err != nil {
		t.Fatalf("Failed to parse verification output: %v", err)
	}

	for _, task := range tasks {
		if task.ID == id {
			return task
		}
	}
	t.Fatalf("Task %s not found", id)
	return Task{}
}

func TestE2E_Happy(t *testing.T) {
	if os.Getenv("E2E_SKIP") == "1" {
		t.Skip("Skipping E2E test")
	}
	h := setupHarness(t)
	h.setupFixture(t)

	// Run with AUTO_CLOSE mode
	h.runMachinator(t, []string{"DUMMY_GEMINI_MODE=AUTO_CLOSE"}, nil, 60*time.Second)

	task := h.getTask(t, "task-1")
	if task.Status != "closed" {
		t.Errorf("Expected task-1 status closed, got %s", task.Status)
		h.dumpLogs(t)
	}
}

func TestE2E_Stuck(t *testing.T) {
	if os.Getenv("E2E_SKIP") == "1" {
		t.Skip("Skipping E2E test")
	}
	h := setupHarness(t)
	h.setupFixture(t)

	// Configure short timeouts via env vars
	env := []string{
		"DUMMY_GEMINI_MODE=STUCK",
		"MACHINATOR_IDLE_TIMEOUT=3s",
		"MACHINATOR_MAX_TASK_RUNTIME=5s",
	}

	h.runMachinator(t, env, nil, 20*time.Second)

	// Task should NOT be closed.
	task := h.getTask(t, "task-1")
	if task.Status == "closed" {
		t.Error("Expected task-1 to NOT be closed (stuck agent)")
	}

	// Check logs for timeout message
	logPath := filepath.Join(h.RepoDir, "machinator", "logs", "tui_debug.log")
	content, _ := os.ReadFile(logPath)
	if !strings.Contains(string(content), "TIMEOUT") {
		t.Error("Logs should contain TIMEOUT message")
		t.Logf("LOGS:\n%s", string(content))
	}
}
func TestE2E_Error(t *testing.T) {
	if os.Getenv("E2E_SKIP") == "1" {
		t.Skip("Skipping E2E test")
	}
	h := setupHarness(t)
	h.setupFixture(t)

	// Run with ERROR mode
	h.runMachinator(t, []string{"DUMMY_GEMINI_MODE=ERROR"}, nil, 30*time.Second)

	task := h.getTask(t, "task-1")
	// It should be in_progress (since we started it) but failed execution.
	if task.Status != "in_progress" {
		t.Logf("Expected task-1 status in_progress, got %s", task.Status)
	}

	// Check logs for error message
	logPath := filepath.Join(h.RepoDir, "machinator", "logs", "tui_debug.log")
	content, _ := os.ReadFile(logPath)
	if !strings.Contains(string(content), "Gemini exited with error") {
		t.Error("Logs should contain Gemini exit error")
		t.Logf("LOGS:\n%s", string(content))
	}
}

func TestE2E_ExecuteFlag(t *testing.T) {
	if os.Getenv("E2E_SKIP") == "1" {
		t.Skip("Skipping E2E test")
	}
	h := setupHarness(t)
	h.setupFixture(t)

	// Execute task-2 specifically
	// Run with AUTO_CLOSE mode for Gemini
	h.runMachinator(t, []string{"DUMMY_GEMINI_MODE=AUTO_CLOSE"}, []string{"--execute", "task-2"}, 60*time.Second)

	// task-2 should be closed
	task := h.getTask(t, "task-2")
	if task.Status != "closed" {
		t.Errorf("Expected task-2 status closed, got %s", task.Status)
		h.dumpLogs(t)
	}

	// task-1 should NOT be closed (even if ready, it shouldn't run if targeted execution worked and exited)
	task1 := h.getTask(t, "task-1")
	if task1.Status == "closed" {
		t.Errorf("Expected task-1 status NOT closed (execution targeted task-2), got %s", task1.Status)
	}
}

func findProjectRoot(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Could not find project root (go.mod)")
		}
		dir = parent
	}
}

func copyDir(src, dst string) error {
	return exec.Command("cp", "-RL", src, dst).Run()
}

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0644)
}
