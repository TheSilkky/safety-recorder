package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteErrorPreservesPublicEnvelopeAndHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()

	writeError(recorder, http.StatusTeapot, "sample_error", "sample message")

	response := recorder.Result()
	if response.StatusCode != http.StatusTeapot {
		t.Fatalf("expected status %d, got %d", http.StatusTeapot, response.StatusCode)
	}
	assertJSONSecurityHeaders(t, response)

	var body jsonErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Error.Code != "sample_error" {
		t.Fatalf("expected error code sample_error, got %q", body.Error.Code)
	}
	if body.Error.Message != "sample message" {
		t.Fatalf("expected error message sample message, got %q", body.Error.Message)
	}
}

func TestWriteJSONSetsStableJSONHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()

	writeJSON(recorder, http.StatusAccepted, map[string]string{"status": "accepted"})

	response := recorder.Result()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, response.StatusCode)
	}
	assertJSONSecurityHeaders(t, response)
}

func assertJSONSecurityHeaders(t *testing.T, response *http.Response) {
	t.Helper()

	if response.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("expected application/json, got %q", response.Header.Get("Content-Type"))
	}
	if response.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("expected nosniff, got %q", response.Header.Get("X-Content-Type-Options"))
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("expected no-store cache policy, got %q", response.Header.Get("Cache-Control"))
	}
	if value := response.Header.Get("Strict-Transport-Security"); value != "" {
		t.Fatalf("expected no app-level Strict-Transport-Security header in local/dev HTTP mode, got %q", value)
	}
}
