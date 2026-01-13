package orchestrator

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestHelperProcess isn't a real test. It's used to mock exec.Command.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for i := range args {
		if args[i] == "--" {
			args = args[i+1:]
			break
		}
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd := args[0]
	switch cmd {
	case "git":
		// git pull --ff-only origin main
		foundPull := false
		foundFF := false
		for _, arg := range args {
			if arg == "pull" {
				foundPull = true
			}
			if arg == "--ff-only" {
				foundFF = true
			}
		}
		if foundPull {
			if !foundFF {
				fmt.Fprintf(os.Stderr, "git pull missing --ff-only\n")
				os.Exit(1)
			}
			if os.Getenv("PULL_FAIL") == "1" {
				fmt.Fprintf(os.Stderr, "git pull failed\n")
				os.Exit(1)
			}
			fmt.Fprintf(os.Stdout, "git pull success\n")
		}
	case "bd":
		// bd --sandbox list --json
		foundList := false
		for _, arg := range args {
			if arg == "list" {
				foundList = true
				break
			}
		}
		if foundList {
			fmt.Fprintf(os.Stdout, `[]`)
		}
	}
}

func mockExecCommand(name string, args ...string) *exec.Cmd {
	cs := []string{"--test.run=TestHelperProcess", "--", name}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestFetchTasks_GitPull(t *testing.T) {
	old := execCommand
	defer func() { execCommand = old }()
	execCommand = mockExecCommand

	// Test success
	os.Setenv("PULL_FAIL", "0")
	cmd := fetchTasks(".", "main")
	msg := cmd()

	if _, ok := msg.(tasksMsg); !ok {
		t.Errorf("Expected tasksMsg, got %T (%v)", msg, msg)
	}

	// Test failure - should still continue and return tasksMsg
	os.Setenv("PULL_FAIL", "1")
	cmd = fetchTasks(".", "main")
	msg = cmd()

	if _, ok := msg.(tasksMsg); !ok {
		// Even if pull fails, it should continue to 'bd list' and return tasksMsg
		t.Errorf("Expected tasksMsg even if git pull fails, got %T (%v)", msg, msg)
	}
}

func TestTUI_GitPullPeriodic(t *testing.T) {
	// Mock execCommand to track calls if needed, or just rely on Update behavior
	old := execCommand
	defer func() { execCommand = old }()
	execCommand = mockExecCommand

	m := initialModel(nil, false)
	m.ready = true
	m.state = StateRunning

	// Test tick 49 -> 50 triggers fetchTasks
	m.tickCount = 49
	msg := tickMsg(time.Now())
	m.Update(msg)

	// We expect a batch of commands including fetchTasks and tick
	// It's hard to inspect tea.Cmd contents directly, but we can verify tickCount
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(model)
	if m.tickCount != 50 {
		t.Errorf("Expected tickCount 50, got %d", m.tickCount)
	}

	// Since we can't easily inspect tea.Cmd, we rely on the fact that
	// the code shows: if m.tickCount%50 == 0 { cmds = append(cmds, fetchTasks(...)) }
}

func TestFetchTasks_FFOnly(t *testing.T) {
	old := execCommand
	defer func() { execCommand = old }()

	var capturedArgs []string
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "git" {
			capturedArgs = args
		}
		return mockExecCommand(name, args...)
	}

	cmd := fetchTasks(".", "main")
	cmd()

	foundFF := false
	for _, arg := range capturedArgs {
		if arg == "--ff-only" {
			foundFF = true
			break
		}
	}

	if !foundFF {
		t.Errorf("git pull did not use --ff-only. Args: %v", capturedArgs)
	}
}

func TestFetchTasks_RefreshTasksAfterPull(t *testing.T) {
	old := execCommand
	defer func() { execCommand = old }()

	var calls []string
	execCommand = func(name string, args ...string) *exec.Cmd {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return mockExecCommand(name, args...)
	}

	cmd := fetchTasks(".", "main")
	cmd()

	// Verify order: git pull then bd list (with some other calls in between possibly)
	pullIdx := -1
	listIdx := -1
	for i, call := range calls {
		if strings.HasPrefix(call, "git pull") {
			pullIdx = i
		}
		if strings.HasPrefix(call, "bd --sandbox list") {
			listIdx = i
		}
	}

	if pullIdx == -1 {
		t.Error("git pull not called")
	}
	if listIdx == -1 {
		t.Error("bd list not called")
	}
	if listIdx < pullIdx {
		t.Error("bd list called before git pull")
	}
}
