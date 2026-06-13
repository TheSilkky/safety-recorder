package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/email"
)

const secondFactorChallengeAcceptedMessage = "If the email challenge can be completed, a verification email will be sent."

type secondFactorResponse struct {
	ID         string     `json:"id"`
	FactorType string     `json:"factor_type"`
	State      string     `json:"state"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
}

func (a *API) requestEmailSecondFactorChallenge(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	var request struct {
		Email string `json:"email,omitempty"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}

	if auth.CanAccessProductRoutes(principal.Account) {
		required, err := a.sessionRequiresSecondFactorVerification(r.Context(), principal.Account, principal.Session)
		if err != nil {
			a.internalError(w, "check email second factor session requirement", err)
			return
		}
		if required {
			a.requestActiveEmailSecondFactorChallenge(w, r, principal)
			return
		}
	}

	emailAddress := auth.NormalizeEmail(request.Email)
	if emailAddress == "" && principal.Account.EmailNormalized != "" && principal.Account.EmailVerifiedAt != nil {
		emailAddress = principal.Account.EmailNormalized
	}
	if emailAddress == "" {
		writeError(w, http.StatusBadRequest, "email_required", "email is required for email second-factor setup")
		return
	}
	if err := auth.ValidateEmail(emailAddress); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_email", err.Error())
		return
	}

	expiresAt := time.Now().UTC().Add(a.secondFactorEmailTTL)
	challenge, rawToken, err := a.repo.CreateEmailSecondFactorChallenge(r.Context(), auth.CreateEmailSecondFactorChallengeParams{
		AccountID:       principal.Account.ID,
		EmailNormalized: emailAddress,
		ExpiresAt:       expiresAt,
	})
	if errors.Is(err, auth.ErrDuplicate) {
		writeError(w, http.StatusConflict, "second_factor_already_configured", "email second factor is already configured")
		return
	}
	if errors.Is(err, auth.ErrNotFound) {
		writeError(w, http.StatusNotFound, "account_not_found", "account was not found")
		return
	}
	if err != nil {
		a.internalError(w, "create email second factor challenge", err)
		return
	}
	if !a.sendSecondFactorChallengeEmail(r, challenge.EmailNormalized, rawToken, challenge.ExpiresAt) {
		writeError(w, http.StatusServiceUnavailable, "email_unavailable", "email delivery is temporarily unavailable")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":     "challenge_sent",
		"message":    secondFactorChallengeAcceptedMessage,
		"expires_at": challenge.ExpiresAt,
	})
}

func (a *API) requestActiveEmailSecondFactorChallenge(w http.ResponseWriter, r *http.Request, principal privatePrincipal) {
	expiresAt := time.Now().UTC().Add(a.secondFactorEmailTTL)
	challenge, rawToken, err := a.repo.CreateActiveEmailSecondFactorChallenge(r.Context(), principal.Account.ID, expiresAt)
	if errors.Is(err, auth.ErrNotFound) {
		writeError(w, http.StatusForbidden, "second_factor_verification_required", "second factor verification is required before account access")
		return
	}
	if err != nil {
		a.internalError(w, "create active email second factor challenge", err)
		return
	}
	if !a.sendSecondFactorChallengeEmail(r, challenge.EmailNormalized, rawToken, challenge.ExpiresAt) {
		writeError(w, http.StatusServiceUnavailable, "email_unavailable", "email delivery is temporarily unavailable")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":     "challenge_sent",
		"message":    secondFactorChallengeAcceptedMessage,
		"expires_at": challenge.ExpiresAt,
	})
}

func (a *API) verifyEmailSecondFactorChallenge(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	var request struct {
		Code  string `json:"code,omitempty"`
		Token string `json:"token,omitempty"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	rawToken := strings.TrimSpace(request.Code)
	if rawToken == "" {
		rawToken = strings.TrimSpace(request.Token)
	}
	if rawToken == "" {
		writeInvalidSecondFactorChallenge(w)
		return
	}

	factor, account, err := a.repo.ConsumeEmailSecondFactorChallenge(r.Context(), principal.Account.ID, rawToken, time.Now().UTC())
	if errors.Is(err, auth.ErrNotFound) {
		writeInvalidSecondFactorChallenge(w)
		return
	}
	if err != nil {
		a.internalError(w, "consume email second factor challenge", err)
		return
	}
	session, err := a.repo.MarkSessionSecondFactorVerified(r.Context(), principal.Session.ID, factor.ID, auth.SecondFactorTypeEmailChallenge, time.Now().UTC())
	if err != nil {
		a.internalError(w, "mark email second factor session verified", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "verified",
		"second_factor": makeSecondFactorResponse(factor),
		"account":       makeAccountResponse(account),
		"session":       makeSecondFactorSessionResponse(session),
	})
}

func (a *API) sendSecondFactorChallengeEmail(r *http.Request, emailAddress, rawToken string, expiresAt time.Time) bool {
	if a.emailSender == nil {
		a.logInternalError("send email second factor challenge", email.ErrDisabled)
		return false
	}
	message := email.NewEmailChallengeMessage(email.EmailChallengeTemplateData{
		To:        emailAddress,
		Code:      rawToken,
		ExpiresAt: expiresAt,
	})
	if err := a.emailSender.Send(r.Context(), message); err != nil {
		a.logInternalError("send email second factor challenge", err)
		return false
	}
	return true
}

func makeSecondFactorResponse(factor auth.SecondFactor) secondFactorResponse {
	return secondFactorResponse{
		ID:         factor.ID,
		FactorType: factor.FactorType,
		State:      factor.FactorState,
		VerifiedAt: factor.VerifiedAt,
	}
}

func writeInvalidSecondFactorChallenge(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "second_factor_challenge_invalid", "second factor challenge is invalid or expired")
}
