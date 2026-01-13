package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRateLimiting(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mock-bd-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	lastPushFile := filepath.Join(tempDir, "last_push")

	// First push - should not wait
	start := time.Now()
	doPrePush(lastPushFile)
	elapsed := time.Since(start)
	if elapsed > 1*time.Second {
		t.Errorf("First push took too long: %v", elapsed)
	}

	// Second push immediately after with a short rate limit
	os.Setenv("MACHINATOR_RATE_LIMIT", "2")
	start = time.Now()
	doPrePush(lastPushFile)
	elapsed = time.Since(start)

	if elapsed < 1*time.Second {
		t.Errorf("Second push should have waited, but only took %v", elapsed)
	}
	if elapsed > 3*time.Second {
		t.Errorf("Second push waited too long: %v", elapsed)
	}

	// Third push after waiting enough time - should not wait
	time.Sleep(2 * time.Second)
	start = time.Now()
	doPrePush(lastPushFile)
	elapsed = time.Since(start)
	if elapsed > 1*time.Second {
		t.Errorf("Third push should not have waited, but took %v", elapsed)
	}
}
