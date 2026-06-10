package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/incidents"
)

type accountIncidentResponseForTest struct {
	ID               string `json:"id"`
	Status           string `json:"status"`
	ClientLabel      string `json:"client_label"`
	IncidentMode     string `json:"incident_mode"`
	CaptureProfile   string `json:"capture_profile"`
	EscalationPolicy string `json:"escalation_policy"`
	SharingState     string `json:"sharing_state"`
	DeletionState    string `json:"deletion_state"`
}

func TestCreateIncident(t *testing.T) {
	app := newTestApp(t)

	incidentID := createIncident(t, app, `{"client_label":"phone","notes":"test"}`)

	if incidentID == "" {
		t.Fatal("expected incident id")
	}
}

func TestCreateIncidentWithModeFields(t *testing.T) {
	app := newTestApp(t)
	requestBody := bytes.NewBufferString(`{
		"client_label":"phone",
		"notes":"test",
		"incident_mode":"interaction_record",
		"capture_profile":"audio_location",
		"escalation_policy":"none",
		"sharing_state":"private"
	}`)

	response, body := post(t, app, "/v1/incidents", "application/json", requestBody)
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create incident status 201, got %d: %s", response.StatusCode, body)
	}
	var created struct {
		IncidentID       string `json:"incident_id"`
		Status           string `json:"status"`
		IncidentMode     string `json:"incident_mode"`
		CaptureProfile   string `json:"capture_profile"`
		EscalationPolicy string `json:"escalation_policy"`
		SharingState     string `json:"sharing_state"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode create incident response: %v", err)
	}
	if created.IncidentMode != incidents.IncidentModeInteractionRecord ||
		created.CaptureProfile != incidents.CaptureProfileAudioLocation ||
		created.EscalationPolicy != incidents.EscalationPolicyNone ||
		created.SharingState != incidents.SharingStatePrivate {
		t.Fatalf("create response did not include mode fields: %+v", created)
	}

	response, body = get(t, app, "/v1/incidents/"+created.IncidentID)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected incident status 200, got %d: %s", response.StatusCode, body)
	}
	var detail struct {
		Incident accountIncidentResponseForTest `json:"incident"`
	}
	if err := json.Unmarshal(body, &detail); err != nil {
		t.Fatalf("decode incident detail: %v", err)
	}
	if detail.Incident.IncidentMode != incidents.IncidentModeInteractionRecord ||
		detail.Incident.CaptureProfile != incidents.CaptureProfileAudioLocation ||
		detail.Incident.EscalationPolicy != incidents.EscalationPolicyNone ||
		detail.Incident.SharingState != incidents.SharingStatePrivate {
		t.Fatalf("get incident did not return mode fields: %+v", detail.Incident)
	}
}

func TestCreateIncidentRejectsInvalidModeFields(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "incident mode",
			body:    `{"incident_mode":"urgent"}`,
			wantErr: "invalid_incident_mode",
		},
		{
			name:    "capture profile",
			body:    `{"capture_profile":"all_the_things"}`,
			wantErr: "invalid_capture_profile",
		},
		{
			name:    "escalation policy",
			body:    `{"escalation_policy":"call_police"}`,
			wantErr: "invalid_escalation_policy",
		},
		{
			name:    "sharing state",
			body:    `{"sharing_state":"shared_with_everyone"}`,
			wantErr: "invalid_sharing_state",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := newTestApp(t)

			response, body := post(t, app, "/v1/incidents", "application/json", bytes.NewBufferString(test.body))
			defer response.Body.Close()

			if response.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d: %s", response.StatusCode, body)
			}
			assertErrorCode(t, body, test.wantErr)
		})
	}
}

func TestMainAPIJSONSecurityHeaders(t *testing.T) {
	app := newTestApp(t)

	response, body := post(t, app, "/v1/incidents", "application/json", bytes.NewBufferString(`{}`))
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create incident status 201, got %d: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)
}

func TestMainAPIErrorSecurityHeaders(t *testing.T) {
	app := newTestApp(t)

	response, body := get(t, app, "/v1/incidents/inc_missing")
	defer response.Body.Close()

	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected missing incident status 404, got %d: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)
	assertErrorCode(t, body, "incident_not_found")
}

func TestMainAPIUnsupportedMethodUsesSecurityHeaders(t *testing.T) {
	app := newTestApp(t)

	response, body := get(t, app, "/v1/incidents")
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected list incidents status 200, got %d: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)
	if !bytes.Contains(body, []byte(`"incidents":[]`)) {
		t.Fatalf("expected empty incident list, got: %s", body)
	}
}

func TestListAccountIncidentsReturnsOnlyOwnedPublicSafeMetadata(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "list-owner", "owner-password", auth.RoleUser)
	otherToken := createAccountAndLogin(t, app, "list-other", "other-password", auth.RoleUser)
	ownerIncidentID := createIncidentWithAuth(t, app, ownerToken, `{
		"client_label":"owner phone",
		"notes":"owner private note",
		"incident_mode":"interaction_record",
		"capture_profile":"audio_location",
		"escalation_policy":"none",
		"sharing_state":"private"
	}`)
	_ = createIncidentWithAuth(t, app, otherToken, `{"client_label":"other phone"}`)
	legacyIncident, err := incidents.NewRepository(app.db).CreateIncident(context.Background(), "legacy", "legacy note")
	if err != nil {
		t.Fatalf("create legacy incident: %v", err)
	}

	stream := createMediaStream(t, app, ownerIncidentID, incidents.MediaTypeMetadata, "metadata")
	payload := []byte("encrypted metadata")
	response, body := uploadChunkWithStream(t, app, ownerIncidentID, stream.ID, 2, "metadata", payload, sha256Hex(payload))
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected upload status 201, got %d: %s", response.StatusCode, body)
	}
	createCheckin(t, app, ownerIncidentID)

	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/incidents", "", nil, ownerToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected list incidents status 200, got %d: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)

	var result struct {
		Incidents []accountIncidentResponseForTest `json:"incidents"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode incident list: %v", err)
	}
	if len(result.Incidents) != 1 {
		t.Fatalf("expected one owned incident, got %+v", result.Incidents)
	}
	got := result.Incidents[0]
	if got.ID != ownerIncidentID ||
		got.ClientLabel != "owner phone" ||
		got.IncidentMode != incidents.IncidentModeInteractionRecord ||
		got.CaptureProfile != incidents.CaptureProfileAudioLocation ||
		got.EscalationPolicy != incidents.EscalationPolicyNone ||
		got.SharingState != incidents.SharingStatePrivate ||
		got.DeletionState != incidents.IncidentDeletionStateActive {
		t.Fatalf("unexpected list incident response: %+v", got)
	}
	for _, disallowed := range [][]byte{
		[]byte(`"owner_account_id"`),
		[]byte(`"notes"`),
		[]byte(`"chunks"`),
		[]byte(`"checkins"`),
		[]byte(`"stored_path"`),
		[]byte(`"latitude"`),
		[]byte("owner private note"),
		[]byte(legacyIncident.ID),
	} {
		if bytes.Contains(body, disallowed) {
			t.Fatalf("incident list exposed %q: %s", disallowed, body)
		}
	}
}

func TestGetAccountIncidentReturnsPublicSafeMetadata(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{
		"client_label":"phone",
		"notes":"private note",
		"incident_mode":"interaction_record",
		"capture_profile":"audio_location",
		"escalation_policy":"none",
		"sharing_state":"private"
	}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeMetadata, "metadata")
	payload := []byte("encrypted metadata")
	response, body := uploadChunkWithStream(t, app, incidentID, stream.ID, 2, "metadata", payload, sha256Hex(payload))
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected upload status 201, got %d: %s", response.StatusCode, body)
	}
	createCheckin(t, app, incidentID)

	response, body = get(t, app, "/v1/incidents/"+incidentID)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected incident status 200, got %d: %s", response.StatusCode, body)
	}

	var detail struct {
		Incident accountIncidentResponseForTest `json:"incident"`
	}
	if err := json.Unmarshal(body, &detail); err != nil {
		t.Fatalf("decode incident detail: %v", err)
	}
	if detail.Incident.ID != incidentID ||
		detail.Incident.ClientLabel != "phone" ||
		detail.Incident.IncidentMode != incidents.IncidentModeInteractionRecord ||
		detail.Incident.CaptureProfile != incidents.CaptureProfileAudioLocation ||
		detail.Incident.EscalationPolicy != incidents.EscalationPolicyNone ||
		detail.Incident.SharingState != incidents.SharingStatePrivate {
		t.Fatalf("unexpected account incident detail: %+v", detail.Incident)
	}
	for _, disallowed := range [][]byte{
		[]byte(`"owner_account_id"`),
		[]byte(`"notes"`),
		[]byte(`"chunks"`),
		[]byte(`"checkins"`),
		[]byte(`"stored_path"`),
		[]byte(`"latitude"`),
		[]byte("private note"),
	} {
		if bytes.Contains(body, disallowed) {
			t.Fatalf("account incident detail exposed %q: %s", disallowed, body)
		}
	}
}

func TestCloseIncident(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)

	response, body := post(t, app, "/v1/incidents/"+incidentID+"/close", "application/json", bytes.NewBufferString(`{}`))
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected close status 200, got %d: %s", response.StatusCode, body)
	}
	var incident incidents.Incident
	if err := json.Unmarshal(body, &incident); err != nil {
		t.Fatalf("decode incident: %v", err)
	}
	if incident.Status != incidents.StatusClosed {
		t.Fatalf("expected closed incident, got %+v", incident)
	}
}

func TestRejectUploadAfterClose(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	response, body := post(t, app, "/v1/incidents/"+incidentID+"/close", "application/json", bytes.NewBufferString(`{}`))
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected close status 200, got %d: %s", response.StatusCode, body)
	}

	payload := []byte("encrypted audio data")
	response, body = uploadChunk(t, app, incidentID, 1, "audio", payload, sha256Hex(payload))
	defer response.Body.Close()

	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected upload after close status 409, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "incident_closed")
}

func TestAccountIncidentDetailHidesCrossAccountAndLegacyIncidents(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "detail-owner", "owner-password", auth.RoleUser)
	otherToken := createAccountAndLogin(t, app, "detail-other", "other-password", auth.RoleUser)
	incidentID := createIncidentWithAuth(t, app, ownerToken, `{}`)
	legacyIncident, err := incidents.NewRepository(app.db).CreateIncident(context.Background(), "legacy", "legacy note")
	if err != nil {
		t.Fatalf("create legacy incident: %v", err)
	}

	response, body := requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/incidents/"+incidentID, "", nil, otherToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected cross-account detail status 404, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "incident_not_found")

	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/incidents/"+legacyIncident.ID, "", nil, ownerToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected legacy detail status 404, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "incident_not_found")
}
