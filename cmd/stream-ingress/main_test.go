package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigUsesEnvironmentAndFlags(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "core-token")
	if err := os.WriteFile(secretPath, []byte("file-core-service-token-1234567890\n"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	t.Setenv("SAFE_STREAM_INGRESS_BIND_ADDR", "127.0.0.1:9000")
	t.Setenv("SAFE_STREAM_INGRESS_RELAY_ID", "relay-secret-label")
	t.Setenv("SAFE_STREAM_INGRESS_REGION", "private-region-label")
	t.Setenv("SAFE_STREAM_INGRESS_READY", "true")
	t.Setenv("SAFE_STREAM_INGRESS_CORE_BASE_URL", "https://core.example.invalid/api/")
	t.Setenv("SAFE_STREAM_INGRESS_CORE_SERVICE_AUTH_TOKEN_FILE", secretPath)
	t.Setenv("SAFE_STREAM_INGRESS_DATA_DIR", "/tmp/relay-data")
	t.Setenv("SAFE_STREAM_INGRESS_MAX_UPLOAD_BYTES", "8MB")
	t.Setenv("SAFE_STREAM_INGRESS_TEMP_STAGING_QUOTA_BYTES", "16MB")
	t.Setenv("SAFE_STREAM_INGRESS_CORE_REQUEST_TIMEOUT", "5s")
	t.Setenv("SAFE_STREAM_INGRESS_MAX_IN_FLIGHT_PER_SESSION", "3")
	t.Setenv("SAFE_STREAM_INGRESS_MAX_IN_FLIGHT_PER_CLIENT", "7")

	cfg, err := loadConfig([]string{
		"--bind", "127.0.0.1:9001",
		"--ready=false",
		"--core-url", "http://127.0.0.1:8080/",
		"--max-upload-bytes", "4MB",
		"--temp-staging-quota-bytes", "9MB",
		"--core-request-timeout", "2s",
		"--max-in-flight-per-session", "5",
		"--max-in-flight-per-client", "6",
	})
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.BindAddr != "127.0.0.1:9001" {
		t.Fatalf("BindAddr = %q, want flag override", cfg.BindAddr)
	}
	if cfg.RelayID != "relay-secret-label" {
		t.Fatalf("RelayID = %q, want environment value", cfg.RelayID)
	}
	if cfg.Region != "private-region-label" {
		t.Fatalf("Region = %q, want environment value", cfg.Region)
	}
	if cfg.Ready {
		t.Fatal("Ready = true, want flag override false")
	}
	if cfg.CoreBaseURL != "http://127.0.0.1:8080" {
		t.Fatalf("CoreBaseURL = %q, want normalized flag override", cfg.CoreBaseURL)
	}
	if cfg.CoreServiceAuthToken != "file-core-service-token-1234567890" {
		t.Fatalf("CoreServiceAuthToken = %q, want file value", cfg.CoreServiceAuthToken)
	}
	if cfg.DataDir != "/tmp/relay-data" {
		t.Fatalf("DataDir = %q, want env value", cfg.DataDir)
	}
	if cfg.MaxUploadBytes != 4*1024*1024 {
		t.Fatalf("MaxUploadBytes = %d, want 4MB", cfg.MaxUploadBytes)
	}
	if cfg.TempStagingQuotaBytes != 9*1024*1024 {
		t.Fatalf("TempStagingQuotaBytes = %d, want 9MB", cfg.TempStagingQuotaBytes)
	}
	if cfg.CoreRequestTimeout != 2*time.Second {
		t.Fatalf("CoreRequestTimeout = %s, want 2s", cfg.CoreRequestTimeout)
	}
	if cfg.MaxInFlightPerSession != 5 || cfg.MaxInFlightPerClient != 6 {
		t.Fatalf("in-flight limits = %d/%d, want 5/6", cfg.MaxInFlightPerSession, cfg.MaxInFlightPerClient)
	}
}

func TestLoadConfigRejectsInvalidReadyEnvironment(t *testing.T) {
	t.Setenv("SAFE_STREAM_INGRESS_READY", "sometimes")

	_, err := loadConfig(nil)
	if err == nil {
		t.Fatal("loadConfig succeeded, want invalid boolean error")
	}
	var parseErr configParseError
	if !strings.Contains(err.Error(), "SAFE_STREAM_INGRESS_READY") || !strings.Contains(err.Error(), "boolean") {
		t.Fatalf("loadConfig error = %v, want safe boolean config error", err)
	}
	if !errors.As(err, &parseErr) {
		t.Fatalf("loadConfig error type = %T, want configParseError", err)
	}
}

func TestLoadConfigRejectsInvalidUploadSettingsSafely(t *testing.T) {
	t.Run("short core token", func(t *testing.T) {
		t.Setenv("SAFE_STREAM_INGRESS_CORE_SERVICE_AUTH_TOKEN", "short-secret-token")
		_, err := loadConfig(nil)
		if err == nil {
			t.Fatal("loadConfig succeeded, want short token error")
		}
		if !strings.Contains(err.Error(), "SAFE_STREAM_INGRESS_CORE_SERVICE_AUTH_TOKEN") ||
			strings.Contains(err.Error(), "short-secret-token") {
			t.Fatalf("loadConfig error = %v, want safe token config error", err)
		}
	})
	t.Run("invalid core URL", func(t *testing.T) {
		t.Setenv("SAFE_STREAM_INGRESS_CORE_BASE_URL", "ftp://private.example.invalid")
		_, err := loadConfig(nil)
		if err == nil {
			t.Fatal("loadConfig succeeded, want invalid URL error")
		}
		if !strings.Contains(err.Error(), "SAFE_STREAM_INGRESS_CORE_BASE_URL") ||
			strings.Contains(err.Error(), "private.example.invalid") {
			t.Fatalf("loadConfig error = %v, want safe core URL config error", err)
		}
	})
}

func TestHealthRoutesAndReadinessDoNotExposeRelayLabels(t *testing.T) {
	handler := newHandler(streamIngressConfig{
		RelayID: "relay-secret-label",
		Region:  "private-region-label",
		Ready:   true,
	}, nil)

	live := httptest.NewRecorder()
	handler.ServeHTTP(live, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if live.Code != http.StatusOK {
		t.Fatalf("/health/live status = %d, want 200", live.Code)
	}

	ready := httptest.NewRecorder()
	handler.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if ready.Code != http.StatusOK {
		t.Fatalf("/health/ready status = %d, want 200", ready.Code)
	}
	body := ready.Body.String()
	if strings.Contains(body, "relay-secret-label") || strings.Contains(body, "private-region-label") {
		t.Fatalf("readiness response exposed relay labels: %s", body)
	}

	var decoded map[string]any
	if err := json.Unmarshal(ready.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode readiness: %v", err)
	}
	if decoded["uploads"] != "unconfigured" {
		t.Fatalf("readiness uploads = %v, want unconfigured", decoded["uploads"])
	}
	if decoded["relay_identity_configured"] != true || decoded["region_configured"] != true {
		t.Fatalf("readiness config booleans = %v", decoded)
	}
}

func TestReadinessDefaultsToNotReady(t *testing.T) {
	handler := newHandler(streamIngressConfig{}, nil)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("/health/ready status = %d, want 503", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"status":"not_ready"`) {
		t.Fatalf("/health/ready body = %s, want not_ready", recorder.Body.String())
	}
}

func TestStreamIngressRouteSurfaceIsMinimal(t *testing.T) {
	handler := newHandler(streamIngressConfig{Ready: true}, nil)
	disallowed := []string{
		"/",
		"/health/live/",
		"/health/ready/",
		"/v1",
		"/v1/",
		"/v1/health/live",
		"/v1/incidents",
		"/v1/admin/accounts",
		"/admin",
		"/admin/",
		"/admin/api/accounts",
		"/i/viewer-token",
		"/e/viewer-token",
		"/bundle",
		"/download",
		"/delete",
		"/retention",
		"/backup",
		"/restore",
		"/escrow",
		"/break-glass",
		"/decrypt",
		"/raw-key",
		"/operator",
		"/metrics",
		"/v1/relay/preflight",
		"/v1/relay/commit",
	}

	for _, path := range disallowed {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("GET %s status = %d, want 404", path, recorder.Code)
		}
	}
}

func TestHealthRoutesRejectNonGetMethods(t *testing.T) {
	handler := newHandler(streamIngressConfig{Ready: true}, nil)
	for _, path := range []string{"/health/live", "/health/ready"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, nil))
		if recorder.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s status = %d, want 405", path, recorder.Code)
		}
		if recorder.Header().Get("Allow") != http.MethodGet {
			t.Fatalf("POST %s Allow = %q, want GET", path, recorder.Header().Get("Allow"))
		}
	}
}

func TestUploadRouteRejectsNonPostMethods(t *testing.T) {
	handler := newHandler(streamIngressConfig{Ready: true}, nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/upload/complete-chunk", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /upload/complete-chunk status = %d, want 405", recorder.Code)
	}
	if recorder.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("GET /upload/complete-chunk Allow = %q, want POST", recorder.Header().Get("Allow"))
	}
}
