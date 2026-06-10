package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoadConfigUsesEnvironmentAndFlags(t *testing.T) {
	t.Setenv("SAFE_STREAM_INGRESS_BIND_ADDR", "127.0.0.1:9000")
	t.Setenv("SAFE_STREAM_INGRESS_RELAY_ID", "relay-secret-label")
	t.Setenv("SAFE_STREAM_INGRESS_REGION", "private-region-label")
	t.Setenv("SAFE_STREAM_INGRESS_READY", "true")

	cfg, err := loadConfig([]string{"--bind", "127.0.0.1:9001", "--ready=false"})
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

func TestHealthRoutesAndReadinessDoNotExposeRelayLabels(t *testing.T) {
	handler := newHandler(streamIngressConfig{
		RelayID: "relay-secret-label",
		Region:  "private-region-label",
		Ready:   true,
	})

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
	if decoded["uploads"] != "unimplemented" {
		t.Fatalf("readiness uploads = %v, want unimplemented", decoded["uploads"])
	}
	if decoded["relay_identity_configured"] != true || decoded["region_configured"] != true {
		t.Fatalf("readiness config booleans = %v", decoded)
	}
}

func TestReadinessDefaultsToNotReady(t *testing.T) {
	handler := newHandler(streamIngressConfig{})

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
	handler := newHandler(streamIngressConfig{Ready: true})
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
	handler := newHandler(streamIngressConfig{Ready: true})
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
