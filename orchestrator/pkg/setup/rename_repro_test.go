package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRenamePersistence(t *testing.T) {
	// Setup a temporary projects directory
	tmpDir, err := os.MkdirTemp("", "machinator-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	projectsDir := filepath.Join(tmpDir, "projects")
	err = os.MkdirAll(projectsDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create a dummy project with directory name NOT matching ID
	projectID := 1
	dirName := "my-project"
	projectDir := filepath.Join(projectsDir, dirName)
	err = os.MkdirAll(projectDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	config := ProjectConfig{
		ID:      projectID,
		Name:    "Original Name",
		DirName: dirName,
	}
	configData, _ := json.MarshalIndent(config, "", "  ")
	err = os.WriteFile(filepath.Join(projectDir, "project.json"), configData, 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Initialize model
	m := initialModel()
	m.projectsDir = projectsDir
	m.projects = []ProjectConfig{config}
	// We need to simulate how loadProjects would have loaded it
	// But since loadProjects is not easily mockable here without changing more things,
	// let's just see if updateProjectConfig uses the ID.
	m.selectedProject = 0
	m.screen = screenEditProjectName
	m.editBuffer = "New Name"
	m.editCursor = 1 // Save button

	// Simulate pressing Enter on Save button
	newModel, _ := m.handleEditFieldKeys("enter", tea.KeyMsg{Type: tea.KeyEnter})
	m2 := newModel.(model)

	// Check if in-memory model is updated
	if m2.projects[0].Name != "New Name" {
		t.Errorf("Expected in-memory name to be 'New Name', got '%s'", m2.projects[0].Name)
	}

	// Check if file on disk is updated
	updatedData, err := os.ReadFile(filepath.Join(projectDir, "project.json"))
	if err != nil {
		t.Fatal(err)
	}

	var updatedConfig ProjectConfig
	err = json.Unmarshal(updatedData, &updatedConfig)
	if err != nil {
		t.Fatal(err)
	}

	if updatedConfig.Name != "New Name" {
		t.Errorf("Expected on-disk name to be 'New Name' in %s, got '%s'", projectDir, updatedConfig.Name)
	}
}
