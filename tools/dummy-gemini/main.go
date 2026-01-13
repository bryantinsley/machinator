package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
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
	// Define flags
	_ = flag.String("prompt", "", "Prompt for the LLM")
	_ = flag.String("repo", "", "Repository path")
	_ = flag.Int("tokens", 0, "Token limit")
	_ = flag.String("model", "", "Model name")
	_ = flag.Float64("temperature", 0.0, "Sampling temperature")
	_ = flag.Bool("json", false, "Output in JSON format (ignored, we use stream-json if requested or default for ACP)")
	_ = flag.Bool("yolo", false, "YOLO mode")
	_ = flag.String("output-format", "", "Output format")
	version := flag.Bool("version", false, "Show version")
	dumpQuota := flag.Bool("dump-quota", false, "Dump quota information")

	// Flags for compatibility but ignored
	_ = flag.String("system", "", "System prompt")

	flag.Parse()

	if *version {
		fmt.Println("dummy-gemini v0.2.0")
		return
	}

	if *dumpQuota {
		fmt.Println(`{"quota": 100, "remaining": 1000000}`)
		return
	}

	mode := strings.ToLower(os.Getenv("DUMMY_GEMINI_MODE"))
	if mode == "" {
		mode = strings.ToLower(os.Getenv("GEMINI_MODE"))
	}
	if mode == "" {
		mode = "happy"
	}

	// Default to stream-json style for most modes as requested by task ("Outputs ACP-format JSON events")
	// Unless specified otherwise by mode-specific logic.

	switch mode {
	case "error":
		printError()
	case "stuck":
		// Hang indefinitely
		select {}
	case "scripted":
		printScripted()
	case "happy":
		printHappy()
	default:
		// Fallback for any other modes that might have been there
		if mode == "auto_close" {
			printAutoClose(flag.Arg(0))
		} else {
			printHappy()
		}
	}
}

func emit(event ACPEvent) {
	b, _ := json.Marshal(event)
	fmt.Println(string(b))
}

func printHappy() {
	emit(ACPEvent{Type: "init", Model: "dummy-model-3.5"})
	time.Sleep(1 * time.Second)
	emit(ACPEvent{
		Type:    "message",
		Role:    "assistant",
		Content: "This is a happy mock response from dummy-gemini.",
	})
	time.Sleep(1 * time.Second)
	emit(ACPEvent{Type: "result", Status: "success"})
}

func printError() {
	emit(ACPEvent{
		Type:    "error",
		Status:  "failure",
		Content: "Quota exceeded: You have reached your limit.",
	})
	os.Exit(1)
}

func printScripted() {
	scriptPath := os.Getenv("DUMMY_GEMINI_SCRIPT")
	if scriptPath == "" {
		fmt.Fprintln(os.Stderr, "Error: DUMMY_GEMINI_SCRIPT env var not set")
		os.Exit(1)
	}

	content, err := os.ReadFile(scriptPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading script file: %v\n", err)
		os.Exit(1)
	}

	// We assume the script file already contains the JSON events line by line
	fmt.Print(string(content))
}

func printAutoClose(directive string) {
	// Find TaskID in directive. It's usually after "Beads Task: " or "Task "
	taskID := "unknown"
	if idx := strings.Index(directive, "Beads Task: "); idx != -1 {
		rest := directive[idx+len("Beads Task: "):]
		taskID = extractFirstWord(rest)
	} else if idx := strings.Index(directive, "Task "); idx != -1 {
		rest := directive[idx+len("Task "):]
		taskID = extractFirstWord(rest)
	}
	// Clean up taskID
	taskID = strings.TrimFunc(taskID, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-')
	})

	emit(ACPEvent{Type: "init", Model: "dummy-model-3.5"})
	emit(ACPEvent{
		Type:    "message",
		Role:    "assistant",
		Content: fmt.Sprintf("I will now close task %s.", taskID),
	})
	emit(ACPEvent{
		Type:     "tool_use",
		ToolName: "run_shell_command",
		ToolID:   "t1",
		ToolArgs: map[string]interface{}{"command": fmt.Sprintf("./bd close %s", taskID)},
	})

	emit(ACPEvent{
		Type:    "tool_result",
		ToolID:  "t1",
		Status:  "success",
		Content: fmt.Sprintf("Closed %s", taskID),
	})
	emit(ACPEvent{Type: "result", Status: "success"})
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
