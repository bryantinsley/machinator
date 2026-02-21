package executor

import "testing"

func TestParseDiffStat(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected int
	}{
		{
			name:     "empty output",
			output:   "",
			expected: 0,
		},
		{
			name:     "single file changed",
			output:   " file1.txt | 10 ++++++++++\n 1 file changed, 10 insertions(+)",
			expected: 10,
		},
		{
			name:     "multiple files changed",
			output:   " file1.txt | 10 ++++++++++\n file2.txt | 5 +++++\n 2 files changed, 10 insertions(+), 5 deletions(-)",
			expected: 15,
		},
		{
			name:     "summary with only insertions",
			output:   " 1 file changed, 5 insertions(+)",
			expected: 5,
		},
		{
			name:     "summary with only deletions",
			output:   " 1 file changed, 3 deletions(-)",
			expected: 3,
		},
		{
			name:     "complex summary line",
			output:   " 3 files changed, 10 insertions(+), 5 deletions(-), 2 modifications(!)",
			expected: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDiffStat(tt.output)
			if got != tt.expected {
				t.Errorf("parseDiffStat() = %v, want %v", got, tt.expected)
			}
		})
	}
}
