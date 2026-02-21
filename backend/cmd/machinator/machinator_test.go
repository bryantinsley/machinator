package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
)

func TestBinaryHelp(t *testing.T) {
	// Find the machinator binary via Bazel runfiles.
	// We use the runfiles path relative to the workspace.
	// The binary label is :machinator, and it's in backend/cmd/machinator.
	binaryPath, err := bazel.Runfile("backend/cmd/machinator/machinator_/machinator")
	if err != nil {
		// Try alternate path
		binaryPath, err = bazel.Runfile("backend/cmd/machinator/machinator")
		if err != nil {
			t.Fatalf("Could not find machinator binary in runfiles: %v", err)
		}
	}

	t.Logf("Using binary path: %s", binaryPath)

	cmd := exec.Command(binaryPath, "--help")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if runErr != nil {
		t.Fatalf("Binary failed with error: %v\nStdout: %s\nStderr: %s", runErr, stdout.String(), stderr.String())
	}

	output := stdout.String() + stderr.String()
	t.Logf("Output: %s", output)

	requiredStrings := []string{"machinator", "run", "setup"}
	for _, s := range requiredStrings {
		if !strings.Contains(output, s) {
			t.Errorf("Output does not contain required string %q", s)
		}
	}
}
