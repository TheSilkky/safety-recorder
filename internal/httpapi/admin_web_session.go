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

func (a *API) parseAdminWebForm(w http.ResponseWriter, r *http.Request, data adminWebData) bool {
	r.Body = http.MaxBytesReader(w, r.Body, fieldLimit)
	defer r.Body.Close()
	if err := r.ParseForm(); err != nil {
		a.renderAdminWeb(w, http.StatusBadRequest, data)
		return false
	}
	return true
}

func (a *API) parseAdminWebDashboardForm(w http.ResponseWriter, r *http.Request, principal privatePrincipal, message string) bool {
	return a.parseAdminWebPageForm(w, r, principal, adminWebPageOverview, message)
}

func (a *API) parseAdminWebPageForm(w http.ResponseWriter, r *http.Request, principal privatePrincipal, page, message string) bool {
	r.Body = http.MaxBytesReader(w, r.Body, fieldLimit)
	defer r.Body.Close()
	if err := r.ParseForm(); err != nil {
		a.renderAdminWebPage(w, r, principal, page, http.StatusBadRequest, "", message)
		return false
	}
	return true
}

func (a *API) parseAdminWebSessionForm(w http.ResponseWriter, r *http.Request, principal privatePrincipal, message string) bool {
	r.Body = http.MaxBytesReader(w, r.Body, fieldLimit)
	defer r.Body.Close()
	if err := r.ParseForm(); err != nil {
		a.renderAdminWebSessionGate(w, r, principal, http.StatusBadRequest, "", message)
		return false
	}
	return true
}

func (a *API) requireAdminWebSession(w http.ResponseWriter, r *http.Request) (privatePrincipal, bool) {
	principal, ok, err := a.adminWebPrincipal(r)
	if err != nil {
		a.adminWebInternalError(w, "load admin web session", err)
		return privatePrincipal{}, false
	}
	if !ok {
		a.renderAdminWeb(w, http.StatusUnauthorized, makeAdminWebLoginData("Admin login is required."))
		return privatePrincipal{}, false
	}
	if principal.Account.Role != auth.RoleAdmin {
		clearAdminWebSessionCookie(w)
		a.renderAdminWeb(w, http.StatusForbidden, makeAdminWebForbiddenData())
		return privatePrincipal{}, false
	}
	return principal, true
}

func (a *API) requireAdminWeb(w http.ResponseWriter, r *http.Request) (privatePrincipal, bool) {
	principal, ok := a.requireAdminWebSession(w, r)
	if !ok {
		return privatePrincipal{}, false
	}
	if !a.adminWebSecondFactorSatisfied(w, r, principal, "", "") {
		return privatePrincipal{}, false
	}
	return principal, true
}

func (a *API) adminWebSecondFactorSatisfied(w http.ResponseWriter, r *http.Request, principal privatePrincipal, notice, message string) bool {
	if adminRequiresSecondFactorSetup(principal.Account) {
		a.renderAdminWebSecondFactorSetup(w, r, principal, http.StatusForbidden, notice, message)
		return false
	}
	required, err := a.sessionRequiresSecondFactorVerification(r.Context(), principal.Account, principal.Session)
	if err != nil {
		a.adminWebInternalError(w, "check admin web session second factor requirement", err)
		return false
	}
	if required {
		a.renderAdminWebSecondFactorVerification(w, r, principal, http.StatusForbidden, notice, message)
		return false
	}
	return true
}

func (a *API) renderAdminWebSessionGate(w http.ResponseWriter, r *http.Request, principal privatePrincipal, status int, notice, message string) {
	if !a.adminWebSecondFactorSatisfied(w, r, principal, notice, message) {
		return
	}
	a.renderAdminWebDashboard(w, r, principal, status, notice, message)
}

func (a *API) createAdminWebBootstrapAccount(r *http.Request) (auth.Account, int, string, error, bool) {
	username := auth.NormalizeUsername(r.FormValue("username"))
	if err := auth.ValidateUsername(username); err != nil {
		return auth.Account{}, http.StatusBadRequest, err.Error(), nil, false
	}
	passwordHash, err := auth.HashPassword(r.FormValue("password"), a.passwordCost)
	if err != nil {
		return auth.Account{}, http.StatusBadRequest, err.Error(), nil, false
	}
	account, err := a.repo.CreateAccount(r.Context(), auth.CreateAccountParams{
		Username:          username,
		SecondFactorSetup: auth.SecondFactorSetupStateSetupRequired,
		PasswordHash:      passwordHash,
		Role:              auth.RoleAdmin,
	})
	if errors.Is(err, auth.ErrDuplicate) {
		return auth.Account{}, http.StatusConflict, "Username is already in use.", nil, false
	}
	if err != nil {
		return auth.Account{}, 0, "", err, false
	}
	return account, http.StatusCreated, "", nil, true
}

func (a *API) adminWebCreateManagedAccount(r *http.Request, username, password, role string) (auth.Account, int, string, error, bool) {
	username = auth.NormalizeUsername(username)
	role = strings.TrimSpace(role)
	if err := auth.ValidateUsername(username); err != nil {
		return auth.Account{}, http.StatusBadRequest, err.Error(), nil, false
	}
	if !auth.ValidRole(role) {
		return auth.Account{}, http.StatusBadRequest, "Role must be user or admin.", nil, false
	}
	passwordHash, err := auth.HashPassword(password, a.passwordCost)
	if err != nil {
		return auth.Account{}, http.StatusBadRequest, err.Error(), nil, false
	}
	account, err := a.repo.CreateAccount(r.Context(), auth.CreateAccountParams{
		Username:          username,
		SecondFactorSetup: auth.SecondFactorSetupStateSetupRequired,
		PasswordHash:      passwordHash,
		Role:              role,
	})
	if errors.Is(err, auth.ErrDuplicate) {
		return auth.Account{}, http.StatusConflict, "Username is already in use.", nil, false
	}
	if err != nil {
		return auth.Account{}, 0, "", err, false
	}
	return account, http.StatusCreated, "", nil, true
}

func (a *API) adminWebUpdatePassword(r *http.Request, accountID, password string) (auth.Account, int, string, error, bool) {
	passwordHash, err := auth.HashPassword(password, a.passwordCost)
	if err != nil {
		return auth.Account{}, http.StatusBadRequest, err.Error(), nil, false
	}
	account, err := a.repo.UpdateAccountPassword(r.Context(), accountID, passwordHash)
	if errors.Is(err, auth.ErrNotFound) {
		return auth.Account{}, http.StatusNotFound, "Account was not found.", nil, false
	}
	if err != nil {
		return auth.Account{}, 0, "", err, false
	}
	return account, http.StatusOK, "", nil, true
}

func (a *API) issueAdminWebSession(w http.ResponseWriter, r *http.Request, accountID string) bool {
	session, rawToken, err := a.repo.CreateSession(r.Context(), accountID, time.Now().UTC().Add(a.sessionTTL))
	if err != nil {
		a.adminWebInternalError(w, "create admin web session", err)
		return false
	}
	http.SetCookie(w, adminWebSessionCookie(r, rawToken, session.ExpiresAt))
	return true
}

func (a *API) adminWebPrincipal(r *http.Request) (privatePrincipal, bool, error) {
	cookie, err := r.Cookie(adminWebSessionCookieName)
	if errors.Is(err, http.ErrNoCookie) {
		return privatePrincipal{}, false, nil
	}
	if err != nil {
		return privatePrincipal{}, false, err
	}
	rawToken := strings.TrimSpace(cookie.Value)
	if rawToken == "" {
		return privatePrincipal{}, false, nil
	}
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
	if !auth.CanAuthenticate(account) {
		return privatePrincipal{}, false, nil
	}
	return privatePrincipal{Account: account, Session: session}, true, nil
}

func adminWebSessionCookie(r *http.Request, rawToken string, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     adminWebSessionCookieName,
		Value:    rawToken,
		Path:     "/admin",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   r.TLS != nil,
	}
}

func clearAdminWebSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     adminWebSessionCookieName,
		Value:    "",
		Path:     "/admin",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func (a *API) validateAdminWebCSRF(w http.ResponseWriter, r *http.Request, principal privatePrincipal) bool {
	if !adminWebCSRFTokenFromFormValid(r) {
		a.renderAdminWebDashboard(w, r, principal, http.StatusForbidden, "", "The form expired. Reload the page and try again.")
		return false
	}
	return true
}

func (a *API) validateAdminWebSessionCSRF(w http.ResponseWriter, r *http.Request, principal privatePrincipal) bool {
	if !adminWebCSRFTokenFromFormValid(r) {
		a.renderAdminWebSessionGate(w, r, principal, http.StatusForbidden, "", "The form expired. Reload the page and try again.")
		return false
	}
	return true
}

func (a *API) validateAdminWebCSRFForData(w http.ResponseWriter, r *http.Request, data adminWebData) bool {
	if !adminWebCSRFTokenFromFormValid(r) {
		data.Error = "The form expired. Reload the page and try again."
		a.renderAdminWeb(w, http.StatusForbidden, data)
		return false
	}
	return true
}

func adminWebCSRFTokenFromFormValid(r *http.Request) bool {
	want := adminWebCSRFTokenFromRequest(r)
	got := strings.TrimSpace(r.FormValue("csrf_token"))
	return want != "" && got != "" && adminWebCSRFTokenValid(want, got)
}

func adminWebCSRFTokenFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(adminWebSessionCookieName)
	if err != nil {
		return ""
	}
	return adminWebCSRFToken(strings.TrimSpace(cookie.Value))
}

func adminWebCSRFToken(rawSessionToken string) string {
	if rawSessionToken == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(rawSessionToken))
	mac.Write([]byte("proofline-admin-web-csrf-v1"))
	return hex.EncodeToString(mac.Sum(nil))
}

func adminWebCSRFTokenValid(want, got string) bool {
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
