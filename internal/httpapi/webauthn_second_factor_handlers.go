package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/open-proofline/server/internal/auth"
)

type webAuthnRegistrationChallengeResponse struct {
	Status                  string                       `json:"status"`
	CredentialCreation      *protocol.CredentialCreation `json:"credential_creation"`
	ExpiresAt               time.Time                    `json:"expires_at"`
	AuthenticatorAttachment string                       `json:"authenticator_attachment,omitempty"`
}

type webAuthnAssertionChallengeResponse struct {
	Status              string                        `json:"status"`
	CredentialAssertion *protocol.CredentialAssertion `json:"credential_assertion"`
	ExpiresAt           time.Time                     `json:"expires_at"`
}

type webAuthnCredentialResponse struct {
	ID             string     `json:"id"`
	FactorType     string     `json:"factor_type"`
	Attachment     string     `json:"attachment,omitempty"`
	Transports     []string   `json:"transports,omitempty"`
	BackupEligible bool       `json:"backup_eligible"`
	BackupState    bool       `json:"backup_state"`
	UserVerified   bool       `json:"user_verified"`
	CloneWarning   bool       `json:"clone_warning"`
	VerifiedAt     *time.Time `json:"verified_at,omitempty"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
}

func (a *API) startWebAuthnRegistration(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	var request struct {
		AuthenticatorAttachment string `json:"authenticator_attachment,omitempty"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	attachment, ok := normalizeWebAuthnAttachment(w, request.AuthenticatorAttachment)
	if !ok {
		return
	}
	provider, uv, ok := a.webAuthnProvider(w)
	if !ok {
		return
	}

	webauthnUser, err := a.repo.GetOrCreateWebAuthnUser(r.Context(), principal.Account.ID, a.webAuthn.RPID)
	if errors.Is(err, auth.ErrNotFound) {
		writeError(w, http.StatusNotFound, "account_not_found", "account was not found")
		return
	}
	if err != nil {
		a.internalError(w, "get or create WebAuthn user", err)
		return
	}
	credentials, err := a.repo.ListWebAuthnCredentials(r.Context(), principal.Account.ID, a.webAuthn.RPID)
	if err != nil {
		a.internalError(w, "list WebAuthn credentials", err)
		return
	}
	user := auth.WebAuthnAccountUser{
		Account:     principal.Account,
		User:        webauthnUser,
		Credentials: credentials,
	}
	selection := protocol.AuthenticatorSelection{
		UserVerification:   uv,
		ResidentKey:        protocol.ResidentKeyRequirementPreferred,
		RequireResidentKey: protocol.ResidentKeyNotRequired(),
	}
	hints := []protocol.PublicKeyCredentialHints{}
	if attachment != "" {
		selection.AuthenticatorAttachment = protocol.AuthenticatorAttachment(attachment)
		switch attachment {
		case string(protocol.Platform):
			hints = append(hints, protocol.PublicKeyCredentialHintClientDevice)
		case string(protocol.CrossPlatform):
			hints = append(hints, protocol.PublicKeyCredentialHintSecurityKey)
		}
	}
	opts := []webauthn.RegistrationOption{
		webauthn.WithAuthenticatorSelection(selection),
		webauthn.WithExclusions(webauthn.Credentials(user.WebAuthnCredentials()).CredentialDescriptors()),
	}
	if len(hints) > 0 {
		opts = append(opts, webauthn.WithPublicKeyCredentialHints(hints))
	}
	creation, session, err := provider.BeginRegistration(user, opts...)
	if err != nil {
		a.internalError(w, "begin WebAuthn registration", err)
		return
	}
	expiresAt := time.Now().UTC().Add(a.webAuthn.ChallengeTTL)
	session.Expires = expiresAt
	sessionData, err := json.Marshal(session)
	if err != nil {
		a.internalError(w, "marshal WebAuthn registration session", err)
		return
	}
	challenge, err := a.repo.CreateWebAuthnChallenge(r.Context(), auth.CreateWebAuthnChallengeParams{
		AccountID:       principal.Account.ID,
		SessionID:       principal.Session.ID,
		RPID:            a.webAuthn.RPID,
		ChallengeType:   auth.SecondFactorChallengeTypeWebAuthnRegistration,
		SessionDataJSON: sessionData,
		ExpiresAt:       expiresAt,
	})
	if errors.Is(err, auth.ErrNotFound) {
		writeError(w, http.StatusNotFound, "account_not_found", "account was not found")
		return
	}
	if err != nil {
		a.internalError(w, "store WebAuthn registration challenge", err)
		return
	}

	writeJSON(w, http.StatusCreated, webAuthnRegistrationChallengeResponse{
		Status:                  "registration_challenge_created",
		CredentialCreation:      creation,
		ExpiresAt:               challenge.ExpiresAt,
		AuthenticatorAttachment: attachment,
	})
}

func (a *API) finishWebAuthnRegistration(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	provider, _, ok := a.webAuthnProvider(w)
	if !ok {
		return
	}
	now := time.Now().UTC()
	challenge, err := a.repo.ConsumeWebAuthnChallenge(r.Context(), principal.Account.ID, principal.Session.ID, a.webAuthn.RPID, auth.SecondFactorChallengeTypeWebAuthnRegistration, now)
	if errors.Is(err, auth.ErrNotFound) {
		writeInvalidWebAuthnChallenge(w)
		return
	}
	if err != nil {
		a.internalError(w, "consume WebAuthn registration challenge", err)
		return
	}
	session, ok := decodeWebAuthnSession(w, challenge.SessionDataJSON)
	if !ok {
		return
	}
	webauthnUser, err := a.repo.GetOrCreateWebAuthnUser(r.Context(), principal.Account.ID, a.webAuthn.RPID)
	if err != nil {
		a.internalError(w, "get WebAuthn registration user", err)
		return
	}
	credentials, err := a.repo.ListWebAuthnCredentials(r.Context(), principal.Account.ID, a.webAuthn.RPID)
	if err != nil {
		a.internalError(w, "list WebAuthn registration credentials", err)
		return
	}
	user := auth.WebAuthnAccountUser{
		Account:     principal.Account,
		User:        webauthnUser,
		Credentials: credentials,
	}
	r.Body = http.MaxBytesReader(w, r.Body, jsonBodyLimit)
	credential, err := provider.FinishRegistration(user, session, r)
	if err != nil {
		writeInvalidWebAuthnChallenge(w)
		return
	}
	stored, account, err := a.repo.CreateWebAuthnCredential(r.Context(), auth.WebAuthnCredentialParamsFromLibrary(principal.Account.ID, a.webAuthn.RPID, credential, now))
	if errors.Is(err, auth.ErrDuplicate) {
		writeError(w, http.StatusConflict, "webauthn_credential_already_configured", "WebAuthn credential is already configured")
		return
	}
	if errors.Is(err, auth.ErrNotFound) {
		writeInvalidWebAuthnChallenge(w)
		return
	}
	if err != nil {
		a.internalError(w, "store WebAuthn credential", err)
		return
	}
	sessionResult, err := a.repo.MarkSessionSecondFactorVerified(r.Context(), principal.Session.ID, stored.ID, auth.SecondFactorTypeWebAuthn, now)
	if err != nil {
		a.internalError(w, "mark WebAuthn setup session verified", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "verified",
		"second_factor": makeWebAuthnCredentialResponse(stored),
		"account":       makeAccountResponse(account),
		"session":       makeSecondFactorSessionResponse(sessionResult),
	})
}

func (a *API) startWebAuthnVerification(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	var request struct{}
	if !decodeJSON(w, r, &request) {
		return
	}
	provider, uv, ok := a.webAuthnProvider(w)
	if !ok {
		return
	}
	webauthnUser, err := a.repo.GetOrCreateWebAuthnUser(r.Context(), principal.Account.ID, a.webAuthn.RPID)
	if errors.Is(err, auth.ErrNotFound) {
		writeInvalidWebAuthnChallenge(w)
		return
	}
	if err != nil {
		a.internalError(w, "get WebAuthn assertion user", err)
		return
	}
	credentials, err := a.repo.ListWebAuthnCredentials(r.Context(), principal.Account.ID, a.webAuthn.RPID)
	if err != nil {
		a.internalError(w, "list WebAuthn assertion credentials", err)
		return
	}
	if len(credentials) == 0 {
		writeInvalidWebAuthnChallenge(w)
		return
	}
	user := auth.WebAuthnAccountUser{
		Account:     principal.Account,
		User:        webauthnUser,
		Credentials: credentials,
	}
	assertion, session, err := provider.BeginLogin(user, webauthn.WithUserVerification(uv))
	if err != nil {
		writeInvalidWebAuthnChallenge(w)
		return
	}
	expiresAt := time.Now().UTC().Add(a.webAuthn.ChallengeTTL)
	session.Expires = expiresAt
	sessionData, err := json.Marshal(session)
	if err != nil {
		a.internalError(w, "marshal WebAuthn assertion session", err)
		return
	}
	challenge, err := a.repo.CreateWebAuthnChallenge(r.Context(), auth.CreateWebAuthnChallengeParams{
		AccountID:       principal.Account.ID,
		SessionID:       principal.Session.ID,
		RPID:            a.webAuthn.RPID,
		ChallengeType:   auth.SecondFactorChallengeTypeWebAuthnAssertion,
		SessionDataJSON: sessionData,
		ExpiresAt:       expiresAt,
	})
	if errors.Is(err, auth.ErrNotFound) {
		writeInvalidWebAuthnChallenge(w)
		return
	}
	if err != nil {
		a.internalError(w, "store WebAuthn assertion challenge", err)
		return
	}

	writeJSON(w, http.StatusCreated, webAuthnAssertionChallengeResponse{
		Status:              "verification_challenge_created",
		CredentialAssertion: assertion,
		ExpiresAt:           challenge.ExpiresAt,
	})
}

func (a *API) finishWebAuthnVerification(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	provider, _, ok := a.webAuthnProvider(w)
	if !ok {
		return
	}
	now := time.Now().UTC()
	challenge, err := a.repo.ConsumeWebAuthnChallenge(r.Context(), principal.Account.ID, principal.Session.ID, a.webAuthn.RPID, auth.SecondFactorChallengeTypeWebAuthnAssertion, now)
	if errors.Is(err, auth.ErrNotFound) {
		writeInvalidWebAuthnChallenge(w)
		return
	}
	if err != nil {
		a.internalError(w, "consume WebAuthn assertion challenge", err)
		return
	}
	session, ok := decodeWebAuthnSession(w, challenge.SessionDataJSON)
	if !ok {
		return
	}
	webauthnUser, err := a.repo.GetOrCreateWebAuthnUser(r.Context(), principal.Account.ID, a.webAuthn.RPID)
	if errors.Is(err, auth.ErrNotFound) {
		writeInvalidWebAuthnChallenge(w)
		return
	}
	if err != nil {
		a.internalError(w, "get WebAuthn verification user", err)
		return
	}
	credentials, err := a.repo.ListWebAuthnCredentials(r.Context(), principal.Account.ID, a.webAuthn.RPID)
	if err != nil {
		a.internalError(w, "list WebAuthn verification credentials", err)
		return
	}
	byCredentialID := webAuthnCredentialMap(credentials)
	user := auth.WebAuthnAccountUser{
		Account:     principal.Account,
		User:        webauthnUser,
		Credentials: credentials,
	}
	r.Body = http.MaxBytesReader(w, r.Body, jsonBodyLimit)
	credential, err := provider.FinishLogin(user, session, r)
	if err != nil {
		writeInvalidWebAuthnChallenge(w)
		return
	}
	stored, ok := byCredentialID[string(credential.ID)]
	if !ok {
		writeInvalidWebAuthnChallenge(w)
		return
	}
	updated, err := a.repo.UpdateWebAuthnCredentialAfterAssertion(r.Context(), auth.WebAuthnCredentialUpdateFromLibrary(stored, credential, now))
	if errors.Is(err, auth.ErrNotFound) {
		writeInvalidWebAuthnChallenge(w)
		return
	}
	if err != nil {
		a.internalError(w, "update WebAuthn credential", err)
		return
	}
	sessionResult, err := a.repo.MarkSessionSecondFactorVerified(r.Context(), principal.Session.ID, updated.ID, auth.SecondFactorTypeWebAuthn, now)
	if err != nil {
		a.internalError(w, "mark WebAuthn challenge session verified", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "verified",
		"second_factor": makeWebAuthnCredentialResponse(updated),
		"session":       makeSecondFactorSessionResponse(sessionResult),
	})
}

func (a *API) webAuthnProvider(w http.ResponseWriter) (*webauthn.WebAuthn, protocol.UserVerificationRequirement, bool) {
	if !a.webAuthn.Enabled || a.webAuthn.RPID == "" || len(a.webAuthn.AllowedOrigins) == 0 {
		writeError(w, http.StatusServiceUnavailable, "webauthn_unavailable", "WebAuthn second factor is not configured")
		return nil, "", false
	}
	uv := protocol.UserVerificationRequirement(a.webAuthn.UserVerification)
	switch uv {
	case protocol.VerificationRequired, protocol.VerificationPreferred, protocol.VerificationDiscouraged:
	default:
		writeError(w, http.StatusServiceUnavailable, "webauthn_unavailable", "WebAuthn second factor is not configured")
		return nil, "", false
	}
	provider, err := webauthn.New(&webauthn.Config{
		RPID:          a.webAuthn.RPID,
		RPDisplayName: a.webAuthn.RPDisplayName,
		RPOrigins:     append([]string(nil), a.webAuthn.AllowedOrigins...),
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			UserVerification: uv,
		},
	})
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "webauthn_unavailable", "WebAuthn second factor is not configured")
		return nil, "", false
	}
	return provider, uv, true
}

func normalizeWebAuthnAttachment(w http.ResponseWriter, raw string) (string, bool) {
	switch raw {
	case "":
		return "", true
	case string(protocol.Platform), string(protocol.CrossPlatform):
		return raw, true
	default:
		writeError(w, http.StatusBadRequest, "invalid_webauthn_attachment", "WebAuthn authenticator attachment must be platform or cross-platform")
		return "", false
	}
}

func decodeWebAuthnSession(w http.ResponseWriter, data []byte) (webauthn.SessionData, bool) {
	var session webauthn.SessionData
	if err := json.Unmarshal(data, &session); err != nil {
		writeInvalidWebAuthnChallenge(w)
		return webauthn.SessionData{}, false
	}
	return session, true
}

func webAuthnCredentialMap(credentials []auth.WebAuthnCredential) map[string]auth.WebAuthnCredential {
	byID := make(map[string]auth.WebAuthnCredential, len(credentials))
	for _, credential := range credentials {
		byID[string(credential.CredentialID)] = credential
	}
	return byID
}

func makeWebAuthnCredentialResponse(credential auth.WebAuthnCredential) webAuthnCredentialResponse {
	return webAuthnCredentialResponse{
		ID:             credential.ID,
		FactorType:     auth.SecondFactorTypeWebAuthn,
		Attachment:     credential.Attachment,
		Transports:     append([]string(nil), credential.Transports...),
		BackupEligible: credential.BackupEligible,
		BackupState:    credential.BackupState,
		UserVerified:   credential.UserVerified,
		CloneWarning:   credential.CloneWarning,
		VerifiedAt:     credential.VerifiedAt,
		LastUsedAt:     credential.LastUsedAt,
	}
}

func writeInvalidWebAuthnChallenge(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "webauthn_challenge_invalid", "WebAuthn challenge is invalid or expired")
}
