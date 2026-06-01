package httpapi_test

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/httpapi"
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
	for _, expected := range []string{"Proofline Admin", "Admin session", "Private /admin", "Public viewer", "Not mounted"} {
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
	cookie := loginAdminWeb(t, app)

	response, body := requestWithCookie(t, app.adminHandler, http.MethodGet, "/admin", "", nil, cookie)
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected admin dashboard status 200, got %d: %s", response.StatusCode, body)
	}
	for _, expected := range []string{
		"Admin Password",
		"User Accounts",
		"test-admin",
		"managed-user",
		`action="/admin/password"`,
		`action="/admin/accounts/` + userAccount.ID + `/password"`,
		`name="csrf_token"`,
	} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Fatalf("admin dashboard missing %q: %s", expected, body)
		}
	}
	for _, disallowed := range []string{app.authToken, "test-password", "managed-password", "password_hash", "Authorization"} {
		if bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("admin dashboard exposed %q: %s", disallowed, body)
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
	if location := response.Header.Get("Location"); location != "/admin?notice=password_changed" {
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
	if location := response.Header.Get("Location"); location != "/admin?notice=account_password_reset" {
		t.Fatalf("expected account password reset redirect notice, got %q", location)
	}

	response, body = requestWithAuth(t, app.mainHandler, http.MethodGet, "/v1/account", "", nil, userToken)
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected reset user session to be revoked, got %d: %s", response.StatusCode, body)
	}
	loginForTest(t, app, "reset-user", "replacement-password")
}

func TestAdminWebPasswordFormsRequireCSRFToken(t *testing.T) {
	app := newTestApp(t)
	cookie := loginAdminWeb(t, app)

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
	if response.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("User Accounts")) {
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
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected bootstrapped dashboard status 200, got %d: %s", response.StatusCode, body)
	}
	if !bytes.Contains(body, []byte("Private /admin")) {
		t.Fatalf("expected bootstrapped dashboard: %s", body)
	}
}

func TestAdminWebStaticAssetsAreUnauthenticated(t *testing.T) {
	app := newTestApp(t)

	response, body := request(t, app.adminHandler, http.MethodGet, "/admin/static/styles.css", "", nil)
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected admin static status 200, got %d: %s", response.StatusCode, body)
	}
	assertContentTypeContains(t, response, "text/css")
	assertPublicBrowserSecurityHeaders(t, response)
	if !bytes.Contains(body, []byte(".admin-shell")) {
		t.Fatalf("admin static CSS did not contain expected admin styles: %s", body)
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
