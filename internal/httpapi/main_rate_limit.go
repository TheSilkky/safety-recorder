package httpapi

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
)

const mainRateLimitKeyPrefix = "proofline:main-api-rate:v1"

type mainRateLimitClass string

const (
	mainRateLimitAuth          mainRateLimitClass = "auth"
	mainRateLimitBootstrap     mainRateLimitClass = "bootstrap"
	mainRateLimitAccount       mainRateLimitClass = "account"
	mainRateLimitIncidentRead  mainRateLimitClass = "incident_read"
	mainRateLimitIncidentWrite mainRateLimitClass = "incident_write"
	mainRateLimitUpload        mainRateLimitClass = "upload"
	mainRateLimitReconcile     mainRateLimitClass = "reconcile"
	mainRateLimitStream        mainRateLimitClass = "stream"
	mainRateLimitToken         mainRateLimitClass = "token"
	mainRateLimitDownload      mainRateLimitClass = "download"
	mainRateLimitAdmin         mainRateLimitClass = "admin"
)

func (a *API) mainRateLimitMiddleware(next http.Handler) http.Handler {
	return a.mainRateLimitMiddlewareWithClassFilter(next, nil)
}

func (a *API) mainAPIRouteRateLimitMiddleware(next http.Handler) http.Handler {
	return a.mainRateLimitMiddlewareWithClassFilter(next, func(class mainRateLimitClass) bool {
		return class != mainRateLimitBootstrap
	})
}

func (a *API) mainRateLimitMiddlewareWithClassFilter(next http.Handler, allowClass func(mainRateLimitClass) bool) http.Handler {
	cfg := a.mainRateLimit
	limiter := a.mainRateLimiter
	if !cfg.Enabled || limiter == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		class, ok := classifyMainAPIRateLimit(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		if allowClass != nil && !allowClass(class) {
			next.ServeHTTP(w, r)
			return
		}

		limit := cfg.limitFor(class)
		if limit <= 0 {
			next.ServeHTTP(w, r)
			return
		}

		allowed, err := limiter.Allow(r.Context(), mainAPIRateLimitKey(r, class), limit, cfg.Window)
		if err != nil {
			a.logInternalError("main api rate limit", err)
			writeError(w, http.StatusServiceUnavailable, "rate_limit_unavailable", "rate limiter is temporarily unavailable")
			return
		}
		if !allowed {
			w.Header().Set("Retry-After", retryAfterSeconds(cfg.Window))
			writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (cfg MainRateLimitConfig) limitFor(class mainRateLimitClass) int {
	switch class {
	case mainRateLimitAuth:
		return cfg.AuthLimit
	case mainRateLimitBootstrap:
		return cfg.BootstrapLimit
	case mainRateLimitAccount:
		return cfg.AccountLimit
	case mainRateLimitIncidentRead:
		return cfg.IncidentReadLimit
	case mainRateLimitIncidentWrite:
		return cfg.IncidentWriteLimit
	case mainRateLimitUpload:
		return cfg.UploadLimit
	case mainRateLimitReconcile:
		return cfg.ReconcileLimit
	case mainRateLimitStream:
		return cfg.StreamLimit
	case mainRateLimitToken:
		return cfg.TokenLimit
	case mainRateLimitDownload:
		return cfg.DownloadLimit
	case mainRateLimitAdmin:
		return cfg.AdminLimit
	default:
		return 0
	}
}

func classifyMainAPIRateLimit(r *http.Request) (mainRateLimitClass, bool) {
	segments := strings.Split(strings.Trim(r.URL.EscapedPath(), "/"), "/")
	if len(segments) < 2 || segments[0] != "v1" {
		return "", false
	}

	switch segments[1] {
	case "auth":
		return classifyMainAPIAuthRateLimit(r, segments)
	case "bootstrap":
		if r.Method == http.MethodPost && len(segments) == 3 && segments[2] == "admin" {
			return mainRateLimitBootstrap, true
		}
	case "account":
		if (r.Method == http.MethodGet && len(segments) == 2) ||
			(r.Method == http.MethodPost && len(segments) == 3 && segments[2] == "password") {
			return mainRateLimitAccount, true
		}
	case "admin":
		return mainRateLimitAdmin, true
	case "incidents":
		return classifyMainAPIIncidentRateLimit(r, segments)
	case "incident-tokens":
		if r.Method == http.MethodPost && len(segments) == 4 && segments[3] == "revoke" {
			return mainRateLimitToken, true
		}
	}

	return "", false
}

func classifyMainAPIAuthRateLimit(r *http.Request, segments []string) (mainRateLimitClass, bool) {
	if r.Method == http.MethodPost && len(segments) == 3 {
		switch segments[2] {
		case "login", "logout":
			return mainRateLimitAuth, true
		}
	}
	return "", false
}

func classifyMainAPIIncidentRateLimit(r *http.Request, segments []string) (mainRateLimitClass, bool) {
	if len(segments) == 2 && r.Method == http.MethodPost {
		return mainRateLimitIncidentWrite, true
	}
	if len(segments) < 3 {
		return "", false
	}
	if len(segments) == 3 && r.Method == http.MethodGet {
		return mainRateLimitIncidentRead, true
	}
	if len(segments) < 4 {
		return "", false
	}

	switch segments[3] {
	case "deletion":
		if r.Method == http.MethodGet {
			return mainRateLimitIncidentRead, true
		}
		if r.Method == http.MethodPost {
			return mainRateLimitIncidentWrite, true
		}
	case "chunks":
		return classifyMainAPIChunkRateLimit(r, segments)
	case "download":
		if r.Method == http.MethodGet && len(segments) == 4 {
			return mainRateLimitDownload, true
		}
	case "checkins", "close":
		if r.Method == http.MethodPost && len(segments) == 4 {
			return mainRateLimitIncidentWrite, true
		}
	case "streams":
		return classifyMainAPIStreamRateLimit(r, segments)
	case "incident-tokens":
		if r.Method == http.MethodPost && len(segments) == 4 {
			return mainRateLimitToken, true
		}
	}

	return "", false
}

func classifyMainAPIChunkRateLimit(r *http.Request, segments []string) (mainRateLimitClass, bool) {
	switch {
	case r.Method == http.MethodPost && len(segments) == 4:
		return mainRateLimitUpload, true
	case r.Method == http.MethodPost && len(segments) == 5 && segments[4] == "reconcile":
		return mainRateLimitReconcile, true
	case r.Method == http.MethodGet && len(segments) == 4:
		return mainRateLimitIncidentRead, true
	case r.Method == http.MethodGet && len(segments) == 6:
		return mainRateLimitDownload, true
	default:
		return "", false
	}
}

func classifyMainAPIStreamRateLimit(r *http.Request, segments []string) (mainRateLimitClass, bool) {
	switch {
	case len(segments) == 4 && (r.Method == http.MethodGet || r.Method == http.MethodPost):
		return mainRateLimitStream, true
	case len(segments) == 5 && r.Method == http.MethodGet:
		return mainRateLimitStream, true
	case len(segments) == 6 && r.Method == http.MethodPost && (segments[5] == "complete" || segments[5] == "fail"):
		return mainRateLimitStream, true
	case len(segments) == 6 && r.Method == http.MethodGet && segments[5] == "download":
		return mainRateLimitDownload, true
	default:
		return "", false
	}
}

func mainAPIRateLimitKey(r *http.Request, class mainRateLimitClass) string {
	identityHash := sha256.Sum256([]byte(clientIdentitySignal(r)))
	return fmt.Sprintf("%s:%s:%x", mainRateLimitKeyPrefix, class, identityHash)
}
