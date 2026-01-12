package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bryantinsley/machinator/orchestrator/pkg/setup"
)

func main() {
	headless := flag.Bool("headless", false, "Run in headless mode and dump initial view")
	flag.Parse()

	if *headless {
		view, err := setup.RenderInitialView()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(view)
		return
	}

	config, err := setup.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if config != nil {
		fmt.Printf("Selected project: %s (ID: %d)\n", config.Name, config.ID)
		fmt.Printf("Repo: %s\n", config.RepoURL)
		fmt.Printf("Agents: %d\n", config.AgentCount)
	} else {
		fmt.Println("No project selected.")
	}
}
