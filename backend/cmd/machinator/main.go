package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/bryantinsley/machinator/backend/internal/beads"
	"github.com/bryantinsley/machinator/backend/internal/config"
	"github.com/bryantinsley/machinator/backend/internal/quota"
	"github.com/bryantinsley/machinator/backend/internal/state"
)

func usage() {
	fmt.Println(`machinator - Autonomous Agent Orchestration System

Usage:
  machinator <command> [options]

Commands:
  run            Run the orchestrator
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
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	st, err := state.Load(cfg.MachinatorDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading state: %v\n", err)
		os.Exit(1)
	}

	q := quota.New(cfg.MachinatorDir)

	// Ensure we have at least one agent
	if len(st.Agents) == 0 {
		for i := 0; i < cfg.Agents.Count; i++ {
			st.AddAgent()
		}
		st.Save()
	}

	log.Println("Machinator v3 starting...")
	log.Printf("  MACHINATOR_DIR: %s", cfg.MachinatorDir)
	log.Printf("  Agents: %d", len(st.Agents))
	log.Printf("  Idle timeout: %s", cfg.Timeouts.Idle)
	log.Printf("  Max runtime: %s", cfg.Timeouts.MaxRuntime)

	// Start watchers
	go quotaWatcher(q, cfg)
	go assigner(st, q, cfg)
	// TODO: go agentWatcher(st, cfg)

	log.Println("Orchestrator running. Press Ctrl+C to stop.")

	// Wait for signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down...")
	st.Save()
}

func quotaWatcher(q *quota.Quota, cfg *config.Config) {
	for {
		if err := q.Refresh(); err != nil {
			log.Printf("Quota refresh error: %v", err)
		} else {
			log.Printf("Quota refreshed: %d accounts", len(q.Accounts))
		}
		time.Sleep(cfg.Intervals.QuotaRefresh)
	}
}

func assigner(st *state.State, q *quota.Quota, cfg *config.Config) {
	for {
		if st.AssignmentPaused {
			time.Sleep(cfg.Intervals.Assigner)
			continue
		}

		for _, agent := range st.ReadyAgents() {
			log.Printf("Agent %d: ready, looking for work...", agent.ID)

			// TODO: Find task and assign
			// For now, just log
		}

		time.Sleep(cfg.Intervals.Assigner)
	}
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
