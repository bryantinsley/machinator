package executor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	time "time"
)

// ACPEvent represents a generic event from the Gemini stream.
type ACPEvent struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	RawJSON   json.RawMessage `json:"-"`
}

// ToolCallEvent represents a tool execution request.
type ToolCallEvent struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// ToolResultEvent represents the result of a tool execution.
type ToolResultEvent struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
}

// TextEvent represents a text message from the model.
type TextEvent struct {
	Content string `json:"content"`
}

// ParseACPStream reads line-by-line from the reader and sends parsed events to the events channel.
// This function is intended to run in a goroutine.
func ParseACPStream(reader io.Reader, events chan<- ACPEvent, done chan<- error) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event ACPEvent
		if err := json.Unmarshal(line, &event); err != nil {
			fmt.Printf("failed to parse ACP event line: %v\n", err)
			continue
		}

		// Store the raw JSON for further processing if needed
		event.RawJSON = json.RawMessage(line)

		// Ensure timestamp is set if it was missing or zero
		if event.Timestamp.IsZero() {
			event.Timestamp = time.Now()
		}

		events <- event
	}

	if err := scanner.Err(); err != nil {
		done <- err
	} else {
		done <- nil
	}
}
