package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/open-proofline/server/internal/incidents"
)

type incidentViewerIncidentSummary struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	ClientLabel string    `json:"client_label,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type incidentViewerCheckinSummary struct {
	CreatedAt            time.Time `json:"created_at"`
	DeviceBatteryPercent *int      `json:"device_battery_percent,omitempty"`
	DeviceNetwork        *string   `json:"device_network,omitempty"`
	Latitude             *float64  `json:"latitude,omitempty"`
	Longitude            *float64  `json:"longitude,omitempty"`
	AccuracyMeters       *float64  `json:"accuracy_meters,omitempty"`
}

type incidentViewerChunkSummary struct {
	ChunkIndex       int       `json:"chunk_index"`
	MediaType        string    `json:"media_type"`
	StartedAt        time.Time `json:"started_at"`
	EndedAt          time.Time `json:"ended_at"`
	OriginalFilename string    `json:"original_filename,omitempty"`
	ByteSize         int64     `json:"byte_size"`
	SHA256Hex        string    `json:"sha256_hex"`
	CreatedAt        time.Time `json:"created_at"`
}

type incidentViewerMediaSummary struct {
	MediaType   string                      `json:"media_type"`
	ChunkCount  int                         `json:"chunk_count"`
	LatestChunk *incidentViewerChunkSummary `json:"latest_chunk,omitempty"`
}

type incidentViewerStreamSummary struct {
	ID                 string     `json:"id"`
	MediaType          string     `json:"media_type"`
	Label              string     `json:"label,omitempty"`
	Status             string     `json:"status"`
	ExpectedChunkCount *int       `json:"expected_chunk_count,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	FailedAt           *time.Time `json:"failed_at,omitempty"`
	FailureReason      string     `json:"failure_reason,omitempty"`
	ChunkCount         int        `json:"chunk_count"`
	TotalBytes         int64      `json:"total_bytes"`
}

type incidentViewData struct {
	Incident               incidentViewerIncidentSummary          `json:"incident"`
	LatestCheckin          *incidentViewerCheckinSummary          `json:"latest_checkin,omitempty"`
	ChunkCountByMediaType  map[string]int                         `json:"chunk_count_by_media_type"`
	LatestChunkByMediaType map[string]*incidentViewerChunkSummary `json:"latest_chunk_by_media_type"`
	Media                  []incidentViewerMediaSummary           `json:"media"`
	Streams                []incidentViewerStreamSummary          `json:"streams"`
	CompletedStreams       []incidentViewerStreamSummary          `json:"completed_streams"`
	Warning                string                                 `json:"warning"`
	GeneratedAt            time.Time                              `json:"generated_at"`
}

type createIncidentTokenResponse struct {
	TokenID    string     `json:"token_id"`
	IncidentID string     `json:"incident_id"`
	Token      string     `json:"token"`
	Label      string     `json:"label,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

type createIncidentTokenRequest struct {
	Label        string     `json:"label"`
	ExpiresAt    *time.Time `json:"expires_at"`
	ExpiresAtSet bool       `json:"-"`
}

func (request *createIncidentTokenRequest) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for name := range fields {
		if name != "label" && name != "expires_at" {
			return fmt.Errorf("unknown field %q", name)
		}
	}
	if rawLabel, ok := fields["label"]; ok {
		if err := json.Unmarshal(rawLabel, &request.Label); err != nil {
			return err
		}
	}
	rawExpiresAt, ok := fields["expires_at"]
	if !ok {
		return nil
	}
	request.ExpiresAtSet = true
	if string(rawExpiresAt) == "null" {
		request.ExpiresAt = nil
		return nil
	}
	var expiresAt time.Time
	if err := json.Unmarshal(rawExpiresAt, &expiresAt); err != nil {
		return err
	}
	request.ExpiresAt = &expiresAt
	return nil
}

const incidentWarning = "If you are concerned about immediate safety, call emergency services now."

// createIncidentToken is an authenticated main API route that mints a read-only
// incident viewer capability for one incident.
func (a *API) createIncidentToken(w http.ResponseWriter, r *http.Request) {
	incidentID := r.PathValue("incident_id")
	if _, ok := a.authorizeIncident(w, r, incidentID, actionCreatePublicLink, dataClassPublicLinkGrant); !ok {
		return
	}

	var request createIncidentTokenRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	expiresAt := a.incidentTokenExpiresAt(request.ExpiresAt, request.ExpiresAtSet)
	token, rawToken, err := a.repo.CreateIncidentToken(r.Context(), incidentID, request.Label, expiresAt)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "incident_not_found", "incident was not found")
		return
	}
	if err != nil {
		a.internalError(w, "create incident token", err)
		return
	}

	setNoStore(w)
	// The raw token is returned only in this response; the repository stores
	// only its hash.
	writeJSON(w, http.StatusCreated, createIncidentTokenResponse{
		TokenID:    token.ID,
		IncidentID: token.IncidentID,
		Token:      rawToken,
		Label:      token.Label,
		CreatedAt:  token.CreatedAt,
		ExpiresAt:  token.ExpiresAt,
	})
}

func (a *API) incidentTokenExpiresAt(requestExpiresAt *time.Time, requestExpiresAtSet bool) *time.Time {
	if requestExpiresAtSet || a.defaultIncidentTokenTTL <= 0 {
		return requestExpiresAt
	}
	expiresAt := time.Now().UTC().Add(a.defaultIncidentTokenTTL)
	return &expiresAt
}

// revokeIncidentToken is an authenticated main API route that disables an
// incident token without deleting its audit metadata.
func (a *API) revokeIncidentToken(w http.ResponseWriter, r *http.Request) {
	tokenID := r.PathValue("token_id")
	token, err := a.repo.GetIncidentToken(r.Context(), tokenID)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "incident_token_not_found", "incident token was not found")
		return
	}
	if err != nil {
		a.internalError(w, "get incident token", err)
		return
	}
	if _, ok := a.authorizeIncident(w, r, token.IncidentID, actionRevokePublicLink, dataClassPublicLinkGrant); !ok {
		return
	}
	if err := a.repo.RevokeIncidentToken(r.Context(), tokenID); errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "incident_token_not_found", "incident token was not found")
		return
	} else if err != nil {
		a.internalError(w, "revoke incident token", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token_id": tokenID,
		"revoked":  true,
	})
}

// incidentViewerPage renders the public read-only HTML view after token validation.
func (a *API) incidentViewerPage(w http.ResponseWriter, r *http.Request) {
	setIncidentViewerPrivacyHeaders(w)
	data, ok := a.loadIncidentViewData(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := incidentViewerPageTemplate.Execute(w, data); err != nil {
		a.logger.Error("render incident viewer page", "err", err)
	}
}

// incidentViewData returns the same read-only summary as JSON for page polling.
func (a *API) incidentViewData(w http.ResponseWriter, r *http.Request) {
	setIncidentViewerPrivacyHeaders(w)
	data, ok := a.loadIncidentViewData(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func setIncidentViewerPrivacyHeaders(w http.ResponseWriter) {
	setPublicBrowserSecurityHeaders(w)
	setNoStore(w)
}

// loadIncidentViewData collapses invalid, expired, and revoked tokens into one
// public error so callers cannot distinguish token state.
func (a *API) loadIncidentViewData(w http.ResponseWriter, r *http.Request) (incidentViewData, bool) {
	token, ok := a.loadIncidentToken(w, r)
	if !ok {
		return incidentViewData{}, false
	}
	data, err := a.buildIncidentViewData(r.Context(), token)
	if err != nil {
		a.internalError(w, "load incident viewer data", err)
		return incidentViewData{}, false
	}
	return data, true
}

// buildIncidentViewData loads incident metadata only after token validation.
func (a *API) buildIncidentViewData(ctx context.Context, token incidents.IncidentToken) (incidentViewData, error) {
	detail, err := a.repo.GetIncidentDetail(ctx, token.IncidentID)
	if err != nil {
		return incidentViewData{}, err
	}
	return summarizeIncidentViewData(detail), nil
}

func (a *API) loadIncidentToken(w http.ResponseWriter, r *http.Request) (incidents.IncidentToken, bool) {
	setIncidentViewerPrivacyHeaders(w)
	token, err := a.repo.LookupIncidentToken(r.Context(), r.PathValue("token"))
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "incident_token_invalid", "incident token is invalid, expired, or revoked")
		return incidents.IncidentToken{}, false
	}
	if err != nil {
		a.internalError(w, "lookup incident token", err)
		return incidents.IncidentToken{}, false
	}
	return token, true
}

// summarizeIncidentViewData prepares viewer-safe incident data without exposing
// stored paths or encrypted file bytes.
func summarizeIncidentViewData(detail incidents.IncidentDetail) incidentViewData {
	chunkStats := collectIncidentViewerChunkStats(detail.Chunks)
	streams, completedStreams := summarizeIncidentViewerStreams(detail.Streams, chunkStats)

	return incidentViewData{
		Incident:               summarizeIncident(detail.Incident),
		LatestCheckin:          summarizeLatestCheckin(detail.Checkins),
		ChunkCountByMediaType:  chunkStats.chunkCountByMediaType,
		LatestChunkByMediaType: chunkStats.latestChunkByMediaType,
		Media:                  summarizeIncidentViewerMedia(chunkStats),
		Streams:                streams,
		CompletedStreams:       completedStreams,
		Warning:                incidentWarning,
		GeneratedAt:            time.Now().UTC(),
	}
}

type incidentViewerChunkStats struct {
	chunkCountByMediaType  map[string]int
	latestChunkByMediaType map[string]*incidentViewerChunkSummary
	chunkCountByStreamID   map[string]int
	byteCountByStreamID    map[string]int64
}

func collectIncidentViewerChunkStats(chunks []incidents.Chunk) incidentViewerChunkStats {
	stats := incidentViewerChunkStats{
		chunkCountByMediaType:  make(map[string]int),
		latestChunkByMediaType: make(map[string]*incidentViewerChunkSummary),
		chunkCountByStreamID:   make(map[string]int),
		byteCountByStreamID:    make(map[string]int64),
	}
	for _, chunk := range chunks {
		stats.chunkCountByMediaType[chunk.MediaType]++
		if chunk.StreamID != "" {
			stats.chunkCountByStreamID[chunk.StreamID]++
			stats.byteCountByStreamID[chunk.StreamID] += chunk.ByteSize
		}

		summary := summarizeChunk(chunk)
		current := stats.latestChunkByMediaType[chunk.MediaType]
		if current == nil || chunkReceivedAfter(summary, *current) {
			stats.latestChunkByMediaType[chunk.MediaType] = &summary
		}
	}
	return stats
}

func summarizeIncident(incident incidents.Incident) incidentViewerIncidentSummary {
	return incidentViewerIncidentSummary{
		ID:          incident.ID,
		Status:      incident.Status,
		ClientLabel: incident.ClientLabel,
		CreatedAt:   incident.CreatedAt,
		UpdatedAt:   incident.UpdatedAt,
	}
}

func summarizeLatestCheckin(checkins []incidents.Checkin) *incidentViewerCheckinSummary {
	if len(checkins) == 0 {
		return nil
	}
	summary := summarizeCheckin(checkins[len(checkins)-1])
	return &summary
}

func summarizeCheckin(checkin incidents.Checkin) incidentViewerCheckinSummary {
	return incidentViewerCheckinSummary{
		CreatedAt:            checkin.CreatedAt,
		DeviceBatteryPercent: checkin.DeviceBatteryPercent,
		DeviceNetwork:        checkin.DeviceNetwork,
		Latitude:             checkin.Latitude,
		Longitude:            checkin.Longitude,
		AccuracyMeters:       checkin.AccuracyMeters,
	}
}

func summarizeIncidentViewerMedia(stats incidentViewerChunkStats) []incidentViewerMediaSummary {
	mediaTypes := []string{
		incidents.MediaTypeAudio,
		incidents.MediaTypeVideo,
		incidents.MediaTypeLocation,
		incidents.MediaTypeMetadata,
	}
	media := make([]incidentViewerMediaSummary, 0, len(mediaTypes))
	for _, mediaType := range mediaTypes {
		media = append(media, incidentViewerMediaSummary{
			MediaType:   mediaType,
			ChunkCount:  stats.chunkCountByMediaType[mediaType],
			LatestChunk: stats.latestChunkByMediaType[mediaType],
		})
	}
	return media
}

func summarizeIncidentViewerStreams(streams []incidents.MediaStream, stats incidentViewerChunkStats) ([]incidentViewerStreamSummary, []incidentViewerStreamSummary) {
	summaries := make([]incidentViewerStreamSummary, 0, len(streams))
	completed := []incidentViewerStreamSummary{}
	for _, stream := range streams {
		summary := summarizeIncidentViewerStream(stream, stats)
		summaries = append(summaries, summary)
		if stream.Status == incidents.StreamStatusComplete {
			completed = append(completed, summary)
		}
	}
	return summaries, completed
}

func summarizeIncidentViewerStream(stream incidents.MediaStream, stats incidentViewerChunkStats) incidentViewerStreamSummary {
	return incidentViewerStreamSummary{
		ID:                 stream.ID,
		MediaType:          stream.MediaType,
		Label:              stream.Label,
		Status:             stream.Status,
		ExpectedChunkCount: stream.ExpectedChunkCount,
		CompletedAt:        stream.CompletedAt,
		FailedAt:           stream.FailedAt,
		FailureReason:      stream.FailureReason,
		ChunkCount:         stats.chunkCountByStreamID[stream.ID],
		TotalBytes:         stats.byteCountByStreamID[stream.ID],
	}
}

// summarizeChunk copies only metadata that is safe for the incident viewer.
func summarizeChunk(chunk incidents.Chunk) incidentViewerChunkSummary {
	return incidentViewerChunkSummary{
		ChunkIndex:       chunk.ChunkIndex,
		MediaType:        chunk.MediaType,
		StartedAt:        chunk.StartedAt,
		EndedAt:          chunk.EndedAt,
		OriginalFilename: chunk.OriginalFilename,
		ByteSize:         chunk.ByteSize,
		SHA256Hex:        chunk.SHA256Hex,
		CreatedAt:        chunk.CreatedAt,
	}
}

func chunkReceivedAfter(candidate, current incidentViewerChunkSummary) bool {
	if candidate.CreatedAt.Equal(current.CreatedAt) {
		return candidate.ChunkIndex > current.ChunkIndex
	}
	return candidate.CreatedAt.After(current.CreatedAt)
}
