package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/auth"
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
