package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/open-proofline/server/internal/incidents"
)

const (
	idempotencyKeyHeader    = "Idempotency-Key"
	idempotencyReplayHeader = "Idempotency-Replayed"
	maxIdempotencyKeyBytes  = 255
	metadataTimeTolerance   = time.Microsecond
)

func readIdempotencyKeyHash(w http.ResponseWriter, r *http.Request) (string, bool, bool) {
	values := r.Header.Values(idempotencyKeyHeader)
	if len(values) == 0 {
		return "", false, true
	}
	if len(values) != 1 {
		writeError(w, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key must be a single header value")
		return "", false, false
	}
	rawValue := values[0]
	rawKey := strings.TrimSpace(rawValue)
	if rawKey == "" || rawKey != rawValue || len(rawKey) > maxIdempotencyKeyBytes || !visibleASCII(rawKey) {
		writeError(w, http.StatusBadRequest, "invalid_idempotency_key", "Idempotency-Key must be 1-255 visible ASCII characters")
		return "", false, false
	}
	sum := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(sum[:]), true, true
}

func visibleASCII(value string) bool {
	for _, char := range value {
		if char < 0x21 || char > 0x7e {
			return false
		}
	}
	return true
}

func uploadOperationParams(keyHash, incidentID string, upload chunkUpload) incidents.UploadOperationParams {
	return incidents.UploadOperationParams{
		Operation:          incidents.UploadOperationUploadChunk,
		IdempotencyKeyHash: keyHash,
		IncidentID:         incidentID,
		StreamID:           upload.streamID,
		ChunkIndex:         upload.chunkIndex,
		MediaType:          upload.mediaType,
		StartedAt:          upload.startedAt,
		EndedAt:            upload.endedAt,
		OriginalFilename:   upload.originalFilename,
		ByteSize:           upload.temp.ByteSize,
		SHA256Hex:          upload.sha256Hex,
		FingerprintHash:    uploadFingerprintHash(incidentID, upload),
	}
}

func uploadFingerprintHash(incidentID string, upload chunkUpload) string {
	var builder strings.Builder
	appendFingerprintField(&builder, "incident_id", incidentID)
	appendFingerprintField(&builder, "stream_id", upload.streamID)
	appendFingerprintField(&builder, "chunk_index", strconv.Itoa(upload.chunkIndex))
	appendFingerprintField(&builder, "media_type", upload.mediaType)
	appendFingerprintField(&builder, "started_at", upload.startedAt.UTC().Format(time.RFC3339Nano))
	appendFingerprintField(&builder, "ended_at", upload.endedAt.UTC().Format(time.RFC3339Nano))
	appendFingerprintField(&builder, "original_filename", upload.originalFilename)
	appendFingerprintField(&builder, "byte_size", strconv.FormatInt(upload.temp.ByteSize, 10))
	appendFingerprintField(&builder, "sha256_hex", upload.sha256Hex)
	sum := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(sum[:])
}

func appendFingerprintField(builder *strings.Builder, name, value string) {
	builder.WriteString(name)
	builder.WriteByte('=')
	builder.WriteString(strconv.Itoa(len(value)))
	builder.WriteByte(':')
	builder.WriteString(value)
	builder.WriteByte('\n')
}

func uploadMatchesChunk(incidentID string, upload chunkUpload, chunk incidents.Chunk) bool {
	return chunk.IncidentID == incidentID &&
		chunk.StreamID == upload.streamID &&
		chunk.ChunkIndex == upload.chunkIndex &&
		chunk.MediaType == upload.mediaType &&
		metadataTimesEqual(chunk.StartedAt, upload.startedAt) &&
		metadataTimesEqual(chunk.EndedAt, upload.endedAt) &&
		chunk.OriginalFilename == upload.originalFilename &&
		chunk.ByteSize == upload.temp.ByteSize &&
		chunk.SHA256Hex == upload.sha256Hex
}

func metadataTimesEqual(stored, input time.Time) bool {
	delta := stored.UTC().Sub(input.UTC())
	if delta < 0 {
		delta = -delta
	}
	return delta < metadataTimeTolerance
}

func idempotencyConflictError(w http.ResponseWriter) {
	writeError(w, http.StatusConflict, "idempotency_conflict", "Idempotency-Key was reused with different upload inputs")
}

func replayChunkResponse(w http.ResponseWriter, chunk incidents.Chunk) {
	w.Header().Set(idempotencyReplayHeader, "true")
	writeJSON(w, http.StatusOK, chunk)
}
