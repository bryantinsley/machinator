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

	// Ensure Machinator tools are in PATH
	machinatorDir := setup.GetMachinatorDir()
	os.Setenv("PATH", machinatorDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Initial state
	beadsDir := filepath.Join(cwd, ".beads")
	_, beadsErr := os.Stat(beadsDir)

	// Determine initial mode
	runSetup := *forceSetup || os.IsNotExist(beadsErr)

	var config *setup.ProjectConfig

	for {
		if runSetup {
			// Clear screen before launching setup
			fmt.Print("\033[H\033[2J")

			newConfig, err := setup.Run()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Setup error: %v\n", err)
				os.Exit(1)
			}
			if newConfig == nil {
				fmt.Println("Setup cancelled or no project selected.")
				os.Exit(0)
			}
			config = newConfig
			runSetup = false // Switch to orchestrator
		} else {
			// Launch orchestrator
			err := orchestrator.Run(*debug, *once, *headless, config)
			if err == orchestrator.ErrSwitchToSetup {
				runSetup = true
				config = nil // Reset config so setup starts fresh? Or keep it? Setup doesn't take input config.
				continue
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Orchestrator error: %v\n", err)
				os.Exit(1)
			}
			// Normal exit
			break
		}
	}
}
