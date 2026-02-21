package executor

import "time"

// ExecutionConfig defines the configuration for task execution.
type ExecutionConfig struct {
	GeminiPath     string
	HomeDir        string
	WorktreeDir    string
	RepoDir        string
	Model          string
	TaskID         string
	AgentID        int
	IdleTimeout    time.Duration
	MaxRuntime     time.Duration
	SandboxEnabled bool
}

// ExecutionResult captures the outcome of an execution.
type ExecutionResult struct {
	ExitCode       int
	Duration       time.Duration
	CommitsCreated int
	Error          error
}

// Logger defines the interface for logging execution progress.
type Logger interface {
	Log(source, message string)
}
