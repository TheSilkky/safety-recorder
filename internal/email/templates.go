package email

import (
	"strings"
	"time"
)

const (
	AccountVerificationSubject = "Verify your Proofline account"
	EmailChallengeSubject      = "Your Proofline security code"
)

type AccountVerificationTemplateData struct {
	To               string
	VerificationLink string
	ExpiresAt        time.Time
}

type EmailChallengeTemplateData struct {
	To        string
	Code      string
	ExpiresAt time.Time
}

func NewAccountVerificationMessage(data AccountVerificationTemplateData) Message {
	return Message{
		To:      data.To,
		Subject: AccountVerificationSubject,
		Body: strings.Join([]string{
			"Proofline account verification",
			"",
			"Use this link to verify the email address for your Proofline account:",
			"",
			data.VerificationLink,
			"",
			"This link expires at " + data.ExpiresAt.UTC().Format(time.RFC3339) + ".",
			"If you did not create this Proofline account, ignore this email. The account cannot be used until email verification succeeds.",
			"",
			"Proofline is preview server software. This email does not send alerts, contact responders, change billing, or guarantee any response.",
			"",
		}, "\n"),
	}
}

func NewEmailChallengeMessage(data EmailChallengeTemplateData) Message {
	return Message{
		To:      data.To,
		Subject: EmailChallengeSubject,
		Body: strings.Join([]string{
			"Proofline email security code",
			"",
			"Use this code to finish Proofline email second-factor setup:",
			"",
			data.Code,
			"",
			"This code expires at " + data.ExpiresAt.UTC().Format(time.RFC3339) + ".",
			"If you did not request this email challenge, ignore this email and keep using your existing sign-in method.",
			"",
			"Proofline is preview server software. This code does not send alerts, contact responders, change billing, or guarantee any response.",
			"",
		}, "\n"),
	}
}
