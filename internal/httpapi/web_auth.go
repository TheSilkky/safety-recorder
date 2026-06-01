package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/open-proofline/server/internal/auth"
)

type webAuthSessionResponse struct {
	SessionID string          `json:"session_id"`
	Account   accountResponse `json:"account"`
	CreatedAt time.Time       `json:"created_at"`
	ExpiresAt time.Time       `json:"expires_at"`
}

func (a *API) webLogin(w http.ResponseWriter, r *http.Request) {
	if !a.requireWebAuthEnabled(w) {
		return
	}
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}

	account, err := a.repo.GetAccountByUsername(r.Context(), auth.NormalizeUsername(request.Username))
	if errors.Is(err, auth.ErrNotFound) {
		auth.SpendPasswordHashCost(request.Password, a.passwordCost)
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "username or password is invalid")
		return
	}
	if err != nil {
		a.internalError(w, "get web login account", err)
		return
	}
	if !auth.VerifyPassword(account.PasswordHash, request.Password) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "username or password is invalid")
		return
	}

	session, rawToken, err := a.repo.CreateSession(r.Context(), account.ID, time.Now().UTC().Add(a.sessionTTL))
	if err != nil {
		a.internalError(w, "create web auth session", err)
		return
	}
	http.SetCookie(w, a.webSessionCookie(rawToken, session.ExpiresAt))
	writeJSON(w, http.StatusCreated, webAuthSessionResponse{
		SessionID: session.ID,
		Account:   makeAccountResponse(account),
		CreatedAt: session.CreatedAt,
		ExpiresAt: session.ExpiresAt,
	})
}

func (a *API) webLogout(w http.ResponseWriter, r *http.Request) {
	if !a.requireWebAuthEnabled(w) {
		return
	}
	_, hasBearerToken := bearerToken(r.Header.Get("Authorization"))
	rawToken, ok := a.webSessionCookieToken(r)
	if hasBearerToken && ok {
		writeError(w, http.StatusBadRequest, "ambiguous_credentials", "send either bearer authorization or browser session cookie, not both")
		return
	}
	if hasBearerToken {
		writeError(w, http.StatusBadRequest, "web_cookie_required", "browser session cookie authentication is required")
		return
	}
	if !ok {
		a.clearWebSessionCookie(w)
		writeJSON(w, http.StatusOK, map[string]bool{"revoked": false})
		return
	}
	principal, valid, err := a.webCookiePrincipal(r, rawToken)
	if err != nil {
		a.internalError(w, "lookup web auth session", err)
		return
	}
	if !valid {
		a.clearWebSessionCookie(w)
		writeJSON(w, http.StatusOK, map[string]bool{"revoked": false})
		return
	}
	if !a.validateWebCSRF(w, r, principal) {
		return
	}
	if err := a.repo.RevokeSession(r.Context(), principal.Session.ID); err != nil && !errors.Is(err, auth.ErrNotFound) {
		a.internalError(w, "revoke web auth session", err)
		return
	}
	a.clearWebSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"revoked": true})
}

func (a *API) webCookiePrincipal(r *http.Request, rawToken string) (privatePrincipal, bool, error) {
	session, err := a.repo.LookupSession(r.Context(), rawToken)
	if errors.Is(err, auth.ErrNotFound) {
		return privatePrincipal{}, false, nil
	}
	if err != nil {
		return privatePrincipal{}, false, err
	}
	account, err := a.repo.GetAccountByID(r.Context(), session.AccountID)
	if errors.Is(err, auth.ErrNotFound) {
		return privatePrincipal{}, false, nil
	}
	if err != nil {
		return privatePrincipal{}, false, err
	}
	return privatePrincipal{Account: account, Session: session, AuthSource: privateAuthSourceWebCookie}, true, nil
}

func (a *API) webCSRF(w http.ResponseWriter, r *http.Request) {
	if !a.requireWebAuthEnabled(w) {
		return
	}
	principal, ok := principalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication_required", "authentication is required")
		return
	}
	if principal.AuthSource != privateAuthSourceWebCookie {
		writeError(w, http.StatusBadRequest, "web_cookie_required", "browser session cookie authentication is required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"csrf_token":  webCSRFToken(principal.Session),
		"header_name": a.webAuth.CSRFHeaderName,
	})
}

func (a *API) requireWebAuthEnabled(w http.ResponseWriter) bool {
	if a.webAuth.Enabled {
		return true
	}
	writeError(w, http.StatusNotFound, "not_found", "endpoint was not found")
	return false
}

func (a *API) webSessionCookieToken(r *http.Request) (string, bool) {
	if !a.webAuth.Enabled {
		return "", false
	}
	cookie, err := r.Cookie(a.webAuth.SessionCookieName)
	if err != nil {
		return "", false
	}
	rawToken := strings.TrimSpace(cookie.Value)
	if rawToken == "" {
		return "", false
	}
	return rawToken, true
}

func (a *API) webSessionCookie(rawToken string, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     a.webAuth.SessionCookieName,
		Value:    rawToken,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: a.webAuth.SessionCookieSameSite,
		Secure:   a.webAuth.SessionCookieSecure,
	}
}

func (a *API) clearWebSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     a.webAuth.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: a.webAuth.SessionCookieSameSite,
		Secure:   a.webAuth.SessionCookieSecure,
	})
}

func (a *API) validateWebCSRF(w http.ResponseWriter, r *http.Request, principal privatePrincipal) bool {
	want := webCSRFToken(principal.Session)
	got := strings.TrimSpace(r.Header.Get(a.webAuth.CSRFHeaderName))
	if want == "" || got == "" || !webCSRFTokenValid(want, got) {
		writeError(w, http.StatusForbidden, "csrf_required", "CSRF token is required for browser cookie requests")
		return false
	}
	return true
}

func webCSRFToken(session auth.Session) string {
	if session.TokenHash == "" || session.ID == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(session.TokenHash))
	mac.Write([]byte("proofline-web-csrf-v1"))
	mac.Write([]byte{0})
	mac.Write([]byte(session.ID))
	mac.Write([]byte{0})
	mac.Write([]byte(session.AccountID))
	mac.Write([]byte{0})
	mac.Write([]byte(session.ExpiresAt.UTC().Format(time.RFC3339Nano)))
	return hex.EncodeToString(mac.Sum(nil))
}

func webCSRFTokenValid(want, got string) bool {
	wantBytes, err := hex.DecodeString(want)
	if err != nil {
		return false
	}
	gotBytes, err := hex.DecodeString(got)
	if err != nil {
		return false
	}
	return hmac.Equal(wantBytes, gotBytes)
}
