package email_test

import (
	"strings"
	"testing"

	"github.com/mayloo89/circl/backend/internal/email"
)

// --- ConsoleSender ---

func TestConsoleSender_Send(t *testing.T) {
	s := email.NewConsoleSender()
	msg := email.Message{
		To:      "user@example.com",
		Subject: "Hello",
		HTML:    "<p>Hi</p>",
		Text:    "Hi",
	}
	if err := s.Send(t.Context(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Templates ---

func TestPasswordResetMessage(t *testing.T) {
	msg := email.PasswordResetMessage("https://app.example", "user@example.com", "https://example.com/reset?token=abc")
	if msg.To != "user@example.com" {
		t.Errorf("To = %q, want %q", msg.To, "user@example.com")
	}
	if msg.Subject == "" {
		t.Error("Subject is empty")
	}
	if !strings.Contains(msg.HTML, "https://example.com/reset?token=abc") {
		t.Error("HTML does not contain reset URL")
	}
	if !strings.Contains(msg.Text, "https://example.com/reset?token=abc") {
		t.Error("Text does not contain reset URL")
	}
}

func TestEmailVerificationMessage(t *testing.T) {
	msg := email.EmailVerificationMessage("https://app.example", "user@example.com", "https://example.com/verify?token=xyz")
	if msg.To != "user@example.com" {
		t.Errorf("To = %q, want %q", msg.To, "user@example.com")
	}
	if msg.Subject == "" {
		t.Error("Subject is empty")
	}
	if !strings.Contains(msg.HTML, "https://example.com/verify?token=xyz") {
		t.Error("HTML does not contain verify URL")
	}
	if !strings.Contains(msg.Text, "https://example.com/verify?token=xyz") {
		t.Error("Text does not contain verify URL")
	}
}
