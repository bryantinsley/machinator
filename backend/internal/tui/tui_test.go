package tui

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bryantinsley/machinator/backend/internal/beads"
	"github.com/bryantinsley/machinator/backend/internal/config"
	"github.com/bryantinsley/machinator/backend/internal/project"
	"github.com/bryantinsley/machinator/backend/internal/quota"
	"github.com/bryantinsley/machinator/backend/internal/state"
)

var update = flag.Bool("update", false, "update golden files")

func renderToString(t *TUI) string {
	// Set dimensions to make rendering deterministic
	t.leftWidth = 40
	t.rightWidth = 80
	t.rightHeight = 24

	// Just testing the left panel per the requirements
	return t.buildLeftContent()
}

func checkGolden(t *testing.T, name string, got string) {
	t.Helper()
	goldenFile := filepath.Join("testdata", name+".golden")

	if *update {
		workspaceDir := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
		if workspaceDir != "" {
			goldenFile = filepath.Join(workspaceDir, "backend", "internal", "tui", "testdata", name+".golden")
		}

		dir := filepath.Dir(goldenFile)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create testdata dir: %v", err)
		}
		if err := os.WriteFile(goldenFile, []byte(got), 0644); err != nil {
			t.Fatalf("failed to update golden file: %v", err)
		}
		return
	}

	expected, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatalf("failed to read golden file %s: %v\nRun with -update flag to create it.", goldenFile, err)
	}

	if string(expected) != got {
		t.Errorf("output mismatch for %s.\nExpected:\n%s\nGot:\n%s", name, expected, got)
	}
}

func setupTestTUI() *TUI {
	st := state.New("dummy_repo")
	q := &quota.Quota{}
	cfg := &config.Config{}
	projCfg := &project.Config{
		SimpleModelName:  "gemini-3-flash",
		ComplexModelName: "gemini-3-pro",
	}

	ui := New(st, q, "dummy_repo", cfg, projCfg, "")
	// Prevent async data loading by setting cache times far in the future
	future := time.Now().Add(24 * time.Hour)
	ui.cachedTasksTime = future
	ui.cachedGitLogTime = future
	return ui
}

func TestLeftPanel_EmptyState(t *testing.T) {
	ui := setupTestTUI()
	got := renderToString(ui)
	checkGolden(t, "empty_state", got)
}

func TestLeftPanel_WithTasks(t *testing.T) {
	ui := setupTestTUI()
	ui.cachedTasks = []*beads.Task{
		{ID: "task-1", Status: "open"},
		{ID: "task-2", Status: "open", BlockedBy: []string{"task-1"}}, // Blocked
		{ID: "task-3", Status: "in_progress"},                         // Assigned
	}
	got := renderToString(ui)
	checkGolden(t, "with_tasks", got)
}

func TestLeftPanel_WithAgents(t *testing.T) {
	ui := setupTestTUI()
	ui.state.Agents = []*state.Agent{
		{ID: 1, State: "pending"},
		{ID: 2, State: "ready"},
		{ID: 3, State: "assigned", TaskID: "task-1"}, 
	}
	// To make output deterministic, we don't set StartedAt, so elapsed time isn't shown
	
got := renderToString(ui)
	checkGolden(t, "with_agents", got)
}

func TestLeftPanel_WithCriticalError(t *testing.T) {
	ui := setupTestTUI()
	ui.addCriticalError("test", "Something went critically wrong")
	got := renderToString(ui)
	checkGolden(t, "with_critical_error", got)
}
