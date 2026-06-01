package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/incidents"
)

func TestContactPublicKeyAndSharingGrantRoutesAreOwnerScoped(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "sharing-owner", "owner-password", auth.RoleUser)
	otherToken := createAccountAndLogin(t, app, "sharing-other", "other-password", auth.RoleUser)
	incidentID := createIncidentWithToken(t, app, ownerToken)
	stream := createMediaStreamWithToken(t, app, ownerToken, incidentID, incidents.MediaTypeAudio, "owner audio")
	contactKey := createContactPublicKeyWithToken(t, app, ownerToken, `{
		"display_label":"Trusted contact",
		"wrapping_algorithm":"age-v1-x25519",
		"public_key":"age1examplepublickey",
		"public_key_fingerprint":"fingerprint-1",
		"key_state":"active"
	}`)

	response, body := requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/contact-public-keys", "", nil, ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected owner list contact keys status 200, got %d: %s", response.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(contactKey.ID)) || !bytes.Contains(body, []byte(contactKey.ContactID)) {
		t.Fatalf("owner contact key list missing created key: %s", body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/contact-public-keys/"+contactKey.ID, "", nil, otherToken)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected other account contact key status 404, got %d: %s", response.StatusCode, body)
	}

	grantBody := `{"stream_id":"` + stream.ID + `","contact_id":"` + contactKey.ContactID + `"}`
	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/incidents/"+incidentID+"/sharing-grants", "application/json", bytes.NewBufferString(grantBody), otherToken)
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected other account sharing grant status 403, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/incidents/"+incidentID+"/sharing-grants", "application/json", bytes.NewBufferString(grantBody), app.authToken)
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected admin non-owner sharing grant status 403, got %d: %s", response.StatusCode, body)
	}

	grant := createSharingGrantWithToken(t, app, ownerToken, incidentID, grantBody)
	if grant.StreamID != stream.ID || grant.ContactPublicKeyID != contactKey.ID || grant.ContactPublicKeyVersion != 1 {
		t.Fatalf("unexpected sharing grant: %+v", grant)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/sharing-grants/"+grant.ID+"/revoke", "application/json", bytes.NewBufferString(`{}`), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected revoke grant status 200, got %d: %s", response.StatusCode, body)
	}
	var revoked struct {
		SharingGrant incidents.SharingGrant `json:"sharing_grant"`
	}
	if err := json.Unmarshal(body, &revoked); err != nil {
		t.Fatalf("decode revoked grant: %v", err)
	}
	if revoked.SharingGrant.GrantState != incidents.SharingGrantStateRevoked || revoked.SharingGrant.RevokedAt == nil {
		t.Fatalf("grant was not revoked: %+v", revoked.SharingGrant)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/contact-public-keys/"+contactKey.ID+"/revoke", "application/json", bytes.NewBufferString(`{}`), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected revoke contact key status 200, got %d: %s", response.StatusCode, body)
	}
	response, body = requestWithAuth(t, app.privateHandler, http.MethodPatch, "/v1/contact-public-keys/"+contactKey.ID, "application/json", bytes.NewBufferString(`{"key_state":"active"}`), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected revoked key reactivation status 409, got %d: %s", response.StatusCode, body)
	}

	response, body = request(t, app.publicHandler, http.MethodGet, "/v1/contact-public-keys", "", nil)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected public handler contact key status 404, got %d: %s", response.StatusCode, body)
	}
	response, body = request(t, app.publicHandler, http.MethodPost, "/v1/incidents/"+incidentID+"/sharing-grants", "application/json", bytes.NewBufferString(grantBody))
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected public handler sharing grant status 404, got %d: %s", response.StatusCode, body)
	}
}

func TestSharingGrantRequiresActiveContactKey(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "pending-owner", "owner-password", auth.RoleUser)
	incidentID := createIncidentWithToken(t, app, ownerToken)
	contactKey := createContactPublicKeyWithToken(t, app, ownerToken, `{
		"display_label":"Pending contact",
		"wrapping_algorithm":"age-v1-x25519",
		"public_key":"age1pendingpublickey",
		"public_key_fingerprint":"fingerprint-pending"
	}`)

	grantBody := `{"contact_id":"` + contactKey.ContactID + `"}`
	response, body := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/incidents/"+incidentID+"/sharing-grants", "application/json", bytes.NewBufferString(grantBody), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected pending key grant status 404, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPatch, "/v1/contact-public-keys/"+contactKey.ID, "application/json", bytes.NewBufferString(`{"key_state":"active"}`), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected activate contact key status 200, got %d: %s", response.StatusCode, body)
	}
	grant := createSharingGrantWithToken(t, app, ownerToken, incidentID, grantBody)
	if grant.ContactPublicKeyID != contactKey.ID {
		t.Fatalf("grant used public key %q, want %q", grant.ContactPublicKeyID, contactKey.ID)
	}
}

func TestContactPublicKeyRoutesRejectSecretFields(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "secret-field-owner", "owner-password", auth.RoleUser)

	response, body := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/contact-public-keys", "application/json", bytes.NewBufferString(`{
		"display_label":"Bad contact",
		"wrapping_algorithm":"age-v1-x25519",
		"public_key":"age1public",
		"public_key_fingerprint":"fingerprint",
		"contact_private_key":"must-not-be-accepted"
	}`), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected unknown secret field status 400, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "invalid_json")
	for _, disallowed := range []string{"must-not-be-accepted", "contact_private_key"} {
		if strings.Contains(string(body), disallowed) {
			t.Fatalf("error response exposed rejected secret field %q: %s", disallowed, body)
		}
	}
}

func createIncidentWithToken(t *testing.T, app *testApp, token string) string {
	t.Helper()
	response, body := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/incidents", "application/json", bytes.NewBufferString(`{}`), token)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create incident status 201, got %d: %s", response.StatusCode, body)
	}
	var created struct {
		IncidentID string `json:"incident_id"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode created incident: %v", err)
	}
	return created.IncidentID
}

func createMediaStreamWithToken(t *testing.T, app *testApp, token, incidentID, mediaType, label string) incidents.MediaStream {
	t.Helper()
	requestBody, err := json.Marshal(map[string]string{
		"media_type": mediaType,
		"label":      label,
	})
	if err != nil {
		t.Fatalf("marshal stream request: %v", err)
	}
	response, body := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/incidents/"+incidentID+"/streams", "application/json", bytes.NewReader(requestBody), token)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create stream status 201, got %d: %s", response.StatusCode, body)
	}
	var created struct {
		Stream incidents.MediaStream `json:"stream"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode created stream: %v", err)
	}
	return created.Stream
}

func createContactPublicKeyWithToken(t *testing.T, app *testApp, token, body string) incidents.ContactPublicKey {
	t.Helper()
	response, responseBody := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/contact-public-keys", "application/json", bytes.NewBufferString(body), token)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create contact public key status 201, got %d: %s", response.StatusCode, responseBody)
	}
	var created struct {
		ContactPublicKey incidents.ContactPublicKey `json:"contact_public_key"`
	}
	if err := json.Unmarshal(responseBody, &created); err != nil {
		t.Fatalf("decode contact public key: %v", err)
	}
	return created.ContactPublicKey
}

func createSharingGrantWithToken(t *testing.T, app *testApp, token, incidentID, body string) incidents.SharingGrant {
	t.Helper()
	response, responseBody := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/incidents/"+incidentID+"/sharing-grants", "application/json", bytes.NewBufferString(body), token)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create sharing grant status 201, got %d: %s", response.StatusCode, responseBody)
	}
	var created struct {
		SharingGrant incidents.SharingGrant `json:"sharing_grant"`
	}
	if err := json.Unmarshal(responseBody, &created); err != nil {
		t.Fatalf("decode sharing grant: %v", err)
	}
	return created.SharingGrant
}
