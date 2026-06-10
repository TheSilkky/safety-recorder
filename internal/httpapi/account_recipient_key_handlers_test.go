package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/envelope/pq"
	"github.com/open-proofline/server/internal/incidents"
)

func TestAccountRecipientKeyRoutesAreOwnerScopedAndStateful(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "recipient-owner", "owner-password", auth.RoleUser)
	otherToken := createAccountAndLogin(t, app, "recipient-other", "other-password", auth.RoleUser)

	key := createAccountRecipientKeyWithToken(t, app, ownerToken, accountRecipientKeyBody(map[string]string{
		"recipient_type":         incidents.AccountRecipientTypeDevice,
		"key_id":                 "recipient-key-1",
		"display_label":          "Owner phone",
		"public_key":             "mlkem-public-key-1",
		"public_key_fingerprint": "fingerprint-1",
		"key_state":              incidents.AccountRecipientKeyStateActive,
	}))
	if key.Version != 1 || key.KeyState != incidents.AccountRecipientKeyStateActive || key.RecipientID == "" {
		t.Fatalf("unexpected created account recipient key: %+v", key)
	}

	response, body := requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/account-recipient-keys", "", nil, ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected owner list status 200, got %d: %s", response.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(key.ID)) || !bytes.Contains(body, []byte("recipient-key-1")) {
		t.Fatalf("owner list missing created account recipient key: %s", body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/account-recipient-keys/"+key.ID, "", nil, otherToken)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected other account get status 404, got %d: %s", response.StatusCode, body)
	}

	replacement := replaceAccountRecipientKeyWithToken(t, app, ownerToken, key.ID, accountRecipientKeyReplacementBody(map[string]string{
		"key_id":                 "recipient-key-2",
		"display_label":          "Owner phone replacement",
		"public_key":             "mlkem-public-key-2",
		"public_key_fingerprint": "fingerprint-2",
		"key_state":              incidents.AccountRecipientKeyStateActive,
	}))
	if replacement.Version != 2 || replacement.RecipientID != key.RecipientID || replacement.KeyState != incidents.AccountRecipientKeyStateActive {
		t.Fatalf("unexpected replacement account recipient key: %+v", replacement)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/account-recipient-keys/"+key.ID, "", nil, ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected original get status 200, got %d: %s", response.StatusCode, body)
	}
	var replaced struct {
		AccountRecipientKey incidents.AccountRecipientKey `json:"account_recipient_key"`
	}
	if err := json.Unmarshal(body, &replaced); err != nil {
		t.Fatalf("decode replaced key: %v", err)
	}
	if replaced.AccountRecipientKey.KeyState != incidents.AccountRecipientKeyStateReplaced ||
		replaced.AccountRecipientKey.ReplacedAt == nil ||
		replaced.AccountRecipientKey.ReplacedByRecipientKeyID != replacement.ID {
		t.Fatalf("original key was not marked replaced: %+v", replaced.AccountRecipientKey)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/account-recipient-keys/"+key.ID+"/replace", "application/json", bytes.NewBufferString(accountRecipientKeyReplacementBody(map[string]string{
		"key_id":                 "recipient-key-3",
		"public_key":             "mlkem-public-key-3",
		"public_key_fingerprint": "fingerprint-3",
	})), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected replaced key replace status 409, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/account-recipient-keys/"+replacement.ID+"/revoke", "application/json", bytes.NewBufferString(`{}`), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected revoke status 200, got %d: %s", response.StatusCode, body)
	}
	var revoked struct {
		AccountRecipientKey incidents.AccountRecipientKey `json:"account_recipient_key"`
	}
	if err := json.Unmarshal(body, &revoked); err != nil {
		t.Fatalf("decode revoked key: %v", err)
	}
	if revoked.AccountRecipientKey.KeyState != incidents.AccountRecipientKeyStateRevoked || revoked.AccountRecipientKey.RevokedAt == nil {
		t.Fatalf("replacement key was not revoked: %+v", revoked.AccountRecipientKey)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPatch, "/v1/account-recipient-keys/"+replacement.ID, "application/json", bytes.NewBufferString(`{"key_state":"active"}`), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected revoked key activation status 409, got %d: %s", response.StatusCode, body)
	}

	lostKey := createAccountRecipientKeyWithToken(t, app, ownerToken, accountRecipientKeyBody(map[string]string{
		"recipient_type":         incidents.AccountRecipientTypeDevice,
		"key_id":                 "recipient-key-lost",
		"display_label":          "Lost tablet",
		"public_key":             "mlkem-public-key-lost",
		"public_key_fingerprint": "fingerprint-lost",
		"key_state":              incidents.AccountRecipientKeyStateActive,
	}))
	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/account-recipient-keys/"+lostKey.ID+"/lost", "application/json", bytes.NewBufferString(`{}`), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected lost status 200, got %d: %s", response.StatusCode, body)
	}
	var lost struct {
		AccountRecipientKey incidents.AccountRecipientKey `json:"account_recipient_key"`
	}
	if err := json.Unmarshal(body, &lost); err != nil {
		t.Fatalf("decode lost key: %v", err)
	}
	if lost.AccountRecipientKey.KeyState != incidents.AccountRecipientKeyStateLost || lost.AccountRecipientKey.LostAt == nil {
		t.Fatalf("key was not marked lost: %+v", lost.AccountRecipientKey)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/account-recipient-keys", "application/json", bytes.NewBufferString(accountRecipientKeyBody(map[string]string{
		"recipient_type":         incidents.AccountRecipientTypeDevice,
		"key_id":                 "recipient-key-lost",
		"public_key":             "mlkem-public-key-duplicate",
		"public_key_fingerprint": "fingerprint-duplicate",
	})), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected duplicate key_id status 409, got %d: %s", response.StatusCode, body)
	}

	response, body = request(t, app.publicHandler, http.MethodGet, "/v1/account-recipient-keys", "", nil)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected public handler status 404, got %d: %s", response.StatusCode, body)
	}
}

func TestAccountRecipientKeyRoutesRejectSecretFields(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "recipient-secret-owner", "owner-password", auth.RoleUser)

	response, body := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/account-recipient-keys", "application/json", bytes.NewBufferString(`{
		"recipient_type":"device",
		"key_id":"recipient-key-secret",
		"scheme":"`+pq.SchemeID+`",
		"suite_id":"`+pq.SuiteID+`",
		"public_key":"mlkem-public-key-secret",
		"public_key_fingerprint":"fingerprint-secret",
		"private_key":"must-not-be-accepted"
	}`), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected unknown secret field status 400, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "invalid_json")
	for _, disallowed := range []string{"must-not-be-accepted", "private_key"} {
		if strings.Contains(string(body), disallowed) {
			t.Fatalf("error response exposed rejected secret field %q: %s", disallowed, body)
		}
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/account-recipient-keys", "application/json", bytes.NewBufferString(accountRecipientKeyBody(map[string]string{
		"recipient_type":         incidents.AccountRecipientTypeDevice,
		"key_id":                 "recipient-key-private-material",
		"public_key":             "-----BEGIN PRIVATE KEY-----",
		"public_key_fingerprint": "fingerprint-private-material",
	})), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected private material status 400, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "invalid_public_key")
	if strings.Contains(string(body), "BEGIN PRIVATE KEY") {
		t.Fatalf("error response exposed rejected key material: %s", body)
	}
}

func createAccountRecipientKeyWithToken(t *testing.T, app *testApp, token, body string) incidents.AccountRecipientKey {
	t.Helper()
	response, responseBody := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/account-recipient-keys", "application/json", bytes.NewBufferString(body), token)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create account recipient key status 201, got %d: %s", response.StatusCode, responseBody)
	}
	var created struct {
		AccountRecipientKey incidents.AccountRecipientKey `json:"account_recipient_key"`
	}
	if err := json.Unmarshal(responseBody, &created); err != nil {
		t.Fatalf("decode account recipient key: %v", err)
	}
	return created.AccountRecipientKey
}

func replaceAccountRecipientKeyWithToken(t *testing.T, app *testApp, token, recipientKeyID, body string) incidents.AccountRecipientKey {
	t.Helper()
	response, responseBody := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/account-recipient-keys/"+recipientKeyID+"/replace", "application/json", bytes.NewBufferString(body), token)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected replace account recipient key status 201, got %d: %s", response.StatusCode, responseBody)
	}
	var created struct {
		AccountRecipientKey incidents.AccountRecipientKey `json:"account_recipient_key"`
	}
	if err := json.Unmarshal(responseBody, &created); err != nil {
		t.Fatalf("decode account recipient replacement key: %v", err)
	}
	return created.AccountRecipientKey
}

func accountRecipientKeyBody(values map[string]string) string {
	if _, ok := values["scheme"]; !ok {
		values["scheme"] = pq.SchemeID
	}
	if _, ok := values["suite_id"]; !ok {
		values["suite_id"] = pq.SuiteID
	}
	body, err := json.Marshal(values)
	if err != nil {
		panic(err)
	}
	return string(body)
}

func accountRecipientKeyReplacementBody(values map[string]string) string {
	return accountRecipientKeyBody(values)
}
