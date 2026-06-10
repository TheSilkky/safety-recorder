package httpapi_test

import (
	"bytes"
	"errors"
	"log/slog"
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
			AuthRegisterLimit:  22,
			AuthEmailVerify:    23,
			BootstrapLimit:     12,
			AccountLimit:       13,
			IncidentReadLimit:  14,
			IncidentWriteLimit: 15,
			UploadLimit:        16,
			ReconcileLimit:     17,
			StreamLimit:        18,
			TokenLimit:         19,
			DownloadLimit:      20,
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
		{http.MethodPost, "/v1/auth/logout", ":auth:", 11},
		{http.MethodPost, "/v1/auth/web/login", ":auth:", 11},
		{http.MethodPost, "/v1/auth/web/logout", ":auth:", 11},
		{http.MethodGet, "/v1/auth/web/csrf", ":auth:", 11},
		{http.MethodPost, "/v1/auth/register", ":auth_register:", 22},
		{http.MethodPost, "/v1/auth/email/verify", ":auth_email_verify:", 23},
		{http.MethodGet, "/v1/account", ":account:", 13},
		{http.MethodPost, "/v1/account/password", ":account:", 13},
		{http.MethodPost, "/v1/account/second-factor/email/challenge", ":auth_email_verify:", 23},
		{http.MethodPost, "/v1/account/second-factor/email/verify", ":auth_email_verify:", 23},
		{http.MethodPost, "/v1/account/second-factor/totp/enroll", ":auth_email_verify:", 23},
		{http.MethodPost, "/v1/account/second-factor/totp/confirm", ":auth_email_verify:", 23},
		{http.MethodPost, "/v1/account/second-factor/totp/verify", ":auth_email_verify:", 23},
		{http.MethodPost, "/v1/account-recipient-keys", ":account:", 13},
		{http.MethodGet, "/v1/account-recipient-keys", ":account:", 13},
		{http.MethodGet, "/v1/account-recipient-keys/recipient_key_secret", ":account:", 13},
		{http.MethodPatch, "/v1/account-recipient-keys/recipient_key_secret", ":account:", 13},
		{http.MethodPost, "/v1/account-recipient-keys/recipient_key_secret/revoke", ":account:", 13},
		{http.MethodPost, "/v1/account-recipient-keys/recipient_key_secret/lost", ":account:", 13},
		{http.MethodPost, "/v1/account-recipient-keys/recipient_key_secret/replace", ":account:", 13},
		{http.MethodPost, "/v1/contact-public-keys", ":account:", 13},
		{http.MethodGet, "/v1/contact-public-keys", ":account:", 13},
		{http.MethodGet, "/v1/contact-public-keys/contact_secret", ":account:", 13},
		{http.MethodPatch, "/v1/contact-public-keys/contact_secret", ":account:", 13},
		{http.MethodPost, "/v1/contact-public-keys/contact_secret/revoke", ":account:", 13},
		{http.MethodPost, "/v1/contact-public-keys/contact_secret/lost", ":account:", 13},
		{http.MethodPost, "/v1/contact-public-keys/contact_secret/replace", ":account:", 13},
		{http.MethodPost, "/v1/trusted-contact-relationships", ":account:", 13},
		{http.MethodGet, "/v1/trusted-contact-relationships", ":account:", 13},
		{http.MethodGet, "/v1/trusted-contact-relationships/relationship_secret", ":account:", 13},
		{http.MethodPost, "/v1/trusted-contact-relationships/relationship_secret/accept", ":account:", 13},
		{http.MethodPost, "/v1/trusted-contact-relationships/relationship_secret/decline", ":account:", 13},
		{http.MethodPost, "/v1/trusted-contact-relationships/relationship_secret/revoke", ":account:", 13},
		{http.MethodPost, "/v1/trusted-contact-relationships/relationship_secret/replace", ":account:", 13},
		{http.MethodGet, "/v1/trusted-contact/incidents/inc_secret/wrapped-keys", ":incident_read:", 14},
		{http.MethodGet, "/v1/trusted-contact/wrapped-keys/wrapped_secret", ":incident_read:", 14},
		{http.MethodGet, "/v1/incidents", ":incident_read:", 14},
		{http.MethodGet, "/v1/incidents/inc_secret", ":incident_read:", 14},
		{http.MethodGet, "/v1/incidents/inc_secret/deletion", ":incident_read:", 14},
		{http.MethodGet, "/v1/incidents/inc_secret/sharing-grants", ":incident_read:", 14},
		{http.MethodGet, "/v1/incidents/inc_secret/wrapped-keys", ":incident_read:", 14},
		{http.MethodGet, "/v1/incidents/inc_secret/chunks", ":incident_read:", 14},
		{http.MethodGet, "/v1/sharing-grants/grant_secret", ":incident_read:", 14},
		{http.MethodGet, "/v1/wrapped-keys/wrapped_secret", ":incident_read:", 14},
		{http.MethodPost, "/v1/incidents", ":incident_write:", 15},
		{http.MethodPost, "/v1/incidents/inc_secret/deletion", ":incident_write:", 15},
		{http.MethodPost, "/v1/incidents/inc_secret/sharing-grants", ":incident_write:", 15},
		{http.MethodPost, "/v1/incidents/inc_secret/wrapped-keys", ":incident_write:", 15},
		{http.MethodPost, "/v1/incidents/inc_secret/checkins", ":incident_write:", 15},
		{http.MethodPost, "/v1/incidents/inc_secret/close", ":incident_write:", 15},
		{http.MethodPost, "/v1/sharing-grants/grant_secret/revoke", ":incident_write:", 15},
		{http.MethodPost, "/v1/wrapped-keys/wrapped_secret/revoke", ":incident_write:", 15},
		{http.MethodPost, "/v1/incidents/inc_secret/chunks", ":upload:", 16},
		{http.MethodPost, "/v1/incidents/inc_secret/chunks/reconcile", ":reconcile:", 17},
		{http.MethodPost, "/v1/incidents/inc_secret/streams", ":stream:", 18},
		{http.MethodGet, "/v1/incidents/inc_secret/streams", ":stream:", 18},
		{http.MethodGet, "/v1/incidents/inc_secret/streams/str_secret", ":stream:", 18},
		{http.MethodPost, "/v1/incidents/inc_secret/streams/str_secret/complete", ":stream:", 18},
		{http.MethodPost, "/v1/incidents/inc_secret/streams/str_secret/fail", ":stream:", 18},
		{http.MethodGet, "/v1/incidents/inc_secret/incident-tokens", ":token:", 19},
		{http.MethodPost, "/v1/incidents/inc_secret/incident-tokens", ":token:", 19},
		{http.MethodGet, "/v1/incidents/inc_secret/incident-tokens/token_secret", ":token:", 19},
		{http.MethodPost, "/v1/incident-tokens/token_secret/revoke", ":token:", 19},
		{http.MethodGet, "/v1/incidents/inc_secret/chunks/audio/1", ":download:", 20},
		{http.MethodGet, "/v1/incidents/inc_secret/download", ":download:", 20},
		{http.MethodGet, "/v1/incidents/inc_secret/streams/str_secret/download", ":download:", 20},
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
			"recipient_key_secret",
			"relationship_secret",
			"inc_secret",
			"str_secret",
			"contact_secret",
			"grant_secret",
			"wrapped_secret",
			"token_secret",
			"/v1/",
			"192.0.2.1",
			"Authorization",
		} {
			if strings.Contains(limiter.calls[i].key, disallowed) {
				t.Fatalf("limiter key exposed %q: %s", disallowed, limiter.calls[i].key)
			}
		}
	}

	response, body := requestWithAuthAndHeaders(t, app.mainHandler, http.MethodGet, "/v1/admin/accounts", "application/json", bytes.NewBufferString(`{}`), "raw-session-token-secret", headers)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected unmounted main admin route status 404, got %d: %s", response.StatusCode, body)
	}
	if len(limiter.calls) != len(routes) {
		t.Fatalf("admin route should not use main API rate limiter: calls = %d, want %d", len(limiter.calls), len(routes))
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
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	limiter := &recordingPublicRateLimiter{
		err: errors.New("dependency failure with <private endpoint> and <credential>"),
	}
	app := newTestAppWithOptions(t, httpapi.Options{
		MainRateLimit: httpapi.MainRateLimitConfig{
			Enabled:     true,
			Window:      time.Minute,
			UploadLimit: 1,
		},
		MainRateLimiter: limiter,
		Logger:          logger,
	})

	headers := map[string]string{"Idempotency-Key": "raw-idempotency-key-secret"}
	response, body := requestWithAuthAndHeaders(t, app.privateHandler, http.MethodPost, "/v1/incidents/inc_secret/chunks", "application/json", bytes.NewBufferString(`{}`), "raw-session-token-secret", headers)
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("backend failure status = %d, want 503: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)
	assertErrorCode(t, body, "rate_limit_unavailable")
	for _, disallowed := range []string{"inc_secret", "raw-session-token-secret", "raw-idempotency-key-secret", "<private endpoint>", "<credential>"} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("rate limiter error response exposed %q: %s", disallowed, body)
		}
	}
	for _, disallowed := range []string{"inc_secret", "raw-session-token-secret", "raw-idempotency-key-secret", "<private endpoint>", "<credential>"} {
		if bytes.Contains(logs.Bytes(), []byte(disallowed)) {
			t.Fatalf("rate limiter log exposed %q: %s", disallowed, logs.String())
		}
	}
	for _, want := range []string{
		"component=httpapi",
		"operation=\"main api rate limit\"",
		"route_class=upload",
		"error_category=rate_limit_unavailable",
	} {
		if !bytes.Contains(logs.Bytes(), []byte(want)) {
			t.Fatalf("rate limiter log omitted %q: %s", want, logs.String())
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
