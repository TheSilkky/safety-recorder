package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/open-proofline/server/internal/incidents"
	"github.com/open-proofline/server/internal/relaycap"
)

type relaySessionResponse struct {
	RelaySession relaySessionPayload `json:"relay_session"`
}

type relaySessionPayload struct {
	RelaySessionID    string    `json:"relay_session_id"`
	Capability        string    `json:"capability"`
	FanoutCapability  string    `json:"fanout_capability"`
	Role              string    `json:"role"`
	IncidentID        string    `json:"incident_id"`
	StreamID          string    `json:"stream_id"`
	ExpiresAt         time.Time `json:"expires_at"`
	MaxChunkBytes     int64     `json:"max_chunk_bytes"`
	MaxChunks         int       `json:"max_chunks"`
	AllowedMediaTypes []string  `json:"allowed_media_types"`
}

func (a *API) createRelaySession(w http.ResponseWriter, r *http.Request) {
	incidentID := r.PathValue("incident_id")
	incident, ok := a.authorizeIncident(w, r, incidentID, actionWriteIncident, dataClassCiphertext)
	if !ok {
		return
	}
	if incident.Status == incidents.StatusClosed {
		writeError(w, http.StatusConflict, "incident_closed", "incident is closed")
		return
	}

	stream, err := a.repo.GetMediaStream(r.Context(), incidentID, r.PathValue("stream_id"))
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "stream_not_found", "media stream was not found")
		return
	}
	if err != nil {
		a.internalError(w, "get media stream", err)
		return
	}
	if stream.Status != incidents.StreamStatusOpen {
		writeError(w, http.StatusConflict, "stream_not_open", "media stream is not open")
		return
	}

	secret, err := relaycap.SecretBytes(a.relayCapability.Secret)
	if errors.Is(err, relaycap.ErrInvalidSecret) {
		writeError(w, http.StatusServiceUnavailable, "relay_capability_not_configured", "relay capability issuance is not configured")
		return
	}
	if err != nil {
		a.internalError(w, "load relay capability secret", err)
		return
	}

	now := time.Now().UTC()
	expiresAt := now.Add(a.relayCapability.TTL)
	sessionID, err := relaycap.NewSessionID()
	if err != nil {
		a.internalError(w, "create relay session id", err)
		return
	}
	uploadCapability := relaycap.Capability{
		Version:           relaycap.Version,
		RelaySessionID:    sessionID,
		Role:              relaycap.RoleUpload,
		IncidentID:        incidentID,
		StreamID:          stream.ID,
		IssuedAtUnix:      now.Unix(),
		ExpiresAtUnix:     expiresAt.Unix(),
		MaxChunkBytes:     a.maxUploadBytes,
		MaxChunks:         a.relayCapability.MaxChunks,
		AllowedMediaTypes: []string{stream.MediaType},
	}
	uploadToken, err := relaycap.Sign(secret, uploadCapability)
	if err != nil {
		a.internalError(w, "sign relay capability", err)
		return
	}
	fanoutCapability := uploadCapability
	fanoutCapability.Role = relaycap.RoleFanout
	fanoutToken, err := relaycap.Sign(secret, fanoutCapability)
	if err != nil {
		a.internalError(w, "sign relay fanout capability", err)
		return
	}

	writeJSON(w, http.StatusCreated, relaySessionResponse{RelaySession: relaySessionPayload{
		RelaySessionID:    sessionID,
		Capability:        uploadToken,
		FanoutCapability:  fanoutToken,
		Role:              relaycap.RoleUpload,
		IncidentID:        incidentID,
		StreamID:          stream.ID,
		ExpiresAt:         expiresAt,
		MaxChunkBytes:     a.maxUploadBytes,
		MaxChunks:         a.relayCapability.MaxChunks,
		AllowedMediaTypes: []string{stream.MediaType},
	}})
}
