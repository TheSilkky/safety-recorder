package httpapi

import (
	"errors"
	"net/http"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/incidents"
)

const (
	actionReadIncident         = "read_incident"
	actionWriteIncident        = "write_incident"
	actionCreatePublicLink     = "create_public_link"
	actionRevokePublicLink     = "revoke_public_link"
	actionReadSharingGrant     = "read_sharing_grant"
	actionCreateSharingGrant   = "create_sharing_grant"
	actionRevokeSharingGrant   = "revoke_sharing_grant"
	actionReadWrappedKey       = "read_wrapped_key"
	actionCreateWrappedKey     = "create_wrapped_key"
	actionRevokeWrappedKey     = "revoke_wrapped_key"
	actionReadCiphertextBundle = "read_ciphertext_bundle"
	actionDeleteIncident       = "delete_incident"

	dataClassIncidentMetadata = "incident_metadata"
	dataClassCiphertext       = "ciphertext_evidence"
	dataClassPublicLinkGrant  = "public_link_grant"
	dataClassSharingGrant     = "sharing_grant"
	dataClassWrappedKey       = "wrapped_key"
)

type incidentAuthorizationScope struct {
	action    string
	dataClass string
}

var currentIncidentAuthorizationScopes = map[incidentAuthorizationScope]struct{}{
	{action: actionReadIncident, dataClass: dataClassIncidentMetadata}:    {},
	{action: actionWriteIncident, dataClass: dataClassIncidentMetadata}:   {},
	{action: actionWriteIncident, dataClass: dataClassCiphertext}:         {},
	{action: actionReadCiphertextBundle, dataClass: dataClassCiphertext}:  {},
	{action: actionDeleteIncident, dataClass: dataClassIncidentMetadata}:  {},
	{action: actionCreatePublicLink, dataClass: dataClassPublicLinkGrant}: {},
	{action: actionRevokePublicLink, dataClass: dataClassPublicLinkGrant}: {},
	{action: actionReadSharingGrant, dataClass: dataClassSharingGrant}:    {},
	{action: actionCreateSharingGrant, dataClass: dataClassSharingGrant}:  {},
	{action: actionRevokeSharingGrant, dataClass: dataClassSharingGrant}:  {},
	{action: actionReadWrappedKey, dataClass: dataClassWrappedKey}:        {},
	{action: actionCreateWrappedKey, dataClass: dataClassWrappedKey}:      {},
	{action: actionRevokeWrappedKey, dataClass: dataClassWrappedKey}:      {},
}

func (a *API) authorizeIncident(w http.ResponseWriter, r *http.Request, incidentID, action, dataClass string) (incidents.Incident, bool) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return incidents.Incident{}, false
	}
	if !incidentAuthorizationScopeAllowed(action, dataClass) {
		writeError(w, http.StatusForbidden, "forbidden", "account is not authorized for this incident action")
		return incidents.Incident{}, false
	}
	incident, err := a.repo.GetIncident(r.Context(), incidentID)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "incident_not_found", "incident was not found")
		return incidents.Incident{}, false
	}
	if err != nil {
		a.internalError(w, "get incident", err)
		return incidents.Incident{}, false
	}
	if incident.DeletionState != incidents.IncidentDeletionStateActive &&
		action != actionReadIncident &&
		action != actionDeleteIncident {
		writeIncidentDeleting(w)
		return incidents.Incident{}, false
	}
	if principal.Account.Role == auth.RoleAdmin {
		return incident, true
	}
	if incident.OwnerAccountID != "" && incident.OwnerAccountID == principal.Account.ID {
		return incident, true
	}
	writeError(w, http.StatusForbidden, "forbidden", "account is not authorized for this incident")
	return incidents.Incident{}, false
}

func (a *API) authorizeOwnedIncident(w http.ResponseWriter, r *http.Request, incidentID, action, dataClass string) (incidents.Incident, bool) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return incidents.Incident{}, false
	}
	if !incidentAuthorizationScopeAllowed(action, dataClass) {
		writeError(w, http.StatusForbidden, "forbidden", "account is not authorized for this incident action")
		return incidents.Incident{}, false
	}
	incident, err := a.repo.GetIncident(r.Context(), incidentID)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "incident_not_found", "incident was not found")
		return incidents.Incident{}, false
	}
	if err != nil {
		a.internalError(w, "get incident", err)
		return incidents.Incident{}, false
	}
	if incident.DeletionState != incidents.IncidentDeletionStateActive {
		writeIncidentDeleting(w)
		return incidents.Incident{}, false
	}
	if incident.OwnerAccountID != "" && incident.OwnerAccountID == principal.Account.ID {
		return incident, true
	}
	writeError(w, http.StatusForbidden, "forbidden", "account owner role is required for this incident action")
	return incidents.Incident{}, false
}

func writeIncidentDeleting(w http.ResponseWriter) {
	writeError(w, http.StatusConflict, "incident_deleting", "incident deletion is in progress")
}

func incidentAuthorizationScopeAllowed(action, dataClass string) bool {
	_, ok := currentIncidentAuthorizationScopes[incidentAuthorizationScope{
		action:    action,
		dataClass: dataClass,
	}]
	return ok
}
