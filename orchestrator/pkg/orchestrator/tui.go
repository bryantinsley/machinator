package orchestrator

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"path/filepath"

	"github.com/bryantinsley/machinator/orchestrator/pkg/accountpool"
	"github.com/bryantinsley/machinator/orchestrator/pkg/setup"
	"github.com/bryantinsley/machinator/orchestrator/pkg/ui/agentgrid"
	"github.com/bryantinsley/machinator/orchestrator/pkg/ui/components"
	"github.com/bryantinsley/machinator/orchestrator/pkg/ui/styles"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Store the original working directory at startup
var originalCwd string

// ErrSwitchToSetup is returned when the user wants to switch to setup mode
var ErrSwitchToSetup = errors.New("switch to setup")

// runfilesDir is the Bazel runfiles directory (if running under bazel run)
var runfilesDir string

var execCommand = exec.Command

func init() {
	// For bazel run, use BUILD_WORKING_DIRECTORY env var
	if bwd := os.Getenv("BUILD_WORKING_DIRECTORY"); bwd != "" {
		originalCwd = bwd
	} else {
		originalCwd, _ = os.Getwd()
	}

	// Detect Bazel runfiles directory
	if rd := os.Getenv("RUNFILES_DIR"); rd != "" {
		runfilesDir = rd
	} else {
		// Try to find runfiles relative to the executable
		exe, _ := os.Executable()
		if exe != "" {
			runfilesDir = exe + ".runfiles"
		}
	}

	// Ensure log directory exists
	logDir := filepath.Join(setup.GetMachinatorDir(), "logs")
	os.MkdirAll(logDir, 0755)
}

// Log path helpers for split logging
func orchestratorLogPath() string {
	return filepath.Join(setup.GetMachinatorDir(), "logs", "orchestrator.log")
}

func agentOrchestratorLogPath(agentID int) string {
	return filepath.Join(setup.GetMachinatorDir(), "logs", fmt.Sprintf("agent%d_orchestrator.log", agentID))
}

func agentGeminiLogPath(agentID int) string {
	return filepath.Join(setup.GetMachinatorDir(), "logs", fmt.Sprintf("agent%d_gemini.log", agentID))
}

// regenerateGeminiWrapper recreates the gemini wrapper script to apply any code fixes
func regenerateGeminiWrapper() {
	machinatorDir := setup.GetMachinatorDir()
	geminiCLIDir := filepath.Join(machinatorDir, "resources", "gemini-cli-mods")
	geminiPath := filepath.Join(machinatorDir, "gemini")

	if _, err := os.Stat(geminiCLIDir); err != nil {
		return
	}

	// Call node directly with index.js to preserve working directory
	indexJs := filepath.Join(geminiCLIDir, "packages", "cli", "dist", "index.js")
	wrapperContent := fmt.Sprintf(`#!/bin/bash
exec node "%s" "$@"
`, indexJs)
	os.WriteFile(geminiPath, []byte(wrapperContent), 0755)
}

// findFile looks for a file in multiple locations: runfiles, workspace, or relative paths
func findFile(relativePath string) string {
	// Try runfiles first (for bazel run)
	if runfilesDir != "" {
		// Bazel runfiles structure: runfiles/<workspace>/<path>
		candidates := []string{
			filepath.Join(runfilesDir, "machinator", relativePath),
			filepath.Join(runfilesDir, "_main", relativePath),
		}
		for _, path := range candidates {
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}

	// Try workspace root (BUILD_WORKING_DIRECTORY or cwd)
	path := filepath.Join(originalCwd, relativePath)
	if _, err := os.Stat(path); err == nil {
		return path
	}

	// Return the workspace path even if not found (let caller handle error)
	return path
}

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Background(lipgloss.Color("235")).
			Padding(0, 1)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("235")).
			Padding(1, 1).
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(lipgloss.Color("238"))

	thinkingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212"))

	toolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86"))

	resultStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))

	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 3).
			Align(lipgloss.Center)
)

type OrchestratorState int

const (
	StateRunning OrchestratorState = iota
	StatePaused
	StateStopped
)

// Task represents a beads task
type Task struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Priority int    `json:"priority"`
	Assignee string `json:"assignee,omitempty"`
}

// DirectiveData holds data for template rendering
type DirectiveData struct {
	AgentName      string
	TaskID         string
	TaskContext    string
	ProjectContext string
}

// Config holds orchestrator configuration
type Config struct {
	AgentName          string
	MaxCycles          int
	SleepDuration      time.Duration // Default sleep between cycles if nothing to do? Actually looks like CooldownPeriod
	QuotaCheckInterval time.Duration
	IdleTimeout        time.Duration
	MaxTaskRuntime     time.Duration
	CooldownPeriod     time.Duration
	PoolingEnabled     bool
}

// AgentState tracks the state of a single agent
type AgentState struct {
	ID            int                  // Agent number (1, 2, 3...)
	Name          string               // e.g., "CoderAgent-01"
	WorktreeDir   string               // Path to agent's worktree
	Running       bool                 // Whether Gemini is running
	CurrentTaskID string               // Current task being worked on
	Cmd           *exec.Cmd            // Running Gemini command
	Account       *accountpool.Account // Account being used
	TaskStartTime time.Time            // When current task started
	LastEventTime time.Time            // When last ACP event was received
	FailedTasks   map[string]time.Time // Tasks that failed with cooldown
}

// Messages
type tickMsg time.Time
type quotaMsg map[string]int
type tasksMsg []Task
type acpEventMsg struct {
	AgentID int
	Event   ACPEvent
}
type logMsg string
type geminiDoneMsg struct {
	AgentID int
	TaskID  string
}
type taskFailedMsg struct {
	agentID int
	taskID  string
	reason  string
}
type taskStartedMsg struct {
	agentID int
	taskID  string
	account *accountpool.Account
	cmd     *exec.Cmd
}
type filterChangedMsg struct{}
type selectEventMsg struct{ index int }
type toggleQuotaMsg struct{}

// Global channels for async communication
var acpEventChan = make(chan acpEventMsg, 100)
var geminiDoneChan = make(chan geminiDoneMsg, 10)

// Thread-safe claimed tasks tracking
var claimedTasksMu sync.Mutex
var claimedTasks = make(map[string]int) // taskID -> agentID

// Retry counter for uncommitted changes recovery
var taskRetryCount = make(map[string]int) // taskID -> retry count
const maxTaskRetries = 5

func claimTask(taskID string, agentID int) bool {
	claimedTasksMu.Lock()
	defer claimedTasksMu.Unlock()
	if _, claimed := claimedTasks[taskID]; claimed {
		return false // Already claimed
	}
	claimedTasks[taskID] = agentID
	return true
}

func releaseTask(taskID string) {
	claimedTasksMu.Lock()
	defer claimedTasksMu.Unlock()
	delete(claimedTasks, taskID)
}

func isTaskClaimed(taskID string) bool {
	claimedTasksMu.Lock()
	defer claimedTasksMu.Unlock()
	_, claimed := claimedTasks[taskID]
	return claimed
}

func incrementRetry(taskID string) int {
	claimedTasksMu.Lock()
	defer claimedTasksMu.Unlock()
	taskRetryCount[taskID]++
	return taskRetryCount[taskID]
}

func clearRetry(taskID string) {
	claimedTasksMu.Lock()
	defer claimedTasksMu.Unlock()
	delete(taskRetryCount, taskID)
}

// Track completed tasks to avoid re-picking them before bd closes them
var completedTasks = make(map[string]time.Time)

func markTaskCompleted(taskID string) {
	claimedTasksMu.Lock()
	defer claimedTasksMu.Unlock()
	completedTasks[taskID] = time.Now()
}

func isTaskCompleted(taskID string) bool {
	claimedTasksMu.Lock()
	defer claimedTasksMu.Unlock()
	if completedAt, exists := completedTasks[taskID]; exists {
		// Consider completed for 10 minutes to give bd time to sync
		if time.Since(completedAt) < 10*time.Minute {
			return true
		}
		// Expired - clean up
		delete(completedTasks, taskID)
	}
	return false
}

type model struct {
	width           int
	height          int
	tasks           []Task
	agentActivity   []string   // Display strings
	eventHistory    []ACPEvent // Full event history for detail view
	eventCursor     int        // Currently selected event when browsing (0 = most recent)
	showEventDetail bool       // Whether to show detail panel
	detailEventIdx  int        // Captured array index when entering detail view (stable)
	showRawDetail   bool       // Whether to show raw JSON in detail view
	detailScroll    int        // Scroll offset for detail view content
	activityScroll  int        // Scroll offset for activity panel
	confirmQuit     bool       // Whether we're waiting for quit confirmation
	showHelp        bool       // Whether to show keyboard help modal
	logs            viewport.Model
	quotas          map[string]AccountQuota // Per-model quotas (Flash, Pro)
	quotaLoaded     bool
	agentName       string
	tickCount       int // Counts 1-second ticks for periodic operations
	cycle           int // Counts Gemini execution cycles (start/complete/kill)
	maxCycles       int
	ready           bool
	config          Config

	// Multi-agent state
	agents map[int]*AgentState // Per-agent state indexed by agent ID

	// Legacy single-agent fields (for backward compatibility during refactor)
	currentTaskID   string
	geminiRunning   bool
	geminiCmd       *exec.Cmd
	projectRoot     string
	repoPath        string               // Path to the git repo (projectRoot/repo for managed projects)
	projectBranch   string               // Branch to pull from
	failedTasks     map[string]time.Time // Track tasks that failed, with retry cooldown
	lastTaskAttempt time.Time            // Prevent rapid task execution attempts
	focusPanel      int                  // 0=tasks, 1=activity
	taskStartTime   time.Time            // When current task started
	lastEventTime   time.Time            // When last ACP event was received
	exitOnce        bool                 // Exit after one task completion (for E2E)
	switchToSetup   bool                 // Whether to switch to setup mode
	toolsCheck      ToolsCheckModel      // Sub-model for tools check
	accountPool     *accountpool.Pool    // Account pool for rotation
	activeAccount   *accountpool.Account // Currently active account
	agentGrid       *agentgrid.AgentGrid // Grid of agents
	agentSelector   *components.Dropdown // Agent selector
	clickDispatcher *components.ClickDispatcher
	filteredIndices []int             // Indices into eventHistory that match filter
	state           OrchestratorState // Current orchestration state
	expandQuota     bool              // Whether to show full quota list
	targetTaskID    string            // Task ID to execute (cli flag)
	refreshingQuota bool              // Whether we are currently fetching quotas
}

func initialModel(projectConfig *setup.ProjectConfig, autoRun bool) model {
	vp := viewport.New(40, 10)
	vp.SetContent("")

	// Default config
	config := Config{
		AgentName:          getEnvOrDefault("BD_AGENT_NAME", "CoderAgent"),
		MaxCycles:          10000,
		SleepDuration:      60 * time.Second,
		QuotaCheckInterval: 5 * time.Minute,
		IdleTimeout:        10 * time.Minute,
		MaxTaskRuntime:     30 * time.Minute,
		CooldownPeriod:     5 * time.Second,
		PoolingEnabled:     true, // Enabled by default
	}

	// Environment overrides for testing
	if val := os.Getenv("MACHINATOR_POOLING_ENABLED"); val != "" {
		config.PoolingEnabled = val == "true"
	}
	if val := os.Getenv("MACHINATOR_IDLE_TIMEOUT"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			config.IdleTimeout = d
		}
	}
	if val := os.Getenv("MACHINATOR_MAX_TASK_RUNTIME"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			config.MaxTaskRuntime = d
		}
	}

	projectRoot := getProjectRoot()
	repoPath := projectRoot // Default: repo is same as project root (cwd mode)
	projectBranch := "main" // Default branch
	// If project config is provided, use it to determine project root and override settings
	if projectConfig != nil {
		machinatorDir := setup.GetMachinatorDir()
		projectRoot = filepath.Join(machinatorDir, "projects", fmt.Sprintf("%d", projectConfig.ID))
		repoPath = filepath.Join(projectRoot, "repo") // Managed projects have repo in subdirectory

		if projectConfig.Branch != "" {
			projectBranch = projectConfig.Branch
		}

		// Override defaults if set in project config
		if projectConfig.MaxCycles > 0 {
			config.MaxCycles = projectConfig.MaxCycles
		}
		if projectConfig.IdleTimeout > 0 {
			config.IdleTimeout = projectConfig.IdleTimeout
		}
		if projectConfig.MaxTaskRuntime > 0 {
			config.MaxTaskRuntime = projectConfig.MaxTaskRuntime
		}
		if projectConfig.CooldownPeriod > 0 {
			config.CooldownPeriod = projectConfig.CooldownPeriod
		}
		config.PoolingEnabled = projectConfig.UseAccountPooling
	}

	machinatorDir := setup.GetMachinatorDir()
	pool := accountpool.NewPool()
	pool.LoadFromDir(machinatorDir)

	// Create initial agent cards
	var cards []*agentgrid.AgentCard
	count := 1
	if projectConfig != nil && projectConfig.AgentCount > 0 {
		count = projectConfig.AgentCount
	}

	for i := 1; i <= count; i++ {
		name := config.AgentName
		if count > 1 {
			name = fmt.Sprintf("%s-%d", config.AgentName, i)
		}
		cards = append(cards, agentgrid.NewAgentCard(name, agentgrid.StatusIdle, "", nil))
	}
	grid := agentgrid.NewAgentGrid(cards, 2)

	// Create agent selector
	options := []string{"All"}
	for _, c := range cards {
		options = append(options, c.Name)
	}
	selector := components.NewDropdown("Show", options, func(selected int) tea.Cmd {
		return func() tea.Msg { return filterChangedMsg{} }
	})

	initialState := StatePaused
	if autoRun {
		initialState = StateRunning
	}

	return model{
		tasks: []Task{},
		agentActivity: []string{
			"💭 Initializing orchestrator...",
			fmt.Sprintf("📁 Project root: %s", projectRoot),
			fmt.Sprintf("📂 Repo path: %s", repoPath),
		},
		eventHistory:    []ACPEvent{},
		eventCursor:     0,
		showEventDetail: false,
		activityScroll:  0,
		focusPanel:      1, // Start focused on activity panel
		logs:            vp,
		quotas:          make(map[string]AccountQuota),
		agentName:       config.AgentName,
		cycle:           0,
		maxCycles:       config.MaxCycles,
		width:           120,
		height:          30,
		ready:           true,
		config:          config,
		agents:          make(map[int]*AgentState),
		geminiRunning:   false,
		projectRoot:     projectRoot,
		repoPath:        repoPath,
		projectBranch:   projectBranch,
		failedTasks:     make(map[string]time.Time),
		toolsCheck:      InitialToolsCheckModel(),
		accountPool:     pool,
		agentGrid:       grid,
		agentSelector:   selector,
		clickDispatcher: components.NewClickDispatcher(nil),
		filteredIndices: []int{},
		state:           initialState,
		expandQuota:     false,
		refreshingQuota: true,
	}
}

func (m model) Init() tea.Cmd {
	// Log initialization
	// Use cwd-relative paths

	logPath := orchestratorLogPath()
	// Ensure logs directory exists
	os.MkdirAll(filepath.Join(originalCwd, "machinator", "logs"), 0755)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating log file %s: %v\n", logPath, err)
	} else {
		f.WriteString(fmt.Sprintf("\n=== TUI Started at %s ===\n", time.Now().Format("15:04:05")))
		// Binary dir: using cwd
		f.WriteString(fmt.Sprintf("Project root: %s\n", m.projectRoot))
		f.Close()
		fmt.Fprintf(os.Stderr, "Log file created: %s\n", logPath)
	}

	// Regenerate gemini wrapper on every startup to apply any fixes
	regenerateGeminiWrapper()

	// Debug: log projectRoot to file (using existing logPath)
	f2, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if f2 != nil {
		f2.WriteString(fmt.Sprintf("Init() called, m.projectRoot = %s\n", m.projectRoot))
		f2.WriteString(fmt.Sprintf("Init() about to return tea.Batch with tick(), checkQuota(), fetchTasks()\n"))
		f2.Close()
	}
	return tea.Batch(
		tick(),
		waitForACPEvent(acpEventChan),
		waitForGeminiDone(geminiDoneChan),
		checkQuota(m.accountPool),
		fetchTasks(m.repoPath, m.projectBranch),
		m.toolsCheck.Init(),
	)
}

func isMinorChange(fileCount, lineCount int) bool {
	return fileCount <= 1 && lineCount < 20
}

func parseDiffStat(output string) int {
	lineCount := 0
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return 0
	}
	// Last line has format "X file(s) changed, Y insertions(+), Z deletions(-)"
	for _, line := range lines {
		if strings.Contains(line, "insertion") || strings.Contains(line, "deletion") {
			// Parse numbers from stat line
			parts := strings.Fields(line)
			for _, p := range parts {
				// Remove trailing commas or other punctuation
				p = strings.TrimRight(p, ",+-()")
				if n, err := strconv.Atoi(p); err == nil {
					lineCount += n
				}
			}
		}
	}
	return lineCount
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// Always handle WindowSizeMsg for both models
	if msg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = msg.Width
		m.height = msg.Height
		m.logs.Width = (m.width / 3) - 4
		m.logs.Height = m.height - 10
	}

	// Delegate to tools check if not passed
	m.toolsCheck, cmd = m.toolsCheck.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	// Delegate to AgentGrid if focused
	if m.focusPanel == 0 { // Grid focused
		var gridModel tea.Model
		gridModel, cmd = m.agentGrid.Update(msg)
		m.agentGrid = gridModel.(*agentgrid.AgentGrid)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	} else if _, ok := msg.(tea.MouseMsg); ok {
		// Always pass mouse events to grid (it handles hit testing)
		// Note: hit testing requires correct X/Y offsets which we might not have yet
		var gridModel tea.Model
		gridModel, cmd = m.agentGrid.Update(msg)
		m.agentGrid = gridModel.(*agentgrid.AgentGrid)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Still process messages even if tools check is active,
	// but we'll be careful about what we do.

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// If help is shown, any key dismisses it
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}

		// Handle quit confirmation first
		if m.confirmQuit {
			switch msg.String() {
			case "y", "Y":
				// Confirmed quit
				if m.geminiCmd != nil && m.geminiCmd.Process != nil {
					m.geminiCmd.Process.Kill()
				}
				return m, tea.Quit
			default:
				// Cancel quit - modal will disappear
				m.confirmQuit = false
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			// Emergency quit - no confirmation needed
			if m.geminiCmd != nil && m.geminiCmd.Process != nil {
				m.geminiCmd.Process.Kill()
			}
			return m, tea.Quit
		case "q":
			// Request quit confirmation - modal will show
			m.confirmQuit = true
		case "S":
			if !m.geminiRunning {
				m.switchToSetup = true
				return m, tea.Quit
			} else {
				m.addActivity("⚠️ Cannot switch while task is running")
			}
		case "r":
			// Refresh tasks/quota
			m.refreshingQuota = true
			return m, tea.Batch(checkQuota(m.accountPool), fetchTasks(m.repoPath, m.projectBranch))

		case "p":
			if m.state == StateRunning {
				m.state = StatePaused
				m.addActivity("⏸ Paused orchestration (current task will finish)")
			}
		case "s":
			if m.state != StateRunning {
				m.state = StateRunning
				m.addActivity("▶ Resumed orchestration")

				// Immediate agent sync to avoid "frozen" UI before first cooldown tick
				for i, card := range m.agentGrid.Cards {
					agentID := i + 1
					if _, exists := m.agents[agentID]; !exists {
						m.agents[agentID] = &AgentState{
							ID:          agentID,
							Name:        card.Name,
							WorktreeDir: filepath.Join(m.projectRoot, "agents", fmt.Sprintf("%d", agentID)),
							Running:     false,
							FailedTasks: make(map[string]time.Time),
						}
					}
				}
			}
		case "x":
			if m.state != StateStopped {
				m.state = StateStopped
				m.addActivity("⏹ Stopped orchestration (killing agents...)")
				if m.geminiCmd != nil && m.geminiCmd.Process != nil {
					m.geminiCmd.Process.Kill()
				}
				// Force cleanup
				if m.currentTaskID != "" {
					m.failedTasks[m.currentTaskID] = time.Now()
				}
				m.geminiRunning = false
				m.geminiCmd = nil
				m.currentTaskID = ""
				m.activeAccount = nil
			}

		case "e":
			if !m.geminiRunning && m.toolsCheck.State == ToolsCheckStatePassed && m.state == StateRunning {
				taskID := findReadyTask(m.tasks, m.config.AgentName, m.failedTasks, m.repoPath)
				if taskID != "" {
					m.addLog(fmt.Sprintf("⚡ Starting task: %s", taskID))
					return m, executeTask(1, taskID, m.config.AgentName, m.projectRoot, m.repoPath, m.accountPool, m.config.PoolingEnabled)
				}
			}
		case "+", "=":
			// Add a new agent
			name := fmt.Sprintf("%s-%d", m.config.AgentName, len(m.agentGrid.Cards)+1)
			m.agentGrid.AddCard(agentgrid.NewAgentCard(name, agentgrid.StatusIdle, "", nil))

			// Update selector options
			options := []string{"All"}
			for _, c := range m.agentGrid.Cards {
				options = append(options, c.Name)
			}
			m.agentSelector.Options = options

			m.addActivity(fmt.Sprintf("➕ Added agent: %s", name))

		case "-", "_":
			// Remove last agent (but keep at least one)
			if len(m.agentGrid.Cards) > 1 {
				name := m.agentGrid.Cards[len(m.agentGrid.Cards)-1].Name
				m.agentGrid.RemoveCard(len(m.agentGrid.Cards) - 1)

				// Update selector options
				options := []string{"All"}
				for _, c := range m.agentGrid.Cards {
					options = append(options, c.Name)
				}
				m.agentSelector.Options = options
				// Fix selection if out of bounds
				if m.agentSelector.Selected >= len(options) {
					m.agentSelector.Selected = 0
				}

				m.addActivity(fmt.Sprintf("➖ Removed agent: %s", name))
			}

		case "?":
			m.showHelp = true
			return m, nil

		// Navigation keys - behavior depends on context
		case "up", "k":
			if m.showEventDetail {
				// ONLY scroll in detail view - never change events
				if m.detailScroll > 0 {
					m.detailScroll--
				}
				// Don't do anything else
			} else if m.focusPanel == 2 {
				// Activity panel focused - change selection
				if m.eventCursor < len(m.filteredIndices)-1 {
					m.eventCursor++
				}
			}
		case "down", "j":
			if m.showEventDetail {
				// ONLY scroll in detail view - never change events
				m.detailScroll++
				// Don't do anything else
			} else if m.focusPanel == 2 {
				// Activity panel focused - change selection
				if m.eventCursor > 0 {
					m.eventCursor--
				}
			}
		case "left", "h":
			if m.showEventDetail {
				// Go to previous (older) event - decrease index in array
				if m.detailEventIdx > 0 {
					m.detailEventIdx--
					m.detailScroll = 0 // Reset scroll for new event
				}
			}
		case "right", "l":
			if m.showEventDetail {
				// Go to next (newer) event - increase index in array
				// filteredIndices contains indices into eventHistory.
				// But detail view iterates the full history?
				// "Go to next (newer) event" in filtered view?
				// If I am in detail view, I should probably stay in filtered view context.
				// But detailEventIdx is currently index into eventHistory.
				// Let's keep detail view traversing full history for now?
				// Or match filtered view? "Next" implies next visible event.
				// If I assume filtered view:
				// I need to know where detailEventIdx maps to in filteredIndices.
				// But detailEventIdx is absolute.
				// Let's stick to full history for detail navigation for now, or use cursor?
				// The prompt says "Selecting an agent switches the event feed".
				// So detail view should probably respect filter.

				// Re-eval: Navigation in detail view uses detailEventIdx.
				// If I want to navigate filtered list, I should track cursor instead?
				// But detail view captures a specific event.

				// Let's leave detail view navigation as is (full history) for now to minimize risk,
				// or update it. Since I updated up/k, I should probably check cursor logic.
				// Only up/k modified above.

				if m.detailEventIdx < len(m.eventHistory)-1 {
					m.detailEventIdx++
					m.detailScroll = 0 // Reset scroll for new event
				}
			}
		case "enter", " ":
			if m.focusPanel == 2 && len(m.filteredIndices) > 0 {
				if !m.showEventDetail {
					// Entering detail view - capture the actual array index
					cursorPos := len(m.filteredIndices) - 1 - m.eventCursor
					if cursorPos >= 0 && cursorPos < len(m.filteredIndices) {
						m.detailEventIdx = m.filteredIndices[cursorPos]
					} else {
						m.detailEventIdx = 0 // Fallback
					}
				}
				m.showEventDetail = !m.showEventDetail
				m.detailScroll = 0      // Reset scroll when opening/closing
				m.showRawDetail = false // Start in normal mode
			}
		case "esc":
			m.showEventDetail = false
			m.showRawDetail = false
			m.detailScroll = 0
		case "tab":
			// Cycle through panels: 0=Grid, 1=Tasks, 2=Activity
			m.focusPanel = (m.focusPanel + 1) % 3
		case "shift+tab":
			// Cycle backwards through panels
			m.focusPanel = (m.focusPanel - 1 + 3) % 3
		case "home":
			if m.showEventDetail {
				m.detailScroll = 0
			} else {
				// Jump to most recent
				m.eventCursor = 0
			}
		case "end":
			// Jump to oldest
			if len(m.filteredIndices) > 0 {
				m.eventCursor = len(m.filteredIndices) - 1
			}
		case "pgup":
			// Page up
			m.eventCursor += 10
			if m.eventCursor >= len(m.filteredIndices) {
				m.eventCursor = len(m.filteredIndices) - 1
			}
		case "pgdown":
			// Page down
			m.eventCursor -= 10
			if m.eventCursor < 0 {
				m.eventCursor = 0
			}
		}

	case tea.MouseMsg:
		// Handle mouse clicks
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			// Use click dispatcher for registered components
			if cmd := m.clickDispatcher.HandleMouse(msg); cmd != nil {
				return m, cmd
			}

			// Rough hit testing for panel focus (to be replaced by components later)
			if msg.Y < 15 {
				m.focusPanel = 0
			} else {
				tasksWidth := m.width / 4
				if msg.X < tasksWidth {
					m.focusPanel = 1
				} else {
					m.focusPanel = 2
				}
			}
		} else if msg.Action == tea.MouseActionMotion && msg.Button == tea.MouseButtonWheelUp {
			// Scroll up in activity
			if m.focusPanel == 2 && m.eventCursor < len(m.eventHistory)-1 {
				m.eventCursor++
			}
		} else if msg.Action == tea.MouseActionMotion && msg.Button == tea.MouseButtonWheelDown {
			// Scroll down in activity
			if m.focusPanel == 2 && m.eventCursor > 0 {
				m.eventCursor--
			}
		}

	case tickMsg:
		m.tickCount++

		// Periodic operations (based on tickCount, not cycle)
		if m.tickCount%25 == 0 { // Every 25 seconds
			m.addLog("🔄 Quota check")
			m.refreshingQuota = true
			cmds = append(cmds, checkQuota(m.accountPool))
		}
		if m.tickCount%50 == 0 { // Every 50 seconds
			m.addLog("🔄 Task fetch")
			cmds = append(cmds, fetchTasks(m.repoPath, m.projectBranch))
		}

		// Auto-execute check (using CooldownPeriod)
		cooldownSecs := int(m.config.CooldownPeriod.Seconds())
		if cooldownSecs < 1 {
			cooldownSecs = 5 // Safe default
		}

		// Multi-agent task dispatch: check each agent and dispatch tasks to idle ones
		if m.toolsCheck.State == ToolsCheckStatePassed && m.state == StateRunning && m.tickCount%cooldownSecs == 0 {
			// Sync agents map with agentGrid.Cards
			for i, card := range m.agentGrid.Cards {
				agentID := i + 1 // Agents are 1-indexed
				if _, exists := m.agents[agentID]; !exists {
					m.agents[agentID] = &AgentState{
						ID:          agentID,
						Name:        card.Name,
						WorktreeDir: filepath.Join(m.projectRoot, "agents", fmt.Sprintf("%d", agentID)),
						Running:     false,
						FailedTasks: make(map[string]time.Time),
					}
				}
			}

			// Try to dispatch tasks to each idle agent
			for agentID, agent := range m.agents {
				if !agent.Running {
					// Find a ready task for this agent
					taskID := findReadyTask(m.tasks, agent.Name, agent.FailedTasks, m.repoPath)
					if taskID != "" {
						// Try to claim the task (atomic)
						if !claimTask(taskID, agentID) {
							continue // Another agent claimed it first
						}
						m.addLog(fmt.Sprintf("🚀 Agent %d: Starting task %s", agentID, taskID))
						cmds = append(cmds, executeTask(agentID, taskID, agent.Name, m.projectRoot, m.repoPath, m.accountPool, m.config.PoolingEnabled))
						agent.Running = true
						agent.CurrentTaskID = taskID
						agent.TaskStartTime = time.Now()
						agent.LastEventTime = time.Now()
					}
				}
			}
		}

		// Legacy single-agent targeted execution (backward compat)
		if !m.geminiRunning && m.toolsCheck.State == ToolsCheckStatePassed && m.targetTaskID != "" && m.quotaLoaded {
			m.addLog(fmt.Sprintf("🎯 Executing targeted task: %s", m.targetTaskID))
			cmds = append(cmds, executeTask(1, m.targetTaskID, m.config.AgentName, m.projectRoot, m.repoPath, m.accountPool, m.config.PoolingEnabled))
			m.targetTaskID = "" // One-shot
			m.exitOnce = true   // Ensure exit after completion
		}

		// Check for timeouts
		if m.geminiRunning {
			now := time.Now()

			// Check each running agent for timeout
			for _, agent := range m.agents {
				if !agent.Running {
					continue
				}

				// 1. Inactivity timeout (no events received)
				if !agent.LastEventTime.IsZero() {
					sinceAction := now.Sub(agent.LastEventTime)
					if sinceAction >= m.config.IdleTimeout {
						m.addActivity(fmt.Sprintf("⏰ Agent %d timed out (%s inactive) - killing...", agent.ID, formatDuration(m.config.IdleTimeout)))
						m.addLog(fmt.Sprintf("⏰ IDLE TIMEOUT: Agent %d inactive for %s, killing process", agent.ID, formatDuration(sinceAction)))
						if agent.Cmd != nil && agent.Cmd.Process != nil {
							agent.Cmd.Process.Kill()
						}
						agent.FailedTasks[agent.CurrentTaskID] = now
						agent.Running = false
						agent.CurrentTaskID = ""
						agent.Cmd = nil
						agent.Account = nil

						if m.exitOnce {
							m.addLog("🏁 exit-once mode: Task timed out, exiting...")
							return m, tea.Quit
						}
					}
				}

				// 2. Max Runtime timeout (task taking too long total)
				if !agent.TaskStartTime.IsZero() {
					runtime := now.Sub(agent.TaskStartTime)
					if runtime >= m.config.MaxTaskRuntime {
						m.addActivity(fmt.Sprintf("⏰ Agent %d timed out (%s runtime) - killing...", agent.ID, formatDuration(m.config.MaxTaskRuntime)))
						m.addLog(fmt.Sprintf("⏰ RUNTIME TIMEOUT: Agent %d ran for %s (max %s), killing process", agent.ID, formatDuration(runtime), formatDuration(m.config.MaxTaskRuntime)))
						if agent.Cmd != nil && agent.Cmd.Process != nil {
							agent.Cmd.Process.Kill()
						}
						agent.FailedTasks[agent.CurrentTaskID] = now
						agent.Running = false
						agent.CurrentTaskID = ""
						agent.Cmd = nil
						agent.Account = nil

						if m.exitOnce {
							m.addLog("🏁 exit-once mode: Task timed out, exiting...")
							return m, tea.Quit
						}
					}
				}
			}
		}

		cmds = append(cmds, tick())

	case quotaDetailMsg:
		m.quotas = msg
		m.quotaLoaded = true
		m.refreshingQuota = false
		m.addActivity(fmt.Sprintf("📊 Quotas loaded for %d accounts", len(m.quotas)))
		// Check for ready tasks if we have tasks and quota
		if !m.geminiRunning && m.toolsCheck.State == ToolsCheckStatePassed && len(m.tasks) > 0 {
			taskID := findReadyTask(m.tasks, m.config.AgentName, m.failedTasks, m.repoPath)
			if taskID != "" {
				m.addLog(fmt.Sprintf("✅ Ready to execute: %s", taskID))
				cmds = append(cmds, executeTask(1, taskID, m.config.AgentName, m.projectRoot, m.repoPath, m.accountPool, m.config.PoolingEnabled))
			}
		}

	case tasksMsg:
		m.tasks = msg
		// Direct file write for debugging
		logPath := orchestratorLogPath()
		f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			f.WriteString(fmt.Sprintf("[%s] 📋 Loaded %d tasks\n", time.Now().Format("15:04:05"), len(msg)))
			f.Close()
		}
		m.addLog(fmt.Sprintf("📋 Loaded %d tasks", len(msg)))
		m.addActivity(fmt.Sprintf("📋 %d tasks loaded", len(msg)))
		if len(msg) == 0 {
			m.addLog("⚠️  No tasks found - check projectRoot")
		} else if !m.geminiRunning && m.toolsCheck.State == ToolsCheckStatePassed {
			// Check for ready tasks immediately
			taskID := findReadyTask(m.tasks, m.config.AgentName, m.failedTasks, m.repoPath)
			if taskID != "" {
				m.addLog(fmt.Sprintf("✅ Found ready task: %s", taskID))
				cmds = append(cmds, executeTask(1, taskID, m.config.AgentName, m.projectRoot, m.repoPath, m.accountPool, m.config.PoolingEnabled))
			}
		}

	case acpEventMsg:
		// Format and display the ACP event with icon
		event := msg.Event
		displayText := FormatACPEventForDisplay(event, 100)
		m.addActivity(displayText)

		// Update agent activity time
		if agent, exists := m.agents[msg.AgentID]; exists {
			agent.LastEventTime = time.Now()
		}
		m.lastEventTime = time.Now() // Global legacy fallback

		// Also log important events
		if event.Type == ACPEventToolUse || event.Type == ACPEventError {
			m.addLog(displayText)
		}

		// Reschedule listener
		cmds = append(cmds, waitForACPEvent(acpEventChan))

	case logMsg:
		m.addLog(string(msg))

	case taskStartedMsg:
		// Legacy single-agent state (keep for backward compat)
		m.currentTaskID = msg.taskID
		m.activeAccount = msg.account
		m.geminiCmd = msg.cmd
		m.geminiRunning = true
		m.taskStartTime = time.Now()
		m.lastEventTime = time.Now()

		// Update per-agent state
		if agent, exists := m.agents[msg.agentID]; exists {
			agent.Running = true
			agent.CurrentTaskID = msg.taskID
			agent.Cmd = msg.cmd
			agent.Account = msg.account
			agent.TaskStartTime = time.Now()
			agent.LastEventTime = time.Now()
		}

		accountName := "default"
		if msg.account != nil {
			accountName = msg.account.Name
		}
		m.addActivity(fmt.Sprintf("⚡ Agent %d: Started task %s (account: %s)", msg.agentID, msg.taskID, accountName))
		m.addLog(fmt.Sprintf("🚀 Agent %d: Task %s started with account %s", msg.agentID, msg.taskID, accountName))

	case taskFailedMsg:
		// Release the task claim
		releaseTask(msg.taskID)
		// Update per-agent state
		if agent, exists := m.agents[msg.agentID]; exists {
			agent.FailedTasks[msg.taskID] = time.Now()
			agent.Running = false
			agent.CurrentTaskID = ""
			agent.Cmd = nil
			agent.Account = nil
		}
		// Legacy single-agent state
		m.failedTasks[msg.taskID] = time.Now()
		m.geminiRunning = false
		m.addActivity(fmt.Sprintf("❌ Agent %d: Task %s failed", msg.agentID, msg.taskID))
		m.addLog(fmt.Sprintf("❌ Agent %d: Task %s failed: %s", msg.agentID, msg.taskID, msg.reason))
		m.currentTaskID = ""
		m.geminiCmd = nil
		m.activeAccount = nil

	case geminiDoneMsg:
		// Reschedule listener
		cmds = append(cmds, waitForGeminiDone(geminiDoneChan))

		// Release the completed task claim
		releaseTask(msg.TaskID)
		// Update per-agent state and start next task
		if agent, exists := m.agents[msg.AgentID]; exists {
			agent.Running = false
			agent.CurrentTaskID = ""
			agent.Cmd = nil
			agent.Account = nil

			// Check for uncommitted changes left behind
			agentDir := filepath.Join(m.projectRoot, "agents", fmt.Sprintf("%d", msg.AgentID))
			statusCmd := execCommand("git", "status", "--porcelain")
			statusCmd.Dir = agentDir
			if output, err := statusCmd.Output(); err == nil && len(output) > 0 {
				// Check if changes are minor (1 file, <20 lines diff)
				fileCount := len(strings.Split(strings.TrimSpace(string(output)), "\n"))
				diffCmd := execCommand("git", "diff", "--stat")
				diffCmd.Dir = agentDir
				diffOutput, _ := diffCmd.Output()
				lineCount := parseDiffStat(string(diffOutput))

				if isMinorChange(fileCount, lineCount) {
					// Minor changes - just discard
					m.addActivity(fmt.Sprintf("🗑️ Agent %d: Minor changes discarded (%d lines)", msg.AgentID, lineCount))
					m.addLog(fmt.Sprintf("🗑️ Agent %d: Task %s minor changes discarded:\n%s", msg.AgentID, msg.TaskID, string(output)))
					discardCmd := execCommand("git", "checkout", "--", ".")
					discardCmd.Dir = agentDir
					discardCmd.Run()
				} else {
					// Significant changes - retry
					retryCount := incrementRetry(msg.TaskID)
					if retryCount <= maxTaskRetries {
						m.addActivity(fmt.Sprintf("⚠️ Agent %d: Uncommitted changes - retry %d/%d", msg.AgentID, retryCount, maxTaskRetries))
						m.addLog(fmt.Sprintf("⚠️ Agent %d: Task %s left uncommitted changes (retry %d/%d):\n%s", msg.AgentID, msg.TaskID, retryCount, maxTaskRetries, string(output)))
						// Restart the same task to let the agent finish
						if claimTask(msg.TaskID, msg.AgentID) {
							cmds = append(cmds, executeTask(msg.AgentID, msg.TaskID, agent.Name, m.projectRoot, m.repoPath, m.accountPool, m.config.PoolingEnabled))
						}
					} else {
						m.addActivity(fmt.Sprintf("❌ Agent %d: Task %s gave up after %d retries", msg.AgentID, msg.TaskID, maxTaskRetries))
						m.addLog(fmt.Sprintf("❌ Agent %d: Task %s exceeded retry limit, uncommitted changes abandoned:\n%s", msg.AgentID, msg.TaskID, string(output)))
						clearRetry(msg.TaskID)
						agent.FailedTasks[msg.TaskID] = time.Now()
					}
				}
			} else {
				clearRetry(msg.TaskID)        // Success - clear any retry count
				markTaskCompleted(msg.TaskID) // Prevent re-picking before bd syncs
				m.addActivity(fmt.Sprintf("✅ Agent %d: Task %s completed", msg.AgentID, msg.TaskID))
				m.addLog(fmt.Sprintf("✓ Agent %d: Task %s finished", msg.AgentID, msg.TaskID))

				// Check for next task for this agent
				if m.toolsCheck.State == ToolsCheckStatePassed && m.state == StateRunning {
					taskID := findReadyTask(m.tasks, agent.Name, agent.FailedTasks, m.repoPath)
					if taskID != "" && claimTask(taskID, msg.AgentID) {
						m.addLog(fmt.Sprintf("🔄 Agent %d: Next task %s", msg.AgentID, taskID))
						cmds = append(cmds, executeTask(msg.AgentID, taskID, agent.Name, m.projectRoot, m.repoPath, m.accountPool, m.config.PoolingEnabled))
					}
				}
			}
		}
		// Legacy single-agent state
		m.geminiRunning = false
		for _, a := range m.agents {
			if a.Running {
				m.geminiRunning = true
				break
			}
		}
		m.currentTaskID = ""
		m.geminiCmd = nil
		m.activeAccount = nil
		m.cycle++ // Increment cycle when Gemini execution ends

		// Handle exit-once mode
		if m.exitOnce {
			m.addLog("🏁 exit-once mode: Task finished, exiting...")
			return m, tea.Quit
		}

	case filterChangedMsg:
		m.applyFilter()

	case selectEventMsg:
		m.focusPanel = 2
		m.eventCursor = len(m.filteredIndices) - 1 - msg.index
		// Show details
		if len(m.filteredIndices) > 0 {
			cursorPos := len(m.filteredIndices) - 1 - m.eventCursor
			if cursorPos >= 0 && cursorPos < len(m.filteredIndices) {
				m.detailEventIdx = m.filteredIndices[cursorPos]
				m.showEventDetail = true
				m.detailScroll = 0
				m.showRawDetail = false
			}
		}

	case toggleQuotaMsg:
		m.expandQuota = !m.expandQuota
	}

	// Post-update synchronization:
	// 1. Update AgentGrid state (sync with per-agent state)
	for i, card := range m.agentGrid.Cards {
		agentID := i + 1
		if agent, exists := m.agents[agentID]; exists {
			card.Task = agent.CurrentTaskID
			if agent.Running {
				card.Status = agentgrid.StatusActive
				// Show elapsed time in the card
				if !agent.TaskStartTime.IsZero() {
					elapsed := time.Since(agent.TaskStartTime)
					card.Elapsed = formatDuration(elapsed)
				}
			} else if _, failed := agent.FailedTasks[agent.CurrentTaskID]; failed {
				card.Status = agentgrid.StatusError
				card.Elapsed = ""
			} else {
				card.Status = agentgrid.StatusIdle
				card.Elapsed = ""
			}
		}
	}

	// 2. Sync global geminiRunning state and first agent's task info for title bar
	anyRunning := false
	for _, agent := range m.agents {
		if agent.Running {
			if !anyRunning {
				// Update global task info from the first running agent found
				m.currentTaskID = agent.CurrentTaskID
				m.taskStartTime = agent.TaskStartTime
				m.lastEventTime = agent.LastEventTime
				m.activeAccount = agent.Account
			}
			anyRunning = true
		}
	}
	m.geminiRunning = anyRunning

	m.logs, cmd = m.logs.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m *model) applyFilter() {
	m.filteredIndices = []int{}
	selectedAgent := "All"
	if m.agentSelector.Selected > 0 && m.agentSelector.Selected < len(m.agentSelector.Options) {
		selectedAgent = m.agentSelector.Options[m.agentSelector.Selected]
	}

	for i, event := range m.eventHistory {
		if selectedAgent == "All" || event.AgentName == "" || event.AgentName == selectedAgent {
			m.filteredIndices = append(m.filteredIndices, i)
		}
	}
	// Clamp cursor
	if len(m.filteredIndices) == 0 {
		m.eventCursor = 0
	} else if m.eventCursor >= len(m.filteredIndices) {
		m.eventCursor = len(m.filteredIndices) - 1
	}
}

func (m model) renderHelpModal() string {
	helpTitle := titleStyle.Render("⌨ Keyboard Shortcuts")

	shortcuts := []struct {
		key  string
		desc string
	}{
		{"?", "Toggle help"},
		{"s", "Start/Resume orchestration"},
		{"p", "Pause orchestration"},
		{"x", "Stop orchestration/Kill agents"},
		{"e", "Execute next ready task"},
		{"+/-", "Add/Remove agents"},
		{"r", "Refresh tasks/quota"},
		{"q", "Quit"},
		{"S", "Switch to Setup mode"},
		{"Tab", "Switch focus between panels"},
		{"↑/↓/k/j", "Navigate event history"},
		{"Enter", "View event details"},
		{"Esc", "Close details/Back"},
		{"ctrl+c", "Force quit"},
	}

	var b strings.Builder
	b.WriteString(helpTitle + "\n\n")

	for _, s := range shortcuts {
		key := lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true).Render(s.key)
		desc := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(s.desc)
		b.WriteString(fmt.Sprintf("%10s : %s\n", key, desc))
	}

	b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Press any key to dismiss"))

	return modalStyle.Render(b.String())
}

func (m *model) addActivity(activity string) {
	timestamp := time.Now().Format("15:04:05")
	m.agentActivity = append(m.agentActivity, fmt.Sprintf("[%s] %s", timestamp, activity))
	// Also add a placeholder event to keep eventHistory in sync with agentActivity
	m.eventHistory = append(m.eventHistory, ACPEvent{
		Type:    ACPEventMessage,
		Role:    "system",
		Content: activity,
		Raw:     activity,
	})

	// Keep more history for scrollback
	if len(m.agentActivity) > 100 {
		m.agentActivity = m.agentActivity[1:]
		m.eventHistory = m.eventHistory[1:]
		// Adjust cursors when old items are removed
		if m.eventCursor > 0 {
			m.eventCursor--
		}
		if m.detailEventIdx > 0 {
			m.detailEventIdx--
		}
	}
	m.applyFilter()
}

// addEvent stores a full ACPEvent and its display string
func (m *model) addEvent(event ACPEvent) {
	timestamp := time.Now().Format("15:04:05")
	// Use wider display width since activity panel is now 3/4 of screen
	displayText := FormatACPEventForDisplay(event, 120)
	m.agentActivity = append(m.agentActivity, fmt.Sprintf("[%s] %s", timestamp, displayText))
	m.eventHistory = append(m.eventHistory, event)

	// Record time of last event for "since last action" display
	m.lastEventTime = time.Now()

	// Check for fatal errors that require killing the Gemini process
	fatalErrors := []string{
		"Command rejected because it could not be parsed safely",
		"FATAL",
		"Quota exceeded",
		"429 Too Many Requests",
	}
	for _, fatalErr := range fatalErrors {
		if strings.Contains(event.Content, fatalErr) || strings.Contains(event.Raw, fatalErr) {
			// Mark account as exhausted if it's a quota error
			if (strings.Contains(event.Content, "Quota exceeded") || strings.Contains(event.Content, "429")) && m.activeAccount != nil {
				m.accountPool.MarkExhausted(m.activeAccount.Name)
				m.addLog(fmt.Sprintf("📉 Account %s marked as exhausted", m.activeAccount.Name))
			}

			m.cycle++ // Increment cycle when Gemini execution ends (killed)
			m.addActivity(fmt.Sprintf("💀 Cycle %d: Fatal error, killing Gemini", m.cycle))
			m.addLog(fmt.Sprintf("💀 Cycle %d: FATAL: %s - killing process", m.cycle, fatalErr))
			if m.geminiCmd != nil && m.geminiCmd.Process != nil {
				m.geminiCmd.Process.Kill()
			}
			// Mark task as failed with cooldown
			if m.currentTaskID != "" {
				m.failedTasks[m.currentTaskID] = time.Now()
			}
			m.geminiRunning = false
			m.geminiCmd = nil
			m.currentTaskID = ""
			m.activeAccount = nil
			return
		}
	}

	// Keep history in sync and limited
	if len(m.agentActivity) > 100 {
		m.agentActivity = m.agentActivity[1:]
		m.eventHistory = m.eventHistory[1:]
		// Adjust cursors when old items are removed
		if m.eventCursor > 0 {
			m.eventCursor--
		}
		if m.detailEventIdx > 0 {
			m.detailEventIdx--
		}
	}
	m.applyFilter()
}

func (m *model) addLog(log string) {
	timestamp := time.Now().Format("15:04:05")

	// Sanitize: replace carriage returns and limit length
	log = strings.ReplaceAll(log, "\r", "")
	log = strings.ReplaceAll(log, "\n", " ")
	if len(log) > 200 {
		log = log[:197] + "..."
	}

	logLine := fmt.Sprintf("[%s] %s", timestamp, log)

	// Add to viewport, limit to 10000 lines for scrollback
	current := m.logs.View()
	lines := strings.Split(current, "\n")
	if len(lines) > 10000 {
		lines = lines[len(lines)-10000:]
		current = strings.Join(lines, "\n")
	}
	m.logs.SetContent(current + "\n" + logLine)
	m.logs.GotoBottom()

	// Also write to file
	logPath := orchestratorLogPath()
	os.MkdirAll(filepath.Join(originalCwd, "machinator", "logs"), 0755)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening log: %v\n", err)
	} else {
		defer f.Close()
		f.WriteString(logLine + "\n")
	}
}

// formatDuration formats a duration as a compact string like "1h23m45s" or "5m30s" or "45s"
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	} else if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// getQuotaStyle returns the lipgloss style for a quota percentage
func (m model) getQuotaStyle(percent int) lipgloss.Style {
	if percent < 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true) // Red bold for error
	} else if percent < 10 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
	} else if percent < 30 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("220")) // Yellow
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("46")) // Green
}

// renderQuotaHearts renders 5 hearts that fade from red to grey based on quota percentage
func (m model) renderQuotaHearts(percent int, flash bool) string {
	if percent < 0 {
		// Error state - grey hearts
		greyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#535360"))
		return greyStyle.Render("♥♥♥♥♥")
	}

	// Clamp to 0-100
	if percent > 100 {
		percent = 100
	}

	// Calculate full hearts and partial heart
	// Each heart = 20%, so 5 hearts = 100%
	fullHearts := percent / 20     // 0-5 full hearts
	partialPercent := percent % 20 // 0-19 for the transitioning heart

	var result string
	heart := "♥"

	// Determine base color based on overall percentage
	baseColor := "#990000" // 0-49%: Red
	if percent >= 80 {
		baseColor = "#008000" // 80-100%: Green
	} else if percent >= 50 {
		baseColor = "#FF8C00" // 50-79%: Orange
	}

	// Sweep animation when refreshing
	sweepIndex := -1
	if m.refreshingQuota {
		sweepIndex = int(m.tickCount % 8) // Sweep 5 hearts + some pause
	}

	for i := 0; i < 5; i++ {
		var heartStyle lipgloss.Style
		if i == sweepIndex {
			heartStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFFF")).Bold(true)
			result += heartStyle.Render(heart)
			continue
		}

		if i < fullHearts {
			// Full heart with base color
			heartStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(baseColor))
		} else if i == fullHearts && partialPercent > 0 {
			// Transitioning heart - blend from base color to grey
			// partialPercent 19 = almost full (baseColor), 1 = almost empty (grey)
			ratio := float64(20-partialPercent) / 20.0
			var br, bg, bb int
			fmt.Sscanf(baseColor, "#%02X%02X%02X", &br, &bg, &bb)

			gr, gg, gb := 83, 83, 96 // Grey RGB (#535360)
			r := int(float64(br) + float64(gr-br)*ratio)
			g := int(float64(bg) + float64(gg-bg)*ratio)
			b := int(float64(bb) + float64(gb-bb)*ratio)

			hexColor := fmt.Sprintf("#%02X%02X%02X", r, g, b)
			heartStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(hexColor))
		} else {
			// Empty grey heart
			heartStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#535360"))
		}
		result += heartStyle.Render(heart)
	}

	// Flash effect when below 10% - alternate with pink
	if percent < 10 && percent > 0 && flash {
		pinkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF69B4")) // Hot pink
		// Re-render all filled hearts as pink for flash effect
		result = ""
		for i := 0; i < 5; i++ {
			var heartStyle lipgloss.Style
			if i < fullHearts || (i == fullHearts && partialPercent > 0) {
				heartStyle = pinkStyle
			} else {
				heartStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#535360"))
			}
			result += heartStyle.Render(heart)
		}
	}

	return result
}

// renderQuotaBar renders a progress bar for a quota percentage
func (m model) renderQuotaBar(percent, barLen int) string {
	if percent < 0 {
		greyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#535360"))
		return greyStyle.Render(strings.Repeat("░", barLen))
	}

	// Clamp to 0-100
	if percent > 100 {
		percent = 100
	}

	filled := percent * barLen / 100

	// Determine color based on percentage with a smooth gradient
	// Green (#00FF00) -> Yellow (#FFFF00) -> Red (#FF0000)
	var r, g, b int
	if percent >= 50 {
		// Green to Yellow: (100-percent)/50 ratio
		ratio := float64(100-percent) / 50.0
		r = int(255.0 * ratio)
		g = 255
		b = 0
	} else {
		// Yellow to Red: (50-percent)/50 ratio
		ratio := float64(50-percent) / 50.0
		r = 255
		g = int(255.0 * (1.0 - ratio))
		b = 0
	}
	hexColor := fmt.Sprintf("#%02X%02X%02X", r, g, b)

	style := lipgloss.NewStyle().Foreground(lipgloss.Color(hexColor))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#535360"))

	// Animation effect when refreshing
	if m.refreshingQuota {
		scanPos := m.tickCount % barLen
		var result strings.Builder
		for i := 0; i < barLen; i++ {
			if i == scanPos {
				// Bright block for scanning effect
				whiteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
				result.WriteString(whiteStyle.Render("█"))
			} else if i < filled {
				result.WriteString(style.Render("█"))
			} else {
				result.WriteString(emptyStyle.Render("░"))
			}
		}
		return result.String()
	}

	return style.Render(strings.Repeat("█", filled)) + emptyStyle.Render(strings.Repeat("░", barLen-filled))
}

func (m model) View() string {
	m.clickDispatcher.Clear()
	m.clickDispatcher.Register(m.agentSelector)

	if m.toolsCheck.State != ToolsCheckStatePassed {
		return m.toolsCheck.View()
	}

	if !m.ready {
		return "Initializing..."
	}

	var title string
	accountInfo := ""
	if m.activeAccount != nil {
		accountInfo = fmt.Sprintf("    Account: %s", m.activeAccount.Name)
		if q, ok := m.quotas[m.activeAccount.Name]; ok {
			// Add compact quota hearts AND bars to title bar
			fHearts := m.renderQuotaHearts(q.Flash, m.tickCount%2 == 0 && q.Flash < 20)
			pHearts := m.renderQuotaHearts(q.Pro, m.tickCount%2 == 0 && q.Pro < 20)
			fBar := m.renderQuotaBar(q.Flash, 10)
			pBar := m.renderQuotaBar(q.Pro, 10)
			accountInfo += fmt.Sprintf(" [F:%s %s P:%s %s]", fHearts, fBar, pHearts, pBar)
		}
	}

	// Determine status string
	status := "Idle"
	if m.state == StatePaused {
		status = "⏸ Paused"
	} else if m.state == StateStopped {
		status = "⏹ Stopped"
	} else if m.geminiRunning {
		// Calculate elapsed times
		elapsed := time.Since(m.taskStartTime)
		sinceAction := time.Since(m.lastEventTime)
		status = fmt.Sprintf("⚡ Running for %s, last active %s ago", formatDuration(elapsed), formatDuration(sinceAction))
	}

	// Simplified title bar (quota details are in the dedicated panel)
	title = titleStyle.Width(m.width).Render(fmt.Sprintf(
		"🤖 Machinator    Agent: %s%s    Cycle: %d    %s",
		m.agentName, accountInfo, m.cycle, status,
	))

	// Register title button for quota toggle
	titleHeight := lipgloss.Height(title)
	titleButton := components.NewButton("", func() tea.Cmd {
		return func() tea.Msg { return toggleQuotaMsg{} }
	})
	titleButton.SetBounds(0, 0, m.width, titleHeight)
	m.clickDispatcher.Register(titleButton)

	// Grid - render first to get its height
	grid := m.agentGrid.View()
	gridHeight := lipgloss.Height(grid)

	// Calculate remaining height for panels
	// Total: title (titleHeight) + grid (gridHeight) + panels + statusBar (3)
	statusBarHeight := 3
	panelHeight := m.height - titleHeight - gridHeight - statusBarHeight - 2 // -2 for some padding
	if panelHeight < 10 {
		panelHeight = 10 // Minimum height
	}

	// Layout: Tasks (1/4) | Activity (3/4)
	tasksWidth := m.width / 4
	activityWidth := m.width - tasksWidth - 4 // -4 for borders

	// Determine panel border styles based on focus
	tasksBorder := panelStyle
	activityBorder := panelStyle

	if m.focusPanel == 1 {
		tasksBorder = tasksBorder.BorderForeground(lipgloss.Color("205"))
	} else if m.focusPanel == 2 {
		activityBorder = activityBorder.BorderForeground(lipgloss.Color("205"))
	}

	// Total Y offset to start of panels
	panelYStart := titleHeight + gridHeight
	// Dropdown is inside Activity Panel, which has 1 line border/padding?
	// activityBorder has padding(1).
	// So content starts at panelYStart + 1 (border) + 1 (padding)
	dropdownY := panelYStart + 2

	// Tasks panel (narrow)
	tasksContent := fmt.Sprintf("📋 Tasks (%d)\n\n", len(m.tasks))
	tasksYOffset := panelYStart + 4
	for i, task := range m.tasks {
		if i >= panelHeight-4 { // Dynamic limit based on height
			tasksContent += fmt.Sprintf("  ... +%d more\n", len(m.tasks)-i)
			break
		}
		icon := "⏸"
		if task.Status == "in_progress" {
			icon = "⚡"
		} else if task.Status == "done" {
			icon = "✓"
		}
		highlight := ""
		if task.ID == m.currentTaskID {
			highlight = " ◀"
		}
		// Strip prefix before dash
		shortID := task.ID
		if idx := strings.LastIndex(task.ID, "-"); idx > 0 {
			shortID = task.ID[idx+1:]
		}

		label := fmt.Sprintf("%s %s%s", icon, shortID, highlight)
		item := components.NewListItem(label, func() tea.Cmd {
			// Click to focus panel? Or just handle selection.
			// Currently clicking doesn't do much for tasks except focus panel.
			return func() tea.Msg { return nil }
		})
		item.SetSelected(task.ID == m.currentTaskID)

		rendered := item.Render()
		tasksContent += rendered + "\n"

		// Register with dispatcher
		// X offset: tasksBorder has padding(1) + border(1) = 2
		item.SetBounds(2, tasksYOffset+i, tasksWidth-2, 1)
		m.clickDispatcher.Register(item)
	}
	tasksPanel := tasksBorder.Width(tasksWidth).Height(panelHeight).MaxHeight(panelHeight).Render(tasksContent)

	// Agent activity panel (wide - 3/4 of screen)
	activityHeader := "🤖 Agent Activity"
	if m.focusPanel == 2 {
		activityHeader += " (↑↓ scroll, Enter=details)"
	}

	// Add dropdown to header
	dropdownView := m.agentSelector.Render()
	// Update dropdown position for hit testing
	// Tasks width + borders/padding.
	// Tasks is width/4. +2 for border?
	// This is approximate but good enough for now.
	m.agentSelector.SetBounds(tasksWidth+2+len(activityHeader)+2, dropdownY, 0, 0) // We rely on Render to set w/h

	activityContent := lipgloss.JoinHorizontal(lipgloss.Center, activityHeader, "   ", dropdownView) + "\n\n"

	// Calculate visible range based on panel height
	visibleLines := panelHeight - 4
	if visibleLines < 5 {
		visibleLines = 5
	}

	// Show events from newest to oldest, highlighting selected
	startIdx := 0
	endIdx := len(m.filteredIndices)
	if endIdx > visibleLines {
		// Adjust view window to keep cursor visible
		cursorDisplayPos := len(m.filteredIndices) - 1 - m.eventCursor
		if cursorDisplayPos < startIdx {
			startIdx = cursorDisplayPos
		}
		if cursorDisplayPos >= startIdx+visibleLines {
			startIdx = cursorDisplayPos - visibleLines + 1
		}
		endIdx = startIdx + visibleLines
		if endIdx > len(m.filteredIndices) {
			endIdx = len(m.filteredIndices)
		}
	}

	for i := startIdx; i < endIdx; i++ {
		realIdx := m.filteredIndices[i]
		activity := m.agentActivity[realIdx]

		// Check if this line corresponds to the selected event
		if i == len(m.filteredIndices)-1-m.eventCursor && m.focusPanel == 2 {
			// Highlight selected line
			activityContent += "▶ " + activity + "\n"
		} else {
			activityContent += "  " + activity + "\n"
		}
	}

	// Show scroll indicators
	if startIdx > 0 {
		// Insert "more above" after header
		// We joined header+dropdown.
		// Finding newline might be tricky if dropdown is expanded.
		// Just append to activityContent for now.
		// Actually, activityContent already has header.
		// We can just add it.
		// But wait, the loop appends lines.
		// Let's just prepend to the list part.
	}

	// Refined rendering:
	listContent := ""
	listYOffset := panelYStart + 5
	if startIdx > 0 {
		listContent += "  ↑ more above\n"
		listYOffset++
	}

	for i := startIdx; i < endIdx; i++ {
		idx := i // Capture for closure
		realIdx := m.filteredIndices[idx]
		activity := m.agentActivity[realIdx]
		isSelected := idx == len(m.filteredIndices)-1-m.eventCursor && m.focusPanel == 2

		item := components.NewListItem(activity, func() tea.Cmd {
			return func() tea.Msg {
				// Handle click: select event and show detail
				// We need to send a message or return a cmd that updates the model.
				// Since we are in View, we can't directly update model.
				// But we can return a command.
				// Let's define a new message type for this.
				return selectEventMsg{index: idx}
			}
		})
		item.SetSelected(isSelected)

		listContent += item.Render() + "\n"

		// Register with dispatcher
		// X offset: tasksWidth + 2 (border/padding) + 2 (list padding)
		item.SetBounds(tasksWidth+4, listYOffset+(idx-startIdx), activityWidth-4, 1)
		m.clickDispatcher.Register(item)
	}

	if endIdx < len(m.filteredIndices) {
		listContent += fmt.Sprintf("  ↓ %d more below\n", len(m.filteredIndices)-endIdx)
	}

	agentPanel := activityBorder.Width(activityWidth).Height(panelHeight).MaxHeight(panelHeight).Render(activityContent + listContent)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, tasksPanel, agentPanel)

	// Build quota panel (to the right of agent grid)
	quotaPanelContent := "📊 Quotas\n"
	// Flash state for low quota warning (toggles each tick)
	flashLow := m.tickCount%2 == 0
	if m.quotaLoaded && len(m.quotas) > 0 {
		// Sort account names for stable output
		var accountNames []string
		for name := range m.quotas {
			accountNames = append(accountNames, name)
		}
		sort.Strings(accountNames)

		// Style definitions
		headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
		sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#535360"))
		sep := sepStyle.Render("│")

		// Column width for account data: hearts(5) + bar(5) + space(1) + %(4) = 16, plus padding
		colWidth := 18

		// Table format: rows = models, columns = accounts
		// Header row with account names (right-aligned model column, centered account names)
		quotaPanelContent += fmt.Sprintf("%10s ", "") // Right-align model names (10 chars)
		for i, name := range accountNames {
			// Truncate name if too long
			displayName := name
			if len(displayName) > colWidth-1 {
				displayName = displayName[:colWidth-3] + ".."
			}
			// Center the account name
			padding := (colWidth - len(displayName)) / 2
			centered := fmt.Sprintf("%*s%s%*s", padding, "", displayName, colWidth-len(displayName)-padding, "")
			if i > 0 {
				quotaPanelContent += sep
			}
			quotaPanelContent += headerStyle.Render(centered)
		}
		quotaPanelContent += "\n"

		// Flash row (right-aligned model name)
		quotaPanelContent += fmt.Sprintf("%10s ", "flash")
		for i, name := range accountNames {
			quota := m.quotas[name]
			if i > 0 {
				quotaPanelContent += sep
			}
			if quota.Flash < 0 {
				errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
				quotaPanelContent += errorStyle.Render(fmt.Sprintf("%-*s", colWidth, "⚠ err"))
			} else {
				hearts := m.renderQuotaHearts(quota.Flash, flashLow && quota.Flash < 10)
				bar := m.renderQuotaBar(quota.Flash, 5)
				quotaPanelContent += fmt.Sprintf("%s %s %3d%%", hearts, bar, quota.Flash)
			}
		}
		quotaPanelContent += "\n"

		// Pro row (right-aligned model name)
		quotaPanelContent += fmt.Sprintf("%10s ", "pro")
		for i, name := range accountNames {
			quota := m.quotas[name]
			if i > 0 {
				quotaPanelContent += sep
			}
			if quota.Pro < 0 {
				errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
				quotaPanelContent += errorStyle.Render(fmt.Sprintf("%-*s", colWidth, "⚠ err"))
			} else {
				hearts := m.renderQuotaHearts(quota.Pro, flashLow && quota.Pro < 10)
				bar := m.renderQuotaBar(quota.Pro, 5)
				quotaPanelContent += fmt.Sprintf("%s %s %3d%%", hearts, bar, quota.Pro)
			}
		}
		quotaPanelContent += "\n"
	} else if !m.quotaLoaded {
		quotaPanelContent += " Loading..."
	} else {
		quotaPanelContent += " No accounts"
	}

	// Create quota panel with same height as grid
	quotaPanelWidth := m.width - lipgloss.Width(grid) - 4
	if quotaPanelWidth < 20 {
		quotaPanelWidth = 20
	}
	quotaPanel := panelStyle.Width(quotaPanelWidth).Height(gridHeight).MaxHeight(gridHeight).Render(quotaPanelContent)

	// Join grid and quota panel horizontally
	topSection := lipgloss.JoinHorizontal(lipgloss.Top, grid, quotaPanel)

	// Add top section above panels
	content := lipgloss.JoinVertical(lipgloss.Left, topSection, panels)

	// If showing event details, overlay a detail panel
	// If showing quit confirmation, show modal
	if m.showHelp {
		content = lipgloss.Place(
			m.width,
			panelHeight+2,
			lipgloss.Center,
			lipgloss.Center,
			m.renderHelpModal(),
		)
	} else if m.confirmQuit {
		modalStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("196")).
			Background(lipgloss.Color("52")).
			Padding(2, 4).
			Align(lipgloss.Center)

		modalContent := "⚠️  Quit Machinator?\n\n"
		if m.geminiRunning {
			modalContent += "Gemini is currently running!\n\n"
		}
		modalContent += "Press Y to quit, any other key to cancel"

		quitModal := modalStyle.Render(modalContent)

		// Overlay on top of panels
		content = lipgloss.Place(
			m.width,
			panelHeight+2,
			lipgloss.Center,
			lipgloss.Center,
			quitModal,
		)
	} else if m.showEventDetail && m.detailEventIdx >= 0 && m.detailEventIdx < len(m.eventHistory) {
		// ... existing detail view logic ...
		// Detail view - use the captured detailEventIdx (stable, not affected by new events)
		event := m.eventHistory[m.detailEventIdx]

		// Show position info (1-indexed, from oldest)
		posInfo := fmt.Sprintf("[%d/%d]", m.detailEventIdx+1, len(m.eventHistory))

		// Build content based on mode
		var lines []string

		if m.showRawDetail {
			lines = append(lines, fmt.Sprintf("📋 Raw JSON %s (←→=nav, r=toggle, ↑↓=scroll)", posInfo))
			lines = append(lines, "")

			// Pretty-print and colorize JSON
			var prettyJSON bytes.Buffer
			if err := json.Indent(&prettyJSON, []byte(event.Raw), "", "  "); err != nil {
				lines = append(lines, strings.Split(event.Raw, "\n")...)
			} else {
				// Colorize the JSON
				colorized := ColorizeJSON(prettyJSON.String())
				lines = append(lines, strings.Split(colorized, "\n")...)
			}
		} else {
			lines = append(lines, fmt.Sprintf("📋 Event %s [%s] (←→=nav, r=raw, ↑↓=scroll)", posInfo, event.Icon()))
			lines = append(lines, "")
			lines = append(lines, fmt.Sprintf("Type: %s", event.Type))
			if event.Role != "" {
				lines = append(lines, fmt.Sprintf("Role: %s", event.Role))
			}
			if event.ToolName != "" {
				lines = append(lines, fmt.Sprintf("Tool: %s", event.ToolName))
			}
			if event.ToolID != "" {
				lines = append(lines, fmt.Sprintf("ID: %s", event.ToolID))
			}
			if len(event.ToolArgs) > 0 {
				lines = append(lines, "Args:")
				argsJSON, _ := json.MarshalIndent(event.ToolArgs, "  ", "  ")
				lines = append(lines, strings.Split("  "+string(argsJSON), "\n")...)
			}
			if event.Content != "" {
				lines = append(lines, "")
				lines = append(lines, "Content:")
				lines = append(lines, strings.Split(event.Content, "\n")...)
			}
			if event.Status != "" {
				lines = append(lines, "")
				lines = append(lines, fmt.Sprintf("Status: %s", event.Status))
			}
		}

		// Calculate visible window
		maxVisible := panelHeight - 6
		if maxVisible < 5 {
			maxVisible = 5
		}

		// Clamp scroll
		maxScroll := len(lines) - maxVisible
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.detailScroll > maxScroll {
			m.detailScroll = maxScroll
		}
		if m.detailScroll < 0 {
			m.detailScroll = 0
		}

		// Get visible lines
		start := m.detailScroll
		end := start + maxVisible
		if end > len(lines) {
			end = len(lines)
		}

		// Build display content
		var displayLines []string
		if start > 0 {
			displayLines = append(displayLines, fmt.Sprintf("  ↑ %d more", start))
		}
		displayLines = append(displayLines, lines[start:end]...)
		if end < len(lines) {
			displayLines = append(displayLines, fmt.Sprintf("  ↓ %d more", len(lines)-end))
		}

		detailContent := strings.Join(displayLines, "\n")

		// Style
		detailWidth := m.width - 20
		if detailWidth > 100 {
			detailWidth = 100
		}
		if detailWidth < 40 {
			detailWidth = 40
		}

		detailStyle := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("212")).
			Background(lipgloss.Color("235")).
			Padding(1, 2).
			Width(detailWidth).
			Height(maxVisible + 4)

		detailPanel := detailStyle.Render(detailContent)

		content = lipgloss.Place(
			m.width,
			panelHeight+2,
			lipgloss.Center,
			lipgloss.Center,
			detailPanel,
		)
	}

	// Dynamic status bar based on state
	var barButtons []*components.Button

	// Start/Resume button
	startLabel := "start"
	if m.state == StatePaused {
		startLabel = "resume"
	}
	startBtn := components.NewButtonWithShortcut("s", startLabel, func() tea.Cmd {
		return func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")} }
	})
	startBtn.Label = styles.IconResumeStyle.Render("▶") + " " + startBtn.Label
	startBtn.Dimmed = m.state == StateRunning
	startBtn.Active = m.state == StatePaused || m.state == StateStopped
	barButtons = append(barButtons, startBtn)

	// Pause button
	pauseBtn := components.NewButtonWithShortcut("p", "pause", func() tea.Cmd {
		return func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")} }
	})
	pauseBtn.Label = styles.IconPauseStyle.Render("⏸") + " " + pauseBtn.Label
	pauseBtn.Dimmed = m.state != StateRunning
	pauseBtn.Active = m.state == StateRunning // Highlight pause when running
	barButtons = append(barButtons, pauseBtn)

	// Stop button
	stopBtn := components.NewButtonWithShortcut("x", "stop", func() tea.Cmd {
		return func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")} }
	})
	stopBtn.Label = styles.IconStopStyle.Render("⏹") + " " + stopBtn.Label
	stopBtn.Dimmed = m.state == StateStopped
	barButtons = append(barButtons, stopBtn)

	// Execute button
	execBtn := components.NewButtonWithShortcut("e", "execute", func() tea.Cmd {
		return func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")} }
	})
	execBtn.Label = styles.IconExecuteStyle.Render("⚡") + " " + execBtn.Label
	execBtn.Dimmed = m.state != StateRunning
	barButtons = append(barButtons, execBtn)

	// Agents button
	agentsBtn := components.NewButtonWithShortcut("+/-", "agents", func() tea.Cmd {
		return func() tea.Msg { return nil } // No-op, just shows it's interactive
	})
	agentsBtn.Label = styles.IconAgentStyle.Render("🤖") + " " + agentsBtn.Label
	barButtons = append(barButtons, agentsBtn)

	// Quit button
	quitBtn := components.NewButtonWithShortcut("q", "quit", func() tea.Cmd {
		return func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")} }
	})
	quitBtn.Label = styles.IconQuitStyle.Render("⏻") + " " + quitBtn.Label
	barButtons = append(barButtons, quitBtn)

	// Help button
	helpBtn := components.NewButtonWithShortcut("?", "help", func() tea.Cmd {
		return func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")} }
	})
	helpBtn.Label = styles.IconHelpStyle.Render("?") + " " + helpBtn.Label
	barButtons = append(barButtons, helpBtn)

	var renderedButtons []string
	currentX := 0
	for _, btn := range barButtons {
		rendered := btn.Render()
		w := lipgloss.Width(rendered)
		// Multi-line buttons are 3 lines high with borders
		btn.SetBounds(currentX, m.height-3, w, 3)
		m.clickDispatcher.Register(btn)
		renderedButtons = append(renderedButtons, rendered)
		currentX += w + 2
	}

	statusBarContent := lipgloss.JoinHorizontal(lipgloss.Top, renderedButtons...)
	statusBar := statusBarStyle.Width(m.width).Render(statusBarContent)

	// Wrap everything in a background color
	fullContent := lipgloss.JoinVertical(lipgloss.Left, title, content, statusBar)
	bgStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#333340")).
		Width(m.width).
		Height(m.height)
	return bgStyle.Render(fullContent)
}

// Run executes the orchestrator TUI.
func Run(debug bool, once bool, headless bool, autoRun bool, executeTaskID string, projectConfig *setup.ProjectConfig) error {
	// Write startup log before bubbletea takes over
	logPath := orchestratorLogPath()
	os.MkdirAll(filepath.Join(originalCwd, "machinator", "logs"), 0755)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		f.WriteString(fmt.Sprintf("\n=== TUI Started at %s ===\n", time.Now().Format("15:04:05")))
		f.WriteString("VERSION: HEADLESS-SUPPORT-V1\n")
		f.WriteString(fmt.Sprintf("originalCwd: %s\n", originalCwd))
		f.WriteString(fmt.Sprintf("debugMode: %v\n", debug))
		f.WriteString(fmt.Sprintf("once: %v\n", once))
		f.WriteString(fmt.Sprintf("headless: %v\n", headless))
		f.WriteString(fmt.Sprintf("autoRun: %v\n", autoRun))
		f.WriteString(fmt.Sprintf("executeTaskID: %v\n", executeTaskID))
		f.Close()
	}

	// Debug mode - run tasks without TUI
	if debug {
		fmt.Println("=== Machinator Debug Mode ===")
		fmt.Printf("Project root: %s\n", originalCwd)
		// ... (keep existing debug logic)
		return nil
	}

	m := initialModel(projectConfig, autoRun)
	m.exitOnce = once
	m.targetTaskID = executeTaskID

	// Use tea.WithInputTTY() to open /dev/tty directly for input,
	// allowing the TUI to work even when stdin is not a terminal.
	// In headless environments (like CI/tests), /dev/tty might not be available or usable.
	opts := []tea.ProgramOption{}

	if headless {
		// Headless mode - explicitly use dummy input and stderr for output
		// to avoid Bubble Tea trying to open /dev/tty
		opts = append(opts, tea.WithInput(strings.NewReader("")), tea.WithOutput(os.Stderr))
	} else if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		tty.Close()
		opts = append(opts, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithInputTTY())
	}

	p := tea.NewProgram(m, opts...)

	// Log that we're about to start the program
	f2, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if f2 != nil {
		f2.WriteString(fmt.Sprintf("[%s] About to call p.Run()\n", time.Now().Format("15:04:05")))
		f2.Close()
	}

	finalModel, err := p.Run()
	if err != nil {
		// Log error to file too
		f3, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f3 != nil {
			f3.WriteString(fmt.Sprintf("[%s] p.Run() error: %v\n", time.Now().Format("15:04:05"), err))
			f3.Close()
		}
		return err
	}

	if m, ok := finalModel.(model); ok && m.switchToSetup {
		return ErrSwitchToSetup
	}

	return nil
}

// Commands
func tick() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func waitForACPEvent(ch <-chan acpEventMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func waitForGeminiDone(ch <-chan geminiDoneMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func fetchTasks(projectRoot, branch string) tea.Cmd {
	return func() tea.Msg {
		// Debug log
		logPath := orchestratorLogPath()
		f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			f.WriteString(fmt.Sprintf("[%s] fetchTasks() started, projectRoot=%s, branch=%s\n", time.Now().Format("15:04:05"), projectRoot, branch))
		}

		// Pull latest to sync beads database
		// Use --ff-only to avoid merge conflicts as requested by task machinator-tmo
		pullCmd := execCommand("git", "pull", "--ff-only", "origin", branch)
		pullCmd.Dir = projectRoot
		if pullErr := pullCmd.Run(); pullErr != nil {
			if f != nil {
				f.WriteString(fmt.Sprintf("[%s] git pull warning (branch %s): %v (continuing anyway)\n", time.Now().Format("15:04:05"), branch, pullErr))
			}
		}

		// Import JSONL to SQLite database (mimics post-merge hook)
		// This ensures the database is in sync after git pull
		importCmd := execCommand("bd", "--sandbox", "import", "-i", ".beads/issues.jsonl")
		importCmd.Dir = projectRoot
		importCmd.Run() // Ignore errors - file might not exist yet

		cmd := execCommand("bd", "--sandbox", "list", "--json")
		cmd.Dir = projectRoot
		output, err := cmd.Output()

		if err != nil {
			// Try bd init in case database doesn't exist yet
			if f != nil {
				f.WriteString(fmt.Sprintf("[%s] bd list failed, trying bd init: %v\n", time.Now().Format("15:04:05"), err))
			}
			initCmd := execCommand("bd", "--sandbox", "init")
			initCmd.Dir = projectRoot
			initCmd.Run() // Ignore errors, maybe it's already initialized

			// Retry bd list
			cmd = execCommand("bd", "--sandbox", "list", "--json")
			cmd.Dir = projectRoot
			output, err = cmd.Output()
			if err != nil {
				if f != nil {
					f.WriteString(fmt.Sprintf("[%s] bd list still failing: %v\n", time.Now().Format("15:04:05"), err))
					f.Close()
				}
				return logMsg(fmt.Sprintf("❌ bd error: %v", err))
			}
		}

		if f != nil {
			f.WriteString(fmt.Sprintf("[%s] bd list returned %d bytes\n", time.Now().Format("15:04:05"), len(output)))
		}

		var tasks []Task
		if err := json.Unmarshal(output, &tasks); err != nil {
			if f != nil {
				f.WriteString(fmt.Sprintf("[%s] JSON parse error: %v\n", time.Now().Format("15:04:05"), err))
				f.Close()
			}
			return logMsg(fmt.Sprintf("❌ parse error: %v", err))
		}

		if f != nil {
			f.WriteString(fmt.Sprintf("[%s] Returning tasksMsg with %d tasks\n", time.Now().Format("15:04:05"), len(tasks)))
			f.Close()
		}

		return tasksMsg(tasks)
	}
}

func findReadyTask(tasks []Task, agentName string, failedTasks map[string]time.Time, projectRoot string) string {
	// First check for in-progress tasks assigned to this agent
	for _, task := range tasks {
		if task.Status == "in_progress" && task.Assignee == agentName {
			// But skip if already claimed by another agent
			if isTaskClaimed(task.ID) {
				continue
			}
			// Skip if recently completed (agent finished but bd not synced yet)
			if isTaskCompleted(task.ID) {
				continue
			}
			// But skip if it recently failed
			if failedTime, failed := failedTasks[task.ID]; failed {
				if time.Since(failedTime) < 5*time.Minute {
					continue // Skip this task, it failed recently
				}
			}
			return task.ID
		}
	}

	cmd := execCommand("bd", "--sandbox", "ready", "--json")
	cmd.Dir = projectRoot
	output, err := cmd.Output()
	if err == nil {
		var readyTasks []Task
		if json.Unmarshal(output, &readyTasks) == nil && len(readyTasks) > 0 {
			// Find first task that hasn't failed recently and isn't claimed
			for _, task := range readyTasks {
				// Skip if already claimed by another agent
				if isTaskClaimed(task.ID) {
					continue
				}
				// Skip if recently completed (agent finished but bd not synced yet)
				if isTaskCompleted(task.ID) {
					continue
				}
				if failedTime, failed := failedTasks[task.ID]; failed {
					if time.Since(failedTime) < 5*time.Minute {
						continue // Skip this task, it failed recently
					}
				}
				return task.ID
			}
		}
	}

	return ""
}

func executeTask(agentID int, taskID, agentName, projectRoot, repoPath string, pool *accountpool.Pool, poolingEnabled bool) tea.Cmd {
	return func() tea.Msg {
		logPath := agentOrchestratorLogPath(agentID)
		geminiLog := agentGeminiLogPath(agentID)

		// Select account from pool if enabled
		var selectedAccount *accountpool.Account
		accounts := pool.GetAccounts()
		if poolingEnabled && len(accounts) > 1 {
			acc, err := pool.NextAvailable()
			if err != nil {
				f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if f != nil {
					f.WriteString(fmt.Sprintf("[%s] ❌ No available accounts: %v\n", time.Now().Format("15:04:05"), err))
					f.Close()
				}
				return taskFailedMsg{taskID: taskID, reason: fmt.Sprintf("no available accounts: %v", err)}
			}
			selectedAccount = acc
		} else if len(accounts) > 0 {
			// Just use the first (default) account
			selectedAccount = &accounts[0]
		}

		// Log task start
		f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			accountName := "default"
			if selectedAccount != nil {
				accountName = selectedAccount.Name
			}
			f.WriteString(fmt.Sprintf("[%s] 🚀 executeTask started: %s (account: %s)\n", time.Now().Format("15:04:05"), taskID, accountName))
			f.Close()
		}

		// Ensure we are on the correct branch if configured
		// Load project config to check for branch
		// Note: projectRoot is .../projects/N
		// We need to read project.json
		// Actually projectRoot IS .../projects/N in this context?
		// executeTask receives projectRoot which is the agent workspace or project root?
		// In main.go/executeTask: projectRoot passed is m.projectRoot which is .../projects/N.
		// Wait, executeTask's gemini command sets cmd.Dir = projectRoot.
		// Gemini expects to run inside the agent workspace (e.g. agents/1).
		// But in tui.go:
		// m.projectRoot = filepath.Join(machinatorDir, "projects", fmt.Sprintf("%d", projectConfig.ID))
		// This is the project root, not the agent workspace.

		// Wait, gemini command runs in projectRoot?
		// "geminiCmd.Dir = projectRoot"
		// If projectRoot is .../projects/1, then gemini runs there.
		// But usually gemini runs in .../projects/1/agents/1 ?
		// Let's check where the task work happens.
		// If gemini runs in project root, it uses the repo in project root?
		// machinator structure:
		// projects/1/
		//   project.json
		//   agents/
		//     1/ (repo clone)
		//     2/ (repo clone)

		// If executeTask runs gemini in projects/1, then gemini must know to use agents/X?
		// Or does tui.go expect to run gemini in an agent directory?
		// Currently tui.go sets geminiCmd.Dir = projectRoot.

		// Let's verify projectRoot in initialModel:
		// projectRoot = filepath.Join(machinatorDir, "projects", fmt.Sprintf("%d", projectConfig.ID))

		// So gemini runs in the project dir.
		// Does gemini manage the agents/ subdirectory?
		// Or should we be running it in a specific agent directory?
		// For single agent, it might matter less if configured right.
		// But with multiple agents (which we just added UI for), we need to select an agent workspace.

		// Multi-agent orchestration:
		// We need to pick an available agent workspace (agents/1, agents/2, etc).
		// Currently executeTask just launches "gemini".
		// Does "gemini" CLI handle workspace selection?
		// If we are passing a directive, gemini acts on the current directory.
		// If current dir is projects/1, it acts on projects/1.
		// But projects/1 contains "agents/" directory, not the user code directly.
		// This seems wrong if we want gemini to work on the user's code.

		// HYPOTHESIS: The current implementation assumes single agent or gemini handles it.
		// BUT looking at setup.go, we clone code into agents/N.
		// So we MUST run gemini in agents/N.

		// REQUIRED FIX: Select an agent workspace and run gemini THERE.
		// We already have account pooling (for auth).
		// We need agent workspace pooling or selection.
		// For now, let's assume agent 1 or find a free one.
		// tui.go doesn't seem to track agent busy state fully per-agent-directory yet,
		// except via m.geminiRunning flag (global lock).

		// Since we have a global lock (m.geminiRunning), we can just use agents/1 for now?
		// Or pick a random one if we want to support true parallelism later.
		// For this task (branch enforcement), we need to ensure the workspace we pick is on the branch.

		// Let's use agents/1 for now to match implicit behavior (or check what existing code did).
		// Existing code: geminiCmd.Dir = projectRoot.
		// If projectRoot is projects/1, and code is in agents/1, gemini might be editing the wrapper?
		// Or maybe I misunderstood projectRoot.

		// Agent worktree path
		agentDir := filepath.Join(projectRoot, "agents", fmt.Sprintf("%d", agentID))

		// Create agent worktree on-demand if it doesn't exist
		if _, err := os.Stat(agentDir); os.IsNotExist(err) {
			repoDir := filepath.Join(projectRoot, "repo")

			// Get target branch from project config
			var pConfig setup.ProjectConfig
			if data, err := os.ReadFile(filepath.Join(projectRoot, "project.json")); err == nil {
				json.Unmarshal(data, &pConfig)
			}
			branch := pConfig.Branch
			if branch == "" {
				branch = "main"
			}

			// Create worktree
			f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if f != nil {
				f.WriteString(fmt.Sprintf("[%s] 🔧 Creating worktree for agent %d...\n", time.Now().Format("15:04:05"), agentID))
				f.Close()
			}

			args := []string{"-C", repoDir, "worktree", "add", "--detach", agentDir, branch}
			cmd := execCommand("git", args...)
			if out, err := cmd.CombinedOutput(); err != nil {
				return taskFailedMsg{agentID: agentID, taskID: taskID, reason: fmt.Sprintf("failed to create worktree: %s", string(out))}
			}

			// Configure git hooks
			hooksCmd := execCommand("git", "config", "core.hooksPath", "scripts/hooks")
			hooksCmd.Dir = agentDir
			hooksCmd.Run()

			// Initialize beads from JSONL
			beadsDir := filepath.Join(agentDir, ".beads")
			if _, err := os.Stat(beadsDir); err == nil {
				bdInit := execCommand("bd", "--sandbox", "init", "--from-jsonl")
				bdInit.Dir = agentDir
				bdInit.Run()
			}
		}

		// Read project config to get branch
		var pConfig setup.ProjectConfig
		if data, err := os.ReadFile(filepath.Join(projectRoot, "project.json")); err == nil {
			json.Unmarshal(data, &pConfig)
		}

		if pConfig.Branch != "" {
			// Check current branch
			cmd := execCommand("git", "branch", "--show-current")
			cmd.Dir = agentDir
			out, _ := cmd.Output()
			currentBranch := strings.TrimSpace(string(out))

			if currentBranch != pConfig.Branch {
				// Create a unique branch for this task based on target branch
				// Worktrees can't share branches, so each agent gets machinator/<taskID>
				taskBranch := fmt.Sprintf("machinator/%s", taskID)

				f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if f != nil {
					f.WriteString(fmt.Sprintf("[%s] 🔄 Creating branch '%s' from '%s' in %s\n",
						time.Now().Format("15:04:05"), taskBranch, pConfig.Branch, agentDir))
					f.Close()
				}

				// Discard any local changes that would block checkout
				resetCmd := execCommand("git", "reset", "--hard")
				resetCmd.Dir = agentDir
				resetCmd.Run()

				// First fetch the latest
				fetchCmd := execCommand("git", "fetch", "origin", pConfig.Branch)
				fetchCmd.Dir = agentDir
				fetchCmd.Run()

				// Delete old task branch if it exists (force fresh start)
				deleteCmd := execCommand("git", "branch", "-D", taskBranch)
				deleteCmd.Dir = agentDir
				deleteCmd.Run() // Ignore errors

				// Create and checkout task branch from origin/<target branch>
				// Use -B to force create/reset even if branch exists
				checkout := execCommand("git", "checkout", "-B", taskBranch, "origin/"+pConfig.Branch)
				checkout.Dir = agentDir
				if err := checkout.Run(); err != nil {
					return taskFailedMsg{agentID: agentID, taskID: taskID, reason: fmt.Sprintf("failed to create branch %s: %v", taskBranch, err)}
				}
			}
		}

		// Ensure beads database is initialized/refreshed from JSONL before update
		// This is needed because git checkout may have pulled new JSONL content
		bdInitCmd := execCommand("bd", "--sandbox", "init", "--from-jsonl")
		bdInitCmd.Dir = agentDir
		bdInitCmd.Run() // Ignore errors - file might not exist

		// Update task status to in_progress
		// Use --sandbox to disable daemon and auto-sync
		// cmd.Dir MUST be agentDir (agent worktree) not repo
		bdUpdateCmd := execCommand("bd", "--sandbox", "update", taskID, "--status=in_progress", fmt.Sprintf("--assignee=%s", agentName))
		bdUpdateCmd.Dir = agentDir
		if err := bdUpdateCmd.Run(); err != nil {
			f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if f != nil {
				f.WriteString(fmt.Sprintf("[%s] ⚠️ bd update error: %v\n", time.Now().Format("15:04:05"), err))
				f.Close()
			}
		}

		// Build directive
		directive, err := buildDirective(agentName, taskID, projectRoot)
		if err != nil {
			f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if f != nil {
				f.WriteString(fmt.Sprintf("[%s] ❌ buildDirective failed: %v - aborting task execution\n", time.Now().Format("15:04:05"), err))
				f.Close()
			}
			// Return failure message to mark task as failed
			return taskFailedMsg{taskID: taskID, reason: fmt.Sprintf("buildDirective failed: %v", err)}
		}

		tmpFile, err := os.CreateTemp("", "directive-*.txt")
		if err != nil {
			f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if f != nil {
				f.WriteString(fmt.Sprintf("[%s] ❌ Failed to create temp file: %v\n", time.Now().Format("15:04:05"), err))
				f.Close()
			}
			return taskFailedMsg{taskID: taskID, reason: fmt.Sprintf("temp file creation: %v", err)}
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.WriteString(directive)
		tmpFile.Close()

		// Log directive info
		f2, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f2 != nil {
			f2.WriteString(fmt.Sprintf("[%s] 📝 Directive created: %s (%d bytes)\n",
				time.Now().Format("15:04:05"), tmpFile.Name(), len(directive)))
			// Log first 200 chars of directive for debugging
			preview := directive
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			f2.WriteString(fmt.Sprintf("[%s] 📝 Directive preview: %s\n", time.Now().Format("15:04:05"), preview))
			f2.WriteString(fmt.Sprintf("[%s] 🤖 Launching gemini in dir: %s\n", time.Now().Format("15:04:05"), agentDir))
			f2.Close()
		}

		// ╔══════════════════════════════════════════════════════════════════════════════╗
		// ║  CRITICAL: DO NOT MODIFY THIS GEMINI EXECUTION MECHANISM                     ║
		// ║                                                                              ║
		// ║  This exact pattern was determined through EXTENSIVE trial and error.        ║
		// ║  The following approaches ALL FAILED and caused "stdin incompatible" errors: ║
		// ║                                                                              ║
		// ║  ❌ Using sh -c with $(cat file)     - stdin detection issue                 ║
		// ║  ❌ Using gemini -i flag             - exit status 42                        ║
		// ║  ❌ Using os.Pipe() for stdin        - stdin detection issue                 ║
		// ║  ❌ Using /dev/null redirect         - stdin detection issue                 ║
		// ║  ❌ Using setsid/Setpgid             - stdin detection issue                 ║
		// ║  ❌ Using bytes.Buffer as stdin      - stdin detection issue                 ║
		// ║  ❌ Using StdinPipe()                - stdin detection issue                 ║
		// ║                                                                              ║
		// ║  The ONLY working solution is:                                               ║
		// ║  1. Pass directive as direct positional arg (no shell)                       ║
		// ║  2. Use exec.Command("gemini", "--output-format", "stream-json", directive)  ║
		// ║  3. Wait in a goroutine for async execution                                  ║
		// ║  4. Notify completion via geminiDoneChan                                     ║
		// ║                                                                              ║
		// ║  This allows the TUI tick to run (updating timers) while Gemini executes.    ║
		// ║  DO NOT "optimize" or "simplify" this code without extensive testing!        ║
		// ╚══════════════════════════════════════════════════════════════════════════════╝

		// Determine model based on CHALLENGE tag in task description
		modelFlag := "gemini-3-flash-preview" // Default to Flash (cheaper, faster)

		// Fetch task description to check for CHALLENGE tag
		bdShowCmd := execCommand("bd", "--sandbox", "show", taskID, "--json")
		bdShowCmd.Dir = repoPath
		if taskOutput, err := bdShowCmd.Output(); err == nil {
			var taskData map[string]interface{}
			if json.Unmarshal(taskOutput, &taskData) == nil {
				if desc, ok := taskData["description"].(string); ok {
					if strings.Contains(desc, "CHALLENGE:complex") {
						modelFlag = "gemini-3-pro-preview" // Use Pro for complex tasks
						f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
						if f != nil {
							f.WriteString(fmt.Sprintf("[%s] 🧠 Complex task detected, using %s\n", time.Now().Format("15:04:05"), modelFlag))
							f.Close()
						}
					} else {
						f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
						if f != nil {
							f.WriteString(fmt.Sprintf("[%s] ⚡ Simple task (default), using %s\n", time.Now().Format("15:04:05"), modelFlag))
							f.Close()
						}
					}
				}
			}
		}

		// Ensure we have the latest code before launching gemini
		pullCmd := execCommand("git", "pull", "--ff-only")
		pullCmd.Dir = agentDir
		if out, err := pullCmd.CombinedOutput(); err != nil {
			output := string(out)
			// Only fail if it's a fast-forward error or conflict
			if strings.Contains(output, "Not possible to fast-forward") || strings.Contains(output, "Conflict") {
				f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if f != nil {
					f.WriteString(fmt.Sprintf("[%s] ❌ git pull conflict: %s\n", time.Now().Format("15:04:05"), output))
					f.Close()
				}
				return taskFailedMsg{agentID: agentID, taskID: taskID, reason: "BLOCKED: GIT_CONFLICT"}
			}
			// Log other errors (like no upstream) but continue
			f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if f != nil {
				f.WriteString(fmt.Sprintf("[%s] ℹ️ git pull skipped/failed: %v\n", time.Now().Format("15:04:05"), strings.TrimSpace(output)))
				f.Close()
			}
		}

		machinatorDir := setup.GetMachinatorDir()
		geminiPath := filepath.Join(machinatorDir, "gemini")
		geminiArgs := []string{"--yolo", "--sandbox", "--model", modelFlag, "--output-format", "stream-json", directive}
		geminiCmd := execCommand(geminiPath, geminiArgs...)
		geminiCmd.Dir = agentDir

		// Log the full command for debugging
		fCmd, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if fCmd != nil {
			fCmd.WriteString(fmt.Sprintf("[%s] 🔧 Command: %s %v\n", time.Now().Format("15:04:05"), geminiPath, geminiArgs))
			fCmd.WriteString(fmt.Sprintf("[%s] 📂 Working dir: %s\n", time.Now().Format("15:04:05"), agentDir))
			fCmd.Close()
		}

		// Set environment variables for sandboxed execution
		// GEMINI_FORCE_FILE_STORAGE bypasses macOS keychain
		geminiCmd.Env = os.Environ()
		geminiCmd.Env = append(geminiCmd.Env, "GEMINI_FORCE_FILE_STORAGE=true")

		// Set HOME and GEMINI_CLI_HOME to the account directory
		if selectedAccount != nil {
			geminiCmd.Env = append(geminiCmd.Env, "HOME="+selectedAccount.HomeDir)
			geminiCmd.Env = append(geminiCmd.Env, "GEMINI_CLI_HOME="+selectedAccount.HomeDir)
			f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if f != nil {
				f.WriteString(fmt.Sprintf("[%s] 🏠 HOME=%s\n", time.Now().Format("15:04:05"), selectedAccount.HomeDir))
				f.Close()
			}
		}

		// Enable custom sandbox profile - file is at .gemini/sandbox-macos-custom.sb relative to agentDir
		geminiCmd.Env = append(geminiCmd.Env, "SEATBELT_PROFILE=custom")

		// Merge stderr into stdout for unified output capture
		geminiCmd.Stderr = geminiCmd.Stdout

		// Stream output - must get pipe BEFORE Start()
		stdout, err := geminiCmd.StdoutPipe()
		if err != nil {
			f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if f != nil {
				f.WriteString(fmt.Sprintf("[%s] ❌ Failed to create stdout pipe: %v\n", time.Now().Format("15:04:05"), err))
				f.Close()
			}
			return taskFailedMsg{taskID: taskID, reason: fmt.Sprintf("stdout pipe: %v", err)}
		}

		if err := geminiCmd.Start(); err != nil {
			f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if f != nil {
				f.WriteString(fmt.Sprintf("[%s] ❌ Failed to start gemini: %v\n", time.Now().Format("15:04:05"), err))
				f.Close()
			}
			return taskFailedMsg{taskID: taskID, reason: fmt.Sprintf("gemini start: %v", err)}
		}

		f3, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f3 != nil {
			f3.WriteString(fmt.Sprintf("[%s] ✅ Gemini process started (PID: %d)\n", time.Now().Format("15:04:05"), geminiCmd.Process.Pid))
			f3.Close()
		}

		// ═══════════════════════════════════════════════════════════════════════════════
		// ASYNC GOROUTINE: Reads stdout, waits for completion, notifies via channel.
		// This MUST be a goroutine so that executeTask returns immediately,
		// allowing the TUI tick to continue running (updating timers, etc.)
		// ═══════════════════════════════════════════════════════════════════════════════
		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()
				if len(line) == 0 {
					continue
				}

				// Parse the line as an ACP event
				event := ParseACPEvent(line)
				event.AgentName = agentName

				// Send to channel for main loop to pick up (non-blocking)
				select {
				case acpEventChan <- acpEventMsg{AgentID: agentID, Event: event}:
				default:
					// Channel full - this shouldn't happen with buffer of 100
				}

				// Log raw output to gemini log file
				f, _ := os.OpenFile(geminiLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if f != nil {
					f.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05"), line))
					f.Close()
				}
			}

			// Wait for gemini to complete
			err := geminiCmd.Wait()
			f4, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if f4 != nil {
				if err != nil {
					f4.WriteString(fmt.Sprintf("[%s] ⚠️ Gemini exited with error: %v\n", time.Now().Format("15:04:05"), err))
				} else {
					f4.WriteString(fmt.Sprintf("[%s] ✅ Gemini completed successfully for task %s\n", time.Now().Format("15:04:05"), taskID))
				}
				f4.Close()
			}

			// Notify completion via channel - tick handler will pick this up
			select {
			case geminiDoneChan <- geminiDoneMsg{AgentID: agentID, TaskID: taskID}:
			default:
				// Channel full - shouldn't happen
			}
		}()

		return taskStartedMsg{agentID: agentID, taskID: taskID, account: selectedAccount, cmd: geminiCmd}
	}
}

func buildDirective(agentName, taskID, projectRoot string) (string, error) {
	logPath := orchestratorLogPath()

	taskContext := ""
	cmd := execCommand("bd", "--sandbox", "show", taskID)
	cmd.Dir = projectRoot
	if output, err := cmd.Output(); err == nil {
		taskContext = string(output)
	}

	// Use absolute path for AGENTS.md
	agentsPath := filepath.Join(originalCwd, "AGENTS.md")
	projectContext := ""
	if output, err := execCommand("head", "-100", agentsPath).Output(); err == nil {
		projectContext = string(output)
	}

	// Find template in runfiles or workspace
	templatePath := findFile(filepath.Join("templates", "directive_template.txt"))
	tmplContent, err := os.ReadFile(templatePath)
	if err != nil {
		// Log the error
		f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			f.WriteString(fmt.Sprintf("[%s] ❌ buildDirective error reading template: %v (path: %s)\n",
				time.Now().Format("15:04:05"), err, templatePath))
			f.Close()
		}
		return "", err
	}

	tmpl, err := template.New("directive").Parse(string(tmplContent))
	if err != nil {
		f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			f.WriteString(fmt.Sprintf("[%s] ❌ buildDirective template parse error: %v\n",
				time.Now().Format("15:04:05"), err))
			f.Close()
		}
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, DirectiveData{
		AgentName:      agentName,
		TaskID:         taskID,
		TaskContext:    taskContext,
		ProjectContext: projectContext,
	}); err != nil {
		f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			f.WriteString(fmt.Sprintf("[%s] ❌ buildDirective template execute error: %v\n",
				time.Now().Format("15:04:05"), err))
			f.Close()
		}
		return "", err
	}

	return buf.String(), nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
