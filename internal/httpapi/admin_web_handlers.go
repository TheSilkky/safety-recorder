package httpapi

import (
	"errors"
	"net/http"

	"github.com/open-proofline/server/internal/auth"
)

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
