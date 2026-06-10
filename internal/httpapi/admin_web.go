package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/open-proofline/server/internal/auth"
)

const adminWebSessionCookieName = "proofline_admin_session"

//go:embed web/templates/admin.html web/admin/static/styles.css
var adminWebFS embed.FS

var adminWebTemplate = template.Must(template.New("admin.html").Funcs(template.FuncMap{
	"humanTime": humanTime,
}).ParseFS(adminWebFS, "web/templates/admin.html"))

type adminWebData struct {
	Title       string
	Mode        string
	Error       string
	Notice      string
	CSRFToken   string
	Account     adminWebAccount
	Accounts    []adminWebAccount
	NavItems    []adminWebNavItem
	StatusItems []adminWebStatusItem
}

type adminWebAccount struct {
	ID                string
	Username          string
	Role              string
	CreatedAt         time.Time
	PasswordChangedAt time.Time
	IsCurrent         bool
}

type adminWebNavItem struct {
	Label string
	State string
}

type adminWebStatusItem struct {
	Label string
	Value string
	Tone  string
}

func (a *API) adminWebPage(w http.ResponseWriter, r *http.Request) {
	setAdminWebPageHeaders(w)

	hasAdmin, err := a.repo.HasAdminAccount(r.Context())
	if err != nil {
		a.adminWebInternalError(w, "check admin account for admin web", err)
		return
	}
	if !hasAdmin {
		status := http.StatusOK
		data := makeAdminWebBootstrapData("")
		if a.bootstrapSecret == "" {
			status = http.StatusForbidden
			data.Error = "Bootstrap is not enabled."
		}
		a.renderAdminWeb(w, status, data)
		return
	}

	principal, ok, err := a.adminWebPrincipal(r)
	if err != nil {
		a.adminWebInternalError(w, "load admin web session", err)
		return
	}
	if !ok {
		a.renderAdminWeb(w, http.StatusOK, makeAdminWebLoginData(""))
		return
	}
	if principal.Account.Role != auth.RoleAdmin {
		clearAdminWebSessionCookie(w)
		a.renderAdminWeb(w, http.StatusForbidden, makeAdminWebForbiddenData())
		return
	}

	a.renderAdminWebDashboard(w, r, principal, http.StatusOK, adminWebNotice(r), "")
}

func (a *API) adminWebLogin(w http.ResponseWriter, r *http.Request) {
	setAdminWebPageHeaders(w)
	if ok := a.parseAdminWebForm(w, r, makeAdminWebLoginData("The login form could not be read.")); !ok {
		return
	}

	account, err := a.repo.GetAccountByUsername(r.Context(), auth.NormalizeUsername(r.FormValue("username")))
	if errors.Is(err, auth.ErrNotFound) {
		auth.SpendPasswordHashCost(r.FormValue("password"), a.passwordCost)
		a.renderAdminWeb(w, http.StatusUnauthorized, makeAdminWebLoginData("Username or password is invalid."))
		return
	}
	if err != nil {
		a.adminWebInternalError(w, "get admin web login account", err)
		return
	}
	if !auth.VerifyPassword(account.PasswordHash, r.FormValue("password")) {
		a.renderAdminWeb(w, http.StatusUnauthorized, makeAdminWebLoginData("Username or password is invalid."))
		return
	}
	if !auth.CanAuthenticate(account) {
		a.renderAdminWeb(w, http.StatusUnauthorized, makeAdminWebLoginData("Username or password is invalid."))
		return
	}
	if account.Role != auth.RoleAdmin {
		a.renderAdminWeb(w, http.StatusForbidden, makeAdminWebLoginData("Admin role is required."))
		return
	}

	if !a.issueAdminWebSession(w, r, account.ID) {
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (a *API) adminWebBootstrap(w http.ResponseWriter, r *http.Request) {
	setAdminWebPageHeaders(w)

	hasAdmin, err := a.repo.HasAdminAccount(r.Context())
	if err != nil {
		a.adminWebInternalError(w, "check admin account for admin web bootstrap", err)
		return
	}
	if hasAdmin {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	if a.bootstrapSecret == "" {
		a.renderAdminWeb(w, http.StatusForbidden, makeAdminWebBootstrapData("Bootstrap is not enabled."))
		return
	}
	if ok := a.parseAdminWebForm(w, r, makeAdminWebBootstrapData("The bootstrap form could not be read.")); !ok {
		return
	}
	if !sameSecret(a.bootstrapSecret, r.FormValue("bootstrap_secret")) {
		a.renderAdminWeb(w, http.StatusUnauthorized, makeAdminWebBootstrapData("Bootstrap secret is invalid."))
		return
	}

	account, status, message, createErr, ok := a.createAdminWebBootstrapAccount(r)
	if createErr != nil {
		a.adminWebInternalError(w, "create admin web bootstrap account", createErr)
		return
	}
	if !ok {
		a.renderAdminWeb(w, status, makeAdminWebBootstrapData(message))
		return
	}
	if !a.issueAdminWebSession(w, r, account.ID) {
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (a *API) adminWebLogout(w http.ResponseWriter, r *http.Request) {
	setAdminWebPageHeaders(w)

	principal, ok, err := a.adminWebPrincipal(r)
	if err != nil {
		a.adminWebInternalError(w, "load admin web logout session", err)
		return
	}
	if !ok {
		clearAdminWebSessionCookie(w)
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	if principal.Account.Role != auth.RoleAdmin {
		clearAdminWebSessionCookie(w)
		a.renderAdminWeb(w, http.StatusForbidden, makeAdminWebForbiddenData())
		return
	}
	if ok := a.parseAdminWebDashboardForm(w, r, principal, "The logout form could not be read."); !ok {
		return
	}
	if !a.validateAdminWebCSRF(w, r, principal) {
		return
	}
	if err := a.repo.RevokeSession(r.Context(), principal.Session.ID); err != nil && !errors.Is(err, auth.ErrNotFound) {
		a.adminWebInternalError(w, "revoke admin web session", err)
		return
	}

	clearAdminWebSessionCookie(w)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (a *API) adminWebChangeOwnPassword(w http.ResponseWriter, r *http.Request) {
	setAdminWebPageHeaders(w)
	principal, ok := a.requireAdminWeb(w, r)
	if !ok {
		return
	}
	if ok := a.parseAdminWebDashboardForm(w, r, principal, "The password form could not be read."); !ok {
		return
	}
	if !a.validateAdminWebCSRF(w, r, principal) {
		return
	}
	if !auth.VerifyPassword(principal.Account.PasswordHash, r.FormValue("current_password")) {
		a.renderAdminWebDashboard(w, r, principal, http.StatusUnauthorized, "", "Current password is invalid.")
		return
	}
	account, status, message, err, ok := a.adminWebUpdatePassword(r, principal.Account.ID, r.FormValue("new_password"))
	if err != nil {
		a.adminWebInternalError(w, "change admin web own password", err)
		return
	}
	if !ok {
		a.renderAdminWebDashboard(w, r, principal, status, "", message)
		return
	}
	if _, err := a.repo.RevokeAccountSessions(r.Context(), account.ID, principal.Session.ID); err != nil {
		a.adminWebInternalError(w, "revoke admin web own sessions", err)
		return
	}
	http.Redirect(w, r, "/admin?notice=password_changed", http.StatusSeeOther)
}

func (a *API) adminWebResetAccountPassword(w http.ResponseWriter, r *http.Request) {
	setAdminWebPageHeaders(w)
	principal, ok := a.requireAdminWeb(w, r)
	if !ok {
		return
	}
	if ok := a.parseAdminWebDashboardForm(w, r, principal, "The password reset form could not be read."); !ok {
		return
	}
	if !a.validateAdminWebCSRF(w, r, principal) {
		return
	}

	accountID := r.PathValue("account_id")
	if accountID == principal.Account.ID {
		a.renderAdminWebDashboard(w, r, principal, http.StatusBadRequest, "", "Use the admin password form to change your own password.")
		return
	}
	account, status, message, err, ok := a.adminWebUpdatePassword(r, accountID, r.FormValue("new_password"))
	if err != nil {
		a.adminWebInternalError(w, "reset admin web account password", err)
		return
	}
	if !ok {
		a.renderAdminWebDashboard(w, r, principal, status, "", message)
		return
	}
	if _, err := a.repo.RevokeAccountSessions(r.Context(), account.ID, ""); err != nil {
		a.adminWebInternalError(w, "revoke admin web account sessions", err)
		return
	}
	http.Redirect(w, r, "/admin?notice=account_password_reset", http.StatusSeeOther)
}

func (a *API) adminWebStaticHandler() http.Handler {
	staticFiles, err := fs.Sub(adminWebFS, "web/admin/static")
	if err != nil {
		panic(err)
	}
	fileServer := http.StripPrefix("/admin/static/", http.FileServer(http.FS(staticFiles)))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setAdminWebStaticHeaders(w)
		fileServer.ServeHTTP(w, r)
	})
}

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
	r.Body = http.MaxBytesReader(w, r.Body, fieldLimit)
	defer r.Body.Close()
	if err := r.ParseForm(); err != nil {
		a.renderAdminWebDashboard(w, r, principal, http.StatusBadRequest, "", message)
		return false
	}
	return true
}

func (a *API) requireAdminWeb(w http.ResponseWriter, r *http.Request) (privatePrincipal, bool) {
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

func (a *API) renderAdminWeb(w http.ResponseWriter, status int, data adminWebData) {
	if status == 0 {
		status = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := adminWebTemplate.Execute(w, data); err != nil {
		a.logInternalError("render admin web page", err)
	}
}

func (a *API) renderAdminWebDashboard(w http.ResponseWriter, r *http.Request, principal privatePrincipal, status int, notice, message string) {
	accounts, err := a.repo.ListAccounts(r.Context())
	if err != nil {
		a.adminWebInternalError(w, "list admin web accounts", err)
		return
	}
	a.renderAdminWeb(w, status, makeAdminWebDashboardData(principal, accounts, adminWebCSRFTokenFromRequest(r), notice, message))
}

func (a *API) adminWebInternalError(w http.ResponseWriter, operation string, err error) {
	a.logInternalError(operation, err)
	a.renderAdminWeb(w, http.StatusInternalServerError, adminWebData{
		Title: "Proofline Admin",
		Mode:  "error",
		Error: "Internal server error.",
	})
}

func makeAdminWebLoginData(message string) adminWebData {
	return adminWebData{
		Title: "Proofline Admin Login",
		Mode:  "login",
		Error: message,
	}
}

func adminWebNotice(r *http.Request) string {
	switch r.URL.Query().Get("notice") {
	case "password_changed":
		return "Password changed."
	case "account_password_reset":
		return "Account password reset."
	default:
		return ""
	}
}

func makeAdminWebBootstrapData(message string) adminWebData {
	return adminWebData{
		Title: "Proofline Admin Bootstrap",
		Mode:  "bootstrap",
		Error: message,
	}
}

func makeAdminWebForbiddenData() adminWebData {
	return adminWebData{
		Title: "Proofline Admin",
		Mode:  "forbidden",
		Error: "Admin role is required.",
	}
}

func makeAdminWebDashboardData(principal privatePrincipal, accounts []auth.Account, csrfToken, notice, message string) adminWebData {
	return adminWebData{
		Title:     "Proofline Admin",
		Mode:      "dashboard",
		Error:     message,
		Notice:    notice,
		CSRFToken: csrfToken,
		Account:   makeAdminWebAccount(principal.Account, principal.Account.ID),
		Accounts:  makeAdminWebAccounts(accounts, principal.Account.ID),
		NavItems: []adminWebNavItem{
			{Label: "Accounts", State: "Active"},
			{Label: "Incidents", State: "API only"},
			{Label: "Operations", State: "API only"},
		},
		StatusItems: []adminWebStatusItem{
			{Label: "Admin session", Value: "Verified", Tone: "ok"},
			{Label: "Route group", Value: "Private /admin", Tone: "neutral"},
			{Label: "Public viewer", Value: "Not mounted", Tone: "warn"},
		},
	}
}

func makeAdminWebAccounts(accounts []auth.Account, currentAccountID string) []adminWebAccount {
	response := make([]adminWebAccount, 0, len(accounts))
	for _, account := range accounts {
		response = append(response, makeAdminWebAccount(account, currentAccountID))
	}
	return response
}

func makeAdminWebAccount(account auth.Account, currentAccountID string) adminWebAccount {
	return adminWebAccount{
		ID:                account.ID,
		Username:          account.Username,
		Role:              account.Role,
		CreatedAt:         account.CreatedAt,
		PasswordChangedAt: account.PasswordChangedAt,
		IsCurrent:         account.ID == currentAccountID,
	}
}

func (a *API) validateAdminWebCSRF(w http.ResponseWriter, r *http.Request, principal privatePrincipal) bool {
	want := adminWebCSRFTokenFromRequest(r)
	got := strings.TrimSpace(r.FormValue("csrf_token"))
	if want == "" || got == "" || !adminWebCSRFTokenValid(want, got) {
		a.renderAdminWebDashboard(w, r, principal, http.StatusForbidden, "", "The form expired. Reload the page and try again.")
		return false
	}
	return true
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

func setAdminWebPageHeaders(w http.ResponseWriter) {
	setPublicBrowserSecurityHeaders(w)
	setNoStore(w)
}

func setAdminWebStaticHeaders(w http.ResponseWriter) {
	setPublicBrowserSecurityHeaders(w)
}
