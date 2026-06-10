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

func TestTrustedContactRelationshipRoutesAreAccountScopedAndStateful(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "relationship-owner", "owner-password", auth.RoleUser)
	recipientToken := createAccountAndLogin(t, app, "relationship-recipient", "recipient-password", auth.RoleUser)
	otherToken := createAccountAndLogin(t, app, "relationship-other", "other-password", auth.RoleUser)
	recipientAccountID := accountIDForToken(t, app, recipientToken)
	otherAccountID := accountIDForToken(t, app, otherToken)

	relationship := createTrustedContactRelationshipWithToken(t, app, ownerToken, trustedContactRelationshipBody(map[string]string{
		"recipient_account_id": recipientAccountID,
		"display_label":        "Emergency contact",
	}))
	if relationship.RelationshipState != incidents.TrustedContactRelationshipStatePendingInvite ||
		relationship.RelationshipRole != incidents.TrustedContactRelationshipRoleTrustedContact ||
		relationship.RecipientAccountID != recipientAccountID ||
		relationship.InvitedAt.IsZero() {
		t.Fatalf("unexpected created trusted contact relationship: %+v", relationship)
	}

	response, body := requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/trusted-contact-relationships", "", nil, recipientToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected recipient list status 200, got %d: %s", response.StatusCode, body)
	}
	if !bytes.Contains(body, []byte(relationship.ID)) || !bytes.Contains(body, []byte(recipientAccountID)) {
		t.Fatalf("recipient list missing relationship: %s", body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/trusted-contact-relationships/"+relationship.ID, "", nil, otherToken)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected unrelated account get status 404, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/trusted-contact-relationships/"+relationship.ID+"/accept", "application/json", bytes.NewBufferString(`{}`), otherToken)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected unrelated account accept status 404, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/trusted-contact-relationships/"+relationship.ID+"/accept", "application/json", bytes.NewBufferString(`{}`), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected owner accept status 404, got %d: %s", response.StatusCode, body)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/trusted-contact-relationships/"+relationship.ID+"/accept", "application/json", bytes.NewBufferString(`{}`), recipientToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected recipient accept status 200, got %d: %s", response.StatusCode, body)
	}
	var accepted struct {
		TrustedContactRelationship incidents.TrustedContactRelationship `json:"trusted_contact_relationship"`
	}
	if err := json.Unmarshal(body, &accepted); err != nil {
		t.Fatalf("decode accepted relationship: %v", err)
	}
	if accepted.TrustedContactRelationship.RelationshipState != incidents.TrustedContactRelationshipStateActive ||
		accepted.TrustedContactRelationship.AcceptedAt == nil {
		t.Fatalf("relationship was not accepted: %+v", accepted.TrustedContactRelationship)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/trusted-contact-relationships", "application/json", bytes.NewBufferString(trustedContactRelationshipBody(map[string]string{
		"recipient_account_id": recipientAccountID,
	})), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("expected duplicate open relationship status 409, got %d: %s", response.StatusCode, body)
	}

	replacement := replaceTrustedContactRelationshipWithToken(t, app, ownerToken, relationship.ID, trustedContactRelationshipBody(map[string]string{
		"recipient_account_id": otherAccountID,
		"display_label":        "Replacement contact",
	}))
	if replacement.RelationshipState != incidents.TrustedContactRelationshipStatePendingInvite ||
		replacement.RecipientAccountID != otherAccountID {
		t.Fatalf("unexpected replacement relationship: %+v", replacement)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/trusted-contact-relationships/"+relationship.ID, "", nil, ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected original get status 200, got %d: %s", response.StatusCode, body)
	}
	var replaced struct {
		TrustedContactRelationship incidents.TrustedContactRelationship `json:"trusted_contact_relationship"`
	}
	if err := json.Unmarshal(body, &replaced); err != nil {
		t.Fatalf("decode replaced relationship: %v", err)
	}
	if replaced.TrustedContactRelationship.RelationshipState != incidents.TrustedContactRelationshipStateReplaced ||
		replaced.TrustedContactRelationship.ReplacedAt == nil ||
		replaced.TrustedContactRelationship.ReplacedByRelationshipID != replacement.ID {
		t.Fatalf("original relationship was not marked replaced: %+v", replaced.TrustedContactRelationship)
	}

	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/trusted-contact-relationships/"+replacement.ID+"/decline", "application/json", bytes.NewBufferString(`{}`), otherToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected replacement decline status 200, got %d: %s", response.StatusCode, body)
	}
	var declined struct {
		TrustedContactRelationship incidents.TrustedContactRelationship `json:"trusted_contact_relationship"`
	}
	if err := json.Unmarshal(body, &declined); err != nil {
		t.Fatalf("decode declined relationship: %v", err)
	}
	if declined.TrustedContactRelationship.RelationshipState != incidents.TrustedContactRelationshipStateDeclined ||
		declined.TrustedContactRelationship.DeclinedAt == nil {
		t.Fatalf("relationship was not declined: %+v", declined.TrustedContactRelationship)
	}

	revokedCandidate := createTrustedContactRelationshipWithToken(t, app, ownerToken, trustedContactRelationshipBody(map[string]string{
		"recipient_account_id": recipientAccountID,
		"display_label":        "Second invite",
	}))
	response, body = requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/trusted-contact-relationships/"+revokedCandidate.ID+"/revoke", "application/json", bytes.NewBufferString(`{}`), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected revoke status 200, got %d: %s", response.StatusCode, body)
	}
	var revoked struct {
		TrustedContactRelationship incidents.TrustedContactRelationship `json:"trusted_contact_relationship"`
	}
	if err := json.Unmarshal(body, &revoked); err != nil {
		t.Fatalf("decode revoked relationship: %v", err)
	}
	if revoked.TrustedContactRelationship.RelationshipState != incidents.TrustedContactRelationshipStateRevoked ||
		revoked.TrustedContactRelationship.RevokedAt == nil ||
		revoked.TrustedContactRelationship.RevokedByAccountID == "" {
		t.Fatalf("relationship was not revoked: %+v", revoked.TrustedContactRelationship)
	}

	response, body = request(t, app.publicHandler, http.MethodGet, "/v1/trusted-contact-relationships", "", nil)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected public handler status 404, got %d: %s", response.StatusCode, body)
	}
}

func TestTrustedContactRelationshipRoutesRejectSecretFields(t *testing.T) {
	app := newTestApp(t)
	ownerToken := createAccountAndLogin(t, app, "relationship-secret-owner", "owner-password", auth.RoleUser)
	recipientToken := createAccountAndLogin(t, app, "relationship-secret-recipient", "recipient-password", auth.RoleUser)
	recipientAccountID := accountIDForToken(t, app, recipientToken)

	response, body := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/trusted-contact-relationships", "application/json", bytes.NewBufferString(`{
		"recipient_account_id":"`+recipientAccountID+`",
		"display_label":"Secret field",
		"raw_media_key":"must-not-be-accepted"
	}`), ownerToken)
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected unknown secret field status 400, got %d: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "invalid_json")
	for _, disallowed := range []string{"must-not-be-accepted", "raw_media_key"} {
		if strings.Contains(string(body), disallowed) {
			t.Fatalf("error response exposed rejected secret field %q: %s", disallowed, body)
		}
	}
}

func createTrustedContactRelationshipWithToken(t *testing.T, app *testApp, token, body string) incidents.TrustedContactRelationship {
	t.Helper()
	response, responseBody := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/trusted-contact-relationships", "application/json", bytes.NewBufferString(body), token)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create trusted contact relationship status 201, got %d: %s", response.StatusCode, responseBody)
	}
	var created struct {
		TrustedContactRelationship incidents.TrustedContactRelationship `json:"trusted_contact_relationship"`
	}
	if err := json.Unmarshal(responseBody, &created); err != nil {
		t.Fatalf("decode trusted contact relationship: %v", err)
	}
	return created.TrustedContactRelationship
}

func replaceTrustedContactRelationshipWithToken(t *testing.T, app *testApp, token, relationshipID, body string) incidents.TrustedContactRelationship {
	t.Helper()
	response, responseBody := requestWithAuth(t, app.privateHandler, http.MethodPost, "/v1/trusted-contact-relationships/"+relationshipID+"/replace", "application/json", bytes.NewBufferString(body), token)
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected replace trusted contact relationship status 201, got %d: %s", response.StatusCode, responseBody)
	}
	var created struct {
		TrustedContactRelationship incidents.TrustedContactRelationship `json:"trusted_contact_relationship"`
	}
	if err := json.Unmarshal(responseBody, &created); err != nil {
		t.Fatalf("decode replacement trusted contact relationship: %v", err)
	}
	return created.TrustedContactRelationship
}

func trustedContactRelationshipBody(values map[string]string) string {
	body, err := json.Marshal(values)
	if err != nil {
		panic(err)
	}
	return string(body)
}

func accountIDForToken(t *testing.T, app *testApp, token string) string {
	t.Helper()
	response, body := requestWithAuth(t, app.privateHandler, http.MethodGet, "/v1/account", "", nil, token)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected get account status 200, got %d: %s", response.StatusCode, body)
	}
	var result struct {
		Account auth.Account `json:"account"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode account: %v", err)
	}
	return result.Account.ID
}
