package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/envelope"
)

func TestParseByteSize(t *testing.T) {
	tests := map[string]int64{
		"1":     1,
		"64KiB": 64 * 1024,
		"2MiB":  2 * 1024 * 1024,
		"3MB":   3 * 1000 * 1000,
		"4 gb":  4 * 1000 * 1000 * 1000,
	}

	for input, want := range tests {
		got, err := parseByteSize(input)
		if err != nil {
			t.Fatalf("parseByteSize(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("parseByteSize(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestParseByteSizeRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{"", "KiB", "12XB"} {
		if _, err := parseByteSize(input); err == nil {
			t.Fatalf("parseByteSize(%q) succeeded, want error", input)
		}
	}
}

func TestBuildViewerURL(t *testing.T) {
	got := buildViewerURL("http://localhost:8081/", "abc/123")
	want := "http://localhost:8081/i/abc%2F123"
	if got != want {
		t.Fatalf("buildViewerURL returned %q, want %q", got, want)
	}
}

func TestClientWriteRoutesUseMainAPIBase(t *testing.T) {
	expectedPaths := []string{
		"POST /v1/auth/login",
		"POST /v1/incidents",
		"POST /v1/incidents/inc_1/incident-tokens",
		"POST /v1/incidents/inc_1/streams",
		"POST /v1/incidents/inc_1/checkins",
		"POST /v1/incidents/inc_1/streams/str_1/complete",
		"POST /v1/incidents/inc_1/streams/str_1/fail",
		"POST /v1/incidents/inc_1/close",
	}
	var gotPaths []string
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.example" {
			t.Fatalf("write route used host %q, want api.example", r.URL.Host)
		}
		gotPaths = append(gotPaths, r.Method+" "+r.URL.Path)
		if r.URL.Path != "/v1/auth/login" && r.Header.Get("Authorization") != "Bearer session-token" {
			t.Fatalf("write route %s missing session Authorization header", r.URL.Path)
		}
		switch r.URL.Path {
		case "/v1/auth/login":
			return testResponse(http.StatusCreated, "application/json", `{"token":"session-token"}`), nil
		case "/v1/incidents":
			return testResponse(http.StatusCreated, "application/json", `{"incident_id":"inc_1","status":"open"}`), nil
		case "/v1/incidents/inc_1/incident-tokens":
			return testResponse(http.StatusCreated, "application/json", `{"token_id":"itk_1","incident_id":"inc_1","token":"tok_1","created_at":"2026-05-22T10:00:00Z"}`), nil
		case "/v1/incidents/inc_1/streams":
			return testResponse(http.StatusCreated, "application/json", `{"stream":{"id":"str_1","incident_id":"inc_1","media_type":"audio","status":"open","created_at":"2026-05-22T10:00:00Z","updated_at":"2026-05-22T10:00:00Z"}}`), nil
		case "/v1/incidents/inc_1/checkins":
			return testResponse(http.StatusCreated, "application/json", `{"id":"chk_1"}`), nil
		case "/v1/incidents/inc_1/streams/str_1/complete":
			return testResponse(http.StatusOK, "application/json", `{"stream":{"id":"str_1","incident_id":"inc_1","media_type":"audio","status":"complete","created_at":"2026-05-22T10:00:00Z","updated_at":"2026-05-22T10:01:00Z","completed_at":"2026-05-22T10:01:00Z"}}`), nil
		case "/v1/incidents/inc_1/streams/str_1/fail":
			return testResponse(http.StatusOK, "application/json", `{"stream":{"id":"str_1","incident_id":"inc_1","media_type":"audio","status":"failed","created_at":"2026-05-22T10:00:00Z","updated_at":"2026-05-22T10:01:00Z","failed_at":"2026-05-22T10:01:00Z"}}`), nil
		case "/v1/incidents/inc_1/close":
			return testResponse(http.StatusOK, "application/json", `{"id":"inc_1","status":"closed"}`), nil
		default:
			return testResponse(http.StatusNotFound, "application/json", `{"error":{"code":"not_found"}}`), nil
		}
	})}

	sim := client{
		httpClient: httpClient,
		apiBase:    "http://api.example",
		viewerBase: "http://viewer.example",
	}
	ctx := context.Background()

	sessionToken, err := sim.login(ctx, "test-admin", "test-password")
	if err != nil {
		t.Fatalf("login returned error: %v", err)
	}
	sim.sessionToken = sessionToken
	incidentID, err := sim.createIncident(ctx)
	if err != nil {
		t.Fatalf("createIncident returned error: %v", err)
	}
	if incidentID != "inc_1" {
		t.Fatalf("incidentID = %q", incidentID)
	}
	token, err := sim.createIncidentToken(ctx, incidentID)
	if err != nil {
		t.Fatalf("createIncidentToken returned error: %v", err)
	}
	if token != "tok_1" {
		t.Fatalf("token = %q", token)
	}
	streamID, err := sim.createMediaStream(ctx, incidentID, "audio")
	if err != nil {
		t.Fatalf("createMediaStream returned error: %v", err)
	}
	if streamID != "str_1" {
		t.Fatalf("streamID = %q", streamID)
	}
	if err := sim.createCheckin(ctx, incidentID, 1); err != nil {
		t.Fatalf("createCheckin returned error: %v", err)
	}
	if err := sim.completeMediaStream(ctx, incidentID, streamID, 1); err != nil {
		t.Fatalf("completeMediaStream returned error: %v", err)
	}
	if err := sim.failMediaStream(ctx, incidentID, streamID, "test failure"); err != nil {
		t.Fatalf("failMediaStream returned error: %v", err)
	}
	if err := sim.closeIncident(ctx, incidentID); err != nil {
		t.Fatalf("closeIncident returned error: %v", err)
	}

	if len(gotPaths) != len(expectedPaths) {
		t.Fatalf("got paths %v, want %v", gotPaths, expectedPaths)
	}
	for i, want := range expectedPaths {
		if gotPaths[i] != want {
			t.Fatalf("path %d = %q, want %q; all paths: %v", i, gotPaths[i], want, gotPaths)
		}
	}
}

func TestClientUploadChunkUsesMainAPIBaseAndStreamID(t *testing.T) {
	body := []byte("encrypted bytes")
	upload := buildChunkUpload("inc_1", "str_1", 2, "audio", time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC), body)

	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.example" {
			t.Fatalf("upload used host %q, want api.example", r.URL.Host)
		}
		if r.Method != http.MethodPost || r.URL.Path != "/v1/incidents/inc_1/chunks" {
			t.Fatalf("unexpected upload route %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer session-token" {
			t.Fatalf("upload omitted session Authorization header")
		}
		if r.Header.Get("Idempotency-Key") == "" {
			t.Fatalf("upload omitted Idempotency-Key header")
		}
		if err := r.ParseMultipartForm(1024 * 1024); err != nil {
			t.Fatalf("ParseMultipartForm returned error: %v", err)
		}
		if got := r.FormValue("stream_id"); got != "str_1" {
			t.Fatalf("stream_id = %q", got)
		}
		if got := r.FormValue("chunk_index"); got != "2" {
			t.Fatalf("chunk_index = %q", got)
		}
		if got := r.FormValue("sha256_hex"); got != sha256Hex(body) {
			t.Fatalf("sha256_hex = %q", got)
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile returned error: %v", err)
		}
		defer file.Close()
		fileBody, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("read multipart file: %v", err)
		}
		if !bytes.Equal(fileBody, body) {
			t.Fatalf("uploaded file body = %q, want %q", fileBody, body)
		}
		return testResponse(http.StatusCreated, "application/json", `{"id":"chunk_1"}`), nil
	})}

	sim := client{
		httpClient:   httpClient,
		apiBase:      "http://api.example",
		viewerBase:   "http://viewer.example",
		sessionToken: "session-token",
	}
	if err := sim.uploadChunk(context.Background(), upload); err != nil {
		t.Fatalf("uploadChunk returned error: %v", err)
	}
}

func TestUploadDesktopStageRetriesAcceptedUploadAfterResponseLoss(t *testing.T) {
	stage, err := openDesktopStage(t.TempDir())
	if err != nil {
		t.Fatalf("openDesktopStage returned error: %v", err)
	}
	key, err := envelope.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	manifest := newDesktopManifest("inc_ambiguous", "str_ambiguous", "audio", desktopSourceGenerated)
	startedAt := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	if err := stage.stageChunk(&manifest, key, []byte("ambiguous response body"), startedAt, startedAt.Add(chunkDuration)); err != nil {
		t.Fatalf("stageChunk returned error: %v", err)
	}

	requests := 0
	var firstIDKey string
	var firstBody []byte
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/incidents/inc_ambiguous/chunks" {
			t.Fatalf("unexpected upload route %s %s", r.Method, r.URL.Path)
		}
		idempotencyKey := r.Header.Get("Idempotency-Key")
		if idempotencyKey == "" {
			t.Fatalf("upload omitted idempotency key")
		}
		if r.Header.Get("Authorization") != "Bearer session-token" {
			t.Fatalf("upload omitted session Authorization header")
		}
		if err := r.ParseMultipartForm(1024 * 1024); err != nil {
			t.Fatalf("ParseMultipartForm returned error: %v", err)
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile returned error: %v", err)
		}
		defer file.Close()
		body, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("read multipart file: %v", err)
		}

		if requests == 1 {
			firstIDKey = idempotencyKey
			firstBody = body
			return nil, errSimulatedNetwork
		}
		if idempotencyKey != firstIDKey {
			t.Fatalf("retry idempotency key = %q, want original key", idempotencyKey)
		}
		if !bytes.Equal(body, firstBody) {
			t.Fatalf("retry body changed")
		}
		response := testResponse(http.StatusOK, "application/json", `{"id":"chunk_1"}`)
		response.Header.Set("Idempotency-Replayed", "true")
		return response, nil
	})}
	sim := client{
		httpClient:   httpClient,
		apiBase:      "http://api.example",
		viewerBase:   "http://viewer.example",
		sessionToken: "session-token",
	}
	cfg := config{desktopMaxAttempts: 2, desktopRetryDelay: 0}
	var output bytes.Buffer

	if err := uploadDesktopStage(context.Background(), &output, sim, cfg, stage, &manifest); err != nil {
		t.Fatalf("uploadDesktopStage returned error: %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if !manifest.Chunks[0].Uploaded {
		t.Fatal("chunk was not marked uploaded after replay")
	}
	if manifest.Chunks[0].UploadAttempts != 2 {
		t.Fatalf("UploadAttempts = %d, want 2", manifest.Chunks[0].UploadAttempts)
	}
	for _, disallowed := range [][]byte{
		[]byte(firstIDKey),
		[]byte("session-token"),
		firstBody,
		[]byte(stage.dir),
	} {
		if len(disallowed) > 0 && bytes.Contains(output.Bytes(), disallowed) {
			t.Fatalf("ambiguous retry output exposed %q: %s", disallowed, output.String())
		}
	}
}

func TestClientUploadChunkConflictErrorIsTokenAndPathSafe(t *testing.T) {
	originalBody := []byte("original encrypted bytes")
	conflictingBody := []byte("different encrypted bytes")
	originalUpload := buildChunkUpload("inc_1", "str_1", 1, "audio", time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC), originalBody)
	originalUpload.idempotencyKey = "raw-idempotency-key-secret"
	conflictingUpload := buildChunkUpload("inc_1", "str_1", 1, "audio", time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC), conflictingBody)
	conflictingUpload.idempotencyKey = originalUpload.idempotencyKey

	requests := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.Header.Get("Idempotency-Key") != originalUpload.idempotencyKey {
			t.Fatalf("Idempotency-Key = %q, want simulator chunk key", r.Header.Get("Idempotency-Key"))
		}
		if err := r.ParseMultipartForm(1024 * 1024); err != nil {
			t.Fatalf("ParseMultipartForm returned error: %v", err)
		}
		if got := r.FormValue("stream_id"); got != "str_1" {
			t.Fatalf("stream_id = %q", got)
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile returned error: %v", err)
		}
		defer file.Close()
		body, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("read multipart file: %v", err)
		}
		switch requests {
		case 1:
			if got := r.FormValue("sha256_hex"); got != sha256Hex(originalBody) {
				t.Fatalf("original sha256_hex = %q", got)
			}
			if !bytes.Equal(body, originalBody) {
				t.Fatalf("original body changed")
			}
			return testResponse(http.StatusCreated, "application/json", `{"id":"chunk_1"}`), nil
		case 2:
			if got := r.FormValue("sha256_hex"); got != sha256Hex(conflictingBody) {
				t.Fatalf("conflicting sha256_hex = %q", got)
			}
			if !bytes.Equal(body, conflictingBody) {
				t.Fatalf("conflicting body changed")
			}
			return testResponse(http.StatusConflict, "application/json", `{"error":{"code":"idempotency_conflict","message":"raw-idempotency-key-secret different encrypted bytes http://api.example/private-path"}}`), nil
		default:
			t.Fatalf("unexpected request %d", requests)
			return nil, errSimulatedNetwork
		}
	})}
	sim := client{
		httpClient:   httpClient,
		apiBase:      "http://api.example/private-path",
		viewerBase:   "http://viewer.example",
		sessionToken: "session-token",
	}

	if err := sim.uploadChunk(context.Background(), originalUpload); err != nil {
		t.Fatalf("original upload returned error: %v", err)
	}
	err := sim.uploadChunk(context.Background(), conflictingUpload)
	if err == nil {
		t.Fatal("expected idempotency conflict error")
	}
	message := err.Error()
	if !strings.Contains(message, "idempotency_conflict") {
		t.Fatalf("conflict error = %q, want idempotency_conflict", message)
	}
	for _, disallowed := range []string{
		originalUpload.idempotencyKey,
		string(originalBody),
		string(conflictingBody),
		"session-token",
		"api.example",
		"private-path",
		"/v1/incidents",
	} {
		if strings.Contains(message, disallowed) {
			t.Fatalf("conflict error exposed %q: %s", disallowed, message)
		}
	}
}

func TestClientExpectIdempotentReplayRequiresReplayHeader(t *testing.T) {
	body := []byte("encrypted bytes")
	upload := buildChunkUpload("inc_1", "str_1", 1, "audio", time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC), body)
	seenRequests := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seenRequests++
		if r.Header.Get("Idempotency-Key") != upload.idempotencyKey {
			t.Fatalf("Idempotency-Key = %q, want simulator chunk key", r.Header.Get("Idempotency-Key"))
		}
		response := testResponse(http.StatusOK, "application/json", `{"id":"chunk_1"}`)
		response.Header.Set("Idempotency-Replayed", "true")
		return response, nil
	})}
	sim := client{
		httpClient:   httpClient,
		apiBase:      "http://api.example",
		viewerBase:   "http://viewer.example",
		sessionToken: "session-token",
	}

	if err := sim.expectIdempotentReplay(context.Background(), upload); err != nil {
		t.Fatalf("expectIdempotentReplay returned error: %v", err)
	}
	if seenRequests != 1 {
		t.Fatalf("seen requests = %d, want 1", seenRequests)
	}
}

func TestClientDownloadStreamBundleUsesViewerBase(t *testing.T) {
	bundleBytes := []byte("zip bytes")
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "viewer.example" {
			t.Fatalf("bundle download used host %q, want viewer.example", r.URL.Host)
		}
		if r.Method != http.MethodGet || r.URL.Path != "/i/tok_1/streams/str_1/download" {
			t.Fatalf("unexpected viewer route %s %s", r.Method, r.URL.Path)
		}
		return testResponse(http.StatusOK, "application/zip", string(bundleBytes)), nil
	})}

	sim := client{
		httpClient: httpClient,
		apiBase:    "http://api.example",
		viewerBase: "http://viewer.example",
	}
	got, err := sim.downloadStreamBundle(context.Background(), "tok_1", "str_1")
	if err != nil {
		t.Fatalf("downloadStreamBundle returned error: %v", err)
	}
	if !bytes.Equal(got, bundleBytes) {
		t.Fatalf("bundle body = %q, want %q", got, bundleBytes)
	}
}

func TestPostJSONUnexpectedStatusOmitsTokenBearingBody(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return testResponse(http.StatusOK, "application/json", `{"token":"raw-session-token","other":"raw-viewer-token"}`), nil
	})}
	sim := client{
		httpClient: httpClient,
		apiBase:    "http://api.example",
		viewerBase: "http://viewer.example",
	}

	err := sim.postJSON(context.Background(), "/v1/auth/login", map[string]string{"username": "admin"}, http.StatusCreated, nil)
	if err == nil {
		t.Fatal("expected unexpected status error")
	}
	message := err.Error()
	for _, secret := range []string{"raw-session-token", "raw-viewer-token"} {
		if strings.Contains(message, secret) {
			t.Fatalf("unexpected status error exposed %q: %s", secret, message)
		}
	}
	if !strings.Contains(message, "response body omitted") {
		t.Fatalf("unexpected status error = %q, want omitted body summary", message)
	}
}

func TestPostJSONUnexpectedStatusIncludesAPIErrorSummary(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return testResponse(http.StatusUnauthorized, "application/json", `{"error":{"code":"unauthorized","message":"authentication required"}}`), nil
	})}
	sim := client{
		httpClient: httpClient,
		apiBase:    "http://api.example",
		viewerBase: "http://viewer.example",
	}

	err := sim.postJSON(context.Background(), "/v1/incidents", map[string]string{}, http.StatusCreated, nil)
	if err == nil {
		t.Fatal("expected unexpected status error")
	}
	message := err.Error()
	if !strings.Contains(message, "unauthorized: authentication required") {
		t.Fatalf("unexpected status error = %q, want API error summary", message)
	}
}

func TestParseConfigStreamFlags(t *testing.T) {
	cfg, err := parseConfig(withAuthArgs("--chunks", "2", "--interval", "0", "--download-bundle"))
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if !cfg.completeStream {
		t.Fatal("expected complete-stream to default true")
	}
	if !cfg.downloadBundle {
		t.Fatal("expected download-bundle flag to be set")
	}
}

func TestParseConfigBundleOutput(t *testing.T) {
	cfg, err := parseConfig(withAuthArgs(
		"--chunks", "2",
		"--interval", "0",
		"--download-bundle",
		"--bundle-output", "bundle.zip",
	))
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if cfg.bundleOutput != "bundle.zip" {
		t.Fatalf("bundleOutput = %q", cfg.bundleOutput)
	}
}

func TestParseConfigVerifyBundleDoesNotRequireAuth(t *testing.T) {
	cfg, err := parseConfig([]string{
		"--verify-bundle", "bundle.zip",
		"--key-file", "key.json",
	})
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if cfg.verifyBundlePath != "bundle.zip" {
		t.Fatalf("verifyBundlePath = %q", cfg.verifyBundlePath)
	}
	if cfg.keyFile != "key.json" {
		t.Fatalf("keyFile = %q", cfg.keyFile)
	}
}

func TestParseConfigVerifyBundleUsesStageKeyByDefault(t *testing.T) {
	cfg, err := parseConfig([]string{
		"--verify-bundle", "bundle.zip",
		"--stage-dir", "stage",
	})
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	want := filepath.Join("stage", "simulator-key.json")
	if cfg.keyFile != want {
		t.Fatalf("keyFile = %q, want %q", cfg.keyFile, want)
	}
}

func TestParseConfigEncryptionDefaults(t *testing.T) {
	cfg, err := parseConfig(withAuthArgs("--chunks", "2", "--interval", "0"))
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if !cfg.encrypt {
		t.Fatal("expected encryption to default true")
	}
	if !cfg.verifyBundleDecrypt {
		t.Fatal("expected bundle decryption verification to default true")
	}
	if cfg.keyFile != "" {
		t.Fatalf("keyFile = %q, want empty", cfg.keyFile)
	}
}

func TestParseConfigWrappedKeyOutputDefaultsContactKeyFile(t *testing.T) {
	cfg, err := parseConfig(withAuthArgs(
		"--wrapped-key-output", filepath.Join("stage", "proofline-sim-wrapped-keys.json"),
	))
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if cfg.wrappedKeyOutput != filepath.Join("stage", "proofline-sim-wrapped-keys.json") {
		t.Fatalf("wrappedKeyOutput = %q", cfg.wrappedKeyOutput)
	}
	wantContactKey := filepath.Join("stage", defaultContactKeyFileName)
	if cfg.contactKeyFile != wantContactKey {
		t.Fatalf("contactKeyFile = %q, want %q", cfg.contactKeyFile, wantContactKey)
	}
	if cfg.wrappedKeyContactID != defaultWrappedKeyContactID {
		t.Fatalf("wrappedKeyContactID = %q", cfg.wrappedKeyContactID)
	}
}

func TestParseConfigAllowsEncryptionToBeDisabled(t *testing.T) {
	cfg, err := parseConfig(withAuthArgs("--encrypt=false"))
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if cfg.encrypt {
		t.Fatal("expected encryption to be disabled")
	}
}

func TestParseConfigCleansBasesAndChunkSize(t *testing.T) {
	cfg, err := parseConfig([]string{
		"--username", " test-admin ",
		"--password", "test-password",
		"--api", "http://private.example/",
		"--viewer", "http://public.example/",
		"--chunk-size", "2KiB",
	})
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if cfg.apiBase != "http://private.example" {
		t.Fatalf("apiBase = %q", cfg.apiBase)
	}
	if cfg.viewerBase != "http://public.example" {
		t.Fatalf("viewerBase = %q", cfg.viewerBase)
	}
	if cfg.username != "test-admin" {
		t.Fatalf("username = %q", cfg.username)
	}
	if cfg.chunkSize != 2048 {
		t.Fatalf("chunkSize = %d", cfg.chunkSize)
	}
}

func TestParseConfigRejectsDownloadWithoutCompleteStream(t *testing.T) {
	if _, err := parseConfig(withAuthArgs("--download-bundle", "--complete-stream=false")); err == nil {
		t.Fatal("expected --download-bundle without --complete-stream to fail")
	}
}

func TestParseConfigRejectsDownloadWithoutChunks(t *testing.T) {
	if _, err := parseConfig(withAuthArgs("--chunks", "0", "--download-bundle")); err == nil {
		t.Fatal("expected --download-bundle without chunks to fail")
	}
}

func TestParseConfigRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "negative chunks",
			args: withAuthArgs("--chunks", "-1"),
		},
		{
			name: "negative interval",
			args: withAuthArgs("--interval", "-1s"),
		},
		{
			name: "invalid media type",
			args: withAuthArgs("--media-type", "screen"),
		},
		{
			name: "zero chunk size",
			args: withAuthArgs("--chunk-size", "0"),
		},
		{
			name: "negative failure interval",
			args: withAuthArgs("--simulate-failure-every", "-1"),
		},
		{
			name: "desktop recorder without stage dir",
			args: withAuthArgs("--desktop-recorder"),
		},
		{
			name: "desktop files without input",
			args: withAuthArgs("--desktop-recorder", "--stage-dir", "stage", "--desktop-source", "files"),
		},
		{
			name: "ffmpeg requires video media type",
			args: withAuthArgs("--desktop-recorder", "--stage-dir", "stage", "--desktop-source", "ffmpeg"),
		},
		{
			name: "invalid network failure rate",
			args: withAuthArgs("--network-failure-rate", "1.5"),
		},
		{
			name: "bundle output without bundle download",
			args: withAuthArgs("--bundle-output", "bundle.zip"),
		},
		{
			name: "bundle output requires encrypted upload",
			args: withAuthArgs("--download-bundle", "--bundle-output", "bundle.zip", "--encrypt=false"),
		},
		{
			name: "verify bundle requires key source",
			args: []string{"--verify-bundle", "bundle.zip"},
		},
		{
			name: "verify bundle requires encrypted bundle",
			args: []string{"--verify-bundle", "bundle.zip", "--key-file", "key.json", "--encrypt=false"},
		},
		{
			name: "verify bundle is offline",
			args: withAuthArgs("--verify-bundle", "bundle.zip", "--key-file", "key.json", "--download-bundle"),
		},
		{
			name: "wrapped key output requires encrypted upload",
			args: withAuthArgs("--wrapped-key-output", "proofline-sim-wrapped-keys.json", "--encrypt=false"),
		},
		{
			name: "contact key file requires artifact",
			args: withAuthArgs("--contact-key-file", "proofline-sim-contact.key.json"),
		},
		{
			name: "invalid wrapped key contact id",
			args: withAuthArgs("--wrapped-key-output", "proofline-sim-wrapped-keys.json", "--wrapped-key-contact-id", "../contact"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseConfig(tt.args); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseConfigDesktopRecorderFilesMode(t *testing.T) {
	cfg, err := parseConfig(withAuthArgs(
		"--desktop-recorder",
		"--stage-dir", "stage",
		"--desktop-source", "files",
		"--input-file", "first.mp4",
		"--input-file", "second.mp4",
		"--network-bandwidth", "256KiB",
		"--network-latency", "25ms",
		"--network-jitter", "10ms",
		"--network-timeout", "5s",
		"--desktop-max-attempts", "3",
	))
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if !cfg.desktopRecorder || cfg.desktopSource != desktopSourceFiles {
		t.Fatalf("desktop config not set: %+v", cfg)
	}
	if len(cfg.desktopInputFiles) != 2 {
		t.Fatalf("desktop input files = %v", cfg.desktopInputFiles)
	}
	if cfg.networkBandwidth != 256*1024 {
		t.Fatalf("networkBandwidth = %d", cfg.networkBandwidth)
	}
	if cfg.networkTimeout != 5*time.Second {
		t.Fatalf("networkTimeout = %s", cfg.networkTimeout)
	}
	if cfg.desktopMaxAttempts != 3 {
		t.Fatalf("desktopMaxAttempts = %d", cfg.desktopMaxAttempts)
	}
}

func TestParseConfigDesktopRecorderFFmpegMode(t *testing.T) {
	cfg, err := parseConfig(withAuthArgs(
		"--desktop-recorder",
		"--stage-dir", "stage",
		"--desktop-source", "ffmpeg",
		"--media-type", "video",
		"--ffmpeg-input-format", "x11grab",
		"--ffmpeg-input", ":0.0",
		"--ffmpeg-video-codec", "mpeg4",
		"--ffmpeg-duration", "2s",
		"--ffmpeg-segment-time", "1s",
	))
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if cfg.desktopSource != desktopSourceFFmpeg {
		t.Fatalf("desktopSource = %q", cfg.desktopSource)
	}
	if cfg.mediaType != "video" {
		t.Fatalf("mediaType = %q", cfg.mediaType)
	}
	if cfg.ffmpegInputFormat != "x11grab" || cfg.ffmpegInput != ":0.0" {
		t.Fatalf("ffmpeg input = %q %q", cfg.ffmpegInputFormat, cfg.ffmpegInput)
	}
	if cfg.ffmpegVideoCodec != "mpeg4" {
		t.Fatalf("ffmpegVideoCodec = %q", cfg.ffmpegVideoCodec)
	}
}

func withAuthArgs(args ...string) []string {
	base := []string{"--username", "test-admin", "--password", "test-password"}
	return append(base, args...)
}

func TestBadHashFor(t *testing.T) {
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	bad := badHashFor(hash)
	if bad == hash {
		t.Fatal("badHashFor returned original hash")
	}
	if len(bad) != 64 {
		t.Fatalf("badHashFor returned length %d, want 64", len(bad))
	}
}

func TestShouldSimulateFailure(t *testing.T) {
	if !shouldSimulateFailure(4, 4) {
		t.Fatal("expected every fourth chunk to simulate failure")
	}
	if shouldSimulateFailure(3, 4) {
		t.Fatal("did not expect third chunk to simulate failure")
	}
	if shouldSimulateFailure(4, 0) {
		t.Fatal("did not expect failure when disabled")
	}
}

func TestShouldSendCheckin(t *testing.T) {
	tests := []struct {
		index int
		want  bool
	}{
		{index: 1, want: true},
		{index: 2, want: false},
		{index: 3, want: true},
		{index: 4, want: false},
		{index: 6, want: true},
	}

	for _, tt := range tests {
		if got := shouldSendCheckin(tt.index); got != tt.want {
			t.Fatalf("shouldSendCheckin(%d) = %v, want %v", tt.index, got, tt.want)
		}
	}
}

func TestNewChunkUploadIncludesStreamAndHashMetadata(t *testing.T) {
	startedAt := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	upload, err := newChunkUpload("incident-test", "stream-test", 2, "audio", 16, startedAt)
	if err != nil {
		t.Fatalf("newChunkUpload returned error: %v", err)
	}

	if upload.incidentID != "incident-test" {
		t.Fatalf("incidentID = %q", upload.incidentID)
	}
	if upload.streamID != "stream-test" {
		t.Fatalf("streamID = %q", upload.streamID)
	}
	if upload.mediaType != "audio" {
		t.Fatalf("mediaType = %q", upload.mediaType)
	}
	if upload.chunkIndex != 2 {
		t.Fatalf("chunkIndex = %d", upload.chunkIndex)
	}
	if upload.filename != "audio_000002.enc" {
		t.Fatalf("filename = %q", upload.filename)
	}
	if upload.idempotencyKey == "" {
		t.Fatal("expected idempotency key")
	}
	if len(upload.body) != 16 {
		t.Fatalf("body length = %d", len(upload.body))
	}
	if upload.startedAt != startedAt.Add(chunkDuration) {
		t.Fatalf("startedAt = %s", upload.startedAt)
	}
	if upload.endedAt != upload.startedAt.Add(chunkDuration) {
		t.Fatalf("endedAt = %s, startedAt = %s", upload.endedAt, upload.startedAt)
	}

	sum := sha256.Sum256(upload.body)
	if upload.sha256Hex != hex.EncodeToString(sum[:]) {
		t.Fatalf("sha256Hex = %q", upload.sha256Hex)
	}
}

func TestLoadOrCreateSimulatorKeyCreatesAndLoadsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sim.key.json")

	created, err := loadOrCreateSimulatorKey(path)
	if err != nil {
		t.Fatalf("loadOrCreateSimulatorKey create returned error: %v", err)
	}
	if created.KeyID == "" || len(created.Key) != 32 {
		t.Fatalf("unexpected created key: %+v", created)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected key file to be created: %v", err)
	}

	loaded, err := loadOrCreateSimulatorKey(path)
	if err != nil {
		t.Fatalf("loadOrCreateSimulatorKey load returned error: %v", err)
	}
	if loaded.KeyID != created.KeyID || !bytes.Equal(loaded.Key, created.Key) {
		t.Fatal("loaded key did not match created key")
	}
}

func TestPrepareContactWrappedKeyWritesArtifactWithoutSecrets(t *testing.T) {
	dir := t.TempDir()
	mediaKey, err := envelope.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	cfg := config{
		encrypt:             true,
		wrappedKeyOutput:    filepath.Join(dir, "proofline-sim-wrapped-keys.json"),
		contactKeyFile:      filepath.Join(dir, "proofline-sim-contact.key.json"),
		wrappedKeyContactID: "contact_dev_alex",
	}

	var out bytes.Buffer
	unwrapped, ok, err := prepareContactWrappedKey(&out, cfg, "inc_wrapped", "str_wrapped", mediaKey)
	if err != nil {
		t.Fatalf("prepareContactWrappedKey returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected wrapped-key preparation to run")
	}
	if unwrapped.KeyID != mediaKey.KeyID || !bytes.Equal(unwrapped.Key, mediaKey.Key) {
		t.Fatal("unwrapped contact key did not match simulator media key")
	}
	if !strings.Contains(out.String(), "Verified contact unwrap for wrapped-key artifact.") {
		t.Fatalf("output did not mention contact unwrap verification: %q", out.String())
	}

	artifactBody, err := os.ReadFile(cfg.wrappedKeyOutput)
	if err != nil {
		t.Fatalf("read wrapped-key artifact: %v", err)
	}
	contactBody, err := os.ReadFile(cfg.contactKeyFile)
	if err != nil {
		t.Fatalf("read contact key file: %v", err)
	}
	var contact contactKeyFile
	if err := json.Unmarshal(contactBody, &contact); err != nil {
		t.Fatalf("decode contact key file: %v", err)
	}
	rawKeyB64 := base64.RawURLEncoding.EncodeToString(mediaKey.Key)
	for _, disallowed := range []string{
		rawKeyB64,
		contact.Identity,
		"AGE-SECRET-KEY",
		`"key_b64"`,
		dir,
		"tok_secret",
		"http://",
		"https://",
	} {
		if bytes.Contains(artifactBody, []byte(disallowed)) {
			t.Fatalf("wrapped-key artifact exposed %q: %s", disallowed, artifactBody)
		}
	}
	if !bytes.Contains(artifactBody, []byte(wrappingAlgorithmAgeX25519)) {
		t.Fatalf("wrapped-key artifact did not record algorithm: %s", artifactBody)
	}
	if runtime.GOOS != "windows" {
		stat, err := os.Stat(cfg.contactKeyFile)
		if err != nil {
			t.Fatalf("stat contact key file: %v", err)
		}
		if stat.Mode().Perm() != 0o600 {
			t.Fatalf("contact key file mode = %v, want 0600", stat.Mode().Perm())
		}
	}
}

func TestDecodeWrappedKeyArtifactRejectsMalformedMetadata(t *testing.T) {
	base := wrappedKeyArtifact{
		Version:    wrappedKeyArtifactVersion,
		Scope:      wrappedKeyArtifactScope,
		IncidentID: "inc_wrapped",
		StreamID:   "str_wrapped",
		MediaKeyID: "kid_wrapped",
		CreatedAt:  time.Now().UTC(),
		WrappedKeys: []wrappedKeyRecord{
			{
				WrappedKeyID:      "wkey_test",
				RecipientType:     wrappedKeyRecipientType,
				ContactID:         defaultWrappedKeyContactID,
				ContactKeyID:      "ckid_test",
				WrappingAlgorithm: wrappingAlgorithmAgeX25519,
				WrappedKeyB64:     base64.RawURLEncoding.EncodeToString([]byte("wrapped")),
			},
		},
	}
	tests := []struct {
		name   string
		mutate func(*wrappedKeyArtifact)
	}{
		{
			name: "unsupported algorithm",
			mutate: func(artifact *wrappedKeyArtifact) {
				artifact.WrappedKeys[0].WrappingAlgorithm = "custom-ecdh"
			},
		},
		{
			name: "malformed ciphertext",
			mutate: func(artifact *wrappedKeyArtifact) {
				artifact.WrappedKeys[0].WrappedKeyB64 = "not base64"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact := base
			artifact.WrappedKeys = append([]wrappedKeyRecord(nil), base.WrappedKeys...)
			tt.mutate(&artifact)
			body, err := json.Marshal(artifact)
			if err != nil {
				t.Fatalf("marshal artifact: %v", err)
			}
			if _, err := decodeWrappedKeyArtifact(body); err == nil {
				t.Fatal("expected malformed wrapped-key artifact to be rejected")
			}
		})
	}
}

func TestValidateWrappedKeyArtifactForStreamRejectsContactIDMismatch(t *testing.T) {
	artifact := wrappedKeyArtifact{
		Version:    wrappedKeyArtifactVersion,
		Scope:      wrappedKeyArtifactScope,
		IncidentID: "inc_wrapped",
		StreamID:   "str_wrapped",
		MediaKeyID: "kid_wrapped",
		CreatedAt:  time.Now().UTC(),
		WrappedKeys: []wrappedKeyRecord{
			{
				WrappedKeyID:      "wkey_test",
				RecipientType:     wrappedKeyRecipientType,
				ContactID:         "contact_dev_alex",
				ContactKeyID:      "ckid_test",
				WrappingAlgorithm: wrappingAlgorithmAgeX25519,
				WrappedKeyB64:     base64.RawURLEncoding.EncodeToString([]byte("wrapped")),
			},
		},
	}

	if err := validateWrappedKeyArtifactForStream(artifact, "inc_wrapped", "str_wrapped", "kid_wrapped", "contact_dev_alex", "ckid_test"); err != nil {
		t.Fatalf("expected matching contact artifact to validate: %v", err)
	}
	if err := validateWrappedKeyArtifactForStream(artifact, "inc_wrapped", "str_wrapped", "kid_wrapped", "contact_dev_blair", "ckid_test"); err == nil {
		t.Fatal("expected contact_id mismatch to be rejected")
	}
}

func TestNewEncryptedChunkUploadCanDecryptEnvelope(t *testing.T) {
	key, err := envelope.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	startedAt := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	upload, err := newEncryptedChunkUpload(key, "incident-test", "stream-test", 1, "audio", 16, startedAt)
	if err != nil {
		t.Fatalf("newEncryptedChunkUpload returned error: %v", err)
	}
	if len(upload.body) <= 16 {
		t.Fatalf("expected encrypted envelope to be larger than plaintext, got %d bytes", len(upload.body))
	}
	header, err := envelope.ParseHeader(upload.body)
	if err != nil {
		t.Fatalf("ParseHeader returned error: %v", err)
	}
	if header.KeyID != key.KeyID {
		t.Fatalf("header key id = %q, want %q", header.KeyID, key.KeyID)
	}
	plaintext, err := envelope.DecryptChunk(key, chunkContext(upload.incidentID, upload.streamID, upload.mediaType, upload.chunkIndex), upload.body)
	if err != nil {
		t.Fatalf("DecryptChunk returned error: %v", err)
	}
	if len(plaintext) != 16 {
		t.Fatalf("decrypted plaintext length = %d, want 16", len(plaintext))
	}
	if upload.sha256Hex != sha256Hex(upload.body) {
		t.Fatalf("sha256Hex = %q, want ciphertext hash %q", upload.sha256Hex, sha256Hex(upload.body))
	}
}

func TestDesktopStageChunkCreatesEncryptedDurableUpload(t *testing.T) {
	stage, err := openDesktopStage(t.TempDir())
	if err != nil {
		t.Fatalf("openDesktopStage returned error: %v", err)
	}
	key, err := envelope.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	manifest := newDesktopManifest("inc_stage", "str_stage", "audio", desktopSourceFiles)
	startedAt := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	if err := stage.stageChunk(&manifest, key, []byte("encoded media bytes"), startedAt, startedAt.Add(chunkDuration)); err != nil {
		t.Fatalf("stageChunk returned error: %v", err)
	}
	if len(manifest.Chunks) != 1 {
		t.Fatalf("manifest chunks = %d, want 1", len(manifest.Chunks))
	}
	if strings.Contains(manifest.Chunks[0].Filename, string(os.PathSeparator)) {
		t.Fatalf("staged filename contains path separator: %q", manifest.Chunks[0].Filename)
	}

	loaded, err := stage.loadManifest()
	if err != nil {
		t.Fatalf("loadManifest returned error: %v", err)
	}
	upload, err := stage.chunkUpload(loaded, loaded.Chunks[0])
	if err != nil {
		t.Fatalf("chunkUpload returned error: %v", err)
	}
	if upload.incidentID != "inc_stage" || upload.streamID != "str_stage" || upload.chunkIndex != 1 {
		t.Fatalf("unexpected upload identity: %+v", upload)
	}
	plaintext, err := envelope.DecryptChunk(key, chunkContext(upload.incidentID, upload.streamID, upload.mediaType, upload.chunkIndex), upload.body)
	if err != nil {
		t.Fatalf("DecryptChunk returned error: %v", err)
	}
	if string(plaintext) != "encoded media bytes" {
		t.Fatalf("decrypted body = %q", plaintext)
	}
	if upload.sha256Hex != sha256Hex(upload.body) {
		t.Fatalf("upload sha256Hex = %q, want %q", upload.sha256Hex, sha256Hex(upload.body))
	}
}

func TestValidateDesktopManifestRejectsChunkIndexGaps(t *testing.T) {
	manifest := newDesktopManifest("inc_stage", "str_stage", "audio", desktopSourceGenerated)
	startedAt := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	manifest.Chunks = []desktopStageChunk{
		{
			ChunkIndex: 1,
			MediaType:  "audio",
			StartedAt:  startedAt,
			EndedAt:    startedAt.Add(chunkDuration),
			Filename:   "audio_000001.enc",
			ByteSize:   16,
			SHA256Hex:  strings.Repeat("a", 64),
		},
		{
			ChunkIndex: 3,
			MediaType:  "audio",
			StartedAt:  startedAt.Add(chunkDuration),
			EndedAt:    startedAt.Add(2 * chunkDuration),
			Filename:   "audio_000003.enc",
			ByteSize:   16,
			SHA256Hex:  strings.Repeat("b", 64),
		},
	}

	if err := validateDesktopManifest(manifest); err == nil {
		t.Fatal("expected non-contiguous manifest to be rejected")
	}
}

func TestUploadDesktopStageRetriesAndPersistsUploadState(t *testing.T) {
	stage, err := openDesktopStage(t.TempDir())
	if err != nil {
		t.Fatalf("openDesktopStage returned error: %v", err)
	}
	key, err := envelope.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	manifest := newDesktopManifest("inc_retry", "str_retry", "audio", desktopSourceGenerated)
	startedAt := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	if err := stage.stageChunk(&manifest, key, []byte("retry body"), startedAt, startedAt.Add(chunkDuration)); err != nil {
		t.Fatalf("stageChunk returned error: %v", err)
	}

	requests := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return nil, errSimulatedNetwork
		}
		if r.Method != http.MethodPost || r.URL.Path != "/v1/incidents/inc_retry/chunks" {
			t.Fatalf("unexpected upload route %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Idempotency-Key") == "" {
			t.Fatalf("upload omitted idempotency key")
		}
		return testResponse(http.StatusCreated, "application/json", `{"id":"chunk_1"}`), nil
	})}
	sim := client{
		httpClient:   httpClient,
		apiBase:      "http://api.example",
		viewerBase:   "http://viewer.example",
		sessionToken: "session-token",
	}
	cfg := config{desktopMaxAttempts: 2, desktopRetryDelay: 0}
	if err := uploadDesktopStage(context.Background(), io.Discard, sim, cfg, stage, &manifest); err != nil {
		t.Fatalf("uploadDesktopStage returned error: %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if !manifest.Chunks[0].Uploaded {
		t.Fatal("chunk was not marked uploaded")
	}
	if manifest.Chunks[0].UploadAttempts != 2 {
		t.Fatalf("UploadAttempts = %d, want 2", manifest.Chunks[0].UploadAttempts)
	}
	loaded, err := stage.loadManifest()
	if err != nil {
		t.Fatalf("loadManifest returned error: %v", err)
	}
	if !loaded.Chunks[0].Uploaded || loaded.Chunks[0].UploadAttempts != 2 {
		t.Fatalf("persisted upload state = %+v", loaded.Chunks[0])
	}
}

func TestRunDesktopRecorderFailsIncompleteFFmpegStream(t *testing.T) {
	dir := t.TempDir()
	ffmpegBin := filepath.Join(dir, "fake-ffmpeg")
	script := `#!/bin/sh
last=
for arg do
	last="$arg"
done
out=$(printf '%s\n' "$last" | sed 's/%06d/000000/')
printf 'fake encoded segment' > "$out"
`
	if err := os.WriteFile(ffmpegBin, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake ffmpeg: %v", err)
	}

	uploadRequests := 0
	failRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/auth/login":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"token":"session-token"}`))
		case "/v1/incidents":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"incident_id":"inc_ffmpeg","status":"open"}`))
		case "/v1/incidents/inc_ffmpeg/streams":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"stream":{"id":"str_ffmpeg","incident_id":"inc_ffmpeg","media_type":"video","status":"open","created_at":"2026-05-22T10:00:00Z","updated_at":"2026-05-22T10:00:00Z"}}`))
		case "/v1/incidents/inc_ffmpeg/chunks":
			uploadRequests++
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"code":"simulated_upload_failure","message":"upload failed"}}`))
		case "/v1/incidents/inc_ffmpeg/streams/str_ffmpeg/fail":
			failRequests++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"stream":{"id":"str_ffmpeg","incident_id":"inc_ffmpeg","media_type":"video","status":"failed","created_at":"2026-05-22T10:00:00Z","updated_at":"2026-05-22T10:01:00Z","failed_at":"2026-05-22T10:01:00Z"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found"}}`))
		}
	}))
	defer server.Close()

	cfg := config{
		apiBase:               server.URL,
		viewerBase:            "http://viewer.example",
		username:              "admin",
		password:              "test-password",
		mediaType:             "video",
		chunkSize:             16,
		completeStream:        true,
		encrypt:               true,
		desktopRecorder:       true,
		desktopStageDir:       filepath.Join(dir, "stage"),
		desktopFailIncomplete: true,
		desktopSource:         desktopSourceFFmpeg,
		desktopMaxAttempts:    1,
		networkTimeout:        clientRequestTimeout,
		ffmpegBin:             ffmpegBin,
		ffmpegInputFormat:     defaultFFmpegInputFormat,
		ffmpegInput:           defaultFFmpegInput,
		ffmpegVideoCodec:      defaultFFmpegVideoCodec,
		ffmpegDuration:        time.Second,
		ffmpegSegmentTime:     time.Second,
	}

	var out bytes.Buffer
	err := runDesktopRecorder(context.Background(), &out, cfg)
	if err == nil {
		t.Fatal("expected ffmpeg upload failure")
	}
	if uploadRequests != 1 {
		t.Fatalf("uploadRequests = %d, want 1", uploadRequests)
	}
	if failRequests != 1 {
		t.Fatalf("failRequests = %d, want 1", failRequests)
	}
	if !strings.Contains(out.String(), "Failing stream because local staged chunks did not fully upload.") {
		t.Fatalf("output did not mention stream failure: %q", out.String())
	}
}

func TestRunDesktopRecorderResumeVerifiesBundleWithManifestMediaType(t *testing.T) {
	stage, err := openDesktopStage(filepath.Join(t.TempDir(), "stage"))
	if err != nil {
		t.Fatalf("openDesktopStage returned error: %v", err)
	}
	key, err := envelope.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	if err := envelope.SaveKeyFile(filepath.Join(stage.dir, "simulator-key.json"), key); err != nil {
		t.Fatalf("SaveKeyFile returned error: %v", err)
	}

	startedAt := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	manifest := newDesktopManifest("inc_video", "str_video", "video", desktopSourceFiles)
	if err := stage.stageChunk(&manifest, key, []byte("encoded video segment"), startedAt, startedAt.Add(chunkDuration)); err != nil {
		t.Fatalf("stageChunk returned error: %v", err)
	}
	upload, err := stage.chunkUpload(manifest, manifest.Chunks[0])
	if err != nil {
		t.Fatalf("chunkUpload returned error: %v", err)
	}
	uploadedAt := startedAt.Add(time.Minute)
	manifest.Chunks[0].Uploaded = true
	manifest.Chunks[0].UploadedAt = &uploadedAt
	if err := stage.saveManifest(manifest); err != nil {
		t.Fatalf("saveManifest returned error: %v", err)
	}
	bundleBytes := makeTestBundle(t, streamBundleManifest{
		IncidentID: "inc_video",
		StreamID:   "str_video",
		MediaType:  "video",
		ChunkCount: 1,
		Chunks: []bundleChunkManifest{
			{ChunkIndex: 1, MediaType: "video", SHA256Hex: sha256Hex(upload.body)},
		},
	}, map[string][]byte{
		"chunks/video_000001.enc": upload.body,
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/auth/login":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"token":"session-token"}`))
		case "/v1/incidents/inc_video/streams/str_video/complete":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"stream":{"id":"str_video","incident_id":"inc_video","media_type":"video","status":"complete","created_at":"2026-05-22T10:00:00Z","updated_at":"2026-05-22T10:01:00Z","completed_at":"2026-05-22T10:01:00Z"}}`))
		case "/v1/incidents/inc_video/incident-tokens":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"token_id":"itk_video","incident_id":"inc_video","token":"tok_video","created_at":"2026-05-22T10:00:00Z"}`))
		case "/i/tok_video/streams/str_video/download":
			w.Header().Set("Content-Type", "application/zip")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(bundleBytes)
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found"}}`))
		}
	}))
	defer server.Close()

	cfg := config{
		apiBase:             server.URL,
		viewerBase:          server.URL,
		username:            "admin",
		password:            "test-password",
		mediaType:           defaultMediaType,
		completeStream:      true,
		downloadBundle:      true,
		encrypt:             true,
		verifyBundleDecrypt: true,
		desktopRecorder:     true,
		desktopStageDir:     stage.dir,
		desktopResume:       true,
		desktopMaxAttempts:  defaultDesktopMaxAttempts,
		networkTimeout:      clientRequestTimeout,
		wrappedKeyOutput:    filepath.Join(stage.dir, "proofline-sim-wrapped-keys.json"),
		contactKeyFile:      filepath.Join(stage.dir, "proofline-sim-contact.key.json"),
		wrappedKeyContactID: defaultWrappedKeyContactID,
	}
	var out bytes.Buffer
	if err := runDesktopRecorder(context.Background(), &out, cfg); err != nil {
		t.Fatalf("runDesktopRecorder returned error: %v", err)
	}
	if !strings.Contains(out.String(), "Verified contact-wrapped decrypt of 1 encrypted chunks.") {
		t.Fatalf("output did not verify bundle: %q", out.String())
	}
}

func TestNetworkTransportFailsEveryNthRequest(t *testing.T) {
	baseRequests := 0
	transport := newNetworkTransport(networkProfile{
		offlineEvery: 2,
		seed:         1,
	}, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		baseRequests++
		return testResponse(http.StatusOK, "text/plain", "ok"), nil
	}))
	request, err := http.NewRequest(http.MethodGet, "http://api.example/v1/health/live", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}
	if _, err := transport.RoundTrip(request); err != nil {
		t.Fatalf("first request returned error: %v", err)
	}
	if _, err := transport.RoundTrip(request); err != errSimulatedNetwork {
		t.Fatalf("second request error = %v, want simulated network", err)
	}
	if baseRequests != 1 {
		t.Fatalf("baseRequests = %d, want 1", baseRequests)
	}
}

func TestNetworkTransportDelayRespectsContextCancellation(t *testing.T) {
	baseRequests := 0
	transport := newNetworkTransport(networkProfile{
		latency: time.Hour,
		seed:    1,
	}, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		baseRequests++
		return testResponse(http.StatusOK, "text/plain", "ok"), nil
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://api.example/v1/health/live", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}

	_, err = transport.RoundTrip(request)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RoundTrip error = %v, want context canceled", err)
	}
	if baseRequests != 0 {
		t.Fatalf("baseRequests = %d, want 0", baseRequests)
	}
}

func TestVerifyStreamBundleDecryption(t *testing.T) {
	key, err := envelope.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	incidentID := "inc_bundle"
	streamID := "str_bundle"
	mediaType := "audio"
	first := mustEncryptTestChunk(t, key, incidentID, streamID, mediaType, 1, []byte("first plaintext"))
	second := mustEncryptTestChunk(t, key, incidentID, streamID, mediaType, 2, []byte("second plaintext"))
	bundleBytes := makeTestBundle(t, streamBundleManifest{
		IncidentID: incidentID,
		StreamID:   streamID,
		MediaType:  mediaType,
		ChunkCount: 2,
		Chunks: []bundleChunkManifest{
			{ChunkIndex: 1, MediaType: mediaType, SHA256Hex: sha256Hex(first)},
			{ChunkIndex: 2, MediaType: mediaType, SHA256Hex: sha256Hex(second)},
		},
	}, map[string][]byte{
		"chunks/audio_000001.enc": first,
		"chunks/audio_000002.enc": second,
	})

	verified, err := verifyStreamBundleDecryption(bundleBytes, key, incidentID, streamID, mediaType)
	if err != nil {
		t.Fatalf("verifyStreamBundleDecryption returned error: %v", err)
	}
	if verified != 2 {
		t.Fatalf("verified = %d, want 2", verified)
	}
}

func TestRunVerifyBundle(t *testing.T) {
	key, err := envelope.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key.json")
	if err := envelope.SaveKeyFile(keyPath, key); err != nil {
		t.Fatalf("SaveKeyFile returned error: %v", err)
	}

	incidentID := "inc_bundle"
	streamID := "str_bundle"
	mediaType := "audio"
	chunk := mustEncryptTestChunk(t, key, incidentID, streamID, mediaType, 1, []byte("plaintext"))
	bundleBytes := makeTestBundle(t, streamBundleManifest{
		IncidentID: incidentID,
		StreamID:   streamID,
		MediaType:  mediaType,
		ChunkCount: 1,
		Chunks: []bundleChunkManifest{
			{ChunkIndex: 1, MediaType: mediaType, SHA256Hex: sha256Hex(chunk)},
		},
	}, map[string][]byte{
		"chunks/audio_000001.enc": chunk,
	})
	bundlePath := filepath.Join(dir, "bundle.zip")
	if err := os.WriteFile(bundlePath, bundleBytes, 0o600); err != nil {
		t.Fatalf("write test bundle: %v", err)
	}

	var out bytes.Buffer
	if err := run(context.Background(), &out, []string{
		"--verify-bundle", bundlePath,
		"--key-file", keyPath,
	}); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "Verified decrypt of 1 encrypted chunks.") {
		t.Fatalf("output = %q", output)
	}
	if strings.Contains(output, bundlePath) || strings.Contains(output, keyPath) {
		t.Fatalf("output leaked a local path: %q", output)
	}
}

func TestVerifyStreamBundleDecryptionRejectsWrongMetadata(t *testing.T) {
	key, err := envelope.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	incidentID := "inc_bundle"
	streamID := "str_bundle"
	mediaType := "audio"
	chunk := mustEncryptTestChunk(t, key, incidentID, streamID, mediaType, 1, []byte("plaintext"))
	bundleBytes := makeTestBundle(t, streamBundleManifest{
		IncidentID: incidentID,
		StreamID:   streamID,
		MediaType:  mediaType,
		ChunkCount: 1,
		Chunks: []bundleChunkManifest{
			{ChunkIndex: 1, MediaType: mediaType, SHA256Hex: sha256Hex(chunk)},
		},
	}, map[string][]byte{
		"chunks/audio_000001.enc": chunk,
	})

	if _, err := verifyStreamBundleDecryption(bundleBytes, key, "inc_changed", streamID, mediaType); err == nil {
		t.Fatal("verifyStreamBundleDecryption succeeded with wrong incident id")
	}
}

func TestWriteEncryptedBundleDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "bundle.zip")
	first := []byte("encrypted bundle bytes")
	if err := writeEncryptedBundle(path, first); err != nil {
		t.Fatalf("writeEncryptedBundle returned error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !bytes.Equal(got, first) {
		t.Fatalf("written bundle = %q, want %q", got, first)
	}
	if err := writeEncryptedBundle(path, []byte("replacement")); err == nil {
		t.Fatal("expected second write to fail")
	}
	got, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after failed overwrite returned error: %v", err)
	}
	if !bytes.Equal(got, first) {
		t.Fatalf("bundle was overwritten: %q", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func testResponse(status int, contentType, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}

func mustEncryptTestChunk(t *testing.T, key envelope.Key, incidentID, streamID, mediaType string, chunkIndex int, plaintext []byte) []byte {
	t.Helper()

	body, err := envelope.EncryptChunk(key, chunkContext(incidentID, streamID, mediaType, chunkIndex), plaintext)
	if err != nil {
		t.Fatalf("EncryptChunk returned error: %v", err)
	}
	return body
}

func makeTestBundle(t *testing.T, manifest streamBundleManifest, entries map[string][]byte) []byte {
	t.Helper()

	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	manifestWriter, err := writer.Create("manifest.json")
	if err != nil {
		t.Fatalf("create manifest entry: %v", err)
	}
	if err := json.NewEncoder(manifestWriter).Encode(manifest); err != nil {
		t.Fatalf("write manifest entry: %v", err)
	}
	for name, entryBody := range entries {
		entryWriter, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := entryWriter.Write(entryBody); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close test zip: %v", err)
	}
	return body.Bytes()
}
