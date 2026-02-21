package executor

import (
	"bytes"
	"testing"
	"time"
)

func TestParseACPStream(t *testing.T) {
	input := `{"type": "text", "timestamp": "2023-01-01T00:00:00Z"}
{"type": "run_shell_command", "timestamp": "2023-01-01T00:00:01Z"}
invalid json
{"type": "tool_result", "timestamp": "2023-01-01T00:00:02Z"}
`
	reader := bytes.NewReader([]byte(input))
	events := make(chan ACPEvent, 10)
	done := make(chan error, 1)

	go ParseACPStream(reader, events, done)

	var received []ACPEvent
Loop:
	for {
		select {
		case event := <-events:
			received = append(received, event)
		case err := <-done:
			if err != nil {
				t.Fatalf("ParseACPStream failed with error: %v", err)
			}
			// Drain any remaining events in the buffered channel
			for {
				select {
				case event := <-events:
					received = append(received, event)
				default:
					break Loop
				}
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for ParseACPStream to complete")
		}
	}

	if len(received) != 3 {
		t.Errorf("Expected 3 events, got %d", len(received))
	}

	expectedTypes := []string{"text", "run_shell_command", "tool_result"}
	for i, event := range received {
		if event.Type != expectedTypes[i] {
			t.Errorf("Event %d: expected type %s, got %s", i, expectedTypes[i], event.Type)
		}
		if len(event.RawJSON) == 0 {
			t.Errorf("Event %d: RawJSON is empty", i)
		}
	}
}
