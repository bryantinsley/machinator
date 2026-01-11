package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"path/filepath"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Store the original working directory at startup
var originalCwd string

// runfilesDir is the Bazel runfiles directory (if running under bazel run)
var runfilesDir string

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
			Padding(0, 1)

	thinkingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212"))

	toolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86"))

	resultStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))
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
	SleepDuration      time.Duration
	QuotaCheckInterval time.Duration
}

// Messages
type tickMsg time.Time
type quotaMsg int
type tasksMsg []Task
type acpEventMsg ACPEvent // Use our rich ACPEvent type
type logMsg string
type geminiDoneMsg string
type taskFailedMsg struct {
	taskID string
	reason string
}

// Global channels for async communication
var acpEventChan = make(chan ACPEvent, 100)
var geminiDoneChan = make(chan string, 10)

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
	logs            viewport.Model
	quotaPercent    int
	quotaLoaded     bool
	agentName       string
	tickCount       int // Counts 1-second ticks for periodic operations
	cycle           int // Counts Gemini execution cycles (start/complete/kill)
	maxCycles       int
	ready           bool
	config          Config
	currentTaskID   string
	geminiRunning   bool
	geminiCmd       *exec.Cmd
	projectRoot     string
	failedTasks     map[string]time.Time // Track tasks that failed, with retry cooldown
	lastTaskAttempt time.Time            // Prevent rapid task execution attempts
	focusPanel      int                  // 0=tasks, 1=activity
	taskStartTime   time.Time            // When current task started
	lastEventTime   time.Time            // When last ACP event was received
	exitOnce        bool                 // Exit after one task completion (for E2E)
}

func initialModel() model {
	vp := viewport.New(40, 10)
	vp.SetContent("")

	config := Config{
		AgentName:          getEnvOrDefault("BD_AGENT_NAME", "Gemini-01"),
		MaxCycles:          10000,
		SleepDuration:      60 * time.Second,
		QuotaCheckInterval: 5 * time.Minute,
	}

	projectRoot := getProjectRoot()
	return model{
		tasks: []Task{},
		agentActivity: []string{
			"💭 Initializing orchestrator...",
			fmt.Sprintf("📁 Project root: %s", projectRoot),
		},
		eventHistory:    []ACPEvent{},
		eventCursor:     0,
		showEventDetail: false,
		activityScroll:  0,
		focusPanel:      1, // Start focused on activity panel
		logs:            vp,
		quotaPercent:    0,
		agentName:       config.AgentName,
		cycle:           0,
		maxCycles:       config.MaxCycles,
		width:           120,
		height:          30,
		ready:           true,
		config:          config,
		geminiRunning:   false,
		projectRoot:     projectRoot,
		failedTasks:     make(map[string]time.Time),
	}
}

func (m model) Init() tea.Cmd {
	// Log initialization
	// Use cwd-relative paths

	logPath := filepath.Join(originalCwd, "machinator", "logs", "tui_debug.log")
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
	// Debug: log projectRoot to file (using existing logPath)
	f2, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if f2 != nil {
		f2.WriteString(fmt.Sprintf("Init() called, m.projectRoot = %s\n", m.projectRoot))
		f2.WriteString(fmt.Sprintf("Init() about to return tea.Batch with tick(), checkQuota(), fetchTasks()\n"))
		f2.Close()
	}
	return tea.Batch(
		tick(),
		checkQuota(),
		fetchTasks(m.projectRoot),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
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
		case "r":
			if m.showEventDetail {
				// Toggle raw JSON view in detail panel
				m.showRawDetail = !m.showRawDetail
				m.detailScroll = 0
			} else {
				// Refresh tasks/quota
				m.addActivity("🔄 Refreshing...")
				return m, tea.Batch(checkQuota(), fetchTasks(m.projectRoot))
			}
		case "e":
			if !m.geminiRunning {
				taskID := findReadyTask(m.tasks, m.config.AgentName, m.failedTasks)
				if taskID != "" {
					m.currentTaskID = taskID
					m.geminiRunning = true
					m.taskStartTime = time.Now()
					m.lastEventTime = time.Now()
					m.addActivity(fmt.Sprintf("⚡ Executing task: %s", taskID))
					return m, executeTask(&m, taskID, m.config.AgentName)
				}
			}
		case "?":
			m.addActivity("📖 Keys: q=quit r=raw e=execute ↑↓=scroll Enter=details Esc=close")

		// Navigation keys - behavior depends on context
		case "up", "k":
			if m.showEventDetail {
				// ONLY scroll in detail view - never change events
				if m.detailScroll > 0 {
					m.detailScroll--
				}
				// Don't do anything else
			} else if m.focusPanel == 1 {
				// Activity panel focused - change selection
				if m.eventCursor < len(m.eventHistory)-1 {
					m.eventCursor++
				}
			}
		case "down", "j":
			if m.showEventDetail {
				// ONLY scroll in detail view - never change events
				m.detailScroll++
				// Don't do anything else
			} else if m.focusPanel == 1 {
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
				if m.detailEventIdx < len(m.eventHistory)-1 {
					m.detailEventIdx++
					m.detailScroll = 0 // Reset scroll for new event
				}
			}
		case "enter", " ":
			if m.focusPanel == 1 && len(m.eventHistory) > 0 {
				if !m.showEventDetail {
					// Entering detail view - capture the actual array index
					m.detailEventIdx = len(m.eventHistory) - 1 - m.eventCursor
					if m.detailEventIdx < 0 {
						m.detailEventIdx = 0
					}
					if m.detailEventIdx >= len(m.eventHistory) {
						m.detailEventIdx = len(m.eventHistory) - 1
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
			// Cycle through panels
			m.focusPanel = (m.focusPanel + 1) % 2
		case "shift+tab":
			// Cycle backwards through panels
			m.focusPanel = (m.focusPanel + 1) % 2
		case "home":
			if m.showEventDetail {
				m.detailScroll = 0
			} else {
				// Jump to most recent
				m.eventCursor = 0
			}
		case "end":
			// Jump to oldest
			if len(m.eventHistory) > 0 {
				m.eventCursor = len(m.eventHistory) - 1
			}
		case "pgup":
			// Page up
			m.eventCursor += 10
			if m.eventCursor >= len(m.eventHistory) {
				m.eventCursor = len(m.eventHistory) - 1
			}
		case "pgdown":
			// Page down
			m.eventCursor -= 10
			if m.eventCursor < 0 {
				m.eventCursor = 0
			}
		}

	case tea.MouseMsg:
		// Handle mouse clicks in activity panel
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			panelWidth := m.width / 3
			// Check if click is in activity panel (middle panel)
			if msg.X >= panelWidth && msg.X < panelWidth*2 {
				// Focus on activity panel
				m.focusPanel = 1

				// Calculate which event was clicked based on Y position
				// Account for panel header (3 lines) and border
				clickedLine := msg.Y - 5 // Adjust for title bar and panel header
				if clickedLine >= 0 && clickedLine < len(m.eventHistory) {
					// Calculate actual event index (events are displayed newest first)
					newCursor := len(m.eventHistory) - 1 - clickedLine
					if newCursor >= 0 && newCursor < len(m.eventHistory) {
						if newCursor == m.eventCursor {
							// Clicked same item - toggle details
							m.showEventDetail = !m.showEventDetail
						} else {
							m.eventCursor = newCursor
						}
					}
				}
			} else if msg.X < panelWidth {
				// Clicked on tasks panel
				m.focusPanel = 0
				m.showEventDetail = false
			} else {
				// Clicked on logs panel
				m.focusPanel = 2
				m.showEventDetail = false
			}
		} else if msg.Action == tea.MouseActionMotion && msg.Button == tea.MouseButtonWheelUp {
			// Scroll up in activity
			if m.focusPanel == 1 && m.eventCursor < len(m.eventHistory)-1 {
				m.eventCursor++
			}
		} else if msg.Action == tea.MouseActionMotion && msg.Button == tea.MouseButtonWheelDown {
			// Scroll down in activity
			if m.focusPanel == 1 && m.eventCursor > 0 {
				m.eventCursor--
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.logs.Width = (m.width / 3) - 4
		m.logs.Height = m.height - 10

	case tickMsg:
		m.tickCount++

		// Check for any pending ACP events from the goroutine
		for {
			select {
			case event := <-acpEventChan:
				m.addEvent(event)
			default:
				// No more events
				goto doneProcessingEvents
			}
		}
	doneProcessingEvents:

		// Check for Gemini completion
		select {
		case taskID := <-geminiDoneChan:
			if m.geminiRunning && m.currentTaskID == taskID {
				m.geminiRunning = false
				m.cycle++ // Increment cycle when Gemini execution ends
				m.addActivity("✅ Task completed")
				m.addLog(fmt.Sprintf("✓ Cycle %d: Task %s finished", m.cycle, taskID))
				m.currentTaskID = ""
				m.geminiCmd = nil

				// Check for next task
				nextTaskID := findReadyTask(m.tasks, m.config.AgentName, m.failedTasks)
				if nextTaskID != "" {
					m.addLog(fmt.Sprintf("✅ Next task: %s", nextTaskID))
					m.currentTaskID = nextTaskID
					m.geminiRunning = true
					m.taskStartTime = time.Now()
					m.lastEventTime = time.Now()
					m.addActivity(fmt.Sprintf("⚡ Executing: %s", nextTaskID))
					cmds = append(cmds, executeTask(&m, nextTaskID, m.config.AgentName))
				} else {
					m.addLog("⏸ No more ready tasks")
				}
			}
		default:
			// Gemini still running
		}

		// Periodic operations (based on tickCount, not cycle)
		if m.tickCount%25 == 0 { // Every 25 seconds
			m.addLog("🔄 Quota check")
			cmds = append(cmds, checkQuota())
		}
		if m.tickCount%50 == 0 { // Every 50 seconds
			m.addLog("🔄 Task fetch")
			cmds = append(cmds, fetchTasks(m.projectRoot))
		}

		// Auto-execute every 60 seconds
		if !m.geminiRunning && m.tickCount%60 == 0 {
			m.addLog(fmt.Sprintf("🔄 Auto-execute check (%d tasks)", len(m.tasks)))
			m.addActivity("🔍 Looking for ready tasks...")
			taskID := findReadyTask(m.tasks, m.config.AgentName, m.failedTasks)
			if taskID != "" {
				m.addLog(fmt.Sprintf("✅ Found ready task: %s", taskID))
				m.currentTaskID = taskID
				m.geminiRunning = true
				m.taskStartTime = time.Now()
				m.lastEventTime = time.Now()
				m.addActivity(fmt.Sprintf("⚡ Auto-executing: %s", taskID))
				cmds = append(cmds, executeTask(&m, taskID, m.config.AgentName))
			} else {
				m.addLog("No ready tasks")
			}
		}
		// Check for inactivity timeout (5 minutes with no events)
		if m.geminiRunning && !m.lastEventTime.IsZero() {
			sinceAction := time.Since(m.lastEventTime)
			if sinceAction >= 5*time.Minute {
				m.addActivity("⏰ Agent timed out (5m inactive) - killing...")
				m.addLog(fmt.Sprintf("⏰ TIMEOUT: Agent inactive for %s, killing process", formatDuration(sinceAction)))
				if m.geminiCmd != nil && m.geminiCmd.Process != nil {
					m.geminiCmd.Process.Kill()
				}
				m.failedTasks[m.currentTaskID] = time.Now()
				m.geminiRunning = false
				m.geminiCmd = nil
				m.currentTaskID = ""
			}
		}

		cmds = append(cmds, tick())

	case quotaMsg:
		m.quotaPercent = int(msg)
		m.quotaLoaded = true
		// Direct file write for debugging
		logPath := filepath.Join(originalCwd, "machinator", "logs", "tui_debug.log")
		f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			f.WriteString(fmt.Sprintf("[%s] ✅ Quota: %d%%\n", time.Now().Format("15:04:05"), m.quotaPercent))
			f.Close()
		}
		m.addLog(fmt.Sprintf("✅ Quota loaded: %d%%", m.quotaPercent))
		m.addActivity(fmt.Sprintf("📊 Quota: %d%%", m.quotaPercent))
		// Check for ready tasks if we have tasks and quota
		if !m.geminiRunning && len(m.tasks) > 0 {
			taskID := findReadyTask(m.tasks, m.config.AgentName, m.failedTasks)
			if taskID != "" {
				m.addLog(fmt.Sprintf("✅ Ready to execute: %s", taskID))
				m.currentTaskID = taskID
				m.geminiRunning = true
				m.taskStartTime = time.Now()
				m.lastEventTime = time.Now()
				m.addActivity(fmt.Sprintf("⚡ Starting: %s", taskID))
				cmds = append(cmds, executeTask(&m, taskID, m.config.AgentName))
			}
		}

	case tasksMsg:
		m.tasks = msg
		// Direct file write for debugging
		logPath := filepath.Join(originalCwd, "machinator", "logs", "tui_debug.log")
		f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			f.WriteString(fmt.Sprintf("[%s] 📋 Loaded %d tasks\n", time.Now().Format("15:04:05"), len(msg)))
			f.Close()
		}
		m.addLog(fmt.Sprintf("📋 Loaded %d tasks", len(msg)))
		m.addActivity(fmt.Sprintf("📋 %d tasks loaded", len(msg)))
		if len(msg) == 0 {
			m.addLog("⚠️  No tasks found - check projectRoot")
		} else if !m.geminiRunning {
			// Check for ready tasks immediately
			taskID := findReadyTask(m.tasks, m.config.AgentName, m.failedTasks)
			if taskID != "" {
				m.addLog(fmt.Sprintf("✅ Found ready task: %s", taskID))
				m.currentTaskID = taskID
				m.geminiRunning = true
				m.taskStartTime = time.Now()
				m.lastEventTime = time.Now()
				m.addActivity(fmt.Sprintf("⚡ Executing: %s", taskID))
				cmds = append(cmds, executeTask(&m, taskID, m.config.AgentName))
			}
		}

	case acpEventMsg:
		// Format and display the ACP event with icon
		event := ACPEvent(msg)
		displayText := FormatACPEventForDisplay(event, 100)
		m.addActivity(displayText)

		// Also log important events
		if event.Type == ACPEventToolUse || event.Type == ACPEventError {
			m.addLog(displayText)
		}

	case logMsg:
		m.addLog(string(msg))

	case taskFailedMsg:
		// Record the failed task so we don't retry it immediately
		m.failedTasks[msg.taskID] = time.Now()
		m.geminiRunning = false
		m.addActivity(fmt.Sprintf("❌ Task failed: %s", msg.taskID))
		m.addLog(fmt.Sprintf("❌ Task %s failed: %s", msg.taskID, msg.reason))
		m.currentTaskID = ""
		m.geminiCmd = nil
		// Don't immediately retry - wait for next cycle

	case geminiDoneMsg:
		m.geminiRunning = false
		m.addActivity("✅ Task completed")
		m.addLog(fmt.Sprintf("✓ Task %s finished", m.currentTaskID))
		m.currentTaskID = ""
		m.geminiCmd = nil

		if m.exitOnce {
			m.addLog("🏁 exit-once mode: Task finished, exiting...")
			return m, tea.Quit
		}

		// Wait a moment before checking for next task (don't spam)
		m.addLog("🔍 Checking for next task...")
		taskID := findReadyTask(m.tasks, m.config.AgentName, m.failedTasks)
		if taskID != "" {
			m.addLog(fmt.Sprintf("✅ Next task: %s", taskID))
			m.currentTaskID = taskID
			m.geminiRunning = true
			m.taskStartTime = time.Now()
			m.lastEventTime = time.Now()
			m.addActivity(fmt.Sprintf("⚡ Executing: %s", taskID))
			cmds = append(cmds, executeTask(&m, taskID, m.config.AgentName))
		} else {
			m.addLog("⏸ No more ready tasks")
		}
	}

	m.logs, cmd = m.logs.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
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
	}
	for _, fatalErr := range fatalErrors {
		if strings.Contains(event.Content, fatalErr) || strings.Contains(event.Raw, fatalErr) {
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
	logPath := filepath.Join(originalCwd, "machinator", "logs", "tui_debug.log")
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

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var title string
	if m.quotaLoaded {
		quota := strings.Repeat("█", m.quotaPercent/10) + strings.Repeat("░", 10-m.quotaPercent/10)
		status := "Idle"
		if m.geminiRunning {
			// Calculate elapsed times
			elapsed := time.Since(m.taskStartTime)
			sinceAction := time.Since(m.lastEventTime)
			status = fmt.Sprintf("⚡ Running for %s, last active %s ago", formatDuration(elapsed), formatDuration(sinceAction))
		}
		title = titleStyle.Render(fmt.Sprintf(
			"🤖 Machinator    Agent: %s    Quota: %s %d%%    Cycle: %d    %s",
			m.agentName, quota, m.quotaPercent, m.cycle, status,
		))
	} else {
		status := "Idle"
		if m.geminiRunning {
			elapsed := time.Since(m.taskStartTime)
			sinceAction := time.Since(m.lastEventTime)
			status = fmt.Sprintf("⚡ Running for %s, last active %s ago", formatDuration(elapsed), formatDuration(sinceAction))
		}
		title = titleStyle.Render(fmt.Sprintf(
			"🤖 Machinator    Agent: %s    Quota: Loading...    Cycle: %d    %s",
			m.agentName, m.cycle, status,
		))
	}

	// Layout: Tasks (1/4) | Activity (3/4)
	tasksWidth := m.width / 4
	activityWidth := m.width - tasksWidth - 4 // -4 for borders
	panelHeight := m.height - 8

	// Determine panel border styles based on focus
	tasksBorder := panelStyle
	activityBorder := panelStyle

	if m.focusPanel == 0 {
		tasksBorder = tasksBorder.BorderForeground(lipgloss.Color("205"))
	} else {
		activityBorder = activityBorder.BorderForeground(lipgloss.Color("205"))
	}

	// Tasks panel (narrow)
	tasksContent := fmt.Sprintf("📋 Tasks (%d)\n\n", len(m.tasks))
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
		tasksContent += fmt.Sprintf("%s %s%s\n", icon, shortID, highlight)
	}
	tasksPanel := tasksBorder.Width(tasksWidth).Height(panelHeight).Render(tasksContent)

	// Agent activity panel (wide - 3/4 of screen)
	activityHeader := "🤖 Agent Activity"
	if m.focusPanel == 1 {
		activityHeader += " (↑↓ scroll, Enter=details)"
	}
	activityContent := activityHeader + "\n\n"

	// Calculate visible range based on panel height
	visibleLines := panelHeight - 4
	if visibleLines < 5 {
		visibleLines = 5
	}

	// Show events from newest to oldest, highlighting selected
	startIdx := 0
	endIdx := len(m.agentActivity)
	if endIdx > visibleLines {
		// Adjust view window to keep cursor visible
		cursorDisplayPos := len(m.agentActivity) - 1 - m.eventCursor
		if cursorDisplayPos < startIdx {
			startIdx = cursorDisplayPos
		}
		if cursorDisplayPos >= startIdx+visibleLines {
			startIdx = cursorDisplayPos - visibleLines + 1
		}
		endIdx = startIdx + visibleLines
		if endIdx > len(m.agentActivity) {
			endIdx = len(m.agentActivity)
		}
	}

	for i := startIdx; i < endIdx; i++ {
		activity := m.agentActivity[i]
		// Check if this line corresponds to the selected event
		eventIdx := len(m.agentActivity) - 1 - i
		if eventIdx >= 0 && eventIdx < len(m.eventHistory) && eventIdx == m.eventCursor && m.focusPanel == 1 {
			// Highlight selected line
			activityContent += "▶ " + activity + "\n"
		} else {
			activityContent += "  " + activity + "\n"
		}
	}

	// Show scroll indicators
	if startIdx > 0 {
		activityContent = activityContent[:len(activityHeader)+2] + "  ↑ more above\n" + activityContent[len(activityHeader)+3:]
	}
	if endIdx < len(m.agentActivity) {
		activityContent += fmt.Sprintf("  ↓ %d more below\n", len(m.agentActivity)-endIdx)
	}

	agentPanel := activityBorder.Width(activityWidth).Height(panelHeight).Render(activityContent)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, tasksPanel, agentPanel)

	// If showing event details, overlay a detail panel
	// If showing quit confirmation, show modal
	if m.confirmQuit {
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
		panels = lipgloss.Place(
			m.width,
			panelHeight+2,
			lipgloss.Center,
			lipgloss.Center,
			quitModal,
		)
	} else if m.showEventDetail && m.detailEventIdx >= 0 && m.detailEventIdx < len(m.eventHistory) {
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

		panels = lipgloss.Place(
			m.width,
			panelHeight+2,
			lipgloss.Center,
			lipgloss.Center,
			detailPanel,
		)
	}

	statusBar := statusBarStyle.Width(m.width).Render("q: quit  e: execute  ↑↓: scroll  Enter: details  r: raw/refresh  Tab: panel  ?: help")

	return lipgloss.JoinVertical(lipgloss.Left, title, panels, statusBar)
}

// Commands
func tick() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchTasks(projectRoot string) tea.Cmd {
	return func() tea.Msg {
		// Debug log
		logPath := filepath.Join(originalCwd, "machinator", "logs", "tui_debug.log")
		f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			f.WriteString(fmt.Sprintf("[%s] fetchTasks() started, projectRoot=%s\n", time.Now().Format("15:04:05"), projectRoot))
		}

		cmd := exec.Command("bd", "list", "--json")
		cmd.Dir = projectRoot
		output, err := cmd.Output()

		if err != nil {
			if f != nil {
				f.WriteString(fmt.Sprintf("[%s] bd list error: %v\n", time.Now().Format("15:04:05"), err))
				f.Close()
			}
			return logMsg(fmt.Sprintf("❌ bd error: %v", err))
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

func findReadyTask(tasks []Task, agentName string, failedTasks map[string]time.Time) string {
	// First check for in-progress tasks assigned to this agent
	for _, task := range tasks {
		if task.Status == "in_progress" && task.Assignee == agentName {
			// But skip if it recently failed
			if failedTime, failed := failedTasks[task.ID]; failed {
				if time.Since(failedTime) < 5*time.Minute {
					continue // Skip this task, it failed recently
				}
			}
			return task.ID
		}
	}

	projectRoot := getProjectRoot()
	cmd := exec.Command("bd", "ready", "--json")
	cmd.Dir = projectRoot
	output, err := cmd.Output()
	if err == nil {
		var readyTasks []Task
		if json.Unmarshal(output, &readyTasks) == nil && len(readyTasks) > 0 {
			// Find first task that hasn't failed recently
			for _, task := range readyTasks {
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

func executeTask(m *model, taskID, agentName string) tea.Cmd {
	return func() tea.Msg {
		projectRoot := getProjectRoot()
		logPath := filepath.Join(originalCwd, "machinator", "logs", "tui_debug.log")

		// Log task start
		f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f != nil {
			f.WriteString(fmt.Sprintf("[%s] 🚀 executeTask started: %s\n", time.Now().Format("15:04:05"), taskID))
			f.Close()
		}

		// Update task status to in_progress
		cmd := exec.Command("bd", "update", taskID, "--status=in_progress", fmt.Sprintf("--assignee=%s", agentName))
		cmd.Dir = projectRoot
		if err := cmd.Run(); err != nil {
			f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if f != nil {
				f.WriteString(fmt.Sprintf("[%s] ⚠️ bd update error: %v\n", time.Now().Format("15:04:05"), err))
				f.Close()
			}
		}

		// Build directive
		directive, err := buildDirective(agentName, taskID)
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
			f2.WriteString(fmt.Sprintf("[%s] 🤖 Launching gemini in dir: %s\n", time.Now().Format("15:04:05"), projectRoot))
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
		geminiCmd := exec.Command("gemini", "--yolo", "--output-format", "stream-json", directive)
		geminiCmd.Dir = projectRoot
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

		m.geminiCmd = geminiCmd
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

				// Send to channel for main loop to pick up (non-blocking)
				select {
				case acpEventChan <- event:
				default:
					// Channel full - this shouldn't happen with buffer of 100
				}

				// Log raw output for debugging
				f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if f != nil {
					f.WriteString(fmt.Sprintf("[%s] RAW: %s\n", time.Now().Format("15:04:05"), line))
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
			case geminiDoneChan <- taskID:
			default:
				// Channel full - shouldn't happen
			}
		}()

		// CRITICAL: Return nil immediately! The goroutine handles everything async.
		// This allows the Bubble Tea event loop to keep running (processing ticks, etc.)
		return nil
	}
}

func buildDirective(agentName, taskID string) (string, error) {
	projectRoot := getProjectRoot()
	logPath := filepath.Join(originalCwd, "machinator", "logs", "tui_debug.log")

	taskContext := ""
	cmd := exec.Command("bd", "show", taskID)
	cmd.Dir = projectRoot
	if output, err := cmd.Output(); err == nil {
		taskContext = string(output)
	}

	// Use absolute path for AGENTS.md
	agentsPath := filepath.Join(originalCwd, "AGENTS.md")
	projectContext := ""
	if output, err := exec.Command("head", "-100", agentsPath).Output(); err == nil {
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

func main() {
	// Parse command-line flags
	debugMode := flag.Bool("debug", false, "Run a single cycle in debug mode (no TUI)")
	once := flag.Bool("once", false, "Execute one task and exit")
	headless := flag.Bool("headless", false, "Run without TUI")
	flag.Parse()

	// Write startup log before bubbletea takes over
	logPath := filepath.Join(originalCwd, "machinator", "logs", "tui_debug.log")
	os.MkdirAll(filepath.Join(originalCwd, "machinator", "logs"), 0755)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		f.WriteString(fmt.Sprintf("\n=== TUI Started at %s ===\n", time.Now().Format("15:04:05")))
		f.WriteString("VERSION: HEADLESS-SUPPORT-V1\n")
		f.WriteString(fmt.Sprintf("originalCwd: %s\n", originalCwd))
		f.WriteString(fmt.Sprintf("debugMode: %v\n", *debugMode))
		f.WriteString(fmt.Sprintf("once: %v\n", *once))
		f.WriteString(fmt.Sprintf("headless: %v\n", *headless))
		f.Close()
	}

	// Debug mode - run tasks without TUI
	if *debugMode {
		fmt.Println("=== Machinator Debug Mode ===")
		fmt.Printf("Project root: %s\n", originalCwd)

		// Test quota check
		fmt.Println("\n--- Testing Quota Check ---")
		scriptPath := filepath.Join(originalCwd, "machinator", "check_quota.sh")
		fmt.Printf("Script path: %s\n", scriptPath)
		cmd := exec.Command(scriptPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
		fmt.Printf("Output: %s\n", strings.TrimSpace(string(output)))

		// Test task fetch
		fmt.Println("\n--- Testing Task Fetch ---")
		cmd2 := exec.Command("bd", "list", "--json")
		cmd2.Dir = originalCwd
		output2, err := cmd2.Output()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			fmt.Printf("Stderr: Check that 'bd' is in PATH\n")
		} else {
			var tasks []Task
			if err := json.Unmarshal(output2, &tasks); err != nil {
				fmt.Printf("Parse error: %v\n", err)
			} else {
				fmt.Printf("Found %d tasks\n", len(tasks))
				for i, t := range tasks {
					if i >= 5 {
						fmt.Printf("  ... and %d more\n", len(tasks)-5)
						break
					}
					fmt.Printf("  - [%s] %s (%s)\n", t.Status, t.Title, t.ID)
				}
			}
		}

		// Test ready tasks
		fmt.Println("\n--- Testing Ready Tasks ---")
		cmd3 := exec.Command("bd", "ready", "--json")
		cmd3.Dir = originalCwd
		output3, err := cmd3.Output()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			var tasks []Task
			if err := json.Unmarshal(output3, &tasks); err != nil {
				fmt.Printf("Parse error: %v\n", err)
			} else {
				fmt.Printf("Found %d ready tasks\n", len(tasks))
				for _, t := range tasks {
					fmt.Printf("  - %s: %s\n", t.ID, t.Title)
				}
			}
		}

		fmt.Println("\n=== Debug Complete ===")
		return
	}

	m := initialModel()
	m.exitOnce = *once

	// Use tea.WithInputTTY() to open /dev/tty directly for input,
	// allowing the TUI to work even when stdin is not a terminal.
	// In headless environments (like CI/tests), /dev/tty might not be available or usable.
	opts := []tea.ProgramOption{}

	if *headless {
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

	if _, err := p.Run(); err != nil {
		// Log error to file too
		f3, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if f3 != nil {
			f3.WriteString(fmt.Sprintf("[%s] p.Run() error: %v\n", time.Now().Format("15:04:05"), err))
			f3.Close()
		}
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
