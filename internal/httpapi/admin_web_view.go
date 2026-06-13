package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/incidents"
)

type adminWebData struct {
	Title                         string
	Mode                          string
	Error                         string
	Notice                        string
	CSRFToken                     string
	Account                       adminWebAccount
	Accounts                      []adminWebAccount
	IncidentCandidates            []adminWebIncidentCandidate
	DeletionStatus                adminWebDeletionStatus
	NavItems                      []adminWebNavItem
	StatusItems                   []adminWebStatusItem
	SecondFactorEmailAvailable    bool
	SecondFactorTOTPAvailable     bool
	SecondFactorWebAuthnAvailable bool
}

type adminWebAccount struct {
	ID                string
	Username          string
	Role              string
	CreatedAt         time.Time
	PasswordChangedAt time.Time
	IsCurrent         bool
}

type adminWebIncidentCandidate struct {
	IncidentID            string
	Status                string
	DeletionState         string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	StreamCount           int
	ChunkCount            int
	CheckinCount          int
	IncidentTokenCount    int
	HasActiveViewerTokens bool
	IncidentMode          string
	CaptureProfile        string
	EscalationPolicy      string
	SharingState          string
}

type adminWebDeletionStatus struct {
	IncidentID  string
	Source      string
	ReasonCode  string
	AllowOpen   bool
	State       string
	ItemCount   int
	ErrorCode   string
	RequestedAt time.Time
	UpdatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
}

type adminWebNavItem struct {
	Label       string
	Href        string
	Description string
	Current     bool
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
	candidates, err := a.repo.ListLegacyUnownedIncidentCandidates(r.Context(), defaultLegacyUnownedCandidateLimit)
	if err != nil {
		a.adminWebInternalError(w, "list admin web legacy unowned incidents", err)
		return
	}
	deletionStatus, deletionMessage, err := a.adminWebDeletionStatusFromQuery(r)
	if err != nil {
		a.adminWebInternalError(w, "get admin web incident deletion status", err)
		return
	}
	if deletionMessage != "" && message == "" {
		status = http.StatusNotFound
		message = deletionMessage
	}
	a.renderAdminWeb(w, status, makeAdminWebDashboardData(principal, accounts, candidates, deletionStatus, adminWebCSRFTokenFromRequest(r), notice, message))
}

func (a *API) renderAdminWebSecondFactorSetup(w http.ResponseWriter, r *http.Request, principal privatePrincipal, status int, notice, message string) {
	a.renderAdminWeb(w, status, makeAdminWebSecondFactorSetupData(
		principal,
		adminWebCSRFTokenFromRequest(r),
		notice,
		message,
		a.emailSender != nil,
		a.adminWebAuthnAvailable(),
	))
}

func (a *API) renderAdminWebSecondFactorVerification(w http.ResponseWriter, r *http.Request, principal privatePrincipal, status int, notice, message string) {
	data, err := a.makeAdminWebSecondFactorVerificationDataForRequest(r, principal, notice, message)
	if err != nil {
		a.adminWebInternalError(w, "check admin web second factor options", err)
		return
	}
	a.renderAdminWeb(w, status, data)
}

func (a *API) makeAdminWebSecondFactorVerificationDataForRequest(r *http.Request, principal privatePrincipal, notice, message string) (adminWebData, error) {
	totpAvailable, err := a.adminWebTOTPAvailable(r, principal)
	if err != nil {
		return adminWebData{}, err
	}
	emailAvailable, err := a.adminWebEmailAvailable(r, principal)
	if err != nil {
		return adminWebData{}, err
	}
	return makeAdminWebSecondFactorVerificationData(
		principal,
		adminWebCSRFTokenFromRequest(r),
		notice,
		message,
		emailAvailable,
		totpAvailable,
		a.adminWebAuthnAvailable(),
	), nil
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
	case "account_created":
		return "Account created."
	case "password_changed":
		return "Password changed."
	case "account_password_reset":
		return "Account password reset."
	case "account_sessions_revoked":
		return "Account sessions revoked."
	case "account_second_factor_reset":
		return "Account second-factor recovery reset."
	case "incident_deletion_requested":
		return "Incident deletion requested."
	case "incident_reassignment_recorded":
		return "Incident reassignment recorded."
	case "second_factor_challenge_sent":
		return "Second-factor challenge sent."
	case "second_factor_setup_complete":
		return "Admin second-factor setup completed."
	case "second_factor_verified":
		return "Admin second factor verified."
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

func makeAdminWebDashboardData(principal privatePrincipal, accounts []auth.Account, candidates []incidents.LegacyUnownedIncidentCandidate, deletionStatus adminWebDeletionStatus, csrfToken, notice, message string) adminWebData {
	return adminWebData{
		Title:              "Proofline Admin",
		Mode:               "dashboard",
		Error:              message,
		Notice:             notice,
		CSRFToken:          csrfToken,
		Account:            makeAdminWebAccount(principal.Account, principal.Account.ID),
		Accounts:           makeAdminWebAccounts(accounts, principal.Account.ID),
		IncidentCandidates: makeAdminWebIncidentCandidates(candidates),
		DeletionStatus:     deletionStatus,
		NavItems: []adminWebNavItem{
			{Label: "Accounts", Href: "#accounts", Description: "Local users", Current: true},
			{Label: "Operations", Href: "#operations", Description: "Incident controls"},
			{Label: "Boundary", Href: "#boundary", Description: "Private only"},
		},
		StatusItems: []adminWebStatusItem{
			{Label: "Admin session", Value: "Verified", Tone: "ok"},
			{Label: "Route group", Value: "Private /admin", Tone: "neutral"},
			{Label: "Public viewer", Value: "Not mounted", Tone: "warn"},
		},
	}
}

func makeAdminWebIncidentCandidates(candidates []incidents.LegacyUnownedIncidentCandidate) []adminWebIncidentCandidate {
	response := make([]adminWebIncidentCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		response = append(response, adminWebIncidentCandidate{
			IncidentID:            candidate.IncidentID,
			Status:                candidate.Status,
			DeletionState:         candidate.DeletionState,
			CreatedAt:             candidate.CreatedAt,
			UpdatedAt:             candidate.UpdatedAt,
			StreamCount:           candidate.StreamCount,
			ChunkCount:            candidate.ChunkCount,
			CheckinCount:          candidate.CheckinCount,
			IncidentTokenCount:    candidate.IncidentTokenCount,
			HasActiveViewerTokens: candidate.HasActiveViewerTokens,
			IncidentMode:          candidate.IncidentMode,
			CaptureProfile:        candidate.CaptureProfile,
			EscalationPolicy:      candidate.EscalationPolicy,
			SharingState:          candidate.SharingState,
		})
	}
	return response
}

func makeAdminWebDeletionStatus(status incidents.IncidentDeletionStatus) adminWebDeletionStatus {
	return adminWebDeletionStatus{
		IncidentID:  status.IncidentID,
		Source:      status.Source,
		ReasonCode:  status.ReasonCode,
		AllowOpen:   status.AllowOpen,
		State:       status.State,
		ItemCount:   status.ItemCount,
		ErrorCode:   status.ErrorCode,
		RequestedAt: status.RequestedAt,
		UpdatedAt:   status.UpdatedAt,
		StartedAt:   status.StartedAt,
		CompletedAt: status.CompletedAt,
	}
}

func makeAdminWebSecondFactorSetupData(principal privatePrincipal, csrfToken, notice, message string, emailAvailable, webAuthnAvailable bool) adminWebData {
	return adminWebData{
		Title:                         "Proofline Admin 2FA Setup",
		Mode:                          "second_factor_setup",
		Error:                         message,
		Notice:                        notice,
		CSRFToken:                     csrfToken,
		Account:                       makeAdminWebAccount(principal.Account, principal.Account.ID),
		SecondFactorEmailAvailable:    emailAvailable,
		SecondFactorWebAuthnAvailable: webAuthnAvailable,
	}
}

func makeAdminWebSecondFactorVerificationData(principal privatePrincipal, csrfToken, notice, message string, emailAvailable, totpAvailable, webAuthnAvailable bool) adminWebData {
	return adminWebData{
		Title:                         "Proofline Admin 2FA Verification",
		Mode:                          "second_factor_verify",
		Error:                         message,
		Notice:                        notice,
		CSRFToken:                     csrfToken,
		Account:                       makeAdminWebAccount(principal.Account, principal.Account.ID),
		SecondFactorEmailAvailable:    emailAvailable,
		SecondFactorTOTPAvailable:     totpAvailable,
		SecondFactorWebAuthnAvailable: webAuthnAvailable,
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

func (a *API) adminWebAuthnAvailable() bool {
	return a.webAuthn.Enabled && a.webAuthn.RPID != "" && len(a.webAuthn.AllowedOrigins) > 0
}

func (a *API) adminWebTOTPAvailable(r *http.Request, principal privatePrincipal) (bool, error) {
	_, err := a.repo.GetActiveTOTPSecondFactor(r.Context(), principal.Account.ID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, auth.ErrNotFound) {
		return false, nil
	}
	return false, err
}

func (a *API) adminWebEmailAvailable(r *http.Request, principal privatePrincipal) (bool, error) {
	if a.emailSender == nil {
		return false, nil
	}
	_, err := a.repo.GetActiveEmailSecondFactor(r.Context(), principal.Account.ID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, auth.ErrNotFound) {
		return false, nil
	}
	return false, err
}

func setAdminWebPageHeaders(w http.ResponseWriter) {
	setPublicBrowserSecurityHeaders(w)
	setNoStore(w)
}

func setAdminWebStaticHeaders(w http.ResponseWriter) {
	setPublicBrowserSecurityHeaders(w)
}
