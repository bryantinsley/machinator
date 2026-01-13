package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestModes(t *testing.T) {
	tests := []struct {
		name       string
		env        map[string]string
		args       []string
		wantExit   int
		wantOutput []string // substrings to check
	}{
		{
			name:       "happy",
			env:        map[string]string{"DUMMY_GEMINI_MODE": "happy"},
			wantExit:   0,
			wantOutput: []string{"init", "mock response", "success"},
		},
		{
			name:       "error",
			env:        map[string]string{"DUMMY_GEMINI_MODE": "error"},
			wantExit:   1,
			wantOutput: []string{"error", "Quota exceeded"},
		},
		{
			name:     "stuck",
			env:      map[string]string{"DUMMY_GEMINI_MODE": "stuck", "TEST_STUCK_TIMEOUT": "true"},
			wantExit: 0,
		},
		{
			name:       "version",
			args:       []string{"-version"},
			wantExit:   0,
			wantOutput: []string{"dummy-gemini v0.2.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			getEnv := func(key string) string {
				if v, ok := tt.env[key]; ok {
					return v
				}
				return ""
			}
			exitCode := run(&out, getEnv, tt.args)
			if exitCode != tt.wantExit {
				t.Errorf("run() exit code = %v, want %v", exitCode, tt.wantExit)
			}

			output := out.String()
			for _, want := range tt.wantOutput {
				if !strings.Contains(output, want) {
					t.Errorf("run() output missing %q, got: %q", want, output)
				}
			}
		})
	}
}

func TestScripted(t *testing.T) {
	// create temp file
	tmpFile, err := os.CreateTemp("", "dummy_script.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	scriptContent := `{"type":"init"}
{"type":"result","status":"success"}`
	tmpFile.WriteString(scriptContent)
	tmpFile.Close()

	var out bytes.Buffer
	getEnv := func(key string) string {
		if key == "DUMMY_GEMINI_MODE" {
			return "scripted"
		}
		if key == "DUMMY_GEMINI_SCRIPT" {
			return tmpFile.Name()
		}
		return ""
	}

	exitCode := run(&out, getEnv, nil)
	if exitCode != 0 {
		t.Errorf("run() exit code = %v, want 0", exitCode)
	}
	if !strings.Contains(out.String(), `"type":"init"`) {
		t.Errorf("scripted output missing content")
	}
}
