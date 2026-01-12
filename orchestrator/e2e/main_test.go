package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type Task struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Title  string `json:"title"`
}

func TestOrchestratorE2E(t *testing.T) {
	if os.Getenv("E2E_SKIP") == "1" {
		t.Skip("Skipping E2E test")
	}

	// 1. Setup environment
	root := findProjectRoot(t)
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	repoDir := filepath.Join(tmpDir, "repo")

	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}

	t.Logf("Project Root: %s", root)
	t.Logf("Temp Dir: %s", tmpDir)

	// 2. Build binaries
	// Build dummy-gemini
	geminiPath := filepath.Join(binDir, "gemini")
	buildGoBin(t, root, "./tools/dummy-gemini", geminiPath)

	// Build machinator
	machinatorPath := filepath.Join(binDir, "machinator")
	buildGoBin(t, root, "./orchestrator/cmd/machinator", machinatorPath)

	// 3. Setup Fixture Repo
	// Copy testdata/fixture-repo to repoDir
	fixtureSrc := filepath.Join(root, "testdata", "fixture-repo")
	if err := copyDir(fixtureSrc, repoDir); err != nil {
		t.Fatalf("Failed to copy fixture repo: %v", err)
	}

	// Create templates/directive_template.txt in repoDir (orchestrator expects it relative to CWD if not found elsewhere)
	// But orchestrator logic looks in runfiles or workspace.
	// Since we run machinator in repoDir, we should provide templates there or ensure it finds them.
	// The harness should mimic the real environment.
	// In real env, machinator binary is usually run from project root or has templates nearby.
	// Let's copy templates to repoDir/templates
	copyDir(filepath.Join(root, "templates"), filepath.Join(repoDir, "templates"))

	// Also copy AGENTS.md (orchestrator reads it for context)
	copyFile(filepath.Join(root, "AGENTS.md"), filepath.Join(repoDir, "AGENTS.md"))

	// Copy bootstrap/check_quota.sh to repoDir/machinator/check_quota.sh (used by logic)
	// Actually orchestrator/pkg/orchestrator/quota_check.go uses "bootstrap/check_quota.sh" relative to project root?
	// No, it uses filepath.Join(projectRoot, "machinator", "check_quota.sh") usually?
	// Let's check quota_check.go. But assuming it needs it:
	os.MkdirAll(filepath.Join(repoDir, "machinator"), 0755)
	// We might not have check_quota.sh in the fixture, but orchestrator might look for it.
	// Let's dummy it out to avoid errors, or copy real one.
	// Real one is in bootstrap/check_quota.sh
	copyFile(filepath.Join(root, "bootstrap", "check_quota.sh"), filepath.Join(repoDir, "machinator", "check_quota.sh"))
	os.Chmod(filepath.Join(repoDir, "machinator", "check_quota.sh"), 0755)

	// 4. Setup Machinator Environment (Mock HOME)
	machinatorDir := filepath.Join(tmpDir, ".machinator")
	if err := os.MkdirAll(machinatorDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Copy dummy-gemini to ~/.machinator/gemini
	if err := copyFile(geminiPath, filepath.Join(machinatorDir, "gemini")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(machinatorDir, "gemini"), 0755); err != nil {
		t.Fatal(err)
	}

	// 5. Run Orchestrator
	// We use the "task-1" from fixture repo.
	// We expect dummy-gemini (AUTO_CLOSE mode) to close it.

	cmd := exec.Command(machinatorPath, "--once", "--headless")
	cmd.Dir = repoDir

	// Setup Environment
	env := os.Environ()
	// Filter out existing HOME
	var newEnv []string
	for _, e := range env {
		if !strings.HasPrefix(e, "HOME=") && !strings.HasPrefix(e, "PATH=") {
			newEnv = append(newEnv, e)
		}
	}
	env = newEnv

	// Add binDir to PATH
	pathVal := binDir + string(os.PathListSeparator) + os.Getenv("PATH")
	env = append(env, "PATH="+pathVal)

	// Set HOME to tmpDir
	env = append(env, "HOME="+tmpDir)

	// Add other env vars
	env = append(env,
		"DUMMY_GEMINI_MODE=AUTO_CLOSE", // Tells dummy-gemini to close the task
		"BD_AGENT_NAME=E2E-Agent",
		"BUILD_WORKING_DIRECTORY="+repoDir, // Help find templates if needed
	)
	cmd.Env = env

	// Verify bd list works before starting
	// First, run bd import to ensure DB is in sync (since we copied files, timestamps might confuse bd)
	importCmd := exec.Command("bd", "import", "-i", ".beads/issues.jsonl")
	importCmd.Dir = repoDir
	importCmd.Env = env
	if out, err := importCmd.CombinedOutput(); err != nil {
		t.Fatalf("bd import failed: %v\nOutput: %s", err, string(out))
	} else {
		t.Logf("bd import success: %s", string(out))
	}

	verifyBdCmd := exec.Command("bd", "list", "--json") // Default mode (uses DB)
	verifyBdCmd.Dir = repoDir
	verifyBdCmd.Env = env
	if out, err := verifyBdCmd.CombinedOutput(); err != nil {
		t.Logf("Warning: bd list failed in setup: %v\nOutput: %s", err, string(out))
	} else {
		t.Logf("bd list check passed: %s", string(out))
	}

	// Capture output
	stdoutFile, _ := os.Create(filepath.Join(repoDir, "machinator.stdout"))
	stderrFile, _ := os.Create(filepath.Join(repoDir, "machinator.stderr"))
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	t.Log("Starting machinator...")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start machinator: %v", err)
	}

	// Wait for completion (with timeout)
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
	case <-time.After(60 * time.Second):
		cmd.Process.Kill()

		// Dump logs
		t.Log("--- TIMEOUT DEBUGGING ---")

		if content, err := os.ReadFile(filepath.Join(repoDir, "machinator.stdout")); err == nil {
			t.Logf("STDOUT:\n%s", string(content))
		}
		if content, err := os.ReadFile(filepath.Join(repoDir, "machinator.stderr")); err == nil {
			t.Logf("STDERR:\n%s", string(content))
		}

		logPath := filepath.Join(repoDir, "machinator", "logs", "tui_debug.log")
		if content, err := os.ReadFile(logPath); err == nil {
			t.Logf("TUI LOG:\n%s", string(content))
		} else {
			t.Logf("TUI LOG not found at %s", logPath)
		}

		t.Fatal("Test timed out waiting for machinator")
	}

	// 5. Verify Results
	// Check if task-1 is closed in repoDir/.beads/issues.jsonl
	// We can use bd list --json --no-db

	verifyCmd := exec.Command("bd", "list", "--json", "--no-db")
	verifyCmd.Dir = repoDir
	out, err := verifyCmd.Output()
	if err != nil {
		t.Fatalf("Failed to verify with bd: %v", err)
	}

	var tasks []Task
	if err := json.Unmarshal(out, &tasks); err != nil {
		t.Fatalf("Failed to parse verification output: %v", err)
	}

	task1Closed := false
	for _, task := range tasks {
		if task.ID == "task-1" {
			t.Logf("Task 1 status: %s", task.Status)
			if task.Status == "closed" {
				task1Closed = true
			}
		}
	}

	if !task1Closed {
		t.Error("task-1 was not closed by the agent")
		// Dump logs
		logPath := filepath.Join(repoDir, "machinator", "logs", "tui_debug.log")
		if content, err := os.ReadFile(logPath); err == nil {
			t.Logf("--- TUI DEBUG LOG ---\n%s\n---------------------", string(content))
		}
	}
}

func findProjectRoot(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// We are in orchestrator/e2e, so go up 2 levels
	// But let's be robust
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

func buildGoBin(t *testing.T, root, pkg, outPath string) {
	cmd := exec.Command("go", "build", "-o", outPath, pkg)
	cmd.Dir = root
	// Ensure we use local cache to avoid permission errors
	env := os.Environ()
	// We assume GOCACHE/GOPATH are set in the shell running the test,
	// but if not, we might rely on the test runner.
	// Since we run this test via `go test`, we assume env is sane.
	cmd.Env = env

	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build %s: %v\nOutput: %s", pkg, err, string(output))
	}
}

func copyDir(src, dst string) error {
	return exec.Command("cp", "-R", src, dst).Run()
}

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, input, 0644)
}
