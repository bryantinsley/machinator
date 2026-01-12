package orchestrator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bryantinsley/machinator/orchestrator/pkg/setup"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ToolsCheckState represents the state of the tools check
type ToolsCheckState int

const (
	ToolsCheckStateChecking ToolsCheckState = iota
	ToolsCheckStatePassed
	ToolsCheckStateFailed
	ToolsCheckStateInstalling
)

// ToolsCheckModel handles the logic for checking and installing required tools
type ToolsCheckModel struct {
	State         ToolsCheckState
	Err           error
	Version       string
	InstallOutput string
	Width         int
	Height        int
}

// InitialToolsCheckModel creates the initial model
func InitialToolsCheckModel() ToolsCheckModel {
	return ToolsCheckModel{
		State: ToolsCheckStateChecking,
	}
}

type toolsCheckResultMsg struct {
	err     error
	version string
}

type toolsInstallResultMsg struct {
	err    error
	output string
}

// Init performs the initial check
func (m ToolsCheckModel) Init() tea.Cmd {
	return checkGeminiCLI
}

// Update handles messages
func (m ToolsCheckModel) Update(msg tea.Msg) (ToolsCheckModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height

	case toolsCheckResultMsg:
		if msg.err != nil {
			m.State = ToolsCheckStateFailed
			m.Err = msg.err
		} else {
			m.State = ToolsCheckStatePassed
			m.Version = msg.version
		}

	case toolsInstallResultMsg:
		m.InstallOutput = msg.output
		if msg.err != nil {
			m.State = ToolsCheckStateFailed
			m.Err = fmt.Errorf("installation failed: %v", msg.err)
		} else {
			// Re-check after successful install
			m.State = ToolsCheckStateChecking
			m.Err = nil
			return m, checkGeminiCLI
		}

	case tea.KeyMsg:
		if m.State == ToolsCheckStateFailed {
			switch msg.String() {
			case "i", "I":
				m.State = ToolsCheckStateInstalling
				return m, installGeminiCLI
			case "q", "ctrl+c":
				// Let parent handle quit if needed, or we can't really do much here
				// But standard is to let parent handle it via bubbletea propagation if embedded?
				// Actually, if we return tea.Quit it quits the whole program which is what we want.
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

// View renders the tools check screen
func (m ToolsCheckModel) View() string {
	var content string

	switch m.State {
	case ToolsCheckStateChecking:
		content = "🔍 Checking for Gemini CLI..."

	case ToolsCheckStatePassed:
		content = fmt.Sprintf("✅ Gemini CLI found (v%s)", m.Version)

	case ToolsCheckStateInstalling:
		content = "⬇️  Installing Gemini CLI...\n\n" +
			"Running: go install ./tools/dummy-gemini"

	case ToolsCheckStateFailed:
		content = fmt.Sprintf("❌ Gemini CLI check failed\n\nError: %v\n\n", m.Err)
		if m.InstallOutput != "" {
			content += fmt.Sprintf("Install Output:\n%s\n\n", m.InstallOutput)
		}
		content += "The 'gemini' command is required to run the orchestrator.\n"
		content += "Press [i] to install it automatically.\n"
		content += "Press [q] to quit."
	}

	// Center the content
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(2, 4).
		Align(lipgloss.Center).
		Width(60)

	return lipgloss.Place(
		m.Width,
		m.Height,
		lipgloss.Center,
		lipgloss.Center,
		style.Render(content),
	)
}

// checkGeminiCLI checks if gemini is available
func checkGeminiCLI() tea.Msg {
	// Give the UI a moment to render "Checking..."
	time.Sleep(500 * time.Millisecond)

	machinatorDir := setup.GetMachinatorDir()
	path := filepath.Join(machinatorDir, "gemini")

	if _, err := os.Stat(path); err != nil {
		return toolsCheckResultMsg{err: fmt.Errorf("managed gemini not found at %s", path)}
	}

	cmd := exec.Command(path, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return toolsCheckResultMsg{err: fmt.Errorf("failed to run 'gemini --version': %v", err)}
	}

	version := strings.TrimSpace(string(out))
	if version == "" {
		// Even if empty, if it runs exit 0, maybe it's ok?
		// But usually --version prints something.
		// Let's accept it but warn? Or fail.
		// For robustness, let's treat empty as "Unknown" but passed, unless we really need strict check.
		version = "Unknown"
	}

	return toolsCheckResultMsg{version: version}
}

// installGeminiCLI installs the dummy-gemini tool
func installGeminiCLI() tea.Msg {
	projectRoot := getProjectRoot() // Defined in tui.go

	// We'll install the dummy-gemini tool
	// Assuming running in a context where go is available and GOPATH is set up (checked by env setup)

	cmd := exec.Command("go", "install", "./tools/dummy-gemini")
	cmd.Dir = projectRoot

	out, err := cmd.CombinedOutput()
	output := string(out)

	// If it fails, maybe try to rename the binary if the package name is main?
	// go install usually puts it in $GOBIN/dummy-gemini if the directory is named that.
	// But we need it named "gemini".
	// Let's check where it put it.

	if err == nil {
		// Move/Symlink dummy-gemini to gemini if needed
		// Check if "gemini" is in path now.
		// If GOBIN is in path, check if gemini exists there.

		// If the package main is in dummy-gemini, the binary is likely named 'dummy-gemini' or 'fixture-gen' depending on go.mod?
		// Actually, go install uses the directory name usually.

		// Let's force a rename if we can find it.
		// Or better, build it specifically as 'gemini'.
		machinatorDir := setup.GetMachinatorDir()
		os.MkdirAll(machinatorDir, 0755)
		buildCmd := exec.Command("go", "build", "-o", filepath.Join(machinatorDir, "gemini"), "./tools/dummy-gemini")
		buildCmd.Dir = projectRoot
		out2, err2 := buildCmd.CombinedOutput()
		output += "\nBuild output:\n" + string(out2)
		if err2 != nil {
			return toolsInstallResultMsg{err: err2, output: output}
		}
		return toolsInstallResultMsg{err: nil, output: output}
	}

	return toolsInstallResultMsg{err: err, output: output}
}
