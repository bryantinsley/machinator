package executor

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"testing"
	"time"
)

type mockLogger struct {
	logs []string
}

func (l *mockLogger) Log(source, message string) {
	l.logs = append(l.logs, fmt.Sprintf("[%s] %s", source, message))
}

func TestMonitor_NormalExit(t *testing.T) {
	cmd := exec.Command("sleep", "0.1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start cmd: %v", err)
	}

	events := make(chan ACPEvent)
	done := make(chan error)
	logger := &mockLogger{}
	cfg := ExecutionConfig{
		IdleTimeout: 1 * time.Second,
		MaxRuntime:  5 * time.Second,
	}

	go func() {
		done <- cmd.Wait()
		close(events)
	}()

	result := Monitor(cmd, events, done, cfg, logger)

	if result.Error != nil {
		t.Errorf("expected no error, got %v", result.Error)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestMonitor_IdleTimeout(t *testing.T) {
	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start cmd: %v", err)
	}

	events := make(chan ACPEvent)
	done := make(chan error)
	logger := &mockLogger{}
	cfg := ExecutionConfig{
		IdleTimeout: 100 * time.Millisecond,
		MaxRuntime:  5 * time.Second,
	}

	// We don't send anything to events, so it should idle out.

	result := Monitor(cmd, events, done, cfg, logger)

	if result.Error == nil || result.Error.Error() != "idle timeout" {
		t.Errorf("expected idle timeout error, got %v", result.Error)
	}
}

func TestMonitor_MaxRuntime(t *testing.T) {
	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start cmd: %v", err)
	}

	events := make(chan ACPEvent)
	done := make(chan error)
	logger := &mockLogger{}
	cfg := ExecutionConfig{
		IdleTimeout: 5 * time.Second,
		MaxRuntime:  200 * time.Millisecond,
	}

	result := Monitor(cmd, events, done, cfg, logger)

	if result.Error == nil || result.Error.Error() != "max runtime exceeded" {
		t.Errorf("expected max runtime exceeded error, got %v", result.Error)
	}
}

func TestMonitor_FatalParseErrors(t *testing.T) {
	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start cmd: %v", err)
	}

	events := make(chan ACPEvent)
	done := make(chan error)
	logger := &mockLogger{}
	cfg := ExecutionConfig{
		IdleTimeout: 5 * time.Second,
		MaxRuntime:  5 * time.Second,
	}

	rejectionString := "Command rejected because it could not be parsed safely"
	textEvent := TextEvent{Content: rejectionString}
	textJSON, _ := json.Marshal(textEvent)
	
	event := ACPEvent{
		Type:    "text",
		RawJSON: textJSON,
	}

	go func() {
		// Send 3 parse error events
		events <- event
		events <- event
		events <- event
	}()

	result := Monitor(cmd, events, done, cfg, logger)

	if result.Error == nil || result.Error.Error() != "fatal parse errors" {
		t.Errorf("expected fatal parse errors, got %v", result.Error)
	}
}
