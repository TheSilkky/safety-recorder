package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/open-proofline/server/internal/envelope/pq"
	"github.com/open-proofline/server/internal/incidents"
)

const (
	maxAccountRecipientIDBytes             = 255
	maxAccountRecipientKeyIDBytes          = 255
	maxAccountRecipientSchemeBytes         = 120
	maxAccountRecipientSuiteIDBytes        = 160
	maxAccountRecipientPublicKeyBytes      = 4096
	maxAccountRecipientKeyFingerprintBytes = 256
)

var forbiddenAccountRecipientKeyMaterialMarkers = []string{
	"beginprivatekey",
	"beginecprivatekey",
	"beginrsaprivatekey",
	"privatekey",
	"rawmediakey",
	"contentencryptionkey",
	"mlkemsharedsecret",
	"derivedkek",
	"plaintext",
	"decryptedcache",
}

type createAccountRecipientKeyRequest struct {
	RecipientID          string `json:"recipient_id"`
	RecipientType        string `json:"recipient_type"`
	KeyID                string `json:"key_id"`
	DisplayLabel         string `json:"display_label"`
	Scheme               string `json:"scheme"`
	SuiteID              string `json:"suite_id"`
	PublicKey            string `json:"public_key"`
	PublicKeyFingerprint string `json:"public_key_fingerprint"`
	KeyState             string `json:"key_state"`
}

type updateAccountRecipientKeyRequest struct {
	DisplayLabel *string `json:"display_label"`
	KeyState     *string `json:"key_state"`
}

type replaceAccountRecipientKeyRequest struct {
	KeyID                string `json:"key_id"`
	DisplayLabel         string `json:"display_label"`
	Scheme               string `json:"scheme"`
	SuiteID              string `json:"suite_id"`
	PublicKey            string `json:"public_key"`
	PublicKeyFingerprint string `json:"public_key_fingerprint"`
	KeyState             string `json:"key_state"`
}

func (a *API) createAccountRecipientKey(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	var request createAccountRecipientKeyRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	params, ok := createAccountRecipientKeyParams(w, principal.Account.ID, request)
	if !ok {
		return
	}
	key, err := a.repo.CreateAccountRecipientKey(r.Context(), params)
	if errors.Is(err, incidents.ErrDuplicate) {
		writeError(w, http.StatusConflict, "account_recipient_key_duplicate", "recipient_id or key_id already exists for this account")
		return
	}
	if err != nil {
		a.internalError(w, "create account recipient key", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]incidents.AccountRecipientKey{
		"account_recipient_key": key,
	})
}

func (a *API) listAccountRecipientKeys(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	keys, err := a.repo.ListAccountRecipientKeys(r.Context(), principal.Account.ID)
	if err != nil {
		a.internalError(w, "list account recipient keys", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]incidents.AccountRecipientKey{
		"account_recipient_keys": keys,
	})
}

func (a *API) getAccountRecipientKey(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	key, err := a.repo.GetAccountRecipientKey(r.Context(), principal.Account.ID, r.PathValue("recipient_key_id"))
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "account_recipient_key_not_found", "account recipient key was not found")
		return
	}
	if err != nil {
		a.internalError(w, "get account recipient key", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]incidents.AccountRecipientKey{
		"account_recipient_key": key,
	})
}

func (a *API) updateAccountRecipientKey(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	var request updateAccountRecipientKeyRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	params, ok := updateAccountRecipientKeyParams(w, principal.Account.ID, r.PathValue("recipient_key_id"), request)
	if !ok {
		return
	}
	key, err := a.repo.UpdateAccountRecipientKey(r.Context(), params)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "account_recipient_key_not_found", "account recipient key was not found")
		return
	}
	if errors.Is(err, incidents.ErrInvalidState) {
		writeError(w, http.StatusConflict, "invalid_account_recipient_key_state", "account recipient key state transition is not allowed")
		return
	}
	if err != nil {
		a.internalError(w, "update account recipient key", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]incidents.AccountRecipientKey{
		"account_recipient_key": key,
	})
}

func (a *API) revokeAccountRecipientKey(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	key, err := a.repo.RevokeAccountRecipientKey(r.Context(), principal.Account.ID, r.PathValue("recipient_key_id"))
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "account_recipient_key_not_found", "account recipient key was not found")
		return
	}
	if errors.Is(err, incidents.ErrInvalidState) {
		writeError(w, http.StatusConflict, "invalid_account_recipient_key_state", "account recipient key state transition is not allowed")
		return
	}
	if err != nil {
		a.internalError(w, "revoke account recipient key", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]incidents.AccountRecipientKey{
		"account_recipient_key": key,
	})
}

func (a *API) markAccountRecipientKeyLost(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	key, err := a.repo.MarkAccountRecipientKeyLost(r.Context(), principal.Account.ID, r.PathValue("recipient_key_id"))
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "account_recipient_key_not_found", "account recipient key was not found")
		return
	}
	if errors.Is(err, incidents.ErrInvalidState) {
		writeError(w, http.StatusConflict, "invalid_account_recipient_key_state", "account recipient key state transition is not allowed")
		return
	}
	if err != nil {
		a.internalError(w, "mark account recipient key lost", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]incidents.AccountRecipientKey{
		"account_recipient_key": key,
	})
}

func (a *API) replaceAccountRecipientKey(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	var request replaceAccountRecipientKeyRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	params, ok := replaceAccountRecipientKeyParams(w, principal.Account.ID, r.PathValue("recipient_key_id"), request)
	if !ok {
		return
	}
	key, err := a.repo.ReplaceAccountRecipientKey(r.Context(), params)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "account_recipient_key_not_found", "account recipient key was not found")
		return
	}
	if errors.Is(err, incidents.ErrInvalidState) {
		writeError(w, http.StatusConflict, "invalid_account_recipient_key_state", "account recipient key state transition is not allowed")
		return
	}
	if errors.Is(err, incidents.ErrDuplicate) {
		writeError(w, http.StatusConflict, "account_recipient_key_duplicate", "key_id already exists for this account")
		return
	}
	if err != nil {
		a.internalError(w, "replace account recipient key", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]incidents.AccountRecipientKey{
		"account_recipient_key": key,
	})
}

func createAccountRecipientKeyParams(w http.ResponseWriter, ownerAccountID string, request createAccountRecipientKeyRequest) (incidents.CreateAccountRecipientKeyParams, bool) {
	keyState := strings.TrimSpace(request.KeyState)
	if keyState == "" {
		keyState = incidents.AccountRecipientKeyStatePendingVerification
	}
	params := incidents.CreateAccountRecipientKeyParams{
		OwnerAccountID:       ownerAccountID,
		RecipientID:          strings.TrimSpace(request.RecipientID),
		RecipientType:        strings.TrimSpace(request.RecipientType),
		KeyID:                strings.TrimSpace(request.KeyID),
		DisplayLabel:         strings.TrimSpace(request.DisplayLabel),
		Scheme:               strings.TrimSpace(request.Scheme),
		SuiteID:              strings.TrimSpace(request.SuiteID),
		PublicKey:            strings.TrimSpace(request.PublicKey),
		PublicKeyFingerprint: strings.TrimSpace(request.PublicKeyFingerprint),
		KeyState:             keyState,
	}
	if !validateAccountRecipientKeyCreateMetadata(w, params.RecipientType, params.RecipientID, params.KeyID, params.DisplayLabel, params.Scheme, params.SuiteID, params.PublicKey, params.PublicKeyFingerprint, params.KeyState) {
		return incidents.CreateAccountRecipientKeyParams{}, false
	}
	return params, true
}

func updateAccountRecipientKeyParams(w http.ResponseWriter, ownerAccountID, recipientKeyID string, request updateAccountRecipientKeyRequest) (incidents.UpdateAccountRecipientKeyParams, bool) {
	params := incidents.UpdateAccountRecipientKeyParams{
		OwnerAccountID: ownerAccountID,
		RecipientKeyID: recipientKeyID,
	}
	if request.DisplayLabel != nil {
		displayLabel := strings.TrimSpace(*request.DisplayLabel)
		if len(displayLabel) > maxContactDisplayLabelBytes {
			writeError(w, http.StatusBadRequest, "invalid_display_label", "display_label must be 200 bytes or less")
			return incidents.UpdateAccountRecipientKeyParams{}, false
		}
		params.DisplayLabel = &displayLabel
	}
	if request.KeyState != nil {
		keyState := strings.TrimSpace(*request.KeyState)
		if !incidents.ValidAccountRecipientKeyState(keyState) {
			writeError(w, http.StatusBadRequest, "invalid_key_state", "key_state is not supported")
			return incidents.UpdateAccountRecipientKeyParams{}, false
		}
		params.KeyState = &keyState
	}
	return params, true
}

func replaceAccountRecipientKeyParams(w http.ResponseWriter, ownerAccountID, recipientKeyID string, request replaceAccountRecipientKeyRequest) (incidents.ReplaceAccountRecipientKeyParams, bool) {
	keyState := strings.TrimSpace(request.KeyState)
	if keyState == "" {
		keyState = incidents.AccountRecipientKeyStatePendingVerification
	}
	params := incidents.ReplaceAccountRecipientKeyParams{
		OwnerAccountID:       ownerAccountID,
		RecipientKeyID:       recipientKeyID,
		KeyID:                strings.TrimSpace(request.KeyID),
		DisplayLabel:         strings.TrimSpace(request.DisplayLabel),
		Scheme:               strings.TrimSpace(request.Scheme),
		SuiteID:              strings.TrimSpace(request.SuiteID),
		PublicKey:            strings.TrimSpace(request.PublicKey),
		PublicKeyFingerprint: strings.TrimSpace(request.PublicKeyFingerprint),
		KeyState:             keyState,
	}
	if !validateAccountRecipientKeyMaterial(w, params.KeyID, params.DisplayLabel, params.Scheme, params.SuiteID, params.PublicKey, params.PublicKeyFingerprint, params.KeyState) {
		return incidents.ReplaceAccountRecipientKeyParams{}, false
	}
	return params, true
}

func validateAccountRecipientKeyCreateMetadata(w http.ResponseWriter, recipientType, recipientID, keyID, displayLabel, scheme, suiteID, publicKey, publicKeyFingerprint, keyState string) bool {
	if !incidents.ValidAccountRecipientType(recipientType) {
		writeError(w, http.StatusBadRequest, "invalid_recipient_type", "recipient_type must be account or device")
		return false
	}
	if len(recipientID) > maxAccountRecipientIDBytes {
		writeError(w, http.StatusBadRequest, "invalid_recipient_id", "recipient_id must be 255 bytes or less")
		return false
	}
	return validateAccountRecipientKeyMaterial(w, keyID, displayLabel, scheme, suiteID, publicKey, publicKeyFingerprint, keyState)
}

func validateAccountRecipientKeyMaterial(w http.ResponseWriter, keyID, displayLabel, scheme, suiteID, publicKey, publicKeyFingerprint, keyState string) bool {
	if keyID == "" || len(keyID) > maxAccountRecipientKeyIDBytes {
		writeError(w, http.StatusBadRequest, "invalid_key_id", "key_id is required and must be 255 bytes or less")
		return false
	}
	if len(displayLabel) > maxContactDisplayLabelBytes {
		writeError(w, http.StatusBadRequest, "invalid_display_label", "display_label must be 200 bytes or less")
		return false
	}
	if scheme == "" || len(scheme) > maxAccountRecipientSchemeBytes {
		writeError(w, http.StatusBadRequest, "invalid_scheme", "scheme is required and must be 120 bytes or less")
		return false
	}
	if suiteID == "" || len(suiteID) > maxAccountRecipientSuiteIDBytes {
		writeError(w, http.StatusBadRequest, "invalid_suite_id", "suite_id is required and must be 160 bytes or less")
		return false
	}
	if scheme != pq.SchemeID {
		writeError(w, http.StatusBadRequest, "invalid_scheme", "scheme must use the accepted post-quantum profile")
		return false
	}
	if suiteID != pq.SuiteID {
		writeError(w, http.StatusBadRequest, "invalid_suite_id", "suite_id must use the accepted post-quantum profile")
		return false
	}
	if publicKey == "" || len(publicKey) > maxAccountRecipientPublicKeyBytes || containsForbiddenAccountRecipientKeyMaterial(publicKey) {
		writeError(w, http.StatusBadRequest, "invalid_public_key", "public_key must be public key material only and 4096 bytes or less")
		return false
	}
	if publicKeyFingerprint == "" || len(publicKeyFingerprint) > maxAccountRecipientKeyFingerprintBytes {
		writeError(w, http.StatusBadRequest, "invalid_public_key_fingerprint", "public_key_fingerprint is required and must be 256 bytes or less")
		return false
	}
	if !incidents.ValidAccountRecipientKeyState(keyState) {
		writeError(w, http.StatusBadRequest, "invalid_key_state", "key_state is not supported")
		return false
	}
	if incidents.TerminalAccountRecipientKeyState(keyState) {
		writeError(w, http.StatusBadRequest, "invalid_key_state", "new account recipient keys must start pending_verification or active")
		return false
	}
	return true
}

func containsForbiddenAccountRecipientKeyMaterial(value string) bool {
	normalized := strings.NewReplacer("-", "", "_", "", " ", "", "\n", "", "\r", "", "\t", "").Replace(strings.ToLower(value))
	for _, marker := range forbiddenAccountRecipientKeyMaterialMarkers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}
