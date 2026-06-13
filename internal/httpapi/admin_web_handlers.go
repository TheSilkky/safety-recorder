package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/incidents"
)

func (a *API) adminWebPage(w http.ResponseWriter, r *http.Request) {
	a.adminWebServePage(w, r, adminWebPageOverview)
}

func (a *API) adminWebAccountsPage(w http.ResponseWriter, r *http.Request) {
	a.adminWebServePage(w, r, adminWebPageAccounts)
}

func (a *API) adminWebIncidentsPage(w http.ResponseWriter, r *http.Request) {
	a.adminWebServePage(w, r, adminWebPageIncidents)
}

func (a *API) adminWebSettingsPage(w http.ResponseWriter, r *http.Request) {
	a.adminWebServePage(w, r, adminWebPageSettings)
}

func (a *API) adminWebServePage(w http.ResponseWriter, r *http.Request, page string) {
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
	if !a.adminWebSecondFactorSatisfied(w, r, principal, adminWebNotice(r), "") {
		return
	}

	a.renderAdminWebPage(w, r, principal, page, http.StatusOK, adminWebNotice(r), "")
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
	if ok := a.parseAdminWebSessionForm(w, r, principal, "The logout form could not be read."); !ok {
		return
	}
	if !a.validateAdminWebSessionCSRF(w, r, principal) {
		return
	}
	if err := a.repo.RevokeSession(r.Context(), principal.Session.ID); err != nil && !errors.Is(err, auth.ErrNotFound) {
		a.adminWebInternalError(w, "revoke admin web session", err)
		return
	}

	clearAdminWebSessionCookie(w)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (a *API) adminWebEmailSecondFactorVerifyPage(w http.ResponseWriter, r *http.Request) {
	setAdminWebPageHeaders(w)
	principal, ok := a.requireAdminWebSession(w, r)
	if !ok {
		return
	}
	if adminRequiresSecondFactorSetup(principal.Account) {
		if a.emailSender == nil {
			a.renderAdminWebSecondFactorSetup(w, r, principal, http.StatusForbidden, adminWebNotice(r), "")
			return
		}
		a.renderAdminWeb(w, http.StatusForbidden, makeAdminWebSecondFactorSetupEmailVerifyData(
			principal,
			adminWebCSRFTokenFromRequest(r),
			adminWebNotice(r),
			"",
			a.adminWebAuthnAvailable(),
		))
		return
	}
	required, err := a.sessionRequiresSecondFactorVerification(r.Context(), principal.Account, principal.Session)
	if err != nil {
		a.adminWebInternalError(w, "check admin web email second factor requirement", err)
		return
	}
	if !required {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	data, err := a.makeAdminWebSecondFactorVerificationDataForRequest(r, principal, adminWebNotice(r), "")
	if err != nil {
		a.adminWebInternalError(w, "build admin web email second factor verification data", err)
		return
	}
	if !data.SecondFactorEmailAvailable {
		a.renderAdminWeb(w, http.StatusForbidden, data)
		return
	}
	data.Mode = "second_factor_verify_email"
	a.renderAdminWeb(w, http.StatusForbidden, data)
}

func (a *API) adminWebRequestEmailSecondFactorChallenge(w http.ResponseWriter, r *http.Request) {
	setAdminWebPageHeaders(w)
	principal, ok := a.requireAdminWebSession(w, r)
	if !ok {
		return
	}
	if !adminRequiresSecondFactorSetup(principal.Account) {
		required, err := a.sessionRequiresSecondFactorVerification(r.Context(), principal.Account, principal.Session)
		if err != nil {
			a.adminWebInternalError(w, "check admin web email second factor requirement", err)
			return
		}
		if !required {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
			return
		}
		data, err := a.makeAdminWebSecondFactorVerificationDataForRequest(r, principal, "", "The email verification form could not be read.")
		if err != nil {
			a.adminWebInternalError(w, "build admin web email second factor verification data", err)
			return
		}
		if ok := a.parseAdminWebForm(w, r, data); !ok {
			return
		}
		data.Error = ""
		if !a.validateAdminWebCSRFForData(w, r, data) {
			return
		}
		if a.emailSender == nil {
			data.Error = "Email second-factor delivery is not configured."
			a.renderAdminWeb(w, http.StatusServiceUnavailable, data)
			return
		}
		if !data.SecondFactorEmailAvailable {
			data.Error = "Email verification is not configured for this account."
			a.renderAdminWeb(w, http.StatusConflict, data)
			return
		}
		expiresAt := time.Now().UTC().Add(a.secondFactorEmailTTL)
		challenge, rawToken, err := a.repo.CreateActiveEmailSecondFactorChallenge(r.Context(), principal.Account.ID, expiresAt)
		if errors.Is(err, auth.ErrNotFound) {
			data.SecondFactorEmailAvailable = false
			data.Error = "Email verification is not configured for this account."
			a.renderAdminWeb(w, http.StatusConflict, data)
			return
		}
		if err != nil {
			a.adminWebInternalError(w, "create admin web active email second factor challenge", err)
			return
		}
		if !a.sendSecondFactorChallengeEmail(r, challenge.EmailNormalized, rawToken, challenge.ExpiresAt) {
			data.Error = "Email delivery is temporarily unavailable."
			a.renderAdminWeb(w, http.StatusServiceUnavailable, data)
			return
		}
		http.Redirect(w, r, "/admin/second-factor/email/verify?notice=second_factor_challenge_sent", http.StatusSeeOther)
		return
	}
	data := makeAdminWebSecondFactorSetupData(principal, adminWebCSRFTokenFromRequest(r), "", "The second-factor setup form could not be read.", a.emailSender != nil, a.adminWebAuthnAvailable())
	if ok := a.parseAdminWebForm(w, r, data); !ok {
		return
	}
	data.Error = ""
	if !a.validateAdminWebCSRFForData(w, r, data) {
		return
	}
	if a.emailSender == nil {
		data.Error = "Email second-factor delivery is not configured."
		a.renderAdminWeb(w, http.StatusServiceUnavailable, data)
		return
	}

	emailAddress := auth.NormalizeEmail(r.FormValue("email"))
	if emailAddress == "" && principal.Account.EmailNormalized != "" && principal.Account.EmailVerifiedAt != nil {
		emailAddress = principal.Account.EmailNormalized
	}
	if emailAddress == "" {
		data.Error = "Email address is required."
		a.renderAdminWeb(w, http.StatusBadRequest, data)
		return
	}
	if err := auth.ValidateEmail(emailAddress); err != nil {
		data.Error = err.Error()
		a.renderAdminWeb(w, http.StatusBadRequest, data)
		return
	}

	expiresAt := time.Now().UTC().Add(a.secondFactorEmailTTL)
	challenge, rawToken, err := a.repo.CreateEmailSecondFactorChallenge(r.Context(), auth.CreateEmailSecondFactorChallengeParams{
		AccountID:       principal.Account.ID,
		EmailNormalized: emailAddress,
		ExpiresAt:       expiresAt,
	})
	if errors.Is(err, auth.ErrDuplicate) {
		data.Error = "Email second factor is already configured."
		a.renderAdminWeb(w, http.StatusConflict, data)
		return
	}
	if errors.Is(err, auth.ErrNotFound) {
		data.Error = "Account was not found."
		a.renderAdminWeb(w, http.StatusNotFound, data)
		return
	}
	if err != nil {
		a.adminWebInternalError(w, "create admin web email second factor challenge", err)
		return
	}
	if !a.sendSecondFactorChallengeEmail(r, challenge.EmailNormalized, rawToken, challenge.ExpiresAt) {
		data.Error = "Email delivery is temporarily unavailable."
		a.renderAdminWeb(w, http.StatusServiceUnavailable, data)
		return
	}
	http.Redirect(w, r, "/admin/second-factor/email/verify?notice=second_factor_challenge_sent", http.StatusSeeOther)
}

func (a *API) adminWebVerifyEmailSecondFactorChallenge(w http.ResponseWriter, r *http.Request) {
	setAdminWebPageHeaders(w)
	principal, ok := a.requireAdminWebSession(w, r)
	if !ok {
		return
	}
	if !adminRequiresSecondFactorSetup(principal.Account) {
		required, err := a.sessionRequiresSecondFactorVerification(r.Context(), principal.Account, principal.Session)
		if err != nil {
			a.adminWebInternalError(w, "check admin web email second factor requirement", err)
			return
		}
		if !required {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
			return
		}
		data, err := a.makeAdminWebSecondFactorVerificationDataForRequest(r, principal, "", "The email verification form could not be read.")
		if err != nil {
			a.adminWebInternalError(w, "build admin web email second factor verification data", err)
			return
		}
		data.Mode = "second_factor_verify_email"
		if ok := a.parseAdminWebForm(w, r, data); !ok {
			return
		}
		data.Error = ""
		if !a.validateAdminWebCSRFForData(w, r, data) {
			return
		}
		rawToken := strings.TrimSpace(r.FormValue("code"))
		if rawToken == "" {
			data.Error = "Second-factor challenge is invalid or expired."
			a.renderAdminWeb(w, http.StatusBadRequest, data)
			return
		}
		factor, _, err := a.repo.ConsumeEmailSecondFactorChallenge(r.Context(), principal.Account.ID, rawToken, time.Now().UTC())
		if err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				data.Error = "Second-factor challenge is invalid or expired."
				a.renderAdminWeb(w, http.StatusBadRequest, data)
				return
			}
			a.adminWebInternalError(w, "consume admin web active email second factor challenge", err)
			return
		}
		if _, err := a.repo.MarkSessionSecondFactorVerified(r.Context(), principal.Session.ID, factor.ID, auth.SecondFactorTypeEmailChallenge, time.Now().UTC()); err != nil {
			a.adminWebInternalError(w, "mark admin web email session verified", err)
			return
		}
		http.Redirect(w, r, "/admin?notice=second_factor_verified", http.StatusSeeOther)
		return
	}
	data := makeAdminWebSecondFactorSetupEmailVerifyData(principal, adminWebCSRFTokenFromRequest(r), "", "The second-factor verification form could not be read.", a.adminWebAuthnAvailable())
	if ok := a.parseAdminWebForm(w, r, data); !ok {
		return
	}
	data.Error = ""
	if !a.validateAdminWebCSRFForData(w, r, data) {
		return
	}

	rawToken := strings.TrimSpace(r.FormValue("code"))
	if rawToken == "" {
		data.Error = "Second-factor challenge is invalid or expired."
		a.renderAdminWeb(w, http.StatusBadRequest, data)
		return
	}
	factor, _, err := a.repo.ConsumeEmailSecondFactorChallenge(r.Context(), principal.Account.ID, rawToken, time.Now().UTC())
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			data.Error = "Second-factor challenge is invalid or expired."
			a.renderAdminWeb(w, http.StatusBadRequest, data)
			return
		}
		a.adminWebInternalError(w, "consume admin web email second factor challenge", err)
		return
	}
	if _, err := a.repo.MarkSessionSecondFactorVerified(r.Context(), principal.Session.ID, factor.ID, auth.SecondFactorTypeEmailChallenge, time.Now().UTC()); err != nil {
		a.adminWebInternalError(w, "mark admin web email setup session verified", err)
		return
	}
	http.Redirect(w, r, "/admin?notice=second_factor_setup_complete", http.StatusSeeOther)
}

func (a *API) adminWebVerifyTOTPSecondFactorChallenge(w http.ResponseWriter, r *http.Request) {
	setAdminWebPageHeaders(w)
	principal, ok := a.requireAdminWebSession(w, r)
	if !ok {
		return
	}
	if adminRequiresSecondFactorSetup(principal.Account) {
		a.renderAdminWebSecondFactorSetup(w, r, principal, http.StatusForbidden, "", "")
		return
	}
	data, err := a.makeAdminWebSecondFactorVerificationDataForRequest(r, principal, "", "The TOTP verification form could not be read.")
	if err != nil {
		a.adminWebInternalError(w, "build admin web TOTP verification data", err)
		return
	}
	if ok := a.parseAdminWebForm(w, r, data); !ok {
		return
	}
	data.Error = ""
	if !a.validateAdminWebCSRFForData(w, r, data) {
		return
	}

	code := strings.TrimSpace(r.FormValue("code"))
	if code == "" {
		data.Error = "TOTP challenge is invalid or expired."
		a.renderAdminWeb(w, http.StatusBadRequest, data)
		return
	}
	factor, err := a.repo.GetActiveTOTPSecondFactor(r.Context(), principal.Account.ID)
	if errors.Is(err, auth.ErrNotFound) {
		data.SecondFactorTOTPAvailable = false
		data.Error = "TOTP verification is not configured for this account."
		a.renderAdminWeb(w, http.StatusConflict, data)
		return
	}
	if err != nil {
		a.adminWebInternalError(w, "get admin web active TOTP factor", err)
		return
	}
	now := time.Now().UTC()
	timeStep, valid, err := auth.MatchTOTPCode(factor.TOTPSecret, code, now, factor.TOTPPeriodSeconds, factor.TOTPDigits, factor.TOTPAlgorithm)
	if err != nil {
		a.adminWebInternalError(w, "validate admin web TOTP code", err)
		return
	}
	if !valid {
		data.Error = "TOTP challenge is invalid or expired."
		a.renderAdminWeb(w, http.StatusBadRequest, data)
		return
	}
	factor, err = a.repo.MarkTOTPSecondFactorUsed(r.Context(), factor.ID, now, timeStep)
	if errors.Is(err, auth.ErrNotFound) {
		data.Error = "TOTP challenge is invalid or expired."
		a.renderAdminWeb(w, http.StatusBadRequest, data)
		return
	}
	if err != nil {
		a.adminWebInternalError(w, "mark admin web TOTP factor used", err)
		return
	}
	if _, err := a.repo.MarkSessionSecondFactorVerified(r.Context(), principal.Session.ID, factor.ID, auth.SecondFactorTypeTOTP, now); err != nil {
		a.adminWebInternalError(w, "mark admin web TOTP session verified", err)
		return
	}
	http.Redirect(w, r, "/admin?notice=second_factor_verified", http.StatusSeeOther)
}

func (a *API) adminWebChangeOwnPassword(w http.ResponseWriter, r *http.Request) {
	setAdminWebPageHeaders(w)
	principal, ok := a.requireAdminWeb(w, r)
	if !ok {
		return
	}
	if ok := a.parseAdminWebPageForm(w, r, principal, adminWebPageSettings, "The password form could not be read."); !ok {
		return
	}
	if !a.validateAdminWebCSRF(w, r, principal) {
		return
	}
	if !auth.VerifyPassword(principal.Account.PasswordHash, r.FormValue("current_password")) {
		a.renderAdminWebSettings(w, r, principal, http.StatusUnauthorized, "", "Current password is invalid.")
		return
	}
	account, status, message, err, ok := a.adminWebUpdatePassword(r, principal.Account.ID, r.FormValue("new_password"))
	if err != nil {
		a.adminWebInternalError(w, "change admin web own password", err)
		return
	}
	if !ok {
		a.renderAdminWebSettings(w, r, principal, status, "", message)
		return
	}
	if _, err := a.repo.RevokeAccountSessions(r.Context(), account.ID, principal.Session.ID); err != nil {
		a.adminWebInternalError(w, "revoke admin web own sessions", err)
		return
	}
	http.Redirect(w, r, "/admin/settings?notice=password_changed", http.StatusSeeOther)
}

func (a *API) adminWebCreateAccount(w http.ResponseWriter, r *http.Request) {
	setAdminWebPageHeaders(w)
	principal, ok := a.requireAdminWeb(w, r)
	if !ok {
		return
	}
	if ok := a.parseAdminWebPageForm(w, r, principal, adminWebPageAccounts, "The account form could not be read."); !ok {
		return
	}
	if !a.validateAdminWebCSRF(w, r, principal) {
		return
	}

	_, status, message, err, ok := a.adminWebCreateManagedAccount(r, r.FormValue("username"), r.FormValue("password"), r.FormValue("role"))
	if err != nil {
		a.adminWebInternalError(w, "create admin web account", err)
		return
	}
	if !ok {
		a.renderAdminWebAccounts(w, r, principal, status, "", message)
		return
	}
	http.Redirect(w, r, "/admin/accounts?notice=account_created", http.StatusSeeOther)
}

func (a *API) adminWebResetAccountPassword(w http.ResponseWriter, r *http.Request) {
	setAdminWebPageHeaders(w)
	principal, ok := a.requireAdminWeb(w, r)
	if !ok {
		return
	}
	if ok := a.parseAdminWebPageForm(w, r, principal, adminWebPageAccounts, "The password reset form could not be read."); !ok {
		return
	}
	if !a.validateAdminWebCSRF(w, r, principal) {
		return
	}

	accountID := r.PathValue("account_id")
	if accountID == principal.Account.ID {
		a.renderAdminWebAccounts(w, r, principal, http.StatusBadRequest, "", "Use the admin password form to change your own password.")
		return
	}
	account, status, message, err, ok := a.adminWebUpdatePassword(r, accountID, r.FormValue("new_password"))
	if err != nil {
		a.adminWebInternalError(w, "reset admin web account password", err)
		return
	}
	if !ok {
		a.renderAdminWebAccounts(w, r, principal, status, "", message)
		return
	}
	if _, err := a.repo.RevokeAccountSessions(r.Context(), account.ID, ""); err != nil {
		a.adminWebInternalError(w, "revoke admin web account sessions", err)
		return
	}
	http.Redirect(w, r, "/admin/accounts?notice=account_password_reset", http.StatusSeeOther)
}

func (a *API) adminWebResetAccountSecondFactorRecovery(w http.ResponseWriter, r *http.Request) {
	setAdminWebPageHeaders(w)
	principal, ok := a.requireAdminWeb(w, r)
	if !ok {
		return
	}
	if ok := a.parseAdminWebPageForm(w, r, principal, adminWebPageAccounts, "The second-factor recovery form could not be read."); !ok {
		return
	}
	if !a.validateAdminWebCSRF(w, r, principal) {
		return
	}

	accountID := r.PathValue("account_id")
	if accountID == principal.Account.ID {
		a.renderAdminWebAccounts(w, r, principal, http.StatusBadRequest, "", "Second-factor recovery reset for the current admin account is not available from this form.")
		return
	}
	reason := strings.TrimSpace(r.FormValue("reason"))
	if !auth.ValidAccountRecoveryReason(reason) {
		a.renderAdminWebAccounts(w, r, principal, http.StatusBadRequest, "", "Recovery reason is not supported.")
		return
	}
	if _, _, err := a.repo.ResetAccountSecondFactorRecovery(r.Context(), auth.ResetAccountSecondFactorRecoveryParams{
		AccountID:      accountID,
		AdminAccountID: principal.Account.ID,
		Reason:         reason,
	}); errors.Is(err, auth.ErrNotFound) {
		a.renderAdminWebAccounts(w, r, principal, http.StatusNotFound, "", "Account was not found.")
		return
	} else if err != nil {
		a.adminWebInternalError(w, "reset admin web account second-factor recovery", err)
		return
	}
	http.Redirect(w, r, "/admin/accounts?notice=account_second_factor_reset", http.StatusSeeOther)
}

func (a *API) adminWebRevokeAccountSessions(w http.ResponseWriter, r *http.Request) {
	setAdminWebPageHeaders(w)
	principal, ok := a.requireAdminWeb(w, r)
	if !ok {
		return
	}
	if ok := a.parseAdminWebPageForm(w, r, principal, adminWebPageAccounts, "The session revocation form could not be read."); !ok {
		return
	}
	if !a.validateAdminWebCSRF(w, r, principal) {
		return
	}

	accountID := r.PathValue("account_id")
	if accountID == principal.Account.ID {
		a.renderAdminWebAccounts(w, r, principal, http.StatusBadRequest, "", "Use sign out to end the current admin session.")
		return
	}
	if _, err := a.repo.GetAccountByID(r.Context(), accountID); errors.Is(err, auth.ErrNotFound) {
		a.renderAdminWebAccounts(w, r, principal, http.StatusNotFound, "", "Account was not found.")
		return
	} else if err != nil {
		a.adminWebInternalError(w, "get admin web account for session revocation", err)
		return
	}
	if _, err := a.repo.RevokeAccountSessions(r.Context(), accountID, ""); err != nil {
		a.adminWebInternalError(w, "revoke admin web account sessions", err)
		return
	}
	http.Redirect(w, r, "/admin/accounts?notice=account_sessions_revoked", http.StatusSeeOther)
}

func (a *API) adminWebRequestIncidentDeletion(w http.ResponseWriter, r *http.Request) {
	setAdminWebPageHeaders(w)
	principal, ok := a.requireAdminWeb(w, r)
	if !ok {
		return
	}
	if ok := a.parseAdminWebPageForm(w, r, principal, adminWebPageIncidents, "The incident deletion form could not be read."); !ok {
		return
	}
	if !a.validateAdminWebCSRF(w, r, principal) {
		return
	}

	reasonCode, statusCode, message, ok := adminWebNormalizeDeletionReasonCode(r.FormValue("reason_code"))
	if !ok {
		a.renderAdminWebIncidents(w, r, principal, statusCode, "", message)
		return
	}
	status, err := a.repo.RequestIncidentDeletion(r.Context(), incidents.IncidentDeletionRequest{
		IncidentID:     r.PathValue("incident_id"),
		Source:         incidents.IncidentDeletionSourceAdminRequest,
		ReasonCode:     reasonCode,
		ActorAccountID: principal.Account.ID,
		AllowOpen:      adminWebFormBool(r.FormValue("allow_open")),
	})
	if errors.Is(err, incidents.ErrNotFound) {
		a.renderAdminWebIncidents(w, r, principal, http.StatusNotFound, "", "Incident was not found.")
		return
	}
	if errors.Is(err, incidents.ErrInvalidState) {
		a.renderAdminWebIncidents(w, r, principal, http.StatusConflict, "", "Incident cannot be deleted in its current state.")
		return
	}
	if err != nil {
		a.adminWebInternalError(w, "request admin web incident deletion", err)
		return
	}
	redirect := "/admin/incidents?notice=incident_deletion_requested&deletion_incident_id=" + url.QueryEscape(status.IncidentID)
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

func (a *API) adminWebReassignLegacyUnownedIncident(w http.ResponseWriter, r *http.Request) {
	setAdminWebPageHeaders(w)
	principal, ok := a.requireAdminWeb(w, r)
	if !ok {
		return
	}
	if ok := a.parseAdminWebPageForm(w, r, principal, adminWebPageIncidents, "The incident reassignment form could not be read."); !ok {
		return
	}
	if !a.validateAdminWebCSRF(w, r, principal) {
		return
	}

	action := strings.TrimSpace(r.FormValue("action"))
	newOwnerAccountID := strings.TrimSpace(r.FormValue("new_owner_account_id"))
	reasonCode := strings.TrimSpace(r.FormValue("reason_code"))
	if !validLegacyReassignmentAction(action) {
		a.renderAdminWebIncidents(w, r, principal, http.StatusBadRequest, "", "Reassignment action is not supported.")
		return
	}
	if !validLegacyReassignmentReasonCode(reasonCode) {
		a.renderAdminWebIncidents(w, r, principal, http.StatusBadRequest, "", "Reassignment reason code is not supported.")
		return
	}
	if action == incidents.LegacyIncidentReassignmentActionAssignOwner {
		if newOwnerAccountID == "" {
			a.renderAdminWebIncidents(w, r, principal, http.StatusBadRequest, "", "New owner account ID is required.")
			return
		}
		if _, err := a.repo.GetAccountByID(r.Context(), newOwnerAccountID); errors.Is(err, auth.ErrNotFound) {
			a.renderAdminWebIncidents(w, r, principal, http.StatusNotFound, "", "Destination account was not found.")
			return
		} else if err != nil {
			a.adminWebInternalError(w, "get admin web reassignment account", err)
			return
		}
	} else if newOwnerAccountID != "" {
		a.renderAdminWebIncidents(w, r, principal, http.StatusBadRequest, "", "New owner account ID is only allowed when assigning an owner.")
		return
	}

	_, err := a.repo.ReassignLegacyUnownedIncident(r.Context(), incidents.LegacyIncidentReassignmentParams{
		IncidentID:        r.PathValue("incident_id"),
		NewOwnerAccountID: newOwnerAccountID,
		ActorAccountID:    principal.Account.ID,
		Action:            action,
		ReasonCode:        reasonCode,
		Source:            incidents.LegacyIncidentReassignmentSourceAdminAPI,
	})
	if errors.Is(err, incidents.ErrNotFound) {
		a.renderAdminWebIncidents(w, r, principal, http.StatusNotFound, "", "Legacy unowned incident was not found.")
		return
	}
	if errors.Is(err, incidents.ErrInvalidState) {
		a.renderAdminWebIncidents(w, r, principal, http.StatusConflict, "", "Incident is not an active unowned legacy incident.")
		return
	}
	if err != nil {
		a.adminWebInternalError(w, "reassign admin web legacy unowned incident", err)
		return
	}
	http.Redirect(w, r, "/admin/incidents?notice=incident_reassignment_recorded", http.StatusSeeOther)
}

func (a *API) adminWebDeletionStatusFromQuery(r *http.Request) (adminWebDeletionStatus, string, error) {
	incidentID := strings.TrimSpace(r.URL.Query().Get("deletion_incident_id"))
	if incidentID == "" {
		return adminWebDeletionStatus{}, "", nil
	}
	status, err := a.repo.GetIncidentDeletionStatus(r.Context(), incidentID)
	if errors.Is(err, incidents.ErrNotFound) {
		return adminWebDeletionStatus{}, "Incident deletion was not found.", nil
	}
	if err != nil {
		return adminWebDeletionStatus{}, "", err
	}
	return makeAdminWebDeletionStatus(status), "", nil
}

func adminWebNormalizeDeletionReasonCode(value string) (string, int, string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", http.StatusOK, "", true
	}
	if len(value) > 64 {
		return "", http.StatusBadRequest, "Reason code must be a short non-sensitive code.", false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' || r == '-' || r == '.' || r == ':' {
			continue
		}
		return "", http.StatusBadRequest, "Reason code must be a short non-sensitive code.", false
	}
	return value, http.StatusOK, "", true
}

func adminWebFormBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "on", "yes":
		return true
	default:
		return false
	}
}
