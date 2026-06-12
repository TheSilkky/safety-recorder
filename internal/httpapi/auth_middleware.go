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
		principal, ok := a.authenticatePrivateRequest(w, r)
		if !ok {
			return
		}
		if principal.AuthSource == privateAuthSourceWebCookie && unsafeMethod(r.Method) && !a.validateWebCSRF(w, r, principal) {
			return
		}
		if !auth.CanAccessProductRoutes(principal.Account) && !secondFactorSetupAllowedRoute(r) {
			writeError(w, http.StatusForbidden, "second_factor_setup_required", "second factor setup is required before account access")
			return
		}
		if auth.CanAccessProductRoutes(principal.Account) {
			required, err := a.sessionRequiresSecondFactorVerification(r.Context(), principal.Account, principal.Session)
			if err != nil {
				a.internalError(w, "check session second factor requirement", err)
				return
			}
			if required && !secondFactorVerificationAllowedRoute(r) {
				writeError(w, http.StatusForbidden, "second_factor_verification_required", "second factor verification is required before account access")
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(contextWithPrincipal(r.Context(), principal)))
	})
}

func (a *API) authenticatePrivateRequest(w http.ResponseWriter, r *http.Request) (privatePrincipal, bool) {
	rawBearerToken, hasBearerToken := bearerToken(r.Header.Get("Authorization"))
	rawCookieToken, hasCookieToken := a.webSessionCookieToken(r)
	if hasBearerToken && hasCookieToken {
		writeError(w, http.StatusBadRequest, "ambiguous_credentials", "send either bearer authorization or browser session cookie, not both")
		return privatePrincipal{}, false
	}
	if hasBearerToken {
		return a.lookupPrivatePrincipal(w, r, rawBearerToken, privateAuthSourceBearer)
	}
	if hasCookieToken {
		return a.lookupPrivatePrincipal(w, r, rawCookieToken, privateAuthSourceWebCookie)
	}
	writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
	return privatePrincipal{}, false
}

func (a *API) lookupPrivatePrincipal(w http.ResponseWriter, r *http.Request, rawToken string, source privateAuthSource) (privatePrincipal, bool) {
	session, err := a.repo.LookupSession(r.Context(), rawToken)
	if errors.Is(err, auth.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return privatePrincipal{}, false
	}
	if err != nil {
		a.internalError(w, "lookup auth session", err)
		return privatePrincipal{}, false
	}
	account, err := a.repo.GetAccountByID(r.Context(), session.AccountID)
	if errors.Is(err, auth.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return privatePrincipal{}, false
	}
	if err != nil {
		a.internalError(w, "get auth account", err)
		return privatePrincipal{}, false
	}
	if !auth.CanAuthenticate(account) {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return privatePrincipal{}, false
	}
	return privatePrincipal{Account: account, Session: session, AuthSource: source}, true
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

func unsafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

func secondFactorSetupAllowedRoute(r *http.Request) bool {
	path := strings.Trim(r.URL.EscapedPath(), "/")
	if strings.HasPrefix(path, "admin/api/") {
		return true
	}
	if strings.HasPrefix(path, "v1/account/second-factor") {
		return true
	}
	switch {
	case r.Method == http.MethodGet && path == "v1/account":
		return true
	case r.Method == http.MethodPost && path == "v1/auth/logout":
		return true
	case r.Method == http.MethodGet && path == "v1/auth/web/csrf":
		return true
	default:
		return false
	}
}

func secondFactorVerificationAllowedRoute(r *http.Request) bool {
	path := strings.Trim(r.URL.EscapedPath(), "/")
	switch {
	case r.Method == http.MethodGet && path == "v1/account":
		return true
	case r.Method == http.MethodPost && path == "v1/auth/logout":
		return true
	case r.Method == http.MethodGet && path == "v1/auth/web/csrf":
		return true
	case r.Method == http.MethodPost && path == "v1/account/second-factor/totp/verify":
		return true
	case r.Method == http.MethodPost && path == "v1/account/second-factor/webauthn/verify/start":
		return true
	case r.Method == http.MethodPost && path == "v1/account/second-factor/webauthn/verify/finish":
		return true
	default:
		return false
	}
}
