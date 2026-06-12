package httpapi

import (
	"net/http"
	"strings"
)

const webCORSAllowedMethods = "GET, POST, PATCH"

func (a *API) webCORSMiddleware(next http.Handler) http.Handler {
	if !a.webAuth.Enabled || len(a.webAuth.AllowedOrigins) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		addVaryHeader(w, "Origin")

		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			addVaryHeader(w, "Access-Control-Request-Method")
			addVaryHeader(w, "Access-Control-Request-Headers")
			if a.webOriginAllowed(origin) && a.webPreflightAllowed(r) {
				a.setWebCORSHeaders(w, origin)
				w.Header().Set("Access-Control-Allow-Methods", webCORSAllowedMethods)
				w.Header().Set("Access-Control-Allow-Headers", a.webAllowedHeaders())
				w.Header().Set("Access-Control-Max-Age", "600")
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if a.webOriginAllowed(origin) {
			a.setWebCORSHeaders(w, origin)
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) setWebCORSHeaders(w http.ResponseWriter, origin string) {
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
}

func (a *API) webOriginAllowed(origin string) bool {
	for _, allowed := range a.webAuth.AllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

func (a *API) webPreflightAllowed(r *http.Request) bool {
	switch r.Header.Get("Access-Control-Request-Method") {
	case http.MethodGet, http.MethodPost, http.MethodPatch:
	default:
		return false
	}
	return a.webRequestedHeadersAllowed(r.Header.Get("Access-Control-Request-Headers"))
}

func (a *API) webRequestedHeadersAllowed(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return true
	}
	allowed := map[string]bool{
		"accept":          true,
		"authorization":   true,
		"content-type":    true,
		"idempotency-key": true,
		strings.ToLower(a.webAuth.CSRFHeaderName): true,
	}
	for _, part := range strings.Split(raw, ",") {
		if !allowed[strings.ToLower(strings.TrimSpace(part))] {
			return false
		}
	}
	return true
}

func (a *API) webAllowedHeaders() string {
	return "Accept, Authorization, Content-Type, Idempotency-Key, " + http.CanonicalHeaderKey(a.webAuth.CSRFHeaderName)
}

func addVaryHeader(w http.ResponseWriter, value string) {
	current := w.Header().Values("Vary")
	for _, header := range current {
		for _, part := range strings.Split(header, ",") {
			if strings.EqualFold(strings.TrimSpace(part), value) {
				return
			}
		}
	}
	w.Header().Add("Vary", value)
}
