package httpapi_test

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/httpapi"
)

func TestMainAPIRateLimitGroupsRoutesWithSafeKeys(t *testing.T) {
	limiter := &recordingPublicRateLimiter{allowed: true}
	app := newTestAppWithOptions(t, httpapi.Options{
		MainRateLimit: httpapi.MainRateLimitConfig{
			Enabled:            true,
			Window:             time.Minute,
			AuthLimit:          11,
			BootstrapLimit:     12,
			AccountLimit:       13,
			IncidentReadLimit:  14,
			IncidentWriteLimit: 15,
			UploadLimit:        16,
			ReconcileLimit:     17,
			StreamLimit:        18,
			TokenLimit:         19,
			DownloadLimit:      20,
			AdminLimit:         21,
		},
		MainRateLimiter: limiter,
	})

	routes := []struct {
		method string
		target string
		class  string
		limit  int
	}{
		{http.MethodPost, "/v1/auth/login", ":auth:", 11},
		{http.MethodGet, "/v1/account", ":account:", 13},
		{http.MethodGet, "/v1/incidents/inc_secret", ":incident_read:", 14},
		{http.MethodPost, "/v1/incidents", ":incident_write:", 15},
		{http.MethodPost, "/v1/incidents/inc_secret/chunks", ":upload:", 16},
		{http.MethodPost, "/v1/incidents/inc_secret/chunks/reconcile", ":reconcile:", 17},
		{http.MethodPost, "/v1/incidents/inc_secret/streams/str_secret/complete", ":stream:", 18},
		{http.MethodPost, "/v1/incidents/inc_secret/incident-tokens", ":token:", 19},
		{http.MethodGet, "/v1/incidents/inc_secret/download", ":download:", 20},
		{http.MethodGet, "/v1/admin/accounts", ":admin:", 21},
	}

	headers := map[string]string{"Idempotency-Key": "raw-idempotency-key-secret"}
	for _, route := range routes {
		response, _ := requestWithAuthAndHeaders(t, app.mainHandler, route.method, route.target, "application/json", bytes.NewBufferString(`{}`), "raw-session-token-secret", headers)
		response.Body.Close()
	}

	if len(limiter.calls) != len(routes) {
		t.Fatalf("limiter calls = %d, want %d", len(limiter.calls), len(routes))
	}
	for i, route := range routes {
		assertRateLimitCall(t, limiter.calls[i], route.class, route.limit)
		if limiter.calls[i].window != time.Minute {
			t.Fatalf("window = %s, want 1m", limiter.calls[i].window)
		}
		for _, disallowed := range []string{
			"raw-session-token-secret",
			"raw-idempotency-key-secret",
			"inc_secret",
			"str_secret",
			"/v1/",
			"192.0.2.1",
			"Authorization",
		} {
			if strings.Contains(limiter.calls[i].key, disallowed) {
				t.Fatalf("limiter key exposed %q: %s", disallowed, limiter.calls[i].key)
			}
		}
	}
}

func TestMainAPIRateLimitExhaustionUsesSafeNoStoreResponse(t *testing.T) {
	app := newTestAppWithOptions(t, httpapi.Options{
		MainRateLimit: httpapi.MainRateLimitConfig{
			Enabled:   true,
			Window:    time.Minute,
			AuthLimit: 1,
		},
	})

	response, body := postUnauthenticated(t, app, "/v1/auth/login", "application/json", bytes.NewBufferString(`{"username":"admin","password":"bad"}`))
	response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests {
		t.Fatalf("first login request was rate limited: %s", body)
	}

	response, body = postUnauthenticated(t, app, "/v1/auth/login", "application/json", bytes.NewBufferString(`{"username":"admin","password":"bad"}`))
	defer response.Body.Close()
	if response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("limited login status = %d, want 429: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)
	assertErrorCode(t, body, "rate_limited")
	if response.Header.Get("Retry-After") != "60" {
		t.Fatalf("Retry-After = %q, want 60", response.Header.Get("Retry-After"))
	}
	for _, disallowed := range []string{"admin", "bad"} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("rate limit response exposed %q: %s", disallowed, body)
		}
	}
}

func TestMainAPIRateLimitBackendFailureUsesSafeResponse(t *testing.T) {
	limiter := &recordingPublicRateLimiter{
		err: errors.New("dial 10.0.0.5:6379 with password secret failed"),
	}
	app := newTestAppWithOptions(t, httpapi.Options{
		MainRateLimit: httpapi.MainRateLimitConfig{
			Enabled:     true,
			Window:      time.Minute,
			UploadLimit: 1,
		},
		MainRateLimiter: limiter,
	})

	headers := map[string]string{"Idempotency-Key": "raw-idempotency-key-secret"}
	response, body := requestWithAuthAndHeaders(t, app.privateHandler, http.MethodPost, "/v1/incidents/inc_secret/chunks", "application/json", bytes.NewBufferString(`{}`), "raw-session-token-secret", headers)
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("backend failure status = %d, want 503: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)
	assertErrorCode(t, body, "rate_limit_unavailable")
	for _, disallowed := range []string{"inc_secret", "raw-session-token-secret", "raw-idempotency-key-secret", "10.0.0.5", "secret"} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("rate limiter error response exposed %q: %s", disallowed, body)
		}
	}
}

func TestMainAPIRateLimitCanBeDisabled(t *testing.T) {
	limiter := &recordingPublicRateLimiter{allowed: false}
	app := newTestAppWithOptions(t, httpapi.Options{
		MainRateLimit: httpapi.MainRateLimitConfig{
			Enabled:      false,
			Window:       time.Minute,
			AccountLimit: 1,
		},
		MainRateLimiter: limiter,
	})

	response, body := getUnauthenticated(t, app, "/v1/account")
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests {
		t.Fatalf("disabled limiter rejected request: %s", body)
	}
	if len(limiter.calls) != 0 {
		t.Fatalf("limiter calls = %d, want 0", len(limiter.calls))
	}
}

func TestMainAPIRateLimitSeparatesUploadAndDownloadClasses(t *testing.T) {
	app := newTestAppWithOptions(t, httpapi.Options{
		MainRateLimit: httpapi.MainRateLimitConfig{
			Enabled:       true,
			Window:        time.Minute,
			UploadLimit:   1,
			DownloadLimit: 2,
		},
	})

	response, body := postUnauthenticated(t, app, "/v1/incidents/inc_secret/chunks", "application/json", bytes.NewBufferString(`{}`))
	response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests {
		t.Fatalf("first upload-class request was rate limited: %s", body)
	}
	response, body = postUnauthenticated(t, app, "/v1/incidents/inc_secret/chunks", "application/json", bytes.NewBufferString(`{}`))
	response.Body.Close()
	if response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second upload-class status = %d, want 429: %s", response.StatusCode, body)
	}

	for i := 0; i < 2; i++ {
		response, body = getUnauthenticated(t, app, "/v1/incidents/inc_secret/download")
		response.Body.Close()
		if response.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("download-class request %d was limited early: %s", i+1, body)
		}
	}
	response, body = getUnauthenticated(t, app, "/v1/incidents/inc_secret/download")
	defer response.Body.Close()
	if response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("third download-class status = %d, want 429: %s", response.StatusCode, body)
	}
}
