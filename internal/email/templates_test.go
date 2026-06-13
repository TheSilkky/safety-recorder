package email

import (
	"strings"
	"testing"
	"time"
)

func TestAccountVerificationTemplateContentAndHeaderSafety(t *testing.T) {
	expiresAt := time.Date(2026, 6, 13, 12, 30, 0, 0, time.UTC)
	msg := NewAccountVerificationMessage(AccountVerificationTemplateData{
		To:               "new.user@example.invalid",
		VerificationLink: "https://app.example.invalid/verify-email#token=test-token",
		ExpiresAt:        expiresAt,
	})

	if msg.To != "new.user@example.invalid" {
		t.Fatalf("to = %q", msg.To)
	}
	if msg.Subject != AccountVerificationSubject {
		t.Fatalf("subject = %q", msg.Subject)
	}
	assertTemplateHeaderSafe(t, msg)
	for _, expected := range []string{
		"Proofline account verification",
		"https://app.example.invalid/verify-email#token=test-token",
		"This link expires at 2026-06-13T12:30:00Z.",
		"If you did not create this Proofline account, ignore this email.",
		"does not send alerts, contact responders, change billing, or guarantee any response",
	} {
		if !strings.Contains(msg.Body, expected) {
			t.Fatalf("verification body missing %q: %q", expected, msg.Body)
		}
	}
	for _, disallowed := range []string{"emergency dispatch", "production-ready", "SMS", "Messenger"} {
		if strings.Contains(msg.Body, disallowed) {
			t.Fatalf("verification body included disallowed claim %q: %q", disallowed, msg.Body)
		}
	}
}

func TestEmailChallengeTemplateContentAndHeaderSafety(t *testing.T) {
	expiresAt := time.Date(2026, 6, 13, 12, 40, 0, 0, time.UTC)
	msg := NewEmailChallengeMessage(EmailChallengeTemplateData{
		To:        "setup.user@example.invalid",
		Code:      "123456",
		ExpiresAt: expiresAt,
	})

	if msg.To != "setup.user@example.invalid" {
		t.Fatalf("to = %q", msg.To)
	}
	if msg.Subject != EmailChallengeSubject {
		t.Fatalf("subject = %q", msg.Subject)
	}
	assertTemplateHeaderSafe(t, msg)
	for _, expected := range []string{
		"Proofline email security code",
		"Use this code to finish Proofline email second-factor setup:",
		"\n123456\n",
		"This code expires at 2026-06-13T12:40:00Z.",
		"If you did not request this email challenge, ignore this email",
		"does not send alerts, contact responders, change billing, or guarantee any response",
	} {
		if !strings.Contains(msg.Body, expected) {
			t.Fatalf("challenge body missing %q: %q", expected, msg.Body)
		}
	}
	for _, disallowed := range []string{"emergency dispatch", "production-ready", "SMS", "Messenger"} {
		if strings.Contains(msg.Body, disallowed) {
			t.Fatalf("challenge body included disallowed claim %q: %q", disallowed, msg.Body)
		}
	}
}

func assertTemplateHeaderSafe(t *testing.T, msg Message) {
	t.Helper()
	if strings.ContainsAny(msg.Subject, "\r\n") {
		t.Fatalf("subject contains newline: %q", msg.Subject)
	}
	if strings.ContainsAny(msg.To, "\r\n") {
		t.Fatalf("recipient contains newline: %q", msg.To)
	}
	if strings.Contains(msg.Body, "\r") {
		t.Fatalf("template body should use LF before SMTP normalization: %q", msg.Body)
	}
	if err := validateMessage("Proofline <noreply@example.invalid>", msg); err != nil {
		t.Fatalf("template message failed validation: %v", err)
	}
}
