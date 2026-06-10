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

func TestContactPublicKeyLifecycleRoutes(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "contact-lifecycle-owner", "owner-password", auth.RoleUser)
	otherToken := createAccountAndLogin(t, app, "contact-lifecycle-other", "other-password", auth.RoleUser)
	incidentID := createIncidentWithToken(t, app, ownerToken)
	contactKey := createContactPublicKeyWithToken(t, app, ownerToken, `{
		"display_label":"Lifecycle contact",
		"wrapping_algorithm":"age-v1-x25519",
		"public_key":"age1lifecycle",
		"public_key_fingerprint":"fingerprint-lifecycle"
	}`)
	if contactKey.KeyState != incidents.ContactKeyStatePendingVerification {
		t.Fatalf("default contact key state = %q, want pending_verification", contactKey.KeyState)
	}

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
	if grant.ContactPublicKeyID != contactKey.ID || grant.ContactPublicKeyVersion != 1 {
		t.Fatalf("grant used contact key %q version %d, want %q version 1", grant.ContactPublicKeyID, grant.ContactPublicKeyVersion, contactKey.ID)
	}

	replacement := replaceContactPublicKeyWithToken(t, app, ownerToken, contactKey.ID, `{
		"display_label":"Lifecycle contact replacement",
		"wrapping_algorithm":"age-v1-x25519",
		"public_key":"age1replacement",
		"public_key_fingerprint":"fingerprint-replacement",
		"key_state":"active"
	}`)
	if replacement.ContactID != contactKey.ContactID ||
		replacement.Version != 2 ||
		replacement.KeyState != incidents.ContactKeyStateActive {
		t.Fatalf("unexpected replacement key: %+v", replacement)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/contact-public-keys/"+contactKey.ID, "", nil, ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected original key get status 200, got %d: %s", response.StatusCode, body)
	}
	var original struct {
		ContactPublicKey incidents.ContactPublicKey `json:"contact_public_key"`
	}
	if err := json.Unmarshal(body, &original); err != nil {
		t.Fatalf("decode replaced contact key: %v", err)
	}
	if original.ContactPublicKey.KeyState != incidents.ContactKeyStateReplaced ||
		original.ContactPublicKey.ReplacedAt == nil ||
		original.ContactPublicKey.ReplacedByPublicKeyID != replacement.ID {
		t.Fatalf("original key was not marked replaced: %+v", original.ContactPublicKey)
	}

	nextGrant := createSharingGrantWithToken(t, app, ownerToken, incidentID, grantBody)
	if nextGrant.ContactPublicKeyID != replacement.ID || nextGrant.ContactPublicKeyVersion != 2 {
		t.Fatalf("grant used key %q version %d, want replacement %q version 2", nextGrant.ContactPublicKeyID, nextGrant.ContactPublicKeyVersion, replacement.ID)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/contact-public-keys/"+contactKey.ID+"/replace", "application/json", bytes.NewBufferString(`{
		"display_label":"invalid",
		"wrapping_algorithm":"age-v1-x25519",
		"public_key":"age1invalid",
		"public_key_fingerprint":"fingerprint-invalid"
	}`), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected replaced key replace status 409, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/contact-public-keys/"+replacement.ID+"/lost", "application/json", bytes.NewBufferString(`{}`), otherToken)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected other account lost status 404, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/contact-public-keys/"+replacement.ID+"/lost", "application/json", bytes.NewBufferString(`{}`), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected lost contact key status 200, got %d: %s", response.StatusCode, body)
	}
	var lost struct {
		ContactPublicKey incidents.ContactPublicKey `json:"contact_public_key"`
	}
	if err := json.Unmarshal(body, &lost); err != nil {
		t.Fatalf("decode lost contact key: %v", err)
	}
	if lost.ContactPublicKey.KeyState != incidents.ContactKeyStateLost || lost.ContactPublicKey.LostAt == nil {
		t.Fatalf("contact key was not marked lost: %+v", lost.ContactPublicKey)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/incidents/"+incidentID+"/sharing-grants", "application/json", bytes.NewBufferString(grantBody), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected lost key grant status 404, got %d: %s", response.StatusCode, body)
	}
	response, body = requestWithAuth(t, app.privateHandler, http.MethodPatch, "/v1/contact-public-keys/"+replacement.ID, "application/json", bytes.NewBufferString(`{"key_state":"active"}`), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected lost key reactivation status 409, got %d: %s", response.StatusCode, body)
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

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/contact-public-keys", "application/json", bytes.NewBufferString(`{
		"display_label":"Bad contact",
		"wrapping_algorithm":"age-v1-x25519",
		"public_key":"-----BEGIN PRIVATE KEY-----",
		"public_key_fingerprint":"fingerprint"
	}`), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected private-key marker status 400, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "invalid_public_key")
	if strings.Contains(string(body), "BEGIN PRIVATE KEY") {
		t.Fatalf("error response exposed rejected public key material: %s", body)
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

func replaceContactPublicKeyWithToken(t *testing.T, app *testApp, token, publicKeyID, body string) incidents.ContactPublicKey {
	t.Helper()
	response, responseBody := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/contact-public-keys/"+publicKeyID+"/replace", "application/json", bytes.NewBufferString(body), token)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected replace contact public key status 201, got %d: %s", response.StatusCode, responseBody)
	}
	var created struct {
		ContactPublicKey incidents.ContactPublicKey `json:"contact_public_key"`
	}
	if err := json.Unmarshal(responseBody, &created); err != nil {
		t.Fatalf("decode replacement contact public key: %v", err)
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
