package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/coordination"
	"github.com/open-proofline/server/internal/httpapi"
	"github.com/open-proofline/server/internal/incidents"
)

func TestUploadValidChunk(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)

	payload := []byte("encrypted audio data")
	response, body := uploadChunk(t, app, incidentID, 1, "audio", payload, sha256Hex(payload))
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", response.StatusCode, body)
	}
	var chunk incidents.Chunk
	if err := json.Unmarshal(body, &chunk); err != nil {
		t.Fatalf("decode chunk: %v", err)
	}
	if chunk.MediaType != "audio" || chunk.ChunkIndex != 1 || chunk.ByteSize != int64(len(payload)) {
		t.Fatalf("unexpected chunk response: %+v", chunk)
	}

	storedPath := filepath.Join(app.dataDir, "incidents", incidentID, "audio_000001.enc")
	stored, err := os.ReadFile(storedPath)
	if err != nil {
		t.Fatalf("read stored chunk: %v", err)
	}
	if !bytes.Equal(stored, payload) {
		t.Fatalf("stored payload mismatch")
	}

	response, body = get(t, app, "/v1/incidents/"+incidentID+"/chunks/audio/1")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected chunk bytes status 200, got %d: %s", response.StatusCode, body)
	}
	assertNoSniff(t, response)
	assertNoStore(t, response)
}

func TestLegacyUnstreamedChunkIndexZeroIsAccepted(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	payload := []byte("legacy encrypted audio data")

	response, body := uploadChunk(t, app, incidentID, 0, "audio", payload, sha256Hex(payload))
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected legacy zero-index upload status 201, got %d: %s", response.StatusCode, body)
	}
	var chunk incidents.Chunk
	if err := json.Unmarshal(body, &chunk); err != nil {
		t.Fatalf("decode chunk: %v", err)
	}
	if chunk.StreamID != "" || chunk.ChunkIndex != 0 {
		t.Fatalf("unexpected legacy chunk response: %+v", chunk)
	}
}

func TestRejectDuplicateChunkIndex(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	payload := []byte("encrypted audio data")
	duplicatePayload := []byte("different encrypted audio data")

	response, body := uploadChunk(t, app, incidentID, 1, "audio", payload, sha256Hex(payload))
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected first upload status 201, got %d: %s", response.StatusCode, body)
	}

	response, body = uploadChunk(t, app, incidentID, 1, "audio", duplicatePayload, sha256Hex(duplicatePayload))
	defer response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected duplicate status 409, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "duplicate_chunk")

	storedPath := filepath.Join(app.dataDir, "incidents", incidentID, "audio_000001.enc")
	stored, err := os.ReadFile(storedPath)
	if err != nil {
		t.Fatalf("read stored chunk: %v", err)
	}
	if !bytes.Equal(stored, payload) {
		t.Fatalf("duplicate upload overwrote stored payload")
	}
	assertTempDirEmpty(t, app)
}

func TestUploadIdempotencyKeyEquivalentRetryReturnsSuccess(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "audio recording")
	payload := []byte("encrypted stream audio data")
	key := "chunk-upload-key-1"

	response, body := uploadChunkWithIdempotencyKey(t, app, incidentID, stream.ID, 1, incidents.MediaTypeAudio, payload, sha256Hex(payload), "chunk.enc", key)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected first upload status 201, got %d: %s", response.StatusCode, body)
	}
	var first incidents.Chunk
	if err := json.Unmarshal(body, &first); err != nil {
		t.Fatalf("decode first chunk: %v", err)
	}

	response, body = uploadChunkWithIdempotencyKey(t, app, incidentID, stream.ID, 1, incidents.MediaTypeAudio, payload, sha256Hex(payload), "chunk.enc", key)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected idempotent retry status 200, got %d: %s", response.StatusCode, body)
	}
	if response.Header.Get("Idempotency-Replayed") != "true" {
		t.Fatalf("expected Idempotency-Replayed header, got %q", response.Header.Get("Idempotency-Replayed"))
	}
	var replayed incidents.Chunk
	if err := json.Unmarshal(body, &replayed); err != nil {
		t.Fatalf("decode replayed chunk: %v", err)
	}
	if replayed.ID != first.ID || replayed.StoredPath != first.StoredPath {
		t.Fatalf("expected replayed chunk to match first response: first=%+v replayed=%+v", first, replayed)
	}

	storedPath := filepath.Join(app.dataDir, "incidents", incidentID, "streams", stream.ID, "audio_000001.enc")
	stored, err := os.ReadFile(storedPath)
	if err != nil {
		t.Fatalf("read stored chunk: %v", err)
	}
	if !bytes.Equal(stored, payload) {
		t.Fatalf("idempotent retry changed stored payload")
	}

	var storedHash string
	var operationState string
	if err := app.db.QueryRowContext(t.Context(), `
		SELECT idempotency_key_hash, state
		FROM upload_operations
		WHERE chunk_id = ?`,
		first.ID,
	).Scan(&storedHash, &operationState); err != nil {
		t.Fatalf("read upload operation: %v", err)
	}
	if storedHash == key || len(storedHash) != 64 {
		t.Fatalf("idempotency key was not stored as a 64-character hash")
	}
	if operationState != incidents.UploadOperationStateMetadataCommitted {
		t.Fatalf("operation state = %q, want %q", operationState, incidents.UploadOperationStateMetadataCommitted)
	}
	assertTempDirEmpty(t, app)
}

func TestUploadIdempotencyKeyReuseWithDifferentInputsConflicts(t *testing.T) {
	var logs bytes.Buffer
	app := newTestAppWithMaxUploadBytesAndLogger(t, 1024*1024, slog.New(slog.NewTextHandler(&logs, nil)))
	incidentID := createIncident(t, app, `{}`)
	payload := []byte("encrypted audio data")
	key := "raw-idempotency-key-secret"

	response, body := uploadChunkWithIdempotencyKey(t, app, incidentID, "", 1, incidents.MediaTypeAudio, payload, sha256Hex(payload), "chunk.enc", key)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected first upload status 201, got %d: %s", response.StatusCode, body)
	}

	response, body = uploadChunkWithIdempotencyKey(t, app, incidentID, "", 1, incidents.MediaTypeAudio, payload, sha256Hex(payload), "other-name.enc", key)
	defer response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected idempotency conflict status 409, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "idempotency_conflict")
	for _, disallowed := range []string{key, "other-name.enc", string(payload), app.dataDir} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("idempotency conflict response exposed %q: %s", disallowed, body)
		}
		if strings.TrimSpace(disallowed) != "" && bytes.Contains(logs.Bytes(), []byte(disallowed)) {
			t.Fatalf("idempotency conflict logs exposed %q: %s", disallowed, logs.String())
		}
	}
}

func TestDuplicateChunkWithoutIdempotencyKeyKeepsExistingBehavior(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	payload := []byte("encrypted audio data")

	response, body := uploadChunk(t, app, incidentID, 1, incidents.MediaTypeAudio, payload, sha256Hex(payload))
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected first upload status 201, got %d: %s", response.StatusCode, body)
	}
	response, body = uploadChunk(t, app, incidentID, 1, incidents.MediaTypeAudio, payload, sha256Hex(payload))
	defer response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected duplicate status 409, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "duplicate_chunk")

	var operations int
	if err := app.db.QueryRowContext(t.Context(), `
		SELECT count(*)
		FROM upload_operations`,
	).Scan(&operations); err != nil {
		t.Fatalf("count upload operations: %v", err)
	}
	if operations != 0 {
		t.Fatalf("unexpected upload operation rows for no-key upload: %d", operations)
	}
}

func TestUploadCoordinationUsesSafeLeaseKey(t *testing.T) {
	coord := &recordingUploadCoordinator{
		lease: coordination.UploadLease{Acquired: true, Token: "server-generated-lease-token"},
	}
	app := newTestAppWithOptions(t, httpapi.Options{
		UploadCoordinator:          coord,
		UploadCoordinationLeaseTTL: 45 * time.Second,
	})
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "audio recording")
	payload := []byte("encrypted stream audio data")

	response, body := uploadChunkWithStream(t, app, incidentID, stream.ID, 1, incidents.MediaTypeAudio, payload, sha256Hex(payload))
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected coordinated upload status 201, got %d: %s", response.StatusCode, body)
	}
	if len(coord.acquireCalls) != 1 {
		t.Fatalf("lease acquire calls = %d, want 1", len(coord.acquireCalls))
	}
	call := coord.acquireCalls[0]
	if !strings.HasPrefix(call.key, "proofline:upload-operation:v1:") {
		t.Fatalf("unexpected coordination key %q", call.key)
	}
	if call.ttl != 45*time.Second {
		t.Fatalf("coordination ttl = %s, want 45s", call.ttl)
	}
	for _, disallowed := range []string{incidentID, stream.ID, incidents.MediaTypeAudio, "chunk.enc", string(payload)} {
		if strings.Contains(call.key, disallowed) {
			t.Fatalf("coordination key exposed %q: %s", disallowed, call.key)
		}
	}
	if len(coord.releaseCalls) != 1 {
		t.Fatalf("lease release calls = %d, want 1", len(coord.releaseCalls))
	}
	if coord.releaseCalls[0].Key != call.key || coord.releaseCalls[0].Token == "" {
		t.Fatalf("release did not use acquired lease: %+v", coord.releaseCalls[0])
	}
}

func TestUploadCoordinationBusyReturnsRetryableHint(t *testing.T) {
	coord := &recordingUploadCoordinator{
		lease: coordination.UploadLease{Acquired: false, RetryAfter: 15 * time.Second},
	}
	app := newTestAppWithOptions(t, httpapi.Options{
		UploadCoordinator:          coord,
		UploadCoordinationLeaseTTL: time.Minute,
	})
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "audio recording")
	payload := []byte("encrypted stream audio data")

	response, body := uploadChunkWithStream(t, app, incidentID, stream.ID, 1, incidents.MediaTypeAudio, payload, sha256Hex(payload))
	defer response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected in-progress status 409, got %d: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)
	assertErrorCode(t, body, "upload_in_progress")
	if response.Header.Get("Retry-After") != "15" {
		t.Fatalf("Retry-After = %q, want 15", response.Header.Get("Retry-After"))
	}
	for _, disallowed := range []string{incidentID, stream.ID, string(payload)} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("upload coordination response exposed %q: %s", disallowed, body)
		}
	}
	if len(coord.releaseCalls) != 0 {
		t.Fatalf("unexpected release calls for busy lease: %d", len(coord.releaseCalls))
	}
	assertNoStoredFile(t, app, incidentID, "streams/"+stream.ID+"/audio_000001.enc")
}

func TestUploadCoordinationUnavailableReturnsSafeRetryableError(t *testing.T) {
	var logs bytes.Buffer
	coord := &recordingUploadCoordinator{
		err: errors.New("dial 10.0.0.5:6379 with password secret failed"),
	}
	app := newTestAppWithOptions(t, httpapi.Options{
		UploadCoordinator:          coord,
		UploadCoordinationLeaseTTL: time.Minute,
		Logger:                     slog.New(slog.NewTextHandler(&logs, nil)),
	})
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "audio recording")
	payload := []byte("encrypted stream audio data")

	response, body := uploadChunkWithStream(t, app, incidentID, stream.ID, 1, incidents.MediaTypeAudio, payload, sha256Hex(payload))
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected coordination failure status 503, got %d: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)
	assertErrorCode(t, body, "upload_coordination_unavailable")
	if response.Header.Get("Retry-After") != "60" {
		t.Fatalf("Retry-After = %q, want 60", response.Header.Get("Retry-After"))
	}
	for _, disallowed := range []string{"10.0.0.5", "secret", incidentID, stream.ID, string(payload)} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("upload coordination response exposed %q: %s", disallowed, body)
		}
		if bytes.Contains(logs.Bytes(), []byte(disallowed)) {
			t.Fatalf("upload coordination logs exposed %q: %s", disallowed, logs.String())
		}
	}
}

func TestReconcileStreamedDuplicateMatched(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "audio recording")
	payload := []byte("encrypted stream audio data")

	response, body := uploadChunkWithStream(t, app, incidentID, stream.ID, 1, incidents.MediaTypeAudio, payload, sha256Hex(payload))
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected upload status 201, got %d: %s", response.StatusCode, body)
	}
	var chunk incidents.Chunk
	if err := json.Unmarshal(body, &chunk); err != nil {
		t.Fatalf("decode chunk: %v", err)
	}

	response, body = reconcileChunk(t, app, incidentID, reconcileChunkRequest(stream.ID, 1, incidents.MediaTypeAudio, payload, "chunk.enc"))
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected reconciliation status 200, got %d: %s", response.StatusCode, body)
	}

	result := decodeReconciliationResponse(t, body)
	if result.Reconciliation.Status != "matched" {
		t.Fatalf("expected matched reconciliation, got %+v", result.Reconciliation)
	}
	if result.Reconciliation.ChunkID != chunk.ID || result.Reconciliation.Identity.StreamID != stream.ID {
		t.Fatalf("unexpected reconciliation response: %+v", result.Reconciliation)
	}
	if result.Reconciliation.ByteSize != int64(len(payload)) || result.Reconciliation.SHA256Hex != sha256Hex(payload) {
		t.Fatalf("unexpected matched fingerprint: %+v", result.Reconciliation)
	}
	for _, disallowed := range []string{"stored_path", chunk.StoredPath, "original_filename", "chunk.enc"} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("matched reconciliation response exposed %q: %s", disallowed, body)
		}
	}
}

func TestReconcileStreamedDuplicateConflictOmitsStoredValues(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "audio recording")
	storedPayload := []byte("encrypted stream audio data")
	expectedPayload := []byte("different encrypted stream audio data")

	response, body := uploadChunkWithOptions(t, app, incidentID, stream.ID, 1, incidents.MediaTypeAudio, storedPayload, sha256Hex(storedPayload), "stored-name.enc", "")
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected upload status 201, got %d: %s", response.StatusCode, body)
	}
	var chunk incidents.Chunk
	if err := json.Unmarshal(body, &chunk); err != nil {
		t.Fatalf("decode chunk: %v", err)
	}

	response, body = reconcileChunk(t, app, incidentID, reconcileChunkRequest(stream.ID, 1, incidents.MediaTypeAudio, expectedPayload, "expected-name.enc"))
	defer response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected reconciliation conflict status 409, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "duplicate_chunk_conflict")

	result := decodeReconciliationResponse(t, body)
	wantFields := []string{"original_filename", "byte_size", "sha256_hex"}
	if !stringSlicesEqual(result.Reconciliation.MismatchedFields, wantFields) {
		t.Fatalf("mismatched_fields = %#v, want %#v", result.Reconciliation.MismatchedFields, wantFields)
	}
	for _, disallowed := range []string{
		"stored_path",
		chunk.StoredPath,
		"stored-name.enc",
		"expected-name.enc",
		sha256Hex(storedPayload),
		sha256Hex(expectedPayload),
		string(storedPayload),
		string(expectedPayload),
		app.dataDir,
	} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("conflict reconciliation response exposed %q: %s", disallowed, body)
		}
	}
}

func TestReconcileLegacyDuplicateMatchedAndConflict(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	storedPayload := []byte("legacy encrypted audio data")
	expectedPayload := []byte("different legacy encrypted audio data")

	response, body := uploadChunk(t, app, incidentID, 0, incidents.MediaTypeAudio, storedPayload, sha256Hex(storedPayload))
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected upload status 201, got %d: %s", response.StatusCode, body)
	}

	response, body = reconcileChunk(t, app, incidentID, reconcileChunkRequest("", 0, incidents.MediaTypeAudio, storedPayload, "chunk.enc"))
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected legacy reconciliation status 200, got %d: %s", response.StatusCode, body)
	}
	result := decodeReconciliationResponse(t, body)
	if result.Reconciliation.Status != "matched" || result.Reconciliation.Identity.StreamID != "" || result.Reconciliation.Identity.ChunkIndex != 0 {
		t.Fatalf("unexpected legacy matched response: %+v", result.Reconciliation)
	}

	response, body = reconcileChunk(t, app, incidentID, reconcileChunkRequest("", 0, incidents.MediaTypeAudio, expectedPayload, "chunk.enc"))
	defer response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected legacy reconciliation conflict status 409, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "duplicate_chunk_conflict")
	result = decodeReconciliationResponse(t, body)
	wantFields := []string{"byte_size", "sha256_hex"}
	if !stringSlicesEqual(result.Reconciliation.MismatchedFields, wantFields) {
		t.Fatalf("mismatched_fields = %#v, want %#v", result.Reconciliation.MismatchedFields, wantFields)
	}
}

func TestReconcileChunkNotFound(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	payload := []byte("encrypted audio data")

	response, body := reconcileChunk(t, app, incidentID, reconcileChunkRequest("", 99, incidents.MediaTypeAudio, payload, "chunk.enc"))
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected missing chunk reconciliation status 404, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "chunk_not_found")
}

func TestReconcileDuplicateAfterClosedIncidentAndTerminalStreams(t *testing.T) {
	app := newTestApp(t)

	closedIncidentID := createIncident(t, app, `{}`)
	legacyPayload := []byte("closed incident encrypted data")
	response, body := uploadChunk(t, app, closedIncidentID, 1, incidents.MediaTypeAudio, legacyPayload, sha256Hex(legacyPayload))
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected closed-incident setup upload status 201, got %d: %s", response.StatusCode, body)
	}
	response, body = post(t, app, "/v1/incidents/"+closedIncidentID+"/close", "application/json", bytes.NewBufferString(`{}`))
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected close status 200, got %d: %s", response.StatusCode, body)
	}
	response, body = reconcileChunk(t, app, closedIncidentID, reconcileChunkRequest("", 1, incidents.MediaTypeAudio, legacyPayload, "chunk.enc"))
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected closed-incident reconciliation status 200, got %d: %s", response.StatusCode, body)
	}

	completedIncidentID, completedStream := createIncidentStreamWithChunks(t, app, 1)
	completeMediaStream(t, app, completedIncidentID, completedStream.ID, 1)
	completedPayload := []byte("encrypted audio data 1")
	response, body = reconcileChunk(t, app, completedIncidentID, reconcileChunkRequest(completedStream.ID, 1, incidents.MediaTypeAudio, completedPayload, "chunk.enc"))
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected completed-stream reconciliation status 200, got %d: %s", response.StatusCode, body)
	}

	failedIncidentID := createIncident(t, app, `{}`)
	failedStream := createMediaStream(t, app, failedIncidentID, incidents.MediaTypeAudio, "failed audio")
	failedPayload := []byte("failed stream encrypted audio data")
	response, body = uploadChunkWithStream(t, app, failedIncidentID, failedStream.ID, 1, incidents.MediaTypeAudio, failedPayload, sha256Hex(failedPayload))
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected failed-stream setup upload status 201, got %d: %s", response.StatusCode, body)
	}
	response, body = post(t, app, "/v1/incidents/"+failedIncidentID+"/streams/"+failedStream.ID+"/fail", "application/json", bytes.NewBufferString(`{"failure_reason":"stopped"}`))
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected fail stream status 200, got %d: %s", response.StatusCode, body)
	}
	response, body = reconcileChunk(t, app, failedIncidentID, reconcileChunkRequest(failedStream.ID, 1, incidents.MediaTypeAudio, failedPayload, "chunk.enc"))
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected failed-stream reconciliation status 200, got %d: %s", response.StatusCode, body)
	}
}

func TestRejectHashMismatchRemovesTempFile(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	payload := []byte("encrypted audio data")

	response, body := uploadChunk(t, app, incidentID, 1, "audio", payload, stringsOf("0", 64))
	defer response.Body.Close()

	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected hash mismatch status 400, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "hash_mismatch")
	assertNoStoredFile(t, app, incidentID, "audio_000001.enc")
	assertTempDirEmpty(t, app)
}

func TestRejectUploadTooLargeRemovesTempFile(t *testing.T) {
	app := newTestAppWithMaxUploadBytes(t, 8)
	incidentID := createIncident(t, app, `{}`)
	payload := []byte("this encrypted payload is too large")

	response, body := uploadChunk(t, app, incidentID, 1, "audio", payload, sha256Hex(payload))
	defer response.Body.Close()

	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected upload too large status 413, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "upload_too_large")
	assertNoStoredFile(t, app, incidentID, "audio_000001.enc")
	assertTempDirEmpty(t, app)
}

func TestHugeConfiguredUploadLimitDoesNotOverflowRequestLimit(t *testing.T) {
	app := newTestAppWithMaxUploadBytes(t, int64(1<<63-1))
	incidentID := createIncident(t, app, `{}`)
	payload := []byte("encrypted audio data")

	response, body := uploadChunk(t, app, incidentID, 1, "audio", payload, sha256Hex(payload))
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected upload status 201, got %d: %s", response.StatusCode, body)
	}
}

func TestRejectUploadToMissingIncident(t *testing.T) {
	app := newTestApp(t)
	payload := []byte("encrypted audio data")

	response, body := uploadChunk(t, app, "inc_missing", 1, "audio", payload, sha256Hex(payload))
	defer response.Body.Close()

	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected missing incident status 404, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "incident_not_found")
}

func reconcileChunk(t *testing.T, app *testApp, incidentID string, requestBody map[string]any) (*http.Response, []byte) {
	t.Helper()

	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("marshal reconciliation request: %v", err)
	}
	return post(t, app, "/v1/incidents/"+incidentID+"/chunks/reconcile", "application/json", bytes.NewReader(body))
}

func reconcileChunkRequest(streamID string, index int, mediaType string, payload []byte, originalFilename string) map[string]any {
	request := map[string]any{
		"chunk_index":       index,
		"media_type":        mediaType,
		"started_at":        testChunkStartedAtString(),
		"ended_at":          testChunkEndedAtString(),
		"byte_size":         int64(len(payload)),
		"sha256_hex":        sha256Hex(payload),
		"original_filename": originalFilename,
	}
	if streamID != "" {
		request["stream_id"] = streamID
	}
	return request
}

type reconciliationResponse struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
	Reconciliation struct {
		Status   string `json:"status"`
		Identity struct {
			IncidentID string `json:"incident_id"`
			StreamID   string `json:"stream_id"`
			ChunkIndex int    `json:"chunk_index"`
			MediaType  string `json:"media_type"`
		} `json:"identity"`
		ChunkID          string   `json:"chunk_id"`
		ByteSize         int64    `json:"byte_size"`
		SHA256Hex        string   `json:"sha256_hex"`
		MismatchedFields []string `json:"mismatched_fields"`
	} `json:"reconciliation"`
}

func decodeReconciliationResponse(t *testing.T, body []byte) reconciliationResponse {
	t.Helper()

	var result reconciliationResponse
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode reconciliation response: %v", err)
	}
	return result
}

func stringSlicesEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

type recordingUploadCoordinator struct {
	lease        coordination.UploadLease
	err          error
	releaseErr   error
	acquireCalls []uploadCoordinationCall
	releaseCalls []coordination.UploadLease
}

type uploadCoordinationCall struct {
	key string
	ttl time.Duration
}

func (c *recordingUploadCoordinator) Check(context.Context) error {
	return nil
}

func (c *recordingUploadCoordinator) AcquireUploadLease(_ context.Context, key string, ttl time.Duration) (coordination.UploadLease, error) {
	c.acquireCalls = append(c.acquireCalls, uploadCoordinationCall{key: key, ttl: ttl})
	if c.err != nil {
		return coordination.UploadLease{}, c.err
	}
	lease := c.lease
	if lease.Key == "" {
		lease.Key = key
	}
	if lease.Acquired && lease.Token == "" {
		lease.Token = "server-generated-lease-token"
	}
	return lease, nil
}

func (c *recordingUploadCoordinator) ReleaseUploadLease(_ context.Context, lease coordination.UploadLease) error {
	c.releaseCalls = append(c.releaseCalls, lease)
	return c.releaseErr
}

func (c *recordingUploadCoordinator) Close() error {
	return nil
}
