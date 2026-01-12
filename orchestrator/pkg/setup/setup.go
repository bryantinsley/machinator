package setup

import (
	_ "embed"
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

//go:embed nameplate.txt
var nameplateText string

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
			BorderForeground(lipgloss.Color("62")).
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

	badgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Padding(0, 1)

	activeBadgeStyle = badgeStyle.Copy().
				Background(lipgloss.Color("42")) // Green

	idleBadgeStyle = badgeStyle.Copy().
			Background(lipgloss.Color("214")) // Orange

	pausedBadgeStyle = badgeStyle.Copy().
				Background(lipgloss.Color("240")) // Gray
)

// ═══════════════════════════════════════════════════════════════════════════════
// Model Methods
// ═══════════════════════════════════════════════════════════════════════════════

// Run executes the setup TUI and returns the selected project configuration.
func Run() (*ProjectConfig, error) {
	m := initialModel()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	if m, ok := finalModel.(model); ok {
		return m.selectedProjectConfig, nil
	}
	return nil, nil
}

// Subscription to listen for progress updates
func listenForProgress(ch chan string) tea.Cmd {
	return func() tea.Msg {
		msg := <-ch
		return agentProgressMsg(msg)
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tick(), listenForProgress(m.progressChan), m.checkInit())
}

func tick() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) checkInit() tea.Cmd {
	ch := m.progressChan
	return func() tea.Msg {
		var result initCheckMsg

		// Check if machinator directory exists
		if _, err := os.Stat(m.machinatorDir); err == nil {
			result.machinatorExists = true
		}

		// Check Gemini CLI
		select {
		case ch <- "Checking Gemini CLI...":
		default:
		}

		if _, err := os.Stat(m.geminiCLIPath); err == nil {
			result.geminiStatus = geminiChecking
			// Try to run --version
			cmd := exec.Command(m.geminiCLIPath, "--version")
			if out, err := cmd.Output(); err == nil {
				result.geminiStatus = geminiInstalled
				result.geminiVersion = strings.TrimSpace(string(out))
				select {
				case ch <- "✓ Gemini CLI is up to date":
				default:
				}
			} else {
				result.geminiStatus = geminiNeedsUpdate
				select {
				case ch <- "⚠ Gemini CLI needs rebuild":
				default:
				}
			}
		} else {
			result.geminiStatus = geminiNotInstalled
			select {
			case ch <- "✗ Gemini CLI not installed":
			default:
			}
		}

		// Load projects
		select {
		case ch <- "Loading projects...":
		default:
		}

		result.projects = m.loadProjects()

		// Load accounts
		InitAccountsDir(m.machinatorDir)
		SetupDefaultAccount(m.machinatorDir)
		result.accounts, _ = GetAccounts(m.machinatorDir)

		if len(result.projects) > 0 {
			select {
			case ch <- fmt.Sprintf("✓ Loaded %d project(s)", len(result.projects)):
			default:
			}
		} else {
			select {
			case ch <- "No projects configured":
			default:
			}
		}

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
				config.DirName = entry.Name()
				// Refresh beads stats from agents/1 workspace
				agent1Dir := filepath.Join(m.projectsDir, entry.Name(), "agents", "1")
				beadsDir := filepath.Join(agent1Dir, ".beads")

				if _, err := os.Stat(beadsDir); err == nil {
					config.HasBeads = true
					// Run bd list to get fresh task counts
					cmd := exec.Command("bd", "--no-db", "list", "--json")
					cmd.Dir = agent1Dir
					if out, err := cmd.Output(); err == nil {
						var tasks []map[string]interface{}
						if json.Unmarshal(out, &tasks) == nil {
							config.TasksTotal = len(tasks)
							config.TasksReady = 0
							config.TasksOpen = 0
							config.TasksDone = 0
							for _, task := range tasks {
								if status, ok := task["status"].(string); ok {
									switch status {
									case "open":
										config.TasksReady++
									case "in_progress":
										config.TasksOpen++
									case "closed", "done", "completed":
										config.TasksDone++
										// "blocked" tasks not counted in ready/open
									}
								}
							}
						}
					}
				} else {
					config.HasBeads = false
				}

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

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tickMsg:
		return m, tick()

	case initCheckMsg:
		m.machinatorExists = msg.machinatorExists
		m.geminiStatus = msg.geminiStatus
		m.geminiVersion = msg.geminiVersion
		m.projects = msg.projects
		m.accounts = msg.accounts
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

	case accountAddedMsg:
		m.accounts = msg.accounts
		m.screen = screenManageAccounts
		m.addStatus("Account added successfully")

	case agentProgressMsg:
		m.addStatus(string(msg))
		return m, listenForProgress(m.progressChan)

	case agentActionMsg:
		if msg.success {
			// Update the project's agent count
			if m.selectedProject < len(m.projects) {
				oldCount := m.projects[m.selectedProject].AgentCount
				m.projects[m.selectedProject].AgentCount = msg.agentCount
				m.updateProjectConfig(m.projects[m.selectedProject])
				if msg.agentCount > oldCount {
					m.addStatus(fmt.Sprintf("✓ Added %d agent(s), now at %d", msg.agentCount-oldCount, msg.agentCount))
				} else {
					m.addStatus(fmt.Sprintf("✓ Removed %d agent(s), now at %d", oldCount-msg.agentCount, msg.agentCount))
				}
			}
		} else {
			m.addStatus(fmt.Sprintf("✗ Failed: %v", msg.err))
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
	case screenEditProjectName, screenEditProjectRepo:
		return m.handleEditFieldKeys(key, msg)
	case screenConfirmDeleteProject:
		return m.handleConfirmDeleteKeys(key)
	case screenConfirmExit:
		return m.handleConfirmExitKeys(key)
	case screenManageAccounts:
		return m.handleManageAccountsKeys(key)
	case screenAddAccountName:
		return m.handleAddAccountNameKeys(key, msg)
	case screenAddAccountAuthType:
		return m.handleAddAccountAuthTypeKeys(key)
	case screenAddAccountAPIKey:
		return m.handleAddAccountAPIKeyKeys(key, msg)
	case screenAddAccountGoogleInfo:
		return m.handleAddAccountGoogleInfoKeys(key)
	}

	return m, nil
}

func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Only handle left clicks
	if msg.Action != tea.MouseActionRelease || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	// Handle clicks based on current screen
	switch m.screen {
	case screenProjectDetail:
		// Delete button around x=4-12, Back around x=14-22
		// Buttons are near bottom of the content area
		if msg.Y >= 18 && msg.Y <= 22 {
			if msg.X >= 4 && msg.X <= 14 {
				// Delete clicked
				if m.selectedProject < len(m.projects) {
					p := m.projects[m.selectedProject]
					projectDir := m.getProjectRootDir(p)
					os.RemoveAll(projectDir)
					m.addStatus(fmt.Sprintf("Removed project: %s", p.Name))
					m.screen = screenMain
					m.detailCursor = 0
					return m, m.reloadProjects()
				}
			} else if msg.X >= 16 && msg.X <= 26 {
				// Back clicked
				m.screen = screenMain
				m.detailCursor = 0
			}
		}

	case screenEditAgentCount:
		// Button positions are approximate - Apply button starts around x=4, Cancel around x=14
		// Buttons are on line ~10 within the left pane
		if msg.Y >= 10 && msg.Y <= 12 {
			if msg.X >= 4 && msg.X <= 12 {
				// Apply clicked
				if m.selectedProject < len(m.projects) {
					p := m.projects[m.selectedProject]
					if m.desiredAgentCount != p.AgentCount {
						m.screen = screenApplyingAgents
						diff := m.desiredAgentCount - p.AgentCount
						if diff > 0 {
							m.addStatus(fmt.Sprintf("Adding %d agent(s)...", diff))
						} else {
							m.addStatus(fmt.Sprintf("Removing %d agent(s)...", -diff))
						}
						return m, tea.Batch(
							listenForProgress(m.progressChan),
							m.applyAgentChanges(p, m.desiredAgentCount),
						)
					}
					m.screen = screenProjectDetail
				}
			} else if msg.X >= 14 && msg.X <= 24 {
				// Cancel clicked
				m.screen = screenProjectDetail
			}
		}

	case screenEditProjectName, screenEditProjectRepo:
		// Save/Cancel buttons around line 8-10
		if msg.Y >= 8 && msg.Y <= 12 {
			if msg.X >= 4 && msg.X <= 12 {
				// Save clicked - trigger enter
				return m.handleEditFieldKeys("", tea.KeyMsg{Type: tea.KeyEnter})
			} else if msg.X >= 14 && msg.X <= 24 {
				// Cancel clicked
				m.screen = screenProjectDetail
			}
		}

	case screenMain:
		// Add/Exit buttons are near the bottom
		// Approximate positions based on content
		projectCount := len(m.projects)
		buttonLine := projectCount + 8 // After title, gemini, projects section

		if msg.Y >= buttonLine && msg.Y <= buttonLine+2 {
			if msg.X >= 4 && msg.X <= 18 {
				// Add Project clicked
				m.startAddProject()
			} else if msg.X >= 20 && msg.X <= 30 {
				// Exit clicked
				m.screen = screenConfirmExit
			}
		}

	case screenConfirmExit:
		// Dialog centered in screen - buttons are in center
		centerY := m.height / 2
		centerX := m.width / 2
		// Yes button approx at center-8 to center-2, No button at center+2 to center+8
		if msg.Y >= centerY && msg.Y <= centerY+2 {
			if msg.X >= centerX-10 && msg.X <= centerX-2 {
				// Yes clicked
				return m, tea.Quit
			} else if msg.X >= centerX+2 && msg.X <= centerX+10 {
				// No clicked
				m.screen = screenMain
			}
		}

	case screenConfirmDeleteProject:
		// Dialog centered in screen
		centerY := m.height / 2
		centerX := m.width / 2
		if msg.Y >= centerY+2 && msg.Y <= centerY+4 {
			if msg.X >= centerX-10 && msg.X <= centerX-2 {
				// Yes clicked - delete
				if m.selectedProject < len(m.projects) {
					p := m.projects[m.selectedProject]
					projectDir := m.getProjectRootDir(p)
					os.RemoveAll(projectDir)
					m.addStatus(fmt.Sprintf("Deleted project: %s", p.Name))
					m.screen = screenMain
					m.detailCursor = 0
					return m, m.reloadProjects()
				}
			} else if msg.X >= centerX+2 && msg.X <= centerX+10 {
				// No clicked
				m.screen = screenProjectDetail
			}
		}
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
	// Menu items: Gemini CLI (0), projects (1..n), Add Project (n+1), Manage Accounts (n+2), Exit (n+3)
	maxItems := len(m.projects) + 4 // gemini + projects + add + accounts + exit

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
		} else if m.cursor == len(m.projects)+2 {
			// Manage Accounts
			m.screen = screenManageAccounts
			m.accountCursor = 0
		} else {
			// Exit
			m.screen = screenConfirmExit
		}
	case "a":
		m.startAddProject()
	case "q", "x":
		m.screen = screenConfirmExit
	}
	return m, nil
}

func (m model) handleProjectDetailKeys(key string) (tea.Model, tea.Cmd) {
	// Menu items: 0=Name, 1=Repository, 2=Agents, 3=Open, 4=Delete, 5=Back
	maxItems := 6

	switch key {
	case "up", "k":
		if m.detailCursor > 0 {
			m.detailCursor--
		}
	case "down", "j":
		if m.detailCursor < maxItems-1 {
			m.detailCursor++
		}
	case "c":
		if m.selectedProject < len(m.projects) {
			p := m.projects[m.selectedProject]
			return m, m.copyToClipboard(m.getProjectRootDir(p))
		}
	case "enter":
		if m.selectedProject >= len(m.projects) {
			return m, nil
		}
		p := m.projects[m.selectedProject]

		switch m.detailCursor {
		case 0: // Name
			m.editBuffer = p.Name
			m.screen = screenEditProjectName
		case 1: // Repository
			m.editBuffer = p.RepoURL
			m.screen = screenEditProjectRepo
		case 2: // Agents
			m.desiredAgentCount = p.AgentCount
			m.screen = screenEditAgentCount
		case 3: // Open
			m.selectedProjectConfig = &m.projects[m.selectedProject]
			return m, tea.Quit
		case 4: // Delete - show confirmation
			m.dialogCursor = 1 // Default to "No" for safety
			m.screen = screenConfirmDeleteProject
		case 5: // Back
			m.screen = screenMain
			m.detailCursor = 0
		}
	case "q", "esc":
		m.screen = screenMain
		m.detailCursor = 0
	}
	return m, nil
}

func (m model) handleEditAgentCountKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "q":
		m.screen = screenProjectDetail
		m.agentCursor = 0
	case "tab":
		m.agentCursor = (m.agentCursor + 1) % 3 // 0=number, 1=apply, 2=cancel
	case "up", "+", "=", "k":
		if m.agentCursor == 0 {
			m.desiredAgentCount++
			if m.desiredAgentCount > 10 {
				m.desiredAgentCount = 10
			}
		}
	case "down", "-", "_", "j":
		if m.agentCursor == 0 && m.desiredAgentCount > 1 {
			m.desiredAgentCount--
		}
	case "enter":
		if m.agentCursor == 2 {
			// Cancel button
			m.screen = screenProjectDetail
			m.agentCursor = 0
			return m, nil
		}
		if m.agentCursor == 1 || m.agentCursor == 0 {
			// Apply button or enter on number
			if m.selectedProject < len(m.projects) {
				p := m.projects[m.selectedProject]
				if m.desiredAgentCount != p.AgentCount {
					m.screen = screenApplyingAgents
					m.agentCursor = 0
					diff := m.desiredAgentCount - p.AgentCount
					if diff > 0 {
						m.addStatus(fmt.Sprintf("Adding %d agent(s)...", diff))
					} else {
						m.addStatus(fmt.Sprintf("Removing %d agent(s)...", -diff))
					}
					return m, tea.Batch(
						listenForProgress(m.progressChan),
						m.applyAgentChanges(p, m.desiredAgentCount),
					)
				}
				m.screen = screenProjectDetail
				m.agentCursor = 0
			}
		}
	}
	return m, nil
}

func (m model) handleEditFieldKeys(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = screenProjectDetail
		m.editCursor = 0
		return m, nil
	case tea.KeyTab:
		m.editCursor = (m.editCursor + 1) % 3 // 0=input, 1=save, 2=cancel
	case tea.KeyEnter:
		if m.editCursor == 2 {
			// Cancel button
			m.screen = screenProjectDetail
			m.editCursor = 0
			return m, nil
		}
		if m.editCursor == 1 || m.editCursor == 0 {
			// Save button or enter in input
			if m.editBuffer == "" || m.selectedProject >= len(m.projects) {
				return m, nil
			}
			p := m.projects[m.selectedProject]

			if m.screen == screenEditProjectName {
				m.projects[m.selectedProject].Name = m.editBuffer
				m.updateProjectConfig(m.projects[m.selectedProject])
				m.addStatus(fmt.Sprintf("Renamed to: %s", m.editBuffer))
				m.screen = screenProjectDetail
				m.editCursor = 0
				return m, m.reloadProjects()
			} else if m.screen == screenEditProjectRepo {
				if m.editBuffer != p.RepoURL {
					m.projects[m.selectedProject].RepoURL = m.editBuffer
					m.updateProjectConfig(m.projects[m.selectedProject])
					m.addStatus(fmt.Sprintf("Repository changed to: %s", m.editBuffer))
					m.addStatus("Recloning all agents...")
					m.screen = screenApplyingAgents
					m.editCursor = 0
					return m, m.recloneAllAgents(m.projects[m.selectedProject])
				}
				m.screen = screenProjectDetail
				m.editCursor = 0
			}
		}
		return m, nil
	case tea.KeyBackspace:
		if m.editCursor == 0 && len(m.editBuffer) > 0 {
			m.editBuffer = m.editBuffer[:len(m.editBuffer)-1]
		}
	case tea.KeyRunes:
		if m.editCursor == 0 {
			m.editBuffer += string(msg.Runes)
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
	case "tab", "left", "right":
		m.dialogCursor = 1 - m.dialogCursor // Toggle 0<->1
	case "enter":
		if m.dialogCursor == 0 {
			return m, tea.Quit
		} else {
			m.screen = screenMain
			m.dialogCursor = 0
		}
	case "y":
		return m, tea.Quit
	case "n", "esc", "q":
		m.screen = screenMain
		m.dialogCursor = 0
	}
	return m, nil
}

func (m model) handleConfirmDeleteKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "tab", "left", "right":
		m.dialogCursor = 1 - m.dialogCursor // Toggle 0<->1
	case "enter":
		if m.dialogCursor == 0 {
			// Yes - delete
			if m.selectedProject < len(m.projects) {
				p := m.projects[m.selectedProject]
				projectDir := m.getProjectRootDir(p)
				os.RemoveAll(projectDir)
				m.addStatus(fmt.Sprintf("Deleted project: %s", p.Name))
				m.screen = screenMain
				m.detailCursor = 0
				m.dialogCursor = 0
				return m, m.reloadProjects()
			}
		} else {
			// No - cancel
			m.screen = screenProjectDetail
			m.dialogCursor = 0
		}
	case "y":
		if m.selectedProject < len(m.projects) {
			p := m.projects[m.selectedProject]
			projectDir := m.getProjectRootDir(p)
			os.RemoveAll(projectDir)
			m.addStatus(fmt.Sprintf("Deleted project: %s", p.Name))
			m.screen = screenMain
			m.detailCursor = 0
			m.dialogCursor = 0
			return m, m.reloadProjects()
		}
		m.screen = screenMain
	case "n", "esc", "q":
		m.screen = screenProjectDetail
		m.dialogCursor = 0
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

	// Layout: Nameplate at top, then 3 panels
	// Left column: Project Settings (top) + Doctinator (bottom)
	// Right: Status pane, Exit button at very bottom right
	topPadding := 1
	nameplateHeight := 5
	exitBtnHeight := 1

	leftWidth := m.width/2 - 2
	rightWidth := m.width - leftWidth - 4

	if leftWidth < 30 {
		leftWidth = 30
		rightWidth = m.width - leftWidth - 4
	}
	if rightWidth < 30 {
		rightWidth = 30
		// If right is too small, we might overflow width if we force it, but let's try to be safe
		// Ideally we need min width support or scroll
	}

	// Total height available for both left and right sides (must match)
	totalHeight := m.height - topPadding - nameplateHeight - exitBtnHeight - 2
	if totalHeight < 16 {
		totalHeight = 16
	}

	// Left side splits into top (60%) and bottom (40%)
	// Each bordered pane adds 2 to rendered height (top+bottom border)
	// When we stack two panes, their CONTENT heights + 4 borders must equal right pane content height + 2 borders
	// So: topContent + bottomContent + 4 = rightContent + 2
	// Therefore: topContent + bottomContent = rightContent - 2

	// Right pane gets full height
	rightHeight := totalHeight

	// Left panes split the height minus the extra border overhead
	leftTotalContent := totalHeight - 2 // Account for the extra 2 borders when stacking
	topHeight := leftTotalContent * 6 / 10
	bottomHeight := leftTotalContent - topHeight

	if topHeight < 6 {
		topHeight = 6
	}
	if bottomHeight < 4 {
		bottomHeight = 4
	}

	topPane := leftPaneStyle.Width(leftWidth).Height(topHeight)
	bottomPane := leftPaneStyle.Width(leftWidth).Height(bottomHeight)
	rightPane := rightPaneStyle.Width(rightWidth).Height(rightHeight)

	var topContent, bottomContent, rightContent string

	// For screens that use modal overlays, show the main content behind
	switch m.screen {
	case screenInit:
		topContent = m.viewInitLeft()
		bottomContent = m.viewDoctinator()
		rightContent = m.viewStatusPane()
	case screenAddProjectInput:
		topContent = m.viewAddProjectInputLeft()
		bottomContent = m.viewDoctinator()
		rightContent = m.viewStatusPane()
	case screenAddProjectCloning, screenApplyingAgents:
		topContent = m.viewAddProjectCloningLeft()
		bottomContent = m.viewDoctinator()
		rightContent = m.viewStatusPane()
	case screenManageAccounts:
		topContent = m.viewManageAccountsLeft()
		bottomContent = m.viewDoctinator()
		rightContent = m.viewStatusPane()
	case screenAddAccountName:
		topContent = m.viewAddAccountNameLeft()
		bottomContent = m.viewDoctinator()
		rightContent = m.viewStatusPane()
	case screenAddAccountAuthType:
		topContent = m.viewAddAccountAuthTypeLeft()
		bottomContent = m.viewDoctinator()
		rightContent = m.viewStatusPane()
	case screenAddAccountAPIKey:
		topContent = m.viewAddAccountAPIKeyLeft()
		bottomContent = m.viewDoctinator()
		rightContent = m.viewStatusPane()
	case screenAddAccountGoogleInfo:
		topContent = m.viewAddAccountGoogleInfoLeft()
		bottomContent = m.viewDoctinator()
		rightContent = m.viewStatusPane()
	default:
		// Main screen or screens with modal overlays
		topContent = m.viewProjectSettings()
		bottomContent = m.viewDoctinator()
		rightContent = m.viewStatusPane()
	}

	topRendered := topPane.Render(topContent)
	bottomRendered := bottomPane.Render(bottomContent)
	rightRendered := rightPane.Render(rightContent)

	leftColumn := lipgloss.JoinVertical(lipgloss.Left, topRendered, bottomRendered)

	// Exit button at bottom right
	exitBg := lipgloss.Color("240")
	if m.screen == screenMain && m.cursor == len(m.projects)+3 {
		exitBg = lipgloss.Color("196")
	}
	exitBtn := lipgloss.NewStyle().
		Background(exitBg).
		Foreground(lipgloss.Color("255")).
		Padding(0, 2).
		Render("Exit")

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, rightRendered)
	panelsWithExit := lipgloss.JoinVertical(lipgloss.Right, panels, exitBtn)

	// Rainbow nameplate - centered with top padding
	nameplate := rainbowNameplate()
	centeredNameplate := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, nameplate)

	// Add empty line at top to ensure nothing is cut off
	baseView := lipgloss.JoinVertical(lipgloss.Left, "", centeredNameplate, panelsWithExit)

	// Modal overlays for various screens
	var modal string
	switch m.screen {
	case screenConfirmExit:
		modal = m.renderExitDialog()
	case screenConfirmDeleteProject:
		modal = m.renderDeleteDialog()
	case screenProjectDetail:
		modal = m.renderProjectDetailModal()
	case screenEditProjectName, screenEditProjectRepo:
		modal = m.renderEditFieldModal()
	case screenEditAgentCount:
		modal = m.renderEditAgentCountModal()
	}

	if modal != "" {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("0")))
	}

	return baseView
}

func (m model) viewProjectSettings() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("📁 Project Settings"))
	b.WriteString("\n\n")

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
			b.WriteString(fmt.Sprintf("%s#%d) %s\n", cursor, p.ID, style.Render(p.Name)))
		}
	}

	b.WriteString("\n")

	// Add project button at bottom
	addIdx := len(m.projects) + 1
	addBg := lipgloss.Color("240")
	if m.cursor == addIdx {
		addBg = lipgloss.Color("34") // Green when selected
	}
	addBtn := lipgloss.NewStyle().
		Background(addBg).
		Foreground(lipgloss.Color("255")).
		Padding(0, 2).
		Render("Add Project")

	b.WriteString(addBtn)

	// Manage Accounts button
	accountsIdx := len(m.projects) + 2
	accountsBg := lipgloss.Color("240")
	if m.cursor == accountsIdx {
		accountsBg = lipgloss.Color("34") // Green when selected
	}

	accountsBtn := lipgloss.NewStyle().
		Background(accountsBg).
		Foreground(lipgloss.Color("255")).
		Padding(0, 2).
		Render("Manage Accounts")

	b.WriteString("\n\n")
	b.WriteString(accountsBtn)

	return b.String()
}

func (m model) viewDoctinator() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("👩‍⚕️ Doctinator"))
	b.WriteString("\n\n")

	// Gemini CLI status
	switch m.geminiStatus {
	case geminiInstalled:
		b.WriteString(successStyle.Render("  ✓ Gemini CLI\n"))
	case geminiNeedsUpdate:
		b.WriteString(warningStyle.Render("  ⚠ Gemini CLI (update)\n"))
	case geminiNotInstalled:
		b.WriteString(warningStyle.Render("  ✗ Gemini CLI\n"))
	case geminiInstalling:
		b.WriteString(dimStyle.Render("  ↻ Installing...\n"))
	default:
		b.WriteString(dimStyle.Render("  Checking...\n"))
	}

	// Beads status
	if m.projectsLoaded && len(m.projects) > 0 {
		projectsWithBeads := 0
		openTasks := 0
		for _, p := range m.projects {
			if p.HasBeads {
				projectsWithBeads++
				openTasks += p.TasksOpen
			}
		}

		if projectsWithBeads == len(m.projects) {
			b.WriteString(successStyle.Render(fmt.Sprintf("  ✓ Beads %d/%d\n", projectsWithBeads, len(m.projects))))
		} else if projectsWithBeads > 0 {
			b.WriteString(warningStyle.Render(fmt.Sprintf("  ⚠ Beads %d/%d\n", projectsWithBeads, len(m.projects))))
		} else {
			b.WriteString(dimStyle.Render("  ○ No beads\n"))
		}

		if openTasks > 0 {
			b.WriteString(dimStyle.Render(fmt.Sprintf("    %d active\n", openTasks)))
		}
	}

	return b.String()
}

func rainbowNameplate() string {
	// Read lines from embedded file
	lines := strings.Split(strings.TrimSuffix(nameplateText, "\n"), "\n")

	// Find the longest line for padding
	maxLen := 0
	for _, line := range lines {
		runeLen := len([]rune(line))
		if runeLen > maxLen {
			maxLen = runeLen
		}
	}

	// Rainbow colors - vibrant spectrum
	colors := []string{"196", "202", "208", "214", "220", "226", "190", "154", "118", "82", "46", "47", "48", "49", "50", "51", "45", "39", "33", "27", "21", "57", "93", "129", "165", "201", "200", "199", "198", "197"}

	var result strings.Builder
	for lineIdx, line := range lines {
		runes := []rune(line)
		// Pad to max length
		for len(runes) < maxLen {
			runes = append(runes, ' ')
		}
		for i, r := range runes {
			if r == ' ' {
				result.WriteRune(r)
			} else {
				colorIdx := (i + lineIdx*3) % len(colors)
				style := lipgloss.NewStyle().Foreground(lipgloss.Color(colors[colorIdx]))
				result.WriteString(style.Render(string(r)))
			}
		}
		if lineIdx < len(lines)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

func (m model) viewInitLeft() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("🔧 Machinator Setup"))
	b.WriteString("\n\n")

	if !m.machinatorExists {
		b.WriteString(warningStyle.Render(fmt.Sprintf("No %s directory found.\n\n", m.machinatorDir)))
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
			b.WriteString(fmt.Sprintf("%s#%d) %s\n", cursor, p.ID, style.Render(p.Name)))
		}
	}

	b.WriteString("\n")

	// Add project button only - Exit is now at bottom right of screen
	addIdx := len(m.projects) + 1

	addBg := lipgloss.Color("240") // Gray
	if m.cursor == addIdx {
		addBg = lipgloss.Color("34") // Green when selected
	}

	addBtn := lipgloss.NewStyle().
		Background(addBg).
		Foreground(lipgloss.Color("255")).
		Padding(0, 2).
		Render("Add Project")

	b.WriteString(addBtn)

	return b.String()
}

func (m model) viewEditFieldLeft() string {
	var b strings.Builder

	title := "Edit Name"
	warning := ""

	if m.screen == screenEditProjectRepo {
		title = "Edit Repository"
		warning = "⚠️ Changing repository will reclone all agents!"
	}

	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	if warning != "" {
		b.WriteString(warningStyle.Render(warning))
		b.WriteString("\n\n")
	}

	// Simple text input with cursor
	b.WriteString(m.editBuffer)
	b.WriteString(selectedStyle.Render("█"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("────────────────────────────────────────"))
	b.WriteString("\n\n")

	// Buttons
	saveBtn := lipgloss.NewStyle().
		Background(lipgloss.Color("34")).
		Foreground(lipgloss.Color("255")).
		Padding(0, 2).
		Render("Save")

	cancelBtn := lipgloss.NewStyle().
		Background(lipgloss.Color("240")).
		Foreground(lipgloss.Color("255")).
		Padding(0, 2).
		Render("Cancel")

	b.WriteString(saveBtn)
	b.WriteString("  ")
	b.WriteString(cancelBtn)

	return b.String()
}

func (m model) viewProjectDetailLeft() string {
	var b strings.Builder

	if m.selectedProject >= len(m.projects) {
		return "No project selected"
	}

	p := m.projects[m.selectedProject]

	b.WriteString(titleStyle.Render(p.Name))
	b.WriteString("\n\n")

	// Menu items with consistent layout
	// Format: [cursor] Label: Value (or on next line for long values)

	// Name
	if m.detailCursor == 0 {
		b.WriteString("▸ " + selectedStyle.Render("Name:") + " " + p.Name + "\n")
	} else {
		b.WriteString("  " + dimStyle.Render("Name:") + " " + p.Name + "\n")
	}

	// Repository (value on next line due to length)
	if m.detailCursor == 1 {
		b.WriteString("▸ " + selectedStyle.Render("Repository:") + "\n")
	} else {
		b.WriteString("  " + dimStyle.Render("Repository:") + "\n")
	}
	b.WriteString("    " + dimStyle.Render(p.RepoURL) + "\n")

	// Agents
	if m.detailCursor == 2 {
		b.WriteString("▸ " + selectedStyle.Render("Agents:") + " " + fmt.Sprintf("%d", p.AgentCount) + "\n")
	} else {
		b.WriteString("  " + dimStyle.Render("Agents:") + " " + fmt.Sprintf("%d", p.AgentCount) + "\n")
	}

	b.WriteString("\n")

	// Beads grid (not selectable)
	b.WriteString("  " + dimStyle.Render("Beads:") + "\n")
	if p.HasBeads {
		b.WriteString(fmt.Sprintf("    Ready: %-3d  Active: %-3d\n", p.TasksReady, p.TasksOpen))
		b.WriteString(fmt.Sprintf("    Done:  %-3d  Total:  %-3d\n", p.TasksDone, p.TasksTotal))
	} else {
		b.WriteString("    " + dimStyle.Render("Not configured") + "\n")
	}

	b.WriteString("\n")

	// Buttons: gray when not selected, colored when selected
	deleteBg := lipgloss.Color("240") // Gray
	backBg := lipgloss.Color("240")   // Gray

	if m.detailCursor == 3 {
		deleteBg = lipgloss.Color("196") // Red when selected
	}
	if m.detailCursor == 4 {
		backBg = lipgloss.Color("39") // Blue when selected
	}

	deleteBtn := lipgloss.NewStyle().
		Background(deleteBg).
		Foreground(lipgloss.Color("255")).
		Padding(0, 2).
		Render("Delete")

	backBtn := lipgloss.NewStyle().
		Background(backBg).
		Foreground(lipgloss.Color("255")).
		Padding(0, 2).
		Render("Back")

	b.WriteString(deleteBtn + "  " + backBtn)

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
	return "" // Unused - using overlay now
}

func (m model) viewConfirmDeleteLeft() string {
	return "" // Unused - using overlay now
}

func (m model) renderExitDialog() string {
	// Buttons: selected is colored, unselected is gray
	yesBg := lipgloss.Color("240")
	noBg := lipgloss.Color("240")
	if m.dialogCursor == 0 {
		yesBg = lipgloss.Color("196") // Red when selected
	} else {
		noBg = lipgloss.Color("39") // Blue when selected
	}

	yesBtn := lipgloss.NewStyle().
		Background(yesBg).
		Foreground(lipgloss.Color("255")).
		Padding(0, 3).
		Render("Yes")

	noBtn := lipgloss.NewStyle().
		Background(noBg).
		Foreground(lipgloss.Color("255")).
		Padding(0, 3).
		Render("No")

	content := lipgloss.JoinVertical(lipgloss.Center,
		warningStyle.Render("Exit Machinator?"),
		"",
		yesBtn+"  "+noBtn,
	)

	return modalStyle.Render(content)
}

func (m model) renderDeleteDialog() string {
	projectName := ""
	if m.selectedProject < len(m.projects) {
		projectName = m.projects[m.selectedProject].Name
	}

	// Buttons: selected is colored, unselected is gray
	yesBg := lipgloss.Color("240")
	noBg := lipgloss.Color("240")
	if m.dialogCursor == 0 {
		yesBg = lipgloss.Color("196") // Red when selected
	} else {
		noBg = lipgloss.Color("39") // Blue when selected
	}

	yesBtn := lipgloss.NewStyle().
		Background(yesBg).
		Foreground(lipgloss.Color("255")).
		Padding(0, 3).
		Render("Yes")

	noBtn := lipgloss.NewStyle().
		Background(noBg).
		Foreground(lipgloss.Color("255")).
		Padding(0, 3).
		Render("No")

	content := lipgloss.JoinVertical(lipgloss.Center,
		lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true).Render("Delete Project?"),
		"",
		itemStyle.Render(projectName),
		"",
		dimStyle.Render("This will permanently delete"),
		dimStyle.Render("all agents and project data."),
		"",
		yesBtn+"  "+noBtn,
	)

	return modalStyle.Render(content)
}

func (m model) renderProjectDetailModal() string {
	if m.selectedProject >= len(m.projects) {
		return modalStyle.Render("No project selected")
	}

	p := m.projects[m.selectedProject]

	// Menu items with cursor
	items := []struct {
		label    string
		value    string
		selected bool
	}{
		{"Name", p.Name, m.detailCursor == 0},
		{"Repository", p.RepoURL, m.detailCursor == 1},
		{"Agents", fmt.Sprintf("%d", p.AgentCount), m.detailCursor == 2},
	}

	var lines []string
	header := lipgloss.JoinHorizontal(lipgloss.Center, titleStyle.Render(p.Name), " ", m.getProjectStatusBadge(p))
	lines = append(lines, header)
	lines = append(lines, "")

	for _, item := range items {
		cursor := "  "
		style := dimStyle
		if item.selected {
			cursor = "▸ "
			style = selectedStyle
		}
		lines = append(lines, cursor+style.Render(item.label+":")+dimStyle.Render(" "+item.value))
	}

	// Root directory (not selectable but displayed)
	lines = append(lines, "  "+dimStyle.Render("Root Dir:")+dimStyle.Render(" "+m.getProjectRootDir(p)))

	lines = append(lines, "")

	// Beads info
	if p.HasBeads {
		lines = append(lines, dimStyle.Render(fmt.Sprintf("  Beads: %d ready, %d active, %d done", p.TasksReady, p.TasksOpen, p.TasksDone)))
	} else {
		lines = append(lines, dimStyle.Render("  Beads: not configured"))
	}

	lines = append(lines, "")

	// Buttons
	openBg := lipgloss.Color("240")
	deleteBg := lipgloss.Color("240")
	backBg := lipgloss.Color("240")
	if m.detailCursor == 3 {
		openBg = lipgloss.Color("42") // Green for Open
	}
	if m.detailCursor == 4 {
		deleteBg = lipgloss.Color("196")
	}
	if m.detailCursor == 5 {
		backBg = lipgloss.Color("39")
	}

	openBtn := lipgloss.NewStyle().Background(openBg).Foreground(lipgloss.Color("255")).Padding(0, 2).Render("Open")
	deleteBtn := lipgloss.NewStyle().Background(deleteBg).Foreground(lipgloss.Color("255")).Padding(0, 2).Render("Delete")
	backBtn := lipgloss.NewStyle().Background(backBg).Foreground(lipgloss.Color("255")).Padding(0, 2).Render("Back")

	lines = append(lines, openBtn+"  "+deleteBtn+"  "+backBtn)
	lines = append(lines, "")
	lines = append(lines, dimStyle.Render(" (c) copy path"))

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return modalStyle.Width(60).Render(content)
}

func (m model) renderEditFieldModal() string {
	title := "Edit Name"
	if m.screen == screenEditProjectRepo {
		title = "Edit Repository"
	}

	var lines []string
	lines = append(lines, titleStyle.Render(title))
	lines = append(lines, "")

	// Text input with cursor
	inputStyle := dimStyle
	if m.editCursor == 0 {
		inputStyle = selectedStyle
	}
	inputLine := inputStyle.Render(m.editBuffer + "█")
	lines = append(lines, inputLine)
	lines = append(lines, dimStyle.Render("────────────────────────────────────────"))
	lines = append(lines, "")

	// Buttons - highlight based on cursor
	saveBg := lipgloss.Color("240")
	cancelBg := lipgloss.Color("240")
	if m.editCursor == 1 {
		saveBg = lipgloss.Color("34") // Green when selected
	}
	if m.editCursor == 2 {
		cancelBg = lipgloss.Color("39") // Blue when selected
	}

	saveBtn := lipgloss.NewStyle().Background(saveBg).Foreground(lipgloss.Color("255")).Padding(0, 2).Render("Save")
	cancelBtn := lipgloss.NewStyle().Background(cancelBg).Foreground(lipgloss.Color("255")).Padding(0, 2).Render("Cancel")

	lines = append(lines, saveBtn+"  "+cancelBtn)

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return modalStyle.Width(50).Render(content)
}

func (m model) renderEditAgentCountModal() string {
	if m.selectedProject >= len(m.projects) {
		return modalStyle.Render("No project selected")
	}

	p := m.projects[m.selectedProject]

	var lines []string
	lines = append(lines, titleStyle.Render("Agents"))
	lines = append(lines, "")
	lines = append(lines, dimStyle.Render(p.Name))
	lines = append(lines, "")

	// Number display - highlight if focused
	numStyle := dimStyle
	if m.agentCursor == 0 {
		numStyle = selectedStyle
	}
	lines = append(lines, fmt.Sprintf("  %s", numStyle.Render(fmt.Sprintf(" %d ", m.desiredAgentCount))))
	lines = append(lines, "")

	if m.desiredAgentCount != p.AgentCount {
		diff := m.desiredAgentCount - p.AgentCount
		if diff > 0 {
			lines = append(lines, successStyle.Render(fmt.Sprintf("  +%d agent(s)", diff)))
		} else {
			lines = append(lines, warningStyle.Render(fmt.Sprintf("  %d agent(s)", diff)))
		}
		lines = append(lines, "")
	}

	// Buttons - highlight based on cursor
	applyBg := lipgloss.Color("240")
	cancelBg := lipgloss.Color("240")
	if m.agentCursor == 1 {
		applyBg = lipgloss.Color("34") // Green when selected
	}
	if m.agentCursor == 2 {
		cancelBg = lipgloss.Color("39") // Blue when selected
	}

	applyBtn := lipgloss.NewStyle().Background(applyBg).Foreground(lipgloss.Color("255")).Padding(0, 2).Render("Apply")
	cancelBtn := lipgloss.NewStyle().Background(cancelBg).Foreground(lipgloss.Color("255")).Padding(0, 2).Render("Cancel")

	lines = append(lines, applyBtn+"  "+cancelBtn)

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return modalStyle.Width(40).Render(content)
}

func (m model) viewEditAgentCountLeft() string {
	var b strings.Builder

	if m.selectedProject >= len(m.projects) {
		return "No project selected"
	}

	p := m.projects[m.selectedProject]

	b.WriteString(titleStyle.Render("Agents"))
	b.WriteString("\n\n")

	b.WriteString(sectionStyle.Render(p.Name))
	b.WriteString("\n\n")

	// Simple display with inline diff
	b.WriteString(itemStyle.Render("Number of Agents: "))
	b.WriteString(selectedStyle.Render(fmt.Sprintf("%d", m.desiredAgentCount)))

	diff := m.desiredAgentCount - p.AgentCount
	if diff > 0 {
		b.WriteString(successStyle.Render(fmt.Sprintf(" (+%d)", diff)))
	} else if diff < 0 {
		b.WriteString(warningStyle.Render(fmt.Sprintf(" (%d)", diff)))
	}
	b.WriteString("\n\n")

	// Styled buttons
	applyBtn := lipgloss.NewStyle().
		Background(lipgloss.Color("34")).
		Foreground(lipgloss.Color("255")).
		Padding(0, 2).
		Render("Apply")

	cancelBtn := lipgloss.NewStyle().
		Background(lipgloss.Color("240")).
		Foreground(lipgloss.Color("255")).
		Padding(0, 2).
		Render("Cancel")

	b.WriteString(applyBtn)
	b.WriteString("  ")
	b.WriteString(cancelBtn)

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

func (m model) getProjectRootDir(p ProjectConfig) string {
	dirName := p.DirName
	if dirName == "" {
		dirName = fmt.Sprintf("%d", p.ID)
	}
	return filepath.Join(m.projectsDir, dirName)
}

func (m model) getProjectStatusBadge(p ProjectConfig) string {
	if !p.HasBeads {
		return pausedBadgeStyle.Render("NO BEADS")
	}
	if p.TasksOpen > 0 {
		return activeBadgeStyle.Render("ACTIVE")
	}
	if p.TasksReady > 0 {
		return idleBadgeStyle.Render("IDLE")
	}
	if p.TasksTotal > 0 {
		return pausedBadgeStyle.Render("PAUSED")
	}
	return pausedBadgeStyle.Render("NEW")
}

func (m model) copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			return agentProgressMsg(fmt.Sprintf("✗ Failed to copy: %v", err))
		}
		return agentProgressMsg("✓ Copied path to clipboard")
	}
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

func (m model) handleManageAccountsKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "q", "backspace":
		m.screen = screenMain
		return m, nil
	case "up", "k":
		if m.accountCursor > 0 {
			m.accountCursor--
		}
	case "down", "j":
		if m.accountCursor < len(m.accounts)-1 {
			m.accountCursor++
		}
	case "a":
		m.screen = screenAddAccountName
		m.inputBuffer = ""
		m.inputPrompt = "Account Name"
		m.inputHint = "e.g., secondary-google-account"
	}
	return m, nil
}

func (m model) handleAddAccountNameKeys(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = screenManageAccounts
		return m, nil
	case tea.KeyEnter:
		if m.inputBuffer == "" {
			return m, nil
		}
		m.newAccountName = m.inputBuffer

		// Create directories immediately
		accountDir := filepath.Join(m.machinatorDir, "accounts", m.newAccountName)
		if err := os.MkdirAll(accountDir, 0755); err != nil {
			return m, func() tea.Msg { return agentProgressMsg(fmt.Sprintf("✗ Failed to create directory: %v", err)) }
		}
		geminiDir := filepath.Join(accountDir, ".gemini")
		if err := os.MkdirAll(geminiDir, 0755); err != nil {
			return m, func() tea.Msg {
				return agentProgressMsg(fmt.Sprintf("✗ Failed to create .gemini directory: %v", err))
			}
		}

		m.screen = screenAddAccountAuthType
		m.dialogCursor = 0
		return m, nil
	case tea.KeyBackspace:
		if len(m.inputBuffer) > 0 {
			m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
		}
	case tea.KeyRunes:
		m.inputBuffer += string(msg.Runes)
	}
	return m, nil
}

func (m model) handleAddAccountAuthTypeKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "q":
		m.screen = screenManageAccounts
		return m, nil
	case "up", "down", "j", "k", "tab":
		m.dialogCursor = 1 - m.dialogCursor
	case "enter":
		if m.dialogCursor == 0 {
			m.newAccountAuthType = "api_key"
			m.screen = screenAddAccountAPIKey
			m.inputBuffer = ""
			m.inputPrompt = "API Key"
			m.inputHint = "Paste your Gemini API key here"
		} else {
			m.newAccountAuthType = "google"
			m.screen = screenAddAccountGoogleInfo
		}
	}
	return m, nil
}

func (m model) handleAddAccountAPIKeyKeys(key string, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.screen = screenManageAccounts
		return m, nil
	case tea.KeyEnter:
		if m.inputBuffer == "" {
			return m, nil
		}
		m.newAccountAPIKey = m.inputBuffer
		return m, m.finishAddAccount()
	case tea.KeyBackspace:
		if len(m.inputBuffer) > 0 {
			m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
		}
	case tea.KeyRunes:
		m.inputBuffer += string(msg.Runes)
	}
	return m, nil
}

func (m model) handleAddAccountGoogleInfoKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "esc", "q":
		return m, m.finishAddAccount()
	}
	return m, nil
}

func (m model) finishAddAccount() tea.Cmd {
	return func() tea.Msg {
		accountDir := filepath.Join(m.machinatorDir, "accounts", m.newAccountName)
		geminiDir := filepath.Join(accountDir, ".gemini")

		if m.newAccountAuthType == "api_key" {
			settings := map[string]string{
				"apiKey": m.newAccountAPIKey,
			}
			data, _ := json.MarshalIndent(settings, "", "  ")
			settingsPath := filepath.Join(geminiDir, "settings.json")
			if err := os.WriteFile(settingsPath, data, 0644); err != nil {
				return agentProgressMsg(fmt.Sprintf("✗ Failed to write settings.json: %v", err))
			}
		}

		// Save account.json
		accConfig := map[string]string{
			"name":      m.newAccountName,
			"auth_type": m.newAccountAuthType,
		}
		configData, _ := json.MarshalIndent(accConfig, "", "  ")
		configPath := filepath.Join(accountDir, "account.json")
		if err := os.WriteFile(configPath, configData, 0644); err != nil {
			return agentProgressMsg(fmt.Sprintf("✗ Failed to write account.json: %v", err))
		}

		accounts, _ := GetAccounts(m.machinatorDir)
		return accountAddedMsg{
			accounts: accounts,
		}
	}
}

func (m model) viewManageAccountsLeft() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("👥 Manage Accounts"))
	b.WriteString("\n\n")

	if len(m.accounts) == 0 {
		b.WriteString(dimStyle.Render("  No accounts found\n"))
	} else {
		for i, acc := range m.accounts {
			cursor := "  "
			style := itemStyle
			if i == m.accountCursor {
				cursor = "▸ "
				style = selectedStyle
			}

			status := statusFail
			if acc.Authenticated {
				status = statusOK
			}

			b.WriteString(fmt.Sprintf("%s%s %s %s\n",
				cursor,
				status,
				style.Render(acc.Name),
				dimStyle.Render("("+acc.AuthType+")")))
		}
	}

	b.WriteString("\n")

	addBg := lipgloss.Color("240")
	// Using a special value or just checking if 'a' is pressed?
	// For now let's just show it as a hint
	addBtn := lipgloss.NewStyle().
		Background(addBg).
		Foreground(lipgloss.Color("255")).
		Padding(0, 2).
		Render("Press 'a' to Add Account")

	b.WriteString(addBtn)

	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("Esc to back"))

	return b.String()
}

func (m model) viewAddAccountNameLeft() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("👤 Add Account"))
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

func (m model) viewAddAccountAuthTypeLeft() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("👤 Add Account: " + m.newAccountName))
	b.WriteString("\n\n")

	b.WriteString(sectionStyle.Render("Select Authentication Type"))
	b.WriteString("\n\n")

	apiKeyStyle := itemStyle
	googleStyle := itemStyle

	apiKeyCursor := "  "
	googleCursor := "  "

	if m.dialogCursor == 0 {
		apiKeyStyle = selectedStyle
		apiKeyCursor = "▸ "
	} else {
		googleStyle = selectedStyle
		googleCursor = "▸ "
	}

	b.WriteString(apiKeyCursor + apiKeyStyle.Render("API Key (Manual)") + "\n")
	b.WriteString(googleCursor + googleStyle.Render("Google OAuth (Interactive)") + "\n")

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Use arrow keys to select • Enter to continue"))

	return b.String()
}

func (m model) viewAddAccountAPIKeyLeft() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("👤 Add Account: " + m.newAccountName))
	b.WriteString("\n\n")

	b.WriteString(sectionStyle.Render("Paste Gemini API Key"))
	b.WriteString("\n\n")

	// Text input box with border
	inputBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(0, 1).
		Width(35)

	// Mask API key for display if it's long enough
	displayKey := m.inputBuffer
	if len(displayKey) > 10 {
		displayKey = displayKey[:4] + "...." + displayKey[len(displayKey)-4:]
	}
	inputContent := displayKey + "█"

	b.WriteString(inputBoxStyle.Render(inputContent))
	b.WriteString("\n\n")

	b.WriteString(dimStyle.Render(m.inputHint))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("Enter to save • Esc to cancel"))

	return b.String()
}

func (m model) viewAddAccountGoogleInfoLeft() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("👤 Add Account: " + m.newAccountName))
	b.WriteString("\n\n")

	b.WriteString(sectionStyle.Render("Google OAuth Setup"))
	b.WriteString("\n\n")

	b.WriteString(itemStyle.Render("Account created! To authenticate, run:"))
	b.WriteString("\n\n")

	accountDir := filepath.Join(m.machinatorDir, "accounts", m.newAccountName)
	cmd := fmt.Sprintf("HOME=%s gemini auth", accountDir)

	b.WriteString(lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("86")).
		Padding(1, 2).
		Render(cmd))

	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("Press Enter once you are finished."))

	return b.String()
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

func (m model) recloneAllAgents(p ProjectConfig) tea.Cmd {
	ch := m.progressChan
	return func() tea.Msg {
		projectDir := m.getProjectRootDir(p)
		agentsDir := filepath.Join(projectDir, "agents")

		// Remove all existing agents
		select {
		case ch <- "Removing existing agents...":
		default:
		}
		os.RemoveAll(agentsDir)

		// Clone fresh for each agent
		var wg sync.WaitGroup
		var mu sync.Mutex
		errCount := 0

		for i := 1; i <= p.AgentCount; i++ {
			wg.Add(1)
			go func(agentNum int) {
				defer wg.Done()
				agentDir := filepath.Join(agentsDir, fmt.Sprintf("%d", agentNum))

				select {
				case ch <- fmt.Sprintf("Cloning repo for agent #%d...", agentNum):
				default:
				}

				cmd := exec.Command("git", "clone", p.RepoURL, agentDir)
				if _, err := cmd.CombinedOutput(); err != nil {
					mu.Lock()
					errCount++
					mu.Unlock()
					select {
					case ch <- fmt.Sprintf("✗ Agent #%d clone failed", agentNum):
					default:
					}
				} else {
					select {
					case ch <- fmt.Sprintf("✓ Agent #%d ready", agentNum):
					default:
					}
				}
			}(i)
		}

		wg.Wait()

		if errCount > 0 {
			return agentActionMsg{
				success:    false,
				action:     "reclone",
				agentCount: p.AgentCount,
				err:        fmt.Errorf("%d clone(s) failed", errCount),
			}
		}

		select {
		case ch <- fmt.Sprintf("✓ Finished recloning: %d agents ready", p.AgentCount):
		default:
		}

		return agentActionMsg{
			success:    true,
			action:     "recloned",
			agentCount: p.AgentCount,
			done:       true,
		}
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
		cmd := exec.Command("bd", "--no-db", "list", "--json")
		cmd.Dir = m.newAgentDir
		out, err := cmd.Output()
		if err == nil {
			var tasks []map[string]interface{}
			if json.Unmarshal(out, &tasks) == nil {
				result.tasksTotal = len(tasks)
				for _, task := range tasks {
					if status, ok := task["status"].(string); ok {
						switch status {
						case "open":
							result.tasksReady++
						case "in_progress":
							result.tasksOpen++
						case "closed", "done", "completed":
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
	ch := m.progressChan
	return func() tea.Msg {
		projectDir := m.getProjectRootDir(p)
		currentCount := p.AgentCount

		if desiredCount > currentCount {
			// Add agents in parallel
			var wg sync.WaitGroup
			var mu sync.Mutex
			errCount := 0

			for i := currentCount + 1; i <= desiredCount; i++ {
				wg.Add(1)
				go func(agentNum int) {
					defer wg.Done()
					agentDir := filepath.Join(projectDir, "agents", fmt.Sprintf("%d", agentNum))

					select {
					case ch <- fmt.Sprintf("Setting up agent #%d...", agentNum):
					default:
					}

					select {
					case ch <- fmt.Sprintf("Cloning repo for agent #%d...", agentNum):
					default:
					}

					cmd := exec.Command("git", "clone", p.RepoURL, agentDir)
					if _, err := cmd.CombinedOutput(); err != nil {
						mu.Lock()
						errCount++
						mu.Unlock()
						select {
						case ch <- fmt.Sprintf("✗ Agent #%d setup failed", agentNum):
						default:
						}
					} else {
						select {
						case ch <- fmt.Sprintf("✓ Finished agent #%d setup", agentNum):
						default:
						}
					}
				}(i)
			}

			wg.Wait()

			if errCount > 0 {
				return agentActionMsg{
					success:    false,
					action:     "add",
					agentCount: currentCount,
					err:        fmt.Errorf("%d clone(s) failed", errCount),
				}
			}
		} else if desiredCount < currentCount {
			// Remove agents
			for i := currentCount; i > desiredCount; i-- {
				agentDir := filepath.Join(projectDir, "agents", fmt.Sprintf("%d", i))
				select {
				case ch <- fmt.Sprintf("Cleaning up agent #%d workspace...", i):
				default:
				}
				os.RemoveAll(agentDir)
				select {
				case ch <- fmt.Sprintf("✓ Agent #%d removed", i):
				default:
				}
			}
		}

		// Send final completion message
		select {
		case ch <- fmt.Sprintf("✓ Finished all agent updates: %d available", desiredCount):
		default:
		}

		return agentActionMsg{
			success:    true,
			action:     "updated",
			agentCount: desiredCount,
			done:       true,
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
		DirName:    fmt.Sprintf("%d", m.newProjectID),
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
	projectDir := m.getProjectRootDir(p)
	configPath := filepath.Join(projectDir, "project.json")
	data, _ := json.MarshalIndent(p, "", "  ")
	os.WriteFile(configPath, data, 0644)
}

func (m model) addAgentCmd(p ProjectConfig) tea.Cmd {
	return func() tea.Msg {
		projectDir := m.getProjectRootDir(p)
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

		projectDir := m.getProjectRootDir(p)
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
