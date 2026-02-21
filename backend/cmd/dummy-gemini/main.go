package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

type Event struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Content   string    `json:"content,omitempty"`
	Name      string    `json:"name,omitempty"`
	Args      any       `json:"args,omitempty"`
	Success   *bool     `json:"success,omitempty"`
	Output    string    `json:"output,omitempty"`
}

func main() {
	yolo := flag.Bool("yolo", false, "Enable YOLO mode")
	sandbox := flag.Bool("sandbox", false, "Enable sandbox mode")
	model := flag.String("model", "", "Model name")
	outputFormat := flag.String("output-format", "json-stream", "Output format")
	flag.Parse()

	// Mimic reading from stdin
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			// Just consume stdin
		}
	}()

	mode := os.Getenv("DUMMY_GEMINI_MODE")
	if mode == "" {
		mode = "happy"
	}

	delay := 1 * time.Second
	if mode == "fast" {
		delay = 0
	}

	switch mode {
	case "happy", "fast":
		emitEvent(Event{Type: "text", Content: "Hello! I am a dummy Gemini CLI. I will help you with your task."}) 
		time.Sleep(delay)
		emitEvent(Event{Type: "run_shell_command", Name: "run_shell_command", Args: map[string]string{"command": "ls -la"}})
		time.Sleep(delay)
		bTrue := true
		emitEvent(Event{Type: "tool_result", Name: "run_shell_command", Success: &bTrue, Output: "total 0\ndrwxr-xr-x  2 user  group  64 Feb 20 12:00 ."})
		time.Sleep(delay)
		emitEvent(Event{Type: "text", Content: "I have listed the files. It seems the directory is empty."}) 
		time.Sleep(delay)
		emitEvent(Event{Type: "text", Content: "Task complete."}) 
		os.Exit(0)

	case "error":
		emitEvent(Event{Type: "text", Content: "I encountered an error while processing your request."}) 
		time.Sleep(delay)
		emitEvent(Event{Type: "text", Content: "Error: Something went wrong."}) 
		os.Exit(1)

	case "stuck":
		emitEvent(Event{Type: "text", Content: "I am starting the task..."})
		select {} // Sleep forever

	default:
		fmt.Fprintf(os.Stderr, "Unknown DUMMY_GEMINI_MODE: %s\n", mode)
		os.Exit(1)
	}

	_ = yolo
	_ = sandbox
	_ = model
	_ = outputFormat
}

func emitEvent(e Event) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	data, _ := json.Marshal(e)
	fmt.Println(string(data))
}
