package project

import (
	"testing"
	"time"
)

func TestConfig_ParseDurations(t *testing.T) {
	cfg := &Config{
		IdleTimeout: "10m",
		MaxRuntime:  "1h",
	}

	idle, max, err := cfg.ParseDurations()
	if err != nil {
		t.Fatalf("ParseDurations failed: %v", err)
	}

	if idle != 10*time.Minute {
		t.Errorf("expected idle to be 10m, got %v", idle)
	}

	if max != time.Hour {
		t.Errorf("expected max to be 1h, got %v", max)
	}
}

func TestConfig_ParseDurations_Error(t *testing.T) {
	cfg := &Config{
		IdleTimeout: "invalid",
		MaxRuntime:  "30m",
	}

	_, _, err := cfg.ParseDurations()
	if err == nil {
		t.Error("expected error for invalid idle_timeout, got nil")
	}

	cfg = &Config{
		IdleTimeout: "5m",
		MaxRuntime:  "invalid",
	}

	_, _, err = cfg.ParseDurations()
	if err == nil {
		t.Error("expected error for invalid max_runtime, got nil")
	}
}
