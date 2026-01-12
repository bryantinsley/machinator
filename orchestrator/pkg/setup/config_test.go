package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetMachinatorDir(t *testing.T) {
	// Save original env
	original := os.Getenv("MACHINATOR_DIR")
	defer os.Setenv("MACHINATOR_DIR", original)

	t.Run("Default", func(t *testing.T) {
		os.Unsetenv("MACHINATOR_DIR")
		homeDir, _ := os.UserHomeDir()
		expected := filepath.Join(homeDir, ".machinator")
		actual := GetMachinatorDir()
		if actual != expected {
			t.Errorf("expected %s, got %s", expected, actual)
		}
	})

	t.Run("Custom", func(t *testing.T) {
		customDir := "/tmp/custom-machinator"
		os.Setenv("MACHINATOR_DIR", customDir)
		actual := GetMachinatorDir()
		if actual != customDir {
			t.Errorf("expected %s, got %s", customDir, actual)
		}
	})
}

func TestLoadProject(t *testing.T) {
	// Setup temp machinator dir
	tempDir, err := os.MkdirTemp("", "machinator-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set env
	original := os.Getenv("MACHINATOR_DIR")
	defer os.Setenv("MACHINATOR_DIR", original)
	os.Setenv("MACHINATOR_DIR", tempDir)

	// Create project structure
	projectsDir := filepath.Join(tempDir, "projects")
	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		t.Fatalf("failed to create projects dir: %v", err)
	}

	// Create project 1
	p1Dir := filepath.Join(projectsDir, "project-1")
	if err := os.MkdirAll(p1Dir, 0755); err != nil {
		t.Fatalf("failed to create p1 dir: %v", err)
	}
	p1Config := `{"id": 1, "name": "Project One"}`
	if err := os.WriteFile(filepath.Join(p1Dir, "project.json"), []byte(p1Config), 0644); err != nil {
		t.Fatalf("failed to write p1 config: %v", err)
	}

	t.Run("ExistingProject", func(t *testing.T) {
		config, err := LoadProject(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.ID != 1 {
			t.Errorf("expected ID 1, got %d", config.ID)
		}
		if config.Name != "Project One" {
			t.Errorf("expected name 'Project One', got '%s'", config.Name)
		}
	})

	t.Run("NonExistingProject", func(t *testing.T) {
		_, err := LoadProject(999)
		if err == nil {
			t.Error("expected error for non-existing project, got nil")
		}
	})
}

func TestWorktreeStrategy(t *testing.T) {
	t.Run("DefaultValue", func(t *testing.T) {
		p := &ProjectConfig{}
		if p.GetWorktreeStrategy() != WorktreeStrategyPerInvocation {
			t.Errorf("expected default strategy %s, got %s", WorktreeStrategyPerInvocation, p.GetWorktreeStrategy())
		}
	})

	t.Run("ExplicitValue", func(t *testing.T) {
		p := &ProjectConfig{WorktreeStrategy: WorktreeStrategyPersistent}
		if p.GetWorktreeStrategy() != WorktreeStrategyPersistent {
			t.Errorf("expected strategy %s, got %s", WorktreeStrategyPersistent, p.GetWorktreeStrategy())
		}
	})

	t.Run("Validation", func(t *testing.T) {
		validStrategies := []string{
			WorktreeStrategyPerTask,
			WorktreeStrategyPerInvocation,
			WorktreeStrategyPersistent,
			"", // Should default to per-invocation and be valid
		}

		for _, s := range validStrategies {
			p := &ProjectConfig{WorktreeStrategy: s}
			if err := p.Validate(); err != nil {
				t.Errorf("expected strategy '%s' to be valid, got error: %v", s, err)
			}
		}

		invalidStrategies := []string{
			"invalid",
			"per-project",
			"random",
		}

		for _, s := range invalidStrategies {
			p := &ProjectConfig{WorktreeStrategy: s}
			if err := p.Validate(); err == nil {
				t.Errorf("expected strategy '%s' to be invalid, got nil error", s)
			}
		}
	})
}
