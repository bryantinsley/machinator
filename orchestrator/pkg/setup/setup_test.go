package setup

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

// TestViewRenderingAtDifferentSizes tests layout at various terminal sizes
func TestViewRenderingAtDifferentSizes(t *testing.T) {
	sizes := []struct {
		name          string
		width, height int
	}{
		{"80x24", 80, 24},
		{"100x36", 100, 36},
		{"120x40", 120, 40},
		{"200x50", 200, 50},
	}

	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			m := initialModel()
			m.width = size.width
			m.height = size.height
			m.screen = screenMain
			m.projectsLoaded = true

			view := m.View()

			// Check no rendering errors
			if strings.Contains(view, "panic") {
				t.Errorf("View at %s contains panic", size.name)
			}

			// Check key elements present
			if !strings.Contains(view, "Project Settings") {
				t.Errorf("View at %s missing 'Project Settings'", size.name)
			}
			if !strings.Contains(view, "Doctinator") {
				t.Errorf("View at %s missing 'Doctinator'", size.name)
			}
			if !strings.Contains(view, "Status") {
				t.Errorf("View at %s missing 'Status'", size.name)
			}
		})
	}
}

// TestDeleteDefaultsToNo verifies delete dialog defaults to "No"
func TestDeleteDefaultsToNo(t *testing.T) {
	m := initialModel()
	m.width = 100
	m.height = 36
	m.projectsLoaded = true
	m.projects = []ProjectConfig{
		{ID: 1, Name: "test-project", AgentCount: 1},
	}
	m.selectedProject = 0
	m.screen = screenProjectDetail
	m.detailCursor = 4 // Delete button (was 3 before Open button added)

	// Simulate pressing enter on delete
	newModel, _ := m.handleProjectDetailKeys("enter")
	m2 := newModel.(model)

	if m2.dialogCursor != 1 {
		t.Errorf("Delete dialog should default to 'No' (1), got %d", m2.dialogCursor)
	}
}

// TestEscapeReturnsToMainScreen tests ESC navigation
func TestEscapeReturnsToMainScreen(t *testing.T) {
	m := initialModel()
	m.screen = screenProjectDetail
	m.projectsLoaded = true
	m.projects = []ProjectConfig{{ID: 1, Name: "test"}}
	m.selectedProject = 0

	newModel, _ := m.handleProjectDetailKeys("esc")
	m2 := newModel.(model)

	if m2.screen != screenMain {
		t.Errorf("ESC should return to main screen, got %v", m2.screen)
	}
}

// TestRunShortcut verifies that 'r' shortcut runs the selected project
func TestRunShortcut(t *testing.T) {
	m := initialModel()
	m.projectsLoaded = true
	m.projects = []ProjectConfig{
		{ID: 1, Name: "test-project", AgentCount: 1},
	}
	m.cursor = 1 // On the first project

	// Press 'r'
	newModel, cmd := m.handleMainKeys("r")
	m2 := newModel.(model)

	if m2.selectedProjectConfig == nil {
		t.Error("Project config should be selected")
	}
	if m2.selectedProjectConfig.Name != "test-project" {
		t.Errorf("Expected project 'test-project', got %s", m2.selectedProjectConfig.Name)
	}
	if cmd == nil {
		t.Error("Should return a command (Quit)")
	} else {
		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Errorf("Expected tea.QuitMsg, got %T", msg)
		}
	}
}

// TestEditFieldTabNavigation tests that tab cycles through input/save/cancel
func TestEditFieldTabNavigation(t *testing.T) {
	m := initialModel()
	m.screen = screenEditProjectName
	m.editCursor = 0 // Start at input
	m.editBuffer = "test"
	m.projects = []ProjectConfig{{ID: 1, Name: "original"}}
	m.selectedProject = 0

	// Tab should move to save
	m2, _ := m.handleEditFieldKeys("tab", tea.KeyMsg{Type: tea.KeyTab})
	m3 := m2.(model)
	if m3.editCursor != 1 {
		t.Errorf("Expected editCursor=1 (save), got %d", m3.editCursor)
	}

	// Tab again should move to cancel
	m4, _ := m3.handleEditFieldKeys("tab", tea.KeyMsg{Type: tea.KeyTab})
	m5 := m4.(model)
	if m5.editCursor != 2 {
		t.Errorf("Expected editCursor=2 (cancel), got %d", m5.editCursor)
	}

	// Tab again should wrap to input
	m6, _ := m5.handleEditFieldKeys("tab", tea.KeyMsg{Type: tea.KeyTab})
	m7 := m6.(model)
	if m7.editCursor != 0 {
		t.Errorf("Expected editCursor=0 (input), got %d", m7.editCursor)
	}
}

// TestAgentCountTabNavigation tests tab navigation in agent count modal
func TestAgentCountTabNavigation(t *testing.T) {
	m := initialModel()
	m.screen = screenEditAgentCount
	m.agentCursor = 0 // Start at number
	m.desiredAgentCount = 2
	m.projects = []ProjectConfig{{ID: 1, Name: "test", AgentCount: 1}}
	m.selectedProject = 0

	// Tab should move to apply
	m2, _ := m.handleEditAgentCountKeys("tab")
	m3 := m2.(model)
	if m3.agentCursor != 1 {
		t.Errorf("Expected agentCursor=1 (apply), got %d", m3.agentCursor)
	}

	// Tab again should move to cancel
	m4, _ := m3.handleEditAgentCountKeys("tab")
	m5 := m4.(model)
	if m5.agentCursor != 2 {
		t.Errorf("Expected agentCursor=2 (cancel), got %d", m5.agentCursor)
	}
}

// TestModalRendersWithoutCrash tests that all modal renders don't crash
func TestModalRendersWithoutCrash(t *testing.T) {
	m := initialModel()
	m.width = 100
	m.height = 36
	m.projectsLoaded = true
	m.projects = []ProjectConfig{{ID: 1, Name: "test", AgentCount: 1, RepoURL: "https://github.com/test/repo"}}
	m.selectedProject = 0

	// Test project detail modal
	m.screen = screenProjectDetail
	view := m.renderProjectDetailModal()
	if len(view) == 0 {
		t.Error("Project detail modal should not be empty")
	}

	// Test edit field modal
	m.screen = screenEditProjectName
	m.editBuffer = "test value"
	view = m.renderEditFieldModal()
	if len(view) == 0 {
		t.Error("Edit field modal should not be empty")
	}

	// Test agent count modal
	m.screen = screenEditAgentCount
	m.desiredAgentCount = 2
	view = m.renderEditAgentCountModal()
	if len(view) == 0 {
		t.Error("Agent count modal should not be empty")
	}

	// Test exit dialog
	view = m.renderExitDialog()
	if !strings.Contains(view, "Exit Machinator?") {
		t.Error("Exit dialog should contain 'Exit Machinator?'")
	}

	// Test delete dialog
	view = m.renderDeleteDialog()
	if !strings.Contains(view, "Delete Project?") {
		t.Error("Delete dialog should contain 'Delete Project?'")
	}
}

// TestAddAccountFlow tests the step-by-step account addition flow
func TestAddAccountFlow(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "machinator-account-test")
	defer os.RemoveAll(tempDir)

	m := initialModel()
	m.machinatorDir = tempDir
	m.screen = screenManageAccounts

	// 1. Press 'a' to add account
	m2, _ := m.handleManageAccountsKeys("a")
	m = m2.(model)
	if m.screen != screenAddAccountName {
		t.Errorf("Expected screenAddAccountName, got %v", m.screen)
	}

	// 2. Type name and press Enter
	m.inputBuffer = "test-account"
	m2, _ = m.handleAddAccountNameKeys("enter", tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(model)
	if m.screen != screenAddAccountAuthType {
		t.Errorf("Expected screenAddAccountAuthType, got %v", m.screen)
	}
	if m.newAccountName != "test-account" {
		t.Errorf("Expected name 'test-account', got %s", m.newAccountName)
	}

	// 3. Select API Key (default cursor 0) and press Enter
	m2, _ = m.handleAddAccountAuthTypeKeys("enter")
	m = m2.(model)
	if m.screen != screenAddAccountAPIKey {
		t.Errorf("Expected screenAddAccountAPIKey, got %v", m.screen)
	}

	// 4. Type API key and press Enter
	m.inputBuffer = "AIza-test-key"
	m2, cmd := m.handleAddAccountAPIKeyKeys("enter", tea.KeyMsg{Type: tea.KeyEnter})
	m = m2.(model)

	// Execute the finishAddAccount command
	msg := cmd()
	if _, ok := msg.(accountAddedMsg); !ok {
		t.Errorf("Expected accountAddedMsg, got %T", msg)
	}

	// Verify file creation
	accountDir := filepath.Join(tempDir, "accounts", "test-account")
	if _, err := os.Stat(filepath.Join(accountDir, "account.json")); err != nil {
		t.Errorf("account.json not created")
	}
	if _, err := os.Stat(filepath.Join(accountDir, ".gemini", "settings.json")); err != nil {
		t.Errorf("settings.json not created")
	}
}

// TestDialogCursorNavigation tests left/right/tab navigation in dialogs
func TestDialogCursorNavigation(t *testing.T) {
	m := initialModel()
	m.screen = screenConfirmExit
	m.dialogCursor = 0 // Start at Yes

	// Right/Tab should move to No
	m2, _ := m.handleConfirmExitKeys("right")
	m3 := m2.(model)
	if m3.dialogCursor != 1 {
		t.Errorf("Expected dialogCursor=1 (no), got %d", m3.dialogCursor)
	}

	// Left should move back to Yes
	m4, _ := m3.handleConfirmExitKeys("left")
	m5 := m4.(model)
	if m5.dialogCursor != 0 {
		t.Errorf("Expected dialogCursor=0 (yes), got %d", m5.dialogCursor)
	}
}

// TestNoOutputTruncation ensures long content doesn't get cut off
func TestNoOutputTruncation(t *testing.T) {
	m := initialModel()
	m.width = 100
	m.height = 36
	m.screen = screenMain
	m.projectsLoaded = true
	// Add many projects
	for i := 1; i <= 10; i++ {
		m.projects = append(m.projects, ProjectConfig{
			ID:   i,
			Name: "project" + string(rune('0'+i)),
		})
	}

	view := m.View()
	lines := strings.Split(view, "\n")

	// Check we have reasonable output
	if len(lines) < 10 {
		t.Error("View should have multiple lines")
	}
}

func TestMainScreenGolden(t *testing.T) {
	m := initialModel()
	m.width = 81
	m.height = 24
	m.screen = screenMain
	m.projectsLoaded = true
	m.projects = []ProjectConfig{
		{ID: 1, Name: "Project 1", AgentCount: 1},
		{ID: 2, Name: "Project 2", AgentCount: 2},
	}
	m.cursor = 1

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(81, 24))
	tm.Send(tea.Quit())
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	out, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatal(err)
	}
	teatest.RequireEqualOutput(t, out)
}

func TestProjectDetailModalGolden(t *testing.T) {
	m := initialModel()
	m.width = 80
	m.height = 24
	m.screen = screenProjectDetail
	m.projectsLoaded = true
	m.projects = []ProjectConfig{
		{ID: 1, Name: "Test Project", AgentCount: 1, RepoURL: "git@github.com:test/repo.git"},
	}
	m.selectedProject = 0
	m.detailCursor = 0

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
	tm.Send(tea.Quit())
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	out, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatal(err)
	}
	teatest.RequireEqualOutput(t, out)
}

func TestEditFieldModalGolden(t *testing.T) {
	m := initialModel()
	m.width = 80
	m.height = 24
	m.screen = screenEditProjectName
	m.projectsLoaded = true
	m.projects = []ProjectConfig{
		{ID: 1, Name: "Test Project", AgentCount: 1, RepoURL: "git@github.com:test/repo.git"},
	}
	m.selectedProject = 0
	m.editBuffer = "New Project Name"
	m.editCursor = 0

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
	tm.Send(tea.Quit())
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	out, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatal(err)
	}
	teatest.RequireEqualOutput(t, out)
}

func TestDeleteConfirmationModalGolden(t *testing.T) {
	m := initialModel()
	m.width = 80
	m.height = 24
	m.screen = screenConfirmDeleteProject
	m.projectsLoaded = true
	m.projects = []ProjectConfig{
		{ID: 1, Name: "Test Project", AgentCount: 1, RepoURL: "git@github.com:test/repo.git"},
	}
	m.selectedProject = 0
	m.dialogCursor = 1 // Default to No

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
	tm.Send(tea.Quit())
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	out, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatal(err)
	}
	teatest.RequireEqualOutput(t, out)
}

func TestInitScreenGolden(t *testing.T) {
	m := initialModel()
	m.width = 80
	m.height = 24
	m.screen = screenInit
	m.machinatorExists = false
	m.machinatorDir = "/tmp/.machinator"

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
	tm.Send(tea.Quit())
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	out, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatal(err)
	}
	teatest.RequireEqualOutput(t, out)
}

func TestManageAccountsGolden(t *testing.T) {
	m := initialModel()
	m.width = 80
	m.height = 24
	m.screen = screenManageAccounts
	m.accounts = []AccountInfo{
		{Name: "default", Authenticated: true, AuthType: "api_key"},
		{Name: "secondary", Authenticated: false, AuthType: "google"},
	}
	m.accountCursor = 0

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
	tm.Send(tea.Quit())
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	out, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatal(err)
	}
	teatest.RequireEqualOutput(t, out)
}

func TestAddAccountNameGolden(t *testing.T) {
	m := initialModel()
	m.width = 80
	m.height = 24
	m.screen = screenAddAccountName
	m.inputPrompt = "Account Name"
	m.inputBuffer = "new-account"

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
	tm.Send(tea.Quit())
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	out, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatal(err)
	}
	teatest.RequireEqualOutput(t, out)
}

func TestAddAccountAuthTypeGolden(t *testing.T) {
	m := initialModel()
	m.width = 80
	m.height = 24
	m.screen = screenAddAccountAuthType
	m.newAccountName = "test-acc"
	m.dialogCursor = 0

	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
	tm.Send(tea.Quit())
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
	out, err := io.ReadAll(tm.FinalOutput(t))
	if err != nil {
		t.Fatal(err)
	}
	teatest.RequireEqualOutput(t, out)
}
