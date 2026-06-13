package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const (
	contentSecurityPolicy = "default-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'; object-src 'none'"
	permissionsPolicy     = "geolocation=(), microphone=(), camera=()"
)

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	// Non-upload JSON bodies are intentionally small metadata requests.
	r.Body = http.MaxBytesReader(w, r.Body, jsonBodyLimit)
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		if isMaxBytesError(err) {
			writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "JSON request body is too large")
			return false
		}
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must contain a single JSON object")
		return false
	}
	return true
}

func (a *API) internalError(w http.ResponseWriter, operation string, err error) {
	a.logInternalError(operation, err)
	writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

func isMaxBytesError(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
}

func setNoSniff(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func setNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

func setPublicBrowserSecurityHeaders(w http.ResponseWriter) {
	setNoSniff(w)
	w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
	w.Header().Set("Permissions-Policy", permissionsPolicy)
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
}
