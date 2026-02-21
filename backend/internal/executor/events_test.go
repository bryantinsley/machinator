package executor

import (
	"strings"
	"testing"
	"time"
)

func TestParseACPStream_EmptyInput(t *testing.T) {
	reader := strings.NewReader("")
	events := make(chan ACPEvent, 10)
	done := make(chan error, 1)

	go ParseACPStream(reader, events, done)

	select {
	case event := <-events:
		t.Errorf("expected no events, got %v", event)
	case err := <-done:
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	case <-time.After(time.Second):
		t.Error("timeout")
	}
}

func TestParseACPStream_SingleToolCall(t *testing.T) {
	input := `{"type": "run_shell_command", "timestamp": "2023-01-01T00:00:00Z"}`
	reader := strings.NewReader(input)
	events := make(chan ACPEvent, 10)
	done := make(chan error, 1)

	go ParseACPStream(reader, events, done)

	select {
	case event := <-events:
		if event.Type != "run_shell_command" {
			t.Errorf("expected type run_shell_command, got %s", event.Type)
		}
	case <-time.After(time.Second):
		t.Error("timeout")
	}

	if err := <-done; err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestParseACPStream_MalformedLine(t *testing.T) {
	input := "invalid json\n{\"type\": \"text\", \"timestamp\": \"2023-01-01T00:00:00Z\"}"
	reader := strings.NewReader(input)
	events := make(chan ACPEvent, 10)
	done := make(chan error, 1)

	go ParseACPStream(reader, events, done)

	select {
	case event := <-events:
		if event.Type != "text" {
			t.Errorf("expected type text, got %s", event.Type)
		}
	case <-time.After(time.Second):
		t.Error("timeout")
	}

	if err := <-done; err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestParseACPStream_MultipleEvents(t *testing.T) {
	input := `{"type": "text"}
{"type": "run_shell_command"}
{"type": "tool_result"}
{"type": "text"}
{"type": "run_shell_command"}`
	reader := strings.NewReader(input)
	events := make(chan ACPEvent, 10)
	done := make(chan error, 1)

	go ParseACPStream(reader, events, done)

	count := 0
	expectedTypes := []string{"text", "run_shell_command", "tool_result", "text", "run_shell_command"}

	for i := 0; i < 5; i++ {
		select {
		case event := <-events:
			if event.Type != expectedTypes[i] {
				t.Errorf("event %d: expected type %s, got %s", i, expectedTypes[i], event.Type)
			}
			count++
		case <-time.After(time.Second):
			t.Errorf("timeout waiting for event %d", i)
		}
	}

	if count != 5 {
		t.Errorf("expected 5 events, got %d", count)
	}

	if err := <-done; err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestParseACPStream_TextEvent(t *testing.T) {
	input := `{"type": "text", "timestamp": "2023-01-01T00:00:00Z", "content": "hello"}`
	reader := strings.NewReader(input)
	events := make(chan ACPEvent, 10)
	done := make(chan error, 1)

	go ParseACPStream(reader, events, done)

	select {
	case event := <-events:
		if event.Type != "text" {
			t.Errorf("expected type text, got %s", event.Type)
		}
		// Since ParseACPStream only populates ACPEvent, we check the RawJSON if we wanted specific details.
		if len(event.RawJSON) == 0 {
			t.Error("expected RawJSON to be populated")
		}
	case <-time.After(time.Second):
		t.Error("timeout")
	}

	if err := <-done; err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}