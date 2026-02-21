package executor

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// LaunchGemini starts the Gemini CLI process as a subprocess.
func LaunchGemini(cfg ExecutionConfig, directivePath string) (*exec.Cmd, io.ReadCloser, error) {
	// Read the directive file content to pass via stdin.
	content, err := os.ReadFile(directivePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read directive file: %w", err)
	}

	// Prepare command arguments.
	args := []string{
		"--yolo",
		"--sandbox",
		"--model", cfg.Model,
		"--output-format", "stream-json",
	}

	// Create the command.
	cmd := exec.Command(cfg.GeminiPath, args...)

	// Set environment variables.
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("HOME=%s", cfg.HomeDir),
		fmt.Sprintf("GEMINI_CLI_HOME=%s", cfg.HomeDir),
		"GEMINI_FORCE_FILE_STORAGE=true",
	)

	// Set command working directory and stdin.
	cmd.Dir = cfg.WorktreeDir
	cmd.Stdin = bytes.NewReader(content)

	// Create a pipe for stdout.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the command.
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start Gemini process: %w", err)
	}

	return cmd, stdout, nil
}
