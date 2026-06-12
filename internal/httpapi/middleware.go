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
			"component", "httpapi",
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
	if strings.HasPrefix(r.URL.Path, "/admin/api/") {
		return redactedAdminAPIPath(r.Method, strings.Split(strings.Trim(r.URL.Path, "/"), "/"))
	}
	if strings.HasPrefix(r.URL.Path, "/v1/") {
		return redactedMainAPIPath(r.Method, r.URL.Path)
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
	if strings.HasSuffix(path, "/viewer-payload") {
		return prefix + "/{token}/viewer-payload"
	}
	if strings.HasSuffix(path, "/incident/download") {
		return prefix + "/{token}/incident/download"
	}
	if strings.Contains(path, "/streams/") && strings.HasSuffix(path, "/download") {
		return prefix + "/{token}/streams/{stream_id}/download"
	}
	return prefix + "/{token}"
}

func redactedMainAPIPath(method, rawPath string) string {
	segments := strings.Split(strings.Trim(rawPath, "/"), "/")
	if len(segments) < 2 || segments[0] != "v1" {
		return method + " /v1/{route}"
	}

	switch segments[1] {
	case "auth":
		return redactedAuthPath(method, segments)
	case "bootstrap":
		if len(segments) == 3 && segments[2] == "admin" {
			return method + " /v1/bootstrap/admin"
		}
	case "account":
		if len(segments) == 2 {
			return method + " /v1/account"
		}
		if len(segments) == 3 && segments[2] == "password" {
			return method + " /v1/account/password"
		}
	case "account-recipient-keys":
		return redactedRecordPath(method, segments, "account-recipient-keys", "recipient_key_id")
	case "contact-public-keys":
		return redactedRecordPath(method, segments, "contact-public-keys", "public_key_id")
	case "incidents":
		return redactedIncidentPath(method, segments)
	case "incident-tokens":
		if len(segments) == 4 && segments[3] == "revoke" {
			return method + " /v1/incident-tokens/{token_id}/revoke"
		}
	case "sharing-grants":
		return redactedRecordPath(method, segments, "sharing-grants", "grant_id")
	case "wrapped-keys":
		return redactedRecordPath(method, segments, "wrapped-keys", "wrapped_key_id")
	}
	return method + " /v1/{route}"
}

func redactedAdminAPIPath(method string, segments []string) string {
	if len(segments) < 3 {
		return method + " /admin/api/{route}"
	}
	switch segments[2] {
	case "accounts":
		if len(segments) == 3 {
			return method + " /admin/api/accounts"
		}
		if len(segments) == 5 && segments[4] == "password" {
			return method + " /admin/api/accounts/{account_id}/password"
		}
		if len(segments) == 7 && segments[4] == "second-factor" && segments[5] == "recovery" && segments[6] == "reset" {
			return method + " /admin/api/accounts/{account_id}/second-factor/recovery/reset"
		}
		if len(segments) == 6 && segments[4] == "sessions" && segments[5] == "revoke" {
			return method + " /admin/api/accounts/{account_id}/sessions/revoke"
		}
	case "incidents":
		if len(segments) == 4 && segments[3] == "unowned" {
			return method + " /admin/api/incidents/unowned"
		}
		if len(segments) == 5 {
			switch segments[4] {
			case "deletion", "reassignment":
				return method + " /admin/api/incidents/{incident_id}/" + segments[4]
			}
		}
	}
	return method + " /admin/api/{route}"
}

func redactedAuthPath(method string, segments []string) string {
	if len(segments) == 3 {
		switch segments[2] {
		case "login", "logout", "register":
			return method + " /v1/auth/" + segments[2]
		}
	}
	if len(segments) == 4 && segments[2] == "email" && segments[3] == "verify" {
		return method + " /v1/auth/email/verify"
	}
	if len(segments) == 4 && segments[2] == "web" {
		switch segments[3] {
		case "login", "logout", "csrf":
			return method + " /v1/auth/web/" + segments[3]
		}
	}
	return method + " /v1/auth/{route}"
}

func redactedRecordPath(method string, segments []string, base, idName string) string {
	if len(segments) == 2 {
		return method + " /v1/" + base
	}
	if len(segments) == 3 {
		return method + " /v1/" + base + "/{" + idName + "}"
	}
	if len(segments) == 4 {
		switch segments[3] {
		case "revoke", "lost", "replace":
			return method + " /v1/" + base + "/{" + idName + "}/" + segments[3]
		}
	}
	return method + " /v1/" + base + "/{route}"
}

func redactedIncidentPath(method string, segments []string) string {
	if len(segments) == 2 {
		return method + " /v1/incidents"
	}
	if len(segments) == 3 {
		return method + " /v1/incidents/{incident_id}"
	}
	if len(segments) < 4 {
		return method + " /v1/incidents/{route}"
	}

	switch segments[3] {
	case "chunks":
		return redactedChunkPath(method, segments)
	case "streams":
		return redactedStreamPath(method, segments)
	case "deletion", "download", "checkins", "close", "sharing-grants", "incident-tokens", "wrapped-keys":
		if len(segments) == 4 {
			return method + " /v1/incidents/{incident_id}/" + segments[3]
		}
		if len(segments) == 5 && segments[3] == "incident-tokens" {
			return method + " /v1/incidents/{incident_id}/incident-tokens/{token_id}"
		}
	}
	return method + " /v1/incidents/{incident_id}/{route}"
}

func redactedChunkPath(method string, segments []string) string {
	if len(segments) == 4 {
		return method + " /v1/incidents/{incident_id}/chunks"
	}
	if len(segments) == 5 && segments[4] == "reconcile" {
		return method + " /v1/incidents/{incident_id}/chunks/reconcile"
	}
	if len(segments) == 6 {
		return method + " /v1/incidents/{incident_id}/chunks/{media_type}/{chunk_index}"
	}
	return method + " /v1/incidents/{incident_id}/chunks/{route}"
}

func redactedStreamPath(method string, segments []string) string {
	if len(segments) == 4 {
		return method + " /v1/incidents/{incident_id}/streams"
	}
	if len(segments) == 5 {
		return method + " /v1/incidents/{incident_id}/streams/{stream_id}"
	}
	if len(segments) == 6 {
		switch segments[5] {
		case "complete", "fail", "download":
			return method + " /v1/incidents/{incident_id}/streams/{stream_id}/" + segments[5]
		}
	}
	return method + " /v1/incidents/{incident_id}/streams/{route}"
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
