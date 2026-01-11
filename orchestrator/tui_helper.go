package main

func getProjectRoot() string {
	// Use the original cwd captured at startup
	return originalCwd
}
