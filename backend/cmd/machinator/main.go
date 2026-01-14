package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bryantinsley/machinator/backend/internal/config"
	"github.com/bryantinsley/machinator/backend/internal/quota"
	"github.com/bryantinsley/machinator/backend/internal/state"
)

func usage() {
	fmt.Println(`machinator - Autonomous Agent Orchestration System

Usage:
  machinator <command> [options]

Commands:
  serve          Run the orchestrator
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
		cmdQuota()
	case "select-task":
		cmdSelectTask()
	case "serve":
		cmdServe()
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		usage()
		os.Exit(1)
	}
}

func cmdQuota() {
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

func cmdSelectTask() {
	fmt.Println("TODO: Implement select-task")
}

func cmdServe() {
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
