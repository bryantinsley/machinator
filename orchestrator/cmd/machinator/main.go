package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bryantinsley/machinator/orchestrator/pkg/orchestrator"
	"github.com/bryantinsley/machinator/orchestrator/pkg/setup"
)

func main() {
	debug := flag.Bool("debug", false, "Run in debug mode")
	once := flag.Bool("once", false, "Execute one task and exit")
	headless := flag.Bool("headless", false, "Run without TUI")
	forceSetup := flag.Bool("setup", false, "Force launch setup wizard")
	flag.Parse()

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	// Startup logic:
	// 1. If --setup flag is provided, launch setup.
	// 2. If .beads does not exist in CWD, launch setup.
	// 3. Otherwise, launch orchestrator.

	beadsDir := filepath.Join(cwd, ".beads")
	_, beadsErr := os.Stat(beadsDir)

	if *forceSetup || os.IsNotExist(beadsErr) {
		fmt.Println("No .beads directory found or setup forced. Launching setup wizard...")
		config, err := setup.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Setup error: %v\n", err)
			os.Exit(1)
		}

		if config == nil {
			fmt.Println("Setup cancelled or no project selected.")
			return
		}

		// After setup, if a project was selected, we could potentially
		// change directory to that project and run orchestrator.
		// For now, we'll just exit as the task doesn't specify auto-launch after setup.
		fmt.Printf("Project %s ready. Run machinator inside the project directory to start orchestration.\n", config.Name)
		return
	}

	// Ensure Machinator tools are in PATH
	machinatorDir := setup.GetMachinatorDir()
	os.Setenv("PATH", machinatorDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Launch orchestrator
	if err := orchestrator.Run(*debug, *once, *headless); err != nil {
		fmt.Fprintf(os.Stderr, "Orchestrator error: %v\n", err)
		os.Exit(1)
	}
}
