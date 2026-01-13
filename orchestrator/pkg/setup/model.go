package setup

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/bryantinsley/machinator/orchestrator/pkg/ui/components"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Data Types
// ═══════════════════════════════════════════════════════════════════════════════

type ProjectConfig struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	RepoURL    string `json:"repo_url"`
	Branch     string `json:"branch"`
	AgentCount int    `json:"agent_count"`
	HasBeads   bool   `json:"has_beads"`
	TasksReady int    `json:"tasks_ready"`
	TasksOpen  int    `json:"tasks_open"`
	TasksDone  int    `json:"tasks_done"`
	TasksTotal int    `json:"tasks_total"`
	CreatedAt  string `json:"created_at"`

	// DirName is the name of the directory containing the project.json file.
	// It is not stored in the JSON file itself.
	DirName string `json:"-"`

	// Configurable settings
	WorktreeStrategy  string        `json:"worktree_strategy"` // "per-task", "per-invocation", "persistent"
	UseAccountPooling bool          `json:"use_account_pooling"`
	IdleTimeout       time.Duration `json:"idle_timeout,omitempty"`
	MaxTaskRuntime    time.Duration `json:"max_task_runtime,omitempty"`
	MaxCycles         int           `json:"max_cycles,omitempty"`
	CooldownPeriod    time.Duration `json:"cooldown_period,omitempty"`
}

func (p *ProjectConfig) GetWorktreeStrategy() string {
	if p.WorktreeStrategy == "" {
		return WorktreeStrategyPerInvocation
	}
	return p.WorktreeStrategy
}

func (p *ProjectConfig) Validate() error {
	strategy := p.GetWorktreeStrategy()
	switch strategy {
	case WorktreeStrategyPerTask, WorktreeStrategyPerInvocation, WorktreeStrategyPersistent:
		return nil
	default:
		return fmt.Errorf("invalid worktree_strategy: %s", strategy)
	}
}

const (
	WorktreeStrategyPerTask       = "per-task"
	WorktreeStrategyPerInvocation = "per-invocation"
	WorktreeStrategyPersistent    = "persistent"
)

type GlobalSettings struct {
	GeminiCLIPath string `json:"gemini_cli_path"`
	GeminiCLIRepo string `json:"gemini_cli_repo"`
	InitializedAt string `json:"initialized_at"`
	NextProjectID int    `json:"next_project_id"`
}

type screen int

const (
	screenInit screen = iota
	screenMain
	screenProjectDetail
	screenAddProject
	screenAddProjectInput
	screenAddProjectBranch
	screenAddProjectCloning
	screenEditAgentCount
	screenEditProjectName
	screenEditProjectRepo
	screenApplyingAgents
	screenConfirmDeleteProject
	screenConfirmExit
	screenManageAccounts
	screenAddAccountName
	screenAddAccountAuthType
	screenAddAccountAPIKey
	screenAddAccountGoogleInfo
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
	showHelp       bool
	statusMessages []string

	// Accounts
	accounts      []AccountInfo
	accountCursor int

	// Machinator state
	machinatorExists bool
	geminiStatus     geminiStatus
	geminiVersion    string

	// Projects
	projects          []ProjectConfig
	projectsLoaded    bool
	selectedProject   int
	detailCursor      int // Cursor position in project detail view
	desiredAgentCount int
	currentAgentOp    int    // Current agent being processed
	targetAgentCount  int    // Final target count
	editBuffer        string // Buffer for editing name/repo
	dialogCursor      int    // 0=Yes, 1=No for dialogs
	editCursor        int    // 0=input, 1=save, 2=cancel for edit fields
	agentCursor       int    // 0=number, 1=apply, 2=cancel for agent count

	// Add project flow
	inputBuffer    string
	inputPrompt    string
	inputHint      string
	inputStep      int
	newProjectID   int
	newProjectName string
	newRepoURL     string
	newBranch      string
	branches       []string
	branchSelector *components.Dropdown
	newProjectDir  string
	newAgentDir    string
	hasBeads       bool
	beadsTasks     int
	beadsOpen      int
	beadsDone      int
	beadsTotal     int
	addWarnings    []string
	progressMsg    string

	// Add account flow
	newAccountName     string
	newAccountAuthType string // Using string to avoid circular dependency if needed, but it should be fine.
	newAccountAPIKey   string

	// Paths
	machinatorDir string
	resourcesDir  string
	projectsDir   string
	geminiCLIDir  string
	settingsFile  string
	geminiCLIPath string

	// Result
	selectedProjectConfig *ProjectConfig

	// Click Dispatcher
	clickDispatcher *components.ClickDispatcher

	// Progress channel for async updates
	progressChan chan string
}

func initialModel() model {
	machinatorDir := GetMachinatorDir()
	resourcesDir := filepath.Join(machinatorDir, "resources")
	projectsDir := filepath.Join(machinatorDir, "projects")

	return model{
		screen:          screenMain,
		machinatorDir:   machinatorDir,
		resourcesDir:    resourcesDir,
		projectsDir:     projectsDir,
		geminiCLIDir:    filepath.Join(resourcesDir, "gemini-cli-mods"),
		settingsFile:    filepath.Join(machinatorDir, "settings.json"),
		geminiCLIPath:   filepath.Join(machinatorDir, "gemini"),
		geminiStatus:    geminiChecking,
		statusMessages:  []string{},
		projects:        []ProjectConfig{},
		addWarnings:     []string{},
		cursor:          1, // Start on first project or add button
		clickDispatcher: components.NewClickDispatcher(nil),
		progressChan:    make(chan string, 20),
	}
}

// Messages
type initCheckMsg struct {
	machinatorExists bool
	geminiStatus     geminiStatus
	geminiVersion    string
	projects         []ProjectConfig
	accounts         []AccountInfo
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

type accountAddedMsg struct {
	accounts []AccountInfo
}

type agentActionMsg struct {
	success    bool
	action     string // "cloned" or "removed"
	agentNum   int    // Which agent was processed
	agentCount int    // Current count after this operation
	done       bool   // Is this the last operation?
	err        error
}

type agentProgressMsg string

type fetchBranchesMsg struct {
	branches []string
	err      error
}

type selectProjectMsg int
type selectAccountMsg int
type selectDetailMsg int
type selectAuthTypeMsg int
type selectGeminiMsg struct{}
type selectEditCursorMsg int
type selectAgentCursorMsg int
type setScreenMsg screen
type startAddProjectMsg struct{}
type geminiAuthDoneMsg struct {
	err error
}

type tickMsg time.Time
