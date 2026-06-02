package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/incidents"
)

const (
	defaultLegacyUnownedCandidateLimit = 100
	maxLegacyUnownedCandidateLimit     = 500
)

var validLegacyReassignmentReasonCodes = map[string]struct{}{
	"owner_verified":   {},
	"owner_request":    {},
	"operator_review":  {},
	"keep_admin_only":  {},
	"unknown_owner":    {},
	"other_controlled": {},
}

func (a *API) listLegacyUnownedIncidentCandidates(w http.ResponseWriter, r *http.Request) {
	if !a.requireAdmin(w, r) {
		return
	}
	limit, ok := parseLegacyUnownedCandidateLimit(w, r)
	if !ok {
		return
	}
	candidates, err := a.repo.ListLegacyUnownedIncidentCandidates(r.Context(), limit)
	if err != nil {
		a.internalError(w, "list legacy unowned incident candidates", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]incidents.LegacyUnownedIncidentCandidate{
		"incidents": candidates,
	})
}

func (a *API) reassignLegacyUnownedIncident(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	if principal.Account.Role != auth.RoleAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin role is required")
		return
	}

	var request struct {
		Action            string `json:"action"`
		NewOwnerAccountID string `json:"new_owner_account_id"`
		ReasonCode        string `json:"reason_code"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Action = strings.TrimSpace(request.Action)
	request.NewOwnerAccountID = strings.TrimSpace(request.NewOwnerAccountID)
	request.ReasonCode = strings.TrimSpace(request.ReasonCode)
	if !validLegacyReassignmentAction(request.Action) {
		writeError(w, http.StatusBadRequest, "invalid_action", "action must be assign_owner or keep_unowned")
		return
	}
	if !validLegacyReassignmentReasonCode(request.ReasonCode) {
		writeError(w, http.StatusBadRequest, "invalid_reason_code", "reason_code is not supported")
		return
	}
	if request.Action == incidents.LegacyIncidentReassignmentActionAssignOwner {
		if request.NewOwnerAccountID == "" {
			writeError(w, http.StatusBadRequest, "missing_new_owner_account_id", "new_owner_account_id is required for assign_owner")
			return
		}
		if _, err := a.repo.GetAccountByID(r.Context(), request.NewOwnerAccountID); errors.Is(err, auth.ErrNotFound) {
			writeError(w, http.StatusNotFound, "account_not_found", "destination account was not found")
			return
		} else if err != nil {
			a.internalError(w, "get reassignment account", err)
			return
		}
	} else if request.NewOwnerAccountID != "" {
		writeError(w, http.StatusBadRequest, "unexpected_new_owner_account_id", "new_owner_account_id is only allowed for assign_owner")
		return
	}

	event, err := a.repo.ReassignLegacyUnownedIncident(r.Context(), incidents.LegacyIncidentReassignmentParams{
		IncidentID:        r.PathValue("incident_id"),
		NewOwnerAccountID: request.NewOwnerAccountID,
		ActorAccountID:    principal.Account.ID,
		Action:            request.Action,
		ReasonCode:        request.ReasonCode,
		Source:            incidents.LegacyIncidentReassignmentSourceAdminAPI,
	})
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "incident_not_found", "legacy unowned incident was not found")
		return
	}
	if errors.Is(err, incidents.ErrInvalidState) {
		writeError(w, http.StatusConflict, "incident_not_eligible", "incident is not an active unowned legacy incident")
		return
	}
	if err != nil {
		a.internalError(w, "reassign legacy unowned incident", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]incidents.LegacyIncidentReassignmentEvent{
		"event": event,
	})
}

func parseLegacyUnownedCandidateLimit(w http.ResponseWriter, r *http.Request) (int, bool) {
	value := strings.TrimSpace(r.URL.Query().Get("limit"))
	if value == "" {
		return defaultLegacyUnownedCandidateLimit, true
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be a positive integer")
		return 0, false
	}
	if limit > maxLegacyUnownedCandidateLimit {
		limit = maxLegacyUnownedCandidateLimit
	}
	return limit, true
}

func validLegacyReassignmentAction(action string) bool {
	switch action {
	case incidents.LegacyIncidentReassignmentActionAssignOwner,
		incidents.LegacyIncidentReassignmentActionKeepUnowned:
		return true
	default:
		return false
	}
}

func validLegacyReassignmentReasonCode(reasonCode string) bool {
	_, ok := validLegacyReassignmentReasonCodes[reasonCode]
	return ok
}
