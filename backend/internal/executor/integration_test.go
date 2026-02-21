package executor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// DummyLogger is a simple logger for testing.
type DummyLogger struct {
	t *testing.T
}

func TestMain(m *testing.M) {
	// Change working directory to repo root so BuildDirective can find files
	srcDir := os.Getenv("TEST_SRCDIR")
	workspace := os.Getenv("TEST_WORKSPACE")
	if srcDir != "" && workspace != "" {
		os.Chdir(filepath.Join(srcDir, workspace))
	}
	os.Exit(m.Run())
}

func (l *DummyLogger) Log(source, message string) {
	l.t.Logf("[%s] %s", source, message)
}

// findDummyGemini locates the dummy-gemini binary.
func findDummyGemini(t *testing.T) string {
	srcDir := os.Getenv("TEST_SRCDIR")
	if srcDir != "" {
		var foundPath string
		err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && info.Name() == "dummy-gemini" {
				if info.Mode()&0111 != 0 {
					foundPath = path
					return filepath.SkipAll
				}
			}
			return nil
		})
		if err == nil && foundPath != "" {
			return foundPath
		}
	}
	
	// Fallback: shell out to bazel build (useful when running test outside bazel)
	t.Logf("dummy-gemini not found in TEST_SRCDIR, attempting bazel build...")
	cmd := exec.Command("bazel", "build", "//backend/cmd/dummy-gemini")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build dummy-gemini: %v\n%s", err, out)
	}
	
	// Assuming it's in bazel-bin
	cwd, _ := os.Getwd()
	// Find repo root
	for {
		if _, err := os.Stat(filepath.Join(cwd, "WORKSPACE")); err == nil {
			break
		}
		if _, err := os.Stat(filepath.Join(cwd, "MODULE.bazel")); err == nil {
			break
		}
		cwd = filepath.Dir(cwd)
		if cwd == "/" {
			break
		}
	}
	binPath := filepath.Join(cwd, "bazel-bin", "backend", "cmd", "dummy-gemini", "dummy-gemini_", "dummy-gemini")
	if _, err := os.Stat(binPath); err == nil {
		return binPath
	}
	
t.Fatalf("Could not locate dummy-gemini binary")
	return ""
}

// setupWorktree creates a temporary git repository with an initial commit.
func setupWorktree(t *testing.T, taskID string) string {
	dir := t.TempDir()

	runCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	runCmd("init")
	runCmd("config", "user.name", "Test User")
	runCmd("config", "user.email", "test@example.com")
	
	// Create an initial commit
	err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test Repo\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to write README.md: %v", err)
	}
	
	runCmd("add", "README.md")
	runCmd("commit", "-m", "Initial commit")
	
	// Add a dummy origin remote
	dummyRemote := t.TempDir()
	runCmdInit := exec.Command("git", "init", "--bare")
	runCmdInit.Dir = dummyRemote
	if err := runCmdInit.Run(); err != nil {
		t.Fatalf("Failed to init bare repo: %v", err)
	}
	runCmd("remote", "add", "origin", dummyRemote)
	runCmd("push", "-u", "origin", "HEAD")

	if taskID != "" {
		runCmd("checkout", "-b", taskID)
		runCmd("push", "-u", "origin", taskID)
	}

	return dir
}

func setupConfig(t *testing.T, taskID string) (ExecutionConfig, string) {
	geminiPath := findDummyGemini(t)
	worktreeDir := setupWorktree(t, taskID)
	machinatorDir := t.TempDir()

	cfg := ExecutionConfig{
		GeminiPath:     geminiPath,
		WorktreeDir:    worktreeDir,
		HomeDir:        t.TempDir(),
		RepoDir:        worktreeDir,
		Model:          "dummy-model",
		TaskID:         taskID,
		AgentID:        1,
		IdleTimeout:    10 * time.Second,
		MaxRuntime:     1 * time.Minute,
		SandboxEnabled: false,
	}
	return cfg, machinatorDir
}

func TestExecuteTask_HappyPath(t *testing.T) {
	cfg, machinatorDir := setupConfig(t, "task-happy")
	t.Setenv("DUMMY_GEMINI_MODE", "fast")

	logger := &DummyLogger{t: t}
	result := ExecuteTask(cfg, "task-happy", "A happy task", "context", machinatorDir, logger)

	if result.Error != nil {
		t.Errorf("Expected success, got error: %v", result.Error)
	}
	if result.ExitCode != 0 {
		t.Errorf("Expected ExitCode 0, got %d", result.ExitCode)
	}
}

func TestExecuteTask_Error(t *testing.T) {
	cfg, machinatorDir := setupConfig(t, "task-error")
	t.Setenv("DUMMY_GEMINI_MODE", "error")

	logger := &DummyLogger{t: t}
	result := ExecuteTask(cfg, "task-error", "An error task", "context", machinatorDir, logger)

	if result.Error == nil {
		t.Fatalf("Expected error, got nil")
	}
	if result.ExitCode == 0 {
		t.Errorf("Expected non-zero ExitCode, got %d", result.ExitCode)
	}
}

func TestExecuteTask_Timeout(t *testing.T) {
	cfg, machinatorDir := setupConfig(t, "task-timeout")
	cfg.IdleTimeout = 2 * time.Second

	// Use a wrapper script to avoid Go's deadlock detector crashing dummy-gemini
	// when it uses `select {}` with no other active goroutines.
	wrapperScript := filepath.Join(t.TempDir(), "gemini-stuck.sh")
	wrapperContent := `#!/bin/bash
echo '{"type":"text","content":"I am starting the task..."}'
sleep 30
`
	if err := os.WriteFile(wrapperScript, []byte(wrapperContent), 0755); err != nil {
		t.Fatalf("Failed to write wrapper script: %v", err)
	}
	cfg.GeminiPath = wrapperScript

	logger := &DummyLogger{t: t}
	result := ExecuteTask(cfg, "task-timeout", "A stuck task", "context", machinatorDir, logger)

	if result.Error == nil {
		t.Fatalf("Expected error due to timeout, got nil")
	}
	if !strings.Contains(result.Error.Error(), "idle timeout") && !strings.Contains(result.Error.Error(), "killed") && !strings.Contains(result.Error.Error(), "timeout") {
		t.Errorf("Expected error containing 'idle timeout', 'killed' or 'timeout', got: %v", result.Error)
	}
}

func TestExecuteWithRetry_SuccessAfterRetry(t *testing.T) {
	cfg, machinatorDir := setupConfig(t, "task-retry")

	// We use a state file to simulate a failure on the first run, and success on the second.
	stateFile := filepath.Join(t.TempDir(), "retry_state")
	
	// Override the DUMMY_GEMINI_MODE behavior via a wrapper script because
	// ExecuteTask doesn't let us easily change env vars between retries without modifying the wrapper or environment.
	// Actually, we can just replace GeminiPath with a wrapper script.
	
	wrapperScript := filepath.Join(t.TempDir(), "gemini-wrapper.sh")
	wrapperContent := fmt.Sprintf(`#!/bin/bash
if [ ! -f "%[1]s" ]; then
    touch "%[1]s"
    export DUMMY_GEMINI_MODE=error
else
    export DUMMY_GEMINI_MODE=fast
fi
exec "%[2]s" "$@"
`, stateFile, cfg.GeminiPath)

	err := os.WriteFile(wrapperScript, []byte(wrapperContent), 0755)
	if err != nil {
		t.Fatalf("Failed to write wrapper script: %v", err)
	}
	
	cfg.GeminiPath = wrapperScript
	
	// Use small timeouts to speed up retry waiting
	// But ExecuteWithRetry hardcodes 30s * attempt wait time!
	// Wait, is there a way to override it? No, ExecuteWithRetry hardcodes time.Duration(30*(attempt+1))*time.Second.
	// To avoid waiting 30 seconds in tests, let's see if we can patch it or if we just have to wait.
	// Actually, wait... 30 seconds per attempt is long for a test. Let me check if there's a workaround.
	// I'll run the test anyway; the instructions demand it.

	logger := &DummyLogger{t: t}
	
	// We might need a very fast test. Let me look at retry.go to see if I can use a monkey patch or mock.
	// It just uses time.Sleep. Nothing is mockable.
	// If the user requires the test, we have to wait 30 seconds unless I change retry.go.
	// The instructions said "Do NOT modify existing source files", so I'll just wait the 30 seconds.

	startTime := time.Now()
	result := ExecuteWithRetry(cfg, "task-retry", "A retry task", "context", machinatorDir, logger, 1)
	duration := time.Since(startTime)

	if result.Error != nil {
		t.Errorf("Expected success after retry, got error: %v", result.Error)
	}
	
	if duration < 30*time.Second {
		t.Logf("Warning: Test finished in %v, expected at least 30s due to retry delay", duration)
	}
}
