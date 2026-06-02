package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/open-proofline/server/internal/incidents"
)

type createIncidentRequest struct {
	ClientLabel      string `json:"client_label"`
	Notes            string `json:"notes"`
	IncidentMode     string `json:"incident_mode"`
	CaptureProfile   string `json:"capture_profile"`
	EscalationPolicy string `json:"escalation_policy"`
	SharingState     string `json:"sharing_state"`
}

type createIncidentResponse struct {
	IncidentID       string `json:"incident_id"`
	Status           string `json:"status"`
	IncidentMode     string `json:"incident_mode,omitempty"`
	CaptureProfile   string `json:"capture_profile,omitempty"`
	EscalationPolicy string `json:"escalation_policy,omitempty"`
	SharingState     string `json:"sharing_state,omitempty"`
}

type accountIncidentResponse struct {
	ID               string    `json:"id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Status           string    `json:"status"`
	ClientLabel      string    `json:"client_label,omitempty"`
	IncidentMode     string    `json:"incident_mode,omitempty"`
	CaptureProfile   string    `json:"capture_profile,omitempty"`
	EscalationPolicy string    `json:"escalation_policy,omitempty"`
	SharingState     string    `json:"sharing_state,omitempty"`
	DeletionState    string    `json:"deletion_state"`
}

func (a *API) createIncident(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}

	var request createIncidentRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	params, ok := createIncidentParams(w, request)
	if !ok {
		return
	}

	incident, err := a.repo.CreateIncidentForAccount(r.Context(), principal.Account.ID, params)
	if err != nil {
		a.internalError(w, "create incident", err)
		return
	}

	writeJSON(w, http.StatusCreated, createIncidentResponse{
		IncidentID:       incident.ID,
		Status:           incident.Status,
		IncidentMode:     incident.IncidentMode,
		CaptureProfile:   incident.CaptureProfile,
		EscalationPolicy: incident.EscalationPolicy,
		SharingState:     incident.SharingState,
	})
}

func (a *API) listAccountIncidents(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}

	ownedIncidents, err := a.repo.ListIncidentsForAccount(r.Context(), principal.Account.ID)
	if err != nil {
		a.internalError(w, "list account incidents", err)
		return
	}

	response := make([]accountIncidentResponse, 0, len(ownedIncidents))
	for _, incident := range ownedIncidents {
		response = append(response, makeAccountIncidentResponse(incident))
	}
	writeJSON(w, http.StatusOK, map[string][]accountIncidentResponse{"incidents": response})
}

func createIncidentParams(w http.ResponseWriter, request createIncidentRequest) (incidents.CreateIncidentParams, bool) {
	params := incidents.CreateIncidentParams{
		ClientLabel:      request.ClientLabel,
		Notes:            request.Notes,
		IncidentMode:     strings.TrimSpace(request.IncidentMode),
		CaptureProfile:   strings.TrimSpace(request.CaptureProfile),
		EscalationPolicy: strings.TrimSpace(request.EscalationPolicy),
		SharingState:     strings.TrimSpace(request.SharingState),
	}
	if params.IncidentMode != "" && !incidents.ValidIncidentMode(params.IncidentMode) {
		writeError(w, http.StatusBadRequest, "invalid_incident_mode", "incident_mode must be emergency, interaction_record, safety_check, or evidence_note")
		return incidents.CreateIncidentParams{}, false
	}
	if params.CaptureProfile != "" && !incidents.ValidCaptureProfile(params.CaptureProfile) {
		writeError(w, http.StatusBadRequest, "invalid_capture_profile", "capture_profile must be audio_video_location, audio_location, location_checkin, note_or_attachment, or custom")
		return incidents.CreateIncidentParams{}, false
	}
	if params.EscalationPolicy != "" && !incidents.ValidEscalationPolicy(params.EscalationPolicy) {
		writeError(w, http.StatusBadRequest, "invalid_escalation_policy", "escalation_policy must be none, trusted_contacts_on_start, trusted_contacts_on_missed_checkin, or urgent_trusted_contact_alert")
		return incidents.CreateIncidentParams{}, false
	}
	if params.SharingState != "" && !incidents.ValidSharingState(params.SharingState) {
		writeError(w, http.StatusBadRequest, "invalid_sharing_state", "sharing_state must be private, trusted_contact_access, public_link_created, legal_export_created, or revoked_or_expired")
		return incidents.CreateIncidentParams{}, false
	}
	return params, true
}

func (a *API) getIncident(w http.ResponseWriter, r *http.Request) {
	incident, ok := a.authorizeOwnedIncidentRead(w, r, r.PathValue("incident_id"))
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, map[string]accountIncidentResponse{
		"incident": makeAccountIncidentResponse(incident),
	})
}

func makeAccountIncidentResponse(incident incidents.Incident) accountIncidentResponse {
	return accountIncidentResponse{
		ID:               incident.ID,
		CreatedAt:        incident.CreatedAt,
		UpdatedAt:        incident.UpdatedAt,
		Status:           incident.Status,
		ClientLabel:      incident.ClientLabel,
		IncidentMode:     incident.IncidentMode,
		CaptureProfile:   incident.CaptureProfile,
		EscalationPolicy: incident.EscalationPolicy,
		SharingState:     incident.SharingState,
		DeletionState:    incident.DeletionState,
	}
}

func (a *API) createCheckin(w http.ResponseWriter, r *http.Request) {
	incidentID := r.PathValue("incident_id")
	if _, ok := a.authorizeIncident(w, r, incidentID, actionWriteIncident, dataClassIncidentMetadata); !ok {
		return
	}

	var request struct {
		DeviceBatteryPercent *int     `json:"device_battery_percent"`
		DeviceNetwork        *string  `json:"device_network"`
		Latitude             *float64 `json:"latitude"`
		Longitude            *float64 `json:"longitude"`
		AccuracyMeters       *float64 `json:"accuracy_meters"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}

	checkin, err := a.repo.CreateCheckin(r.Context(), incidentID, incidents.CreateCheckinParams{
		DeviceBatteryPercent: request.DeviceBatteryPercent,
		DeviceNetwork:        request.DeviceNetwork,
		Latitude:             request.Latitude,
		Longitude:            request.Longitude,
		AccuracyMeters:       request.AccuracyMeters,
	})
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "incident_not_found", "incident was not found")
		return
	}
	if errors.Is(err, incidents.ErrIncidentDeleting) {
		writeIncidentDeleting(w)
		return
	}
	if err != nil {
		a.internalError(w, "create checkin", err)
		return
	}

	writeJSON(w, http.StatusCreated, checkin)
}

func (a *API) closeIncident(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.authorizeIncident(w, r, r.PathValue("incident_id"), actionWriteIncident, dataClassIncidentMetadata); !ok {
		return
	}
	incident, err := a.repo.CloseIncident(r.Context(), r.PathValue("incident_id"))
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "incident_not_found", "incident was not found")
		return
	}
	if errors.Is(err, incidents.ErrIncidentDeleting) {
		writeIncidentDeleting(w)
		return
	}
	if err != nil {
		a.internalError(w, "close incident", err)
		return
	}
	writeJSON(w, http.StatusOK, incident)
}
