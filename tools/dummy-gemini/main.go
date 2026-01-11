package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	// Define flags
	_ = flag.String("prompt", "", "Prompt for the LLM")
	_ = flag.String("repo", "", "Repository path")
	_ = flag.Int("tokens", 0, "Token limit")
	_ = flag.String("model", "", "Model name")
	_ = flag.Float64("temperature", 0.0, "Sampling temperature")
	isJSON := flag.Bool("json", false, "Output in JSON format")
	_ = flag.Bool("yolo", false, "YOLO mode")
	outputFormat := flag.String("output-format", "", "Output format")

	flag.Parse()

	// If output-format is stream-json, treat as JSON
	if *outputFormat == "stream-json" {
		*isJSON = true
	}

	mode := os.Getenv("GEMINI_MODE")

	switch mode {
	case "ERROR":
		fmt.Fprintln(os.Stderr, "Quota exceeded")
		os.Exit(1)
	case "STUCK":
		time.Sleep(30 * time.Second)
		printHappy(*isJSON)
	case "AUTO_CLOSE":
		printAutoClose(*isJSON, flag.Arg(0))
	case "HAPPY":
		fallthrough
	default:
		printHappy(*isJSON)
	}
}

func printAutoClose(isJSON bool, directive string) {
	// Find TaskID in directive. It's usually after "execute Beads Task: " or "Task "
	taskID := "unknown"
	if idx := strings.Index(directive, "Beads Task: "); idx != -1 {
		rest := directive[idx+len("Beads Task: "):]
		taskID = extractFirstWord(rest)
	} else if idx := strings.Index(directive, "Task "); idx != -1 {
		rest := directive[idx+len("Task "):]
		taskID = extractFirstWord(rest)
	}
	// Clean up taskID (strip trailing punctuation and whitespace)
	taskID = strings.TrimFunc(taskID, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-')
	})

	if !isJSON {
		fmt.Printf("Simulating closing task %s\n", taskID)
		return
	}

	// Output stream-json events
	fmt.Printf(`{"type": "init", "model": "dummy-model"}` + "\n")
	fmt.Printf(`{"type": "message", "role": "assistant", "content": "I will now close task %s."}`+"\n", taskID)
	fmt.Printf(`{"type": "tool_use", "tool_name": "run_shell_command", "tool_id": "t1", "args": {"command": "bd close %s"}}`+"\n", taskID)

	// Actually execute the command so the state changes in the fixture repo
	// We need to make sure we don't have weird characters in taskID
	cmd := exec.Command("bd", "close", taskID)
	_ = cmd.Run()

	fmt.Printf(`{"type": "tool_result", "tool_id": "t1", "status": "success", "output": "Closed %s"}`+"\n", taskID)
	fmt.Printf(`{"type": "result", "status": "success"}` + "\n")
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

func printHappy(isJSON bool) {
	if isJSON {
		fmt.Println(`{"response": "mock response"}`)
	} else {
		fmt.Println("mock response")
	}
}
