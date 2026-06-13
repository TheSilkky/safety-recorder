package httpapi

import (
	"net/http"
	"time"

	"github.com/open-proofline/server/internal/auth"
)

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

func setAdminWebPageHeaders(w http.ResponseWriter) {
	setPublicBrowserSecurityHeaders(w)
	setNoStore(w)
}

func setAdminWebStaticHeaders(w http.ResponseWriter) {
	setPublicBrowserSecurityHeaders(w)
}
