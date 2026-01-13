package orchestrator

import "os"

func getProjectRoot() string {
	if override := os.Getenv("MACHINATOR_PROJECT_ROOT"); override != "" {
		return override
	}
	// Use the original cwd captured at startup
	return originalCwd
}
