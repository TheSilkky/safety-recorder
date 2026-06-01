package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/auth"
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
