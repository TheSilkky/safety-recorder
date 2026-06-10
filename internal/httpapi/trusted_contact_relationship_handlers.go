package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/open-proofline/server/internal/incidents"
)

const (
	maxTrustedContactRelationshipAccountIDBytes = 255
	maxTrustedContactRelationshipRoleBytes      = 80
)

type createTrustedContactRelationshipRequest struct {
	RecipientAccountID string `json:"recipient_account_id"`
	RelationshipRole   string `json:"relationship_role"`
	DisplayLabel       string `json:"display_label"`
}

type replaceTrustedContactRelationshipRequest struct {
	RecipientAccountID string `json:"recipient_account_id"`
	RelationshipRole   string `json:"relationship_role"`
	DisplayLabel       string `json:"display_label"`
}

func (a *API) createTrustedContactRelationship(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	var request createTrustedContactRelationshipRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	params, ok := createTrustedContactRelationshipParams(w, principal.Account.ID, request)
	if !ok {
		return
	}
	relationship, err := a.repo.CreateTrustedContactRelationship(r.Context(), params)
	if errors.Is(err, incidents.ErrDuplicate) {
		writeError(w, http.StatusConflict, "trusted_contact_relationship_duplicate", "an open trusted-contact relationship already exists for this recipient")
		return
	}
	if errors.Is(err, incidents.ErrInvalidState) {
		writeError(w, http.StatusConflict, "invalid_trusted_contact_relationship_state", "trusted-contact relationship state transition is not allowed")
		return
	}
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "recipient_account_not_found", "recipient account was not found")
		return
	}
	if err != nil {
		a.internalError(w, "create trusted contact relationship", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]incidents.TrustedContactRelationship{
		"trusted_contact_relationship": relationship,
	})
}

func (a *API) listTrustedContactRelationships(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	relationships, err := a.repo.ListTrustedContactRelationshipsForAccount(r.Context(), principal.Account.ID)
	if err != nil {
		a.internalError(w, "list trusted contact relationships", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]incidents.TrustedContactRelationship{
		"trusted_contact_relationships": relationships,
	})
}

func (a *API) getTrustedContactRelationship(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	relationship, err := a.repo.GetTrustedContactRelationshipForAccount(r.Context(), principal.Account.ID, r.PathValue("relationship_id"))
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "trusted_contact_relationship_not_found", "trusted-contact relationship was not found")
		return
	}
	if err != nil {
		a.internalError(w, "get trusted contact relationship", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]incidents.TrustedContactRelationship{
		"trusted_contact_relationship": relationship,
	})
}

func (a *API) acceptTrustedContactRelationship(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	relationship, err := a.repo.AcceptTrustedContactRelationship(r.Context(), principal.Account.ID, r.PathValue("relationship_id"))
	writeTrustedContactRelationshipTransitionResponse(a, w, relationship, err, "accept trusted contact relationship")
}

func (a *API) declineTrustedContactRelationship(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	relationship, err := a.repo.DeclineTrustedContactRelationship(r.Context(), principal.Account.ID, r.PathValue("relationship_id"))
	writeTrustedContactRelationshipTransitionResponse(a, w, relationship, err, "decline trusted contact relationship")
}

func (a *API) revokeTrustedContactRelationship(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	relationship, err := a.repo.RevokeTrustedContactRelationship(r.Context(), principal.Account.ID, r.PathValue("relationship_id"), principal.Account.ID)
	writeTrustedContactRelationshipTransitionResponse(a, w, relationship, err, "revoke trusted contact relationship")
}

func (a *API) replaceTrustedContactRelationship(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	var request replaceTrustedContactRelationshipRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	params, ok := replaceTrustedContactRelationshipParams(w, principal.Account.ID, r.PathValue("relationship_id"), request)
	if !ok {
		return
	}
	relationship, err := a.repo.ReplaceTrustedContactRelationship(r.Context(), params)
	if errors.Is(err, incidents.ErrDuplicate) {
		writeError(w, http.StatusConflict, "trusted_contact_relationship_duplicate", "an open trusted-contact relationship already exists for this recipient")
		return
	}
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "trusted_contact_relationship_not_found", "trusted-contact relationship or recipient account was not found")
		return
	}
	if errors.Is(err, incidents.ErrInvalidState) {
		writeError(w, http.StatusConflict, "invalid_trusted_contact_relationship_state", "trusted-contact relationship state transition is not allowed")
		return
	}
	if err != nil {
		a.internalError(w, "replace trusted contact relationship", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]incidents.TrustedContactRelationship{
		"trusted_contact_relationship": relationship,
	})
}

func createTrustedContactRelationshipParams(w http.ResponseWriter, ownerAccountID string, request createTrustedContactRelationshipRequest) (incidents.CreateTrustedContactRelationshipParams, bool) {
	role := strings.TrimSpace(request.RelationshipRole)
	if role == "" {
		role = incidents.TrustedContactRelationshipRoleTrustedContact
	}
	params := incidents.CreateTrustedContactRelationshipParams{
		OwnerAccountID:     ownerAccountID,
		RecipientAccountID: strings.TrimSpace(request.RecipientAccountID),
		RelationshipRole:   role,
		DisplayLabel:       strings.TrimSpace(request.DisplayLabel),
	}
	if !validateTrustedContactRelationshipMetadata(w, params.OwnerAccountID, params.RecipientAccountID, params.RelationshipRole, params.DisplayLabel, true) {
		return incidents.CreateTrustedContactRelationshipParams{}, false
	}
	return params, true
}

func replaceTrustedContactRelationshipParams(w http.ResponseWriter, ownerAccountID, relationshipID string, request replaceTrustedContactRelationshipRequest) (incidents.ReplaceTrustedContactRelationshipParams, bool) {
	role := strings.TrimSpace(request.RelationshipRole)
	params := incidents.ReplaceTrustedContactRelationshipParams{
		OwnerAccountID:     ownerAccountID,
		RelationshipID:     relationshipID,
		RecipientAccountID: strings.TrimSpace(request.RecipientAccountID),
		RelationshipRole:   role,
		DisplayLabel:       strings.TrimSpace(request.DisplayLabel),
	}
	if params.RecipientAccountID != "" || params.RelationshipRole != "" || params.DisplayLabel != "" {
		if !validateTrustedContactRelationshipMetadata(w, params.OwnerAccountID, params.RecipientAccountID, params.RelationshipRole, params.DisplayLabel, false) {
			return incidents.ReplaceTrustedContactRelationshipParams{}, false
		}
	}
	return params, true
}

func validateTrustedContactRelationshipMetadata(w http.ResponseWriter, ownerAccountID, recipientAccountID, role, displayLabel string, recipientRequired bool) bool {
	if recipientRequired && recipientAccountID == "" {
		writeError(w, http.StatusBadRequest, "invalid_recipient_account_id", "recipient_account_id is required")
		return false
	}
	if recipientAccountID != "" {
		if len(recipientAccountID) > maxTrustedContactRelationshipAccountIDBytes {
			writeError(w, http.StatusBadRequest, "invalid_recipient_account_id", "recipient_account_id must be 255 bytes or less")
			return false
		}
		if recipientAccountID == ownerAccountID {
			writeError(w, http.StatusBadRequest, "invalid_recipient_account_id", "recipient_account_id must identify another account")
			return false
		}
	}
	if role != "" {
		if len(role) > maxTrustedContactRelationshipRoleBytes || !incidents.ValidTrustedContactRelationshipRole(role) {
			writeError(w, http.StatusBadRequest, "invalid_relationship_role", "relationship_role is not supported")
			return false
		}
	}
	if len(displayLabel) > maxContactDisplayLabelBytes {
		writeError(w, http.StatusBadRequest, "invalid_display_label", "display_label must be 200 bytes or less")
		return false
	}
	return true
}

func writeTrustedContactRelationshipTransitionResponse(a *API, w http.ResponseWriter, relationship incidents.TrustedContactRelationship, err error, operation string) {
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "trusted_contact_relationship_not_found", "trusted-contact relationship was not found")
		return
	}
	if errors.Is(err, incidents.ErrInvalidState) {
		writeError(w, http.StatusConflict, "invalid_trusted_contact_relationship_state", "trusted-contact relationship state transition is not allowed")
		return
	}
	if err != nil {
		a.internalError(w, operation, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]incidents.TrustedContactRelationship{
		"trusted_contact_relationship": relationship,
	})
}
