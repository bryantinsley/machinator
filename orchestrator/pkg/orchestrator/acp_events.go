package orchestrator

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ACPEventType represents different types of ACP events
type ACPEventType string

const (
	ACPEventInit       ACPEventType = "init"
	ACPEventMessage    ACPEventType = "message"
	ACPEventToolUse    ACPEventType = "tool_use"
	ACPEventToolResult ACPEventType = "tool_result"
	ACPEventResult     ACPEventType = "result"
	ACPEventError      ACPEventType = "error"
	ACPEventUnknown    ACPEventType = "unknown"
)

// ACPEvent represents a parsed event from Gemini stream-json output
type ACPEvent struct {
	Type      ACPEventType
	Role      string // "user" or "assistant" for messages
	Content   string
	ToolName  string
	ToolID    string
	ToolArgs  map[string]interface{}
	Status    string
	SessionID string
	Model     string
	Stats     map[string]interface{}
	Raw       string
	AgentName string // Name of the agent that generated this event
}

// Icon returns an emoji icon for the event type
func (e ACPEvent) Icon() string {
	switch e.Type {
	case ACPEventInit:
		return "🚀"
	case ACPEventMessage:
		if e.Role == "user" {
			return "👤"
		}
		return "🤖"
	case ACPEventToolUse:
		return "🔧"
	case ACPEventToolResult:
		return "📤"
	case ACPEventResult:
		return "✅"
	case ACPEventError:
		return "❌"
	default:
		return "📝"
	}
}

// Summary returns a short summary of the event for display
func (e ACPEvent) Summary(maxLen int) string {
	switch e.Type {
	case ACPEventInit:
		return fmt.Sprintf("Session started (model: %s)", e.Model)
	case ACPEventMessage:
		content := e.Content
		if len(content) > maxLen {
			content = content[:maxLen-3] + "..."
		}
		// Clean up newlines
		content = strings.ReplaceAll(content, "\n", " ")
		content = strings.ReplaceAll(content, "\r", "")
		return content
	case ACPEventToolUse:
		// Show tool-specific details
		switch e.ToolName {
		case "run_shell_command":
			if cmd, ok := e.ToolArgs["command"].(string); ok {
				if len(cmd) > maxLen-20 {
					cmd = cmd[:maxLen-23] + "..."
				}
				return fmt.Sprintf("🖥️  %s", cmd)
			}
		case "read_file":
			if path, ok := e.ToolArgs["file_path"].(string); ok {
				// Show just filename, not full path
				parts := strings.Split(path, "/")
				filename := parts[len(parts)-1]
				return fmt.Sprintf("📖 read: %s", filename)
			}
		case "list_directory":
			if path, ok := e.ToolArgs["dir_path"].(string); ok {
				return fmt.Sprintf("📂 ls: %s", path)
			}
		case "write_file", "create_file":
			if path, ok := e.ToolArgs["file_path"].(string); ok {
				parts := strings.Split(path, "/")
				filename := parts[len(parts)-1]
				return fmt.Sprintf("✏️  write: %s", filename)
			}
		case "edit_file":
			if path, ok := e.ToolArgs["file_path"].(string); ok {
				parts := strings.Split(path, "/")
				filename := parts[len(parts)-1]
				return fmt.Sprintf("📝 edit: %s", filename)
			}
		case "search_files", "grep":
			if query, ok := e.ToolArgs["query"].(string); ok {
				if len(query) > 30 {
					query = query[:27] + "..."
				}
				return fmt.Sprintf("🔍 search: %s", query)
			}
		}
		return fmt.Sprintf("🔧 %s", e.ToolName)
	case ACPEventToolResult:
		// Check for success/failure status
		icon := "📤"
		if e.Status == "success" || e.Status == "" {
			icon = "✅"
		} else if e.Status == "error" || e.Status == "failure" {
			icon = "❌"
		}
		// Try to show a snippet of the output
		content := e.Content
		if content == "" {
			return fmt.Sprintf("%s Result for: %s", icon, e.ToolID)
		}
		// Clean up and truncate
		content = strings.ReplaceAll(content, "\n", " ")
		content = strings.ReplaceAll(content, "\r", "")
		content = strings.TrimSpace(content)
		if len(content) > maxLen-5 {
			content = content[:maxLen-8] + "..."
		}
		return fmt.Sprintf("%s %s", icon, content)
	case ACPEventResult:
		return fmt.Sprintf("Complete (status: %s)", e.Status)
	case ACPEventError:
		content := e.Content
		if len(content) > maxLen {
			content = content[:maxLen-3] + "..."
		}
		return content
	default:
		raw := e.Raw
		if len(raw) > maxLen {
			raw = raw[:maxLen-3] + "..."
		}
		return raw
	}
}

// ParseACPEvent parses a line of Gemini stream-json output
func ParseACPEvent(line string) ACPEvent {
	line = strings.TrimSpace(line)

	// Skip empty lines
	if len(line) == 0 {
		return ACPEvent{Type: ACPEventUnknown, Raw: line}
	}

	// Try to parse as JSON
	if strings.HasPrefix(line, "{") {
		return parseStreamJSON(line)
	}

	// Non-JSON line (like "Loaded cached credentials.")
	return ACPEvent{
		Type:    ACPEventMessage,
		Role:    "system",
		Content: line,
		Raw:     line,
	}
}

// parseStreamJSON parses Gemini's stream-json format
func parseStreamJSON(line string) ACPEvent {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return ACPEvent{Type: ACPEventUnknown, Raw: line, Content: line}
	}

	event := ACPEvent{Raw: line}

	// Get type field
	if t, ok := raw["type"].(string); ok {
		switch t {
		case "init":
			event.Type = ACPEventInit
		case "message":
			event.Type = ACPEventMessage
		case "tool_use":
			event.Type = ACPEventToolUse
		case "tool_result":
			event.Type = ACPEventToolResult
		case "result":
			event.Type = ACPEventResult
		case "error":
			event.Type = ACPEventError
		default:
			event.Type = ACPEventUnknown
		}
	}

	// Extract common fields
	if role, ok := raw["role"].(string); ok {
		event.Role = role
	}
	if content, ok := raw["content"].(string); ok {
		event.Content = content
	}
	// tool_result uses "output" instead of "content"
	if output, ok := raw["output"].(string); ok {
		if event.Content == "" {
			event.Content = output
		}
	}
	if toolName, ok := raw["tool_name"].(string); ok {
		event.ToolName = toolName
	}
	if toolID, ok := raw["tool_id"].(string); ok {
		event.ToolID = toolID
	}
	if sessionID, ok := raw["session_id"].(string); ok {
		event.SessionID = sessionID
	}
	if model, ok := raw["model"].(string); ok {
		event.Model = model
	}
	if status, ok := raw["status"].(string); ok {
		event.Status = status
	}
	// Try both "parameters" and "args" for tool arguments
	if params, ok := raw["parameters"].(map[string]interface{}); ok {
		event.ToolArgs = params
	} else if args, ok := raw["args"].(map[string]interface{}); ok {
		event.ToolArgs = args
	}
	if stats, ok := raw["stats"].(map[string]interface{}); ok {
		event.Stats = stats
	}

	return event
}

// FormatACPEventForDisplay formats an ACP event for TUI display
func FormatACPEventForDisplay(event ACPEvent, maxWidth int) string {
	icon := event.Icon()

	// Calculate available width for summary
	availableWidth := maxWidth - len(icon) - 2 // -2 for space and buffer
	if availableWidth < 30 {
		availableWidth = 30
	}

	summary := event.Summary(availableWidth)

	return icon + " " + summary
}

// ColorizeJSON adds ANSI color codes to pretty-printed JSON
// Keys: cyan, Strings: green, Numbers: yellow, Booleans: magenta, Null: dim
func ColorizeJSON(jsonStr string) string {
	// ANSI color codes
	const (
		reset    = "\033[0m"
		cyan     = "\033[36m"   // Keys
		green    = "\033[32m"   // Strings
		yellow   = "\033[33m"   // Numbers
		magenta  = "\033[35m"   // Booleans
		dimWhite = "\033[2;37m" // Null
	)

	var result strings.Builder
	inString := false
	isKey := false
	escaped := false

	for i := 0; i < len(jsonStr); i++ {
		c := jsonStr[i]

		if escaped {
			result.WriteByte(c)
			escaped = false
			continue
		}

		if c == '\\' && inString {
			result.WriteByte(c)
			escaped = true
			continue
		}

		if c == '"' {
			if !inString {
				// Starting a string
				inString = true
				// Check if this is a key (followed by :)
				isKey = false
				for j := i + 1; j < len(jsonStr); j++ {
					if jsonStr[j] == '"' {
						// Look for colon after the closing quote
						for k := j + 1; k < len(jsonStr); k++ {
							if jsonStr[k] == ':' {
								isKey = true
								break
							} else if jsonStr[k] != ' ' && jsonStr[k] != '\n' && jsonStr[k] != '\t' {
								break
							}
						}
						break
					}
				}
				if isKey {
					result.WriteString(cyan)
				} else {
					result.WriteString(green)
				}
				result.WriteByte(c)
			} else {
				// Ending a string
				result.WriteByte(c)
				result.WriteString(reset)
				inString = false
			}
			continue
		}

		if inString {
			result.WriteByte(c)
			continue
		}

		// Check for special values outside strings
		remaining := jsonStr[i:]
		if strings.HasPrefix(remaining, "true") {
			result.WriteString(magenta + "true" + reset)
			i += 3
			continue
		}
		if strings.HasPrefix(remaining, "false") {
			result.WriteString(magenta + "false" + reset)
			i += 4
			continue
		}
		if strings.HasPrefix(remaining, "null") {
			result.WriteString(dimWhite + "null" + reset)
			i += 3
			continue
		}

		// Numbers
		if c >= '0' && c <= '9' || c == '-' {
			result.WriteString(yellow)
			result.WriteByte(c)
			for i+1 < len(jsonStr) {
				next := jsonStr[i+1]
				if (next >= '0' && next <= '9') || next == '.' || next == 'e' || next == 'E' || next == '+' || next == '-' {
					i++
					result.WriteByte(next)
				} else {
					break
				}
			}
			result.WriteString(reset)
			continue
		}

		result.WriteByte(c)
	}

	return result.String()
}
