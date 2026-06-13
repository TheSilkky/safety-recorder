package httpapi_test

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/httpapi"
	"github.com/open-proofline/server/internal/incidents"
	"golang.org/x/crypto/bcrypt"
)

const adminWebSessionCookieName = "proofline_admin_session"

func TestAdminWebShowsLoginBeforeCookieSession(t *testing.T) {
	app := newTestApp(t)

	response, body := request(t, app.adminHandler, http.MethodGet, "/admin", "", nil)
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected admin login status 200, got %d: %s", response.StatusCode, body)
	}
	assertContentTypePrefix(t, response, "text/html")
	assertAdminWebPageHeaders(t, response)
	for _, expected := range []string{"Admin Login", `action="/admin/login"`, `name="username"`, `name="password"`} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("admin login page missing %q: %s", expected, body)
		}
	}
}

func TestAdminWebLoginSetsHttpOnlyCookieAndOpensDashboard(t *testing.T) {
	app := newTestApp(t)

	loginForm := url.Values{
		"username": {"test-admin"},
		"password": {"test-password"},
	}
	response, body := postAdminWebForm(t, app, "/admin/login", loginForm)
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected admin login redirect 303, got %d: %s", response.StatusCode, body)
	}
	if location := response.Header.Get("Location"); location != "/admin" {
		t.Fatalf("expected admin login redirect to /admin, got %q", location)
	}
	cookie := adminWebCookieFromResponse(t, response)
	setCookie := response.Header.Get("Set-Cookie")
	for _, expected := range []string{"HttpOnly", "SameSite=Strict", "Path=/admin"} {
		if !strings.Contains(setCookie, expected) {
			t.Fatalf("admin web session cookie missing %q: %s", expected, setCookie)
		}
	}

	response, body = requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin", "", nil, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected admin dashboard status 200, got %d: %s", response.StatusCode, body)
	}
	assertAdminWebPageHeaders(t, response)
	for _, expected := range []string{
		"Proofline Admin",
		"Operator Console",
		`/admin/static/admin.js`,
		`data-mobile-nav-details`,
		`id="admin-mobile-navigation"`,
		`href="/admin/accounts"`,
		`href="/admin/incidents"`,
		`href="/admin/settings"`,
		"Dashboard Overview",
		"Metadata",
		"Blob store",
		"Registered local accounts",
		"Committed blobs",
		"Private Only",
		"Private listener only",
	} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("admin dashboard missing %q: %s", expected, body)
		}
	}
	for _, disallowed := range []string{app.authToken, "test-password", "Authorization"} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("admin dashboard exposed %q: %s", disallowed, body)
		}
	}
}

func TestAdminWebDashboardListsAccounts(t *testing.T) {
	app := newTestApp(t)
	createAccountAndLogin(t, app, "managed-user", "managed-password", auth.RoleUser)
	userAccount := mustGetAccountByUsername(t, app, "managed-user")
	if _, err := app.db.ExecContext(t.Context(), `UPDATE accounts SET email_normalized = ? WHERE id = ?`, "managed@example.invalid", userAccount.ID); err != nil {
		t.Fatalf("set managed account email: %v", err)
	}
	cookie := loginAdminWeb(t, app)

	response, body := requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin/accounts", "", nil, cookie)
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected admin accounts status 200, got %d: %s", response.StatusCode, body)
	}
	for _, expected := range []string{
		"User Accounts",
		"Search accounts",
		`action="/admin/accounts"`,
		`method="get"`,
		`name="q"`,
		`<select name="role" required>`,
		`<option value="user" selected>User</option>`,
		`<option value="admin">Admin</option>`,
		"Back",
		"Next",
		"test-admin",
		"managed-user",
		`action="/admin/accounts/` + userAccount.ID + `/password"`,
		`action="/admin/accounts/` + userAccount.ID + `/sessions/revoke"`,
		`action="/admin/accounts/` + userAccount.ID + `/second-factor/recovery/reset"`,
		`name="reason" value="operator_review"`,
		`name="csrf_token"`,
	} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("admin dashboard missing %q: %s", expected, body)
		}
	}
	for _, disallowed := range []string{app.authToken, "test-password", "managed-password", "password_hash", "Authorization", `type="text" value="user" pattern="user|admin"`} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("admin dashboard exposed %q: %s", disallowed, body)
		}
	}

	response, body = requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin/accounts?q=managed@example.invalid", "", nil, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected admin account search status 200, got %d: %s", response.StatusCode, body)
	}
	for _, expected := range []string{"managed-user", "managed@example.invalid", `value="managed@example.invalid"`} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("admin account search missing %q: %s", expected, body)
		}
	}
	if bytes.Contains(body, []byte(">test-admin<")) {
		t.Fatalf("admin account email search included unrelated account: %s", body)
	}
}

func TestAdminWebSettingsShowsAdminControlsAndRedactedConfig(t *testing.T) {
	app := newTestAppWithOptions(t, httpapi.Options{
		MaxUploadBytes:        4096,
		AccountBlobQuotaBytes: 8192,
		WebAuth: httpapi.WebAuthConfig{
			Enabled: true,
		},
		WebAuthn: httpapi.WebAuthnConfig{
			Enabled:        true,
			RPID:           "admin.example.invalid",
			RPDisplayName:  "Proofline Admin Test",
			AllowedOrigins: []string{"https://admin.example.invalid"},
		},
		EmailSender: &recordingEmailSender{},
		MainRateLimit: httpapi.MainRateLimitConfig{
			Enabled: true,
			Window:  time.Minute,
		},
		PublicRateLimit: httpapi.PublicRateLimitConfig{
			Enabled: true,
			Window:  2 * time.Minute,
		},
		RelayCapability: httpapi.RelayCapabilityConfig{
			Secret: "relay-capability-secret",
		},
		RelayService: httpapi.RelayServiceConfig{
			AuthToken: "relay-service-token",
		},
	})
	cookie := loginAdminWeb(t, app)

	response, body := requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin/settings", "", nil, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected admin settings status 200, got %d: %s", response.StatusCode, body)
	}
	for _, expected := range []string{
		"Settings",
		"Admin Password",
		`action="/admin/password"`,
		"Second Factor",
		"Email challenge",
		"TOTP",
		"Security keys",
		"Recommended fallback without WebAuthn",
		"Configuration",
		"Max upload",
		"Account blob quota",
		"Registration",
		"Browser sessions",
		"WebAuthn",
		"Email sender",
		"Relay capability",
		"Relay service auth",
		"Provider details redacted",
		"RP details redacted",
		"Secret redacted",
		"Token redacted",
	} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("admin settings missing %q: %s", expected, body)
		}
	}
	for _, disallowed := range []string{
		app.authToken,
		"test-password",
		"relay-capability-secret",
		"relay-service-token",
		"admin.example.invalid",
		"https://admin.example.invalid",
		"Authorization",
	} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("admin settings exposed %q: %s", disallowed, body)
		}
	}
}

func TestAdminWebAdminCanCreateAccount(t *testing.T) {
	app := newTestApp(t)
	cookie := loginAdminWeb(t, app)
	csrfToken := adminWebDashboardCSRFToken(t, app, cookie)

	form := url.Values{
		"csrf_token": {csrfToken},
		"username":   {"created-user"},
		"role":       {auth.RoleUser},
		"password":   {"created-password"},
	}
	response, body := postAdminWebFormWithCookie(t, app, "/admin/accounts", form, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected account create redirect 303, got %d: %s", response.StatusCode, body)
	}
	if location := response.Header.Get("Location"); location != "/admin/accounts?notice=account_created" {
		t.Fatalf("expected account create redirect notice, got %q", location)
	}

	account := mustGetRegistrationAccount(t, app, "created-user")
	if account.Role != auth.RoleUser || account.SecondFactorSetup != auth.SecondFactorSetupStateSetupRequired {
		t.Fatalf("unexpected created account state: %+v", account)
	}
	_, loginAccount := loginWithAccountForTest(t, app, "created-user", "created-password")
	if loginAccount.SecondFactorSetup != auth.SecondFactorSetupStateSetupRequired || !loginAccount.RequiresSetup {
		t.Fatalf("created account should require setup: %+v", loginAccount)
	}

	response, body = requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin/accounts?notice=account_created", "", nil, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected dashboard after account create status 200, got %d: %s", response.StatusCode, body)
	}
	for _, expected := range []string{"Account created.", "created-user"} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("account create dashboard missing %q: %s", expected, body)
		}
	}
	for _, disallowed := range []string{"created-password", app.authToken, "Authorization"} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("account create dashboard exposed %q: %s", disallowed, body)
		}
	}
}

func TestAdminWebAdminCanChangeOwnPassword(t *testing.T) {
	app := newTestApp(t)
	cookie := loginAdminWeb(t, app)
	csrfToken := adminWebDashboardCSRFToken(t, app, cookie)

	form := url.Values{
		"csrf_token":       {csrfToken},
		"current_password": {"test-password"},
		"new_password":     {"replacement-password"},
	}
	response, body := postAdminWebFormWithCookie(t, app, "/admin/password", form, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected own password change redirect 303, got %d: %s", response.StatusCode, body)
	}
	if location := response.Header.Get("Location"); location != "/admin/settings?notice=password_changed" {
		t.Fatalf("expected own password change redirect notice, got %q", location)
	}

	response, body = requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin", "", nil, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected current admin web session to remain valid, got %d: %s", response.StatusCode, body)
	}

	response, body = postUnauthenticated(t, app, "/v1/auth/login", "application/json", bytes.NewBufferString(`{"username":"test-admin","password":"test-password"}`))
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected old admin password to fail, got %d: %s", response.StatusCode, body)
	}
	loginForTest(t, app, "test-admin", "replacement-password")
}

func TestAdminWebAdminCanResetUserPassword(t *testing.T) {
	app := newTestApp(t)
	userToken := createAccountAndLogin(t, app, "reset-user", "original-password", auth.RoleUser)
	userAccount := mustGetAccountByUsername(t, app, "reset-user")
	cookie := loginAdminWeb(t, app)
	csrfToken := adminWebDashboardCSRFToken(t, app, cookie)

	form := url.Values{
		"csrf_token":   {csrfToken},
		"new_password": {"replacement-password"},
	}
	response, body := postAdminWebFormWithCookie(t, app, "/admin/accounts/"+userAccount.ID+"/password", form, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected account password reset redirect 303, got %d: %s", response.StatusCode, body)
	}
	if location := response.Header.Get("Location"); location != "/admin/accounts?notice=account_password_reset" {
		t.Fatalf("expected account password reset redirect notice, got %q", location)
	}

	response, body = requestWithAuth(t, app.mainHandler, http.MethodGet, "/v1/account", "", nil, userToken)
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected reset user session to be revoked, got %d: %s", response.StatusCode, body)
	}
	loginForTest(t, app, "reset-user", "replacement-password")
}

func TestAdminWebAdminCanRevokeUserSessions(t *testing.T) {
	app := newTestApp(t)
	userToken := createAccountAndLogin(t, app, "session-web-user", "session-password", auth.RoleUser)
	userAccount := mustGetAccountByUsername(t, app, "session-web-user")
	cookie := loginAdminWeb(t, app)
	csrfToken := adminWebDashboardCSRFToken(t, app, cookie)

	form := url.Values{"csrf_token": {csrfToken}}
	response, body := postAdminWebFormWithCookie(t, app, "/admin/accounts/"+userAccount.ID+"/sessions/revoke", form, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected session revoke redirect 303, got %d: %s", response.StatusCode, body)
	}
	if location := response.Header.Get("Location"); location != "/admin/accounts?notice=account_sessions_revoked" {
		t.Fatalf("expected session revoke redirect notice, got %q", location)
	}

	response, body = requestWithAuth(t, app.mainHandler, http.MethodGet, "/v1/account", "", nil, userToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected revoked user session status 401, got %d: %s", response.StatusCode, body)
	}
}

func TestAdminWebAdminCanResetUserSecondFactorRecovery(t *testing.T) {
	app := newTestApp(t)
	userToken := createAccountAndLogin(t, app, "recovery-web-user", "recovery-password", auth.RoleUser)
	userAccount := mustGetRegistrationAccount(t, app, "recovery-web-user")
	if userAccount.SecondFactorSetup != auth.SecondFactorSetupStateComplete {
		t.Fatalf("test account setup state = %q, want complete", userAccount.SecondFactorSetup)
	}
	cookie := loginAdminWeb(t, app)
	csrfToken := adminWebDashboardCSRFToken(t, app, cookie)

	form := url.Values{
		"csrf_token": {csrfToken},
		"reason":     {auth.AccountRecoveryReasonOperatorReview},
	}
	response, body := postAdminWebFormWithCookie(t, app, "/admin/accounts/"+userAccount.ID+"/second-factor/recovery/reset", form, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected second-factor recovery redirect 303, got %d: %s", response.StatusCode, body)
	}
	if location := response.Header.Get("Location"); location != "/admin/accounts?notice=account_second_factor_reset" {
		t.Fatalf("expected second-factor recovery redirect notice, got %q", location)
	}

	updated := mustGetRegistrationAccount(t, app, "recovery-web-user")
	if updated.SecondFactorSetup != auth.SecondFactorSetupStateSetupRequired {
		t.Fatalf("updated account second-factor setup = %q, want setup_required", updated.SecondFactorSetup)
	}
	response, body = requestWithAuth(t, app.mainHandler, http.MethodGet, "/v1/account", "", nil, userToken)
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected recovered user session status 401, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin/accounts?notice=account_second_factor_reset", "", nil, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected dashboard after recovery reset status 200, got %d: %s", response.StatusCode, body)
	}
	for _, disallowed := range []string{"recovery-password", app.authToken, "Authorization", "raw operator note"} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("recovery reset dashboard exposed %q: %s", disallowed, body)
		}
	}
}

func TestAdminWebDashboardShowsIncidentOperationsSafely(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "candidate-web-owner", "owner-password", auth.RoleUser)
	ownedIncidentID := createIncidentWithAuth(t, app, ownerToken, `{"client_label":"owned phone"}`)
	legacyIncident := createLegacyIncidentForTest(t, app, "legacy phone", "legacy private note")

	stream := createMediaStream(t, app, legacyIncident.ID, incidents.MediaTypeAudio, "legacy stream")
	payload := []byte("encrypted audio bytes")
	response, body := uploadChunkWithStream(t, app, legacyIncident.ID, stream.ID, 1, incidents.MediaTypeAudio, payload, sha256Hex(payload))
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected chunk upload status 201, got %d: %s", response.StatusCode, body)
	}
	createCheckin(t, app, legacyIncident.ID)
	viewerToken := createIncidentToken(t, app, legacyIncident.ID, "viewer", nil)
	cookie := loginAdminWeb(t, app)

	response, body = requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin/incidents", "", nil, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected admin incidents status 200, got %d: %s", response.StatusCode, body)
	}
	for _, expected := range []string{
		"Incident Operations",
		legacyIncident.ID,
		"Viewer tokens",
		`action="/admin/incidents/` + legacyIncident.ID + `/reassignment"`,
		`action="/admin/incidents/` + legacyIncident.ID + `/deletion"`,
		`name="action" value="assign_owner"`,
		`name="action" value="keep_unowned"`,
		`name="allow_open" value="true"`,
	} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("admin dashboard incident operations missing %q: %s", expected, body)
		}
	}
	for _, disallowed := range []string{
		ownedIncidentID,
		"legacy phone",
		"legacy private note",
		string(payload),
		viewerToken.Token,
		app.authToken,
		"stored_path",
		"object_key",
		"latitude",
		"longitude",
		"Authorization",
	} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("admin dashboard incident operations exposed %q: %s", disallowed, body)
		}
	}
}

func TestAdminWebAdminCanReassignLegacyIncident(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "legacy-web-owner", "owner-password", auth.RoleUser)
	owner := getAccountByUsernameForTest(t, app, "legacy-web-owner")
	legacyIncident := createLegacyIncidentForTest(t, app, "legacy", "legacy note")
	cookie := loginAdminWeb(t, app)
	csrfToken := adminWebDashboardCSRFToken(t, app, cookie)

	assignForm := url.Values{
		"csrf_token":           {csrfToken},
		"action":               {incidents.LegacyIncidentReassignmentActionAssignOwner},
		"new_owner_account_id": {owner.ID},
		"reason_code":          {"owner_verified"},
	}
	response, body := postAdminWebFormWithCookie(t, app, "/admin/incidents/"+legacyIncident.ID+"/reassignment", assignForm, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected reassignment redirect 303, got %d: %s", response.StatusCode, body)
	}
	if location := response.Header.Get("Location"); location != "/admin/incidents?notice=incident_reassignment_recorded" {
		t.Fatalf("expected reassignment notice redirect, got %q", location)
	}
	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/incidents/"+legacyIncident.ID, "", nil, ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected owner incident detail after web reassignment status 200, got %d: %s", response.StatusCode, body)
	}
	assertLegacyReassignmentAuditRow(t, app, legacyIncident.ID, incidents.LegacyIncidentReassignmentActionAssignOwner, owner.ID)

	quarantinedIncident := createLegacyIncidentForTest(t, app, "legacy quarantine", "legacy quarantine note")
	keepForm := url.Values{
		"csrf_token":  {csrfToken},
		"action":      {incidents.LegacyIncidentReassignmentActionKeepUnowned},
		"reason_code": {"keep_admin_only"},
	}
	response, body = postAdminWebFormWithCookie(t, app, "/admin/incidents/"+quarantinedIncident.ID+"/reassignment", keepForm, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected keep-unowned redirect 303, got %d: %s", response.StatusCode, body)
	}
	if location := response.Header.Get("Location"); location != "/admin/incidents?notice=incident_reassignment_recorded" {
		t.Fatalf("expected keep-unowned notice redirect, got %q", location)
	}
	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/incidents/"+quarantinedIncident.ID, "", nil, ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected keep-unowned incident to stay hidden from owner, got %d: %s", response.StatusCode, body)
	}
	assertLegacyReassignmentAuditRow(t, app, quarantinedIncident.ID, incidents.LegacyIncidentReassignmentActionKeepUnowned, "")
}

func TestAdminWebAdminCanRequestIncidentDeletionAndViewStatus(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "delete-web-owner", "owner-password", auth.RoleUser)
	incidentID := createIncidentWithAuth(t, app, ownerToken, `{"client_label":"delete phone","notes":"delete private note"}`)
	viewerToken := createIncidentTokenWithAuth(t, app, ownerToken, incidentID, "viewer")
	cookie := loginAdminWeb(t, app)
	csrfToken := adminWebDashboardCSRFToken(t, app, cookie)

	form := url.Values{
		"csrf_token":  {csrfToken},
		"reason_code": {"admin_delete"},
		"allow_open":  {"true"},
	}
	response, body := postAdminWebFormWithCookie(t, app, "/admin/incidents/"+incidentID+"/deletion", form, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected deletion request redirect 303, got %d: %s", response.StatusCode, body)
	}
	location := response.Header.Get("Location")
	if location != "/admin/incidents?notice=incident_deletion_requested&deletion_incident_id="+incidentID {
		t.Fatalf("expected deletion status notice redirect, got %q", location)
	}

	response, body = requestWithCookie(t, app.adminHandler, http.MethodGet, location, "", nil, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected dashboard with deletion status 200, got %d: %s", response.StatusCode, body)
	}
	for _, expected := range []string{"Incident deletion requested.", "Deletion Status", incidentID, incidents.IncidentDeletionStatePending, "admin_delete"} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("deletion status dashboard missing %q: %s", expected, body)
		}
	}
	for _, disallowed := range []string{
		"delete phone",
		"delete private note",
		viewerToken.Token,
		app.authToken,
		"stored_path",
		"object_key",
		"Authorization",
	} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("deletion status dashboard exposed %q: %s", disallowed, body)
		}
	}
}

func TestAdminWebIncidentFormsRequireCSRFToken(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "csrf-incident-owner", "owner-password", auth.RoleUser)
	owner := getAccountByUsernameForTest(t, app, "csrf-incident-owner")
	legacyIncident := createLegacyIncidentForTest(t, app, "legacy", "legacy note")
	incidentID := createIncidentWithAuth(t, app, ownerToken, `{}`)
	cookie := loginAdminWeb(t, app)

	tests := []struct {
		name   string
		target string
		form   url.Values
	}{
		{
			name:   "reassign legacy incident",
			target: "/admin/incidents/" + legacyIncident.ID + "/reassignment",
			form: url.Values{
				"action":               {incidents.LegacyIncidentReassignmentActionAssignOwner},
				"new_owner_account_id": {owner.ID},
				"reason_code":          {"owner_verified"},
			},
		},
		{
			name:   "request deletion",
			target: "/admin/incidents/" + incidentID + "/deletion",
			form: url.Values{
				"reason_code": {"admin_delete"},
				"allow_open":  {"true"},
			},
		},
	}
	for _, tt := range tests {
		response, body := postAdminWebFormWithCookie(t, app, tt.target, tt.form, cookie)
		response.Body.Close()
		if response.StatusCode != http.StatusForbidden {
			t.Fatalf("%s: expected bad CSRF status 403, got %d: %s", tt.name, response.StatusCode, body)
		}
		if !bytes.Contains(body, []byte("The form expired.")) {
			t.Fatalf("%s: expected CSRF error message: %s", tt.name, body)
		}
	}
}

func TestAdminWebIncidentFormsRequireAdminSession(t *testing.T) {
	app := newTestApp(t)
	userToken := createAccountAndLogin(t, app, "incident-form-user", "user-password", auth.RoleUser)
	incidentID := createIncidentWithAuth(t, app, userToken, `{}`)
	form := url.Values{
		"reason_code": {"admin_delete"},
		"allow_open":  {"true"},
	}

	response, body := postAdminWebForm(t, app, "/admin/incidents/"+incidentID+"/deletion", form)
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected incident form without session status 401, got %d: %s", response.StatusCode, body)
	}
	if !bytes.Contains(body, []byte("Admin login is required.")) {
		t.Fatalf("expected admin login required message: %s", body)
	}

	userCookie := &http.Cookie{Name: adminWebSessionCookieName, Value: userToken}
	response, body = postAdminWebFormWithCookie(t, app, "/admin/incidents/"+incidentID+"/deletion", form, userCookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected incident form non-admin status 403, got %d: %s", response.StatusCode, body)
	}
	if !bytes.Contains(body, []byte("Access Denied")) {
		t.Fatalf("expected access denied page: %s", body)
	}
	if !strings.Contains(response.Header.Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("expected non-admin cookie to be cleared, got %q", response.Header.Get("Set-Cookie"))
	}
}

func TestAdminWebIncidentFormsShowSafeErrors(t *testing.T) {
	app := newTestApp(t)
	legacyIncident := createLegacyIncidentForTest(t, app, "legacy", "legacy note")
	cookie := loginAdminWeb(t, app)
	csrfToken := adminWebDashboardCSRFToken(t, app, cookie)

	badReason := "raw operator note"
	form := url.Values{
		"csrf_token":  {csrfToken},
		"reason_code": {badReason},
		"allow_open":  {"true"},
	}
	response, body := postAdminWebFormWithCookie(t, app, "/admin/incidents/"+legacyIncident.ID+"/deletion", form, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid deletion reason status 400, got %d: %s", response.StatusCode, body)
	}
	if !bytes.Contains(body, []byte("Reason code must be a short non-sensitive code.")) || bytes.Contains(body, []byte(badReason)) {
		t.Fatalf("expected safe deletion reason error: %s", body)
	}

	form = url.Values{
		"csrf_token":  {csrfToken},
		"action":      {incidents.LegacyIncidentReassignmentActionKeepUnowned},
		"reason_code": {badReason},
	}
	response, body = postAdminWebFormWithCookie(t, app, "/admin/incidents/"+legacyIncident.ID+"/reassignment", form, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected invalid reassignment reason status 400, got %d: %s", response.StatusCode, body)
	}
	if !bytes.Contains(body, []byte("Reassignment reason code is not supported.")) || bytes.Contains(body, []byte(badReason)) {
		t.Fatalf("expected safe reassignment reason error: %s", body)
	}
}

func TestAdminWebAccountFormsRequireCSRFToken(t *testing.T) {
	app := newTestApp(t)
	cookie := loginAdminWeb(t, app)
	userToken := createAccountAndLogin(t, app, "csrf-account-user", "csrf-password", auth.RoleUser)
	userAccount := mustGetAccountByUsername(t, app, "csrf-account-user")

	form := url.Values{
		"csrf_token":       {"not-a-valid-token"},
		"current_password": {"test-password"},
		"new_password":     {"replacement-password"},
	}
	response, body := postAdminWebFormWithCookie(t, app, "/admin/password", form, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected bad CSRF status 403, got %d: %s", response.StatusCode, body)
	}
	if !bytes.Contains(body, []byte("The form expired.")) {
		t.Fatalf("expected CSRF error message: %s", body)
	}
	loginForTest(t, app, "test-admin", "test-password")

	tests := []struct {
		name   string
		target string
		form   url.Values
	}{
		{
			name:   "create account",
			target: "/admin/accounts",
			form: url.Values{
				"username": {"csrf-created-user"},
				"role":     {auth.RoleUser},
				"password": {"csrf-created-password"},
			},
		},
		{
			name:   "revoke sessions",
			target: "/admin/accounts/" + userAccount.ID + "/sessions/revoke",
			form:   url.Values{},
		},
		{
			name:   "reset second factor recovery",
			target: "/admin/accounts/" + userAccount.ID + "/second-factor/recovery/reset",
			form: url.Values{
				"reason": {auth.AccountRecoveryReasonOperatorReview},
			},
		},
	}
	for _, tt := range tests {
		response, body := postAdminWebFormWithCookie(t, app, tt.target, tt.form, cookie)
		response.Body.Close()
		if response.StatusCode != http.StatusForbidden {
			t.Fatalf("%s: expected bad CSRF status 403, got %d: %s", tt.name, response.StatusCode, body)
		}
		if !bytes.Contains(body, []byte("The form expired.")) {
			t.Fatalf("%s: expected CSRF error message: %s", tt.name, body)
		}
	}
	response, body = requestWithAuth(t, app.mainHandler, http.MethodGet, "/v1/account", "", nil, userToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected bad-CSRF user session to remain valid, got %d: %s", response.StatusCode, body)
	}
}

func TestAdminWebAccountFormsRequireAdminSession(t *testing.T) {
	app := newTestApp(t)

	form := url.Values{
		"username": {"unauthorized-user"},
		"role":     {auth.RoleUser},
		"password": {"unauthorized-password"},
	}
	response, body := postAdminWebForm(t, app, "/admin/accounts", form)
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected account form without session status 401, got %d: %s", response.StatusCode, body)
	}
	if !bytes.Contains(body, []byte("Admin login is required.")) {
		t.Fatalf("expected admin login required message: %s", body)
	}

	userToken := createAccountAndLogin(t, app, "account-form-user", "account-form-password", auth.RoleUser)
	userCookie := &http.Cookie{Name: adminWebSessionCookieName, Value: userToken}
	response, body = postAdminWebFormWithCookie(t, app, "/admin/accounts", form, userCookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected account form non-admin status 403, got %d: %s", response.StatusCode, body)
	}
	if !bytes.Contains(body, []byte("Access Denied")) {
		t.Fatalf("expected access denied page: %s", body)
	}
	if !strings.Contains(response.Header.Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("expected non-admin cookie to be cleared, got %q", response.Header.Get("Set-Cookie"))
	}
}

func TestAdminWebBlocksUnsafeOwnAccountActions(t *testing.T) {
	app := newTestApp(t)
	adminAccount := mustGetAccountByUsername(t, app, "test-admin")
	cookie := loginAdminWeb(t, app)
	csrfToken := adminWebDashboardCSRFToken(t, app, cookie)

	tests := []struct {
		name    string
		target  string
		form    url.Values
		message string
	}{
		{
			name:   "password reset",
			target: "/admin/accounts/" + adminAccount.ID + "/password",
			form: url.Values{
				"csrf_token":   {csrfToken},
				"new_password": {"replacement-password"},
			},
			message: "Use the admin password form to change your own password.",
		},
		{
			name:    "session revoke",
			target:  "/admin/accounts/" + adminAccount.ID + "/sessions/revoke",
			form:    url.Values{"csrf_token": {csrfToken}},
			message: "Use sign out to end the current admin session.",
		},
		{
			name:   "second factor recovery reset",
			target: "/admin/accounts/" + adminAccount.ID + "/second-factor/recovery/reset",
			form: url.Values{
				"csrf_token": {csrfToken},
				"reason":     {auth.AccountRecoveryReasonOperatorReview},
			},
			message: "Second-factor recovery reset for the current admin account is not available from this form.",
		},
	}
	for _, tt := range tests {
		response, body := postAdminWebFormWithCookie(t, app, tt.target, tt.form, cookie)
		response.Body.Close()
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: expected own-account action status 400, got %d: %s", tt.name, response.StatusCode, body)
		}
		if !bytes.Contains(body, []byte(tt.message)) {
			t.Fatalf("%s: expected own-account message %q: %s", tt.name, tt.message, body)
		}
	}

	response, body := requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin/accounts", "", nil, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("User Accounts")) {
		t.Fatalf("expected own-account block to preserve admin session, got %d: %s", response.StatusCode, body)
	}
}

func TestAdminWebLogoutRequiresCSRFToken(t *testing.T) {
	app := newTestApp(t)
	cookie := loginAdminWeb(t, app)

	response, body := postAdminWebFormWithCookie(t, app, "/admin/logout", url.Values{}, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected logout without CSRF status 403, got %d: %s", response.StatusCode, body)
	}
	if strings.Contains(response.Header.Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("logout without CSRF should not clear cookie, got %q", response.Header.Get("Set-Cookie"))
	}

	response, body = requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin", "", nil, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("Dashboard Overview")) {
		t.Fatalf("expected admin session to remain valid after bad logout, got %d: %s", response.StatusCode, body)
	}

	csrfToken := adminWebCSRFTokenFromBody(t, body)
	response, body = postAdminWebFormWithCookie(t, app, "/admin/logout", url.Values{"csrf_token": []string{csrfToken}}, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected logout redirect 303, got %d: %s", response.StatusCode, body)
	}
	if !strings.Contains(response.Header.Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("expected logout to clear cookie, got %q", response.Header.Get("Set-Cookie"))
	}

	response, body = requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin", "", nil, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("Admin Login")) {
		t.Fatalf("expected revoked admin session to return login page, got %d: %s", response.StatusCode, body)
	}
}

func TestAdminWebRejectsNonAdminCookieSession(t *testing.T) {
	app := newTestApp(t)
	userToken := createAccountAndLogin(t, app, "admin-web-user", "regular-password", auth.RoleUser)
	cookie := &http.Cookie{Name: adminWebSessionCookieName, Value: userToken}

	response, body := requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin", "", nil, cookie)
	defer response.Body.Close()

	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected non-admin admin web status 403, got %d: %s", response.StatusCode, body)
	}
	assertAdminWebPageHeaders(t, response)
	if !bytes.Contains(body, []byte("Access Denied")) {
		t.Fatalf("expected access denied page: %s", body)
	}
	if !strings.Contains(response.Header.Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("expected non-admin cookie to be cleared, got %q", response.Header.Get("Set-Cookie"))
	}
}

func TestAdminWebBootstrapScreenCreatesFirstAdminSession(t *testing.T) {
	app := newTestAppWithoutTestAccount(t, httpapi.Options{
		BootstrapSecret: "bootstrap-secret",
		PasswordCost:    bcrypt.MinCost,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	response, body := request(t, app.adminHandler, http.MethodGet, "/admin", "", nil)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected bootstrap page status 200, got %d: %s", response.StatusCode, body)
	}
	for _, expected := range []string{"Create First Admin", `action="/admin/bootstrap"`, `name="bootstrap_secret"`} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("bootstrap page missing %q: %s", expected, body)
		}
	}

	badForm := url.Values{
		"bootstrap_secret": {"wrong-secret"},
		"username":         {"admin"},
		"password":         {"replace-with-long-local-password"},
	}
	response, body = postAdminWebForm(t, app, "/admin/bootstrap", badForm)
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected invalid bootstrap status 401, got %d: %s", response.StatusCode, body)
	}
	if !bytes.Contains(body, []byte("Bootstrap secret is invalid.")) {
		t.Fatalf("expected invalid bootstrap message: %s", body)
	}

	goodForm := url.Values{
		"bootstrap_secret": {"bootstrap-secret"},
		"username":         {"Admin.Web"},
		"password":         {"replace-with-long-local-password"},
	}
	response, body = postAdminWebForm(t, app, "/admin/bootstrap", goodForm)
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected bootstrap redirect 303, got %d: %s", response.StatusCode, body)
	}
	cookie := adminWebCookieFromResponse(t, response)

	response, body = requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin", "", nil, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected bootstrapped setup status 403, got %d: %s", response.StatusCode, body)
	}
	for _, expected := range []string{"Set Up Admin 2FA", "Email fallback is not configured", `action="/admin/logout"`} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("bootstrapped setup page missing %q: %s", expected, body)
		}
	}
	for _, disallowed := range []string{"User Accounts", `action="/admin/password"`, "replace-with-long-local-password"} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("bootstrapped setup page exposed %q: %s", disallowed, body)
		}
	}

	csrfToken := adminWebCSRFTokenFromBody(t, body)
	response, body = postAdminWebFormWithCookie(t, app, "/admin/password", url.Values{
		"csrf_token":       {csrfToken},
		"current_password": {"replace-with-long-local-password"},
		"new_password":     {"new-local-password"},
	}, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected setup-required admin password action status 403, got %d: %s", response.StatusCode, body)
	}
	if !bytes.Contains(body, []byte("Set Up Admin 2FA")) || bytes.Contains(body, []byte("Password changed.")) {
		t.Fatalf("expected setup gate for password action: %s", body)
	}
}

func TestAdminWebEmailSecondFactorSetupUnlocksDashboard(t *testing.T) {
	sender := &recordingEmailSender{}
	app := newTestAppWithOptions(t, httpapi.Options{
		EmailSender:          sender,
		SecondFactorEmailTTL: time.Minute,
	})
	if _, err := app.db.ExecContext(t.Context(), `UPDATE accounts SET second_factor_setup_state = ? WHERE username = ?`, auth.SecondFactorSetupStateSetupRequired, "test-admin"); err != nil {
		t.Fatalf("mark admin setup required: %v", err)
	}
	cookie := loginAdminWeb(t, app)

	response, body := requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin", "", nil, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected admin web setup status 403, got %d: %s", response.StatusCode, body)
	}
	for _, expected := range []string{"Set Up Admin 2FA", "Authenticator app", "Set up authenticator app", "Email challenge", "Send email code"} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("admin web setup page missing %q: %s", expected, body)
		}
	}
	for _, disallowed := range []string{"Complete setup", `action="/admin/second-factor/email/verify"`} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("admin web setup page exposed code verification form %q: %s", disallowed, body)
		}
	}
	csrfToken := adminWebCSRFTokenFromBody(t, body)

	response, body = postAdminWebFormWithCookie(t, app, "/admin/second-factor/email/challenge", url.Values{
		"csrf_token": {csrfToken},
		"email":      {"admin-2fa@example.invalid"},
	}, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected email challenge redirect 303, got %d: %s", response.StatusCode, body)
	}
	if location := response.Header.Get("Location"); location != "/admin/second-factor/email/verify?notice=second_factor_challenge_sent" {
		t.Fatalf("expected email challenge notice redirect, got %q", location)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("expected one second-factor email, got %d", len(sender.messages))
	}
	code := secondFactorCodeFromEmail(t, sender.messages[0])

	response, body = requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin/second-factor/email/verify?notice=second_factor_challenge_sent", "", nil, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected admin web setup code status 403, got %d: %s", response.StatusCode, body)
	}
	for _, expected := range []string{"Verify Email Code", "Second-factor challenge sent.", "Complete setup", `action="/admin/second-factor/email/verify"`} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("admin web setup code page missing %q: %s", expected, body)
		}
	}
	for _, disallowed := range []string{"Send email code", "admin-2fa@example.invalid", app.authToken} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("admin web setup code page exposed %q: %s", disallowed, body)
		}
	}
	csrfToken = adminWebCSRFTokenFromBody(t, body)
	response, body = postAdminWebFormWithCookie(t, app, "/admin/second-factor/email/verify", url.Values{
		"csrf_token": {csrfToken},
		"code":       {code},
	}, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected email verification redirect 303, got %d: %s", response.StatusCode, body)
	}
	if location := response.Header.Get("Location"); location != "/admin?notice=second_factor_setup_complete" {
		t.Fatalf("expected setup complete notice redirect, got %q", location)
	}

	response, body = requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin", "", nil, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected admin dashboard after email setup status 200, got %d: %s", response.StatusCode, body)
	}
	for _, expected := range []string{"Dashboard Overview", "Metadata", "Private Only"} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("admin dashboard after email setup missing %q: %s", expected, body)
		}
	}
	for _, disallowed := range []string{code, "admin-2fa@example.invalid", app.authToken} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("admin dashboard after email setup exposed %q: %s", disallowed, body)
		}
	}

	newCookie := loginAdminWeb(t, app)
	response, body = requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin", "", nil, newCookie)
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected admin email 2FA gate after relogin status 403, got %d: %s", response.StatusCode, body)
	}
	for _, expected := range []string{"Verify Admin 2FA", "Email challenge", "Send email code", `action="/admin/second-factor/email/challenge"`} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("admin web email gate missing %q: %s", expected, body)
		}
	}
	for _, disallowed := range []string{"User Accounts", `action="/admin/password"`, `action="/admin/second-factor/email/verify"`, code, "admin-2fa@example.invalid", app.authToken} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("admin web email gate exposed %q: %s", disallowed, body)
		}
	}

	csrfToken = adminWebCSRFTokenFromBody(t, body)
	response, body = postAdminWebFormWithCookie(t, app, "/admin/second-factor/email/challenge", url.Values{
		"csrf_token": {csrfToken},
	}, newCookie)
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected email verification challenge redirect 303, got %d: %s", response.StatusCode, body)
	}
	if location := response.Header.Get("Location"); location != "/admin/second-factor/email/verify?notice=second_factor_challenge_sent" {
		t.Fatalf("expected email verification challenge notice redirect, got %q", location)
	}
	if len(sender.messages) != 2 {
		t.Fatalf("expected two second-factor emails, got %d", len(sender.messages))
	}
	verifyCode := secondFactorCodeFromEmail(t, sender.messages[1])

	response, body = requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin/second-factor/email/verify?notice=second_factor_challenge_sent", "", nil, newCookie)
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected admin web email code status 403, got %d: %s", response.StatusCode, body)
	}
	for _, expected := range []string{"Verify Email Code", "Second-factor challenge sent.", "Verify email code", `action="/admin/second-factor/email/verify"`} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("admin web email code page missing %q: %s", expected, body)
		}
	}
	for _, disallowed := range []string{"Send email code", "admin-2fa@example.invalid", app.authToken} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("admin web email code page exposed %q: %s", disallowed, body)
		}
	}
	csrfToken = adminWebCSRFTokenFromBody(t, body)
	response, body = postAdminWebFormWithCookie(t, app, "/admin/second-factor/email/verify", url.Values{
		"csrf_token": {csrfToken},
		"code":       {verifyCode},
	}, newCookie)
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected email session verification redirect 303, got %d: %s", response.StatusCode, body)
	}
	if location := response.Header.Get("Location"); location != "/admin?notice=second_factor_verified" {
		t.Fatalf("expected email verified notice redirect, got %q", location)
	}

	response, body = requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin", "", nil, newCookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected admin dashboard after email session verification status 200, got %d: %s", response.StatusCode, body)
	}
	if !bytes.Contains(body, []byte("Dashboard Overview")) || bytes.Contains(body, []byte(verifyCode)) {
		t.Fatalf("expected dashboard without email code exposure: %s", body)
	}
}

func TestAdminWebTOTPSecondFactorSetupUnlocksDashboard(t *testing.T) {
	app := newTestApp(t)
	if _, err := app.db.ExecContext(t.Context(), `UPDATE accounts SET second_factor_setup_state = ? WHERE username = ?`, auth.SecondFactorSetupStateSetupRequired, "test-admin"); err != nil {
		t.Fatalf("mark admin setup required: %v", err)
	}
	cookie := loginAdminWeb(t, app)

	response, body := requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin", "", nil, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected admin web setup status 403, got %d: %s", response.StatusCode, body)
	}
	for _, expected := range []string{"Set Up Admin 2FA", "Authenticator app", "Recommended", `action="/admin/second-factor/totp/enroll"`} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("admin web TOTP setup page missing %q: %s", expected, body)
		}
	}
	for _, disallowed := range []string{"Manual setup key", `action="/admin/second-factor/totp/confirm"`, app.authToken} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("admin web TOTP setup page exposed %q: %s", disallowed, body)
		}
	}
	csrfToken := adminWebCSRFTokenFromBody(t, body)

	response, body = postAdminWebFormWithCookie(t, app, "/admin/second-factor/totp/enroll", url.Values{
		"csrf_token": {csrfToken},
	}, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected TOTP enrollment setup page status 403, got %d: %s", response.StatusCode, body)
	}
	adminAccount := mustGetAccountByUsername(t, app, "test-admin")
	factor, err := incidents.NewRepository(app.db).GetPendingTOTPSecondFactor(t.Context(), adminAccount.ID)
	if err != nil {
		t.Fatalf("get pending TOTP factor: %v", err)
	}
	for _, expected := range []string{
		"TOTP setup started.",
		"Manual setup key",
		factor.TOTPSecret,
		"otpauth://totp/Proofline:test-admin",
		`action="/admin/second-factor/totp/confirm"`,
		"Confirm authenticator app",
	} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("admin web TOTP enrollment page missing %q: %s", expected, body)
		}
	}
	for _, disallowed := range []string{app.authToken} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("admin web TOTP enrollment page exposed %q: %s", disallowed, body)
		}
	}
	csrfToken = adminWebCSRFTokenFromBody(t, body)

	code, err := auth.GenerateTOTPCodeForTest(factor.TOTPSecret, time.Now().UTC())
	if err != nil {
		t.Fatalf("generate TOTP setup code: %v", err)
	}
	response, body = postAdminWebFormWithCookie(t, app, "/admin/second-factor/totp/confirm", url.Values{
		"csrf_token": {csrfToken},
		"code":       {code},
	}, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected TOTP confirmation redirect 303, got %d: %s", response.StatusCode, body)
	}
	if location := response.Header.Get("Location"); location != "/admin?notice=second_factor_setup_complete" {
		t.Fatalf("expected TOTP setup complete notice redirect, got %q", location)
	}

	response, body = requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin", "", nil, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected admin dashboard after TOTP setup status 200, got %d: %s", response.StatusCode, body)
	}
	for _, disallowed := range []string{factor.TOTPSecret, code, app.authToken} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("admin dashboard after TOTP setup exposed %q: %s", disallowed, body)
		}
	}
}

func TestAdminWebRequiresTOTPVerifiedSessionBeforeDashboard(t *testing.T) {
	app := newTestApp(t)
	enrollment := startTOTPEnrollmentForTest(t, app, app.authToken)
	confirmTOTPEnrollmentForTest(t, app, app.authToken, enrollment.Secret, time.Now().UTC())
	cookie := loginAdminWeb(t, app)

	response, body := requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin", "", nil, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected admin web TOTP gate status 403, got %d: %s", response.StatusCode, body)
	}
	for _, expected := range []string{"Verify Admin 2FA", "TOTP code", `action="/admin/second-factor/totp/verify"`} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("admin web TOTP gate missing %q: %s", expected, body)
		}
	}
	for _, disallowed := range []string{"User Accounts", `action="/admin/password"`, enrollment.Secret, app.authToken} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("admin web TOTP gate exposed %q: %s", disallowed, body)
		}
	}
	csrfToken := adminWebCSRFTokenFromBody(t, body)

	codeTime := time.Now().UTC().Add(time.Duration(auth.TOTPDefaultPeriodSeconds) * time.Second)
	code, err := auth.GenerateTOTPCodeForTest(enrollment.Secret, codeTime)
	if err != nil {
		t.Fatalf("generate admin web TOTP code: %v", err)
	}
	response, body = postAdminWebFormWithCookie(t, app, "/admin/second-factor/totp/verify", url.Values{
		"csrf_token": {csrfToken},
		"code":       {code},
	}, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected admin web TOTP redirect 303, got %d: %s", response.StatusCode, body)
	}
	if location := response.Header.Get("Location"); location != "/admin?notice=second_factor_verified" {
		t.Fatalf("expected TOTP verified notice redirect, got %q", location)
	}

	response, body = requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin", "", nil, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected admin dashboard after TOTP status 200, got %d: %s", response.StatusCode, body)
	}
	if !bytes.Contains(body, []byte("Dashboard Overview")) || bytes.Contains(body, []byte(code)) {
		t.Fatalf("expected dashboard without TOTP code exposure: %s", body)
	}
}

func TestAdminWebStaticAssetsAreUnauthenticated(t *testing.T) {
	app := newTestApp(t)

	for _, asset := range []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/admin/static/styles.css", contentType: "text/css", contains: ".admin-shell"},
		{path: "/admin/static/admin.js", contentType: "text/javascript", contains: "data-mobile-nav-details"},
		{path: "/admin/static/proofline-shield-logo.svg", contentType: "image/svg+xml", contains: "<svg"},
		{path: "/admin/static/proofline-p-mark.svg", contentType: "image/svg+xml", contains: "<svg"},
		{path: "/admin/static/favicon.svg", contentType: "image/svg+xml", contains: "<svg"},
		{path: "/admin/static/site.webmanifest", contains: "/admin/static/android-chrome-192x192.png"},
	} {
		response, body := request(t, app.adminHandler, http.MethodGet, asset.path, "", nil)
		response.Body.Close()

		if response.StatusCode != http.StatusOK {
			t.Fatalf("expected admin static %s status 200, got %d: %s", asset.path, response.StatusCode, body)
		}
		if asset.contentType != "" {
			assertContentTypeContains(t, response, asset.contentType)
		}
		assertPublicBrowserSecurityHeaders(t, response)
		if !bytes.Contains(body, []byte(asset.contains)) {
			t.Fatalf("admin static %s did not contain %q: %s", asset.path, asset.contains, body)
		}
	}
}

func TestAdminWebRoutesStayOnPrivateAdminHandler(t *testing.T) {
	app := newTestApp(t)

	response, body := request(t, app.mainHandler, http.MethodGet, "/admin", "", nil)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected main handler /admin status 404, got %d: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)
	assertErrorCode(t, body, "not_found")

	response, body = request(t, app.adminHandler, http.MethodGet, "/admin", "", nil)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected private admin handler /admin status 200, got %d: %s", response.StatusCode, body)
	}
	assertAdminWebPageHeaders(t, response)
	if !bytes.Contains(body, []byte("Admin Login")) {
		t.Fatalf("expected private admin login page: %s", body)
	}
}

func TestV1AdminWebRouteIsNotMounted(t *testing.T) {
	app := newTestApp(t)

	response, body := get(t, app, "/v1/admin")
	defer response.Body.Close()

	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected authenticated /v1/admin status 404, got %d: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)
	assertErrorCode(t, body, "not_found")
}

func postAdminWebForm(t *testing.T, app *testApp, target string, form url.Values) (*http.Response, []byte) {
	t.Helper()

	return request(t, app.adminHandler, http.MethodPost, target, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
}

func postAdminWebFormWithCookie(t *testing.T, app *testApp, target string, form url.Values, cookie *http.Cookie) (*http.Response, []byte) {
	t.Helper()

	return requestWithCookie(t, app.adminHandler, http.MethodPost, target, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()), cookie)
}

func loginAdminWeb(t *testing.T, app *testApp) *http.Cookie {
	t.Helper()

	loginForm := url.Values{
		"username": {"test-admin"},
		"password": {"test-password"},
	}
	response, body := postAdminWebForm(t, app, "/admin/login", loginForm)
	defer response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected admin login redirect 303, got %d: %s", response.StatusCode, body)
	}
	return adminWebCookieFromResponse(t, response)
}

func adminWebDashboardCSRFToken(t *testing.T, app *testApp, cookie *http.Cookie) string {
	t.Helper()

	response, body := requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin", "", nil, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected admin dashboard status 200, got %d: %s", response.StatusCode, body)
	}
	return adminWebCSRFTokenFromBody(t, body)
}

func adminWebCSRFTokenFromBody(t *testing.T, body []byte) string {
	t.Helper()

	marker := []byte(`name="csrf_token" value="`)
	start := bytes.Index(body, marker)
	if start == -1 {
		t.Fatalf("admin dashboard missing CSRF token: %s", body)
	}
	start += len(marker)
	end := bytes.IndexByte(body[start:], '"')
	if end == -1 {
		t.Fatalf("admin dashboard has malformed CSRF token: %s", body)
	}
	token := string(body[start : start+end])
	if token == "" {
		t.Fatal("admin dashboard CSRF token was empty")
	}
	return token
}

func requestWithCookie(t *testing.T, handler http.Handler, method, target, contentType string, body io.Reader, cookie *http.Cookie) (*http.Response, []byte) {
	t.Helper()

	request := newPrivateRequest(t, method, target, contentType, body)
	request.AddCookie(cookie)
	return serve(t, handler, request)
}

func adminWebCookieFromResponse(t *testing.T, response *http.Response) *http.Cookie {
	t.Helper()

	for _, cookie := range response.Cookies() {
		if cookie.Name == adminWebSessionCookieName {
			if cookie.Value == "" {
				t.Fatal("admin web session cookie was empty")
			}
			return cookie
		}
	}
	t.Fatalf("admin web session cookie missing from %q", response.Header.Get("Set-Cookie"))
	return nil
}

func assertAdminWebPageHeaders(t *testing.T, response *http.Response) {
	t.Helper()

	assertPublicBrowserSecurityHeaders(t, response)
	assertNoStore(t, response)
}
