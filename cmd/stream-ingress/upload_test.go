package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testCoreServiceAuthToken = "stream-ingress-core-service-token-12345"

func TestRelayUploadForwardsEncryptedChunkToCore(t *testing.T) {
	payload := []byte("opaque encrypted relay chunk")
	input := relayUploadTestMetadata("relay-session", "capability-token", "inc_test", "str_test", 1, "audio", payload)
	var sawPreflight atomic.Bool
	var sawCommit atomic.Bool
	handler := newRelayUploadTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertCoreServiceAuth(t, r)
		switch r.URL.Path {
		case "/v1/relay/preflight":
			sawPreflight.Store(true)
			assertCorePreflightBody(t, r, input)
			relayUploadWriteJSON(w, http.StatusOK, map[string]any{
				"relay_preflight": map[string]any{"status": "accepted"},
			})
		case "/v1/relay/commit":
			sawCommit.Store(true)
			assertCoreCommitMultipart(t, r, input, payload)
			relayUploadWriteJSON(w, http.StatusCreated, relayUploadCoreCommitResponse(input))
		default:
			t.Fatalf("unexpected core path %s", r.URL.Path)
		}
	}), streamIngressConfig{})

	response, body := postRelayUpload(t, handler, input, payload)
	if response.Code != http.StatusCreated {
		t.Fatalf("relay upload status = %d, want 201: %s", response.Code, body)
	}
	assertRelayUploadStatus(t, body, "committed")
	assertRedactedRelayUploadBody(t, body, testCoreServiceAuthToken, input.Capability)
	if !sawPreflight.Load() || !sawCommit.Load() {
		t.Fatalf("core preflight/commit seen = %v/%v, want true/true", sawPreflight.Load(), sawCommit.Load())
	}
	assertRelayTempDirEmpty(t, handler)
}

func TestRelayUploadPreflightRejectionDoesNotCommitOrStage(t *testing.T) {
	payload := []byte("opaque encrypted relay chunk")
	input := relayUploadTestMetadata("relay-session", "expired-capability", "inc_test", "str_test", 1, "audio", payload)
	var commitCalls atomic.Int64
	handler := newRelayUploadTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertCoreServiceAuth(t, r)
		switch r.URL.Path {
		case "/v1/relay/preflight":
			relayUploadWriteJSON(w, http.StatusUnauthorized, relayUploadError("relay_capability_expired"))
		case "/v1/relay/commit":
			commitCalls.Add(1)
			relayUploadWriteJSON(w, http.StatusCreated, relayUploadCoreCommitResponse(input))
		default:
			t.Fatalf("unexpected core path %s", r.URL.Path)
		}
	}), streamIngressConfig{})

	response, body := postRelayUpload(t, handler, input, payload)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("relay upload status = %d, want 401: %s", response.Code, body)
	}
	assertErrorCode(t, body, "core_preflight_rejected")
	if !bytes.Contains(body, []byte(`"core_error_code":"relay_capability_expired"`)) {
		t.Fatalf("relay response omitted safe core error code: %s", body)
	}
	if commitCalls.Load() != 0 {
		t.Fatalf("commit calls = %d, want 0", commitCalls.Load())
	}
	assertRelayTempDirEmpty(t, handler)
}

func TestRelayUploadRejectsUnexpectedMetadataBeforeCore(t *testing.T) {
	payload := []byte("opaque encrypted relay chunk")
	input := relayUploadTestMetadata("relay-session", "capability-token", "inc_test", "str_test", 1, "audio", payload)
	var coreCalls atomic.Int64
	handler := newRelayUploadTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		coreCalls.Add(1)
		relayUploadWriteJSON(w, http.StatusOK, map[string]any{
			"relay_preflight": map[string]any{"status": "accepted"},
		})
	}), streamIngressConfig{})

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range input.formFields() {
		if err := writer.WriteField(name, value); err != nil {
			t.Fatalf("write metadata field: %v", err)
		}
	}
	if err := writer.WriteField("unexpected_metadata", "value"); err != nil {
		t.Fatalf("write unexpected metadata field: %v", err)
	}
	fileWriter, err := writer.CreateFormFile("file", input.OriginalFilename)
	if err != nil {
		t.Fatalf("create file field: %v", err)
	}
	if _, err := fileWriter.Write(payload); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/upload/complete-chunk", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.RemoteAddr = "192.0.2.55:12345"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	responseBody := recorder.Body.Bytes()
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("relay upload status = %d, want 400: %s", recorder.Code, responseBody)
	}
	assertErrorCode(t, responseBody, "unexpected_field")
	if coreCalls.Load() != 0 {
		t.Fatalf("core calls = %d, want 0", coreCalls.Load())
	}
	assertRelayTempDirEmpty(t, handler)
}

func TestRelayUploadRejectsHashMismatchAndCleansStaging(t *testing.T) {
	payload := []byte("opaque encrypted relay chunk")
	input := relayUploadTestMetadata("relay-session", "capability-token", "inc_test", "str_test", 1, "audio", payload)
	input.SHA256Hex = strings.Repeat("a", 64)
	var commitCalls atomic.Int64
	handler := newRelayUploadTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertCoreServiceAuth(t, r)
		switch r.URL.Path {
		case "/v1/relay/preflight":
			relayUploadWriteJSON(w, http.StatusOK, map[string]any{
				"relay_preflight": map[string]any{"status": "accepted"},
			})
		case "/v1/relay/commit":
			commitCalls.Add(1)
			relayUploadWriteJSON(w, http.StatusCreated, relayUploadCoreCommitResponse(input))
		default:
			t.Fatalf("unexpected core path %s", r.URL.Path)
		}
	}), streamIngressConfig{})

	response, body := postRelayUpload(t, handler, input, payload)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("relay upload status = %d, want 400: %s", response.Code, body)
	}
	assertErrorCode(t, body, "hash_mismatch")
	if commitCalls.Load() != 0 {
		t.Fatalf("commit calls = %d, want 0", commitCalls.Load())
	}
	assertRelayTempDirEmpty(t, handler)
}

func TestRelayUploadMapsCoreCommitRejectionAndCleansStaging(t *testing.T) {
	payload := []byte("opaque encrypted relay chunk")
	input := relayUploadTestMetadata("relay-session", "capability-token", "inc_test", "str_test", 1, "audio", payload)
	handler := newRelayUploadTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertCoreServiceAuth(t, r)
		switch r.URL.Path {
		case "/v1/relay/preflight":
			relayUploadWriteJSON(w, http.StatusOK, map[string]any{
				"relay_preflight": map[string]any{"status": "accepted"},
			})
		case "/v1/relay/commit":
			assertCoreCommitMultipart(t, r, input, payload)
			relayUploadWriteJSON(w, http.StatusConflict, relayUploadError("duplicate_chunk"))
		default:
			t.Fatalf("unexpected core path %s", r.URL.Path)
		}
	}), streamIngressConfig{})

	response, body := postRelayUpload(t, handler, input, payload)
	if response.Code != http.StatusConflict {
		t.Fatalf("relay upload status = %d, want 409: %s", response.Code, body)
	}
	assertErrorCode(t, body, "core_commit_rejected")
	if !bytes.Contains(body, []byte(`"core_error_code":"duplicate_chunk"`)) {
		t.Fatalf("relay response omitted safe core error code: %s", body)
	}
	assertRelayTempDirEmpty(t, handler)
}

func TestRelayUploadMapsEarlyCoreCommitRejectionAndCleansStaging(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 128*1024)
	input := relayUploadTestMetadata("relay-session", "capability-token", "inc_test", "str_test", 1, "audio", payload)
	handler := newRelayUploadTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertCoreServiceAuth(t, r)
		switch r.URL.Path {
		case "/v1/relay/preflight":
			relayUploadWriteJSON(w, http.StatusOK, map[string]any{
				"relay_preflight": map[string]any{"status": "accepted"},
			})
		case "/v1/relay/commit":
			relayUploadWriteJSON(w, http.StatusForbidden, relayUploadError("relay_capability_wrong_binding"))
		default:
			t.Fatalf("unexpected core path %s", r.URL.Path)
		}
	}), streamIngressConfig{
		MaxUploadBytes:        256 * 1024,
		TempStagingQuotaBytes: 512 * 1024,
	})

	response, body := postRelayUpload(t, handler, input, payload)
	if response.Code != http.StatusForbidden {
		t.Fatalf("relay upload status = %d, want 403: %s", response.Code, body)
	}
	assertErrorCode(t, body, "core_commit_rejected")
	if !bytes.Contains(body, []byte(`"core_error_code":"relay_capability_wrong_binding"`)) {
		t.Fatalf("relay response omitted safe core error code: %s", body)
	}
	assertRelayTempDirEmpty(t, handler)
}

func TestRelayUploadRejectsTempStagingPressureAndCleansStaging(t *testing.T) {
	payload := []byte("opaque encrypted relay chunk")
	input := relayUploadTestMetadata("relay-session", "capability-token", "inc_test", "str_test", 1, "audio", payload)
	var commitCalls atomic.Int64
	handler := newRelayUploadTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertCoreServiceAuth(t, r)
		switch r.URL.Path {
		case "/v1/relay/preflight":
			relayUploadWriteJSON(w, http.StatusOK, map[string]any{
				"relay_preflight": map[string]any{"status": "accepted"},
			})
		case "/v1/relay/commit":
			commitCalls.Add(1)
			relayUploadWriteJSON(w, http.StatusCreated, relayUploadCoreCommitResponse(input))
		default:
			t.Fatalf("unexpected core path %s", r.URL.Path)
		}
	}), streamIngressConfig{
		TempStagingQuotaBytes: 1,
	})

	response, body := postRelayUpload(t, handler, input, payload)
	if response.Code != http.StatusInsufficientStorage {
		t.Fatalf("relay upload status = %d, want 507: %s", response.Code, body)
	}
	assertErrorCode(t, body, "relay_temp_staging_quota_exceeded")
	if commitCalls.Load() != 0 {
		t.Fatalf("commit calls = %d, want 0", commitCalls.Load())
	}
	assertRelayTempDirEmpty(t, handler)
}

func TestRelayUploadCoreTimeoutCleansStaging(t *testing.T) {
	payload := []byte("opaque encrypted relay chunk")
	input := relayUploadTestMetadata("relay-session", "capability-token", "inc_test", "str_test", 1, "audio", payload)
	handler := newRelayUploadTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertCoreServiceAuth(t, r)
		switch r.URL.Path {
		case "/v1/relay/preflight":
			relayUploadWriteJSON(w, http.StatusOK, map[string]any{
				"relay_preflight": map[string]any{"status": "accepted"},
			})
		case "/v1/relay/commit":
			time.Sleep(200 * time.Millisecond)
			relayUploadWriteJSON(w, http.StatusCreated, relayUploadCoreCommitResponse(input))
		default:
			t.Fatalf("unexpected core path %s", r.URL.Path)
		}
	}), streamIngressConfig{
		CoreRequestTimeout: 20 * time.Millisecond,
	})

	response, body := postRelayUpload(t, handler, input, payload)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("relay upload status = %d, want 503: %s", response.Code, body)
	}
	assertErrorCode(t, body, "core_commit_unavailable")
	assertRelayTempDirEmpty(t, handler)
}

func TestRelayUploadRejectsConcurrentDuplicateChunk(t *testing.T) {
	payload := []byte("opaque encrypted relay chunk")
	input := relayUploadTestMetadata("relay-session", "capability-token", "inc_test", "str_test", 1, "audio", payload)
	preflightStarted := make(chan struct{})
	releasePreflight := make(chan struct{})
	var preflightCalls atomic.Int64
	handler := newRelayUploadTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertCoreServiceAuth(t, r)
		switch r.URL.Path {
		case "/v1/relay/preflight":
			if preflightCalls.Add(1) == 1 {
				close(preflightStarted)
				<-releasePreflight
			}
			relayUploadWriteJSON(w, http.StatusOK, map[string]any{
				"relay_preflight": map[string]any{"status": "accepted"},
			})
		case "/v1/relay/commit":
			relayUploadWriteJSON(w, http.StatusCreated, relayUploadCoreCommitResponse(input))
		default:
			t.Fatalf("unexpected core path %s", r.URL.Path)
		}
	}), streamIngressConfig{})

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		response, body := postRelayUpload(t, handler, input, payload)
		if response.Code != http.StatusCreated {
			t.Errorf("first relay upload status = %d, want 201: %s", response.Code, body)
		}
	}()

	select {
	case <-preflightStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first preflight")
	}
	response, body := postRelayUpload(t, handler, input, payload)
	if response.Code != http.StatusConflict {
		t.Fatalf("duplicate relay upload status = %d, want 409: %s", response.Code, body)
	}
	assertErrorCode(t, body, "relay_chunk_in_progress")
	close(releasePreflight)

	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first upload")
	}
	assertRelayTempDirEmpty(t, handler)
}

type relayUploadTestHandler struct {
	http.Handler
	dataDir string
}

func newRelayUploadTestHandler(t *testing.T, core http.Handler, overrides streamIngressConfig) relayUploadTestHandler {
	t.Helper()
	coreServer := httptest.NewServer(core)
	t.Cleanup(coreServer.Close)
	cfg := streamIngressConfig{
		CoreBaseURL:           coreServer.URL,
		CoreServiceAuthToken:  testCoreServiceAuthToken,
		DataDir:               t.TempDir(),
		MaxUploadBytes:        4096,
		TempStagingQuotaBytes: 8192,
		CoreRequestTimeout:    time.Second,
		MaxInFlightPerSession: 2,
		MaxInFlightPerClient:  2,
	}
	if overrides.CoreBaseURL != "" {
		cfg.CoreBaseURL = overrides.CoreBaseURL
	}
	if overrides.CoreServiceAuthToken != "" {
		cfg.CoreServiceAuthToken = overrides.CoreServiceAuthToken
	}
	if overrides.DataDir != "" {
		cfg.DataDir = overrides.DataDir
	}
	if overrides.MaxUploadBytes != 0 {
		cfg.MaxUploadBytes = overrides.MaxUploadBytes
	}
	if overrides.TempStagingQuotaBytes != 0 {
		cfg.TempStagingQuotaBytes = overrides.TempStagingQuotaBytes
	}
	if overrides.CoreRequestTimeout != 0 {
		cfg.CoreRequestTimeout = overrides.CoreRequestTimeout
	}
	if overrides.MaxInFlightPerSession != 0 {
		cfg.MaxInFlightPerSession = overrides.MaxInFlightPerSession
	}
	if overrides.MaxInFlightPerClient != 0 {
		cfg.MaxInFlightPerClient = overrides.MaxInFlightPerClient
	}
	uploader, err := newRelayUploader(cfg, nil)
	if err != nil {
		t.Fatalf("newRelayUploader: %v", err)
	}
	return relayUploadTestHandler{
		Handler: newHandler(cfg, uploader),
		dataDir: cfg.DataDir,
	}
}

func postRelayUpload(t *testing.T, handler http.Handler, input relayUploadMetadata, payload []byte) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	contentType, body, err := relayUploadMultipartBody(input, payload)
	if err != nil {
		t.Fatalf("relay upload multipart body: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/upload/complete-chunk", body)
	request.Header.Set("Content-Type", contentType)
	request.RemoteAddr = "192.0.2.55:12345"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder, recorder.Body.Bytes()
}

func relayUploadTestMetadata(sessionID, capability, incidentID, streamID string, chunkIndex int, mediaType string, payload []byte) relayUploadMetadata {
	return relayUploadMetadata{
		RelaySessionID:   sessionID,
		Capability:       capability,
		IncidentID:       incidentID,
		StreamID:         streamID,
		ChunkIndex:       chunkIndex,
		MediaType:        mediaType,
		StartedAt:        "2026-06-11T10:00:00Z",
		EndedAt:          "2026-06-11T10:00:10Z",
		ByteSize:         int64(len(payload)),
		SHA256Hex:        relayUploadSHA256(payload),
		OriginalFilename: "chunk.pq",
	}
}

func relayUploadSHA256(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func relayUploadMultipartBody(input relayUploadMetadata, payload []byte) (string, io.Reader, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range input.formFields() {
		if err := writer.WriteField(name, value); err != nil {
			return "", nil, err
		}
	}
	fileWriter, err := writer.CreateFormFile("file", input.OriginalFilename)
	if err != nil {
		return "", nil, err
	}
	if _, err := fileWriter.Write(payload); err != nil {
		return "", nil, err
	}
	if err := writer.Close(); err != nil {
		return "", nil, err
	}
	return writer.FormDataContentType(), &body, nil
}

func relayUploadCoreCommitResponse(input relayUploadMetadata) map[string]any {
	return map[string]any{
		"relay_commit": map[string]any{
			"status":      "committed",
			"chunk_id":    "chk_test",
			"incident_id": input.IncidentID,
			"stream_id":   input.StreamID,
			"chunk_index": input.ChunkIndex,
			"media_type":  input.MediaType,
			"byte_size":   input.ByteSize,
			"sha256_hex":  input.SHA256Hex,
			"created_at":  "2026-06-11T10:00:11Z",
		},
	}
}

func relayUploadError(code string) map[string]any {
	return map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": "core rejected relay upload",
		},
	}
}

func relayUploadWriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func assertCoreServiceAuth(t *testing.T, r *http.Request) {
	t.Helper()
	if r.Header.Get(relayCoreServiceAuthHeader) != testCoreServiceAuthToken {
		t.Fatalf("core service auth header = %q, want configured token", r.Header.Get(relayCoreServiceAuthHeader))
	}
}

func assertCorePreflightBody(t *testing.T, r *http.Request, want relayUploadMetadata) {
	t.Helper()
	var got map[string]any
	if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
		t.Fatalf("decode core preflight body: %v", err)
	}
	for field, wantValue := range map[string]string{
		"relay_session_id": want.RelaySessionID,
		"capability":       want.Capability,
		"incident_id":      want.IncidentID,
		"stream_id":        want.StreamID,
		"media_type":       want.MediaType,
		"started_at":       want.StartedAt,
		"ended_at":         want.EndedAt,
		"sha256_hex":       want.SHA256Hex,
	} {
		if got[field] != wantValue {
			t.Fatalf("core preflight %s = %v, want %q", field, got[field], wantValue)
		}
	}
	if got["chunk_index"] != float64(want.ChunkIndex) || got["byte_size"] != float64(want.ByteSize) {
		t.Fatalf("core preflight numeric fields = %v, want chunk %d bytes %d", got, want.ChunkIndex, want.ByteSize)
	}
}

func assertCoreCommitMultipart(t *testing.T, r *http.Request, want relayUploadMetadata, payload []byte) {
	t.Helper()
	if err := r.ParseMultipartForm(8192); err != nil {
		t.Fatalf("parse core commit multipart: %v", err)
	}
	for field, wantValue := range want.formFields() {
		if got := r.FormValue(field); got != wantValue {
			t.Fatalf("core commit %s = %q, want %q", field, got, wantValue)
		}
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		t.Fatalf("core commit file: %v", err)
	}
	defer file.Close()
	gotPayload, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read core commit file: %v", err)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Fatalf("core commit payload = %q, want %q", gotPayload, payload)
	}
}

func assertRelayUploadStatus(t *testing.T, body []byte, want string) {
	t.Helper()
	var decoded map[string]map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode relay upload response: %v", err)
	}
	if decoded["relay_upload"]["status"] != want {
		t.Fatalf("relay upload status = %v, want %q in %s", decoded["relay_upload"]["status"], want, body)
	}
}

func assertErrorCode(t *testing.T, body []byte, want string) {
	t.Helper()
	var decoded struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if decoded.Error.Code != want {
		t.Fatalf("error code = %q, want %q in %s", decoded.Error.Code, want, body)
	}
}

func assertRedactedRelayUploadBody(t *testing.T, body []byte, disallowed ...string) {
	t.Helper()
	for _, value := range disallowed {
		if value != "" && bytes.Contains(body, []byte(value)) {
			t.Fatalf("relay upload response exposed %q: %s", value, body)
		}
	}
	for _, value := range []string{"stored_path", "object_key", "Authorization", "plaintext", "raw_key", "wrapped_key"} {
		if bytes.Contains(body, []byte(value)) {
			t.Fatalf("relay upload response exposed %q: %s", value, body)
		}
	}
}

func assertRelayTempDirEmpty(t *testing.T, handler relayUploadTestHandler) {
	t.Helper()
	tempDir := filepath.Join(handler.dataDir, "tmp")
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "upload-") {
			t.Fatalf("temp upload was not cleaned up: %s", entry.Name())
		}
	}
}
