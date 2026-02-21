package quota

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewQuota(t *testing.T) {
	q := New("/tmp/machinator")
	if len(q.Accounts) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(q.Accounts))
	}
	if q.MachinatorDir != "/tmp/machinator" {
		t.Errorf("expected MachinatorDir /tmp/machinator, got %s", q.MachinatorDir)
	}
}

func TestTotalFor(t *testing.T) {
	q := &Quota{
		Accounts: []AccountQuota{
			{
				Name: "acc1",
				Models: map[string]float64{
					"gemini-1.5-pro": 0.5,
				},
			},
			{
				Name: "acc2",
				Models: map[string]float64{
					"gemini-1.5-pro": 0.3,
				},
			},
		},
	}

	total := q.TotalFor("gemini-1.5-pro")
	if total != 0.8 {
		t.Errorf("expected total 0.8, got %f", total)
	}

	totalNone := q.TotalFor("non-existent")
	if totalNone != 0.0 {
		t.Errorf("expected total 0.0 for non-existent model, got %f", totalNone)
	}
}

func TestBestAccountFor(t *testing.T) {
	q := &Quota{
		Accounts: []AccountQuota{
			{
				Name: "acc1",
				Models: map[string]float64{
					"gemini-1.5-pro": 0.5,
				},
			},
			{
				Name: "acc2",
				Models: map[string]float64{
					"gemini-1.5-pro": 0.8,
				},
			},
			{
				Name: "acc3",
				Models: map[string]float64{
					"gemini-1.5-pro": 0.2,
				},
			},
		},
	}

	best, err := q.BestAccountFor("gemini-1.5-pro")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if best != "acc2" {
		t.Errorf("expected acc2, got %s", best)
	}
}

func TestBestAccountForEmpty(t *testing.T) {
	q := &Quota{
		Accounts: []AccountQuota{},
	}

	best, err := q.BestAccountFor("gemini-1.5-pro")
	if err == nil {
		t.Errorf("expected error for no accounts, got nil (best: %s)", best)
	}
	if best != "" {
		t.Errorf("expected empty string for best account, got %s", best)
	}
}

func TestRefreshNoAccounts(t *testing.T) {
	tmpDir := t.TempDir()
	q := New(tmpDir)

	err := q.Refresh()
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	if len(q.Accounts) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(q.Accounts))
	}
}

func TestExtractAndParseJSON(t *testing.T) {
	output := []byte(`
Some spurious output
{
  "buckets": [
    {
      "modelId": "gemini-1.5-pro",
      "remainingFraction": 0.75
    },
    {
      "modelId": "gemini-1.5-flash",
      "remainingFraction": 0.5
    }
  ]
}
More spurious output
`)

	jsonBytes := extractJSON(output)
	if jsonBytes == nil {
		t.Fatal("failed to extract JSON")
	}

	// This duplicates some logic from fetchQuotaForAccount but allows us to test the parsing logic in isolation
	type bucket struct {
		ModelId           string  `json:"modelId"`
		RemainingFraction float64 `json:"remainingFraction"`
	}
	var result struct {
		Buckets []bucket `json:"buckets"`
	}

	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if len(result.Buckets) != 2 {
		t.Errorf("expected 2 buckets, got %d", len(result.Buckets))
	}

	models := make(map[string]float64)
	for _, b := range result.Buckets {
		models[b.ModelId] = b.RemainingFraction
	}

	if models["gemini-1.5-pro"] != 0.75 {
		t.Errorf("expected 0.75 for gemini-1.5-pro, got %f", models["gemini-1.5-pro"])
	}
	if models["gemini-1.5-flash"] != 0.5 {
		t.Errorf("expected 0.5 for gemini-1.5-flash, got %f", models["gemini-1.5-flash"])
	}
}

func TestDiscoverAccounts(t *testing.T) {
	tmpDir := t.TempDir()
	accountsDir := filepath.Join(tmpDir, "accounts")
	if err := os.MkdirAll(accountsDir, 0755); err != nil {
		t.Fatalf("failed to create accounts dir: %v", err)
	}

	accNames := []string{"acc1", "acc2", "acc3"}
	for _, name := range accNames {
		if err := os.Mkdir(filepath.Join(accountsDir, name), 0755); err != nil {
			t.Fatalf("failed to create account dir %s: %v", name, err)
		}
	}

	// Create a file that should be ignored
	if err := os.WriteFile(filepath.Join(accountsDir, "not-a-dir"), []byte("data"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	q := New(tmpDir)
	accounts, err := q.discoverAccounts()
	if err != nil {
		t.Fatalf("discoverAccounts failed: %v", err)
	}

	if len(accounts) != 3 {
		t.Errorf("expected 3 accounts, got %d", len(accounts))
	}

	// Check that they are the right ones
	for _, acc := range accounts {
		base := filepath.Base(acc)
		found := false
		for _, name := range accNames {
			if base == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("discovered unexpected account: %s", base)
		}
	}
}
