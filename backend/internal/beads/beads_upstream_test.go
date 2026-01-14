package beads

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

// ╔══════════════════════════════════════════════════════════════════════════════╗
// ║ DO NOT REMOVE - UPSTREAM SYNC CHECK                                          ║
// ╠══════════════════════════════════════════════════════════════════════════════╣
// ║ This test fetches the upstream beads Issue struct and compares it with our   ║
// ║ local Task struct to detect when new fields are added upstream.              ║
// ║                                                                              ║
// ║ If this test fails, it means the upstream Issue struct has new fields that   ║
// ║ we haven't added to our Task struct yet. See beads.go for update steps.      ║
// ╚══════════════════════════════════════════════════════════════════════════════╝

const upstreamTypesURL = "https://raw.githubusercontent.com/steveyegge/beads/main/internal/types/types.go"

// knownIgnoredFields are upstream fields we intentionally don't sync
// (internal routing fields, advanced features we don't use yet, etc.)
var knownIgnoredFields = map[string]bool{
	"ContentHash":       true, // Internal, not exported to JSONL
	"SourceRepo":        true, // Internal routing
	"IDPrefix":          true, // Internal routing
	"PrefixOverride":    true, // Internal routing
	"CompactionLevel":   true, // Compaction metadata
	"CompactedAt":       true, // Compaction metadata
	"CompactedAtCommit": true, // Compaction metadata
	"OriginalSize":      true, // Compaction metadata
	"DeletedAt":         true, // Tombstone
	"DeletedBy":         true, // Tombstone
	"DeleteReason":      true, // Tombstone
	"OriginalType":      true, // Tombstone
	"Sender":            true, // Messaging
	"Ephemeral":         true, // Messaging
	"Pinned":            true, // Context markers
	"IsTemplate":        true, // Templates
	"BondedFrom":        true, // Bonding/compound molecules
	"Creator":           true, // HOP entity tracking
	"Validations":       true, // HOP validations
	"QualityScore":      true, // HOP quality
	"Crystallizes":      true, // HOP
	"AwaitType":         true, // Gate fields
	"AwaitID":           true, // Gate fields
	"Timeout":           true, // Gate fields
	"Waiters":           true, // Gate fields
	"Holder":            true, // Slot fields
	"SourceFormula":     true, // Formula cooking
	"SourceLocation":    true, // Formula cooking
	"HookBead":          true, // Agent identity
	"RoleBead":          true, // Agent identity
	"AgentState":        true, // Agent identity
	"LastActivity":      true, // Agent identity
	"RoleType":          true, // Agent identity
	"Rig":               true, // Agent identity
	"MolType":           true, // Swarm coordination
	"WorkType":          true, // Work assignment
	"EventKind":         true, // Event fields
	"Actor":             true, // Event fields
	"Target":            true, // Event fields
	"Payload":           true, // Event fields
	"ExternalRef":       true, // External integration
	"SourceSystem":      true, // External integration
	"ClosedBySession":   true, // Session tracking
	"Dependencies":      true, // We use BlockedBy instead
}

// TestUpstreamSync checks if our Task struct is in sync with upstream Issue struct.
// This test will WARN (not fail) if new fields are detected, since network may be unavailable.
func TestUpstreamSync(t *testing.T) {
	// Fetch upstream types.go
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(upstreamTypesURL)
	if err != nil {
		t.Skipf("Could not fetch upstream types.go (network unavailable?): %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Skipf("Upstream types.go returned status %d", resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Skipf("Could not read upstream types.go: %v", err)
		return
	}

	upstreamFields := extractStructFields(string(body), "Issue struct")
	localFields := getLocalTaskFields()

	// Check for missing fields
	var missing []string
	for field := range upstreamFields {
		if knownIgnoredFields[field] {
			continue
		}
		if !localFields[field] {
			missing = append(missing, field)
		}
	}

	if len(missing) > 0 {
		t.Errorf("Task struct is missing %d field(s) from upstream Issue struct:\n  - %s\n\nSee beads.go for update instructions.",
			len(missing), strings.Join(missing, "\n  - "))
	}
}

// extractStructFields extracts field names from a Go struct definition in source code.
// It returns a map of field names found in the struct.
func extractStructFields(source, structName string) map[string]bool {
	fields := make(map[string]bool)

	// Find the struct definition
	structPattern := regexp.MustCompile(`(?s)type\s+` + regexp.QuoteMeta(structName) + `\s*\{([^}]+(?:\{[^}]*\}[^}]*)*)\}`)
	matches := structPattern.FindStringSubmatch(source)
	if len(matches) < 2 {
		return fields
	}

	structBody := matches[1]

	// Extract field names (first word on each line that's capitalized)
	fieldPattern := regexp.MustCompile(`(?m)^\s*([A-Z][a-zA-Z0-9]*)\s+`)
	fieldMatches := fieldPattern.FindAllStringSubmatch(structBody, -1)

	for _, match := range fieldMatches {
		if len(match) >= 2 {
			fields[match[1]] = true
		}
	}

	return fields
}

// getLocalTaskFields returns a map of field names in our local Task struct.
// This is hardcoded to avoid reflect dependency and keep it simple.
func getLocalTaskFields() map[string]bool {
	return map[string]bool{
		// Core identification
		"ID": true,
		// Issue content
		"Title":              true,
		"Description":        true,
		"Design":             true,
		"AcceptanceCriteria": true,
		"Notes":              true,
		// Status & workflow
		"Status":    true,
		"Priority":  true,
		"IssueType": true,
		// Assignment
		"Assignee":         true,
		"Owner":            true,
		"EstimatedMinutes": true,
		// Timestamps
		"CreatedAt":   true,
		"CreatedBy":   true,
		"UpdatedAt":   true,
		"ClosedAt":    true,
		"CloseReason": true,
		"DueAt":       true,
		"DeferUntil":  true,
		// Relations
		"Labels":    true,
		"BlockedBy": true,
		"Comments":  true,
		// Derived (not in JSON)
		"IsComplex": true,
	}
}
