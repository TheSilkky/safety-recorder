package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/open-proofline/server/internal/auth"
)

func (a *API) withPrivateAuth(next http.HandlerFunc) http.HandlerFunc {
	authenticated := a.requirePrivateAuth(http.HandlerFunc(next))
	return func(w http.ResponseWriter, r *http.Request) {
		authenticated.ServeHTTP(w, r)
	}
}

func (a *API) requirePrivateAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawToken, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
			return
		}
		session, err := a.repo.LookupSession(r.Context(), rawToken)
		if errors.Is(err, auth.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
			return
		}
		if err != nil {
			a.internalError(w, "lookup auth session", err)
			return
		}
		account, err := a.repo.GetAccountByID(r.Context(), session.AccountID)
		if errors.Is(err, auth.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
			return
		}
		if err != nil {
			a.internalError(w, "get auth account", err)
			return
		}

		next.ServeHTTP(w, r.WithContext(contextWithPrincipal(r.Context(), privatePrincipal{
			Account: account,
			Session: session,
		})))
	})
}

func bearerToken(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(value, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(value, prefix))
	if token == "" || strings.ContainsAny(token, " \t\r\n") {
		return "", false
	}
	return token, true
}
