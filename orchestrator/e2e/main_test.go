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
}

func TestOrchestratorE2E(t *testing.T) {
	// Find project root
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "WORKSPACE")); err == nil {
			break
		}
		if _, err := os.Stat(filepath.Join(root, "MODULE.bazel")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("could not find project root")
		}
		root = parent
	}
	t.Logf("Project root: %s", root)

	// 1. Setup temp directory
	tmpDir, err := os.MkdirTemp("", "machinator-e2e-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	projectDir := filepath.Join(tmpDir, "project")
	os.MkdirAll(projectDir, 0755)

	binDir := filepath.Join(tmpDir, "bin")
	os.MkdirAll(binDir, 0755)

	t.Logf("Temp dir: %s", tmpDir)

	// 2. Build binaries using bazel
	buildBin(t, root, "//tools/fixture-gen", filepath.Join(binDir, "fixture-gen"))
	buildBin(t, root, "//tools/dummy-gemini", filepath.Join(binDir, "gemini"))
	buildBin(t, root, "//:machinator", filepath.Join(binDir, "machinator"))

	// 3. Generate fixture repo
	cmd := exec.Command(filepath.Join(binDir, "fixture-gen"), projectDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture-gen failed: %v\nOutput: %s", err, string(out))
	}

	// 4. Run orchestrator
	path := os.Getenv("PATH")
	newPath := binDir + ":" + path

	orchestratorCmd := exec.Command(filepath.Join(binDir, "machinator"), "--once", "--headless")
	orchestratorCmd.Dir = projectDir
	orchestratorCmd.Env = append(os.Environ(),
		"PATH="+newPath,
		"GEMINI_MODE=AUTO_CLOSE",
		"BD_AGENT_NAME=Gemini-01",
		// Pass BUILD_WORKING_DIRECTORY so it knows where to find templates
		"BUILD_WORKING_DIRECTORY="+projectDir,
	)

	// Create templates dir in projectDir and copy directive_template.txt
	os.MkdirAll(filepath.Join(projectDir, "templates"), 0755)
	os.MkdirAll(filepath.Join(projectDir, "machinator", "logs"), 0755) // Orchestrator expects this

	tmplPath := filepath.Join(root, "templates", "directive_template.txt")
	tmplData, err := os.ReadFile(tmplPath)
	if err != nil {
		t.Fatalf("failed to read template: %v", err)
	}
	os.WriteFile(filepath.Join(projectDir, "templates", "directive_template.txt"), tmplData, 0644)

	// Also need AGENTS.md
	agentsData, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	os.WriteFile(filepath.Join(projectDir, "AGENTS.md"), agentsData, 0644)

	// Copy check_quota.sh
	os.MkdirAll(filepath.Join(projectDir, "machinator"), 0755)
	quotaData, _ := os.ReadFile(filepath.Join(root, "bootstrap", "check_quota.sh"))
	os.WriteFile(filepath.Join(projectDir, "machinator", "check_quota.sh"), quotaData, 0755)

	t.Log("Starting orchestrator...")
	err = orchestratorCmd.Start()
	if err != nil {
		t.Fatalf("failed to start orchestrator: %v", err)
	}

	// Wait for it to hopefully process the first task
	// Increasing to 60s because it takes some time to initialize and run the task
	time.Sleep(60 * time.Second)
	orchestratorCmd.Process.Kill()

	// 5. Verify results
	verifyCmd := exec.Command("bd", "show", "bead-1", "--json")
	verifyCmd.Dir = projectDir
	out, err := verifyCmd.Output()
	if err != nil {
		t.Fatalf("failed to run bd show: %v", err)
	}

	var tasks []Task
	if err := json.Unmarshal(out, &tasks); err != nil {
		t.Fatalf("failed to parse bd show output: %v\nOutput: %s", err, string(out))
	}

	if len(tasks) == 0 {
		t.Fatal("bd show returned no tasks")
	}

	if tasks[0].Status != "closed" {
		t.Errorf("Expected bead-1 to be closed, but got %s", tasks[0].Status)

		// Print logs for debugging
		logPath := filepath.Join(projectDir, "machinator", "logs", "tui_debug.log")
		if logData, err := os.ReadFile(logPath); err == nil {
			t.Logf("Orchestrator logs:\n%s", string(logData))
		} else {
			t.Logf("Could not read logs at %s: %v", logPath, err)
		}
	} else {
		t.Log("Success: bead-1 is closed")
	}
}

func buildBin(t *testing.T, root, target, dest string) {
	t.Logf("Building %s...", target)
	cmd := exec.Command("bazel", "build", target)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bazel build %s failed: %v\nOutput: %s", target, err, string(out))
	}

	// Use 'bazel cquery' to find the output file, but try to be very specific
	outCmd := exec.Command("bazel", "cquery", "--output=files", target)
	outCmd.Dir = root
	out, err := outCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bazel cquery failed: %v\nOutput: %s", err, string(out))
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var src string
	for _, line := range lines {
		if strings.HasPrefix(line, "bazel-out/") {
			src = filepath.Join(root, line)
			break
		}
	}

	if src == "" {
		t.Fatalf("could not find binary path in cquery output: %s", string(out))
	}

	t.Logf("Copying %s to %s", src, dest)
	input, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("failed to read %s: %v", src, err)
	}
	if err := os.WriteFile(dest, input, 0755); err != nil {
		t.Fatalf("failed to write %s: %v", dest, err)
	}
}
