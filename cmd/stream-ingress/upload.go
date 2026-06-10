package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/open-proofline/server/internal/storage"
)

const relayCoreServiceAuthHeader = "X-Proofline-Relay-Service-Token"

type relayUploader struct {
	cfg     streamIngressConfig
	store   *storage.Store
	client  *http.Client
	limiter *relayInFlightLimiter
	fanout  *relayFanoutHub
}

type relayUploadMetadata struct {
	RelaySessionID   string
	Capability       string
	IncidentID       string
	StreamID         string
	ChunkIndex       int
	MediaType        string
	StartedAt        string
	EndedAt          string
	ByteSize         int64
	SHA256Hex        string
	OriginalFilename string
}

type relayCoreError struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

type relayCoreCommitOutcome struct {
	Publish       bool
	State         string
	ErrorCode     string
	CoreErrorCode string
	Retryable     *bool
	Terminal      bool
}

func newRelayUploader(cfg streamIngressConfig, client *http.Client) (*relayUploader, error) {
	cfg = cfg.withDefaults()
	store, err := storage.NewWithOptions(cfg.DataDir, storage.Options{
		TempStagingQuotaBytes: cfg.TempStagingQuotaBytes,
	})
	if err != nil {
		return nil, configParseError{Name: "SAFE_STREAM_INGRESS_DATA_DIR", Message: "relay temp storage could not be prepared"}
	}
	if client == nil {
		client = &http.Client{Timeout: cfg.CoreRequestTimeout}
	}
	return &relayUploader{
		cfg:     cfg,
		store:   store,
		client:  client,
		limiter: newRelayInFlightLimiter(cfg.MaxInFlightPerSession, cfg.MaxInFlightPerClient),
		fanout:  newRelayFanoutHub(),
	}, nil
}

func (cfg streamIngressConfig) withDefaults() streamIngressConfig {
	if cfg.DataDir == "" {
		cfg.DataDir = defaultDataDir
	}
	if cfg.MaxUploadBytes <= 0 {
		cfg.MaxUploadBytes = defaultMaxUploadBytes
	}
	if cfg.TempStagingQuotaBytes <= 0 {
		cfg.TempStagingQuotaBytes = defaultTempStagingQuotaBytes
	}
	if cfg.CoreRequestTimeout <= 0 {
		cfg.CoreRequestTimeout = defaultCoreRequestTimeout
	}
	if cfg.MaxInFlightPerSession <= 0 {
		cfg.MaxInFlightPerSession = defaultMaxInFlightPerSession
	}
	if cfg.MaxInFlightPerClient <= 0 {
		cfg.MaxInFlightPerClient = defaultMaxInFlightPerClient
	}
	return cfg
}

func (u *relayUploader) configured() bool {
	return u != nil && u.cfg.CoreBaseURL != "" && len([]byte(strings.TrimSpace(u.cfg.CoreServiceAuthToken))) >= minCoreServiceAuthTokenBytes
}

func (u *relayUploader) uploadCompleteChunk(w http.ResponseWriter, r *http.Request) {
	if !u.configured() {
		writeError(w, http.StatusServiceUnavailable, "relay_core_not_configured", "relay core forwarding is not configured")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, u.cfg.MaxUploadBytes+relayUploadMultipartOverhead)
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_multipart", "request must be multipart/form-data")
		return
	}

	fields := make(map[string]string)
	var input relayUploadMetadata
	var release func()
	var temp *storage.TempUpload
	var sawFile bool

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			cleanupTemp(temp)
			writeError(w, http.StatusBadRequest, "invalid_multipart", "could not read multipart request")
			return
		}
		if part.FormName() == "file" {
			if sawFile {
				cleanupTemp(temp)
				writeError(w, http.StatusBadRequest, "duplicate_file", "file field must be supplied once")
				return
			}
			sawFile = true
			parsed, ok := parseRelayUploadMetadata(w, fields, u.cfg.MaxUploadBytes)
			if !ok {
				return
			}
			input = parsed
			acquire, ok := u.acquireUploadSlot(w, r, input)
			if !ok {
				return
			}
			release = acquire
			defer release()

			if !u.corePreflight(w, r.Context(), input) {
				return
			}
			accepted, ok := u.saveRelayFilePart(w, r.Context(), part)
			if !ok {
				return
			}
			temp = accepted
			continue
		}
		fieldName := part.FormName()
		if fieldName == "" {
			continue
		}
		if sawFile {
			cleanupTemp(temp)
			writeError(w, http.StatusBadRequest, "metadata_after_file", "metadata fields must be sent before file")
			return
		}
		if !relayUploadMetadataFieldAllowed(fieldName) {
			writeError(w, http.StatusBadRequest, "unexpected_field", "metadata field is not allowed")
			return
		}
		if _, exists := fields[fieldName]; exists {
			writeError(w, http.StatusBadRequest, "duplicate_field", "metadata fields must not be duplicated")
			return
		}
		value, ok := readRelayMetadataField(w, part)
		if !ok {
			return
		}
		fields[fieldName] = value
	}
	if temp == nil {
		writeError(w, http.StatusBadRequest, "missing_file", "file field is required")
		return
	}
	defer temp.Cleanup()

	if temp.ByteSize != input.ByteSize {
		writeError(w, http.StatusBadRequest, "byte_size_mismatch", "computed byte size did not match declared byte_size")
		return
	}
	if temp.SHA256Hex != input.SHA256Hex {
		writeError(w, http.StatusBadRequest, "hash_mismatch", "computed SHA-256 did not match provided hash")
		return
	}
	fanoutPublished := u.publishRelayFanout(input, temp)
	outcome := u.coreCommit(w, r.Context(), input, temp)
	if fanoutPublished {
		u.publishRelayFanoutOutcome(input, outcome)
	}
}

func (u *relayUploader) acquireUploadSlot(w http.ResponseWriter, r *http.Request, input relayUploadMetadata) (func(), bool) {
	release, status, code, message, ok := u.limiter.acquire(input.RelaySessionID, clientLimitKey(r.RemoteAddr), relayChunkKey(input))
	if !ok {
		writeError(w, status, code, message)
		return nil, false
	}
	return release, true
}

func (u *relayUploader) corePreflight(w http.ResponseWriter, ctx context.Context, input relayUploadMetadata) bool {
	body, err := json.Marshal(input.coreFields())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "relay_internal_error", "relay upload failed")
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.cfg.CoreBaseURL+"/v1/relay/preflight", bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "core_preflight_unavailable", "core preflight is unavailable")
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(relayCoreServiceAuthHeader, u.cfg.CoreServiceAuthToken)

	resp, err := u.client.Do(req)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "core_preflight_unavailable", "core preflight is unavailable")
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeCoreRejected(w, resp, "core_preflight_rejected", "core rejected relay preflight")
		return false
	}
	return true
}

func (u *relayUploader) coreCommit(w http.ResponseWriter, ctx context.Context, input relayUploadMetadata, temp *storage.TempUpload) relayCoreCommitOutcome {
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		errCh <- writeRelayCommitMultipart(writer, multipartWriter, input, temp.Path)
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.cfg.CoreBaseURL+"/v1/relay/commit", reader)
	if err != nil {
		_ = reader.Close()
		<-errCh
		writeError(w, http.StatusServiceUnavailable, "core_commit_unavailable", "core commit is unavailable")
		return relayCommitUnavailableOutcome("core_commit_unavailable")
	}
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	req.Header.Set(relayCoreServiceAuthHeader, u.cfg.CoreServiceAuthToken)

	resp, err := u.client.Do(req)
	if err != nil {
		_ = reader.Close()
		<-errCh
		writeError(w, http.StatusServiceUnavailable, "core_commit_unavailable", "core commit is unavailable")
		return relayCommitUnavailableOutcome("core_commit_unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		_ = reader.Close()
		<-errCh
		coreCode := writeCoreRejected(w, resp, "core_commit_rejected", "core rejected relay commit")
		return relayCommitRejectedOutcome(resp.StatusCode, coreCode)
	}
	if writeErr := <-errCh; writeErr != nil {
		writeError(w, http.StatusServiceUnavailable, "core_commit_unavailable", "core commit is unavailable")
		return relayCommitUnavailableOutcome("core_commit_unavailable")
	}
	if !relayUploadResponse(w, resp) {
		return relayCommitUnavailableOutcome("core_commit_invalid_response")
	}
	return relayCommitConfirmedOutcome()
}

func writeRelayCommitMultipart(pipeWriter *io.PipeWriter, multipartWriter *multipart.Writer, input relayUploadMetadata, tempPath string) error {
	defer pipeWriter.Close()
	for name, value := range input.formFields() {
		if err := multipartWriter.WriteField(name, value); err != nil {
			_ = multipartWriter.Close()
			return pipeWriter.CloseWithError(err)
		}
	}
	filename := input.OriginalFilename
	if filename == "" {
		filename = "chunk.pq"
	}
	fileWriter, err := multipartWriter.CreateFormFile("file", filename)
	if err != nil {
		_ = multipartWriter.Close()
		return pipeWriter.CloseWithError(err)
	}
	file, err := os.Open(tempPath)
	if err != nil {
		_ = multipartWriter.Close()
		return pipeWriter.CloseWithError(err)
	}
	_, copyErr := io.Copy(fileWriter, file)
	closeErr := file.Close()
	if copyErr != nil {
		_ = multipartWriter.Close()
		return pipeWriter.CloseWithError(copyErr)
	}
	if closeErr != nil {
		_ = multipartWriter.Close()
		return pipeWriter.CloseWithError(closeErr)
	}
	if err := multipartWriter.Close(); err != nil {
		return pipeWriter.CloseWithError(err)
	}
	return nil
}

func relayUploadResponse(w http.ResponseWriter, resp *http.Response) bool {
	var decoded map[string]map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&decoded); err != nil {
		writeError(w, http.StatusBadGateway, "core_commit_invalid_response", "core commit response was invalid")
		return false
	}
	payload := map[string]any{"status": "committed"}
	if coreCommit, ok := decoded["relay_commit"]; ok {
		for _, key := range []string{"status", "chunk_id", "incident_id", "stream_id", "chunk_index", "media_type", "byte_size", "sha256_hex", "created_at"} {
			if value, exists := coreCommit[key]; exists {
				payload[key] = value
			}
		}
	}
	writeJSON(w, resp.StatusCode, map[string]map[string]any{
		"relay_upload": payload,
	})
	return true
}

func writeCoreRejected(w http.ResponseWriter, resp *http.Response, code, message string) string {
	coreCode := coreErrorCode(resp.Body)
	status := safeCoreStatus(resp.StatusCode)
	body := map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	}
	if coreCode != "" {
		body["core_error_code"] = coreCode
	}
	writeJSON(w, status, body)
	return coreCode
}

func relayCommitConfirmedOutcome() relayCoreCommitOutcome {
	return relayCoreCommitOutcome{
		Publish: true,
		State:   "confirmed",
	}
}

func relayCommitRejectedOutcome(status int, coreCode string) relayCoreCommitOutcome {
	if status == http.StatusTooManyRequests || status >= 500 {
		return relayCoreCommitOutcome{
			Publish:       true,
			State:         "terminal_failure",
			ErrorCode:     "core_commit_rejected",
			CoreErrorCode: coreCode,
			Retryable:     boolPointer(true),
			Terminal:      true,
		}
	}
	return relayCoreCommitOutcome{
		Publish:       true,
		State:         "rejected",
		ErrorCode:     "core_commit_rejected",
		CoreErrorCode: coreCode,
		Retryable:     boolPointer(false),
		Terminal:      true,
	}
}

func relayCommitUnavailableOutcome(code string) relayCoreCommitOutcome {
	return relayCoreCommitOutcome{
		Publish:   true,
		State:     "terminal_failure",
		ErrorCode: code,
		Retryable: boolPointer(true),
		Terminal:  true,
	}
}

func boolPointer(value bool) *bool {
	return &value
}

func coreErrorCode(body io.Reader) string {
	var decoded relayCoreError
	if err := json.NewDecoder(io.LimitReader(body, 64*1024)).Decode(&decoded); err != nil {
		return ""
	}
	return safeErrorToken(decoded.Error.Code)
}

func safeCoreStatus(status int) int {
	switch status {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden,
		http.StatusNotFound, http.StatusConflict, http.StatusRequestEntityTooLarge,
		http.StatusTooManyRequests, http.StatusInsufficientStorage:
		return status
	default:
		if status >= 500 {
			return http.StatusServiceUnavailable
		}
		return http.StatusBadGateway
	}
}

func (u *relayUploader) saveRelayFilePart(w http.ResponseWriter, ctx context.Context, part *multipart.Part) (*storage.TempUpload, bool) {
	temp, err := u.store.SaveTemp(ctx, part, u.cfg.MaxUploadBytes)
	if errors.Is(err, storage.ErrTooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "upload_too_large", "upload exceeded configured relay upload limit")
		return nil, false
	}
	if errors.Is(err, storage.ErrTempStagingQuotaExceeded) {
		writeError(w, http.StatusInsufficientStorage, "relay_temp_staging_quota_exceeded", "relay temporary staging quota is exhausted")
		return nil, false
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "relay_staging_failed", "relay temporary staging failed")
		return nil, false
	}
	return temp, true
}

func readRelayMetadataField(w http.ResponseWriter, part *multipart.Part) (string, bool) {
	var builder strings.Builder
	limited := &io.LimitedReader{R: part, N: relayUploadMetadataFieldMaxBytes + 1}
	if _, err := io.Copy(&builder, limited); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_multipart", "could not read multipart metadata")
		return "", false
	}
	if limited.N == 0 {
		writeError(w, http.StatusBadRequest, "metadata_too_large", "metadata field is too large")
		return "", false
	}
	return builder.String(), true
}

func relayUploadMetadataFieldAllowed(name string) bool {
	switch name {
	case "relay_session_id", "capability", "incident_id", "stream_id", "chunk_index",
		"media_type", "started_at", "ended_at", "byte_size", "sha256_hex",
		"original_filename":
		return true
	default:
		return false
	}
}

func parseRelayUploadMetadata(w http.ResponseWriter, fields map[string]string, maxUploadBytes int64) (relayUploadMetadata, bool) {
	input := relayUploadMetadata{
		RelaySessionID:   strings.TrimSpace(fields["relay_session_id"]),
		Capability:       strings.TrimSpace(fields["capability"]),
		IncidentID:       strings.TrimSpace(fields["incident_id"]),
		StreamID:         strings.TrimSpace(fields["stream_id"]),
		MediaType:        strings.TrimSpace(fields["media_type"]),
		StartedAt:        strings.TrimSpace(fields["started_at"]),
		EndedAt:          strings.TrimSpace(fields["ended_at"]),
		SHA256Hex:        strings.TrimSpace(fields["sha256_hex"]),
		OriginalFilename: cleanRelayFilename(fields["original_filename"]),
	}
	if input.RelaySessionID == "" || input.Capability == "" || input.IncidentID == "" || input.StreamID == "" {
		writeError(w, http.StatusBadRequest, "invalid_relay_request", "relay_session_id, capability, incident_id, and stream_id are required")
		return relayUploadMetadata{}, false
	}
	chunkIndex, err := strconv.Atoi(strings.TrimSpace(fields["chunk_index"]))
	if err != nil || chunkIndex <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_chunk_index", "chunk_index must be positive")
		return relayUploadMetadata{}, false
	}
	input.ChunkIndex = chunkIndex
	if !validRelayMediaType(input.MediaType) {
		writeError(w, http.StatusBadRequest, "invalid_media_type", "media_type must be audio, video, location, or metadata")
		return relayUploadMetadata{}, false
	}
	startedAt, err := time.Parse(time.RFC3339, input.StartedAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_started_at", "started_at must be RFC3339")
		return relayUploadMetadata{}, false
	}
	endedAt, err := time.Parse(time.RFC3339, input.EndedAt)
	if err != nil || endedAt.Before(startedAt) {
		writeError(w, http.StatusBadRequest, "invalid_ended_at", "ended_at must be RFC3339 and not before started_at")
		return relayUploadMetadata{}, false
	}
	byteSize, err := strconv.ParseInt(strings.TrimSpace(fields["byte_size"]), 10, 64)
	if err != nil || byteSize <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_byte_size", "byte_size must be positive")
		return relayUploadMetadata{}, false
	}
	if byteSize > maxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "upload_too_large", "upload exceeded configured relay upload limit")
		return relayUploadMetadata{}, false
	}
	input.ByteSize = byteSize
	if !validRelaySHA256Hex(input.SHA256Hex) {
		writeError(w, http.StatusBadRequest, "invalid_sha256_hex", "sha256_hex must be lowercase SHA-256 hex")
		return relayUploadMetadata{}, false
	}
	return input, true
}

func validRelayMediaType(mediaType string) bool {
	switch mediaType {
	case "audio", "video", "location", "metadata":
		return true
	default:
		return false
	}
}

func validRelaySHA256Hex(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func cleanRelayFilename(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "\\", "/")
	return filepath.Base(value)
}

func (input relayUploadMetadata) coreFields() map[string]any {
	return map[string]any{
		"relay_session_id":  input.RelaySessionID,
		"capability":        input.Capability,
		"incident_id":       input.IncidentID,
		"stream_id":         input.StreamID,
		"chunk_index":       input.ChunkIndex,
		"media_type":        input.MediaType,
		"started_at":        input.StartedAt,
		"ended_at":          input.EndedAt,
		"byte_size":         input.ByteSize,
		"sha256_hex":        input.SHA256Hex,
		"original_filename": input.OriginalFilename,
	}
}

func (input relayUploadMetadata) formFields() map[string]string {
	return map[string]string{
		"relay_session_id":  input.RelaySessionID,
		"capability":        input.Capability,
		"incident_id":       input.IncidentID,
		"stream_id":         input.StreamID,
		"chunk_index":       strconv.Itoa(input.ChunkIndex),
		"media_type":        input.MediaType,
		"started_at":        input.StartedAt,
		"ended_at":          input.EndedAt,
		"byte_size":         strconv.FormatInt(input.ByteSize, 10),
		"sha256_hex":        input.SHA256Hex,
		"original_filename": input.OriginalFilename,
	}
}

func relayChunkKey(input relayUploadMetadata) string {
	return input.RelaySessionID + ":" + strconv.Itoa(input.ChunkIndex)
}

func cleanupTemp(temp *storage.TempUpload) {
	if temp != nil {
		temp.Cleanup()
	}
}

func safeErrorToken(value string) string {
	if value == "" {
		return ""
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return ""
		}
	}
	return value
}

type relayInFlightLimiter struct {
	mu         sync.Mutex
	sessions   map[string]int
	clients    map[string]int
	chunks     map[string]struct{}
	maxSession int
	maxClient  int
}

func newRelayInFlightLimiter(maxSession, maxClient int) *relayInFlightLimiter {
	return &relayInFlightLimiter{
		sessions:   make(map[string]int),
		clients:    make(map[string]int),
		chunks:     make(map[string]struct{}),
		maxSession: maxSession,
		maxClient:  maxClient,
	}
}

func (l *relayInFlightLimiter) acquire(sessionKey, clientKey, chunkKey string) (func(), int, string, string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, exists := l.chunks[chunkKey]; exists {
		return nil, http.StatusConflict, "relay_chunk_in_progress", "relay chunk is already in progress", false
	}
	if l.maxSession > 0 && l.sessions[sessionKey] >= l.maxSession {
		return nil, http.StatusTooManyRequests, "relay_session_in_flight_limit_exceeded", "relay session has too many in-flight uploads", false
	}
	if l.maxClient > 0 && l.clients[clientKey] >= l.maxClient {
		return nil, http.StatusTooManyRequests, "relay_client_in_flight_limit_exceeded", "client has too many in-flight relay uploads", false
	}
	l.sessions[sessionKey]++
	l.clients[clientKey]++
	l.chunks[chunkKey] = struct{}{}
	return func() {
		l.release(sessionKey, clientKey, chunkKey)
	}, 0, "", "", true
}

func (l *relayInFlightLimiter) release(sessionKey, clientKey, chunkKey string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	decrementOrDelete(l.sessions, sessionKey)
	decrementOrDelete(l.clients, clientKey)
	delete(l.chunks, chunkKey)
}

func decrementOrDelete(values map[string]int, key string) {
	if values[key] <= 1 {
		delete(values, key)
		return
	}
	values[key]--
}

func clientLimitKey(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	sum := sha256.Sum256([]byte(host))
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}
