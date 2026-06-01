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

func TestWrappedKeyRoutesAreGrantScoped(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "wrapped-owner", "owner-password", auth.RoleUser)
	otherToken := createAccountAndLogin(t, app, "wrapped-other", "other-password", auth.RoleUser)
	incidentID := createIncidentWithToken(t, app, ownerToken)
	stream := createMediaStreamWithToken(t, app, ownerToken, incidentID, incidents.MediaTypeAudio, "owner audio")
	contactKey := createContactPublicKeyWithToken(t, app, ownerToken, `{
		"display_label":"Trusted contact",
		"wrapping_algorithm":"age-v1-x25519",
		"public_key":"age1wrappedpublickey",
		"public_key_fingerprint":"fingerprint-wrapped",
		"key_state":"active"
	}`)
	grant := createSharingGrantWithToken(t, app, ownerToken, incidentID, `{
		"stream_id":"`+stream.ID+`",
		"contact_id":"`+contactKey.ContactID+`"
	}`)

	wrappedKey := createWrappedKeyWithToken(t, app, ownerToken, incidentID, wrappedKeyRequestBody(stream.ID, grant.ID, "media-key-1"))
	if wrappedKey.GrantID != grant.ID || wrappedKey.StreamID != stream.ID || wrappedKey.ContactPublicKeyID != contactKey.ID {
		t.Fatalf("unexpected wrapped key: %+v", wrappedKey)
	}

	response, body := requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/incidents/"+incidentID+"/wrapped-keys", "", nil, ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected owner list wrapped keys status 200, got %d: %s", response.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(wrappedKey.ID)) || !bytes.Contains(body, []byte("wrapped-ciphertext")) {
		t.Fatalf("owner wrapped key list missing created record: %s", body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/wrapped-keys/"+wrappedKey.ID, "", nil, otherToken)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected other account wrapped key status 404, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/incidents/"+incidentID+"/wrapped-keys", "application/json", bytes.NewBufferString(wrappedKeyRequestBody(stream.ID, grant.ID, "media-key-2")), app.authToken)
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("expected admin non-owner wrapped key status 403, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/sharing-grants/"+grant.ID+"/revoke", "application/json", bytes.NewBufferString(`{}`), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected revoke grant status 200, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/wrapped-keys/"+wrappedKey.ID, "", nil, ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected revoked grant to stop wrapped key delivery, got %d: %s", response.StatusCode, body)
	}

	response, body = request(t, app.publicHandler, http.MethodPost, "/v1/incidents/"+incidentID+"/wrapped-keys", "application/json", bytes.NewBufferString(wrappedKeyRequestBody(stream.ID, grant.ID, "media-key-3")))
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected public handler wrapped key status 404, got %d: %s", response.StatusCode, body)
	}
}

func TestWrappedKeyRoutesRejectSecretFields(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "wrapped-secret-owner", "owner-password", auth.RoleUser)
	incidentID := createIncidentWithToken(t, app, ownerToken)

	response, body := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/incidents/"+incidentID+"/wrapped-keys", "application/json", bytes.NewBufferString(`{
		"grant_id":"sgr_fake",
		"media_key_id":"media-key",
		"wrapping_algorithm":"age-v1-x25519",
		"wrapping_algorithm_version":"1",
		"wrapped_key_ciphertext":"wrapped-ciphertext",
		"public_wrapping_metadata":{"profile":"age-v1-x25519"},
		"raw_media_key":null
	}`), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected unknown secret field status 400, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "invalid_json")
	if strings.Contains(string(body), "raw_media_key") {
		t.Fatalf("error response exposed rejected secret field: %s", body)
	}
}

func TestWrappedKeyRoutesRequireCiphertextGrant(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "wrapped-metadata-owner", "owner-password", auth.RoleUser)
	incidentID := createIncidentWithToken(t, app, ownerToken)
	contactKey := createContactPublicKeyWithToken(t, app, ownerToken, `{
		"display_label":"Trusted contact",
		"wrapping_algorithm":"age-v1-x25519",
		"public_key":"age1metadatapublickey",
		"public_key_fingerprint":"fingerprint-metadata",
		"key_state":"active"
	}`)
	grant := createSharingGrantWithToken(t, app, ownerToken, incidentID, `{
		"contact_id":"`+contactKey.ContactID+`",
		"data_class":"metadata"
	}`)

	response, body := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/incidents/"+incidentID+"/wrapped-keys", "application/json", bytes.NewBufferString(wrappedKeyRequestBody("", grant.ID, "media-key-metadata")), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected metadata-only grant wrapped key status 409, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "wrapped_key_grant_not_authorized")
}

func TestWrappedKeyRoutesRejectSecretMetadataKeys(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "wrapped-secret-metadata-owner", "owner-password", auth.RoleUser)
	incidentID := createIncidentWithToken(t, app, ownerToken)
	contactKey := createContactPublicKeyWithToken(t, app, ownerToken, `{
		"display_label":"Trusted contact",
		"wrapping_algorithm":"age-v1-x25519",
		"public_key":"age1secretmetadata",
		"public_key_fingerprint":"fingerprint-secret-metadata",
		"key_state":"active"
	}`)
	grant := createSharingGrantWithToken(t, app, ownerToken, incidentID, `{
		"contact_id":"`+contactKey.ContactID+`"
	}`)

	body := `{
		"grant_id":"` + grant.ID + `",
		"media_key_id":"media-key-secret-metadata",
		"wrapping_algorithm":"age-v1-x25519",
		"wrapping_algorithm_version":"1",
		"wrapped_key_ciphertext":"wrapped-ciphertext",
		"public_wrapping_metadata":{
			"profile":"age-v1-x25519",
			"recipient":{"raw_media_key":null}
		}
	}`
	response, responseBody := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/incidents/"+incidentID+"/wrapped-keys", "application/json", bytes.NewBufferString(body), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected secret metadata key status 400, got %d: %s", response.StatusCode, responseBody)
	}
	assertErrorCode(t, responseBody, "invalid_public_wrapping_metadata")
	if strings.Contains(string(responseBody), "raw_media_key") {
		t.Fatalf("error response exposed rejected metadata key: %s", responseBody)
	}
}

func wrappedKeyRequestBody(streamID, grantID, mediaKeyID string) string {
	body := map[string]any{
		"grant_id":                   grantID,
		"media_key_id":               mediaKeyID,
		"wrapping_algorithm":         "age-v1-x25519",
		"wrapping_algorithm_version": "1",
		"wrapped_key_ciphertext":     "wrapped-ciphertext",
		"public_wrapping_metadata": map[string]string{
			"profile": "age-v1-x25519",
		},
	}
	if streamID != "" {
		body["stream_id"] = streamID
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func createWrappedKeyWithToken(t *testing.T, app *testApp, token, incidentID, body string) incidents.WrappedKeyRecord {
	t.Helper()
	response, responseBody := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/incidents/"+incidentID+"/wrapped-keys", "application/json", bytes.NewBufferString(body), token)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create wrapped key status 201, got %d: %s", response.StatusCode, responseBody)
	}
	var created struct {
		WrappedKey incidents.WrappedKeyRecord `json:"wrapped_key"`
	}
	if err := json.Unmarshal(responseBody, &created); err != nil {
		t.Fatalf("decode wrapped key: %v", err)
	}
	return created.WrappedKey
}
