package httpapi_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/envelope/pq"
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
		"wrapping_algorithm":"`+pq.WrappingAlgorithm+`",
		"public_key":"pq-test-wrapped-public-key",
		"public_key_fingerprint":"fingerprint-wrapped",
		"key_state":"active"
	}`)
	grant := createSharingGrantWithToken(t, app, ownerToken, incidentID, `{
		"stream_id":"`+stream.ID+`",
		"contact_id":"`+contactKey.ContactID+`"
	}`)

	wrappedKey := createWrappedKeyWithToken(t, app, ownerToken, incidentID, wrappedKeyRequestBody(t, stream.ID, grant.ID, "media-key-1"))
	if wrappedKey.GrantID != grant.ID || wrappedKey.StreamID != stream.ID || wrappedKey.ContactPublicKeyID != contactKey.ID {
		t.Fatalf("unexpected wrapped key: %+v", wrappedKey)
	}

	response, body := requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/incidents/"+incidentID+"/wrapped-keys", "", nil, ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected owner list wrapped keys status 200, got %d: %s", response.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(wrappedKey.ID)) || !bytes.Contains(body, []byte("media-key-1")) {
		t.Fatalf("owner wrapped key list missing created record: %s", body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/wrapped-keys/"+wrappedKey.ID, "", nil, otherToken)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected other account wrapped key status 404, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/incidents/"+incidentID+"/wrapped-keys", "application/json", bytes.NewBufferString(wrappedKeyRequestBody(t, stream.ID, grant.ID, "media-key-2")), app.authToken)
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

	response, body = request(t, app.publicHandler, http.MethodPost, "/v1/incidents/"+incidentID+"/wrapped-keys", "application/json", bytes.NewBufferString(wrappedKeyRequestBody(t, stream.ID, grant.ID, "media-key-3")))
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
		"wrapping_algorithm":"`+pq.WrappingAlgorithm+`",
		"public_key":"pq-test-metadata-public-key",
		"public_key_fingerprint":"fingerprint-metadata",
		"key_state":"active"
	}`)
	grant := createSharingGrantWithToken(t, app, ownerToken, incidentID, `{
		"contact_id":"`+contactKey.ContactID+`",
		"data_class":"metadata"
	}`)

	response, body := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/incidents/"+incidentID+"/wrapped-keys", "application/json", bytes.NewBufferString(wrappedKeyRequestBody(t, "", grant.ID, "media-key-metadata")), ownerToken)
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
		"wrapping_algorithm":"`+pq.WrappingAlgorithm+`",
		"public_key":"pq-test-secret-metadata",
		"public_key_fingerprint":"fingerprint-secret-metadata",
		"key_state":"active"
	}`)
	grant := createSharingGrantWithToken(t, app, ownerToken, incidentID, `{
		"contact_id":"`+contactKey.ContactID+`"
	}`)

	body := `{
		"grant_id":"` + grant.ID + `",
		"media_key_id":"media-key-secret-metadata",
		"wrapping_algorithm":"` + pq.WrappingAlgorithm + `",
		"wrapping_algorithm_version":"1",
		"wrapped_key_ciphertext":"wrapped-ciphertext",
		"public_wrapping_metadata":{
			"profile":"` + pq.ProfileID + `",
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

func TestTrustedContactWrappedKeyRoutesAreRelationshipAndGrantScoped(t *testing.T) {
	app := newTestApp(t)
	fixture := newTrustedContactWrappedKeyRouteFixture(t, app, "active")

	assertTrustedContactWrappedKeyVisible(t, app, fixture.recipientToken, fixture.incidentID, fixture.wrappedKey)
	assertTrustedContactWrappedKeyHidden(t, app, fixture.otherToken, fixture.incidentID, fixture.wrappedKey.ID)
	assertTrustedContactWrappedKeyHidden(t, app, fixture.ownerToken, fixture.incidentID, fixture.wrappedKey.ID)

	response, body := request(t, app.publicHandler, http.MethodGet, "/v1/trusted-contact/incidents/"+fixture.incidentID+"/wrapped-keys", "", nil)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected public trusted-contact route status 404, got %d: %s", response.StatusCode, body)
	}

	viewerToken := createIncidentTokenWithAuth(t, app, fixture.ownerToken, fixture.incidentID, "viewer")
	response, body = request(t, app.publicHandler, http.MethodGet, "/i/"+viewerToken.Token+"/data", "", nil)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected public viewer data status 200, got %d: %s", response.StatusCode, body)
	}
	for _, disallowed := range []string{fixture.wrappedKey.ID, fixture.wrappedKey.WrappedKeyCiphertext, "wrapped_key_ciphertext", "public_wrapping_metadata"} {
		if strings.Contains(string(body), disallowed) {
			t.Fatalf("public viewer data exposed trusted-contact wrapped-key field %q: %s", disallowed, body)
		}
	}
}

func TestTrustedContactWrappedKeyRoutesFilterInactiveAuthorization(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(t *testing.T, app *testApp, fixture trustedContactWrappedKeyRouteFixture)
	}{
		{
			name: "revoked relationship",
			mutate: func(t *testing.T, app *testApp, fixture trustedContactWrappedKeyRouteFixture) {
				t.Helper()
				response, body := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/trusted-contact-relationships/"+fixture.relationship.ID+"/revoke", "application/json", bytes.NewBufferString(`{}`), fixture.ownerToken)
				response.Body.Close()
				if response.StatusCode != http.StatusOK {
					t.Fatalf("expected revoke relationship status 200, got %d: %s", response.StatusCode, body)
				}
			},
		},
		{
			name: "revoked grant",
			mutate: func(t *testing.T, app *testApp, fixture trustedContactWrappedKeyRouteFixture) {
				t.Helper()
				response, body := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/sharing-grants/"+fixture.grant.ID+"/revoke", "application/json", bytes.NewBufferString(`{}`), fixture.ownerToken)
				response.Body.Close()
				if response.StatusCode != http.StatusOK {
					t.Fatalf("expected revoke sharing grant status 200, got %d: %s", response.StatusCode, body)
				}
			},
		},
		{
			name: "expired grant",
			mutate: func(t *testing.T, app *testApp, fixture trustedContactWrappedKeyRouteFixture) {
				t.Helper()
				_, err := app.db.ExecContext(context.Background(), `
					UPDATE sharing_grants
					SET expires_at = ?
					WHERE id = ?`,
					time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano),
					fixture.grant.ID,
				)
				if err != nil {
					t.Fatalf("expire sharing grant: %v", err)
				}
			},
		},
		{
			name: "revoked contact key",
			mutate: func(t *testing.T, app *testApp, fixture trustedContactWrappedKeyRouteFixture) {
				t.Helper()
				response, body := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/contact-public-keys/"+fixture.contactKey.ID+"/revoke", "application/json", bytes.NewBufferString(`{}`), fixture.ownerToken)
				response.Body.Close()
				if response.StatusCode != http.StatusOK {
					t.Fatalf("expected revoke contact key status 200, got %d: %s", response.StatusCode, body)
				}
			},
		},
		{
			name: "lost contact key",
			mutate: func(t *testing.T, app *testApp, fixture trustedContactWrappedKeyRouteFixture) {
				t.Helper()
				response, body := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/contact-public-keys/"+fixture.contactKey.ID+"/lost", "application/json", bytes.NewBufferString(`{}`), fixture.ownerToken)
				response.Body.Close()
				if response.StatusCode != http.StatusOK {
					t.Fatalf("expected lost contact key status 200, got %d: %s", response.StatusCode, body)
				}
			},
		},
		{
			name: "replaced contact key",
			mutate: func(t *testing.T, app *testApp, fixture trustedContactWrappedKeyRouteFixture) {
				t.Helper()
				response, body := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/contact-public-keys/"+fixture.contactKey.ID+"/replace", "application/json", bytes.NewBufferString(`{
					"display_label":"Replacement contact key",
					"wrapping_algorithm":"`+pq.WrappingAlgorithm+`",
					"public_key":"pq-test-replaced-trusted-contact-public-key-`+fixture.publicKeySuffix+`",
					"public_key_fingerprint":"fingerprint-replaced-trusted-contact-`+fixture.publicKeySuffix+`",
					"key_state":"active"
				}`), fixture.ownerToken)
				response.Body.Close()
				if response.StatusCode != http.StatusCreated {
					t.Fatalf("expected replace contact key status 201, got %d: %s", response.StatusCode, body)
				}
			},
		},
		{
			name: "revoked wrapped key",
			mutate: func(t *testing.T, app *testApp, fixture trustedContactWrappedKeyRouteFixture) {
				t.Helper()
				response, body := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/wrapped-keys/"+fixture.wrappedKey.ID+"/revoke", "application/json", bytes.NewBufferString(`{}`), fixture.ownerToken)
				response.Body.Close()
				if response.StatusCode != http.StatusOK {
					t.Fatalf("expected revoke wrapped key status 200, got %d: %s", response.StatusCode, body)
				}
			},
		},
		{
			name: "rotated wrapped key",
			mutate: func(t *testing.T, app *testApp, fixture trustedContactWrappedKeyRouteFixture) {
				t.Helper()
				_, err := app.db.ExecContext(context.Background(), `
					UPDATE wrapped_key_records
					SET wrapped_key_state = ?, rotated_at = ?
					WHERE id = ?`,
					incidents.WrappedKeyStateRotated,
					time.Now().UTC().Format(time.RFC3339Nano),
					fixture.wrappedKey.ID,
				)
				if err != nil {
					t.Fatalf("rotate wrapped key: %v", err)
				}
			},
		},
		{
			name: "unbound contact key",
			mutate: func(t *testing.T, app *testApp, fixture trustedContactWrappedKeyRouteFixture) {
				t.Helper()
				_, err := app.db.ExecContext(context.Background(), `
					UPDATE contact_public_keys
					SET recipient_account_id = NULL
					WHERE id = ?`,
					fixture.contactKey.ID,
				)
				if err != nil {
					t.Fatalf("unbind contact key recipient: %v", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestApp(t)
			fixture := newTrustedContactWrappedKeyRouteFixture(t, app, strings.ReplaceAll(tc.name, " ", "-"))
			assertTrustedContactWrappedKeyVisible(t, app, fixture.recipientToken, fixture.incidentID, fixture.wrappedKey)
			tc.mutate(t, app, fixture)
			assertTrustedContactWrappedKeyHidden(t, app, fixture.recipientToken, fixture.incidentID, fixture.wrappedKey.ID)
		})
	}
}

func wrappedKeyRequestBody(t *testing.T, streamID, grantID, mediaKeyID string) string {
	t.Helper()

	recipient, _, err := pq.GenerateRecipientKey(1)
	if err != nil {
		t.Fatalf("GenerateRecipientKey returned error: %v", err)
	}
	ctxStreamID := streamID
	if ctxStreamID == "" {
		ctxStreamID = "streamless-wrapped-key-test"
	}
	env, err := pq.Encrypt([]byte("wrapped-key test payload"), pq.PayloadContext{
		EnvelopeID:  "env_wrapped_key_" + mediaKeyID,
		IncidentID:  "inc_wrapped_key_test",
		StreamID:    ctxStreamID,
		MediaType:   incidents.MediaTypeAudio,
		ChunkIndex:  1,
		PayloadType: pq.PayloadTypeChunk,
		MediaKeyID:  mediaKeyID,
	}, []pq.Recipient{recipient})
	if err != nil {
		t.Fatalf("pq.Encrypt returned error: %v", err)
	}
	metadata, err := json.Marshal(env.Recipients[0].Metadata)
	if err != nil {
		t.Fatalf("marshal public wrapping metadata: %v", err)
	}
	body := map[string]any{
		"grant_id":                   grantID,
		"media_key_id":               mediaKeyID,
		"wrapping_algorithm":         pq.WrappingAlgorithm,
		"wrapping_algorithm_version": strconv.Itoa(pq.WrappingAlgorithmVersion),
		"wrapped_key_ciphertext":     base64.RawURLEncoding.EncodeToString(env.Recipients[0].WrappedKeyFrame),
		"public_wrapping_metadata":   json.RawMessage(metadata),
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

type trustedContactWrappedKeyRouteFixture struct {
	ownerToken      string
	recipientToken  string
	otherToken      string
	incidentID      string
	stream          incidents.MediaStream
	relationship    incidents.TrustedContactRelationship
	contactKey      incidents.ContactPublicKey
	grant           incidents.SharingGrant
	wrappedKey      incidents.WrappedKeyRecord
	recipientID     string
	wrappedKeyBody  string
	publicKeySuffix string
}

func newTrustedContactWrappedKeyRouteFixture(t *testing.T, app *testApp, suffix string) trustedContactWrappedKeyRouteFixture {
	t.Helper()
	cleanSuffix := strings.ReplaceAll(suffix, "_", "-")
	ownerToken := createAccountAndLogin(t, app, "trusted-wrapped-owner-"+cleanSuffix, "owner-password", auth.RoleUser)
	recipientToken := createAccountAndLogin(t, app, "trusted-wrapped-recipient-"+cleanSuffix, "recipient-password", auth.RoleUser)
	otherToken := createAccountAndLogin(t, app, "trusted-wrapped-other-"+cleanSuffix, "other-password", auth.RoleUser)
	recipientID := accountIDForToken(t, app, recipientToken)
	incidentID := createIncidentWithToken(t, app, ownerToken)
	stream := createMediaStreamWithToken(t, app, ownerToken, incidentID, incidents.MediaTypeAudio, "trusted audio")
	relationship := createTrustedContactRelationshipWithToken(t, app, ownerToken, trustedContactRelationshipBody(map[string]string{
		"recipient_account_id": recipientID,
		"display_label":        "Trusted contact " + cleanSuffix,
	}))
	response, body := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/trusted-contact-relationships/"+relationship.ID+"/accept", "application/json", bytes.NewBufferString(`{}`), recipientToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected accept trusted contact relationship status 200, got %d: %s", response.StatusCode, body)
	}
	contactKey := createContactPublicKeyWithToken(t, app, ownerToken, `{
		"recipient_account_id":"`+recipientID+`",
		"display_label":"Trusted contact key `+cleanSuffix+`",
		"wrapping_algorithm":"`+pq.WrappingAlgorithm+`",
		"public_key":"pq-test-trusted-contact-public-key-`+cleanSuffix+`",
		"public_key_fingerprint":"fingerprint-trusted-contact-`+cleanSuffix+`",
		"key_state":"active"
	}`)
	if contactKey.RecipientAccountID != recipientID {
		t.Fatalf("contact key recipient = %q, want %q", contactKey.RecipientAccountID, recipientID)
	}
	grant := createSharingGrantWithToken(t, app, ownerToken, incidentID, `{
		"stream_id":"`+stream.ID+`",
		"contact_id":"`+contactKey.ContactID+`"
	}`)
	wrappedBody := wrappedKeyRequestBody(t, stream.ID, grant.ID, "media-key-trusted-"+cleanSuffix)
	wrappedKey := createWrappedKeyWithToken(t, app, ownerToken, incidentID, wrappedBody)
	return trustedContactWrappedKeyRouteFixture{
		ownerToken:      ownerToken,
		recipientToken:  recipientToken,
		otherToken:      otherToken,
		incidentID:      incidentID,
		stream:          stream,
		relationship:    relationship,
		contactKey:      contactKey,
		grant:           grant,
		wrappedKey:      wrappedKey,
		recipientID:     recipientID,
		wrappedKeyBody:  wrappedBody,
		publicKeySuffix: cleanSuffix,
	}
}

func assertTrustedContactWrappedKeyVisible(t *testing.T, app *testApp, token, incidentID string, wrappedKey incidents.WrappedKeyRecord) {
	t.Helper()
	response, body := requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/trusted-contact/incidents/"+incidentID+"/wrapped-keys", "", nil, token)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected trusted-contact wrapped key list status 200, got %d: %s", response.StatusCode, body)
	}
	var listed struct {
		WrappedKeys []incidents.WrappedKeyRecord `json:"wrapped_keys"`
	}
	if err := json.Unmarshal(body, &listed); err != nil {
		t.Fatalf("decode trusted-contact wrapped key list: %v", err)
	}
	if len(listed.WrappedKeys) != 1 || listed.WrappedKeys[0].ID != wrappedKey.ID {
		t.Fatalf("unexpected trusted-contact wrapped key list: %+v", listed.WrappedKeys)
	}
	if !bytes.Contains(body, []byte(wrappedKey.WrappedKeyCiphertext)) {
		t.Fatalf("trusted-contact list omitted wrapped ciphertext: %s", body)
	}
	for _, disallowed := range []string{"raw_media_key", "contact_private_key", "private_key", "plaintext"} {
		if strings.Contains(string(body), disallowed) {
			t.Fatalf("trusted-contact response exposed disallowed key %q: %s", disallowed, body)
		}
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/trusted-contact/wrapped-keys/"+wrappedKey.ID, "", nil, token)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected trusted-contact wrapped key get status 200, got %d: %s", response.StatusCode, body)
	}
	var got struct {
		WrappedKey incidents.WrappedKeyRecord `json:"wrapped_key"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode trusted-contact wrapped key get: %v", err)
	}
	if got.WrappedKey.ID != wrappedKey.ID || got.WrappedKey.WrappedKeyCiphertext == "" {
		t.Fatalf("unexpected trusted-contact wrapped key get: %+v", got.WrappedKey)
	}
}

func assertTrustedContactWrappedKeyHidden(t *testing.T, app *testApp, token, incidentID, wrappedKeyID string) {
	t.Helper()
	response, body := requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/trusted-contact/incidents/"+incidentID+"/wrapped-keys", "", nil, token)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected hidden trusted-contact wrapped key list status 200, got %d: %s", response.StatusCode, body)
	}
	var listed struct {
		WrappedKeys []incidents.WrappedKeyRecord `json:"wrapped_keys"`
	}
	if err := json.Unmarshal(body, &listed); err != nil {
		t.Fatalf("decode hidden trusted-contact wrapped key list: %v", err)
	}
	if len(listed.WrappedKeys) != 0 || bytes.Contains(body, []byte(wrappedKeyID)) {
		t.Fatalf("hidden trusted-contact list exposed wrapped key %q: %s", wrappedKeyID, body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/trusted-contact/wrapped-keys/"+wrappedKeyID, "", nil, token)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected hidden trusted-contact wrapped key get status 404, got %d: %s", response.StatusCode, body)
	}
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
