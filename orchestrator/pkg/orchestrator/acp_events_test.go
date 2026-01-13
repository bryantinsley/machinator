package orchestrator

import (
	"testing"
)

func TestParseACPEvent(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected ACPEventType
	}{
		{
			name:     "init event",
			line:     `{"type":"init","model":"gemini-pro"}`,
			expected: ACPEventInit,
		},
		{
			name:     "message event",
			line:     `{"type":"message","role":"assistant","content":"hello"}`,
			expected: ACPEventMessage,
		},
		{
			name:     "tool_use event",
			line:     `{"type":"tool_use","tool_name":"read_file","parameters":{"file_path":"test.txt"}}`,
			expected: ACPEventToolUse,
		},
		{
			name:     "tool_result event",
			line:     `{"type":"tool_result","tool_id":"123","output":"success"}`,
			expected: ACPEventToolResult,
		},
		{
			name:     "result event",
			line:     `{"type":"result","status":"success"}`,
			expected: ACPEventResult,
		},
		{
			name:     "error event",
			line:     `{"type":"error","content":"something went wrong"}`,
			expected: ACPEventError,
		},
		{
			name:     "system message",
			line:     `Loaded cached credentials.`,
			expected: ACPEventMessage,
		},
		{
			name:     "empty line",
			line:     ``,
			expected: ACPEventUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := ParseACPEvent(tt.line)
			if event.Type != tt.expected {
				t.Errorf("expected type %v, got %v", tt.expected, event.Type)
			}
		})
	}
}

func TestACPEvent_Summary(t *testing.T) {
	tests := []struct {
		name     string
		event    ACPEvent
		maxLen   int
		expected string
	}{
		{
			name: "init summary",
			event: ACPEvent{
				Type:  ACPEventInit,
				Model: "gemini-pro",
			},
			maxLen:   50,
			expected: "Session started (model: gemini-pro)",
		},
		{
			name: "message summary",
			event: ACPEvent{
				Type:    ACPEventMessage,
				Content: "This is a long message that should be truncated",
			},
			maxLen:   20,
			expected: "This is a long me...",
		},
		{
			name: "run_shell_command summary",
			event: ACPEvent{
				Type:     ACPEventToolUse,
				ToolName: "run_shell_command",
				ToolArgs: map[string]interface{}{"command": "ls -la"},
			},
			maxLen:   50,
			expected: "🖥️  ls -la",
		},
		{
			name: "read_file summary",
			event: ACPEvent{
				Type:     ACPEventToolUse,
				ToolName: "read_file",
				ToolArgs: map[string]interface{}{"file_path": "/path/to/test.txt"},
			},
			maxLen:   50,
			expected: "📖 read: test.txt",
		},
		{
			name: "tool_result success",
			event: ACPEvent{
				Type:    ACPEventToolResult,
				Status:  "success",
				Content: "OK",
			},
			maxLen:   50,
			expected: "✅ OK",
		},
		{
			name: "tool_result failure",
			event: ACPEvent{
				Type:    ACPEventToolResult,
				Status:  "failure",
				Content: "Error: file not found",
			},
			maxLen:   50,
			expected: "❌ Error: file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.event.Summary(tt.maxLen)
			if got != tt.expected {
				t.Errorf("expected summary %q, got %q", tt.expected, got)
			}
		})
	}
}
