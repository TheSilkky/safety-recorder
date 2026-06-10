package httpapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/open-proofline/server/internal/incidents"
	"github.com/open-proofline/server/internal/storage"
)

type chunkUpload struct {
	temp             *storage.TempUpload
	streamID         string
	chunkIndex       int
	mediaType        string
	startedAt        time.Time
	endedAt          time.Time
	sha256Hex        string
	originalFilename string
}

func (a *API) readChunkUpload(w http.ResponseWriter, r *http.Request) (chunkUpload, bool) {
	// Multipart adds envelope bytes around the file. SaveTemp still enforces
	// the exact uploaded file byte limit while streaming the file to disk.
	r.Body = http.MaxBytesReader(w, r.Body, a.maxUploadBytes+multipartOverhead)

	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_multipart", "request must be multipart/form-data")
		return chunkUpload{}, false
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
			return chunkUpload{}, false
		}
		if err != nil {
			cleanupTemp(temp)
			writeError(w, http.StatusBadRequest, "invalid_multipart", "could not read multipart request")
			return chunkUpload{}, false
		}

		if part.FormName() == "file" {
			var ok bool
			temp, partFilename, ok = a.readFilePart(r.Context(), w, part, temp)
			if !ok {
				return chunkUpload{}, false
			}
			continue
		}

		if part.FormName() == "" {
			continue
		}
		value, ok := readMultipartField(w, part, temp)
		if !ok {
			return chunkUpload{}, false
		}
		fields[part.FormName()] = value
	}

	if temp == nil {
		writeError(w, http.StatusBadRequest, "missing_file", "file field is required")
		return chunkUpload{}, false
	}

	parsed, ok := parseChunkFields(w, fields, partFilename)
	if !ok {
		temp.Cleanup()
		return chunkUpload{}, false
	}
	parsed.temp = temp
	return parsed, true
}

func (a *API) readFilePart(ctx context.Context, w http.ResponseWriter, part *multipart.Part, current *storage.TempUpload) (*storage.TempUpload, string, bool) {
	if current != nil {
		current.Cleanup()
		writeError(w, http.StatusBadRequest, "duplicate_file", "only one file field is allowed")
		return nil, "", false
	}

	partFilename := cleanFilename(part.FileName())
	temp, err := a.store.SaveTemp(ctx, part, a.maxUploadBytes)
	if errors.Is(err, storage.ErrTooLarge) || isMaxBytesError(err) {
		writeError(w, http.StatusRequestEntityTooLarge, "upload_too_large", "upload exceeded SAFE_MAX_UPLOAD_BYTES")
		return nil, "", false
	}
	if errors.Is(err, storage.ErrTempStagingQuotaExceeded) {
		writeError(w, http.StatusInsufficientStorage, "upload_staging_quota_exceeded", "upload staging quota exceeded")
		return nil, "", false
	}
	if err != nil {
		a.internalError(w, "save temp upload", err)
		return nil, "", false
	}
	return temp, partFilename, true
}

func readMultipartField(w http.ResponseWriter, part *multipart.Part, temp *storage.TempUpload) (string, bool) {
	value, err := readField(part)
	if isMaxBytesError(err) {
		cleanupTemp(temp)
		writeError(w, http.StatusRequestEntityTooLarge, "upload_too_large", "upload exceeded SAFE_MAX_UPLOAD_BYTES")
		return "", false
	}
	if err != nil {
		cleanupTemp(temp)
		writeError(w, http.StatusBadRequest, "invalid_field", "multipart field was too large or unreadable")
		return "", false
	}
	return value, true
}

func cleanupTemp(temp *storage.TempUpload) {
	if temp != nil {
		temp.Cleanup()
	}
}

func parseChunkFields(w http.ResponseWriter, fields map[string]string, partFilename string) (chunkUpload, bool) {
	chunkIndex, err := parseChunkIndex(requiredField(fields, "chunk_index"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_chunk_index", "chunk_index must be a non-negative integer")
		return chunkUpload{}, false
	}

	mediaType := requiredField(fields, "media_type")
	if !incidents.ValidMediaType(mediaType) {
		writeError(w, http.StatusBadRequest, "invalid_media_type", "media_type must be audio, video, location, or metadata")
		return chunkUpload{}, false
	}

	startedAt, endedAt, ok := parseChunkTimeRange(w, fields)
	if !ok {
		return chunkUpload{}, false
	}

	sha256Hex := requiredField(fields, "sha256_hex")
	if !validSHA256Hex(sha256Hex) {
		writeError(w, http.StatusBadRequest, "invalid_sha256_hex", "sha256_hex must be lowercase SHA-256 hex")
		return chunkUpload{}, false
	}

	originalFilename := cleanFilename(fields["original_filename"])
	if originalFilename == "" {
		originalFilename = partFilename
	}

	streamID := requiredField(fields, "stream_id")
	// Legacy unstreamed uploads may use index 0. Streams start at 1 so
	// completion checks can require contiguous chunks without a zero slot.
	if streamID != "" && chunkIndex <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_chunk_index", "chunk_index must be positive when stream_id is provided")
		return chunkUpload{}, false
	}

	return chunkUpload{
		streamID:         streamID,
		chunkIndex:       chunkIndex,
		mediaType:        mediaType,
		startedAt:        startedAt,
		endedAt:          endedAt,
		sha256Hex:        sha256Hex,
		originalFilename: originalFilename,
	}, true
}

func parseChunkTimeRange(w http.ResponseWriter, fields map[string]string) (time.Time, time.Time, bool) {
	startedAt, err := time.Parse(time.RFC3339Nano, requiredField(fields, "started_at"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_started_at", "started_at must be an RFC3339 timestamp")
		return time.Time{}, time.Time{}, false
	}
	endedAt, err := time.Parse(time.RFC3339Nano, requiredField(fields, "ended_at"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_ended_at", "ended_at must be an RFC3339 timestamp")
		return time.Time{}, time.Time{}, false
	}
	if endedAt.Before(startedAt) {
		writeError(w, http.StatusBadRequest, "invalid_time_range", "ended_at must be after started_at")
		return time.Time{}, time.Time{}, false
	}
	return startedAt.UTC(), endedAt.UTC(), true
}

func requiredField(fields map[string]string, name string) string {
	return strings.TrimSpace(fields[name])
}

func readField(part *multipart.Part) (string, error) {
	var buffer bytes.Buffer
	n, err := io.Copy(&buffer, io.LimitReader(part, fieldLimit+1))
	if err != nil {
		return "", err
	}
	if n > fieldLimit {
		return "", fmt.Errorf("field too large")
	}
	return buffer.String(), nil
}

func parseChunkIndex(raw string) (int, error) {
	if raw == "" {
		return 0, fmt.Errorf("missing chunk_index")
	}
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, fmt.Errorf("chunk_index must be non-negative")
	}
	return int(value), nil
}

func validSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') {
			continue
		}
		return false
	}
	return true
}

func cleanFilename(value string) string {
	cleaned := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if cleaned == "" {
		return ""
	}
	base := path.Base(cleaned)
	if base == "." || base == "/" {
		return ""
	}
	return base
}
