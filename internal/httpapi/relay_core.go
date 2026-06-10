package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/open-proofline/server/internal/incidents"
	"github.com/open-proofline/server/internal/relaycap"
	"github.com/open-proofline/server/internal/storage"
)

const relayServiceAuthHeader = "X-Proofline-Relay-Service-Token"

type relayChunkRequest struct {
	RelaySessionID   string `json:"relay_session_id"`
	Capability       string `json:"capability"`
	IncidentID       string `json:"incident_id"`
	StreamID         string `json:"stream_id"`
	ChunkIndex       *int   `json:"chunk_index"`
	MediaType        string `json:"media_type"`
	StartedAt        string `json:"started_at"`
	EndedAt          string `json:"ended_at"`
	ByteSize         *int64 `json:"byte_size"`
	SHA256Hex        string `json:"sha256_hex"`
	OriginalFilename string `json:"original_filename"`
}

type relayChunkInput struct {
	relaySessionID   string
	capability       string
	incidentID       string
	streamID         string
	chunkIndex       int
	mediaType        string
	startedAt        time.Time
	endedAt          time.Time
	byteSize         int64
	sha256Hex        string
	originalFilename string
}

type relayPreflightResponse struct {
	RelayPreflight relayPreflightPayload `json:"relay_preflight"`
}

type relayPreflightPayload struct {
	Status        string `json:"status"`
	IncidentID    string `json:"incident_id"`
	StreamID      string `json:"stream_id"`
	ChunkIndex    int    `json:"chunk_index"`
	MediaType     string `json:"media_type"`
	MaxChunkBytes int64  `json:"max_chunk_bytes"`
	MaxChunks     int    `json:"max_chunks"`
}

type relayCommitResponse struct {
	RelayCommit relayCommitPayload `json:"relay_commit"`
}

type relayCommitPayload struct {
	Status     string    `json:"status"`
	ChunkID    string    `json:"chunk_id"`
	IncidentID string    `json:"incident_id"`
	StreamID   string    `json:"stream_id"`
	ChunkIndex int       `json:"chunk_index"`
	MediaType  string    `json:"media_type"`
	ByteSize   int64     `json:"byte_size"`
	SHA256Hex  string    `json:"sha256_hex"`
	CreatedAt  time.Time `json:"created_at"`
}

type relayFanoutAuthorizeRequest struct {
	RelaySessionID string `json:"relay_session_id"`
	Capability     string `json:"capability"`
	IncidentID     string `json:"incident_id"`
	StreamID       string `json:"stream_id"`
}

type relayFanoutAuthorizeInput struct {
	relaySessionID string
	capability     string
	incidentID     string
	streamID       string
}

type relayFanoutAuthorizeResponse struct {
	RelayFanout relayFanoutAuthorizePayload `json:"relay_fanout"`
}

type relayFanoutAuthorizePayload struct {
	Status     string `json:"status"`
	IncidentID string `json:"incident_id"`
	StreamID   string `json:"stream_id"`
}

func (a *API) relayPreflight(w http.ResponseWriter, r *http.Request) {
	if !a.requireRelayServiceAuth(w, r) {
		return
	}
	input, ok := parseRelayChunkJSON(w, r)
	if !ok {
		return
	}
	incident, capability, ok := a.validateRelayChunkRequest(w, r, input)
	if !ok {
		return
	}
	if !a.checkAccountBlobQuota(w, r, incident, input.byteSize) {
		return
	}
	exists, err := a.repo.ChunkExists(r.Context(), input.incidentID, input.streamID, input.mediaType, input.chunkIndex)
	if err != nil {
		a.internalError(w, "check relay duplicate chunk", err)
		return
	}
	if exists {
		writeError(w, http.StatusConflict, "duplicate_chunk", "chunk_index already exists for this chunk identity")
		return
	}

	writeJSON(w, http.StatusOK, relayPreflightResponse{RelayPreflight: relayPreflightPayload{
		Status:        "accepted",
		IncidentID:    input.incidentID,
		StreamID:      input.streamID,
		ChunkIndex:    input.chunkIndex,
		MediaType:     input.mediaType,
		MaxChunkBytes: capability.MaxChunkBytes,
		MaxChunks:     capability.MaxChunks,
	}})
}

func (a *API) relayFanoutAuthorize(w http.ResponseWriter, r *http.Request) {
	if !a.requireRelayServiceAuth(w, r) {
		return
	}
	input, ok := parseRelayFanoutAuthorizeJSON(w, r)
	if !ok {
		return
	}
	if !a.validateRelayFanoutRequest(w, r, input) {
		return
	}
	writeJSON(w, http.StatusOK, relayFanoutAuthorizeResponse{RelayFanout: relayFanoutAuthorizePayload{
		Status:     "authorized",
		IncidentID: input.incidentID,
		StreamID:   input.streamID,
	}})
}

func (a *API) relayCommit(w http.ResponseWriter, r *http.Request) {
	if !a.requireRelayServiceAuth(w, r) {
		return
	}
	input, upload, ok := a.readRelayCommitUpload(w, r)
	if !ok {
		return
	}
	defer upload.temp.Cleanup()

	incident, _, ok := a.validateRelayChunkRequest(w, r, input)
	if !ok {
		return
	}
	if upload.temp.ByteSize != input.byteSize {
		writeError(w, http.StatusBadRequest, "byte_size_mismatch", "computed byte size did not match declared byte_size")
		return
	}
	if upload.temp.SHA256Hex != input.sha256Hex {
		writeError(w, http.StatusBadRequest, "hash_mismatch", "computed SHA-256 did not match provided hash")
		return
	}
	if !a.validateChunkEnvelope(w, input.incidentID, upload) {
		return
	}
	uploadLease, ok := a.acquireUploadCoordinationLease(w, r, input.incidentID, upload)
	if !ok {
		return
	}
	defer a.releaseUploadCoordinationLease(uploadLease)

	exists, err := a.repo.ChunkExists(r.Context(), input.incidentID, upload.streamID, upload.mediaType, upload.chunkIndex)
	if err != nil {
		a.internalError(w, "check relay duplicate chunk", err)
		return
	}
	if exists {
		writeError(w, http.StatusConflict, "duplicate_chunk", "chunk_index already exists for this chunk identity")
		return
	}
	if !a.checkAccountBlobQuota(w, r, incident, upload.temp.ByteSize) {
		return
	}

	storedPath, err := a.store.CommitTemp(r.Context(), upload.temp, input.incidentID, upload.streamID, upload.mediaType, upload.chunkIndex)
	if errors.Is(err, storage.ErrAlreadyExists) {
		writeError(w, http.StatusConflict, "duplicate_chunk", "stored chunk already exists for this chunk identity")
		return
	}
	if err != nil {
		a.internalError(w, "commit relay upload", err)
		return
	}

	chunk, err := a.repo.CreateChunk(r.Context(), incidents.CreateChunkParams{
		IncidentID:            input.incidentID,
		StreamID:              upload.streamID,
		ChunkIndex:            upload.chunkIndex,
		MediaType:             upload.mediaType,
		StartedAt:             upload.startedAt,
		EndedAt:               upload.endedAt,
		OriginalFilename:      upload.originalFilename,
		StoredPath:            storedPath,
		ByteSize:              upload.temp.ByteSize,
		SHA256Hex:             upload.sha256Hex,
		AccountBlobQuotaBytes: a.accountBlobQuotaBytes,
	})
	if errors.Is(err, incidents.ErrDuplicate) {
		a.removeCommittedBlobAfterMetadataFailure(storedPath)
		writeError(w, http.StatusConflict, "duplicate_chunk", "chunk_index already exists for this chunk identity")
		return
	}
	if errors.Is(err, incidents.ErrIncidentClosed) {
		a.removeCommittedBlobAfterMetadataFailure(storedPath)
		writeError(w, http.StatusConflict, "incident_closed", "incident is closed")
		return
	}
	if errors.Is(err, incidents.ErrIncidentDeleting) {
		a.removeCommittedBlobAfterMetadataFailure(storedPath)
		writeIncidentDeleting(w)
		return
	}
	if errors.Is(err, incidents.ErrInvalidState) {
		a.removeCommittedBlobAfterMetadataFailure(storedPath)
		writeError(w, http.StatusConflict, "stream_not_open", "media stream is not open")
		return
	}
	if errors.Is(err, incidents.ErrAccountBlobQuotaExceeded) {
		a.removeCommittedBlobAfterMetadataFailure(storedPath)
		writeAccountBlobQuotaExceeded(w)
		return
	}
	if errors.Is(err, incidents.ErrNotFound) {
		a.removeCommittedBlobAfterMetadataFailure(storedPath)
		writeError(w, http.StatusNotFound, "stream_not_found", "media stream was not found")
		return
	}
	if err != nil {
		a.removeCommittedBlobAfterMetadataFailure(storedPath)
		a.internalError(w, "insert relay chunk metadata", err)
		return
	}

	writeJSON(w, http.StatusCreated, relayCommitResponse{RelayCommit: relayCommitPayload{
		Status:     "committed",
		ChunkID:    chunk.ID,
		IncidentID: chunk.IncidentID,
		StreamID:   chunk.StreamID,
		ChunkIndex: chunk.ChunkIndex,
		MediaType:  chunk.MediaType,
		ByteSize:   chunk.ByteSize,
		SHA256Hex:  chunk.SHA256Hex,
		CreatedAt:  chunk.CreatedAt,
	}})
}

func (a *API) requireRelayServiceAuth(w http.ResponseWriter, r *http.Request) bool {
	want := strings.TrimSpace(a.relayService.AuthToken)
	if len([]byte(want)) < 32 {
		writeError(w, http.StatusServiceUnavailable, "relay_service_auth_not_configured", "relay service authentication is not configured")
		return false
	}
	values := r.Header.Values(relayServiceAuthHeader)
	if len(values) != 1 {
		writeError(w, http.StatusUnauthorized, "relay_service_auth_required", "relay service authentication is required")
		return false
	}
	got := values[0]
	if got == "" || strings.TrimSpace(got) != got || !sameRelayServiceToken(want, got) {
		writeError(w, http.StatusUnauthorized, "relay_service_auth_required", "relay service authentication is required")
		return false
	}
	return true
}

func sameRelayServiceToken(want, got string) bool {
	wantHash := sha256.Sum256([]byte(want))
	gotHash := sha256.Sum256([]byte(got))
	return subtle.ConstantTimeCompare(wantHash[:], gotHash[:]) == 1
}

func parseRelayChunkJSON(w http.ResponseWriter, r *http.Request) (relayChunkInput, bool) {
	var request relayChunkRequest
	if !decodeJSON(w, r, &request) {
		return relayChunkInput{}, false
	}
	return parseRelayChunkRequest(w, request, "")
}

func parseRelayFanoutAuthorizeJSON(w http.ResponseWriter, r *http.Request) (relayFanoutAuthorizeInput, bool) {
	var request relayFanoutAuthorizeRequest
	if !decodeJSON(w, r, &request) {
		return relayFanoutAuthorizeInput{}, false
	}
	relaySessionID := strings.TrimSpace(request.RelaySessionID)
	capability := strings.TrimSpace(request.Capability)
	incidentID := strings.TrimSpace(request.IncidentID)
	streamID := strings.TrimSpace(request.StreamID)
	if relaySessionID == "" || capability == "" || incidentID == "" || streamID == "" {
		writeError(w, http.StatusBadRequest, "invalid_relay_fanout_request", "relay_session_id, capability, incident_id, and stream_id are required")
		return relayFanoutAuthorizeInput{}, false
	}
	return relayFanoutAuthorizeInput{
		relaySessionID: relaySessionID,
		capability:     capability,
		incidentID:     incidentID,
		streamID:       streamID,
	}, true
}

func (a *API) readRelayCommitUpload(w http.ResponseWriter, r *http.Request) (relayChunkInput, chunkUpload, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, a.maxUploadBytes+multipartOverhead)

	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_multipart", "request must be multipart/form-data")
		return relayChunkInput{}, chunkUpload{}, false
	}

	fields := make(map[string]string)
	var temp *storage.TempUpload
	var partFilename string

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if isMaxBytesError(err) {
			cleanupTemp(temp)
			writeError(w, http.StatusRequestEntityTooLarge, "upload_too_large", "upload exceeded SAFE_MAX_UPLOAD_BYTES")
			return relayChunkInput{}, chunkUpload{}, false
		}
		if err != nil {
			cleanupTemp(temp)
			writeError(w, http.StatusBadRequest, "invalid_multipart", "could not read multipart request")
			return relayChunkInput{}, chunkUpload{}, false
		}

		if part.FormName() == "file" {
			var ok bool
			temp, partFilename, ok = a.readFilePart(r.Context(), w, part, temp)
			if !ok {
				return relayChunkInput{}, chunkUpload{}, false
			}
			continue
		}

		if part.FormName() == "" {
			continue
		}
		value, ok := readMultipartField(w, part, temp)
		if !ok {
			return relayChunkInput{}, chunkUpload{}, false
		}
		fields[part.FormName()] = value
	}

	if temp == nil {
		writeError(w, http.StatusBadRequest, "missing_file", "file field is required")
		return relayChunkInput{}, chunkUpload{}, false
	}

	input, ok := parseRelayChunkFields(w, fields, partFilename)
	if !ok {
		temp.Cleanup()
		return relayChunkInput{}, chunkUpload{}, false
	}
	upload := input.chunkUpload(temp)
	return input, upload, true
}

func parseRelayChunkFields(w http.ResponseWriter, fields map[string]string, partFilename string) (relayChunkInput, bool) {
	chunkIndex, chunkIndexOK := parseRequiredPositiveInt(fields["chunk_index"])
	byteSize, byteSizeOK := parseRequiredPositiveInt64(fields["byte_size"])
	request := relayChunkRequest{
		RelaySessionID:   fields["relay_session_id"],
		Capability:       fields["capability"],
		IncidentID:       fields["incident_id"],
		StreamID:         fields["stream_id"],
		MediaType:        fields["media_type"],
		StartedAt:        fields["started_at"],
		EndedAt:          fields["ended_at"],
		SHA256Hex:        fields["sha256_hex"],
		OriginalFilename: fields["original_filename"],
	}
	if chunkIndexOK {
		request.ChunkIndex = &chunkIndex
	}
	if byteSizeOK {
		request.ByteSize = &byteSize
	}
	return parseRelayChunkRequest(w, request, partFilename)
}

func parseRelayChunkRequest(w http.ResponseWriter, request relayChunkRequest, partFilename string) (relayChunkInput, bool) {
	relaySessionID := strings.TrimSpace(request.RelaySessionID)
	capability := strings.TrimSpace(request.Capability)
	incidentID := strings.TrimSpace(request.IncidentID)
	streamID := strings.TrimSpace(request.StreamID)
	if relaySessionID == "" || capability == "" || incidentID == "" || streamID == "" {
		writeError(w, http.StatusBadRequest, "invalid_relay_request", "relay_session_id, capability, incident_id, and stream_id are required")
		return relayChunkInput{}, false
	}

	if request.ChunkIndex == nil || *request.ChunkIndex <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_chunk_index", "chunk_index must be positive")
		return relayChunkInput{}, false
	}
	mediaType := strings.TrimSpace(request.MediaType)
	if !incidents.ValidMediaType(mediaType) {
		writeError(w, http.StatusBadRequest, "invalid_media_type", "media_type must be audio, video, location, or metadata")
		return relayChunkInput{}, false
	}
	startedAt, endedAt, ok := parseChunkTimeRange(w, map[string]string{
		"started_at": request.StartedAt,
		"ended_at":   request.EndedAt,
	})
	if !ok {
		return relayChunkInput{}, false
	}
	if request.ByteSize == nil || *request.ByteSize <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_byte_size", "byte_size must be positive")
		return relayChunkInput{}, false
	}
	sha256Hex := strings.TrimSpace(request.SHA256Hex)
	if !validSHA256Hex(sha256Hex) {
		writeError(w, http.StatusBadRequest, "invalid_sha256_hex", "sha256_hex must be lowercase SHA-256 hex")
		return relayChunkInput{}, false
	}
	originalFilename := cleanFilename(request.OriginalFilename)
	if originalFilename == "" {
		originalFilename = partFilename
	}

	return relayChunkInput{
		relaySessionID:   relaySessionID,
		capability:       capability,
		incidentID:       incidentID,
		streamID:         streamID,
		chunkIndex:       *request.ChunkIndex,
		mediaType:        mediaType,
		startedAt:        startedAt,
		endedAt:          endedAt,
		byteSize:         *request.ByteSize,
		sha256Hex:        sha256Hex,
		originalFilename: originalFilename,
	}, true
}

func parseRequiredPositiveInt(raw string) (int, bool) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 32)
	if err != nil || value <= 0 {
		return 0, false
	}
	return int(value), true
}

func parseRequiredPositiveInt64(raw string) (int64, bool) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func (input relayChunkInput) chunkUpload(temp *storage.TempUpload) chunkUpload {
	return chunkUpload{
		temp:             temp,
		streamID:         input.streamID,
		chunkIndex:       input.chunkIndex,
		mediaType:        input.mediaType,
		startedAt:        input.startedAt,
		endedAt:          input.endedAt,
		sha256Hex:        input.sha256Hex,
		originalFilename: input.originalFilename,
	}
}

func (a *API) validateRelayChunkRequest(w http.ResponseWriter, r *http.Request, input relayChunkInput) (incidents.Incident, relaycap.Capability, bool) {
	if input.byteSize > a.maxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "upload_too_large", "upload exceeded SAFE_MAX_UPLOAD_BYTES")
		return incidents.Incident{}, relaycap.Capability{}, false
	}
	secret, err := relaycap.SecretBytes(a.relayCapability.Secret)
	if errors.Is(err, relaycap.ErrInvalidSecret) {
		writeError(w, http.StatusServiceUnavailable, "relay_capability_not_configured", "relay capability validation is not configured")
		return incidents.Incident{}, relaycap.Capability{}, false
	}
	if err != nil {
		a.internalError(w, "load relay capability secret", err)
		return incidents.Incident{}, relaycap.Capability{}, false
	}
	capability, err := relaycap.Validate(secret, input.capability, relaycap.ValidationContext{
		Role:           relaycap.RoleUpload,
		RelaySessionID: input.relaySessionID,
		IncidentID:     input.incidentID,
		StreamID:       input.streamID,
		Now:            time.Now().UTC(),
	})
	if err != nil {
		writeRelayCapabilityValidationError(w, err)
		return incidents.Incident{}, relaycap.Capability{}, false
	}
	if input.byteSize > capability.MaxChunkBytes ||
		input.chunkIndex > capability.MaxChunks ||
		!relayCapabilityAllowsMediaType(capability, input.mediaType) {
		writeError(w, http.StatusForbidden, "relay_capability_limit_exceeded", "relay capability does not authorize this upload")
		return incidents.Incident{}, relaycap.Capability{}, false
	}

	incident, err := a.repo.GetIncident(r.Context(), input.incidentID)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "incident_not_found", "incident was not found")
		return incidents.Incident{}, relaycap.Capability{}, false
	}
	if err != nil {
		a.internalError(w, "get relay incident", err)
		return incidents.Incident{}, relaycap.Capability{}, false
	}
	if incident.DeletionState != incidents.IncidentDeletionStateActive {
		writeIncidentDeleting(w)
		return incidents.Incident{}, relaycap.Capability{}, false
	}
	if incident.Status == incidents.StatusClosed {
		writeError(w, http.StatusConflict, "incident_closed", "incident is closed")
		return incidents.Incident{}, relaycap.Capability{}, false
	}

	stream, err := a.repo.GetMediaStream(r.Context(), input.incidentID, input.streamID)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "stream_not_found", "media stream was not found")
		return incidents.Incident{}, relaycap.Capability{}, false
	}
	if err != nil {
		a.internalError(w, "get relay media stream", err)
		return incidents.Incident{}, relaycap.Capability{}, false
	}
	if stream.Status != incidents.StreamStatusOpen {
		writeError(w, http.StatusConflict, "stream_not_open", "media stream is not open")
		return incidents.Incident{}, relaycap.Capability{}, false
	}
	if stream.MediaType != input.mediaType {
		writeError(w, http.StatusBadRequest, "stream_media_type_mismatch", "stream media_type does not match chunk media_type")
		return incidents.Incident{}, relaycap.Capability{}, false
	}
	return incident, capability, true
}

func (a *API) validateRelayFanoutRequest(w http.ResponseWriter, r *http.Request, input relayFanoutAuthorizeInput) bool {
	secret, err := relaycap.SecretBytes(a.relayCapability.Secret)
	if errors.Is(err, relaycap.ErrInvalidSecret) {
		writeError(w, http.StatusServiceUnavailable, "relay_capability_not_configured", "relay capability validation is not configured")
		return false
	}
	if err != nil {
		a.internalError(w, "load relay fanout capability secret", err)
		return false
	}
	if _, err := relaycap.Validate(secret, input.capability, relaycap.ValidationContext{
		Role:           relaycap.RoleFanout,
		RelaySessionID: input.relaySessionID,
		IncidentID:     input.incidentID,
		StreamID:       input.streamID,
		Now:            time.Now().UTC(),
	}); err != nil {
		writeRelayCapabilityValidationError(w, err)
		return false
	}

	incident, err := a.repo.GetIncident(r.Context(), input.incidentID)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "incident_not_found", "incident was not found")
		return false
	}
	if err != nil {
		a.internalError(w, "get relay fanout incident", err)
		return false
	}
	if incident.DeletionState != incidents.IncidentDeletionStateActive {
		writeIncidentDeleting(w)
		return false
	}
	if incident.Status == incidents.StatusClosed {
		writeError(w, http.StatusConflict, "incident_closed", "incident is closed")
		return false
	}

	stream, err := a.repo.GetMediaStream(r.Context(), input.incidentID, input.streamID)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "stream_not_found", "media stream was not found")
		return false
	}
	if err != nil {
		a.internalError(w, "get relay fanout media stream", err)
		return false
	}
	if stream.Status != incidents.StreamStatusOpen {
		writeError(w, http.StatusConflict, "stream_not_open", "media stream is not open")
		return false
	}
	return true
}

func relayCapabilityAllowsMediaType(capability relaycap.Capability, mediaType string) bool {
	for _, allowed := range capability.AllowedMediaTypes {
		if allowed == mediaType {
			return true
		}
	}
	return false
}

func writeRelayCapabilityValidationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, relaycap.ErrExpired):
		writeError(w, http.StatusUnauthorized, "relay_capability_expired", "relay capability is expired")
	case errors.Is(err, relaycap.ErrWrongRole):
		writeError(w, http.StatusForbidden, "relay_capability_wrong_role", "relay capability role is not allowed")
	case errors.Is(err, relaycap.ErrWrongBinding):
		writeError(w, http.StatusForbidden, "relay_capability_wrong_binding", "relay capability binding does not match request")
	default:
		writeError(w, http.StatusUnauthorized, "relay_capability_invalid", "relay capability is invalid")
	}
}
