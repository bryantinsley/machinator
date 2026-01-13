// dummy-gemini is a mock implementation of the Gemini CLI for testing purposes.
// It simulates different behaviors (happy path, errors, hanging) based on environment variables.
//
// Supported modes (via DUMMY_GEMINI_MODE env var):
// - happy:      Returns a success sequence after a short delay (default).
// - error:      Returns a quota exceeded error.
// - stuck:      Hangs indefinitely (unless TEST_STUCK_TIMEOUT is set).
// - scripted:   Plays back events from the file specified in DUMMY_GEMINI_SCRIPT.
// - auto_close: Simulates an agent closing a task (used for testing automation).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ACPEvent represents a simplified version of the event structure
type ACPEvent struct {
	Type     string                 `json:"type"`
	Role     string                 `json:"role,omitempty"`
	Content  string                 `json:"content,omitempty"`
	ToolName string                 `json:"tool_name,omitempty"`
	ToolID   string                 `json:"tool_id,omitempty"`
	ToolArgs map[string]interface{} `json:"args,omitempty"`
	Status   string                 `json:"status,omitempty"`
	Model    string                 `json:"model,omitempty"`
}

func main() {
	os.Exit(run(os.Stdout, os.Getenv, os.Args[1:]))
}

func run(w io.Writer, getEnv func(string) string, args []string) int {
	// Define flags
	fs := flag.NewFlagSet("dummy-gemini", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Silence flag errors in run, let caller handle or ignore

	_ = fs.String("prompt", "", "Prompt for the LLM")
	_ = fs.String("repo", "", "Repository path")
	_ = fs.Int("tokens", 0, "Token limit")
	_ = fs.String("model", "", "Model name")
	_ = fs.Float64("temperature", 0.0, "Sampling temperature")
	_ = fs.Bool("json", false, "Output in JSON format (ignored, we use stream-json if requested or default for ACP)")
	_ = fs.Bool("yolo", false, "YOLO mode")
	_ = fs.Bool("sandbox", false, "Sandbox mode")
	_ = fs.String("output-format", "", "Output format")
	version := fs.Bool("version", false, "Show version")
	dumpQuota := fs.Bool("dump-quota", false, "Dump quota information")

	// Flags for compatibility but ignored
	_ = fs.String("system", "", "System prompt")

	if err := fs.Parse(args); err != nil {
		// Ignore flag errors for now as we might receive unknown flags
	}

	if *version {
		fmt.Fprintln(w, "dummy-gemini v0.2.0")
		return 0
	}

	if *dumpQuota {
		fmt.Fprintln(w, `{"buckets": [{"modelId": "gemini-3-flash-preview", "remainingFraction": 1.0, "resetTime": "2099-01-01T00:00:00Z"}]}`)
		return 0
	}

	mode := strings.ToLower(getEnv("DUMMY_GEMINI_MODE"))
	if mode == "" {
		mode = strings.ToLower(getEnv("GEMINI_MODE"))
	}
	if mode == "" {
		mode = "happy"
	}

	switch mode {
	case "error":
		return printError(w)
	case "stuck":
		// Hang indefinitely
		if getEnv("TEST_STUCK_TIMEOUT") != "" {
			return 0
		}
		select {}
	case "scripted":
		return printScripted(w, getEnv)
	case "happy":
		return printHappy(w)
	default:
		// Fallback for any other modes that might have been there
		if mode == "auto_close" {
			return printAutoClose(w, fs.Arg(0))
		} else {
			return printHappy(w)
		}
	}
}

func emit(w io.Writer, event ACPEvent) {
	b, _ := json.Marshal(event)
	fmt.Fprintln(w, string(b))
}

func printHappy(w io.Writer) int {
	emit(w, ACPEvent{Type: "init", Model: "dummy-model-3.5"})
	time.Sleep(1 * time.Second)
	emit(w, ACPEvent{
		Type:    "message",
		Role:    "assistant",
		Content: "This is a happy mock response from dummy-gemini.",
	})
	time.Sleep(1 * time.Second)
	emit(w, ACPEvent{Type: "result", Status: "success"})
	return 0
}

func printError(w io.Writer) int {
	emit(w, ACPEvent{
		Type:    "error",
		Status:  "failure",
		Content: "Quota exceeded: You have reached your limit.",
	})
	return 1
}

func printScripted(w io.Writer, getEnv func(string) string) int {
	scriptPath := getEnv("DUMMY_GEMINI_SCRIPT")
	if scriptPath == "" {
		fmt.Fprintln(os.Stderr, "Error: DUMMY_GEMINI_SCRIPT env var not set")
		return 1
	}

	content, err := os.ReadFile(scriptPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading script file: %v\n", err)
		return 1
	}

	// We assume the script file already contains the JSON events line by line
	fmt.Fprint(w, string(content))
	return 0
}

func printAutoClose(w io.Writer, directive string) int {
	fmt.Fprintf(os.Stderr, "DEBUG: printAutoClose called with directive length %d\n", len(directive))
	fmt.Fprintf(os.Stderr, "DEBUG: directive prefix: %s\n", directive[:min(len(directive), 50)])

	// Find TaskID in directive. It's usually after "Beads Task: " or "Task "
	taskID := "unknown"
	if idx := strings.Index(directive, "Beads Task: "); idx != -1 {
		rest := directive[idx+len("Beads Task: "):]
		taskID = extractFirstWord(rest)
	} else if idx := strings.Index(directive, "Task "); idx != -1 {
		rest := directive[idx+len("Task "):]
		taskID = extractFirstWord(rest)
	}
	fmt.Fprintf(os.Stderr, "DEBUG: extracted taskID: %s\n", taskID)

	// Clean up taskID
	taskID = strings.TrimFunc(taskID, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-')
	})

	emit(w, ACPEvent{Type: "init", Model: "dummy-model-3.5"})
	emit(w, ACPEvent{
		Type:    "message",
		Role:    "assistant",
		Content: fmt.Sprintf("I will now close task %s.", taskID),
	})
	emit(w, ACPEvent{
		Type:     "tool_use",
		ToolName: "run_shell_command",
		ToolID:   "t1",
		ToolArgs: map[string]interface{}{"command": fmt.Sprintf("bd close %s", taskID)},
	})

	// Actually execute the command
	cmd := exec.Command("bd", "close", taskID)
	// Inherit environment to ensure PATH is correct
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error executing bd close: %v\n", err)
	}

	emit(w, ACPEvent{
		Type:    "tool_result",
		ToolID:  "t1",
		Status:  "success",
		Content: fmt.Sprintf("Closed %s", taskID),
	})
	emit(w, ACPEvent{Type: "result", Status: "success"})
	return 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func extractFirstWord(s string) string {
	s = strings.TrimSpace(s)
	idx := strings.IndexFunc(s, func(r rune) bool {
		return r == ' ' || r == '\n' || r == '\r' || r == '\t'
	})
	if idx == -1 {
		return s
	}
	return s[:idx]
}
