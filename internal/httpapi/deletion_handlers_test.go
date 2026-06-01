package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/incidents"
)

func TestAccountIncidentDeletionRequiresOwnerAndHidesPublicViewer(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "delete-owner", "owner-password", auth.RoleUser)
	otherToken := createAccountAndLogin(t, app, "delete-other", "other-password", auth.RoleUser)
	owner := mustGetAccountByUsername(t, app, "delete-owner")
	incidentID := createIncidentWithAuth(t, app, ownerToken, `{"client_label":"phone"}`)
	viewerToken := createIncidentTokenWithAuth(t, app, ownerToken, incidentID, "trusted contact")

	response, body := requestWithAuth(
		t,
		app.mainHandler,
		http.MethodPost,
		"/v1/incidents/"+incidentID+"/deletion",
		"application/json",
		bytes.NewBufferString(`{"allow_open":true}`),
		otherToken,
	)
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected other account deletion status 403, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithAuth(
		t,
		app.mainHandler,
		http.MethodPost,
		"/v1/incidents/"+incidentID+"/deletion",
		"application/json",
		bytes.NewBufferString(`{"reason_code":"account_delete","allow_open":true}`),
		ownerToken,
	)
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("expected owner deletion status 202, got %d: %s", response.StatusCode, body)
	}
	status := decodeDeletionResponse(t, body)
	if status.Source != incidents.IncidentDeletionSourceAccountRequest ||
		status.ActorAccountID != owner.ID ||
		status.State != incidents.IncidentDeletionStatePending ||
		status.ReasonCode != "account_delete" {
		t.Fatalf("unexpected owner deletion status: %+v", status)
	}

	response, body = getPublic(t, app, "/i/"+viewerToken.Token+"/data")
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected public viewer token to fail closed, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "incident_token_invalid")
}

func TestAdminIncidentDeletionCanTargetAnyIncident(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "admin-delete-owner", "owner-password", auth.RoleUser)
	incidentID := createIncidentWithAuth(t, app, ownerToken, `{}`)

	response, body := post(t, app, "/v1/incidents/"+incidentID+"/deletion", "application/json", bytes.NewBufferString(`{"allow_open":true}`))
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected admin account-scoped deletion status 403 for another account incident, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithAuth(
		t,
		app.mainHandler,
		http.MethodPost,
		"/v1/admin/incidents/"+incidentID+"/deletion",
		"application/json",
		bytes.NewBufferString(`{"allow_open":true}`),
		ownerToken,
	)
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected non-admin global deletion status 403, got %d: %s", response.StatusCode, body)
	}

	admin := mustGetAccountByUsername(t, app, "test-admin")
	response, body = requestWithAuth(t, app.mainHandler, http.MethodPost, "/v1/admin/incidents/"+incidentID+"/deletion", "application/json", bytes.NewBufferString(`{"reason_code":"admin_delete","allow_open":true}`), app.authToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("expected admin deletion status 202, got %d: %s", response.StatusCode, body)
	}
	status := decodeDeletionResponse(t, body)
	if status.Source != incidents.IncidentDeletionSourceAdminRequest ||
		status.ActorAccountID != admin.ID ||
		status.State != incidents.IncidentDeletionStatePending ||
		status.ReasonCode != "admin_delete" {
		t.Fatalf("unexpected admin deletion status: %+v", status)
	}
}

func TestAccountIncidentDeletionRejectsOpenIncidentWithoutAllowOpen(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)

	response, body := post(t, app, "/v1/incidents/"+incidentID+"/deletion", "application/json", bytes.NewBufferString(`{}`))
	defer response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected open deletion status 409, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "incident_deletion_invalid_state")
}

func createIncidentWithAuth(t *testing.T, app *testApp, token, requestBody string) string {
	t.Helper()
	response, body := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/incidents", "application/json", bytes.NewBufferString(requestBody), token)
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create incident status 201, got %d: %s", response.StatusCode, body)
	}
	var result struct {
		IncidentID string `json:"incident_id"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode create incident response: %v", err)
	}
	if result.IncidentID == "" || result.Status != incidents.StatusOpen {
		t.Fatalf("unexpected create incident response: %+v", result)
	}
	return result.IncidentID
}

func createIncidentTokenWithAuth(t *testing.T, app *testApp, token, incidentID, label string) incidentTokenResponse {
	t.Helper()
	requestBody, err := json.Marshal(struct {
		Label string `json:"label"`
	}{Label: label})
	if err != nil {
		t.Fatalf("marshal incident token request: %v", err)
	}
	response, body := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/incidents/"+incidentID+"/incident-tokens", "application/json", bytes.NewReader(requestBody), token)
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create incident token status 201, got %d: %s", response.StatusCode, body)
	}
	var result incidentTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode create incident token response: %v", err)
	}
	if result.Token == "" {
		t.Fatal("raw incident token was empty")
	}
	return result
}

func decodeDeletionResponse(t *testing.T, body []byte) incidents.IncidentDeletionStatus {
	t.Helper()
	var result struct {
		Deletion incidents.IncidentDeletionStatus `json:"deletion"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode deletion response: %v", err)
	}
	if result.Deletion.DecisionID == "" {
		t.Fatalf("missing deletion decision: %s", body)
	}
	return result.Deletion
}
