package setup

import (
	"fmt"
	"testing"
	"time"
)

func TestGeminiAuthDoneTransitionsToValidating(t *testing.T) {
	m := initialModel()
	m.screen = screenAddAccountAuth
	m.newAccountName = "test-account"

	// Simulate receiving geminiAuthDoneMsg
	msg := geminiAuthDoneMsg{err: nil}
	newModel, cmd := m.Update(msg)
	m2 := newModel.(model)

	if m2.screen != screenValidatingAccount {
		t.Errorf("Expected screenValidatingAccount, got %v", m2.screen)
	}

	if m2.validationStartTime.IsZero() {
		t.Error("validationStartTime should be set")
	}

	if cmd == nil {
		t.Error("Expected a command to validate quota")
	}
}

func TestGeminiAuthDoneWithErrorTransitionsToValidating(t *testing.T) {
	m := initialModel()
	m.screen = screenAddAccountAuth
	m.newAccountName = "test-account"

	// Simulate receiving geminiAuthDoneMsg with error
	msg := geminiAuthDoneMsg{err: fmt.Errorf("auth failed")}
	newModel, cmd := m.Update(msg)
	m2 := newModel.(model)

	if m2.screen != screenValidatingAccount {
		t.Errorf("Expected screenValidatingAccount, got %v", m2.screen)
	}

	if m2.validationError == nil || m2.validationError.Error() != "auth failed" {
		t.Errorf("Expected validationError 'auth failed', got %v", m2.validationError)
	}

	if cmd != nil {
		t.Error("Expected NO command to validate quota when auth failed")
	}
}

func TestQuotaCheckResultUpdatesModal(t *testing.T) {
	m := initialModel()
	m.width = 100
	m.height = 36
	m.screen = screenValidatingAccount
	m.newAccountName = "test-account"
	m.validationStartTime = time.Now()

	// Simulate receiving accountQuotaMsg
	msg := accountQuotaMsg{flash: 50, pro: 80, err: nil}
	newModel, _ := m.Update(msg)
	m2 := newModel.(model)

	if m2.validationFlash != 50 {
		t.Errorf("Expected validationFlash 50, got %d", m2.validationFlash)
	}
	if m2.validationPro != 80 {
		t.Errorf("Expected validationPro 80, got %d", m2.validationPro)
	}

	view := m2.View()
	if !contains(view, "Gemini Flash: 50%") {
		t.Errorf("View should contain Flash quota result")
	}
	if !contains(view, "Gemini Pro:   80%") {
		t.Errorf("View should contain Pro quota result")
	}
}

func TestQuotaValidationViewShowsError(t *testing.T) {
	m := initialModel()
	m.width = 100
	m.height = 36
	m.screen = screenValidatingAccount
	m.validationError = fmt.Errorf("quota check failed")

	view := m.View()
	if !contains(view, "Error: quota check failed") {
		t.Errorf("View should contain error message, got:\n%s", view)
	}
}

func TestValidationTimeout(t *testing.T) {
	m := initialModel()
	m.screen = screenValidatingAccount
	m.newAccountName = "test-account"
	// Set start time to 11 seconds ago
	m.validationStartTime = time.Now().Add(-11 * time.Second)

	// Simulate tickMsg
	msg := tickMsg(time.Now())
	newModel, _ := m.Update(msg)
	m2 := newModel.(model)

	if m2.screen != screenAddAccountAuth {
		t.Errorf("Expected return to screenAddAccountAuth on timeout, got %v", m2.screen)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(substr) > 0 && (len(s) > len(substr)) && (s[0:len(substr)] == substr || contains(s[1:], substr))))
}
