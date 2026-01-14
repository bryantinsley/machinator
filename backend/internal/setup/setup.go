package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Setup handles environment initialization.
type Setup struct {
	MachinatorDir string
}

// New creates a new Setup instance.
func New(machinatorDir string) *Setup {
	return &Setup{MachinatorDir: machinatorDir}
}

// EnsureDirectories creates the required directory structure.
func (s *Setup) EnsureDirectories() error {
	dirs := []string{
		filepath.Join(s.MachinatorDir, "accounts"),
		filepath.Join(s.MachinatorDir, "projects"),
		filepath.Join(s.MachinatorDir, "logs"),
		filepath.Join(s.MachinatorDir, "bin"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return nil
}

// EnsureGeminiCLI downloads and installs the specialized gemini-cli if needed.
func (s *Setup) EnsureGeminiCLI() (string, error) {
	binDir := filepath.Join(s.MachinatorDir, "bin")
	geminiPath := filepath.Join(binDir, "gemini")

	// Check if already installed
	if _, err := os.Stat(geminiPath); err == nil {
		return geminiPath, nil
	}

	fmt.Println("Installing gemini-cli...")

	// Download URL based on OS/arch
	url := getGeminiDownloadURL()
	if url == "" {
		return "", fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Download
	tarPath := filepath.Join(binDir, "gemini.tar.gz")
	cmd := exec.Command("curl", "-L", "-o", tarPath, url)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("download gemini: %w", err)
	}

	// Extract
	cmd = exec.Command("tar", "-xzf", tarPath, "-C", binDir)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("extract gemini: %w", err)
	}

	// Cleanup
	os.Remove(tarPath)

	// Make executable
	if err := os.Chmod(geminiPath, 0755); err != nil {
		return "", fmt.Errorf("chmod gemini: %w", err)
	}

	fmt.Println("gemini-cli installed successfully")
	return geminiPath, nil
}

// CloneRepo clones or updates the project repository.
func (s *Setup) CloneRepo(projectID int, repoURL, branch string) (string, error) {
	projectDir := filepath.Join(s.MachinatorDir, "projects", fmt.Sprintf("%d", projectID))
	repoDir := filepath.Join(projectDir, "repo")

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return "", fmt.Errorf("create project dir: %w", err)
	}

	// Check if repo already exists
	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
		// Already cloned, fetch latest
		fmt.Printf("Fetching latest from %s...\n", repoURL)
		cmd := exec.Command("git", "-C", repoDir, "fetch", "origin")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("git fetch: %w", err)
		}

		cmd = exec.Command("git", "-C", repoDir, "checkout", branch)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("git checkout: %w", err)
		}

		cmd = exec.Command("git", "-C", repoDir, "reset", "--hard", "origin/"+branch)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("git reset: %w", err)
		}
	} else {
		// Clone fresh
		fmt.Printf("Cloning %s...\n", repoURL)
		cmd := exec.Command("git", "clone", "-b", branch, repoURL, repoDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("git clone: %w", err)
		}
	}

	return repoDir, nil
}

// CreateWorktree creates an agent worktree for a project.
func (s *Setup) CreateWorktree(projectID, agentID int, branch string) (string, error) {
	projectDir := filepath.Join(s.MachinatorDir, "projects", fmt.Sprintf("%d", projectID))
	repoDir := filepath.Join(projectDir, "repo")
	agentDir := filepath.Join(projectDir, "agents", fmt.Sprintf("%d", agentID))

	// Remove existing worktree if present
	if _, err := os.Stat(agentDir); err == nil {
		cmd := exec.Command("git", "-C", repoDir, "worktree", "remove", "--force", agentDir)
		cmd.Run() // Ignore errors
		os.RemoveAll(agentDir)
	}

	// Create new worktree
	cmd := exec.Command("git", "-C", repoDir, "worktree", "add", "--detach", agentDir, "origin/"+branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git worktree add: %w", err)
	}

	return agentDir, nil
}

// ResetWorktree resets a worktree to a clean state.
func (s *Setup) ResetWorktree(worktreeDir, branch string) error {
	cmd := exec.Command("git", "-C", worktreeDir, "fetch", "origin")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}

	cmd = exec.Command("git", "-C", worktreeDir, "reset", "--hard", "origin/"+branch)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git reset: %w", err)
	}

	cmd = exec.Command("git", "-C", worktreeDir, "clean", "-fd")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clean: %w", err)
	}

	return nil
}

func getGeminiDownloadURL() string {
	// TODO: Update with actual download URLs
	base := "https://example.com/gemini-cli"
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return base + "/gemini-darwin-arm64.tar.gz"
		}
		return base + "/gemini-darwin-amd64.tar.gz"
	case "linux":
		return base + "/gemini-linux-amd64.tar.gz"
	default:
		return ""
	}
}
