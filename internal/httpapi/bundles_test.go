package httpapi_test

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/open-proofline/server/internal/incidents"
)

func TestPrivateIncidentBundleFailsClosedWhenCompletedStreamChunkFileMissing(t *testing.T) {
	app := newTestApp(t)
	incidentID, stream := createIncidentStreamWithChunks(t, app, 1)
	completeMediaStream(t, app, incidentID, stream.ID, 1)
	removeStoredStreamChunkFile(t, app, incidentID, stream.ID, incidents.MediaTypeAudio, 1)

	response, body := get(t, app, "/v1/incidents/"+incidentID+"/download")
	defer response.Body.Close()

	assertIncidentBundleInconsistent(t, response, body)
	assertIncidentBundleErrorDoesNotExposeStorageDetails(t, body, app.dataDir, stream.ID, "audio_000001.enc")
}

func TestPrivateStreamBundleFailsClosedWhenCompletedStreamChunkFileMissing(t *testing.T) {
	app := newTestApp(t)
	incidentID, stream := createIncidentStreamWithChunks(t, app, 1)
	completeMediaStream(t, app, incidentID, stream.ID, 1)
	removeStoredStreamChunkFile(t, app, incidentID, stream.ID, incidents.MediaTypeAudio, 1)

	response, body := get(t, app, "/v1/incidents/"+incidentID+"/streams/"+stream.ID+"/download")
	defer response.Body.Close()

	assertStreamBundleInconsistent(t, response, body)
	assertNotZipResponse(t, response, body)
	assertIncidentBundleErrorDoesNotExposeStorageDetails(t, body, app.dataDir, stream.ID, "audio_000001.enc")
}

func TestPrivateStreamBundleFailsClosedWhenCompletedStreamChunkHashMismatches(t *testing.T) {
	app := newTestApp(t)
	incidentID, stream := createIncidentStreamWithChunks(t, app, 1)
	completeMediaStream(t, app, incidentID, stream.ID, 1)
	replaceStoredStreamChunkFile(t, app, incidentID, stream.ID, incidents.MediaTypeAudio, 1, []byte("different encrypted bytes"))

	response, body := get(t, app, "/v1/incidents/"+incidentID+"/streams/"+stream.ID+"/download")
	defer response.Body.Close()

	assertStreamBundleInconsistent(t, response, body)
	assertNotZipResponse(t, response, body)
	assertIncidentBundleErrorDoesNotExposeStorageDetails(t, body, app.dataDir, stream.ID, "audio_000001.enc")
	if bytes.Contains(body, []byte("different encrypted bytes")) {
		t.Fatalf("stream bundle inconsistency error exposed stored chunk bytes: %s", body)
	}
}

func TestPrivateIncidentBundleFailsClosedWhenCompletedStreamChunkHashMismatches(t *testing.T) {
	app := newTestApp(t)
	incidentID, stream := createIncidentStreamWithChunks(t, app, 1)
	completeMediaStream(t, app, incidentID, stream.ID, 1)
	replaceStoredStreamChunkFile(t, app, incidentID, stream.ID, incidents.MediaTypeAudio, 1, []byte("tampered encrypted bytes"))

	response, body := get(t, app, "/v1/incidents/"+incidentID+"/download")
	defer response.Body.Close()

	assertIncidentBundleInconsistent(t, response, body)
	assertNotZipResponse(t, response, body)
	assertIncidentBundleErrorDoesNotExposeStorageDetails(t, body, app.dataDir, stream.ID, "audio_000001.enc")
	if bytes.Contains(body, []byte("tampered encrypted bytes")) {
		t.Fatalf("incident bundle inconsistency error exposed stored chunk bytes: %s", body)
	}
}

func TestPrivateIncidentBundleFailsClosedWhenCompletedStreamChunksAreNonContiguous(t *testing.T) {
	app := newTestApp(t)
	incidentID, brokenStream := createIncidentStreamWithChunks(t, app, 1)
	completeMediaStream(t, app, incidentID, brokenStream.ID, 1)
	validStream := createMediaStream(t, app, incidentID, incidents.MediaTypeVideo, "video recording")
	payload := []byte("encrypted video data")
	response, body := uploadChunkWithStream(t, app, incidentID, validStream.ID, 1, incidents.MediaTypeVideo, payload, sha256Hex(payload))
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected video stream chunk upload status 201, got %d: %s", response.StatusCode, body)
	}
	completeMediaStream(t, app, incidentID, validStream.ID, 1)
	updateStoredStreamChunkIndex(t, app, incidentID, brokenStream.ID, 1, 2)

	response, body = get(t, app, "/v1/incidents/"+incidentID+"/download")
	defer response.Body.Close()

	assertIncidentBundleInconsistent(t, response, body)
	assertIncidentBundleErrorDoesNotExposeStorageDetails(t, body, app.dataDir, brokenStream.ID, "audio_000001.enc")
	if bytes.Contains(body, []byte(validStream.ID)) {
		t.Fatalf("incident bundle inconsistency error exposed valid stream ID: %s", body)
	}
}

func TestIncidentViewerIncidentBundleFailsClosedWhenCompletedStreamChunksAreNonContiguous(t *testing.T) {
	app := newTestApp(t)
	incidentID, stream := createIncidentStreamWithChunks(t, app, 1)
	completeMediaStream(t, app, incidentID, stream.ID, 1)
	updateStoredStreamChunkIndex(t, app, incidentID, stream.ID, 1, 2)
	token := createIncidentToken(t, app, incidentID, "trusted contact", nil)

	response, body := getPublic(t, app, "/i/"+token.Token+"/incident/download")
	defer response.Body.Close()

	assertIncidentBundleInconsistent(t, response, body)
	assertIncidentViewerPrivacyHeaders(t, response)
	assertIncidentBundleErrorDoesNotExposeStorageDetails(t, body, app.dataDir, stream.ID, "audio_000001.enc")
	if bytes.Contains(body, []byte(token.Token)) {
		t.Fatalf("incident bundle inconsistency error exposed raw token: %s", body)
	}
}

func assertIncidentBundleInconsistent(t *testing.T, response *http.Response, body []byte) {
	t.Helper()

	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected incident bundle inconsistency status 409, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "incident_bundle_inconsistent")
}

func assertStreamBundleInconsistent(t *testing.T, response *http.Response, body []byte) {
	t.Helper()

	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected stream bundle inconsistency status 409, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "stream_bundle_inconsistent")
}

func assertNotZipResponse(t *testing.T, response *http.Response, body []byte) {
	t.Helper()

	if response.Header.Get("Content-Type") == "application/zip" {
		t.Fatalf("expected non-ZIP error response, got zip content type with body: %s", body)
	}
	if response.Header.Get("Content-Disposition") != "" {
		t.Fatalf("expected no attachment header on error response, got %q", response.Header.Get("Content-Disposition"))
	}
}

func assertIncidentBundleErrorDoesNotExposeStorageDetails(t *testing.T, body []byte, dataDir, streamID, chunkFilename string) {
	t.Helper()

	for _, disallowed := range []string{dataDir, streamID, chunkFilename} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("incident bundle inconsistency error exposed %q: %s", disallowed, body)
		}
	}
}
