package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/incidents"
)

const (
	adminWebPageOverview     = "dashboard"
	adminWebPageAccounts     = "accounts"
	adminWebPageIncidents    = "incidents"
	adminWebPageSettings     = "settings"
	adminWebAccountsPageSize = 10
)

type adminWebData struct {
	Title                         string
	Mode                          string
	AdminShell                    bool
	PageTitle                     string
	PageLead                      string
	Error                         string
	Notice                        string
	CSRFToken                     string
	Account                       adminWebAccount
	Accounts                      []adminWebAccount
	AccountPagination             adminWebAccountPagination
	RoleOptions                   []adminWebOption
	IncidentCandidates            []adminWebIncidentCandidate
	DeletionStatus                adminWebDeletionStatus
	NavItems                      []adminWebNavItem
	StatusItems                   []adminWebStatusItem
	ConfigItems                   []adminWebStatusItem
	SecondFactorItems             []adminWebStatusItem
	SecondFactorEmailAvailable    bool
	SecondFactorTOTPAvailable     bool
	SecondFactorWebAuthnAvailable bool
	SecondFactorTOTPEnrollment    adminWebTOTPEnrollment
}

type adminWebAccount struct {
	ID                string
	Username          string
	Email             string
	Role              string
	AccountState      string
	SecondFactorSetup string
	CreatedAt         time.Time
	PasswordChangedAt time.Time
	IsCurrent         bool
}

type adminWebAccountPagination struct {
	Search        string
	Page          int
	Total         int
	FilteredTotal int
	RangeLabel    string
	PrevHref      string
	NextHref      string
	HasPrev       bool
	HasNext       bool
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
	Label       string
	Value       string
	Description string
	Tone        string
}

type adminWebTOTPEnrollment struct {
	Secret        string
	OTPAuthURL    string
	Issuer        string
	AccountName   string
	PeriodSeconds int
	Digits        int
	Algorithm     string
}

type adminWebOption struct {
	Label    string
	Value    string
	Selected bool
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
	a.renderAdminWebOverview(w, r, principal, status, notice, message)
}

func (a *API) renderAdminWebPage(w http.ResponseWriter, r *http.Request, principal privatePrincipal, page string, status int, notice, message string) {
	switch page {
	case adminWebPageAccounts:
		a.renderAdminWebAccounts(w, r, principal, status, notice, message)
	case adminWebPageIncidents:
		a.renderAdminWebIncidents(w, r, principal, status, notice, message)
	case adminWebPageSettings:
		a.renderAdminWebSettings(w, r, principal, status, notice, message)
	default:
		a.renderAdminWebOverview(w, r, principal, status, notice, message)
	}
}

func (a *API) renderAdminWebOverview(w http.ResponseWriter, r *http.Request, principal privatePrincipal, status int, notice, message string) {
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
	committedBytes, err := a.adminWebCommittedBlobBytes(r, accounts)
	if err != nil {
		a.adminWebInternalError(w, "read admin web committed blob usage", err)
		return
	}
	repoOK := a.repo.Check(r.Context()) == nil
	storeOK := a.store.Check(r.Context()) == nil
	a.renderAdminWeb(w, status, makeAdminWebOverviewData(principal, accounts, len(candidates), committedBytes, repoOK, storeOK, adminWebCSRFTokenFromRequest(r), notice, message))
}

func (a *API) renderAdminWebAccounts(w http.ResponseWriter, r *http.Request, principal privatePrincipal, status int, notice, message string) {
	accounts, err := a.repo.ListAccounts(r.Context())
	if err != nil {
		a.adminWebInternalError(w, "list admin web accounts", err)
		return
	}
	filtered, pagination := adminWebPaginateAccounts(r, accounts, principal.Account.ID)
	a.renderAdminWeb(w, status, makeAdminWebAccountsData(principal, filtered, pagination, adminWebCSRFTokenFromRequest(r), notice, message))
}

func (a *API) renderAdminWebIncidents(w http.ResponseWriter, r *http.Request, principal privatePrincipal, status int, notice, message string) {
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
	a.renderAdminWeb(w, status, makeAdminWebIncidentsData(principal, candidates, deletionStatus, adminWebCSRFTokenFromRequest(r), notice, message))
}

func (a *API) renderAdminWebSettings(w http.ResponseWriter, r *http.Request, principal privatePrincipal, status int, notice, message string) {
	a.renderAdminWeb(w, status, makeAdminWebSettingsData(principal, adminWebCSRFTokenFromRequest(r), notice, message, a.adminWebConfigItems(), a.adminWebSecondFactorItems()))
}

func (a *API) adminWebCommittedBlobBytes(r *http.Request, accounts []auth.Account) (int64, error) {
	var total int64
	for _, account := range accounts {
		usage, err := a.repo.AccountCommittedBlobBytes(r.Context(), account.ID)
		if err != nil {
			return 0, err
		}
		total += usage
	}
	return total, nil
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

func makeAdminWebOverviewData(principal privatePrincipal, accounts []auth.Account, candidateCount int, committedBytes int64, repoOK, storeOK bool, csrfToken, notice, message string) adminWebData {
	data := makeAdminWebShellData(principal, adminWebPageOverview, "Dashboard Overview", "Private operational summary for this Proofline server.", csrfToken, notice, message)
	data.StatusItems = []adminWebStatusItem{
		{Label: "Metadata", Value: adminWebHealthValue(repoOK), Description: "Repository check", Tone: adminWebHealthTone(repoOK)},
		{Label: "Blob store", Value: adminWebHealthValue(storeOK), Description: "Storage boundary check", Tone: adminWebHealthTone(storeOK)},
		{Label: "Accounts", Value: strconv.Itoa(len(accounts)), Description: "Registered local accounts", Tone: "neutral"},
		{Label: "Committed blobs", Value: formatAdminWebBytes(committedBytes), Description: "Tracked encrypted chunk bytes", Tone: "neutral"},
		{Label: "Incident queue", Value: strconv.Itoa(candidateCount), Description: "Visible legacy unowned candidates", Tone: "warn"},
		{Label: "Storage capacity", Value: "Not exposed", Description: "Placeholder until backend usage totals are exposed", Tone: "warn"},
	}
	return data
}

func makeAdminWebAccountsData(principal privatePrincipal, accounts []adminWebAccount, pagination adminWebAccountPagination, csrfToken, notice, message string) adminWebData {
	data := makeAdminWebShellData(principal, adminWebPageAccounts, "Accounts", "Search and manage local Proofline accounts.", csrfToken, notice, message)
	data.Accounts = accounts
	data.AccountPagination = pagination
	return data
}

func makeAdminWebIncidentsData(principal privatePrincipal, candidates []incidents.LegacyUnownedIncidentCandidate, deletionStatus adminWebDeletionStatus, csrfToken, notice, message string) adminWebData {
	data := makeAdminWebShellData(principal, adminWebPageIncidents, "Incidents", "Private incident operation controls.", csrfToken, notice, message)
	data.IncidentCandidates = makeAdminWebIncidentCandidates(candidates)
	data.DeletionStatus = deletionStatus
	return data
}

func makeAdminWebSettingsData(principal privatePrincipal, csrfToken, notice, message string, configItems, secondFactorItems []adminWebStatusItem) adminWebData {
	data := makeAdminWebShellData(principal, adminWebPageSettings, "Settings", "Current admin account settings and safe server configuration.", csrfToken, notice, message)
	data.ConfigItems = configItems
	data.SecondFactorItems = secondFactorItems
	return data
}

func makeAdminWebShellData(principal privatePrincipal, page, title, lead, csrfToken, notice, message string) adminWebData {
	return adminWebData{
		Title:       "Proofline Admin",
		Mode:        page,
		AdminShell:  true,
		PageTitle:   title,
		PageLead:    lead,
		Error:       message,
		Notice:      notice,
		CSRFToken:   csrfToken,
		Account:     makeAdminWebAccount(principal.Account, principal.Account.ID),
		RoleOptions: adminWebRoleOptions(),
		NavItems:    adminWebNavItems(page),
	}
}

func adminWebRoleOptions() []adminWebOption {
	return []adminWebOption{
		{Label: "User", Value: auth.RoleUser, Selected: true},
		{Label: "Admin", Value: auth.RoleAdmin},
	}
}

func adminWebNavItems(page string) []adminWebNavItem {
	return []adminWebNavItem{
		{Label: "Overview", Href: "/admin", Description: "Server status", Current: page == adminWebPageOverview},
		{Label: "Accounts", Href: "/admin/accounts", Description: "Local users", Current: page == adminWebPageAccounts},
		{Label: "Incidents", Href: "/admin/incidents", Description: "Incident controls", Current: page == adminWebPageIncidents},
		{Label: "Settings", Href: "/admin/settings", Description: "Admin controls", Current: page == adminWebPageSettings},
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
		SecondFactorTOTPAvailable:     true,
		SecondFactorWebAuthnAvailable: webAuthnAvailable,
	}
}

func makeAdminWebSecondFactorSetupEmailVerifyData(principal privatePrincipal, csrfToken, notice, message string, webAuthnAvailable bool) adminWebData {
	return adminWebData{
		Title:                         "Proofline Admin 2FA Setup",
		Mode:                          "second_factor_setup_email_verify",
		Error:                         message,
		Notice:                        notice,
		CSRFToken:                     csrfToken,
		Account:                       makeAdminWebAccount(principal.Account, principal.Account.ID),
		SecondFactorEmailAvailable:    true,
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

func makeAdminWebTOTPEnrollment(account auth.Account, factor auth.SecondFactor) adminWebTOTPEnrollment {
	return adminWebTOTPEnrollment{
		Secret:        factor.TOTPSecret,
		OTPAuthURL:    adminWebTOTPAuthURL(account.Username, factor.TOTPSecret, factor.TOTPPeriodSeconds, factor.TOTPDigits, factor.TOTPAlgorithm),
		Issuer:        auth.TOTPIssuer,
		AccountName:   account.Username,
		PeriodSeconds: factor.TOTPPeriodSeconds,
		Digits:        factor.TOTPDigits,
		Algorithm:     factor.TOTPAlgorithm,
	}
}

func adminWebTOTPAuthURL(accountName, secret string, periodSeconds, digits int, algorithm string) string {
	query := url.Values{}
	query.Set("secret", secret)
	query.Set("issuer", auth.TOTPIssuer)
	query.Set("period", strconv.Itoa(periodSeconds))
	query.Set("digits", strconv.Itoa(digits))
	query.Set("algorithm", algorithm)
	return (&url.URL{
		Scheme:   "otpauth",
		Host:     "totp",
		Path:     "/" + auth.TOTPIssuer + ":" + strings.TrimSpace(accountName),
		RawQuery: query.Encode(),
	}).String()
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
		Email:             account.EmailNormalized,
		Role:              account.Role,
		AccountState:      account.AccountState,
		SecondFactorSetup: account.SecondFactorSetup,
		CreatedAt:         account.CreatedAt,
		PasswordChangedAt: account.PasswordChangedAt,
		IsCurrent:         account.ID == currentAccountID,
	}
}

func adminWebPaginateAccounts(r *http.Request, accounts []auth.Account, currentAccountID string) ([]adminWebAccount, adminWebAccountPagination) {
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	searchFolded := strings.ToLower(search)
	filtered := make([]auth.Account, 0, len(accounts))
	for _, account := range accounts {
		if searchFolded == "" ||
			strings.Contains(strings.ToLower(account.Username), searchFolded) ||
			strings.Contains(strings.ToLower(account.EmailNormalized), searchFolded) {
			filtered = append(filtered, account)
		}
	}

	page := 1
	if rawPage := strings.TrimSpace(r.URL.Query().Get("page")); rawPage != "" {
		if parsed, err := strconv.Atoi(rawPage); err == nil && parsed > 0 {
			page = parsed
		}
	}
	totalPages := 1
	if len(filtered) > 0 {
		totalPages = (len(filtered) + adminWebAccountsPageSize - 1) / adminWebAccountsPageSize
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * adminWebAccountsPageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + adminWebAccountsPageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	pagination := adminWebAccountPagination{
		Search:        search,
		Page:          page,
		Total:         len(accounts),
		FilteredTotal: len(filtered),
		RangeLabel:    adminWebAccountRangeLabel(start, end, len(filtered)),
		HasPrev:       page > 1,
		HasNext:       page < totalPages,
	}
	if pagination.HasPrev {
		pagination.PrevHref = adminWebAccountsHref(search, page-1)
	}
	if pagination.HasNext {
		pagination.NextHref = adminWebAccountsHref(search, page+1)
	}
	return makeAdminWebAccounts(filtered[start:end], currentAccountID), pagination
}

func adminWebAccountsHref(search string, page int) string {
	values := url.Values{}
	if search != "" {
		values.Set("q", search)
	}
	if page > 1 {
		values.Set("page", strconv.Itoa(page))
	}
	if encoded := values.Encode(); encoded != "" {
		return "/admin/accounts?" + encoded
	}
	return "/admin/accounts"
}

func adminWebAccountRangeLabel(start, end, total int) string {
	if total == 0 {
		return "0 of 0"
	}
	return fmt.Sprintf("%d-%d of %d", start+1, end, total)
}

func adminWebHealthValue(ok bool) string {
	if ok {
		return "OK"
	}
	return "Issue"
}

func adminWebHealthTone(ok bool) string {
	if ok {
		return "ok"
	}
	return "danger"
}

func formatAdminWebBytes(value int64) string {
	if value < 0 {
		value = 0
	}
	const unit = int64(1024)
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := unit, 0
	for n := value / unit; n >= unit && exp < 4; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(div), "KMGTPE"[exp])
}

func formatAdminWebDuration(value time.Duration) string {
	if value <= 0 {
		return "disabled"
	}
	return value.String()
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

func (a *API) adminWebSecondFactorItems() []adminWebStatusItem {
	emailValue := "Not configured"
	emailTone := "warn"
	if a.emailSender != nil {
		emailValue = "Available"
		emailTone = "neutral"
	}
	webAuthnValue := "Not configured"
	webAuthnTone := "warn"
	if a.adminWebAuthnAvailable() {
		webAuthnValue = "Available"
		webAuthnTone = "neutral"
	}
	return []adminWebStatusItem{
		{Label: "Security keys", Value: webAuthnValue, Description: "Recommended when WebAuthn/FIDO2 is configured", Tone: webAuthnTone},
		{Label: "TOTP", Value: "Available", Description: "Recommended fallback without WebAuthn", Tone: "neutral"},
		{Label: "Email challenge", Value: emailValue, Description: "Backup mail delivery fallback", Tone: emailTone},
	}
}

func (a *API) adminWebConfigItems() []adminWebStatusItem {
	return []adminWebStatusItem{
		{Label: "Max upload", Value: formatAdminWebBytes(a.maxUploadBytes), Description: "Per request body limit", Tone: "neutral"},
		{Label: "Account blob quota", Value: formatAdminWebBytes(a.accountBlobQuotaBytes), Description: "Default committed chunk quota", Tone: "neutral"},
		{Label: "Session TTL", Value: formatAdminWebDuration(a.sessionTTL), Description: "Server-side auth session expiry", Tone: "neutral"},
		{Label: "Incident token TTL", Value: formatAdminWebDuration(a.defaultIncidentTokenTTL), Description: "Default viewer token expiry", Tone: "neutral"},
		{Label: "Registration", Value: a.accountRegistration.Mode, Description: "Account registration mode", Tone: "neutral"},
		{Label: "Browser sessions", Value: adminWebEnabledValue(a.webAuth.Enabled), Description: "Cookie auth for main API", Tone: adminWebEnabledTone(a.webAuth.Enabled)},
		{Label: "WebAuthn", Value: adminWebEnabledValue(a.webAuthn.Enabled), Description: "RP details redacted", Tone: adminWebEnabledTone(a.webAuthn.Enabled)},
		{Label: "Email sender", Value: adminWebConfiguredValue(a.emailSender != nil), Description: "Provider details redacted", Tone: adminWebEnabledTone(a.emailSender != nil)},
		{Label: "Main rate limits", Value: adminWebEnabledValue(a.mainRateLimit.Enabled), Description: formatAdminWebDuration(a.mainRateLimit.Window), Tone: adminWebEnabledTone(a.mainRateLimit.Enabled)},
		{Label: "Viewer rate limits", Value: adminWebEnabledValue(a.publicRateLimit.Enabled), Description: formatAdminWebDuration(a.publicRateLimit.Window), Tone: adminWebEnabledTone(a.publicRateLimit.Enabled)},
		{Label: "Relay capability", Value: adminWebConfiguredValue(a.relayCapability.Secret != ""), Description: "Secret redacted", Tone: adminWebEnabledTone(a.relayCapability.Secret != "")},
		{Label: "Relay service auth", Value: adminWebConfiguredValue(a.relayService.AuthToken != ""), Description: "Token redacted", Tone: adminWebEnabledTone(a.relayService.AuthToken != "")},
	}
}

func adminWebEnabledValue(enabled bool) string {
	if enabled {
		return "Enabled"
	}
	return "Disabled"
}

func adminWebConfiguredValue(configured bool) string {
	if configured {
		return "Configured"
	}
	return "Not configured"
}

func adminWebEnabledTone(enabled bool) string {
	if enabled {
		return "neutral"
	}
	return "warn"
}

func setAdminWebPageHeaders(w http.ResponseWriter) {
	setPublicBrowserSecurityHeaders(w)
	setNoStore(w)
}

func setAdminWebStaticHeaders(w http.ResponseWriter) {
	setPublicBrowserSecurityHeaders(w)
}
