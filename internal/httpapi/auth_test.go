package httpapi_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/email"
	"github.com/open-proofline/server/internal/httpapi"
)

func TestUnauthenticatedPrivateRoutesAreRejected(t *testing.T) {
	app := newTestApp(t)

	response, body := postUnauthenticated(t, app, "/v1/incidents", "application/json", bytes.NewBufferString(`{}`))
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated status 401, got %d: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)
	assertErrorCode(t, body, "authentication_required")
}

func TestJSONBootstrapIsNotMountedOnMainOrAdminHandlers(t *testing.T) {
	app := newTestApp(t)

	for _, handler := range []struct {
		name    string
		handler http.Handler
	}{
		{name: "main", handler: app.mainHandler},
		{name: "admin", handler: app.adminHandler},
	} {
		response, body := request(t, handler.handler, http.MethodPost, "/v1/bootstrap/admin", "application/json", bytes.NewBufferString(`{"username":"Admin.One","password":"test-password"}`))
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("%s handler: expected JSON bootstrap status 404, got %d: %s", handler.name, response.StatusCode, body)
		}
		assertErrorCode(t, body, "not_found")
	}
}

func TestLoginLogoutAndSessionRevocation(t *testing.T) {
	app := newTestApp(t)
	token := loginForTest(t, app, "test-admin", "test-password")

	response, body := requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/account", "", nil, token)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected account status 200 before logout, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/auth/logout", "application/json", bytes.NewBufferString(`{}`), token)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected logout status 200, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/account", "", nil, token)
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected revoked session status 401, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "authentication_required")
}

func TestSessionTokenIsStoredOnlyAsHash(t *testing.T) {
	app := newTestApp(t)
	token := loginForTest(t, app, "test-admin", "test-password")

	var rawMatches int
	if err := app.db.QueryRowContext(context.Background(), `
		SELECT COUNT(*)
		FROM auth_sessions
		WHERE token_hash = ?`,
		token,
	).Scan(&rawMatches); err != nil {
		t.Fatalf("count raw session rows: %v", err)
	}
	if rawMatches != 0 {
		t.Fatalf("raw session token matched %d stored rows", rawMatches)
	}

	var hashLength int
	if err := app.db.QueryRowContext(context.Background(), `
		SELECT length(token_hash)
		FROM auth_sessions
		ORDER BY created_at DESC
		LIMIT 1`,
	).Scan(&hashLength); err != nil {
		t.Fatalf("read session hash length: %v", err)
	}
	if hashLength != 64 {
		t.Fatalf("session token hash length = %d, want 64", hashLength)
	}
}

func TestRegularUserCannotUseAdminRoutes(t *testing.T) {
	app := newTestApp(t)
	userToken := createAccountAndLogin(t, app, "regular-user", "regular-password", auth.RoleUser)

	response, body := requestWithAuth(t, app.mainHandler, http.MethodGet, "/v1/admin/accounts", "", nil, userToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected regular user admin route status 403, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "forbidden")
}

func TestCrossAccountIncidentAccessIsDenied(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "owner-user", "owner-password", auth.RoleUser)
	otherToken := createAccountAndLogin(t, app, "other-user", "other-password", auth.RoleUser)

	response, body := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/incidents", "application/json", bytes.NewBufferString(`{}`), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected owner create incident status 201, got %d: %s", response.StatusCode, body)
	}
	var created struct {
		IncidentID string `json:"incident_id"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode created incident: %v", err)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/incidents/"+created.IncidentID, "", nil, otherToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected cross-account status 403, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "forbidden")
}

func TestAdminCanRevokeAccountSessions(t *testing.T) {
	app := newTestApp(t)
	userToken := createAccountAndLogin(t, app, "session-user", "session-password", auth.RoleUser)
	account := mustGetAccountByUsername(t, app, "session-user")

	response, body := requestWithAuth(t, app.mainHandler, http.MethodPost, "/v1/admin/accounts/"+account.ID+"/sessions/revoke", "application/json", bytes.NewBufferString(`{}`), app.authToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected revoke sessions status 200, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/account", "", nil, userToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected revoked user status 401, got %d: %s", response.StatusCode, body)
	}
}

func TestBrowserLoginSetsCookieWithoutReturningToken(t *testing.T) {
	app := newTestAppWithOptions(t, webAuthTestOptions(true, nil))

	cookie, result := webLoginForTest(t, app, "test-admin", "test-password")
	if cookie.Name != "__Host-proofline_session" {
		t.Fatalf("web session cookie name = %q", cookie.Name)
	}
	if cookie.Value == "" {
		t.Fatal("web session cookie value was empty")
	}
	if cookie.Path != "/" || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Domain != "" {
		t.Fatalf("unexpected web session cookie attributes: %+v", cookie)
	}
	if _, ok := result["token"]; ok {
		t.Fatalf("web login response included token field: %v", result)
	}
	body, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal web login result: %v", err)
	}
	if bytes.Contains(body, []byte(cookie.Value)) {
		t.Fatal("web login response body included the raw cookie session token")
	}

	response, body := requestWithCookie(t, app.privateHandler, http.MethodGet, "/v1/account", "", nil, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected browser cookie account status 200, got %d: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)
}

func TestBrowserLoginCanUseExplicitLocalDevelopmentCookie(t *testing.T) {
	options := webAuthTestOptions(false, []string{"http://127.0.0.1:5173"})
	options.WebAuth.SessionCookieName = "proofline_session"
	app := newTestAppWithOptions(t, options)

	cookie, _ := webLoginForTest(t, app, "test-admin", "test-password")
	if cookie.Name != "proofline_session" {
		t.Fatalf("web session cookie name = %q", cookie.Name)
	}
	if cookie.Secure {
		t.Fatal("local development web session cookie should not be Secure")
	}
	if cookie.Path != "/" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected local development cookie attributes: %+v", cookie)
	}
}

func TestBrowserCookieCSRFAndBearerCompatibility(t *testing.T) {
	app := newTestAppWithOptions(t, webAuthTestOptions(true, nil))
	cookie, _ := webLoginForTest(t, app, "test-admin", "test-password")

	response, body := requestWithCookie(t, app.privateHandler, http.MethodPost, "/v1/incidents", "application/json", bytes.NewBufferString(`{}`), cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected missing CSRF status 403, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "csrf_required")

	response, body = requestWithCookieAndHeaders(t, app.privateHandler, http.MethodPost, "/v1/incidents", "application/json", bytes.NewBufferString(`{}`), cookie, map[string]string{
		"X-CSRF-Token": "not-a-valid-token",
	})
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected invalid CSRF status 403, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "csrf_required")

	csrfToken := webCSRFTokenForTest(t, app, cookie)
	response, body = requestWithCookieAndHeaders(t, app.privateHandler, http.MethodPost, "/v1/incidents", "application/json", bytes.NewBufferString(`{}`), cookie, map[string]string{
		"X-CSRF-Token": csrfToken,
	})
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected valid CSRF incident create status 201, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithCookie(t, app.privateHandler, http.MethodGet, "/v1/account", "", nil, cookie)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected safe cookie-authenticated GET status 200, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/incidents", "application/json", bytes.NewBufferString(`{}`), app.authToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected bearer incident create without CSRF status 201, got %d: %s", response.StatusCode, body)
	}
}

func TestBrowserLogoutRevokesSessionAndClearsCookie(t *testing.T) {
	app := newTestAppWithOptions(t, webAuthTestOptions(true, nil))
	cookie, _ := webLoginForTest(t, app, "test-admin", "test-password")
	csrfToken := webCSRFTokenForTest(t, app, cookie)

	response, body := requestWithCookieAndHeaders(t, app.privateHandler, http.MethodPost, "/v1/auth/web/logout", "application/json", bytes.NewBufferString(`{}`), cookie, map[string]string{
		"X-CSRF-Token": csrfToken,
	})
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected web logout status 200, got %d: %s", response.StatusCode, body)
	}
	if !strings.Contains(response.Header.Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("expected web logout to clear cookie, got %q", response.Header.Get("Set-Cookie"))
	}

	response, body = requestWithCookie(t, app.privateHandler, http.MethodGet, "/v1/account", "", nil, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected revoked cookie session status 401, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "authentication_required")
}

func TestBrowserCookieExpiredSessionFailsClosed(t *testing.T) {
	app := newTestAppWithOptions(t, webAuthTestOptions(true, nil))
	cookie, result := webLoginForTest(t, app, "test-admin", "test-password")
	sessionID, ok := result["session_id"].(string)
	if !ok || sessionID == "" {
		t.Fatalf("web login result missing session_id: %v", result)
	}
	_, err := app.db.ExecContext(context.Background(), `
		UPDATE auth_sessions
		SET expires_at = ?
		WHERE id = ?`,
		time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano),
		sessionID,
	)
	if err != nil {
		t.Fatalf("expire web auth session: %v", err)
	}

	response, body := requestWithCookie(t, app.privateHandler, http.MethodGet, "/v1/account", "", nil, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected expired cookie session status 401, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "authentication_required")

	response, body = requestWithCookie(t, app.privateHandler, http.MethodPost, "/v1/auth/web/logout", "application/json", bytes.NewBufferString(`{}`), cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected stale cookie logout status 200, got %d: %s", response.StatusCode, body)
	}
	if !strings.Contains(response.Header.Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("expected stale cookie logout to clear cookie, got %q", response.Header.Get("Set-Cookie"))
	}
}

func TestPublicRegistrationModesFailClosedWhenDisabledOrPaid(t *testing.T) {
	app := newTestApp(t)

	response, body := postUnauthenticated(t, app, "/v1/auth/register", "application/json", bytes.NewBufferString(`{"username":"new-user","email":"new@example.invalid","password":"valid-password"}`))
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("disabled registration status = %d, want 403: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "registration_disabled")

	adminOnly := newTestAppWithOptions(t, httpapi.Options{
		AccountRegistration: httpapi.AccountRegistrationConfig{Mode: httpapi.AccountRegistrationAdminOnly},
	})
	response, body = postUnauthenticated(t, adminOnly, "/v1/auth/register", "application/json", bytes.NewBufferString(`{"username":"new-user","email":"new@example.invalid","password":"valid-password"}`))
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("admin-only registration status = %d, want 403: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "registration_disabled")

	paid := newTestAppWithOptions(t, httpapi.Options{
		AccountRegistration: httpapi.AccountRegistrationConfig{Mode: httpapi.AccountRegistrationPaid},
	})
	response, body = postUnauthenticated(t, paid, "/v1/auth/register", "application/json", bytes.NewBufferString(`{"username":"paid-user","email":"paid@example.invalid","password":"valid-password"}`))
	response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("paid registration status = %d, want 503: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "registration_payment_unavailable")

	var count int
	if err := paid.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM accounts WHERE username = ?`, "paid-user").Scan(&count); err != nil {
		t.Fatalf("count paid-mode account: %v", err)
	}
	if count != 0 {
		t.Fatalf("paid mode created %d accounts", count)
	}
}

func TestOpenRegistrationRequiresEmailVerificationBeforeLogin(t *testing.T) {
	sender := &recordingEmailSender{}
	app := newTestAppWithOptions(t, httpapi.Options{
		AccountRegistration: httpapi.AccountRegistrationConfig{
			Mode:                 httpapi.AccountRegistrationOpen,
			EmailVerificationTTL: time.Hour,
			PublicWebOrigin:      "https://app.example.invalid",
		},
		EmailSender: sender,
	})

	response, body := postUnauthenticated(t, app, "/v1/auth/register", "application/json", bytes.NewBufferString(`{"username":"New.User","email":" New.User@Example.Invalid ","password":"valid-password"}`))
	response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("open registration status = %d, want 202: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)
	if bytes.Contains(body, []byte("token")) || bytes.Contains(body, []byte("valid-password")) {
		t.Fatalf("registration response exposed token or password: %s", body)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("sent emails = %d, want 1", len(sender.messages))
	}
	if sender.messages[0].To != "new.user@example.invalid" {
		t.Fatalf("verification email to = %q", sender.messages[0].To)
	}
	rawToken := verificationTokenFromEmail(t, sender.messages[0])
	if rawToken == "" {
		t.Fatal("verification email did not contain a token")
	}

	account := mustGetRegistrationAccount(t, app, "new.user")
	if account.EmailNormalized != "new.user@example.invalid" {
		t.Fatalf("account email = %q", account.EmailNormalized)
	}
	if account.AccountState != auth.AccountStatePendingEmailVerification {
		t.Fatalf("account state = %q, want pending email verification", account.AccountState)
	}
	if account.EmailVerifiedAt != nil {
		t.Fatalf("email verified at = %v, want nil", account.EmailVerifiedAt)
	}

	response, body = postUnauthenticated(t, app, "/v1/auth/login", "application/json", bytes.NewBufferString(`{"username":"new.user","password":"valid-password"}`))
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("pending login status = %d, want 403: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "email_verification_required")

	var storedHash string
	if err := app.db.QueryRowContext(context.Background(), `
		SELECT token_hash
		FROM account_verification_tokens
		WHERE account_id = ?`,
		account.ID,
	).Scan(&storedHash); err != nil {
		t.Fatalf("read verification token hash: %v", err)
	}
	if storedHash == rawToken || len(storedHash) != 64 {
		t.Fatalf("verification token was not stored as a 64-character hash")
	}

	response, body = postUnauthenticated(t, app, "/v1/auth/email/verify", "application/json", bytes.NewBufferString(`{"token":"`+rawToken+`"}`))
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("verify email status = %d, want 200: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)

	account = mustGetRegistrationAccount(t, app, "new.user")
	if account.AccountState != auth.AccountStateActive {
		t.Fatalf("account state after verify = %q, want active", account.AccountState)
	}
	if account.EmailVerifiedAt == nil {
		t.Fatal("email_verified_at was not set")
	}

	token := loginForTest(t, app, "new.user", "valid-password")
	if token == "" {
		t.Fatal("verified account did not receive bearer token")
	}

	response, body = postUnauthenticated(t, app, "/v1/auth/email/verify", "application/json", bytes.NewBufferString(`{"token":"`+rawToken+`"}`))
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("reuse verify status = %d, want 400: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "verification_token_invalid")
}

func TestRegistrationDuplicateResponseIsGenericAndResendsPendingVerificationEmail(t *testing.T) {
	sender := &recordingEmailSender{}
	app := newTestAppWithOptions(t, httpapi.Options{
		AccountRegistration: httpapi.AccountRegistrationConfig{
			Mode:                 httpapi.AccountRegistrationOpen,
			EmailVerificationTTL: time.Hour,
			PublicWebOrigin:      "https://app.example.invalid",
		},
		EmailSender: sender,
	})

	requestBody := `{"username":"dup-user","email":"dup@example.invalid","password":"valid-password"}`
	response, body := postUnauthenticated(t, app, "/v1/auth/register", "application/json", bytes.NewBufferString(requestBody))
	response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("first registration status = %d, want 202: %s", response.StatusCode, body)
	}
	response, duplicateBody := postUnauthenticated(t, app, "/v1/auth/register", "application/json", bytes.NewBufferString(requestBody))
	response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("duplicate registration status = %d, want 202: %s", response.StatusCode, duplicateBody)
	}
	if !bytes.Equal(body, duplicateBody) {
		t.Fatalf("duplicate response differed: first=%s duplicate=%s", body, duplicateBody)
	}
	if len(sender.messages) != 2 {
		t.Fatalf("sent emails = %d, want retry verification email", len(sender.messages))
	}
	if firstToken, secondToken := verificationTokenFromEmail(t, sender.messages[0]), verificationTokenFromEmail(t, sender.messages[1]); firstToken == secondToken {
		t.Fatal("duplicate registration reused verification token")
	}
}

func TestRegistrationDuplicateDoesNotSendEmailForActiveAccount(t *testing.T) {
	sender := &recordingEmailSender{}
	app := newTestAppWithOptions(t, httpapi.Options{
		AccountRegistration: httpapi.AccountRegistrationConfig{
			Mode:                 httpapi.AccountRegistrationOpen,
			EmailVerificationTTL: time.Hour,
			PublicWebOrigin:      "https://app.example.invalid",
		},
		EmailSender: sender,
	})
	registerForVerificationTest(t, app, "active-dup", "active-dup@example.invalid")
	rawToken := verificationTokenFromEmail(t, sender.messages[0])
	response, body := postUnauthenticated(t, app, "/v1/auth/email/verify", "application/json", bytes.NewBufferString(`{"token":"`+rawToken+`"}`))
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("verify active duplicate setup status = %d, want 200: %s", response.StatusCode, body)
	}

	response, body = postUnauthenticated(t, app, "/v1/auth/register", "application/json", bytes.NewBufferString(`{"username":"active-dup","email":"active-dup@example.invalid","password":"valid-password"}`))
	response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("active duplicate registration status = %d, want 202: %s", response.StatusCode, body)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("sent emails = %d, want no email for active duplicate", len(sender.messages))
	}
}

func TestRegistrationEmailSendFailureCanBeRetried(t *testing.T) {
	sender := &recordingEmailSender{err: email.ErrDisabled}
	app := newTestAppWithOptions(t, httpapi.Options{
		AccountRegistration: httpapi.AccountRegistrationConfig{
			Mode:                 httpapi.AccountRegistrationOpen,
			EmailVerificationTTL: time.Hour,
			PublicWebOrigin:      "https://app.example.invalid",
		},
		EmailSender: sender,
	})

	requestBody := `{"username":"retry-user","email":"retry@example.invalid","password":"valid-password"}`
	response, body := postUnauthenticated(t, app, "/v1/auth/register", "application/json", bytes.NewBufferString(requestBody))
	response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("failed email registration status = %d, want 202: %s", response.StatusCode, body)
	}
	if len(sender.messages) != 0 {
		t.Fatalf("sent emails after failed send = %d, want 0", len(sender.messages))
	}

	sender.err = nil
	response, body = postUnauthenticated(t, app, "/v1/auth/register", "application/json", bytes.NewBufferString(requestBody))
	response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("retried registration status = %d, want 202: %s", response.StatusCode, body)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("sent emails after retry = %d, want 1", len(sender.messages))
	}
}

func TestRegistrationValidationRejectsInvalidFields(t *testing.T) {
	sender := &recordingEmailSender{}
	app := newTestAppWithOptions(t, httpapi.Options{
		AccountRegistration: httpapi.AccountRegistrationConfig{
			Mode:            httpapi.AccountRegistrationOpen,
			PublicWebOrigin: "https://app.example.invalid",
		},
		EmailSender: sender,
	})

	tests := map[string]string{
		"username": `{"username":"no spaces","email":"new@example.invalid","password":"valid-password"}`,
		"email":    `{"username":"new-user","email":"not-an-address","password":"valid-password"}`,
		"password": `{"username":"new-user","email":"new@example.invalid","password":"short"}`,
	}
	for name, requestBody := range tests {
		t.Run(name, func(t *testing.T) {
			response, body := postUnauthenticated(t, app, "/v1/auth/register", "application/json", bytes.NewBufferString(requestBody))
			response.Body.Close()
			if response.StatusCode != http.StatusBadRequest {
				t.Fatalf("invalid %s status = %d, want 400: %s", name, response.StatusCode, body)
			}
		})
	}
	if len(sender.messages) != 0 {
		t.Fatalf("invalid registration sent %d emails", len(sender.messages))
	}
}

func TestEmailVerificationRejectsInvalidExpiredWrongPurposeAndPathTokens(t *testing.T) {
	sender := &recordingEmailSender{}
	app := newTestAppWithOptions(t, httpapi.Options{
		AccountRegistration: httpapi.AccountRegistrationConfig{
			Mode:                 httpapi.AccountRegistrationOpen,
			EmailVerificationTTL: time.Hour,
			PublicWebOrigin:      "https://app.example.invalid",
		},
		EmailSender: sender,
	})
	registerForVerificationTest(t, app, "expire-user", "expire@example.invalid")
	expiredToken := verificationTokenFromEmail(t, sender.messages[len(sender.messages)-1])
	if _, err := app.db.ExecContext(context.Background(), `UPDATE account_verification_tokens SET expires_at = ? WHERE token_hash = ?`, time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano), auth.SessionTokenHash(expiredToken)); err != nil {
		t.Fatalf("expire verification token: %v", err)
	}
	response, body := postUnauthenticated(t, app, "/v1/auth/email/verify", "application/json", bytes.NewBufferString(`{"token":"`+expiredToken+`"}`))
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expired token status = %d, want 400: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "verification_token_invalid")

	registerForVerificationTest(t, app, "purpose-user", "purpose@example.invalid")
	wrongPurposeToken := verificationTokenFromEmail(t, sender.messages[len(sender.messages)-1])
	if _, err := app.db.ExecContext(context.Background(), `UPDATE account_verification_tokens SET purpose = 'other_purpose' WHERE token_hash = ?`, auth.SessionTokenHash(wrongPurposeToken)); err != nil {
		t.Fatalf("change token purpose: %v", err)
	}
	response, body = postUnauthenticated(t, app, "/v1/auth/email/verify", "application/json", bytes.NewBufferString(`{"token":"`+wrongPurposeToken+`"}`))
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("wrong-purpose token status = %d, want 400: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "verification_token_invalid")

	response, body = postUnauthenticated(t, app, "/v1/auth/email/verify/not-a-token", "application/json", bytes.NewBufferString(`{}`))
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("path token status = %d, want 404: %s", response.StatusCode, body)
	}
}

func TestInactiveAccountStatesCannotAuthenticate(t *testing.T) {
	app := newTestApp(t)
	for _, state := range []string{
		auth.AccountStateDisabled,
		auth.AccountStateSuspended,
		auth.AccountStatePendingPayment,
	} {
		t.Run(state, func(t *testing.T) {
			username := strings.ReplaceAll(state, "_", "-") + "-user"
			createAccountForStateTest(t, app, username, "state-password")
			if _, err := app.db.ExecContext(context.Background(), `UPDATE accounts SET account_state = ? WHERE username = ?`, state, username); err != nil {
				t.Fatalf("set account state: %v", err)
			}
			response, body := postUnauthenticated(t, app, "/v1/auth/login", "application/json", bytes.NewBufferString(`{"username":"`+username+`","password":"state-password"}`))
			response.Body.Close()
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("inactive login status = %d, want 401: %s", response.StatusCode, body)
			}
			assertErrorCode(t, body, "invalid_credentials")
		})
	}
}

func TestAdminCreatedAccountsRemainActiveByDefault(t *testing.T) {
	app := newTestApp(t)

	createAccountForStateTest(t, app, "admin-created-user", "state-password")
	account := mustGetRegistrationAccount(t, app, "admin-created-user")
	if account.AccountState != auth.AccountStateActive {
		t.Fatalf("admin-created account state = %q, want active", account.AccountState)
	}
	if account.EmailNormalized != "" {
		t.Fatalf("admin-created account email = %q, want empty", account.EmailNormalized)
	}
	token := loginForTest(t, app, "admin-created-user", "state-password")
	if token == "" {
		t.Fatal("admin-created active account did not receive bearer token")
	}
}

func TestRegistrationLogsAndResponsesDoNotExposeSecrets(t *testing.T) {
	var logs bytes.Buffer
	sender := &recordingEmailSender{}
	app := newTestAppWithOptions(t, httpapi.Options{
		AccountRegistration: httpapi.AccountRegistrationConfig{
			Mode:                 httpapi.AccountRegistrationOpen,
			EmailVerificationTTL: time.Hour,
			PublicWebOrigin:      "https://app.example.invalid",
		},
		EmailSender: sender,
		Logger:      slog.New(slog.NewTextHandler(&logs, nil)),
	})

	response, body := postUnauthenticated(t, app, "/v1/auth/register", "application/json", bytes.NewBufferString(`{"username":"secret-user","email":"secret@example.invalid","password":"very-secret-password"}`))
	response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("registration status = %d, want 202: %s", response.StatusCode, body)
	}
	rawToken := verificationTokenFromEmail(t, sender.messages[0])
	for _, disallowed := range []string{"secret@example.invalid", "very-secret-password", rawToken} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("registration response exposed %q: %s", disallowed, body)
		}
		if strings.Contains(logs.String(), disallowed) {
			t.Fatalf("registration log exposed %q: %s", disallowed, logs.String())
		}
	}
}

func TestMixedBearerAndBrowserCookieIsRejected(t *testing.T) {
	app := newTestAppWithOptions(t, webAuthTestOptions(true, nil))
	cookie, _ := webLoginForTest(t, app, "test-admin", "test-password")

	response, body := requestWithCookieAndHeaders(t, app.privateHandler, http.MethodGet, "/v1/account", "", nil, cookie, map[string]string{
		"Authorization": "Bearer " + app.authToken,
	})
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected mixed credentials status 400, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "ambiguous_credentials")
}

func TestBrowserCSRFRouteRequiresCookieSession(t *testing.T) {
	app := newTestAppWithOptions(t, webAuthTestOptions(true, nil))

	response, body := requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/auth/web/csrf", "", nil, app.authToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected bearer CSRF route status 400, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "web_cookie_required")
}

func TestCredentialedWebCORS(t *testing.T) {
	app := newTestAppWithOptions(t, webAuthTestOptions(true, []string{"https://web.example.invalid"}))

	headers := map[string]string{
		"Origin":                         "https://web.example.invalid",
		"Access-Control-Request-Method":  "POST",
		"Access-Control-Request-Headers": "Content-Type, X-CSRF-Token, Authorization",
	}
	response, _ := requestWithAuthAndHeaders(t, app.privateHandler, http.MethodOptions, "/v1/auth/web/login", "", nil, "", headers)
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("expected allowed preflight status 204, got %d", response.StatusCode)
	}
	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "https://web.example.invalid" {
		t.Fatalf("allowed origin = %q", got)
	}
	if got := response.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("allow credentials = %q", got)
	}
	if got := response.Header.Get("Access-Control-Allow-Origin"); got == "*" {
		t.Fatal("credentialed CORS must not use wildcard origin")
	}
	if !headerListContains(response.Header.Get("Access-Control-Allow-Headers"), "X-CSRF-Token") {
		t.Fatalf("allowed headers missing CSRF header: %q", response.Header.Get("Access-Control-Allow-Headers"))
	}

	headers["Origin"] = "https://other.example.invalid"
	response, _ = requestWithAuthAndHeaders(t, app.privateHandler, http.MethodOptions, "/v1/auth/web/login", "", nil, "", headers)
	response.Body.Close()
	if response.Header.Get("Access-Control-Allow-Origin") != "" || response.Header.Get("Access-Control-Allow-Credentials") != "" {
		t.Fatalf("disallowed origin received credentialed CORS headers: %v", response.Header)
	}

	response, body := requestWithAuthAndHeaders(t, app.privateHandler, http.MethodGet, "/v1/account", "", nil, "", map[string]string{
		"Origin": "https://web.example.invalid",
	})
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated CORS GET status 401, got %d: %s", response.StatusCode, body)
	}
	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "https://web.example.invalid" {
		t.Fatalf("actual CORS origin = %q", got)
	}
}

func TestWebAuthDisabledAndPublicViewerBoundary(t *testing.T) {
	app := newTestApp(t)

	response, body := postUnauthenticated(t, app, "/v1/auth/web/login", "application/json", bytes.NewBufferString(`{}`))
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected disabled web auth status 404, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "not_found")

	webApp := newTestAppWithOptions(t, webAuthTestOptions(true, nil))
	cookie, _ := webLoginForTest(t, webApp, "test-admin", "test-password")
	response, body = requestWithCookie(t, webApp.publicHandler, http.MethodGet, "/i/not-a-viewer-token", "", nil, cookie)
	defer response.Body.Close()
	if response.StatusCode == http.StatusOK {
		t.Fatalf("public viewer route accepted browser session cookie: %s", body)
	}
}

func loginForTest(t *testing.T, app *testApp, username, password string) string {
	t.Helper()
	requestBody := bytes.NewBufferString(`{"username":"` + username + `","password":"` + password + `"}`)
	response, body := postUnauthenticated(t, app, "/v1/auth/login", "application/json", requestBody)
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected login status 201, got %d: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)
	var result struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if result.Token == "" || !result.ExpiresAt.After(time.Now().UTC()) {
		t.Fatalf("unexpected login response: %+v", result)
	}
	return result.Token
}

func webAuthTestOptions(secure bool, allowedOrigins []string) httpapi.Options {
	return httpapi.Options{
		MaxUploadBytes: 1024 * 1024,
		WebAuth: httpapi.WebAuthConfig{
			Enabled:               true,
			AllowedOrigins:        allowedOrigins,
			SessionCookieName:     "__Host-proofline_session",
			SessionCookieSecure:   secure,
			SessionCookieSameSite: http.SameSiteLaxMode,
			CSRFHeaderName:        "X-CSRF-Token",
		},
	}
}

func webLoginForTest(t *testing.T, app *testApp, username, password string) (*http.Cookie, map[string]any) {
	t.Helper()

	requestBody := bytes.NewBufferString(`{"username":"` + username + `","password":"` + password + `"}`)
	response, body := postUnauthenticated(t, app, "/v1/auth/web/login", "application/json", requestBody)
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected web login status 201, got %d: %s", response.StatusCode, body)
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode web login response: %v", err)
	}
	cookies := response.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one web session cookie, got %d from %q", len(cookies), response.Header.Get("Set-Cookie"))
	}
	return cookies[0], result
}

func webCSRFTokenForTest(t *testing.T, app *testApp, cookie *http.Cookie) string {
	t.Helper()

	response, body := requestWithCookie(t, app.privateHandler, http.MethodGet, "/v1/auth/web/csrf", "", nil, cookie)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected web csrf status 200, got %d: %s", response.StatusCode, body)
	}
	var result struct {
		CSRFToken  string `json:"csrf_token"`
		HeaderName string `json:"header_name"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode web csrf response: %v", err)
	}
	if result.CSRFToken == "" || result.HeaderName != "X-CSRF-Token" {
		t.Fatalf("unexpected web csrf response metadata: header=%q token_empty=%t", result.HeaderName, result.CSRFToken == "")
	}
	return result.CSRFToken
}

func requestWithCookieAndHeaders(t *testing.T, handler http.Handler, method, target, contentType string, body io.Reader, cookie *http.Cookie, headers map[string]string) (*http.Response, []byte) {
	t.Helper()

	request := newPrivateRequest(t, method, target, contentType, body)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	return serve(t, handler, request)
}

func headerListContains(raw, want string) bool {
	for _, part := range strings.Split(raw, ",") {
		if strings.EqualFold(strings.TrimSpace(part), want) {
			return true
		}
	}
	return false
}

func createAccountAndLogin(t *testing.T, app *testApp, username, password, role string) string {
	t.Helper()
	requestBody := bytes.NewBufferString(`{"username":"` + username + `","password":"` + password + `","role":"` + role + `"}`)
	response, body := requestWithAuth(t, app.mainHandler, http.MethodPost, "/v1/admin/accounts", "application/json", requestBody, app.authToken)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create account status 201, got %d: %s", response.StatusCode, body)
	}
	return loginForTest(t, app, username, password)
}

func createAccountForStateTest(t *testing.T, app *testApp, username, password string) {
	t.Helper()
	requestBody := bytes.NewBufferString(`{"username":"` + username + `","password":"` + password + `","role":"user"}`)
	response, body := requestWithAuth(t, app.mainHandler, http.MethodPost, "/v1/admin/accounts", "application/json", requestBody, app.authToken)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create account status 201, got %d: %s", response.StatusCode, body)
	}
}

func registerForVerificationTest(t *testing.T, app *testApp, username, emailAddress string) {
	t.Helper()
	requestBody := `{"username":"` + username + `","email":"` + emailAddress + `","password":"valid-password"}`
	response, body := postUnauthenticated(t, app, "/v1/auth/register", "application/json", bytes.NewBufferString(requestBody))
	response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("expected registration status 202, got %d: %s", response.StatusCode, body)
	}
}

func verificationTokenFromEmail(t *testing.T, message email.Message) string {
	t.Helper()
	const marker = "/verify-email#token="
	index := strings.Index(message.Body, marker)
	if index < 0 {
		t.Fatalf("verification email missing token link: %q", message.Body)
	}
	start := index + len(marker)
	end := strings.IndexAny(message.Body[start:], "\r\n")
	raw := message.Body[start:]
	if end >= 0 {
		raw = message.Body[start : start+end]
	}
	token, err := url.QueryUnescape(strings.TrimSpace(raw))
	if err != nil {
		t.Fatalf("decode verification token: %v", err)
	}
	return token
}

type recordingEmailSender struct {
	messages []email.Message
	err      error
}

func (s *recordingEmailSender) Send(_ context.Context, msg email.Message) error {
	if s.err != nil {
		return s.err
	}
	s.messages = append(s.messages, msg)
	return nil
}

func mustGetAccountByUsername(t *testing.T, app *testApp, username string) auth.Account {
	t.Helper()
	row := app.db.QueryRowContext(context.Background(), `
		SELECT id, username, password_hash, role, created_at, updated_at, password_changed_at
		FROM accounts
		WHERE username = ?`,
		username,
	)
	var account auth.Account
	var createdAt, updatedAt, passwordChangedAt string
	if err := row.Scan(&account.ID, &account.Username, &account.PasswordHash, &account.Role, &createdAt, &updatedAt, &passwordChangedAt); err != nil {
		t.Fatalf("read account %s: %v", username, err)
	}
	return account
}

func mustGetRegistrationAccount(t *testing.T, app *testApp, username string) auth.Account {
	t.Helper()
	row := app.db.QueryRowContext(context.Background(), `
		SELECT id, username, email_normalized, email_verified_at, account_state, password_hash, role, created_at, updated_at, password_changed_at
		FROM accounts
		WHERE username = ?`,
		username,
	)
	var account auth.Account
	var emailNormalized sql.NullString
	var emailVerifiedAt sql.NullString
	var createdAt, updatedAt, passwordChangedAt string
	if err := row.Scan(&account.ID, &account.Username, &emailNormalized, &emailVerifiedAt, &account.AccountState, &account.PasswordHash, &account.Role, &createdAt, &updatedAt, &passwordChangedAt); err != nil {
		t.Fatalf("read registration account %s: %v", username, err)
	}
	if emailNormalized.Valid {
		account.EmailNormalized = emailNormalized.String
	}
	var err error
	if account.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt); err != nil {
		t.Fatalf("parse created_at: %v", err)
	}
	if account.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt); err != nil {
		t.Fatalf("parse updated_at: %v", err)
	}
	if account.PasswordChangedAt, err = time.Parse(time.RFC3339Nano, passwordChangedAt); err != nil {
		t.Fatalf("parse password_changed_at: %v", err)
	}
	if emailVerifiedAt.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, emailVerifiedAt.String)
		if err != nil {
			t.Fatalf("parse email_verified_at: %v", err)
		}
		account.EmailVerifiedAt = &parsed
	}
	return account
}

func newPrivateRequest(t *testing.T, method, target, contentType string, body io.Reader) *http.Request {
	t.Helper()
	request := httptest.NewRequest(method, target, body)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	return request
}

func serve(t *testing.T, handler http.Handler, request *http.Request) (*http.Response, []byte) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	response := recorder.Result()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return response, body
}
