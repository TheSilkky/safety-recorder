package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/open-proofline/server/internal/incidents"
)

const (
	maxContactDisplayLabelBytes         = 200
	maxWrappingAlgorithmBytes           = 80
	maxContactPublicKeyBytes            = 4096
	maxContactPublicKeyFingerprintBytes = 256
)

type createContactPublicKeyRequest struct {
	ContactID            string `json:"contact_id"`
	DisplayLabel         string `json:"display_label"`
	WrappingAlgorithm    string `json:"wrapping_algorithm"`
	PublicKey            string `json:"public_key"`
	PublicKeyFingerprint string `json:"public_key_fingerprint"`
	KeyState             string `json:"key_state"`
}

type updateContactPublicKeyRequest struct {
	DisplayLabel *string `json:"display_label"`
	KeyState     *string `json:"key_state"`
}

type createSharingGrantRequest struct {
	StreamID           string     `json:"stream_id"`
	RecipientType      string     `json:"recipient_type"`
	ContactID          string     `json:"contact_id"`
	ContactPublicKeyID string     `json:"contact_public_key_id"`
	DataClass          string     `json:"data_class"`
	ExpiresAt          *time.Time `json:"expires_at"`
}

func (a *API) createContactPublicKey(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	var request createContactPublicKeyRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	params, ok := createContactPublicKeyParams(w, principal.Account.ID, request)
	if !ok {
		return
	}
	contactKey, err := a.repo.CreateContactPublicKey(r.Context(), params)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "contact_not_found", "contact was not found")
		return
	}
	if err != nil {
		a.internalError(w, "create contact public key", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]incidents.ContactPublicKey{
		"contact_public_key": contactKey,
	})
}

func (a *API) listContactPublicKeys(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	contactKeys, err := a.repo.ListContactPublicKeys(r.Context(), principal.Account.ID)
	if err != nil {
		a.internalError(w, "list contact public keys", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]incidents.ContactPublicKey{
		"contact_public_keys": contactKeys,
	})
}

func (a *API) getContactPublicKey(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	contactKey, err := a.repo.GetContactPublicKey(r.Context(), principal.Account.ID, r.PathValue("public_key_id"))
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "contact_public_key_not_found", "contact public key was not found")
		return
	}
	if err != nil {
		a.internalError(w, "get contact public key", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]incidents.ContactPublicKey{
		"contact_public_key": contactKey,
	})
}

func (a *API) updateContactPublicKey(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	var request updateContactPublicKeyRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	params, ok := updateContactPublicKeyParams(w, principal.Account.ID, r.PathValue("public_key_id"), request)
	if !ok {
		return
	}
	contactKey, err := a.repo.UpdateContactPublicKey(r.Context(), params)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "contact_public_key_not_found", "contact public key was not found")
		return
	}
	if errors.Is(err, incidents.ErrInvalidState) {
		writeError(w, http.StatusConflict, "invalid_contact_key_state", "revoked contact keys cannot be reactivated")
		return
	}
	if err != nil {
		a.internalError(w, "update contact public key", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]incidents.ContactPublicKey{
		"contact_public_key": contactKey,
	})
}

func (a *API) revokeContactPublicKey(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	contactKey, err := a.repo.RevokeContactPublicKey(r.Context(), principal.Account.ID, r.PathValue("public_key_id"))
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "contact_public_key_not_found", "contact public key was not found")
		return
	}
	if err != nil {
		a.internalError(w, "revoke contact public key", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]incidents.ContactPublicKey{
		"contact_public_key": contactKey,
	})
}

func (a *API) createSharingGrant(w http.ResponseWriter, r *http.Request) {
	incidentID := r.PathValue("incident_id")
	incident, ok := a.authorizeOwnedIncident(w, r, incidentID, actionCreateSharingGrant, dataClassSharingGrant)
	if !ok {
		return
	}
	var request createSharingGrantRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	params, ok := createSharingGrantParams(w, incident.OwnerAccountID, incident.ID, request)
	if !ok {
		return
	}
	grant, err := a.repo.CreateSharingGrant(r.Context(), params)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "sharing_grant_dependency_not_found", "incident, stream, or active contact public key was not found")
		return
	}
	if err != nil {
		a.internalError(w, "create sharing grant", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]incidents.SharingGrant{
		"sharing_grant": grant,
	})
}

func (a *API) listSharingGrants(w http.ResponseWriter, r *http.Request) {
	incidentID := r.PathValue("incident_id")
	incident, ok := a.authorizeOwnedIncident(w, r, incidentID, actionReadSharingGrant, dataClassSharingGrant)
	if !ok {
		return
	}
	grants, err := a.repo.ListSharingGrants(r.Context(), incident.OwnerAccountID, incident.ID)
	if err != nil {
		a.internalError(w, "list sharing grants", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]incidents.SharingGrant{
		"sharing_grants": grants,
	})
}

func (a *API) getSharingGrant(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	grant, err := a.repo.GetSharingGrant(r.Context(), principal.Account.ID, r.PathValue("grant_id"))
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "sharing_grant_not_found", "sharing grant was not found")
		return
	}
	if err != nil {
		a.internalError(w, "get sharing grant", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]incidents.SharingGrant{
		"sharing_grant": grant,
	})
}

func (a *API) revokeSharingGrant(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	grant, err := a.repo.RevokeSharingGrant(r.Context(), principal.Account.ID, r.PathValue("grant_id"), principal.Account.ID)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "sharing_grant_not_found", "sharing grant was not found")
		return
	}
	if err != nil {
		a.internalError(w, "revoke sharing grant", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]incidents.SharingGrant{
		"sharing_grant": grant,
	})
}

func createContactPublicKeyParams(w http.ResponseWriter, ownerAccountID string, request createContactPublicKeyRequest) (incidents.CreateContactPublicKeyParams, bool) {
	keyState := strings.TrimSpace(request.KeyState)
	if keyState == "" {
		keyState = incidents.ContactKeyStatePendingVerification
	}
	params := incidents.CreateContactPublicKeyParams{
		OwnerAccountID:       ownerAccountID,
		ContactID:            strings.TrimSpace(request.ContactID),
		DisplayLabel:         strings.TrimSpace(request.DisplayLabel),
		WrappingAlgorithm:    strings.TrimSpace(request.WrappingAlgorithm),
		PublicKey:            strings.TrimSpace(request.PublicKey),
		PublicKeyFingerprint: strings.TrimSpace(request.PublicKeyFingerprint),
		KeyState:             keyState,
	}
	if params.WrappingAlgorithm == "" || len(params.WrappingAlgorithm) > maxWrappingAlgorithmBytes {
		writeError(w, http.StatusBadRequest, "invalid_wrapping_algorithm", "wrapping_algorithm is required and must be 80 bytes or less")
		return incidents.CreateContactPublicKeyParams{}, false
	}
	if params.PublicKey == "" || len(params.PublicKey) > maxContactPublicKeyBytes {
		writeError(w, http.StatusBadRequest, "invalid_public_key", "public_key is required and must be 4096 bytes or less")
		return incidents.CreateContactPublicKeyParams{}, false
	}
	if params.PublicKeyFingerprint == "" || len(params.PublicKeyFingerprint) > maxContactPublicKeyFingerprintBytes {
		writeError(w, http.StatusBadRequest, "invalid_public_key_fingerprint", "public_key_fingerprint is required and must be 256 bytes or less")
		return incidents.CreateContactPublicKeyParams{}, false
	}
	if len(params.DisplayLabel) > maxContactDisplayLabelBytes {
		writeError(w, http.StatusBadRequest, "invalid_display_label", "display_label must be 200 bytes or less")
		return incidents.CreateContactPublicKeyParams{}, false
	}
	if !incidents.ValidContactKeyState(params.KeyState) {
		writeError(w, http.StatusBadRequest, "invalid_key_state", "key_state is not supported")
		return incidents.CreateContactPublicKeyParams{}, false
	}
	return params, true
}

func updateContactPublicKeyParams(w http.ResponseWriter, ownerAccountID, publicKeyID string, request updateContactPublicKeyRequest) (incidents.UpdateContactPublicKeyParams, bool) {
	params := incidents.UpdateContactPublicKeyParams{
		OwnerAccountID: ownerAccountID,
		PublicKeyID:    publicKeyID,
	}
	if request.DisplayLabel != nil {
		displayLabel := strings.TrimSpace(*request.DisplayLabel)
		if len(displayLabel) > maxContactDisplayLabelBytes {
			writeError(w, http.StatusBadRequest, "invalid_display_label", "display_label must be 200 bytes or less")
			return incidents.UpdateContactPublicKeyParams{}, false
		}
		params.DisplayLabel = &displayLabel
	}
	if request.KeyState != nil {
		keyState := strings.TrimSpace(*request.KeyState)
		if !incidents.ValidContactKeyState(keyState) {
			writeError(w, http.StatusBadRequest, "invalid_key_state", "key_state is not supported")
			return incidents.UpdateContactPublicKeyParams{}, false
		}
		params.KeyState = &keyState
	}
	return params, true
}

func createSharingGrantParams(w http.ResponseWriter, ownerAccountID, incidentID string, request createSharingGrantRequest) (incidents.CreateSharingGrantParams, bool) {
	recipientType := strings.TrimSpace(request.RecipientType)
	if recipientType == "" {
		recipientType = incidents.SharingGrantRecipientTrustedContact
	}
	dataClass := strings.TrimSpace(request.DataClass)
	if dataClass == "" {
		dataClass = incidents.SharingGrantDataClassMetadataCiphertext
	}
	params := incidents.CreateSharingGrantParams{
		OwnerAccountID:     ownerAccountID,
		IncidentID:         incidentID,
		StreamID:           strings.TrimSpace(request.StreamID),
		RecipientType:      recipientType,
		ContactID:          strings.TrimSpace(request.ContactID),
		ContactPublicKeyID: strings.TrimSpace(request.ContactPublicKeyID),
		DataClass:          dataClass,
		ExpiresAt:          request.ExpiresAt,
	}
	if !incidents.ValidSharingGrantRecipientType(params.RecipientType) {
		writeError(w, http.StatusBadRequest, "invalid_recipient_type", "recipient_type is not supported")
		return incidents.CreateSharingGrantParams{}, false
	}
	if params.ContactID == "" {
		writeError(w, http.StatusBadRequest, "invalid_contact_id", "contact_id is required")
		return incidents.CreateSharingGrantParams{}, false
	}
	if !incidents.ValidSharingGrantDataClass(params.DataClass) {
		writeError(w, http.StatusBadRequest, "invalid_data_class", "data_class is not supported")
		return incidents.CreateSharingGrantParams{}, false
	}
	if params.ExpiresAt != nil && !params.ExpiresAt.After(time.Now().UTC()) {
		writeError(w, http.StatusBadRequest, "invalid_expires_at", "expires_at must be in the future")
		return incidents.CreateSharingGrantParams{}, false
	}
	return params, true
}
