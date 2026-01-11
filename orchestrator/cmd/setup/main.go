package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	geminiCLIRepo = "https://github.com/bryantinsley/gemini-cli-mods.git"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Styles
// ═══════════════════════════════════════════════════════════════════════════════

var (
	// Layout
	leftPaneStyle = lipgloss.NewStyle().
			Width(40).
			Height(20).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)

	rightPaneStyle = lipgloss.NewStyle().
			Width(50).
			Height(20).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86")).
			MarginTop(1)

	itemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212"))

	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("196")).
			Padding(1, 3).
			Align(lipgloss.Center)

	statusOK = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82")).
			Render("✓")

	statusFail = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render("✗")

	statusWarn = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Render("⚠")

	statusPending = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("○")

	statusLoading = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Render("◐")
)

// ═══════════════════════════════════════════════════════════════════════════════
// Data Types
// ═══════════════════════════════════════════════════════════════════════════════

type ProjectConfig struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	RepoURL    string `json:"repo_url"`
	AgentCount int    `json:"agent_count"`
	HasBeads   bool   `json:"has_beads"`
	TasksReady int    `json:"tasks_ready"`
	TasksOpen  int    `json:"tasks_open"`
	TasksDone  int    `json:"tasks_done"`
	TasksTotal int    `json:"tasks_total"`
	CreatedAt  string `json:"created_at"`
}

type GlobalSettings struct {
	GeminiCLIPath string `json:"gemini_cli_path"`
	GeminiCLIRepo string `json:"gemini_cli_repo"`
	InitializedAt string `json:"initialized_at"`
	NextProjectID int    `json:"next_project_id"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// Model
// ═══════════════════════════════════════════════════════════════════════════════

type screen int

const (
	screenInit screen = iota
	screenMain
	screenProjectDetail
	screenAddProject
	screenAddProjectInput
	screenAddProjectCloning
	screenEditAgentCount
	screenApplyingAgents
	screenConfirmExit
)

type geminiStatus int

const (
	geminiUnknown geminiStatus = iota
	geminiChecking
	geminiNotInstalled
	geminiInstalled
	geminiNeedsUpdate
	geminiInstalling
)

type model struct {
	screen         screen
	width, height  int
	cursor         int
	err            error
	statusMessages []string

	// Machinator state
	machinatorExists bool
	geminiStatus     geminiStatus
	geminiVersion    string

	// Projects
	projects          []ProjectConfig
	projectsLoaded    bool
	selectedProject   int
	desiredAgentCount int

	// Add project flow
	inputBuffer    string
	inputPrompt    string
	inputHint      string
	inputStep      int
	newProjectID   int
	newProjectName string
	newRepoURL     string
	newProjectDir  string
	newAgentDir    string
	hasBeads       bool
	beadsTasks     int
	beadsOpen      int
	beadsDone      int
	beadsTotal     int
	addWarnings    []string
	progressMsg    string

	// Paths
	homeDir       string
	machinatorDir string
	resourcesDir  string
	projectsDir   string
	geminiCLIDir  string
	settingsFile  string
	geminiCLIPath string
}

// Messages
type initCheckMsg struct {
	machinatorExists bool
	geminiStatus     geminiStatus
	geminiVersion    string
	projects         []ProjectConfig
}

type geminiInstallMsg struct {
	success bool
	version string
	err     error
}

type cloneDoneMsg struct {
	success bool
	message string
	err     error
}

type beadsCheckMsg struct {
	hasBeads   bool
	tasksReady int
	tasksOpen  int
	tasksDone  int
	tasksTotal int
}

type projectsReloadedMsg struct {
	projects []ProjectConfig
}

type agentActionMsg struct {
	success    bool
	action     string // "add" or "remove"
	agentCount int
	err        error
}

type tickMsg time.Time

func initialModel() model {
	homeDir, _ := os.UserHomeDir()
	machinatorDir := filepath.Join(homeDir, ".machinator")
	resourcesDir := filepath.Join(machinatorDir, "resources")
	projectsDir := filepath.Join(machinatorDir, "projects")

	return model{
		screen:         screenMain,
		homeDir:        homeDir,
		machinatorDir:  machinatorDir,
		resourcesDir:   resourcesDir,
		projectsDir:    projectsDir,
		geminiCLIDir:   filepath.Join(resourcesDir, "gemini-cli-mods"),
		settingsFile:   filepath.Join(machinatorDir, "settings.json"),
		geminiCLIPath:  filepath.Join(machinatorDir, "gemini"),
		geminiStatus:   geminiChecking,
		statusMessages: []string{},
		projects:       []ProjectConfig{},
		addWarnings:    []string{},
		cursor:         1, // Start on first project or add button
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tick(), m.checkInit())
}

func tick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) checkInit() tea.Cmd {
	return func() tea.Msg {
		var result initCheckMsg

		// Check if ~/.machinator exists
		if _, err := os.Stat(m.machinatorDir); err == nil {
			result.machinatorExists = true
		}

		// Check Gemini CLI
		if _, err := os.Stat(m.geminiCLIPath); err == nil {
			result.geminiStatus = geminiChecking
			// Try to run --version
			cmd := exec.Command(m.geminiCLIPath, "--version")
			if out, err := cmd.Output(); err == nil {
				result.geminiStatus = geminiInstalled
				result.geminiVersion = strings.TrimSpace(string(out))
			} else {
				result.geminiStatus = geminiNeedsUpdate
			}
		} else {
			result.geminiStatus = geminiNotInstalled
		}

		// Load projects
		result.projects = m.loadProjects()

		return result
	}
}

func (m model) loadProjects() []ProjectConfig {
	var projects []ProjectConfig
	entries, err := os.ReadDir(m.projectsDir)
	if err != nil {
		return projects
	}

	for _, entry := range entries {
		if entry.IsDir() {
			configPath := filepath.Join(m.projectsDir, entry.Name(), "project.json")
			data, err := os.ReadFile(configPath)
			if err != nil {
				continue
			}
			var config ProjectConfig
			if json.Unmarshal(data, &config) == nil {
				projects = append(projects, config)
			}
		}
	}
	return projects
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tickMsg:
		return m, tick()

	case initCheckMsg:
		m.machinatorExists = msg.machinatorExists
		m.geminiStatus = msg.geminiStatus
		m.geminiVersion = msg.geminiVersion
		m.projects = msg.projects
		m.projectsLoaded = true

		m.addStatus("System check complete")
		// If not installed and no machinator dir, prompt to install
		if !m.machinatorExists || m.geminiStatus == geminiNotInstalled {
			// Stay on main but cursor will be on Gemini CLI option
			m.cursor = 0
		}

	case geminiInstallMsg:
		if msg.success {
			m.geminiStatus = geminiInstalled
			m.geminiVersion = msg.version
			m.addStatus("Gemini CLI installed successfully")
			m.screen = screenMain
		} else {
			m.geminiStatus = geminiNotInstalled
			m.err = msg.err
			m.addStatus("Installation failed: " + msg.err.Error())
		}

	case cloneDoneMsg:
		if msg.success {
			m.progressMsg = "Checking beads..."
			m.addStatus("Repository cloned")
			return m, m.checkBeads()
		} else {
			m.err = msg.err
			m.addStatus("Clone failed: " + msg.err.Error())
			m.screen = screenMain
		}

	case beadsCheckMsg:
		m.hasBeads = msg.hasBeads
		m.beadsTasks = msg.tasksReady
		m.beadsOpen = msg.tasksOpen
		m.beadsDone = msg.tasksDone
		m.beadsTotal = msg.tasksTotal
		if !msg.hasBeads {
			m.addWarnings = append(m.addWarnings, "No .beads directory")
		}
		m.saveNewProject()
		m.addStatus(fmt.Sprintf("Project #%d added: %s (%d tasks)", m.newProjectID, m.newProjectName, m.beadsTotal))
		m.screen = screenMain
		return m, m.reloadProjects()

	case projectsReloadedMsg:
		m.projects = msg.projects

	case agentActionMsg:
		if msg.success {
			// Update the project's agent count
			if m.selectedProject < len(m.projects) {
				m.projects[m.selectedProject].AgentCount = msg.agentCount
				m.updateProjectConfig(m.projects[m.selectedProject])
			}
			m.addStatus(fmt.Sprintf("Agent %s: now %d agent(s)", msg.action, msg.agentCount))
		} else {
			m.addStatus(fmt.Sprintf("Agent %s failed: %v", msg.action, msg.err))
		}
		m.screen = screenProjectDetail
		return m, m.reloadProjects()
	}

	return m, nil
}

func (m *model) addStatus(msg string) {
	timestamp := time.Now().Format("15:04:05")
	m.statusMessages = append([]string{fmt.Sprintf("[%s] %s", timestamp, msg)}, m.statusMessages...)
	if len(m.statusMessages) > 10 {
		m.statusMessages = m.statusMessages[:10]
	}
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global keys
	if key == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.screen {
	case screenInit:
		return m.handleInitKeys(key)
	case screenMain:
		return m.handleMainKeys(key)
	case screenProjectDetail:
		return m.handleProjectDetailKeys(key)
	case screenAddProjectInput:
		return m.handleAddProjectInputKeys(key, msg)
	case screenAddProjectCloning, screenApplyingAgents:
		// No input during clone/apply
		return m, nil
	case screenEditAgentCount:
		return m.handleEditAgentCountKeys(key)
	case screenConfirmExit:
		return m.handleConfirmExitKeys(key)
	}

	return m, nil
}

func (m model) handleInitKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "y":
		// Create machinator and install
		os.MkdirAll(m.resourcesDir, 0755)
		os.MkdirAll(m.projectsDir, 0755)
		m.machinatorExists = true
		m.geminiStatus = geminiInstalling
		m.addStatus("Installing Gemini CLI...")
		return m, m.installGemini()
	case "n", "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) handleMainKeys(key string) (tea.Model, tea.Cmd) {
	// Menu items: Gemini CLI (0), projects (1..n), Add Project (n+1), Exit (n+2)
	maxItems := len(m.projects) + 3 // gemini + projects + add + exit

	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < maxItems-1 {
			m.cursor++
		}
	case "enter":
		if m.cursor == 0 {
			// Gemini CLI - install or update if needed
			if m.geminiStatus == geminiNotInstalled || m.geminiStatus == geminiNeedsUpdate {
				os.MkdirAll(m.resourcesDir, 0755)
				os.MkdirAll(m.projectsDir, 0755)
				m.machinatorExists = true
				m.geminiStatus = geminiInstalling
				m.addStatus("Installing/updating Gemini CLI...")
				return m, m.installGemini()
			}
		} else if m.cursor <= len(m.projects) {
			// Project selected (cursor 1 = project 0)
			m.selectedProject = m.cursor - 1
			m.screen = screenProjectDetail
		} else if m.cursor == len(m.projects)+1 {
			// Add project
			m.startAddProject()
		} else {
			// Exit
			m.screen = screenConfirmExit
		}
	case "a":
		m.startAddProject()
	case "q":
		m.screen = screenConfirmExit
	}
	return m, nil
}

func (m model) handleProjectDetailKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q", "esc":
		m.screen = screenMain
	case "d":
		// Delete project
		if m.selectedProject < len(m.projects) {
			p := m.projects[m.selectedProject]
			projectDir := filepath.Join(m.projectsDir, fmt.Sprintf("%d", p.ID))
			os.RemoveAll(projectDir)
			m.addStatus(fmt.Sprintf("Removed project #%d: %s", p.ID, p.Name))
			m.screen = screenMain
			return m, m.reloadProjects()
		}
	case "a":
		// Edit agent count
		if m.selectedProject < len(m.projects) {
			m.desiredAgentCount = m.projects[m.selectedProject].AgentCount
			m.screen = screenEditAgentCount
		}
	}
	return m, nil
}

func (m model) handleEditAgentCountKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "q":
		m.screen = screenProjectDetail
	case "up", "+", "=", "k":
		m.desiredAgentCount++
		if m.desiredAgentCount > 10 {
			m.desiredAgentCount = 10 // Max 10 agents
		}
	case "down", "-", "_", "j":
		if m.desiredAgentCount > 1 {
			m.desiredAgentCount--
		}
	case "enter":
		if m.selectedProject < len(m.projects) {
			p := m.projects[m.selectedProject]
			if m.desiredAgentCount != p.AgentCount {
				m.screen = screenApplyingAgents
				m.progressMsg = "Applying changes..."
				return m, m.applyAgentChanges(p, m.desiredAgentCount)
			}
			m.screen = screenProjectDetail
		}
	}
	return m, nil
}

func (m model) handleAddProjectInputKeys(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = screenMain
		return m, nil
	case tea.KeyEnter:
		if m.inputBuffer == "" {
			return m, nil
		}
		if m.inputStep == 0 {
			m.newRepoURL = m.inputBuffer
			m.newProjectName = m.deriveProjectName(m.inputBuffer)
			m.inputStep = 1
			m.inputBuffer = m.newProjectName
			m.inputPrompt = "Project name"
			m.inputHint = "Press Enter to accept"
		} else {
			m.newProjectName = m.inputBuffer
			m.setupNewProjectPaths()
			m.screen = screenAddProjectCloning
			m.progressMsg = "Cloning..."
			m.addStatus(fmt.Sprintf("Cloning %s...", m.newRepoURL))
			return m, m.cloneRepo()
		}
	case tea.KeyBackspace:
		if len(m.inputBuffer) > 0 {
			m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
		}
	case tea.KeyRunes:
		m.inputBuffer += string(msg.Runes)
	case tea.KeySpace:
		m.inputBuffer += " "
	}
	return m, nil
}

func (m model) handleConfirmExitKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "enter":
		return m, tea.Quit
	case "n", "esc", "q":
		m.screen = screenMain
	}
	return m, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// View
// ═══════════════════════════════════════════════════════════════════════════════

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Adjust pane sizes based on terminal
	leftWidth := m.width/2 - 2
	rightWidth := m.width/2 - 2
	paneHeight := m.height - 4

	if leftWidth < 30 {
		leftWidth = 30
	}
	if rightWidth < 30 {
		rightWidth = 30
	}
	if paneHeight < 10 {
		paneHeight = 10
	}

	leftPane := leftPaneStyle.Width(leftWidth).Height(paneHeight)
	rightPane := rightPaneStyle.Width(rightWidth).Height(paneHeight)

	var left, right string

	switch m.screen {
	case screenInit:
		left = m.viewInitLeft()
		right = m.viewStatusPane()
	case screenMain:
		left = m.viewMainLeft()
		right = m.viewStatusPane()
	case screenProjectDetail:
		left = m.viewProjectDetailLeft()
		right = m.viewStatusPane()
	case screenAddProjectInput:
		left = m.viewAddProjectInputLeft()
		right = m.viewStatusPane()
	case screenAddProjectCloning, screenApplyingAgents:
		left = m.viewAddProjectCloningLeft()
		right = m.viewStatusPane()
	case screenEditAgentCount:
		left = m.viewEditAgentCountLeft()
		right = m.viewStatusPane()
	case screenConfirmExit:
		left = m.viewConfirmExitLeft()
		right = m.viewStatusPane()
	}

	leftRendered := leftPane.Render(left)
	rightRendered := rightPane.Render(right)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftRendered, rightRendered)
}

func (m model) viewInitLeft() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🔧 Machinator Setup"))
	b.WriteString("\n\n")

	if !m.machinatorExists {
		b.WriteString(warningStyle.Render("No ~/.machinator directory found.\n\n"))
		b.WriteString(itemStyle.Render("Create directory and install\ncustom Gemini CLI?\n\n"))
		b.WriteString(promptStyle.Render("[Y]es  [N]o"))
	} else if m.geminiStatus == geminiNotInstalled {
		b.WriteString(warningStyle.Render("Gemini CLI not installed.\n\n"))
		b.WriteString(itemStyle.Render("Install custom Gemini CLI?\n\n"))
		b.WriteString(promptStyle.Render("[Y]es  [N]o"))
	} else if m.geminiStatus == geminiInstalling {
		b.WriteString(itemStyle.Render("Installing Gemini CLI...\n\n"))
		b.WriteString(dimStyle.Render("This may take a few minutes."))
	}

	return b.String()
}

func (m model) viewMainLeft() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🔧 Machinator"))
	b.WriteString("\n\n")

	// Gemini CLI - now a selectable item
	cursor := "  "
	style := itemStyle
	if m.cursor == 0 {
		cursor = "▸ "
		style = selectedStyle
	}

	var geminiLine string
	switch m.geminiStatus {
	case geminiChecking:
		geminiLine = statusLoading + " " + dimStyle.Render("Gemini CLI — checking...")
	case geminiInstalled:
		geminiLine = statusOK + " " + style.Render("Gemini CLI") + dimStyle.Render(" — installed")
	case geminiNeedsUpdate:
		geminiLine = statusWarn + " " + style.Render("Gemini CLI") + warningStyle.Render(" — needs rebuild")
	case geminiInstalling:
		geminiLine = statusLoading + " " + dimStyle.Render("Gemini CLI — installing...")
	case geminiNotInstalled:
		geminiLine = statusFail + " " + style.Render("Gemini CLI") + errorStyle.Render(" — not installed")
	default:
		geminiLine = statusPending + " " + dimStyle.Render("Gemini CLI — checking...")
	}
	b.WriteString(cursor + geminiLine + "\n\n")

	// Projects section
	b.WriteString(sectionStyle.Render("Projects"))
	b.WriteString("\n")

	if !m.projectsLoaded {
		b.WriteString(dimStyle.Render("  Loading projects...\n"))
	} else if len(m.projects) == 0 {
		b.WriteString(dimStyle.Render("  No projects configured\n"))
	} else {
		for i, p := range m.projects {
			cursor := "  "
			style := itemStyle
			if i+1 == m.cursor { // +1 because gemini is at 0
				cursor = "▸ "
				style = selectedStyle
			}
			status := statusOK
			if !p.HasBeads {
				status = statusWarn
			}
			b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, status, style.Render(p.Name)))
		}
	}

	b.WriteString("\n")

	// Add project
	addIdx := len(m.projects) + 1
	cursor = "  "
	style = itemStyle
	if m.cursor == addIdx {
		cursor = "▸ "
		style = selectedStyle
	}
	b.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render("➕ Add Project")))

	// Exit
	exitIdx := len(m.projects) + 2
	cursor = "  "
	style = itemStyle
	if m.cursor == exitIdx {
		cursor = "▸ "
		style = selectedStyle
	}
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render("❌ Exit")))

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("↑↓ navigate • Enter select"))

	return b.String()
}

func (m model) viewProjectDetailLeft() string {
	var b strings.Builder

	if m.selectedProject >= len(m.projects) {
		return "No project selected"
	}

	p := m.projects[m.selectedProject]

	b.WriteString(titleStyle.Render(fmt.Sprintf("Project #%d", p.ID)))
	b.WriteString("\n\n")

	b.WriteString(sectionStyle.Render("Name"))
	b.WriteString("\n")
	b.WriteString(itemStyle.Render(p.Name))
	b.WriteString("\n\n")

	b.WriteString(sectionStyle.Render("Repository"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(p.RepoURL))
	b.WriteString("\n\n")

	b.WriteString(sectionStyle.Render("Beads Status"))
	b.WriteString("\n")
	if p.HasBeads {
		b.WriteString(statusOK + " Beads configured\n")
		b.WriteString(fmt.Sprintf("  Ready:     %d\n", p.TasksReady))
		b.WriteString(fmt.Sprintf("  Open:      %d\n", p.TasksOpen))
		b.WriteString(fmt.Sprintf("  Completed: %d\n", p.TasksDone))
		b.WriteString(fmt.Sprintf("  Total:     %d\n", p.TasksTotal))
	} else {
		b.WriteString(statusWarn + " " + warningStyle.Render("No .beads directory"))
	}
	b.WriteString("\n")

	b.WriteString(sectionStyle.Render("Agents"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("%d agent(s) configured", p.AgentCount))
	b.WriteString("\n\n")

	b.WriteString(dimStyle.Render("[A]gents • [D]elete project • [Q]uit"))

	return b.String()
}

func (m model) viewAddProjectInputLeft() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("➕ Add Project"))
	b.WriteString("\n\n")

	b.WriteString(sectionStyle.Render(m.inputPrompt))
	b.WriteString("\n\n")

	// Text input box with border
	inputBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(0, 1).
		Width(35)

	inputContent := m.inputBuffer + "█"
	b.WriteString(inputBoxStyle.Render(inputContent))
	b.WriteString("\n\n")

	b.WriteString(dimStyle.Render(m.inputHint))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("Enter to continue • Esc to cancel"))

	return b.String()
}

func (m model) viewAddProjectCloningLeft() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("➕ Add Project"))
	b.WriteString("\n\n")

	b.WriteString(itemStyle.Render(m.newProjectName))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(m.newRepoURL))
	b.WriteString("\n\n")

	b.WriteString(statusLoading + " " + m.progressMsg)

	return b.String()
}

func (m model) viewConfirmExitLeft() string {
	var b strings.Builder

	modal := modalStyle.Render(
		warningStyle.Render("Exit Machinator?") + "\n\n" +
			dimStyle.Render("[Y]es  [N]o"),
	)

	b.WriteString("\n\n")
	b.WriteString(modal)

	return b.String()
}

func (m model) viewEditAgentCountLeft() string {
	var b strings.Builder

	if m.selectedProject >= len(m.projects) {
		return "No project selected"
	}

	p := m.projects[m.selectedProject]

	b.WriteString(titleStyle.Render("Edit Agent Count"))
	b.WriteString("\n\n")

	b.WriteString(sectionStyle.Render(p.Name))
	b.WriteString("\n\n")

	// Agent count display
	b.WriteString(dimStyle.Render("Current: "))
	b.WriteString(fmt.Sprintf("%d", p.AgentCount))
	b.WriteString("\n\n")

	b.WriteString(promptStyle.Render("New count: "))
	b.WriteString(selectedStyle.Render(fmt.Sprintf("  %d  ", m.desiredAgentCount)))
	b.WriteString("\n\n")

	diff := m.desiredAgentCount - p.AgentCount
	if diff > 0 {
		b.WriteString(successStyle.Render(fmt.Sprintf("Will add %d agent(s)", diff)))
	} else if diff < 0 {
		b.WriteString(warningStyle.Render(fmt.Sprintf("Will remove %d agent(s)", -diff)))
	} else {
		b.WriteString(dimStyle.Render("No changes"))
	}
	b.WriteString("\n\n")

	b.WriteString(dimStyle.Render("↑/+ increase • ↓/- decrease\n"))
	b.WriteString(dimStyle.Render("Enter to apply • Esc to cancel"))

	return b.String()
}

func (m model) viewStatusPane() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Status"))
	b.WriteString("\n\n")

	if len(m.statusMessages) == 0 {
		b.WriteString(dimStyle.Render("No recent activity"))
	} else {
		for _, msg := range m.statusMessages {
			b.WriteString(dimStyle.Render(msg))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// ═══════════════════════════════════════════════════════════════════════════════
// Actions
// ═══════════════════════════════════════════════════════════════════════════════

func (m *model) startAddProject() {
	m.newProjectID = m.getNextProjectID()
	m.inputStep = 0
	m.inputBuffer = ""
	m.inputPrompt = "Repository URL"
	m.inputHint = "e.g., git@github.com:user/project.git"
	m.screen = screenAddProjectInput
	m.addWarnings = []string{}
}

func (m *model) setupNewProjectPaths() {
	m.newProjectDir = filepath.Join(m.projectsDir, fmt.Sprintf("%d", m.newProjectID))
	m.newAgentDir = filepath.Join(m.newProjectDir, "agents", "1")
}

func (m model) getNextProjectID() int {
	nextID := 1
	for _, p := range m.projects {
		if p.ID >= nextID {
			nextID = p.ID + 1
		}
	}
	return nextID
}

func (m model) deriveProjectName(repoURL string) string {
	parts := strings.Split(repoURL, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		name = strings.TrimSuffix(name, ".git")
		return name
	}
	return "project"
}

// ═══════════════════════════════════════════════════════════════════════════════
// Async Commands
// ═══════════════════════════════════════════════════════════════════════════════

func (m model) installGemini() tea.Cmd {
	return func() tea.Msg {
		// Check prerequisites
		if _, err := exec.LookPath("node"); err != nil {
			return geminiInstallMsg{success: false, err: fmt.Errorf("Node.js not found")}
		}
		if _, err := exec.LookPath("npm"); err != nil {
			return geminiInstallMsg{success: false, err: fmt.Errorf("npm not found")}
		}

		os.MkdirAll(m.resourcesDir, 0755)

		// Clone
		if _, err := os.Stat(m.geminiCLIDir); os.IsNotExist(err) {
			cmd := exec.Command("git", "clone", geminiCLIRepo, "gemini-cli-mods")
			cmd.Dir = m.resourcesDir
			if out, err := cmd.CombinedOutput(); err != nil {
				return geminiInstallMsg{success: false, err: fmt.Errorf("clone failed: %s", string(out))}
			}
		}

		// npm install
		cmd := exec.Command("npm", "install")
		cmd.Dir = m.geminiCLIDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return geminiInstallMsg{success: false, err: fmt.Errorf("npm install failed: %s", string(out))}
		}

		// npm build
		cmd = exec.Command("npm", "run", "build")
		cmd.Dir = m.geminiCLIDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return geminiInstallMsg{success: false, err: fmt.Errorf("build failed: %s", string(out))}
		}

		// Create wrapper
		wrapperContent := fmt.Sprintf(`#!/bin/bash
cd "%s"
exec npm run start -- "$@"
`, m.geminiCLIDir)
		if err := os.WriteFile(m.geminiCLIPath, []byte(wrapperContent), 0755); err != nil {
			return geminiInstallMsg{success: false, err: fmt.Errorf("failed to create wrapper: %w", err)}
		}

		// Get version
		cmd = exec.Command(m.geminiCLIPath, "--version")
		out, _ := cmd.Output()
		version := strings.TrimSpace(string(out))

		return geminiInstallMsg{success: true, version: version}
	}
}

func (m model) updateGemini() tea.Cmd {
	return func() tea.Msg {
		// Pull latest
		cmd := exec.Command("git", "pull", "--ff-only")
		cmd.Dir = m.geminiCLIDir
		cmd.CombinedOutput()

		// npm install
		cmd = exec.Command("npm", "install")
		cmd.Dir = m.geminiCLIDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return geminiInstallMsg{success: false, err: fmt.Errorf("npm install failed: %s", string(out))}
		}

		// npm build
		cmd = exec.Command("npm", "run", "build")
		cmd.Dir = m.geminiCLIDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return geminiInstallMsg{success: false, err: fmt.Errorf("build failed: %s", string(out))}
		}

		// Get version
		cmd = exec.Command(m.geminiCLIPath, "--version")
		out, _ := cmd.Output()
		version := strings.TrimSpace(string(out))

		return geminiInstallMsg{success: true, version: version}
	}
}

func (m model) cloneRepo() tea.Cmd {
	return func() tea.Msg {
		os.MkdirAll(filepath.Dir(m.newAgentDir), 0755)
		os.RemoveAll(m.newAgentDir)

		cmd := exec.Command("git", "clone", m.newRepoURL, m.newAgentDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return cloneDoneMsg{success: false, err: fmt.Errorf("clone failed: %s\n%s", err, string(out))}
		}

		return cloneDoneMsg{success: true, message: "Cloned successfully"}
	}
}

func (m model) checkBeads() tea.Cmd {
	return func() tea.Msg {
		beadsDir := filepath.Join(m.newAgentDir, ".beads")
		if _, err := os.Stat(beadsDir); os.IsNotExist(err) {
			return beadsCheckMsg{hasBeads: false}
		}

		result := beadsCheckMsg{hasBeads: true}

		// Get all tasks with bd list
		cmd := exec.Command("bd", "list", "--json")
		cmd.Dir = m.newAgentDir
		out, err := cmd.Output()
		if err == nil {
			var tasks []map[string]interface{}
			if json.Unmarshal(out, &tasks) == nil {
				result.tasksTotal = len(tasks)
				for _, task := range tasks {
					if status, ok := task["status"].(string); ok {
						switch status {
						case "ready":
							result.tasksReady++
						case "open", "in_progress":
							result.tasksOpen++
						case "done", "completed":
							result.tasksDone++
						}
					}
				}
			}
		}

		return result
	}
}

func (m model) applyAgentChanges(p ProjectConfig, desiredCount int) tea.Cmd {
	return func() tea.Msg {
		projectDir := filepath.Join(m.projectsDir, fmt.Sprintf("%d", p.ID))
		currentCount := p.AgentCount

		if desiredCount > currentCount {
			// Add agents in parallel
			var wg sync.WaitGroup
			errChan := make(chan error, desiredCount-currentCount)

			for i := currentCount + 1; i <= desiredCount; i++ {
				wg.Add(1)
				go func(agentNum int) {
					defer wg.Done()
					agentDir := filepath.Join(projectDir, "agents", fmt.Sprintf("%d", agentNum))
					cmd := exec.Command("git", "clone", p.RepoURL, agentDir)
					if _, err := cmd.CombinedOutput(); err != nil {
						errChan <- err
					}
				}(i)
			}

			wg.Wait()
			close(errChan)

			if len(errChan) > 0 {
				return agentActionMsg{
					success: false,
					action:  "add",
					err:     fmt.Errorf("some clones failed"),
				}
			}
		} else if desiredCount < currentCount {
			// Remove agents (no need for parallel, removal is fast)
			for i := currentCount; i > desiredCount; i-- {
				agentDir := filepath.Join(projectDir, "agents", fmt.Sprintf("%d", i))
				os.RemoveAll(agentDir)
			}
		}

		return agentActionMsg{
			success:    true,
			action:     "updated",
			agentCount: desiredCount,
		}
	}
}

func (m model) reloadProjects() tea.Cmd {
	return func() tea.Msg {
		return projectsReloadedMsg{projects: m.loadProjects()}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Save Functions
// ═══════════════════════════════════════════════════════════════════════════════

func (m *model) saveNewProject() {
	config := ProjectConfig{
		ID:         m.newProjectID,
		Name:       m.newProjectName,
		RepoURL:    m.newRepoURL,
		AgentCount: 1,
		HasBeads:   m.hasBeads,
		TasksReady: m.beadsTasks,
		TasksOpen:  m.beadsOpen,
		TasksDone:  m.beadsDone,
		TasksTotal: m.beadsTotal,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	os.MkdirAll(m.newProjectDir, 0755)
	configPath := filepath.Join(m.newProjectDir, "project.json")
	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(configPath, data, 0644)

	// Update global settings
	settings := GlobalSettings{
		GeminiCLIPath: m.geminiCLIPath,
		GeminiCLIRepo: geminiCLIRepo,
		InitializedAt: time.Now().UTC().Format(time.RFC3339),
		NextProjectID: m.newProjectID + 1,
	}
	data, _ = json.MarshalIndent(settings, "", "  ")
	os.WriteFile(m.settingsFile, data, 0644)
}

func (m *model) updateProjectConfig(p ProjectConfig) {
	projectDir := filepath.Join(m.projectsDir, fmt.Sprintf("%d", p.ID))
	configPath := filepath.Join(projectDir, "project.json")
	data, _ := json.MarshalIndent(p, "", "  ")
	os.WriteFile(configPath, data, 0644)
}

func (m model) addAgentCmd(p ProjectConfig) tea.Cmd {
	return func() tea.Msg {
		projectDir := filepath.Join(m.projectsDir, fmt.Sprintf("%d", p.ID))
		newAgentNum := p.AgentCount + 1
		newAgentDir := filepath.Join(projectDir, "agents", fmt.Sprintf("%d", newAgentNum))

		// Clone the repo
		cmd := exec.Command("git", "clone", p.RepoURL, newAgentDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return agentActionMsg{
				success: false,
				action:  "add",
				err:     fmt.Errorf("clone failed: %s", string(out)),
			}
		}

		return agentActionMsg{
			success:    true,
			action:     "added",
			agentCount: newAgentNum,
		}
	}
}

func (m model) removeAgentCmd(p ProjectConfig) tea.Cmd {
	return func() tea.Msg {
		if p.AgentCount <= 1 {
			return agentActionMsg{
				success: false,
				action:  "remove",
				err:     fmt.Errorf("cannot remove last agent"),
			}
		}

		projectDir := filepath.Join(m.projectsDir, fmt.Sprintf("%d", p.ID))
		agentDir := filepath.Join(projectDir, "agents", fmt.Sprintf("%d", p.AgentCount))

		if err := os.RemoveAll(agentDir); err != nil {
			return agentActionMsg{
				success: false,
				action:  "remove",
				err:     fmt.Errorf("failed to remove: %w", err),
			}
		}

		return agentActionMsg{
			success:    true,
			action:     "removed",
			agentCount: p.AgentCount - 1,
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Main
// ═══════════════════════════════════════════════════════════════════════════════

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
