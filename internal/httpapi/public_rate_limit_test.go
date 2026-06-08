package httpapi_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/httpapi"
)

func TestPublicViewerRateLimitGroupsRoutesWithSafeKeys(t *testing.T) {
	limiter := &recordingPublicRateLimiter{allowed: true}
	app := newTestAppWithOptions(t, httpapi.Options{
		PublicRateLimit: httpapi.PublicRateLimitConfig{
			Enabled:       true,
			Window:        time.Minute,
			PageLimit:     11,
			DataLimit:     22,
			DownloadLimit: 33,
			StaticLimit:   44,
		},
		PublicRateLimiter: limiter,
	})

	for _, target := range []string{
		"/i/raw-viewer-token-secret",
		"/e/raw-viewer-token-secret/data",
		"/i/raw-viewer-token-secret/streams/str_secret/download",
		"/static/styles.css",
	} {
		response, _ := getPublic(t, app, target)
		response.Body.Close()
	}

	if len(limiter.calls) != 4 {
		t.Fatalf("limiter calls = %d, want 4", len(limiter.calls))
	}
	assertRateLimitCall(t, limiter.calls[0], ":page:", 11)
	assertRateLimitCall(t, limiter.calls[1], ":data:", 22)
	assertRateLimitCall(t, limiter.calls[2], ":download:", 33)
	assertRateLimitCall(t, limiter.calls[3], ":static:", 44)
	for _, call := range limiter.calls {
		if call.window != time.Minute {
			t.Fatalf("window = %s, want 1m", call.window)
		}
		for _, disallowed := range []string{
			"raw-viewer-token-secret",
			"str_secret",
			"/i/",
			"/e/",
			"192.0.2.1",
			"Authorization",
		} {
			if strings.Contains(call.key, disallowed) {
				t.Fatalf("limiter key exposed %q: %s", disallowed, call.key)
			}
		}
	}
}

func TestPublicViewerRateLimitExhaustionUsesSafeNoStoreResponse(t *testing.T) {
	app := newTestAppWithOptions(t, httpapi.Options{
		PublicRateLimit: httpapi.PublicRateLimitConfig{
			Enabled:   true,
			Window:    time.Minute,
			PageLimit: 1,
		},
	})
	incidentID := createIncident(t, app, `{}`)
	token := createIncidentToken(t, app, incidentID, "trusted contact", nil)

	response, body := getPublic(t, app, "/i/"+token.Token)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("first viewer request status = %d, want 200: %s", response.StatusCode, body)
	}

	response, body = getPublic(t, app, "/i/"+token.Token)
	defer response.Body.Close()
	if response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("limited viewer request status = %d, want 429: %s", response.StatusCode, body)
	}
	assertIncidentViewerPrivacyHeaders(t, response)
	assertErrorCode(t, body, "rate_limited")
	if response.Header.Get("Retry-After") != "60" {
		t.Fatalf("Retry-After = %q, want 60", response.Header.Get("Retry-After"))
	}
	if bytes.Contains(body, []byte(token.Token)) {
		t.Fatalf("rate limit response exposed raw token: %s", body)
	}
}

func TestPublicViewerRateLimitCanLimitStaticAssetsSeparately(t *testing.T) {
	app := newTestAppWithOptions(t, httpapi.Options{
		PublicRateLimit: httpapi.PublicRateLimitConfig{
			Enabled:     true,
			Window:      time.Minute,
			StaticLimit: 1,
		},
	})

	response, body := getPublic(t, app, "/static/styles.css")
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("first static request status = %d, want 200: %s", response.StatusCode, body)
	}

	response, body = getPublic(t, app, "/static/styles.css")
	defer response.Body.Close()
	if response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("limited static request status = %d, want 429: %s", response.StatusCode, body)
	}
	assertPublicBrowserSecurityHeaders(t, response)
	assertNoStore(t, response)
	assertErrorCode(t, body, "rate_limited")
}

func TestPublicViewerRateLimitAppliesToHeadRequests(t *testing.T) {
	limiter := &recordingPublicRateLimiter{allowed: false}
	app := newTestAppWithOptions(t, httpapi.Options{
		PublicRateLimit: httpapi.PublicRateLimitConfig{
			Enabled:   true,
			Window:    time.Minute,
			PageLimit: 7,
		},
		PublicRateLimiter: limiter,
	})

	response, body := request(t, app.publicHandler, http.MethodHead, "/i/raw-viewer-token-secret", "", nil)
	defer response.Body.Close()
	if response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("limited HEAD status = %d, want 429: %s", response.StatusCode, body)
	}
	if len(limiter.calls) != 1 {
		t.Fatalf("limiter calls = %d, want 1", len(limiter.calls))
	}
	assertRateLimitCall(t, limiter.calls[0], ":page:", 7)
}

func TestPublicViewerRateLimitBackendFailureUsesSafeResponse(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	limiter := &recordingPublicRateLimiter{
		err: errors.New("dependency failure with <private endpoint> and <credential>"),
	}
	app := newTestAppWithOptions(t, httpapi.Options{
		PublicRateLimit: httpapi.PublicRateLimitConfig{
			Enabled:   true,
			Window:    time.Minute,
			DataLimit: 1,
		},
		PublicRateLimiter: limiter,
		Logger:            logger,
	})

	response, body := getPublic(t, app, "/i/raw-viewer-token-secret/data")
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("backend failure status = %d, want 503: %s", response.StatusCode, body)
	}
	assertIncidentViewerPrivacyHeaders(t, response)
	assertErrorCode(t, body, "rate_limit_unavailable")
	for _, disallowed := range []string{"raw-viewer-token-secret", "<private endpoint>", "<credential>"} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("rate limiter error response exposed %q: %s", disallowed, body)
		}
	}
	for _, disallowed := range []string{"raw-viewer-token-secret", "<private endpoint>", "<credential>"} {
		if bytes.Contains(logs.Bytes(), []byte(disallowed)) {
			t.Fatalf("rate limiter log exposed %q: %s", disallowed, logs.String())
		}
	}
	for _, want := range []string{
		"component=httpapi",
		"operation=\"public viewer rate limit\"",
		"route_class=data",
		"error_category=rate_limit_unavailable",
	} {
		if !bytes.Contains(logs.Bytes(), []byte(want)) {
			t.Fatalf("rate limiter log omitted %q: %s", want, logs.String())
		}
	}
}

func TestLimitedPublicViewerDownloadLogUsesRedactedRouteClass(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	limiter := &recordingPublicRateLimiter{allowed: false}
	app := newTestAppWithOptions(t, httpapi.Options{
		PublicRateLimit: httpapi.PublicRateLimitConfig{
			Enabled:       true,
			Window:        time.Minute,
			DownloadLimit: 1,
		},
		PublicRateLimiter: limiter,
		Logger:            logger,
	})

	response, body := getPublic(t, app, "/i/raw-viewer-token-secret/streams/str_secret/download")
	defer response.Body.Close()
	if response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("limited download status = %d, want 429: %s", response.StatusCode, body)
	}
	if !bytes.Contains(logs.Bytes(), []byte("path=/i/{token}/streams/{stream_id}/download")) {
		t.Fatalf("expected redacted download route in logs: %s", logs.String())
	}
	for _, disallowed := range []string{"raw-viewer-token-secret", "str_secret"} {
		if bytes.Contains(logs.Bytes(), []byte(disallowed)) {
			t.Fatalf("limited download log exposed %q: %s", disallowed, logs.String())
		}
	}
}

type recordingPublicRateLimiter struct {
	allowed bool
	err     error
	calls   []publicRateLimitCall
}

type publicRateLimitCall struct {
	key    string
	limit  int
	window time.Duration
}

func (l *recordingPublicRateLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (bool, error) {
	l.calls = append(l.calls, publicRateLimitCall{
		key:    key,
		limit:  limit,
		window: window,
	})
	if l.err != nil {
		return false, l.err
	}
	return l.allowed, nil
}

func assertRateLimitCall(t *testing.T, call publicRateLimitCall, class string, limit int) {
	t.Helper()

	if !strings.Contains(call.key, class) {
		t.Fatalf("expected limiter key to include class %q, got %q", class, call.key)
	}
	if call.limit != limit {
		t.Fatalf("limit = %d, want %d", call.limit, limit)
	}
}
