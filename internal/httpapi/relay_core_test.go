package httpapi_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/httpapi"
	"github.com/open-proofline/server/internal/incidents"
	"github.com/open-proofline/server/internal/relaycap"
)

const testRelayServiceAuthToken = "relay-service-auth-token-1234567890"

func TestRelayPreflightAndCommitDurableChunk(t *testing.T) {
	app := newRelayCoreTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "relay audio")
	session := createRelaySessionForTest(t, app, incidentID, stream.ID)
	payload := testPQPayload(t, incidentID, stream.ID, 1, incidents.MediaTypeAudio, []byte("relay encrypted chunk"))

	response, body := relayPreflightRequest(t, app, relayChunkTestRequest{
		Session:    session,
		IncidentID: incidentID,
		StreamID:   stream.ID,
		ChunkIndex: 1,
		MediaType:  incidents.MediaTypeAudio,
		Payload:    payload,
	}, testRelayServiceAuthToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("relay preflight status = %d, want 200: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)
	assertRelayResponseRedacted(t, body, app.authToken, testRelayServiceAuthToken, session.RelaySession.Capability)
	assertJSONField(t, body, "status", "accepted")

	response, body = relayCommitRequest(t, app, relayChunkTestRequest{
		Session:    session,
		IncidentID: incidentID,
		StreamID:   stream.ID,
		ChunkIndex: 1,
		MediaType:  incidents.MediaTypeAudio,
		Payload:    payload,
	}, testRelayServiceAuthToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("relay commit status = %d, want 201: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)
	assertRelayResponseRedacted(t, body, app.authToken, testRelayServiceAuthToken, session.RelaySession.Capability)
	assertJSONField(t, body, "status", "committed")

	chunk, err := incidents.NewRepository(app.db).GetChunkByIdentity(t.Context(), incidentID, stream.ID, incidents.MediaTypeAudio, 1)
	if err != nil {
		t.Fatalf("get committed relay chunk: %v", err)
	}
	if chunk.SHA256Hex != sha256Hex(payload) || chunk.StoredPath == "" {
		t.Fatalf("committed chunk hash/path = %q/%q", chunk.SHA256Hex, chunk.StoredPath)
	}
}

func TestRelayRoutesRequireSeparateServiceAuth(t *testing.T) {
	app := newRelayCoreTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "relay audio")
	session := createRelaySessionForTest(t, app, incidentID, stream.ID)
	payload := testPQPayload(t, incidentID, stream.ID, 1, incidents.MediaTypeAudio, []byte("relay encrypted chunk"))
	request := relayChunkTestRequest{
		Session:    session,
		IncidentID: incidentID,
		StreamID:   stream.ID,
		ChunkIndex: 1,
		MediaType:  incidents.MediaTypeAudio,
		Payload:    payload,
	}

	response, body := relayPreflightRequest(t, app, request, "")
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing service auth status = %d, want 401: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "relay_service_auth_required")

	bodyReader := bytes.NewReader(relayPreflightBody(t, request))
	response, body = requestWithAuthAndHeaders(t, app.privateHandler, http.MethodPost, "/v1/relay/preflight", "application/json", bodyReader, app.authToken, nil)
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("user bearer service auth status = %d, want 401: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "relay_service_auth_required")

	unconfigured := newTestAppWithOptions(t, httpapi.Options{
		RelayCapability: httpapi.RelayCapabilityConfig{
			Secret:    testRelayCapabilitySecret,
			TTL:       3 * time.Minute,
			MaxChunks: 7,
		},
	})
	response, body = relayPreflightRequest(t, unconfigured, request, testRelayServiceAuthToken)
	response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured service auth status = %d, want 503: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "relay_service_auth_not_configured")
}

func TestRelayFanoutAuthorizeRequiresFanoutCapability(t *testing.T) {
	app := newRelayCoreTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "relay audio")
	session := createRelaySessionForTest(t, app, incidentID, stream.ID)

	response, body := relayFanoutAuthorizeRequest(t, app, session, session.RelaySession.FanoutCapability, testRelayServiceAuthToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("relay fanout authorize status = %d, want 200: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)
	assertRelayResponseRedacted(t, body, app.authToken, testRelayServiceAuthToken, session.RelaySession.Capability, session.RelaySession.FanoutCapability)
	assertJSONField(t, body, "status", "authorized")

	response, body = relayFanoutAuthorizeRequest(t, app, session, session.RelaySession.Capability, testRelayServiceAuthToken)
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("upload capability fanout status = %d, want 403: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "relay_capability_wrong_role")
	assertRelayResponseRedacted(t, body, app.authToken, testRelayServiceAuthToken, session.RelaySession.Capability, session.RelaySession.FanoutCapability)

	response, body = relayFanoutAuthorizeRequest(t, app, session, session.RelaySession.FanoutCapability, "")
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing service auth fanout status = %d, want 401: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "relay_service_auth_required")
}

func TestRelayPreflightRejectsInvalidCapabilitiesAndLimits(t *testing.T) {
	app := newRelayCoreTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "relay audio")
	session := createRelaySessionForTest(t, app, incidentID, stream.ID)
	payload := testPQPayload(t, incidentID, stream.ID, 1, incidents.MediaTypeAudio, []byte("relay encrypted chunk"))

	for name, tt := range map[string]struct {
		session relaySessionTestResponse
		want    string
	}{
		"tampered": {
			session: relaySessionWithCapability(session, session.RelaySession.Capability+"tampered"),
			want:    "relay_capability_invalid",
		},
		"expired": {
			session: relaySessionWithCapability(session, signRelayCapabilityForTest(t, "expired-session", relaycap.RoleUpload, incidentID, stream.ID, time.Now().Add(-10*time.Minute), time.Now().Add(-5*time.Minute), 4096, 7, []string{incidents.MediaTypeAudio})),
			want:    "relay_capability_expired",
		},
		"wrong role": {
			session: relaySessionWithCapability(session, signRelayCapabilityForTest(t, "wrong-role-session", "fanout", incidentID, stream.ID, time.Now(), time.Now().Add(5*time.Minute), 4096, 7, []string{incidents.MediaTypeAudio})),
			want:    "relay_capability_wrong_role",
		},
		"limit exceeded": {
			session: relaySessionWithCapability(session, signRelayCapabilityForTest(t, "small-session", relaycap.RoleUpload, incidentID, stream.ID, time.Now(), time.Now().Add(5*time.Minute), 1, 7, []string{incidents.MediaTypeAudio})),
			want:    "relay_capability_limit_exceeded",
		},
	} {
		t.Run(name, func(t *testing.T) {
			testSession := tt.session
			response, body := relayPreflightRequest(t, app, relayChunkTestRequest{
				Session:    testSession,
				IncidentID: incidentID,
				StreamID:   stream.ID,
				ChunkIndex: 1,
				MediaType:  incidents.MediaTypeAudio,
				Payload:    payload,
			}, testRelayServiceAuthToken)
			response.Body.Close()
			if response.StatusCode < 400 {
				t.Fatalf("relay preflight status = %d, want failure: %s", response.StatusCode, body)
			}
			assertErrorCode(t, body, tt.want)
			assertRelayResponseRedacted(t, body, app.authToken, testRelayServiceAuthToken, testSession.RelaySession.Capability)
		})
	}
}

func TestRelayPreflightRejectsWrongStreamBinding(t *testing.T) {
	app := newRelayCoreTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	firstStream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "relay audio")
	secondStream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "relay audio 2")
	session := createRelaySessionForTest(t, app, incidentID, firstStream.ID)
	payload := testPQPayload(t, incidentID, secondStream.ID, 1, incidents.MediaTypeAudio, []byte("relay encrypted chunk"))

	response, body := relayPreflightRequest(t, app, relayChunkTestRequest{
		Session:    session,
		IncidentID: incidentID,
		StreamID:   secondStream.ID,
		ChunkIndex: 1,
		MediaType:  incidents.MediaTypeAudio,
		Payload:    payload,
	}, testRelayServiceAuthToken)
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("wrong stream status = %d, want 403: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "relay_capability_wrong_binding")
}

func TestRelayCommitRejectsHashMismatchAndPreservesDirectUpload(t *testing.T) {
	app := newRelayCoreTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	relayStream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "relay audio")
	session := createRelaySessionForTest(t, app, incidentID, relayStream.ID)
	payload := testPQPayload(t, incidentID, relayStream.ID, 1, incidents.MediaTypeAudio, []byte("relay encrypted chunk"))

	response, body := relayCommitRequest(t, app, relayChunkTestRequest{
		Session:     session,
		IncidentID:  incidentID,
		StreamID:    relayStream.ID,
		ChunkIndex:  1,
		MediaType:   incidents.MediaTypeAudio,
		Payload:     payload,
		SHAOverride: strings.Repeat("a", 64),
	}, testRelayServiceAuthToken)
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("hash mismatch status = %d, want 400: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "hash_mismatch")
	assertRelayResponseRedacted(t, body, app.authToken, testRelayServiceAuthToken, session.RelaySession.Capability)

	directStream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "direct audio")
	directPayload := []byte("direct encrypted chunk")
	response, body = uploadChunkWithStream(t, app, incidentID, directStream.ID, 1, incidents.MediaTypeAudio, directPayload, sha256Hex(directPayload))
	response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("direct upload status = %d, want 201: %s", response.StatusCode, body)
	}
}

type relayChunkTestRequest struct {
	Session     relaySessionTestResponse
	IncidentID  string
	StreamID    string
	ChunkIndex  int
	MediaType   string
	Payload     []byte
	SHAOverride string
}

func newRelayCoreTestApp(t *testing.T) *testApp {
	t.Helper()
	return newTestAppWithOptions(t, httpapi.Options{
		MaxUploadBytes: 4096,
		RelayCapability: httpapi.RelayCapabilityConfig{
			Secret:    testRelayCapabilitySecret,
			TTL:       3 * time.Minute,
			MaxChunks: 7,
		},
		RelayService: httpapi.RelayServiceConfig{
			AuthToken: testRelayServiceAuthToken,
		},
	})
}

func createRelaySessionForTest(t *testing.T, app *testApp, incidentID, streamID string) relaySessionTestResponse {
	t.Helper()
	response, body := post(t, app, "/v1/incidents/"+incidentID+"/streams/"+streamID+"/relay-session", "application/json", bytes.NewBufferString(`{}`))
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create relay session status = %d, want 201: %s", response.StatusCode, body)
	}
	var decoded relaySessionTestResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode relay session: %v", err)
	}
	return decoded
}

func relayPreflightRequest(t *testing.T, app *testApp, request relayChunkTestRequest, serviceToken string) (*http.Response, []byte) {
	t.Helper()
	headers := relayServiceHeaders(serviceToken)
	return requestWithAuthAndHeaders(t, app.privateHandler, http.MethodPost, "/v1/relay/preflight", "application/json", bytes.NewReader(relayPreflightBody(t, request)), "", headers)
}

func relayFanoutAuthorizeRequest(t *testing.T, app *testApp, session relaySessionTestResponse, capability, serviceToken string) (*http.Response, []byte) {
	t.Helper()
	body, err := json.Marshal(map[string]string{
		"relay_session_id": session.RelaySession.RelaySessionID,
		"capability":       capability,
		"incident_id":      session.RelaySession.IncidentID,
		"stream_id":        session.RelaySession.StreamID,
	})
	if err != nil {
		t.Fatalf("marshal relay fanout authorize: %v", err)
	}
	return requestWithAuthAndHeaders(t, app.privateHandler, http.MethodPost, "/v1/relay/fanout-authorize", "application/json", bytes.NewReader(body), "", relayServiceHeaders(serviceToken))
}

func relayPreflightBody(t *testing.T, request relayChunkTestRequest) []byte {
	t.Helper()
	sha := sha256Hex(request.Payload)
	if request.SHAOverride != "" {
		sha = request.SHAOverride
	}
	body, err := json.Marshal(map[string]any{
		"relay_session_id":  request.Session.RelaySession.RelaySessionID,
		"capability":        request.Session.RelaySession.Capability,
		"incident_id":       request.IncidentID,
		"stream_id":         request.StreamID,
		"chunk_index":       request.ChunkIndex,
		"media_type":        request.MediaType,
		"started_at":        testChunkStartedAtString(),
		"ended_at":          testChunkEndedAtString(),
		"byte_size":         len(request.Payload),
		"sha256_hex":        sha,
		"original_filename": "relay.enc",
	})
	if err != nil {
		t.Fatalf("marshal relay preflight: %v", err)
	}
	return body
}

func relayCommitRequest(t *testing.T, app *testApp, request relayChunkTestRequest, serviceToken string) (*http.Response, []byte) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range relayRequestFields(request) {
		must(t, writer.WriteField(name, value))
	}
	fileWriter, err := writer.CreateFormFile("file", "relay.enc")
	if err != nil {
		t.Fatalf("create relay file field: %v", err)
	}
	if _, err := fileWriter.Write(request.Payload); err != nil {
		t.Fatalf("write relay payload: %v", err)
	}
	must(t, writer.Close())
	return requestWithAuthAndHeaders(t, app.privateHandler, http.MethodPost, "/v1/relay/commit", writer.FormDataContentType(), &body, "", relayServiceHeaders(serviceToken))
}

func relayRequestFields(request relayChunkTestRequest) map[string]string {
	sha := sha256Hex(request.Payload)
	if request.SHAOverride != "" {
		sha = request.SHAOverride
	}
	return map[string]string{
		"relay_session_id":  request.Session.RelaySession.RelaySessionID,
		"capability":        request.Session.RelaySession.Capability,
		"incident_id":       request.IncidentID,
		"stream_id":         request.StreamID,
		"chunk_index":       strconv.Itoa(request.ChunkIndex),
		"media_type":        request.MediaType,
		"started_at":        testChunkStartedAtString(),
		"ended_at":          testChunkEndedAtString(),
		"byte_size":         strconv.Itoa(len(request.Payload)),
		"sha256_hex":        sha,
		"original_filename": "relay.enc",
	}
}

func relayServiceHeaders(serviceToken string) map[string]string {
	headers := map[string]string{}
	if serviceToken != "" {
		headers["X-Proofline-Relay-Service-Token"] = serviceToken
	}
	return headers
}

func relaySessionWithCapability(session relaySessionTestResponse, capability string) relaySessionTestResponse {
	session.RelaySession.Capability = capability
	parts := strings.Split(capability, ".")
	if len(parts) == 3 {
		session.RelaySession.RelaySessionID = relaySessionIDFromCapabilityPayload(parts[1])
	}
	return session
}

func relaySessionIDFromCapabilityPayload(encodedPayload string) string {
	payload, err := relaycapPayloadDecode(encodedPayload)
	if err != nil {
		return ""
	}
	var decoded struct {
		RelaySessionID string `json:"relay_session_id"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return ""
	}
	return decoded.RelaySessionID
}

func relaycapPayloadDecode(encodedPayload string) ([]byte, error) {
	return relaycapRawURLEncoding().DecodeString(encodedPayload)
}

func relaycapRawURLEncoding() *base64.Encoding {
	return base64.RawURLEncoding
}

func signRelayCapabilityForTest(t *testing.T, sessionID, role, incidentID, streamID string, issuedAt, expiresAt time.Time, maxChunkBytes int64, maxChunks int, mediaTypes []string) string {
	t.Helper()
	secret, err := relaycap.SecretBytes(testRelayCapabilitySecret)
	if err != nil {
		t.Fatalf("SecretBytes: %v", err)
	}
	token, err := relaycap.Sign(secret, relaycap.Capability{
		Version:           relaycap.Version,
		RelaySessionID:    sessionID,
		Role:              role,
		IncidentID:        incidentID,
		StreamID:          streamID,
		IssuedAtUnix:      issuedAt.Unix(),
		ExpiresAtUnix:     expiresAt.Unix(),
		MaxChunkBytes:     maxChunkBytes,
		MaxChunks:         maxChunks,
		AllowedMediaTypes: mediaTypes,
	})
	if err != nil {
		t.Fatalf("Sign relay capability: %v", err)
	}
	return token
}

func assertRelayResponseRedacted(t *testing.T, body []byte, disallowed ...string) {
	t.Helper()
	for _, value := range disallowed {
		if value != "" && bytes.Contains(body, []byte(value)) {
			t.Fatalf("relay response exposed %q: %s", value, body)
		}
	}
	for _, value := range []string{
		"stored_path",
		"object_key",
		"Authorization",
		"plaintext",
		"raw_key",
		"wrapped_key",
		"latitude",
		"longitude",
	} {
		if bytes.Contains(body, []byte(value)) {
			t.Fatalf("relay response exposed %q: %s", value, body)
		}
	}
}

func assertJSONField(t *testing.T, body []byte, field string, want string) {
	t.Helper()
	var decoded map[string]map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode JSON response: %v", err)
	}
	for _, object := range decoded {
		if got, ok := object[field].(string); ok && got == want {
			return
		}
	}
	t.Fatalf("field %q did not equal %q in %s", field, want, body)
}
