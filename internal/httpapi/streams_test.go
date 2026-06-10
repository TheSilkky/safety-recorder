package httpapi_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pqenv "github.com/open-proofline/server/internal/envelope/pq"
	"github.com/open-proofline/server/internal/incidents"
)

func TestCreateMediaStream(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)

	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "main audio recording")

	if stream.ID == "" {
		t.Fatal("expected stream id")
	}
	if stream.IncidentID != incidentID || stream.MediaType != incidents.MediaTypeAudio || stream.Status != incidents.StreamStatusOpen {
		t.Fatalf("unexpected stream: %+v", stream)
	}
	if stream.Label != "main audio recording" {
		t.Fatalf("expected stream label to round trip, got %q", stream.Label)
	}
}

func TestRejectInvalidMediaStreamType(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)

	response, body := post(t, app, "/v1/incidents/"+incidentID+"/streams", "application/json", bytes.NewBufferString(`{"media_type":"screen"}`))
	defer response.Body.Close()

	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid media type status 400, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "invalid_media_type")
}

func TestUploadChunkWithValidStreamID(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "audio")
	payload := []byte("encrypted audio data")
	expectedPayload := testPQPayload(t, incidentID, stream.ID, 1, "audio", payload)

	response, body := uploadChunkWithStream(t, app, incidentID, stream.ID, 1, "audio", payload, sha256Hex(payload))
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected stream upload status 201, got %d: %s", response.StatusCode, body)
	}
	var chunk incidents.Chunk
	if err := json.Unmarshal(body, &chunk); err != nil {
		t.Fatalf("decode chunk: %v", err)
	}
	if chunk.StreamID != stream.ID {
		t.Fatalf("expected chunk stream_id %s, got %q", stream.ID, chunk.StreamID)
	}
	wantStoredPath := "incidents/" + incidentID + "/streams/" + stream.ID + "/audio_000001.enc"
	if chunk.StoredPath != wantStoredPath {
		t.Fatalf("expected stored path %q, got %q", wantStoredPath, chunk.StoredPath)
	}
	stored, err := os.ReadFile(filepath.Join(app.dataDir, "incidents", incidentID, "streams", stream.ID, "audio_000001.enc"))
	if err != nil {
		t.Fatalf("read stored stream chunk: %v", err)
	}
	if !bytes.Equal(stored, expectedPayload) {
		t.Fatal("stored stream payload mismatch")
	}
	assertPQPayloadFrame(t, stored, incidentID, stream.ID, 1, "audio")
}

func TestTwoSameMediaStreamsAllowOverlappingChunkIndexes(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	firstStream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "first audio")
	secondStream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "second audio")
	firstPayload := []byte("first encrypted audio data")
	secondPayload := []byte("second encrypted audio data")
	duplicatePayload := []byte("duplicate first stream chunk")
	legacyPayload := []byte("legacy encrypted audio data")
	firstPQPayload := testPQPayload(t, incidentID, firstStream.ID, 1, "audio", firstPayload)
	secondPQPayload := testPQPayload(t, incidentID, secondStream.ID, 1, "audio", secondPayload)

	response, body := uploadChunkWithStream(t, app, incidentID, firstStream.ID, 1, "audio", firstPayload, sha256Hex(firstPayload))
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected first stream upload status 201, got %d: %s", response.StatusCode, body)
	}
	response, body = uploadChunkWithStream(t, app, incidentID, firstStream.ID, 1, "audio", duplicatePayload, sha256Hex(duplicatePayload))
	response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected duplicate first stream upload status 409, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "duplicate_chunk")
	response, body = uploadChunkWithStream(t, app, incidentID, secondStream.ID, 1, "audio", secondPayload, sha256Hex(secondPayload))
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected second stream upload status 201, got %d: %s", response.StatusCode, body)
	}
	response, body = uploadChunk(t, app, incidentID, 1, "audio", legacyPayload, sha256Hex(legacyPayload))
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected legacy upload status 400, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "invalid_envelope")

	completeMediaStream(t, app, incidentID, firstStream.ID, 1)
	completeMediaStream(t, app, incidentID, secondStream.ID, 1)

	response, body = get(t, app, "/v1/incidents/"+incidentID+"/streams/"+firstStream.ID+"/download")
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected first stream bundle status 200, got %d: %s", response.StatusCode, body)
	}
	firstEntries := readZipEntries(t, body)
	if !bytes.Equal(firstEntries["chunks/audio_000001.enc"], firstPQPayload) {
		t.Fatal("first stream bundle used wrong overlapping chunk")
	}

	response, body = get(t, app, "/v1/incidents/"+incidentID+"/streams/"+secondStream.ID+"/download")
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected second stream bundle status 200, got %d: %s", response.StatusCode, body)
	}
	secondEntries := readZipEntries(t, body)
	if !bytes.Equal(secondEntries["chunks/audio_000001.enc"], secondPQPayload) {
		t.Fatal("second stream bundle used wrong overlapping chunk")
	}

	response, body = get(t, app, "/v1/incidents/"+incidentID+"/chunks/audio/1")
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected legacy chunk read status 404, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "chunk_not_found")
}

func TestRejectStreamedChunkIndexZero(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "audio")
	payload := []byte("encrypted audio data")

	response, body := uploadChunkWithStream(t, app, incidentID, stream.ID, 0, "audio", payload, sha256Hex(payload))
	defer response.Body.Close()

	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected zero-index stream upload status 400, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "invalid_chunk_index")
	if !bytes.Contains(body, []byte("positive when stream_id is provided")) {
		t.Fatalf("expected stream-specific chunk index message, got: %s", body)
	}
	assertNoStoredFile(t, app, incidentID, filepath.Join("streams", stream.ID, "audio_000000.enc"))
	assertTempDirEmpty(t, app)
}

func TestRejectStreamedNegativeChunkIndex(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "audio")
	payload := []byte("encrypted audio data")

	response, body := uploadChunkWithStream(t, app, incidentID, stream.ID, -1, "audio", payload, sha256Hex(payload))
	defer response.Body.Close()

	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected negative-index stream upload status 400, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "invalid_chunk_index")
}

func TestRejectChunkUploadWhereStreamBelongsToAnotherIncident(t *testing.T) {
	app := newTestApp(t)
	firstIncidentID := createIncident(t, app, `{}`)
	secondIncidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, firstIncidentID, incidents.MediaTypeAudio, "audio")
	payload := []byte("encrypted audio data")

	response, body := uploadChunkWithStream(t, app, secondIncidentID, stream.ID, 1, "audio", payload, sha256Hex(payload))
	defer response.Body.Close()

	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected wrong-incident stream status 404, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "stream_not_found")
}

func TestRejectChunkUploadWhereStreamMediaTypeDoesNotMatch(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "audio")
	payload := []byte("encrypted video data")

	response, body := uploadChunkWithStream(t, app, incidentID, stream.ID, 1, "video", payload, sha256Hex(payload))
	defer response.Body.Close()

	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected media mismatch status 400, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "stream_media_type_mismatch")
}

func TestCompleteStreamWithContiguousChunks(t *testing.T) {
	app := newTestApp(t)
	incidentID, stream := createIncidentStreamWithChunks(t, app, 2)

	updated := completeMediaStream(t, app, incidentID, stream.ID, 2)

	if updated.Status != incidents.StreamStatusComplete {
		t.Fatalf("expected complete stream, got %+v", updated)
	}
	if updated.ExpectedChunkCount == nil || *updated.ExpectedChunkCount != 2 {
		t.Fatalf("expected expected_chunk_count 2, got %+v", updated.ExpectedChunkCount)
	}
	if updated.CompletedAt == nil {
		t.Fatal("expected completed_at to be set")
	}
}

func TestRejectStreamCompletionWithMissingChunk(t *testing.T) {
	app := newTestApp(t)
	incidentID, stream := createIncidentStreamWithChunks(t, app, 1)

	response, body := post(t, app, "/v1/incidents/"+incidentID+"/streams/"+stream.ID+"/complete", "application/json", bytes.NewBufferString(`{"expected_chunk_count":2}`))
	defer response.Body.Close()

	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected incomplete stream status 409, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "stream_chunks_incomplete")
}

func TestRejectDuplicateStreamCompletion(t *testing.T) {
	app := newTestApp(t)
	incidentID, stream := createIncidentStreamWithChunks(t, app, 1)
	completeMediaStream(t, app, incidentID, stream.ID, 1)

	response, body := post(t, app, "/v1/incidents/"+incidentID+"/streams/"+stream.ID+"/complete", "application/json", bytes.NewBufferString(`{"expected_chunk_count":1}`))
	defer response.Body.Close()

	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected duplicate completion status 409, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "stream_already_complete")
}

func TestRejectChunkUploadToCompletedStream(t *testing.T) {
	app := newTestApp(t)
	incidentID, stream := createIncidentStreamWithChunks(t, app, 1)
	completeMediaStream(t, app, incidentID, stream.ID, 1)
	payload := []byte("late encrypted audio data")

	response, body := uploadChunkWithStream(t, app, incidentID, stream.ID, 2, "audio", payload, sha256Hex(payload))
	defer response.Body.Close()

	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected completed stream upload status 409, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "stream_not_open")
}

func TestFailStream(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "audio")

	response, body := post(t, app, "/v1/incidents/"+incidentID+"/streams/"+stream.ID+"/fail", "application/json", bytes.NewBufferString(`{"failure_reason":"client stopped recording unexpectedly"}`))
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected fail stream status 200, got %d: %s", response.StatusCode, body)
	}
	var result struct {
		Stream incidents.MediaStream `json:"stream"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode fail stream response: %v", err)
	}
	if result.Stream.Status != incidents.StreamStatusFailed || result.Stream.FailedAt == nil {
		t.Fatalf("expected failed stream, got %+v", result.Stream)
	}
}

func TestRejectDownloadOfOpenAndFailedStreams(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	openStream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "open audio")
	failedStream := createMediaStream(t, app, incidentID, incidents.MediaTypeVideo, "failed video")

	response, body := post(t, app, "/v1/incidents/"+incidentID+"/streams/"+failedStream.ID+"/fail", "application/json", bytes.NewBufferString(`{"failure_reason":"stopped"}`))
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected fail stream status 200, got %d: %s", response.StatusCode, body)
	}

	for _, stream := range []incidents.MediaStream{openStream, failedStream} {
		response, body := get(t, app, "/v1/incidents/"+incidentID+"/streams/"+stream.ID+"/download")
		response.Body.Close()
		if response.StatusCode != http.StatusConflict {
			t.Fatalf("expected download status 409 for %s stream, got %d: %s", stream.Status, response.StatusCode, body)
		}
		assertErrorCode(t, body, "stream_not_complete")
	}
}

func TestDownloadCompletedStreamBundle(t *testing.T) {
	app := newTestApp(t)
	incidentID, stream := createIncidentStreamWithChunks(t, app, 2)
	completeMediaStream(t, app, incidentID, stream.ID, 2)

	response, body := get(t, app, "/v1/incidents/"+incidentID+"/streams/"+stream.ID+"/download")
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected stream bundle status 200, got %d: %s", response.StatusCode, body)
	}
	assertBundleHeaders(t, response)
	entries := readZipEntries(t, body)
	assertZipEntry(t, entries, "manifest.json")
	assertZipEntry(t, entries, "chunks/audio_000001.enc")
	assertZipEntry(t, entries, "chunks/audio_000002.enc")

	var manifest struct {
		IncidentID string `json:"incident_id"`
		StreamID   string `json:"stream_id"`
		MediaType  string `json:"media_type"`
		Status     string `json:"status"`
		ChunkCount int    `json:"chunk_count"`
		Chunks     []struct {
			ChunkIndex int    `json:"chunk_index"`
			SHA256Hex  string `json:"sha256_hex"`
		} `json:"chunks"`
	}
	if err := json.Unmarshal(entries["manifest.json"], &manifest); err != nil {
		t.Fatalf("decode stream manifest: %v", err)
	}
	if manifest.IncidentID != incidentID || manifest.StreamID != stream.ID || manifest.Status != incidents.StreamStatusComplete || manifest.ChunkCount != 2 {
		t.Fatalf("unexpected stream manifest: %+v", manifest)
	}
	if manifest.Chunks[0].SHA256Hex != sha256Hex(entries["chunks/audio_000001.enc"]) {
		t.Fatalf("first manifest hash does not match zip chunk bytes")
	}
	if manifest.Chunks[1].SHA256Hex != sha256Hex(entries["chunks/audio_000002.enc"]) {
		t.Fatalf("second manifest hash does not match zip chunk bytes")
	}
}

func TestEncryptedEnvelopeChunkRoundTripsThroughOpaqueBackendBundle(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "audio recording")
	recipient, dk, err := pqenv.GenerateRecipientKey(1)
	if err != nil {
		t.Fatalf("generate recipient key: %v", err)
	}
	ctx := pqenv.PayloadContext{
		EnvelopeID:  "env_test_bundle",
		IncidentID:  incidentID,
		StreamID:    stream.ID,
		MediaType:   incidents.MediaTypeAudio,
		ChunkIndex:  1,
		PayloadType: pqenv.PayloadTypeChunk,
		MediaKeyID:  "media-key-test-bundle",
	}
	env, err := pqenv.Encrypt([]byte("plaintext stays client-side"), ctx, []pqenv.Recipient{recipient})
	if err != nil {
		t.Fatalf("encrypt chunk: %v", err)
	}
	payload := env.PayloadFrame

	response, body := uploadChunkWithStream(t, app, incidentID, stream.ID, 1, incidents.MediaTypeAudio, payload, sha256Hex(payload))
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected encrypted envelope upload status 201, got %d: %s", response.StatusCode, body)
	}
	completeMediaStream(t, app, incidentID, stream.ID, 1)

	response, body = get(t, app, "/v1/incidents/"+incidentID+"/streams/"+stream.ID+"/download")
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected stream bundle status 200, got %d: %s", response.StatusCode, body)
	}
	entries := readZipEntries(t, body)
	bundledChunk := entries["chunks/audio_000001.enc"]
	if !bytes.Equal(bundledChunk, payload) {
		t.Fatal("backend changed encrypted envelope bytes")
	}
	plaintext, err := pqenv.Decrypt(pqenv.Envelope{
		PayloadFrame: bundledChunk,
		Recipients:   env.Recipients,
	}, ctx, recipient.KeyID, dk)
	if err != nil {
		t.Fatalf("decrypt bundled chunk: %v", err)
	}
	if string(plaintext) != "plaintext stays client-side" {
		t.Fatalf("unexpected plaintext: %q", plaintext)
	}

	var manifest struct {
		Encryption struct {
			Expected       string `json:"expected"`
			Scheme         string `json:"scheme"`
			SuiteID        string `json:"suite_id"`
			Profile        string `json:"profile"`
			ServerDecrypts bool   `json:"server_decrypts"`
		} `json:"encryption"`
	}
	if err := json.Unmarshal(entries["manifest.json"], &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if manifest.Encryption.Expected != "client-side" ||
		manifest.Encryption.Scheme != pqenv.SchemeID ||
		manifest.Encryption.SuiteID != pqenv.SuiteID ||
		manifest.Encryption.Profile != pqenv.ProfileID ||
		manifest.Encryption.ServerDecrypts {
		t.Fatalf("unexpected encryption hint: %+v", manifest.Encryption)
	}

	rawKeyB64 := base64.RawURLEncoding.EncodeToString(dk.Bytes())
	for name, entry := range entries {
		if strings.Contains(name, "wrapped") || strings.Contains(name, "key") {
			t.Fatalf("bundle included key-related entry %q", name)
		}
		for _, disallowed := range []string{
			rawKeyB64,
			"key_b64",
			"wrapped_keys",
			"wrapped_key_b64",
			"AGE-SECRET-KEY",
		} {
			if bytes.Contains(entry, []byte(disallowed)) {
				t.Fatalf("bundle entry %s exposed %q", name, disallowed)
			}
		}
	}
}
