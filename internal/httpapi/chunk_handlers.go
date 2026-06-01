package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/open-proofline/server/internal/incidents"
	"github.com/open-proofline/server/internal/storage"
)

const rollbackBlobRemoveTimeout = 30 * time.Second

type chunkReconciliationRequest struct {
	StreamID         string `json:"stream_id"`
	ChunkIndex       *int   `json:"chunk_index"`
	MediaType        string `json:"media_type"`
	StartedAt        string `json:"started_at"`
	EndedAt          string `json:"ended_at"`
	ByteSize         *int64 `json:"byte_size"`
	SHA256Hex        string `json:"sha256_hex"`
	OriginalFilename string `json:"original_filename"`
}

type chunkReconciliationInput struct {
	streamID         string
	chunkIndex       int
	mediaType        string
	startedAt        time.Time
	endedAt          time.Time
	byteSize         int64
	sha256Hex        string
	originalFilename string
}

type chunkReconciliationIdentity struct {
	IncidentID string `json:"incident_id"`
	StreamID   string `json:"stream_id,omitempty"`
	ChunkIndex int    `json:"chunk_index"`
	MediaType  string `json:"media_type"`
}

type chunkReconciliationResult struct {
	Status           string                      `json:"status"`
	Identity         chunkReconciliationIdentity `json:"identity"`
	ChunkID          string                      `json:"chunk_id,omitempty"`
	ByteSize         *int64                      `json:"byte_size,omitempty"`
	SHA256Hex        string                      `json:"sha256_hex,omitempty"`
	StartedAt        *time.Time                  `json:"started_at,omitempty"`
	EndedAt          *time.Time                  `json:"ended_at,omitempty"`
	CreatedAt        *time.Time                  `json:"created_at,omitempty"`
	MismatchedFields []string                    `json:"mismatched_fields,omitempty"`
}

type chunkReconciliationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type chunkReconciliationResponse struct {
	Error          *chunkReconciliationError `json:"error,omitempty"`
	Reconciliation chunkReconciliationResult `json:"reconciliation"`
}

func (a *API) uploadChunk(w http.ResponseWriter, r *http.Request) {
	incidentID := r.PathValue("incident_id")
	incident, ok := a.authorizeIncident(w, r, incidentID, actionWriteIncident, dataClassCiphertext)
	if !ok {
		return
	}
	if incident.Status == incidents.StatusClosed {
		writeError(w, http.StatusConflict, "incident_closed", "incident is closed")
		return
	}
	idempotencyKeyHash, hasIdempotencyKey, ok := readIdempotencyKeyHash(w, r)
	if !ok {
		return
	}

	upload, ok := a.readChunkUpload(w, r)
	if !ok {
		return
	}
	defer upload.temp.Cleanup()

	if upload.temp.SHA256Hex != upload.sha256Hex {
		writeError(w, http.StatusBadRequest, "hash_mismatch", "computed SHA-256 did not match provided hash")
		return
	}
	if !a.validateChunkStream(w, r, incidentID, upload) {
		return
	}
	uploadLease, ok := a.acquireUploadCoordinationLease(w, r, incidentID, upload)
	if !ok {
		return
	}
	defer a.releaseUploadCoordinationLease(uploadLease)

	var idempotencyParams incidents.UploadOperationParams
	if hasIdempotencyKey {
		idempotencyParams = uploadOperationParams(idempotencyKeyHash, incidentID, upload)
		operation, err := a.repo.ReserveUploadOperation(r.Context(), idempotencyParams)
		if errors.Is(err, incidents.ErrIdempotencyConflict) {
			idempotencyConflictError(w)
			return
		}
		if errors.Is(err, incidents.ErrIncidentDeleting) {
			writeIncidentDeleting(w)
			return
		}
		if err != nil {
			a.internalError(w, "reserve upload operation", err)
			return
		}
		if operation.State == incidents.UploadOperationStateMetadataCommitted {
			replayed, found, ok := a.replayEquivalentChunkIfPresent(w, r, incidentID, upload, idempotencyParams)
			if !ok || replayed {
				return
			}
			if found {
				idempotencyConflictError(w)
				return
			}
			a.internalError(w, "replay upload operation", incidents.ErrNotFound)
			return
		}
	}

	exists, err := a.repo.ChunkExists(r.Context(), incidentID, upload.streamID, upload.mediaType, upload.chunkIndex)
	if err != nil {
		a.internalError(w, "check duplicate chunk", err)
		return
	}
	if exists {
		if hasIdempotencyKey {
			replayed, _, ok := a.replayEquivalentChunkIfPresent(w, r, incidentID, upload, idempotencyParams)
			if !ok || replayed {
				return
			}
		}
		writeError(w, http.StatusConflict, "duplicate_chunk", "chunk_index already exists for this chunk identity")
		return
	}

	storedPath, err := a.store.CommitTemp(r.Context(), upload.temp, incidentID, upload.streamID, upload.mediaType, upload.chunkIndex)
	if errors.Is(err, storage.ErrAlreadyExists) {
		if hasIdempotencyKey {
			replayed, _, ok := a.replayEquivalentChunkIfPresent(w, r, incidentID, upload, idempotencyParams)
			if !ok || replayed {
				return
			}
		}
		writeError(w, http.StatusConflict, "duplicate_chunk", "stored chunk already exists for this chunk identity")
		return
	}
	if err != nil {
		a.internalError(w, "commit upload", err)
		return
	}

	chunk, err := a.repo.CreateChunk(r.Context(), incidents.CreateChunkParams{
		IncidentID:       incidentID,
		StreamID:         upload.streamID,
		ChunkIndex:       upload.chunkIndex,
		MediaType:        upload.mediaType,
		StartedAt:        upload.startedAt,
		EndedAt:          upload.endedAt,
		OriginalFilename: upload.originalFilename,
		StoredPath:       storedPath,
		ByteSize:         upload.temp.ByteSize,
		SHA256Hex:        upload.sha256Hex,
	})
	if errors.Is(err, incidents.ErrDuplicate) {
		a.removeCommittedBlobAfterMetadataFailure(storedPath)
		if hasIdempotencyKey {
			replayed, _, ok := a.replayEquivalentChunkIfPresent(w, r, incidentID, upload, idempotencyParams)
			if !ok || replayed {
				return
			}
		}
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
	if errors.Is(err, incidents.ErrNotFound) {
		a.removeCommittedBlobAfterMetadataFailure(storedPath)
		if upload.streamID != "" {
			writeError(w, http.StatusNotFound, "stream_not_found", "media stream was not found")
			return
		}
		writeError(w, http.StatusNotFound, "incident_not_found", "incident was not found")
		return
	}
	if err != nil {
		a.removeCommittedBlobAfterMetadataFailure(storedPath)
		a.internalError(w, "insert chunk metadata", err)
		return
	}

	if hasIdempotencyKey {
		if _, err := a.repo.CompleteUploadOperation(r.Context(), idempotencyParams, chunk); errors.Is(err, incidents.ErrIdempotencyConflict) {
			idempotencyConflictError(w)
			return
		} else if err != nil {
			a.internalError(w, "complete upload operation", err)
			return
		}
	}
	writeJSON(w, http.StatusCreated, chunk)
}

func (a *API) reconcileChunk(w http.ResponseWriter, r *http.Request) {
	incidentID := r.PathValue("incident_id")
	if _, ok := a.authorizeIncident(w, r, incidentID, actionReadIncident, dataClassIncidentMetadata); !ok {
		return
	}

	input, ok := parseChunkReconciliationRequest(w, r)
	if !ok {
		return
	}
	if !a.validateChunkReconciliationStream(w, r, incidentID, input) {
		return
	}

	chunk, err := a.repo.GetChunkByIdentity(r.Context(), incidentID, input.streamID, input.mediaType, input.chunkIndex)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "chunk_not_found", "chunk was not found")
		return
	}
	if err != nil {
		a.internalError(w, "reconcile chunk", err)
		return
	}

	mismatchedFields := chunkReconciliationMismatchedFields(input, chunk)
	if len(mismatchedFields) > 0 {
		writeChunkReconciliationConflict(w, incidentID, input, mismatchedFields)
		return
	}
	writeChunkReconciliationMatched(w, chunk)
}

func (a *API) listChunks(w http.ResponseWriter, r *http.Request) {
	incidentID := r.PathValue("incident_id")
	if !a.ensureIncidentExists(w, r, incidentID) {
		return
	}

	chunks, err := a.repo.ListChunks(r.Context(), incidentID)
	if err != nil {
		a.internalError(w, "list chunks", err)
		return
	}
	writeJSON(w, http.StatusOK, chunks)
}

func parseChunkReconciliationRequest(w http.ResponseWriter, r *http.Request) (chunkReconciliationInput, bool) {
	var request chunkReconciliationRequest
	if !decodeJSON(w, r, &request) {
		return chunkReconciliationInput{}, false
	}

	if request.ChunkIndex == nil || *request.ChunkIndex < 0 {
		writeError(w, http.StatusBadRequest, "invalid_chunk_index", "chunk_index must be a non-negative integer")
		return chunkReconciliationInput{}, false
	}

	mediaType := strings.TrimSpace(request.MediaType)
	if !incidents.ValidMediaType(mediaType) {
		writeError(w, http.StatusBadRequest, "invalid_media_type", "media_type must be audio, video, location, or metadata")
		return chunkReconciliationInput{}, false
	}

	startedAt, endedAt, ok := parseChunkTimeRange(w, map[string]string{
		"started_at": request.StartedAt,
		"ended_at":   request.EndedAt,
	})
	if !ok {
		return chunkReconciliationInput{}, false
	}

	sha256Hex := strings.TrimSpace(request.SHA256Hex)
	if !validSHA256Hex(sha256Hex) {
		writeError(w, http.StatusBadRequest, "invalid_sha256_hex", "sha256_hex must be lowercase SHA-256 hex")
		return chunkReconciliationInput{}, false
	}

	if request.ByteSize == nil || *request.ByteSize < 0 {
		writeError(w, http.StatusBadRequest, "invalid_byte_size", "byte_size must be a non-negative integer")
		return chunkReconciliationInput{}, false
	}

	streamID := strings.TrimSpace(request.StreamID)
	if streamID != "" && *request.ChunkIndex <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_chunk_index", "chunk_index must be positive when stream_id is provided")
		return chunkReconciliationInput{}, false
	}

	return chunkReconciliationInput{
		streamID:         streamID,
		chunkIndex:       *request.ChunkIndex,
		mediaType:        mediaType,
		startedAt:        startedAt,
		endedAt:          endedAt,
		byteSize:         *request.ByteSize,
		sha256Hex:        sha256Hex,
		originalFilename: cleanFilename(request.OriginalFilename),
	}, true
}

func (a *API) validateChunkReconciliationStream(w http.ResponseWriter, r *http.Request, incidentID string, input chunkReconciliationInput) bool {
	if input.streamID == "" {
		return true
	}

	stream, err := a.repo.GetMediaStream(r.Context(), incidentID, input.streamID)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "stream_not_found", "media stream was not found")
		return false
	}
	if err != nil {
		a.internalError(w, "get media stream", err)
		return false
	}
	if stream.MediaType != input.mediaType {
		writeError(w, http.StatusBadRequest, "stream_media_type_mismatch", "stream media_type does not match chunk media_type")
		return false
	}
	return true
}

func chunkReconciliationMismatchedFields(input chunkReconciliationInput, chunk incidents.Chunk) []string {
	mismatchedFields := []string{}
	if chunk.MediaType != input.mediaType {
		mismatchedFields = append(mismatchedFields, "media_type")
	}
	if !metadataTimesEqual(chunk.StartedAt, input.startedAt) {
		mismatchedFields = append(mismatchedFields, "started_at")
	}
	if !metadataTimesEqual(chunk.EndedAt, input.endedAt) {
		mismatchedFields = append(mismatchedFields, "ended_at")
	}
	if chunk.OriginalFilename != input.originalFilename {
		mismatchedFields = append(mismatchedFields, "original_filename")
	}
	if chunk.ByteSize != input.byteSize {
		mismatchedFields = append(mismatchedFields, "byte_size")
	}
	if chunk.SHA256Hex != input.sha256Hex {
		mismatchedFields = append(mismatchedFields, "sha256_hex")
	}
	return mismatchedFields
}

func writeChunkReconciliationMatched(w http.ResponseWriter, chunk incidents.Chunk) {
	byteSize := chunk.ByteSize
	startedAt := chunk.StartedAt
	endedAt := chunk.EndedAt
	createdAt := chunk.CreatedAt
	writeJSON(w, http.StatusOK, chunkReconciliationResponse{
		Reconciliation: chunkReconciliationResult{
			Status:    "matched",
			Identity:  chunkReconciliationIdentityFromChunk(chunk),
			ChunkID:   chunk.ID,
			ByteSize:  &byteSize,
			SHA256Hex: chunk.SHA256Hex,
			StartedAt: &startedAt,
			EndedAt:   &endedAt,
			CreatedAt: &createdAt,
		},
	})
}

func writeChunkReconciliationConflict(w http.ResponseWriter, incidentID string, input chunkReconciliationInput, mismatchedFields []string) {
	writeJSON(w, http.StatusConflict, chunkReconciliationResponse{
		Error: &chunkReconciliationError{
			Code:    "duplicate_chunk_conflict",
			Message: "existing chunk does not match expected ciphertext or metadata",
		},
		Reconciliation: chunkReconciliationResult{
			Status:           "conflict",
			Identity:         chunkReconciliationIdentityFromInput(incidentID, input),
			MismatchedFields: mismatchedFields,
		},
	})
}

func chunkReconciliationIdentityFromChunk(chunk incidents.Chunk) chunkReconciliationIdentity {
	return chunkReconciliationIdentity{
		IncidentID: chunk.IncidentID,
		StreamID:   chunk.StreamID,
		ChunkIndex: chunk.ChunkIndex,
		MediaType:  chunk.MediaType,
	}
}

func chunkReconciliationIdentityFromInput(incidentID string, input chunkReconciliationInput) chunkReconciliationIdentity {
	return chunkReconciliationIdentity{
		IncidentID: incidentID,
		StreamID:   input.streamID,
		ChunkIndex: input.chunkIndex,
		MediaType:  input.mediaType,
	}
}

func (a *API) getChunkBytes(w http.ResponseWriter, r *http.Request) {
	incidentID := r.PathValue("incident_id")
	if _, ok := a.authorizeIncident(w, r, incidentID, actionReadCiphertextBundle, dataClassCiphertext); !ok {
		return
	}
	mediaType := r.PathValue("media_type")
	if !incidents.ValidMediaType(mediaType) {
		writeError(w, http.StatusBadRequest, "invalid_media_type", "media_type must be audio, video, location, or metadata")
		return
	}
	chunkIndex, err := parseChunkIndex(r.PathValue("chunk_index"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_chunk_index", "chunk_index must be a non-negative integer")
		return
	}

	chunk, err := a.repo.GetChunkByKey(r.Context(), incidentID, mediaType, chunkIndex)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "chunk_not_found", "chunk was not found")
		return
	}
	if err != nil {
		a.internalError(w, "get chunk", err)
		return
	}

	file, err := a.store.Open(r.Context(), chunk.StoredPath)
	if errors.Is(err, os.ErrNotExist) {
		a.internalError(w, "open chunk bytes", fmt.Errorf("metadata exists but file is missing: %w", err))
		return
	}
	if err != nil {
		a.internalError(w, "open chunk bytes", err)
		return
	}
	defer file.Close()

	setNoSniff(w)
	setNoStore(w)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(chunk.ByteSize, 10))
	if seeker, ok := file.(io.ReadSeeker); ok {
		http.ServeContent(w, r, path.Base(chunk.StoredPath), chunk.CreatedAt, seeker)
		return
	}
	if _, err := io.Copy(w, file); err != nil {
		a.logInternalError("write chunk bytes", err)
	}
}

func (a *API) validateChunkStream(w http.ResponseWriter, r *http.Request, incidentID string, upload chunkUpload) bool {
	if upload.streamID == "" {
		return true
	}

	stream, err := a.repo.GetMediaStream(r.Context(), incidentID, upload.streamID)
	if errors.Is(err, incidents.ErrNotFound) {
		writeError(w, http.StatusNotFound, "stream_not_found", "media stream was not found")
		return false
	}
	if err != nil {
		a.internalError(w, "get media stream", err)
		return false
	}
	if stream.Status != incidents.StreamStatusOpen {
		writeError(w, http.StatusConflict, "stream_not_open", "media stream is not open")
		return false
	}
	if stream.MediaType != upload.mediaType {
		writeError(w, http.StatusBadRequest, "stream_media_type_mismatch", "stream media_type does not match chunk media_type")
		return false
	}
	return true
}

func (a *API) replayEquivalentChunkIfPresent(w http.ResponseWriter, r *http.Request, incidentID string, upload chunkUpload, params incidents.UploadOperationParams) (bool, bool, bool) {
	chunk, err := a.repo.GetChunkByIdentity(r.Context(), incidentID, upload.streamID, upload.mediaType, upload.chunkIndex)
	if errors.Is(err, incidents.ErrNotFound) {
		return false, false, true
	}
	if err != nil {
		a.internalError(w, "get idempotent chunk", err)
		return false, false, false
	}
	if !uploadMatchesChunk(incidentID, upload, chunk) {
		return false, true, true
	}

	file, err := a.store.Open(r.Context(), chunk.StoredPath)
	if err != nil {
		a.internalError(w, "open idempotent chunk", err)
		return false, true, false
	}
	_ = file.Close()

	if _, err := a.repo.CompleteUploadOperation(r.Context(), params, chunk); errors.Is(err, incidents.ErrIdempotencyConflict) {
		idempotencyConflictError(w)
		return false, true, false
	} else if err != nil {
		a.internalError(w, "complete replayed upload operation", err)
		return false, true, false
	}

	replayChunkResponse(w, chunk)
	return true, true, true
}

func (a *API) removeCommittedBlobAfterMetadataFailure(storedPath string) {
	ctx, cancel := context.WithTimeout(context.Background(), rollbackBlobRemoveTimeout)
	defer cancel()
	_ = a.store.Remove(ctx, storedPath)
}
