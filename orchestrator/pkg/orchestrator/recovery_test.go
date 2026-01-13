package orchestrator

import (
	"testing"
)

func TestIsMinorChange(t *testing.T) {
	tests := []struct {
		name      string
		fileCount int
		lineCount int
		want      bool
	}{
		{"minor", 1, 10, true},
		{"borderline", 1, 19, true},
		{"too many lines", 1, 20, false},
		{"too many files", 2, 10, false},
		{"both over", 2, 25, false},
		{"zero", 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMinorChange(tt.fileCount, tt.lineCount); got != tt.want {
				t.Errorf("isMinorChange() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseDiffStat(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   int
	}{
		{
			"standard",
			" file1.txt | 10 +++++++++-\n 1 file changed, 9 insertions(+), 1 deletion(-)",
			11, // 1 + 9 + 1
		},
		{
			"multiple files",
			" f1 | 5 +\n f2 | 5 -\n 2 files changed, 5 insertions(+), 5 deletions(-)",
			12, // 2 + 5 + 5
		},
		{
			"no changes",
			" 0 files changed",
			0,
		},
		{
			"empty",
			"",
			0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseDiffStat(tt.output); got != tt.want {
				t.Errorf("parseDiffStat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRetryCounter(t *testing.T) {
	taskID := "test-task"

	// Ensure we start clean
	clearRetry(taskID)

	// Test increment
	if got := incrementRetry(taskID); got != 1 {
		t.Errorf("incrementRetry() = %v, want 1", got)
	}
	if got := incrementRetry(taskID); got != 2 {
		t.Errorf("incrementRetry() = %v, want 2", got)
	}

	// Test max
	for i := 0; i < maxTaskRetries-2; i++ {
		incrementRetry(taskID)
	}
	if got := taskRetryCount[taskID]; got != maxTaskRetries {
		t.Errorf("retry count = %v, want %v", got, maxTaskRetries)
	}

	// Test clear
	clearRetry(taskID)
	if _, exists := taskRetryCount[taskID]; exists {
		t.Error("clearRetry() did not remove task from map")
	}
}

func TestMaxTaskRetriesConstant(t *testing.T) {
	if maxTaskRetries != 5 {
		t.Errorf("maxTaskRetries = %v, want 5", maxTaskRetries)
	}
}
