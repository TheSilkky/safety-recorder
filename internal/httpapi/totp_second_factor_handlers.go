package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/open-proofline/server/internal/auth"
)

type totpEnrollmentResponse struct {
	ID            string `json:"id"`
	FactorType    string `json:"factor_type"`
	State         string `json:"state"`
	Secret        string `json:"secret"`
	OTPAuthURL    string `json:"otpauth_url"`
	Issuer        string `json:"issuer"`
	AccountName   string `json:"account_name"`
	PeriodSeconds int    `json:"period_seconds"`
	Digits        int    `json:"digits"`
	Algorithm     string `json:"algorithm"`
}

type secondFactorSessionResponse struct {
	SessionID              string     `json:"session_id"`
	SecondFactorVerifiedAt *time.Time `json:"second_factor_verified_at,omitempty"`
	SecondFactorMethod     string     `json:"second_factor_method,omitempty"`
}

func (a *API) startTOTPSecondFactorEnrollment(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	var request struct{}
	if !decodeJSON(w, r, &request) {
		return
	}

	enrollment, err := auth.GenerateTOTPEnrollment(principal.Account.Username)
	if err != nil {
		a.internalError(w, "generate TOTP enrollment", err)
		return
	}
	factor, err := a.repo.CreateTOTPSecondFactorEnrollment(r.Context(), auth.CreateTOTPSecondFactorEnrollmentParams{
		AccountID:     principal.Account.ID,
		Secret:        enrollment.Secret,
		PeriodSeconds: enrollment.PeriodSeconds,
		Digits:        enrollment.Digits,
		Algorithm:     enrollment.Algorithm,
	})
	if errors.Is(err, auth.ErrDuplicate) {
		writeError(w, http.StatusConflict, "second_factor_already_configured", "TOTP second factor is already configured")
		return
	}
	if errors.Is(err, auth.ErrNotFound) {
		writeError(w, http.StatusNotFound, "account_not_found", "account was not found")
		return
	}
	if err != nil {
		a.internalError(w, "create TOTP enrollment", err)
		return
	}

	writeJSON(w, http.StatusCreated, totpEnrollmentResponse{
		ID:            factor.ID,
		FactorType:    factor.FactorType,
		State:         factor.FactorState,
		Secret:        enrollment.Secret,
		OTPAuthURL:    enrollment.URL,
		Issuer:        auth.TOTPIssuer,
		AccountName:   principal.Account.Username,
		PeriodSeconds: enrollment.PeriodSeconds,
		Digits:        enrollment.Digits,
		Algorithm:     enrollment.Algorithm,
	})
}

func (a *API) confirmTOTPSecondFactorEnrollment(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	code, ok := decodeTOTPCode(w, r)
	if !ok {
		return
	}

	factor, err := a.repo.GetPendingTOTPSecondFactor(r.Context(), principal.Account.ID)
	if errors.Is(err, auth.ErrNotFound) {
		writeInvalidTOTPChallenge(w)
		return
	}
	if err != nil {
		a.internalError(w, "get pending TOTP factor", err)
		return
	}
	now := time.Now().UTC()
	timeStep, valid, err := auth.MatchTOTPCode(factor.TOTPSecret, code, now, factor.TOTPPeriodSeconds, factor.TOTPDigits, factor.TOTPAlgorithm)
	if err != nil {
		a.internalError(w, "validate pending TOTP code", err)
		return
	}
	if !valid {
		writeInvalidTOTPChallenge(w)
		return
	}

	factor, account, err := a.repo.ActivateTOTPSecondFactor(r.Context(), principal.Account.ID, factor.ID, now, timeStep)
	if errors.Is(err, auth.ErrNotFound) {
		writeInvalidTOTPChallenge(w)
		return
	}
	if err != nil {
		a.internalError(w, "activate TOTP factor", err)
		return
	}
	session, err := a.repo.MarkSessionSecondFactorVerified(r.Context(), principal.Session.ID, factor.ID, auth.SecondFactorTypeTOTP, now)
	if err != nil {
		a.internalError(w, "mark TOTP setup session verified", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "verified",
		"second_factor": makeSecondFactorResponse(factor),
		"account":       makeAccountResponse(account),
		"session":       makeSecondFactorSessionResponse(session),
	})
}

func (a *API) verifyTOTPSecondFactorChallenge(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	code, ok := decodeTOTPCode(w, r)
	if !ok {
		return
	}

	factor, err := a.repo.GetActiveTOTPSecondFactor(r.Context(), principal.Account.ID)
	if errors.Is(err, auth.ErrNotFound) {
		writeInvalidTOTPChallenge(w)
		return
	}
	if err != nil {
		a.internalError(w, "get active TOTP factor", err)
		return
	}
	now := time.Now().UTC()
	timeStep, valid, err := auth.MatchTOTPCode(factor.TOTPSecret, code, now, factor.TOTPPeriodSeconds, factor.TOTPDigits, factor.TOTPAlgorithm)
	if err != nil {
		a.internalError(w, "validate active TOTP code", err)
		return
	}
	if !valid {
		writeInvalidTOTPChallenge(w)
		return
	}
	factor, err = a.repo.MarkTOTPSecondFactorUsed(r.Context(), factor.ID, now, timeStep)
	if errors.Is(err, auth.ErrNotFound) {
		writeInvalidTOTPChallenge(w)
		return
	}
	if err != nil {
		a.internalError(w, "mark TOTP factor used", err)
		return
	}
	session, err := a.repo.MarkSessionSecondFactorVerified(r.Context(), principal.Session.ID, factor.ID, auth.SecondFactorTypeTOTP, now)
	if err != nil {
		a.internalError(w, "mark TOTP challenge session verified", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "verified",
		"second_factor": makeSecondFactorResponse(factor),
		"session":       makeSecondFactorSessionResponse(session),
	})
}

func (a *API) sessionRequiresSecondFactorVerification(rctx context.Context, account auth.Account, session auth.Session) (bool, error) {
	if session.SecondFactorVerifiedAt != nil {
		return false, nil
	}
	if !auth.CanAccessProductRoutes(account) {
		return false, nil
	}
	_, err := a.repo.GetActiveTOTPSecondFactor(rctx, account.ID)
	if errors.Is(err, auth.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func decodeTOTPCode(w http.ResponseWriter, r *http.Request) (string, bool) {
	var request struct {
		Code string `json:"code,omitempty"`
	}
	if !decodeJSON(w, r, &request) {
		return "", false
	}
	code := strings.TrimSpace(request.Code)
	if code == "" {
		writeInvalidTOTPChallenge(w)
		return "", false
	}
	return code, true
}

func makeSecondFactorSessionResponse(session auth.Session) secondFactorSessionResponse {
	return secondFactorSessionResponse{
		SessionID:              session.ID,
		SecondFactorVerifiedAt: session.SecondFactorVerifiedAt,
		SecondFactorMethod:     session.SecondFactorMethod,
	}
}

func writeInvalidTOTPChallenge(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "totp_challenge_invalid", "TOTP challenge is invalid or expired")
}
