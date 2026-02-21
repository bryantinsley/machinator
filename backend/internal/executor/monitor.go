package executor

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Monitor watches a running Gemini process for timeouts and fatal errors.
// This is a blocking function that runs until the process completes or an error occurs.
func Monitor(cmd *exec.Cmd, events <-chan ACPEvent, done <-chan error, cfg ExecutionConfig, logger Logger) ExecutionResult {
	startTime := time.Now()
	lastEventTime := time.Now()
	parseErrorCount := 0
	rejectionString := "Command rejected because it could not be parsed safely"

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				// Events channel closed, stop tracking it.
				events = nil
				continue
			}
			lastEventTime = time.Now()

			// Check for consecutive parse errors.
			isParseError := false
			if event.Type == "text" {
				var te TextEvent
				if err := json.Unmarshal(event.RawJSON, &te); err == nil {
					if strings.Contains(te.Content, rejectionString) {
						isParseError = true
					}
				}
			}

			if isParseError {
				parseErrorCount++
				logger.Log("monitor", fmt.Sprintf("Observed parse error rejection (%d/3)", parseErrorCount))
				if parseErrorCount >= 3 {
					logger.Log("monitor", "Fatal parse errors threshold reached. Killing process.")
					cmd.Process.Kill()
					cmd.Wait()
					return ExecutionResult{
						ExitCode: -1,
						Duration: time.Since(startTime),
						Error:    fmt.Errorf("fatal parse errors"),
					}
				}
			} else {
				parseErrorCount = 0
			}

		case err := <-done:
			duration := time.Since(startTime)
			if err != nil {
				return ExecutionResult{
					ExitCode: -1,
					Duration: duration,
					Error:    err,
				}
			}
			// Normal completion. ProcessState should be populated by Wait() if it was successful.
			exitCode := 0
			if cmd.ProcessState != nil {
				exitCode = cmd.ProcessState.ExitCode()
			}
			return ExecutionResult{
				ExitCode: exitCode,
				Duration: duration,
			}

		case <-ticker.C:
			// Check total runtime
			if time.Since(startTime) > cfg.MaxRuntime {
				logger.Log("monitor", "Max runtime exceeded. Killing process.")
				cmd.Process.Kill()
				cmd.Wait()
				return ExecutionResult{
					ExitCode: -1,
					Duration: time.Since(startTime),
					Error:    fmt.Errorf("max runtime exceeded"),
				}
			}

			// Check idle timeout - only if we are still expecting events
			if events != nil && time.Since(lastEventTime) > cfg.IdleTimeout {
				logger.Log("monitor", "Idle timeout reached. Killing process.")
				cmd.Process.Kill()
				cmd.Wait()
				return ExecutionResult{
					ExitCode: -1,
					Duration: time.Since(startTime),
					Error:    fmt.Errorf("idle timeout"),
				}
			}
		}
	}
}
