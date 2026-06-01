package httpapi

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const publicRateLimitKeyPrefix = "proofline:public-viewer-rate:v1"

type publicRateLimitClass string

const (
	publicRateLimitPage     publicRateLimitClass = "page"
	publicRateLimitData     publicRateLimitClass = "data"
	publicRateLimitDownload publicRateLimitClass = "download"
	publicRateLimitStatic   publicRateLimitClass = "static"
)

type memoryRateLimiter struct {
	mu      sync.Mutex
	entries map[string]memoryRateLimitEntry
	now     func() time.Time
}

type memoryRateLimitEntry struct {
	count     int
	expiresAt time.Time
}

// NewMemoryRateLimiter returns a process-local fixed-window limiter.
func NewMemoryRateLimiter() RateLimiter {
	return &memoryRateLimiter{
		entries: make(map[string]memoryRateLimitEntry),
		now:     time.Now,
	}
}

// NewMemoryPublicRateLimiter returns a process-local fixed-window limiter for
// public viewer route classes.
func NewMemoryPublicRateLimiter() PublicRateLimiter {
	return NewMemoryRateLimiter()
}

func (l *memoryRateLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (bool, error) {
	if limit <= 0 || window <= 0 {
		return true, nil
	}

	now := l.now().UTC()
	l.mu.Lock()
	defer l.mu.Unlock()

	l.pruneExpired(now)
	entry := l.entries[key]
	if entry.expiresAt.IsZero() || !now.Before(entry.expiresAt) {
		entry = memoryRateLimitEntry{
			count:     1,
			expiresAt: now.Add(window),
		}
		l.entries[key] = entry
		return true, nil
	}

	entry.count++
	l.entries[key] = entry
	return entry.count <= limit, nil
}

func (l *memoryRateLimiter) pruneExpired(now time.Time) {
	for key, entry := range l.entries {
		if !now.Before(entry.expiresAt) {
			delete(l.entries, key)
		}
	}
}

func (a *API) publicRateLimitMiddleware(next http.Handler) http.Handler {
	cfg := a.publicRateLimit
	limiter := a.publicRateLimiter
	if !cfg.Enabled || limiter == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		class, ok := classifyPublicViewerRateLimit(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		limit := cfg.limitFor(class)
		if limit <= 0 {
			next.ServeHTTP(w, r)
			return
		}

		allowed, err := limiter.Allow(r.Context(), publicViewerRateLimitKey(r, class), limit, cfg.Window)
		if err != nil {
			a.logInternalError("public viewer rate limit", err)
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

func (cfg PublicRateLimitConfig) limitFor(class publicRateLimitClass) int {
	switch class {
	case publicRateLimitPage:
		return cfg.PageLimit
	case publicRateLimitData:
		return cfg.DataLimit
	case publicRateLimitDownload:
		return cfg.DownloadLimit
	case publicRateLimitStatic:
		return cfg.StaticLimit
	default:
		return 0
	}
}

func classifyPublicViewerRateLimit(r *http.Request) (publicRateLimitClass, bool) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return "", false
	}

	path := r.URL.EscapedPath()
	if strings.HasPrefix(path, "/static/") {
		return publicRateLimitStatic, true
	}

	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) < 2 || (segments[0] != "i" && segments[0] != "e") {
		return "", false
	}

	switch {
	case len(segments) == 2:
		return publicRateLimitPage, true
	case len(segments) == 3 && segments[2] == "data":
		return publicRateLimitData, true
	case len(segments) == 4 && segments[2] == "incident" && segments[3] == "download":
		return publicRateLimitDownload, true
	case len(segments) == 5 && segments[2] == "streams" && segments[4] == "download":
		return publicRateLimitDownload, true
	default:
		return "", false
	}
}

func publicViewerRateLimitKey(r *http.Request, class publicRateLimitClass) string {
	identityHash := sha256.Sum256([]byte(clientIdentitySignal(r)))
	return fmt.Sprintf("%s:%s:%x", publicRateLimitKeyPrefix, class, identityHash)
}

func clientIdentitySignal(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if strings.TrimSpace(r.RemoteAddr) == "" {
		return "unknown"
	}
	return r.RemoteAddr
}

func retryAfterSeconds(window time.Duration) string {
	seconds := int((window + time.Second - 1) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}
