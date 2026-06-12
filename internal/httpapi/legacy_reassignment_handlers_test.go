package httpapi_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/incidents"
)

func TestAdminListsLegacyUnownedIncidentCandidatesWithSafeFields(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "candidate-owner", "owner-password", auth.RoleUser)
	_ = createIncidentWithAuth(t, app, ownerToken, `{"client_label":"owned phone"}`)
	legacyIncident := createLegacyIncidentForTest(t, app, "legacy phone", "legacy private note")

	stream := createMediaStream(t, app, legacyIncident.ID, incidents.MediaTypeAudio, "legacy stream")
	payload := []byte("encrypted audio bytes")
	response, body := uploadChunkWithStream(t, app, legacyIncident.ID, stream.ID, 1, incidents.MediaTypeAudio, payload, sha256Hex(payload))
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected chunk upload status 201, got %d: %s", response.StatusCode, body)
	}
	createCheckin(t, app, legacyIncident.ID)
	createIncidentToken(t, app, legacyIncident.ID, "viewer", nil)

	response, body = requestWithAuth(t, app.adminHandler, http.MethodGet, "/admin/api/incidents/unowned", "", nil, app.authToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected candidate list status 200, got %d: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)

	var result struct {
		Incidents []incidents.LegacyUnownedIncidentCandidate `json:"incidents"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode candidate list: %v", err)
	}
	if len(result.Incidents) != 1 {
		t.Fatalf("expected one legacy candidate, got %+v", result.Incidents)
	}
	candidate := result.Incidents[0]
	if candidate.IncidentID != legacyIncident.ID ||
		candidate.Status != incidents.StatusOpen ||
		candidate.DeletionState != incidents.IncidentDeletionStateActive ||
		candidate.StreamCount != 1 ||
		candidate.ChunkCount != 1 ||
		candidate.CheckinCount != 1 ||
		candidate.IncidentTokenCount != 1 ||
		!candidate.HasActiveViewerTokens {
		t.Fatalf("unexpected legacy candidate: %+v", candidate)
	}
	for _, disallowed := range []string{
		"legacy private note",
		"chunk.enc",
		"stored_path",
		"owner_account_id",
		"latitude",
		"longitude",
		"raw-session-token-secret",
	} {
		if strings.Contains(string(body), disallowed) {
			t.Fatalf("candidate list exposed %q: %s", disallowed, body)
		}
	}
}

func TestAdminLegacyUnownedIncidentReassignmentAssignsOwner(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "legacy-owner", "owner-password", auth.RoleUser)
	owner := getAccountByUsernameForTest(t, app, "legacy-owner")
	otherToken := createAccountAndLogin(t, app, "legacy-other", "other-password", auth.RoleUser)
	legacyIncident := createLegacyIncidentForTest(t, app, "legacy", "legacy note")
	token := createIncidentToken(t, app, legacyIncident.ID, "viewer", nil)

	response, body := getPublic(t, app, "/i/"+token.Token+"/data")
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected public viewer data before reassignment status 200, got %d: %s", response.StatusCode, body)
	}

	requestBody := bytes.NewBufferString(`{"action":"assign_owner","new_owner_account_id":"` + owner.ID + `","reason_code":"owner_verified"}`)
	response, body = requestWithAuth(t, app.adminHandler, http.MethodPost, "/admin/api/incidents/"+legacyIncident.ID+"/reassignment", "application/json", requestBody, app.authToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected reassignment status 200, got %d: %s", response.StatusCode, body)
	}

	var result struct {
		Event incidents.LegacyIncidentReassignmentEvent `json:"event"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode reassignment event: %v", err)
	}
	if result.Event.IncidentID != legacyIncident.ID ||
		result.Event.NewOwnerAccountID != owner.ID ||
		result.Event.ActorAccountID == "" ||
		result.Event.Action != incidents.LegacyIncidentReassignmentActionAssignOwner ||
		result.Event.ReasonCode != "owner_verified" ||
		result.Event.Source != incidents.LegacyIncidentReassignmentSourceAdminAPI {
		t.Fatalf("unexpected reassignment event: %+v", result.Event)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/incidents/"+legacyIncident.ID, "", nil, ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected owner incident detail after reassignment status 200, got %d: %s", response.StatusCode, body)
	}
	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/incidents/"+legacyIncident.ID, "", nil, otherToken)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected other account incident detail status 404, got %d: %s", response.StatusCode, body)
	}
	response, body = getPublic(t, app, "/i/"+token.Token+"/data")
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected public viewer data after reassignment status 200, got %d: %s", response.StatusCode, body)
	}
	assertLegacyReassignmentAuditRow(t, app, legacyIncident.ID, incidents.LegacyIncidentReassignmentActionAssignOwner, owner.ID)
}

func TestAdminLegacyUnownedIncidentReassignmentKeepsUnowned(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "quarantine-owner", "owner-password", auth.RoleUser)
	legacyIncident := createLegacyIncidentForTest(t, app, "legacy", "legacy note")

	requestBody := bytes.NewBufferString(`{"action":"keep_unowned","reason_code":"keep_admin_only"}`)
	response, body := requestWithAuth(t, app.adminHandler, http.MethodPost, "/admin/api/incidents/"+legacyIncident.ID+"/reassignment", "application/json", requestBody, app.authToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected keep-unowned status 200, got %d: %s", response.StatusCode, body)
	}
	var result struct {
		Event incidents.LegacyIncidentReassignmentEvent `json:"event"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode keep-unowned event: %v", err)
	}
	if result.Event.NewOwnerAccountID != "" ||
		result.Event.Action != incidents.LegacyIncidentReassignmentActionKeepUnowned ||
		result.Event.ReasonCode != "keep_admin_only" {
		t.Fatalf("unexpected keep-unowned event: %+v", result.Event)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/incidents/"+legacyIncident.ID, "", nil, ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected unowned incident to stay hidden from regular account, got %d: %s", response.StatusCode, body)
	}
	assertLegacyReassignmentAuditRow(t, app, legacyIncident.ID, incidents.LegacyIncidentReassignmentActionKeepUnowned, "")
}

func TestAdminLegacyUnownedIncidentReassignmentRejectsInvalidRequests(t *testing.T) {
	app := newTestApp(t)
	owner := getAccountByUsernameForTest(t, app, "test-admin")
	userToken := createAccountAndLogin(t, app, "reassign-user", "user-password", auth.RoleUser)
	legacyIncident := createLegacyIncidentForTest(t, app, "legacy", "legacy note")
	ownedIncidentID := createIncident(t, app, `{}`)

	tests := []struct {
		name       string
		token      string
		incidentID string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "regular user denied",
			token:      userToken,
			incidentID: legacyIncident.ID,
			body:       `{"action":"keep_unowned","reason_code":"operator_review"}`,
			wantStatus: http.StatusForbidden,
			wantCode:   "forbidden",
		},
		{
			name:       "missing destination account",
			token:      app.authToken,
			incidentID: legacyIncident.ID,
			body:       `{"action":"assign_owner","new_owner_account_id":"acct_missing","reason_code":"owner_verified"}`,
			wantStatus: http.StatusNotFound,
			wantCode:   "account_not_found",
		},
		{
			name:       "invalid reason",
			token:      app.authToken,
			incidentID: legacyIncident.ID,
			body:       `{"action":"keep_unowned","reason_code":"private note text"}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_reason_code",
		},
		{
			name:       "already owned incident",
			token:      app.authToken,
			incidentID: ownedIncidentID,
			body:       `{"action":"assign_owner","new_owner_account_id":"` + owner.ID + `","reason_code":"owner_verified"}`,
			wantStatus: http.StatusConflict,
			wantCode:   "incident_not_eligible",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response, body := requestWithAuth(t, app.adminHandler, http.MethodPost, "/admin/api/incidents/"+test.incidentID+"/reassignment", "application/json", bytes.NewBufferString(test.body), test.token)
			defer response.Body.Close()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", test.wantStatus, response.StatusCode, body)
			}
			assertErrorCode(t, body, test.wantCode)
		})
	}
}

func createLegacyIncidentForTest(t *testing.T, app *testApp, clientLabel, notes string) incidents.Incident {
	t.Helper()

	legacyIncident, err := incidents.NewRepository(app.db).CreateIncident(context.Background(), clientLabel, notes)
	if err != nil {
		t.Fatalf("create legacy incident: %v", err)
	}
	return legacyIncident
}

func getAccountByUsernameForTest(t *testing.T, app *testApp, username string) auth.Account {
	t.Helper()

	account, err := incidents.NewRepository(app.db).GetAccountByUsername(context.Background(), auth.NormalizeUsername(username))
	if err != nil {
		t.Fatalf("get account by username: %v", err)
	}
	return account
}

func assertLegacyReassignmentAuditRow(t *testing.T, app *testApp, incidentID, action, newOwnerAccountID string) {
	t.Helper()

	var gotAction string
	var gotNewOwnerAccountID sql.NullString
	var reasonCode string
	var source string
	var createdAt string
	var completedAt string
	if err := app.db.QueryRowContext(context.Background(), `
		SELECT action, new_owner_account_id, reason_code, source, created_at, completed_at
		FROM legacy_incident_reassignment_events
		WHERE incident_id = ?
		ORDER BY created_at DESC
		LIMIT 1`,
		incidentID,
	).Scan(&gotAction, &gotNewOwnerAccountID, &reasonCode, &source, &createdAt, &completedAt); err != nil {
		t.Fatalf("read legacy reassignment audit row: %v", err)
	}
	if gotAction != action {
		t.Fatalf("audit action = %q, want %q", gotAction, action)
	}
	if gotNewOwnerAccountID.String != newOwnerAccountID || gotNewOwnerAccountID.Valid != (newOwnerAccountID != "") {
		t.Fatalf("audit new owner = %+v, want %q", gotNewOwnerAccountID, newOwnerAccountID)
	}
	if reasonCode == "" || source != incidents.LegacyIncidentReassignmentSourceAdminAPI {
		t.Fatalf("unexpected audit metadata reason=%q source=%q", reasonCode, source)
	}
	if _, err := time.Parse(time.RFC3339Nano, createdAt); err != nil {
		t.Fatalf("created_at is not RFC3339Nano: %v", err)
	}
	if _, err := time.Parse(time.RFC3339Nano, completedAt); err != nil {
		t.Fatalf("completed_at is not RFC3339Nano: %v", err)
	}
}
