package executor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type executionSummary struct {
	TaskID         string  `json:"taskID"`
	AgentID        int     `json:"agentID"`
	Model          string  `json:"model"`
	StartTime      string  `json:"startTime"`
	EndTime        string  `json:"endTime"`
	Duration       float64 `json:"duration"`
	ExitCode       int     `json:"exitCode"`
	Error          string  `json:"error"`
	TriageAction   string  `json:"triageAction"`
	CommitsCreated int     `json:"commitsCreated"`
	ExecutionID    string  `json:"executionID"`
}

// ExecuteTask creates the top-level function that ties directive building, launching, monitoring, and triage together.
func ExecuteTask(cfg ExecutionConfig, taskID string, taskDescription string, agentsContext string, machinatorDir string, logger Logger) (result ExecutionResult) {
	startTime := time.Now()
	// 1. Generate execution ID via NewExecutionID(taskID)
	execID := NewExecutionID(taskID)

	var logDir string
	var triageAction string

	defer func() {
		if logDir != "" {
			summary := executionSummary{
				TaskID:         taskID,
				AgentID:        cfg.AgentID,
				Model:          cfg.Model,
				StartTime:      startTime.Format(time.RFC3339),
				EndTime:        time.Now().Format(time.RFC3339),
				Duration:       time.Since(startTime).Seconds(),
				ExitCode:       result.ExitCode,
				TriageAction:   triageAction,
				CommitsCreated: result.CommitsCreated,
				ExecutionID:    execID,
			}
			if result.Error != nil {
				summary.Error = result.Error.Error()
			}

			summaryBytes, _ := json.MarshalIndent(summary, "", "  ")
			_ = os.WriteFile(filepath.Join(logDir, "result.json"), summaryBytes, 0644)
		}
	}()

	// 2. Create log directory via CreateLogDir(machinatorDir, execID)
	var err error
	logDir, err = CreateLogDir(machinatorDir, execID)
	if err != nil {
		return ExecutionResult{Error: fmt.Errorf("failed to create log dir: %w", err), ExitCode: 1}
	}

	// Open orchestrator log to log each step
	orchestratorLogPath := filepath.Join(logDir, "orchestrator.log")
	orchestratorLog, err := os.OpenFile(orchestratorLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return ExecutionResult{Error: fmt.Errorf("failed to open orchestrator log: %w", err), ExitCode: 1}
	}
	defer orchestratorLog.Close()

	// Helper to log to both passed Logger and orchestrator.log
	logStep := func(step string) {
		msg := fmt.Sprintf("[%s] %s", execID, step)
		if logger != nil {
			logger.Log("orchestrator", msg)
		}
		fmt.Fprintf(orchestratorLog, "%s\n", msg)
	}

	logStep("Starting task execution")

	// 3. Set git author via SetGitAuthor(cfg.WorktreeDir, cfg.AgentID)
	logStep(fmt.Sprintf("Setting git author for agent %d", cfg.AgentID))
	if err := SetGitAuthor(cfg.WorktreeDir, cfg.AgentID); err != nil {
		return ExecutionResult{Error: fmt.Errorf("failed to set git author: %w", err), ExitCode: 1}
	}

	// 4. Build directive via BuildDirective(...)
	logStep("Building directive")
	agentName := fmt.Sprintf("Agent-%d", cfg.AgentID)
	directiveContent, err := BuildDirective(taskID, taskDescription, agentsContext, agentName, "")
	if err != nil {
		return ExecutionResult{Error: fmt.Errorf("failed to build directive: %w", err), ExitCode: 1}
	}

	// 5. Copy directive to log dir as directive.md
	logStep("Writing directive to log directory")
	directivePath := filepath.Join(logDir, "directive.md")
	if err := os.WriteFile(directivePath, []byte(directiveContent), 0644); err != nil {
		return ExecutionResult{Error: fmt.Errorf("failed to write directive.md: %w", err), ExitCode: 1}
	}

	// 6. Launch Gemini via LaunchGemini(cfg, directivePath)
	logStep("Launching Gemini")
	cmd, stdout, err := LaunchGemini(cfg, directivePath)
	if err != nil {
		return ExecutionResult{Error: fmt.Errorf("failed to launch Gemini: %w", err), ExitCode: 1}
	}

	// 7. Start ACP parser goroutine via ParseACPStream()
	logStep("Starting ACP stream parser")
	eventsChan := make(chan ACPEvent, 100)
	doneChan := make(chan error, 1)
	
	// Create an internal done channel for ParseACPStream
	parseDoneChan := make(chan error, 1)
	go ParseACPStream(stdout, eventsChan, parseDoneChan)
	
	go func() {
		// Wait for parser to finish
		parseErr := <-parseDoneChan
		// Wait for command to exit
		waitErr := cmd.Wait()
		
		// Prioritize command wait error if parser succeeded
		if parseErr != nil {
			doneChan <- parseErr
		} else {
			doneChan <- waitErr
		}
	}()

	// Create channel for events to be passed to Monitor
	monitorEventsChan := make(chan ACPEvent, 100)

	// 9. Log raw events to events.jsonl in the log dir
	eventsLogPath := filepath.Join(logDir, "events.jsonl")
	eventsLog, err := os.OpenFile(eventsLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return ExecutionResult{Error: fmt.Errorf("failed to open events.jsonl: %w", err), ExitCode: 1}
	}
	defer eventsLog.Close()

	agentSource := fmt.Sprintf("agent-%d", cfg.AgentID)

	go func() {
		defer close(monitorEventsChan)
		for evt := range eventsChan {
			// 8. Forward events to logger: logger.Log('agent-{agentID}', event summary)
			summary := fmt.Sprintf("Event Type: %s", evt.Type)
			if logger != nil {
				logger.Log(agentSource, summary)
			}

			// 9. Log raw events to events.jsonl in the log dir
			raw := evt.RawJSON
			if raw == nil {
				raw, _ = json.Marshal(evt)
			}
			fmt.Fprintf(eventsLog, "%s\n", string(raw))

			// Pass event to monitor
			monitorEventsChan <- evt
		}
	}()

	// 10. Monitor via Monitor() — blocks until completion
	logStep("Monitoring execution")
	result = Monitor(cmd, monitorEventsChan, doneChan, cfg, logger)

	logStep(fmt.Sprintf("Execution finished with code %d, duration %v", result.ExitCode, result.Duration))

	// 11. Triage worktree via TriageWorktree()
	logStep("Triaging worktree")
	triageAction, err = TriageWorktree(cfg.WorktreeDir, taskID, logger)
	if err != nil {
		logStep(fmt.Sprintf("Failed to triage worktree: %v", err))
		result.Error = fmt.Errorf("triage error: %w", err)
		return result
	}
	logStep(fmt.Sprintf("Triage action determined: %s", triageAction))

	// 12. If triage returns 'minor_discard' or 'clean': ResetWorktree()
	if triageAction == "minor_discard" || triageAction == "clean" {
		logStep("Resetting worktree")
		if err := ResetWorktree(cfg.WorktreeDir, taskID); err != nil {
			logStep(fmt.Sprintf("Failed to reset worktree: %v", err))
		}
	} else if triageAction == "salvage" {
		// 13. If triage returns 'salvage': SalvageWorktree()
		logStep("Salvaging worktree")
		if err := SalvageWorktree(cfg.WorktreeDir, taskID, logger); err != nil {
			logStep(fmt.Sprintf("Failed to salvage worktree: %v", err))
		}
	}

	// 14. Return ExecutionResult
	logStep("Task execution complete")
	return result
}
