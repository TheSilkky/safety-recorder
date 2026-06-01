package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/open-proofline/server/internal/incidents"
)

const (
	maxWrappedKeyMediaKeyIDBytes               = 255
	maxWrappedKeyCiphertextBytes               = 16384
	maxWrappedKeyPublicWrappingMetadataBytes   = 4096
	maxWrappedKeyWrappingAlgorithmVersionBytes = 80
)

var forbiddenPublicWrappingMetadataKeys = map[string]struct{}{
	"browserfragmentsecret": {},
	"contactprivatekey":     {},
	"plaintext":             {},
	"privatekey":            {},
	"rawkey":                {},
	"rawmediakey":           {},
	"rawtoken":              {},
	"serverescrowkey":       {},
	"serverescrowmaterial":  {},
	"unwrappedsecret":       {},
	"unwrappedsharedsecret": {},
}

type createWrappedKeyRecordRequest struct {
	StreamID                 string          `json:"stream_id"`
	GrantID                  string          `json:"grant_id"`
	MediaKeyID               string          `json:"media_key_id"`
	WrappingAlgorithm        string          `json:"wrapping_algorithm"`
	WrappingAlgorithmVersion string          `json:"wrapping_algorithm_version"`
	WrappedKeyCiphertext     string          `json:"wrapped_key_ciphertext"`
	PublicWrappingMetadata   json.RawMessage `json:"public_wrapping_metadata"`
}

func (a *API) createWrappedKeyRecord(w http.ResponseWriter, r *http.Request) {
	incidentID := r.PathValue("incident_id")
	incident, ok := a.authorizeOwnedIncident(w, r, incidentID, actionCreateWrappedKey, dataClassWrappedKey)
	if !ok {
		return
	}
	var request createWrappedKeyRecordRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	params, ok := createWrappedKeyRecordParams(w, incident.OwnerAccountID, incident.ID, request)
	if !ok {
		return
	}
	record, err := a.repo.CreateWrappedKeyRecord(r.Context(), params)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "wrapped_key_dependency_not_found", "incident, stream, active grant, or active contact key was not found")
		return
	}
	if errors.Is(err, incidents.ErrInvalidState) {
		writeError(w, http.StatusConflict, "wrapped_key_grant_not_authorized", "sharing grant does not authorize wrapped-key delivery")
		return
	}
	if errors.Is(err, incidents.ErrDuplicate) {
		writeError(w, http.StatusConflict, "wrapped_key_duplicate", "wrapped key record already exists for this grant and media key")
		return
	}
	if err != nil {
		a.internalError(w, "create wrapped key record", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]incidents.WrappedKeyRecord{
		"wrapped_key": record,
	})
}

func (a *API) listWrappedKeyRecords(w http.ResponseWriter, r *http.Request) {
	incidentID := r.PathValue("incident_id")
	incident, ok := a.authorizeOwnedIncident(w, r, incidentID, actionReadWrappedKey, dataClassWrappedKey)
	if !ok {
		return
	}
	records, err := a.repo.ListWrappedKeyRecords(r.Context(), incident.OwnerAccountID, incident.ID)
	if err != nil {
		a.internalError(w, "list wrapped key records", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]incidents.WrappedKeyRecord{
		"wrapped_keys": records,
	})
}

func (a *API) getWrappedKeyRecord(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	record, err := a.repo.GetWrappedKeyRecord(r.Context(), principal.Account.ID, r.PathValue("wrapped_key_id"))
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "wrapped_key_not_found", "wrapped key record was not found")
		return
	}
	if err != nil {
		a.internalError(w, "get wrapped key record", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]incidents.WrappedKeyRecord{
		"wrapped_key": record,
	})
}

func (a *API) revokeWrappedKeyRecord(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	record, err := a.repo.RevokeWrappedKeyRecord(r.Context(), principal.Account.ID, r.PathValue("wrapped_key_id"), principal.Account.ID)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "wrapped_key_not_found", "wrapped key record was not found")
		return
	}
	if err != nil {
		a.internalError(w, "revoke wrapped key record", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]incidents.WrappedKeyRecord{
		"wrapped_key": record,
	})
}

func createWrappedKeyRecordParams(w http.ResponseWriter, ownerAccountID, incidentID string, request createWrappedKeyRecordRequest) (incidents.CreateWrappedKeyRecordParams, bool) {
	params := incidents.CreateWrappedKeyRecordParams{
		OwnerAccountID:           ownerAccountID,
		IncidentID:               incidentID,
		StreamID:                 strings.TrimSpace(request.StreamID),
		GrantID:                  strings.TrimSpace(request.GrantID),
		MediaKeyID:               strings.TrimSpace(request.MediaKeyID),
		WrappingAlgorithm:        strings.TrimSpace(request.WrappingAlgorithm),
		WrappingAlgorithmVersion: strings.TrimSpace(request.WrappingAlgorithmVersion),
		WrappedKeyCiphertext:     strings.TrimSpace(request.WrappedKeyCiphertext),
		PublicWrappingMetadata:   append([]byte(nil), request.PublicWrappingMetadata...),
	}
	if params.GrantID == "" {
		writeError(w, http.StatusBadRequest, "invalid_grant_id", "grant_id is required")
		return incidents.CreateWrappedKeyRecordParams{}, false
	}
	if params.MediaKeyID == "" || len(params.MediaKeyID) > maxWrappedKeyMediaKeyIDBytes {
		writeError(w, http.StatusBadRequest, "invalid_media_key_id", "media_key_id is required and must be 255 bytes or less")
		return incidents.CreateWrappedKeyRecordParams{}, false
	}
	if params.WrappingAlgorithm == "" || len(params.WrappingAlgorithm) > maxWrappingAlgorithmBytes {
		writeError(w, http.StatusBadRequest, "invalid_wrapping_algorithm", "wrapping_algorithm is required and must be 80 bytes or less")
		return incidents.CreateWrappedKeyRecordParams{}, false
	}
	if params.WrappingAlgorithmVersion == "" || len(params.WrappingAlgorithmVersion) > maxWrappedKeyWrappingAlgorithmVersionBytes {
		writeError(w, http.StatusBadRequest, "invalid_wrapping_algorithm_version", "wrapping_algorithm_version is required and must be 80 bytes or less")
		return incidents.CreateWrappedKeyRecordParams{}, false
	}
	if params.WrappedKeyCiphertext == "" || len(params.WrappedKeyCiphertext) > maxWrappedKeyCiphertextBytes {
		writeError(w, http.StatusBadRequest, "invalid_wrapped_key_ciphertext", "wrapped_key_ciphertext is required and must be 16384 bytes or less")
		return incidents.CreateWrappedKeyRecordParams{}, false
	}
	if !validPublicWrappingMetadata(params.PublicWrappingMetadata) {
		writeError(w, http.StatusBadRequest, "invalid_public_wrapping_metadata", "public_wrapping_metadata is required, must be a JSON object, and must be 4096 bytes or less")
		return incidents.CreateWrappedKeyRecordParams{}, false
	}
	return params, true
}

func validPublicWrappingMetadata(raw json.RawMessage) bool {
	return len(raw) > 0 &&
		len(raw) <= maxWrappedKeyPublicWrappingMetadataBytes &&
		json.Valid(raw) &&
		jsonObject(raw) &&
		!publicWrappingMetadataHasForbiddenKeys(raw)
}

func jsonObject(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")
}

func publicWrappingMetadataHasForbiddenKeys(raw json.RawMessage) bool {
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return true
	}
	return hasForbiddenMetadataKey(decoded)
}

func hasForbiddenMetadataKey(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			if _, ok := forbiddenPublicWrappingMetadataKeys[normalizeMetadataKey(key)]; ok {
				return true
			}
			if hasForbiddenMetadataKey(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if hasForbiddenMetadataKey(nested) {
				return true
			}
		}
	}
	return false
}

func normalizeMetadataKey(key string) string {
	replacer := strings.NewReplacer("_", "", "-", "", " ", "")
	return replacer.Replace(strings.ToLower(key))
}
