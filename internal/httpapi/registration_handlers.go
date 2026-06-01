package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/email"
)

const registrationAcceptedMessage = "If registration can be completed, a verification email will be sent."

type registerAccountRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *API) registerAccount(w http.ResponseWriter, r *http.Request) {
	switch a.accountRegistration.Mode {
	case AccountRegistrationDisabled, AccountRegistrationAdminOnly:
		writeError(w, http.StatusForbidden, "registration_disabled", "public account registration is disabled")
	case AccountRegistrationPaid:
		writeError(w, http.StatusServiceUnavailable, "registration_payment_unavailable", "paid registration is not available")
	case AccountRegistrationOpen:
		a.registerOpenAccount(w, r)
	default:
		writeError(w, http.StatusForbidden, "registration_disabled", "public account registration is disabled")
	}
}

func (a *API) registerOpenAccount(w http.ResponseWriter, r *http.Request) {
	var request registerAccountRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	username := auth.NormalizeUsername(request.Username)
	if err := auth.ValidateUsername(username); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_username", err.Error())
		return
	}
	emailAddress := auth.NormalizeEmail(request.Email)
	if err := auth.ValidateEmail(emailAddress); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_email", err.Error())
		return
	}
	passwordHash, err := auth.HashPassword(request.Password, a.passwordCost)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_password", err.Error())
		return
	}

	account, err := a.repo.CreateAccount(r.Context(), auth.CreateAccountParams{
		Username:        username,
		EmailNormalized: emailAddress,
		AccountState:    auth.AccountStatePendingEmailVerification,
		PasswordHash:    passwordHash,
		Role:            auth.RoleUser,
	})
	if errors.Is(err, auth.ErrDuplicate) {
		a.resendPendingRegistrationEmail(w, r, username, emailAddress)
		return
	}
	if err != nil {
		a.internalError(w, "create registered account", err)
		return
	}

	expiresAt := time.Now().UTC().Add(a.accountRegistration.EmailVerificationTTL)
	_, rawToken, err := a.repo.CreateAccountVerificationToken(r.Context(), auth.CreateAccountVerificationTokenParams{
		AccountID: account.ID,
		Purpose:   auth.VerificationPurposeEmail,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		a.internalError(w, "create account verification token", err)
		return
	}
	a.sendVerificationEmail(r, emailAddress, rawToken, expiresAt)
	writeRegistrationAccepted(w)
}

func (a *API) resendPendingRegistrationEmail(w http.ResponseWriter, r *http.Request, username, emailAddress string) {
	account, err := a.repo.GetAccountByUsername(r.Context(), username)
	if errors.Is(err, auth.ErrNotFound) {
		writeRegistrationAccepted(w)
		return
	}
	if err != nil {
		a.internalError(w, "lookup duplicate registered account", err)
		return
	}
	if account.AccountState != auth.AccountStatePendingEmailVerification || account.EmailNormalized != emailAddress {
		writeRegistrationAccepted(w)
		return
	}
	expiresAt := time.Now().UTC().Add(a.accountRegistration.EmailVerificationTTL)
	_, rawToken, err := a.repo.CreateAccountVerificationToken(r.Context(), auth.CreateAccountVerificationTokenParams{
		AccountID: account.ID,
		Purpose:   auth.VerificationPurposeEmail,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		a.internalError(w, "create duplicate account verification token", err)
		return
	}
	a.sendVerificationEmail(r, emailAddress, rawToken, expiresAt)
	writeRegistrationAccepted(w)
}

func (a *API) verifyAccountEmail(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	rawToken := strings.TrimSpace(request.Token)
	if rawToken == "" {
		writeInvalidVerificationToken(w)
		return
	}
	if _, err := a.repo.ConsumeAccountVerificationToken(r.Context(), rawToken, auth.VerificationPurposeEmail, time.Now().UTC()); errors.Is(err, auth.ErrNotFound) {
		writeInvalidVerificationToken(w)
		return
	} else if err != nil {
		a.internalError(w, "consume account verification token", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}

func (a *API) sendVerificationEmail(r *http.Request, emailAddress, rawToken string, expiresAt time.Time) bool {
	if a.emailSender == nil {
		a.logInternalError("send verification email", email.ErrDisabled)
		return false
	}
	link := a.verificationLink(rawToken)
	if link == "" {
		a.logInternalError("build verification email link", email.ErrDisabled)
		return false
	}
	message := email.Message{
		To:      emailAddress,
		Subject: "Verify your Proofline account",
		Body: strings.Join([]string{
			"Verify your Proofline account by opening this link:",
			"",
			link,
			"",
			"This link expires at " + expiresAt.UTC().Format(time.RFC3339) + ".",
			"If you did not create this account, ignore this email.",
			"",
		}, "\n"),
	}
	if err := a.emailSender.Send(r.Context(), message); err != nil {
		a.logInternalError("send verification email", err)
		return false
	}
	return true
}

func (a *API) verificationLink(rawToken string) string {
	origin := strings.TrimRight(a.accountRegistration.PublicWebOrigin, "/")
	if origin == "" || rawToken == "" {
		return ""
	}
	return origin + "/verify-email#token=" + url.QueryEscape(rawToken)
}

func writeRegistrationAccepted(w http.ResponseWriter) {
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "verification_required",
		"message": registrationAcceptedMessage,
	})
}

func writeInvalidVerificationToken(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "verification_token_invalid", "verification token is invalid or expired")
}
