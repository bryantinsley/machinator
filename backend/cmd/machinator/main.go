package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bryantinsley/machinator/backend/internal/beads"
	"github.com/bryantinsley/machinator/backend/internal/config"
	"github.com/bryantinsley/machinator/backend/internal/project"
	"github.com/bryantinsley/machinator/backend/internal/quota"
	"github.com/bryantinsley/machinator/backend/internal/setup"
	"github.com/bryantinsley/machinator/backend/internal/state"
	"github.com/bryantinsley/machinator/backend/internal/tui"
)

func usage() {
	fmt.Println(`machinator - Autonomous Agent Orchestration System

Usage:
  machinator <command> [options]

Commands:
  run            Run the orchestrator
  setup          Setup project (clone repo, build gemini CLI)
  project        List/create/show project configs
  quota          Dump quota for all accounts
  select-task    Show what task would be selected
  help           Show this help

Environment:
  MACHINATOR_DIR   Base directory (default: ~/.machinator)
`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "quota":
		quotaCmd()
	case "select-task":
		selectTaskCmd()
	case "setup":
		setupCmd()
	case "project":
		projectCmd()
	case "run":
		runCmd()
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		usage()
		os.Exit(1)
	}
}

func quotaCmd() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	q := quota.New(cfg.MachinatorDir)
	if err := q.Refresh(); err != nil {
		fmt.Fprintf(os.Stderr, "Error refreshing quota: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Account Quotas:")
	fmt.Println("---------------")
	for _, acc := range q.Accounts {
		fmt.Printf("%s:\n", acc.Name)
		for model, remaining := range acc.Models {
			fmt.Printf("  %s: %.0f%%\n", model, remaining*100)
		}
	}
}

func setupCmd() {
	// Parse flags
	projectID := ""
	repoURL := ""
	branch := "main"
	buildGemini := false

	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "--project=") {
			projectID = strings.TrimPrefix(arg, "--project=")
		} else if strings.HasPrefix(arg, "--repo=") {
			repoURL = strings.TrimPrefix(arg, "--repo=")
		} else if strings.HasPrefix(arg, "--branch=") {
			branch = strings.TrimPrefix(arg, "--branch=")
		} else if arg == "--build-gemini" {
			buildGemini = true
		}
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	s := setup.New(cfg.MachinatorDir)

	// Ensure base directories exist
	fmt.Println("Creating directories...")
	if err := s.EnsureDirectories(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directories: %v\n", err)
		os.Exit(1)
	}

	// Build gemini CLI if requested
	if buildGemini {
		fmt.Println("Building gemini CLI...")
		if err := s.BuildGeminiCLI(); err != nil {
			fmt.Fprintf(os.Stderr, "Error building gemini CLI: %v\n", err)
			os.Exit(1)
		}
	}

	// Clone/update project if specified
	if repoURL != "" {
		if projectID == "" {
			projectID = "1"
		}
		fmt.Printf("Setting up project %s...\n", projectID)

		id, _ := strconv.Atoi(projectID)
		repoDir, err := s.CloneRepo(id, repoURL, branch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error cloning repo: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Repo at: %s\n", repoDir)
	}

	fmt.Println("Setup complete!")
}

func projectCmd() {
	// Parse flags
	projectID := ""
	create := false
	edit := false
	repo := ""
	branch := "main"

	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "--project=") {
			projectID = strings.TrimPrefix(arg, "--project=")
		} else if arg == "--create" {
			create = true
		} else if arg == "--edit" {
			edit = true
		} else if strings.HasPrefix(arg, "--repo=") {
			repo = strings.TrimPrefix(arg, "--repo=")
		} else if strings.HasPrefix(arg, "--branch=") {
			branch = strings.TrimPrefix(arg, "--branch=")
		}
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// If creating a new project
	if create {
		if projectID == "" {
			projectID = "1"
		}
		if repo == "" {
			fmt.Fprintln(os.Stderr, "Usage: machinator project --create --repo=URL [--project=N] [--branch=main]")
			os.Exit(1)
		}

		projCfg := &project.Config{
			Repo:             repo,
			Branch:           branch,
			SimpleModelName:  "gemini-3-flash-preview",
			ComplexModelName: "gemini-3-pro-preview",
		}

		if err := project.Save(cfg.MachinatorDir, projectID, projCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving project: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Created project %s\n", projectID)
		return
	}

	// Edit config in $EDITOR
	if edit {
		if projectID == "" {
			projectID = "1"
		}

		configPath, err := project.EnsureTemplate(cfg.MachinatorDir, projectID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim"
		}

		cmd := exec.Command(editor, configPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running editor: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// List or show projects
	if projectID == "" {
		// List all projects
		projectsDir := filepath.Join(cfg.MachinatorDir, "projects")
		entries, err := os.ReadDir(projectsDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading projects: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Projects:")
		for _, e := range entries {
			if e.IsDir() {
				projCfg, err := project.Load(cfg.MachinatorDir, e.Name())
				if err != nil {
					fmt.Printf("  %s: (error: %v)\n", e.Name(), err)
				} else {
					fmt.Printf("  %s: %s @ %s\n", e.Name(), projCfg.Repo, projCfg.Branch)
				}
			}
		}
	} else {
		// Show specific project
		projCfg, err := project.Load(cfg.MachinatorDir, projectID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Project %s:\n", projectID)
		fmt.Printf("  Repo:          %s\n", projCfg.Repo)
		fmt.Printf("  Branch:        %s\n", projCfg.Branch)
		fmt.Printf("  Simple model:  %s\n", projCfg.SimpleModelName)
		fmt.Printf("  Complex model: %s\n", projCfg.ComplexModelName)
	}
}

func selectTaskCmd() {
	// Parse flags
	noQuotaCheck := false
	projectID := ""
	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--no-quota-check" {
			noQuotaCheck = true
		} else if strings.HasPrefix(arg, "--project=") {
			projectID = strings.TrimPrefix(arg, "--project=")
		}
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Resolve project
	repoDir, err := resolveProjectRepo(cfg.MachinatorDir, projectID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Load quota (or fake it)
	q := quota.New(cfg.MachinatorDir)
	if noQuotaCheck {
		// Fake full quota for both models
		q.Accounts = []quota.AccountQuota{
			{
				Name: "fake",
				Models: map[string]float64{
					"gemini-3-flash-preview": 1.0,
					"gemini-3-pro-preview":   1.0,
				},
			},
		}
		fmt.Println("(Skipping quota check, assuming full quota)")
	} else {
		if err := q.Refresh(); err != nil {
			fmt.Fprintf(os.Stderr, "Error refreshing quota: %v\n", err)
			os.Exit(1)
		}
	}

	// Load tasks
	tasks, err := beads.LoadTasks(repoDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading tasks: %v\n", err)
		os.Exit(1)
	}

	ready := beads.ReadyTasks(tasks)

	fmt.Printf("Total tasks: %d\n", len(tasks))
	fmt.Printf("Ready tasks: %d\n", len(ready))

	if len(ready) == 0 {
		fmt.Println("No ready tasks")
		return
	}

	// Show quota
	fmt.Println("\nQuota:")
	for _, acc := range q.Accounts {
		for model, remaining := range acc.Models {
			fmt.Printf("  %s (%s): %.0f%%\n", model, acc.Name, remaining*100)
		}
	}

	// Show ready tasks with weights
	fmt.Println("\nReady tasks with weights:")
	simpleQuota := q.TotalFor("gemini-3-flash-preview")
	complexQuota := q.TotalFor("gemini-3-pro-preview")

	for _, task := range ready {
		var weight float64
		var model string
		if task.IsComplex {
			model = "complex"
			if complexQuota > 0 {
				weight = 5.0
			}
		} else {
			model = "simple"
			if simpleQuota > 0 {
				weight = 1.0
			} else if complexQuota > 0 {
				weight = 1.0 // Upgrade to complex
				model = "simple→complex"
			}
		}
		fmt.Printf("  %s (%s) weight=%.1f\n", task.ID, model, weight)
	}
}

func runCmd() {
	// Parse flags
	projectID := ""
	headless := false
	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "--project=") {
			projectID = strings.TrimPrefix(arg, "--project=")
		} else if arg == "--headless" {
			headless = true
		}
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Resolve project
	if projectID == "" {
		projectID = "1" // Default to project 1
	}
	projCfg, err := project.Load(cfg.MachinatorDir, projectID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading project: %v\n", err)
		os.Exit(1)
	}
	repoDir := project.RepoDir(cfg.MachinatorDir, projectID)

	st, err := state.Load(cfg.MachinatorDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading state: %v\n", err)
		os.Exit(1)
	}

	q := quota.New(cfg.MachinatorDir)

	// Ensure we have at least one agent
	if len(st.Agents) == 0 {
		for i := 0; i < cfg.DefaultAgentCount; i++ {
			st.AddAgent()
		}
		st.Save()
	}

	// Create file logger (always writes to files)
	logsDir := filepath.Join(cfg.MachinatorDir, "logs")
	logger, err := tui.NewFileLogger(logsDir, headless)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	// Start watchers (quota will be fetched in background)
	go quotaWatcher(q, cfg, logger)
	go setupWatcher(st, cfg, projCfg, projectID, logger)
	go assigner(st, q, cfg, projCfg, repoDir, logger)

	if headless {
		// Headless mode: wait for signal
		logger.Log("main", "Running in headless mode (Ctrl+C to stop)")
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		logger.Log("main", "Shutting down...")
	} else {
		// TUI mode
		projectConfigPath := project.ConfigPath(cfg.MachinatorDir, projectID)
		ui := tui.New(st, q, repoDir, cfg, projCfg, projectConfigPath)
		go func() {
			// Feed file logger to TUI
			// TUI will display its own view
		}()
		if err := ui.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		}
	}

	st.Save()
}

func quotaWatcher(q *quota.Quota, cfg *config.Config, logger tui.Logger) {
	for {
		if err := q.Refresh(); err != nil {
			logger.Log("quota", fmt.Sprintf("Refresh error: %v", err))
		} else {
			logger.Log("quota", fmt.Sprintf("Refreshed: %d accounts", len(q.Accounts)))
		}
		time.Sleep(cfg.Intervals.QuotaRefresh.Duration())
	}
}

func setupWatcher(st *state.State, cfg *config.Config, projCfg *project.Config, projectID string, logger tui.Logger) {
	s := setup.New(cfg.MachinatorDir)

	for {
		// Find pending agents
		for _, agent := range st.PendingAgents() {
			logger.Log("setup", fmt.Sprintf("Setting up agent %d...", agent.ID))

			// Check if repo exists
			repoDir := project.RepoDir(cfg.MachinatorDir, projectID)
			if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {
				// Clone repo first
				logger.Log("setup", fmt.Sprintf("Cloning repo for project %s...", projectID))
				id, _ := strconv.Atoi(projectID)
				_, err := s.CloneRepo(id, projCfg.Repo, projCfg.Branch)
				if err != nil {
					logger.Log("setup", fmt.Sprintf("[red]Clone failed: %v[-]", err))
					time.Sleep(10 * time.Second)
					continue
				}
			}

			// Create worktree for agent
			id, _ := strconv.Atoi(projectID)
			agentDir, err := s.CreateWorktree(id, agent.ID, projCfg.Branch)
			if err != nil {
				logger.Log("setup", fmt.Sprintf("[red]Worktree failed: %v[-]", err))
				time.Sleep(10 * time.Second)
				continue
			}

			logger.Log("setup", fmt.Sprintf("Worktree created: %s", agentDir))

			// Mark as ready
			st.SetAgentReady(agent.ID)
			logger.Log("setup", fmt.Sprintf("[green]Agent %d ready[-]", agent.ID))
		}

		time.Sleep(2 * time.Second)
	}
}

func assigner(st *state.State, q *quota.Quota, cfg *config.Config, projCfg *project.Config, repoDir string, logger tui.Logger) {
	for {
		if st.AssignmentPaused {
			time.Sleep(cfg.Intervals.Assigner.Duration())
			continue
		}

		readyAgents := st.ReadyAgents()
		if len(readyAgents) == 0 {
			time.Sleep(cfg.Intervals.Assigner.Duration())
			continue
		}

		// Load tasks
		tasks, err := beads.LoadTasks(repoDir)
		if err != nil {
			logger.Log("assign", fmt.Sprintf("Error loading tasks: %v", err))
			time.Sleep(cfg.Intervals.Assigner.Duration())
			continue
		}

		readyTasks := beads.ReadyTasks(tasks)
		if len(readyTasks) == 0 {
			time.Sleep(cfg.Intervals.Assigner.Duration())
			continue
		}

		// Get quota info for model selection
		simpleQuota := q.TotalFor(projCfg.SimpleModelName)
		complexQuota := q.TotalFor(projCfg.ComplexModelName)

		for _, agent := range readyAgents {
			// Find a task to assign (weighted selection)
			task := selectTask(readyTasks, simpleQuota, complexQuota, st)
			if task == nil {
				break
			}

			// Determine model
			model := projCfg.SimpleModelName
			if task.IsComplex {
				model = projCfg.ComplexModelName
			} else if simpleQuota <= 0 && complexQuota > 0 {
				model = projCfg.ComplexModelName // Upgrade
			}

			logger.Log("assign", fmt.Sprintf("[green]Agent %d: ASSIGNED[-] %s (%s) → %s",
				agent.ID, task.ID, task.Title, model))

			// Update agent state (auto-saves)
			st.AssignTask(agent.ID, task.ID)

			// Remove task from ready list (for this iteration)
			readyTasks = removeTask(readyTasks, task.ID)
		}

		time.Sleep(cfg.Intervals.Assigner.Duration())
	}
}

func selectTask(tasks []*beads.Task, simpleQuota, complexQuota float64, st *state.State) *beads.Task {
	for _, task := range tasks {
		// Skip barred tasks
		if st.IsTaskBarred(task.ID) {
			continue
		}

		// Skip tasks already assigned to another agent
		if st.IsTaskAssigned(task.ID) {
			continue
		}

		// Check quota
		if task.IsComplex && complexQuota <= 0 {
			continue
		}
		if !task.IsComplex && simpleQuota <= 0 && complexQuota <= 0 {
			continue
		}

		// For now, just return the first available task
		// TODO: weighted random selection
		return task
	}
	return nil
}

func removeTask(tasks []*beads.Task, id string) []*beads.Task {
	var result []*beads.Task
	for _, t := range tasks {
		if t.ID != id {
			result = append(result, t)
		}
	}
	return result
}

func resolveProjectRepo(machinatorDir, projectID string) (string, error) {
	projectsDir := filepath.Join(machinatorDir, "projects")

	// List available projects
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no projects directory at %s", projectsDir)
		}
		return "", fmt.Errorf("read projects: %w", err)
	}

	var projects []string
	for _, e := range entries {
		if e.IsDir() {
			projects = append(projects, e.Name())
		}
	}

	if len(projects) == 0 {
		return "", fmt.Errorf("no projects found in %s", projectsDir)
	}

	// If project specified, use it
	if projectID != "" {
		repoDir := filepath.Join(projectsDir, projectID, "repo")
		if _, err := os.Stat(repoDir); err != nil {
			return "", fmt.Errorf("project %s not found or has no repo", projectID)
		}
		return repoDir, nil
	}

	// Auto-select if only one project
	if len(projects) == 1 {
		fmt.Printf("(Using project %s)\n", projects[0])
		return filepath.Join(projectsDir, projects[0], "repo"), nil
	}

	// Ask user to choose
	fmt.Println("Available projects:")
	for _, p := range projects {
		fmt.Printf("  %s\n", p)
	}
	return "", fmt.Errorf("multiple projects found, use --project=<id> to specify")
}
