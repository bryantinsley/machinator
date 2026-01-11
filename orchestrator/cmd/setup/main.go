package main

import (
	"fmt"
	"os"

	"github.com/bryantinsley/machinator/orchestrator/pkg/setup"
)

func main() {
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
