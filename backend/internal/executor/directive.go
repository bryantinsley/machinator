package executor

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BuildDirective assembles the agent prompt from a template and task context.
// It reads a template from bootstrap/directive_template.txt and performs variable substitutions.
// The result is written to {workdir}/.machinator/directive.md.
// It returns the path to the assembled directive file.
func BuildDirective(taskID string, taskDescription string, agentsContext string, agentName string, branchProtection string) (string, error) {
	// Read template
	templatePath := "bootstrap/directive_template.txt"
	templateBytes, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read directive template from %s: %w", templatePath, err)
	}
	directive := string(templateBytes)

	// Read first 100 lines of AGENTS.md for project context
	agentsPath := "AGENTS.md"
	agentsFile, err := os.Open(agentsPath)
	if err != nil {
		return "", fmt.Errorf("failed to open %s: %w", agentsPath, err)
	}
	defer agentsFile.Close()

	var projectContextBuilder strings.Builder
	scanner := bufio.NewScanner(agentsFile)
	for i := 0; i < 100 && scanner.Scan(); i++ {
		projectContextBuilder.WriteString(scanner.Text())
		projectContextBuilder.WriteString("\n")
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read %s: %w", agentsPath, err)
	}
	projectContext := projectContextBuilder.String()

	// Perform substitutions
	// AGENT_NAME_VAR -> agentName
	// TASK_ID_VAR -> taskID
	// TASK_CONTEXT_VAR -> taskDescription
	// PROJECT_CONTEXT_VAR (or "See AGENTS.md for full project context.") -> first 100 lines of AGENTS.md
	directive = strings.ReplaceAll(directive, "AGENT_NAME_VAR", agentName)
	directive = strings.ReplaceAll(directive, "TASK_ID_VAR", taskID)
	directive = strings.ReplaceAll(directive, "TASK_CONTEXT_VAR", taskDescription)
	directive = strings.ReplaceAll(directive, "PROJECT_CONTEXT_VAR", projectContext)

	// Create .machinator dir in the current working directory
	workDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	machinatorDir := filepath.Join(workDir, ".machinator")
	if err := os.MkdirAll(machinatorDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .machinator directory: %w", err)
	}

	// Write the directive to {workdir}/.machinator/directive.md
	directivePath := filepath.Join(machinatorDir, "directive.md")
	if err := os.WriteFile(directivePath, []byte(directive), 0644); err != nil {
		return "", fmt.Errorf("failed to write directive file to %s: %w", directivePath, err)
	}

	return directivePath, nil
}
