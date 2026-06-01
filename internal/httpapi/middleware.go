package httpapi

import (
	"net/http"
	"strings"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

// WriteHeader records the response status before forwarding it to the client.
func (r *statusRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Write records response size for access logs without inspecting response
// contents.
func (r *statusRecorder) Write(bytes []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(bytes)
	r.bytes += n
	return n, err
}

func (a *API) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		// Log routing metadata only. Bodies, upload bytes, Authorization headers,
		// and any future token-like values are deliberately omitted.
		a.logger.Info("request",
			"method", r.Method,
			"path", safeLogPath(r),
			"status", status,
			"bytes", recorder.bytes,
			"duration_ms", time.Since(started).Milliseconds(),
		)
	})
}

func (a *API) publicSecurityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setPublicBrowserSecurityHeaders(w)
		if isViewerTokenPath(r.URL.Path) {
			setNoStore(w)
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) mainSecurityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setPublicBrowserSecurityHeaders(w)
		if isViewerTokenPath(r.URL.Path) {
			setNoStore(w)
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) privateSecurityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setNoSniff(w)
		setNoStore(w)
		next.ServeHTTP(w, r)
	})
}

func isViewerTokenPath(path string) bool {
	return strings.HasPrefix(path, "/i/") || strings.HasPrefix(path, "/e/")
}

func safeLogPath(r *http.Request) string {
	if r.Pattern != "" && r.Pattern != "/" {
		return r.Pattern
	}
	if strings.HasPrefix(r.URL.Path, "/i/") {
		return redactedViewerPath(r.URL.Path, "/i")
	}
	// Keep redacting pre-rename viewer URLs; they remain compatibility aliases
	// for already shared token-bearing links.
	if strings.HasPrefix(r.URL.Path, "/e/") {
		return redactedViewerPath(r.URL.Path, "/e")
	}
	if r.Pattern != "" {
		return r.Pattern
	}
	return r.URL.Path
}

func redactedViewerPath(path, prefix string) string {
	if strings.HasSuffix(path, "/data") {
		return prefix + "/{token}/data"
	}
	if strings.HasSuffix(path, "/incident/download") {
		return prefix + "/{token}/incident/download"
	}
	if strings.Contains(path, "/streams/") && strings.HasSuffix(path, "/download") {
		return prefix + "/{token}/streams/{stream_id}/download"
	}
	return prefix + "/{token}"
}

func (a *API) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				a.logRecoveredPanic(recovered)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
