package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/httpapi"
	"github.com/open-proofline/server/internal/incidents"
	"github.com/open-proofline/server/internal/relaycap"
)

const testRelayCapabilitySecret = "0123456789abcdef0123456789abcdef"

type relaySessionTestResponse struct {
	RelaySession struct {
		RelaySessionID    string    `json:"relay_session_id"`
		Capability        string    `json:"capability"`
		FanoutCapability  string    `json:"fanout_capability"`
		Role              string    `json:"role"`
		IncidentID        string    `json:"incident_id"`
		StreamID          string    `json:"stream_id"`
		ExpiresAt         time.Time `json:"expires_at"`
		MaxChunkBytes     int64     `json:"max_chunk_bytes"`
		MaxChunks         int       `json:"max_chunks"`
		AllowedMediaTypes []string  `json:"allowed_media_types"`
	} `json:"relay_session"`
}

func TestCreateRelaySessionIssuesBoundCapability(t *testing.T) {
	app := newTestAppWithOptions(t, httpapi.Options{
		MaxUploadBytes: 4096,
		RelayCapability: httpapi.RelayCapabilityConfig{
			Secret:    testRelayCapabilitySecret,
			TTL:       3 * time.Minute,
			MaxChunks: 7,
		},
	})
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "audio relay")

	response, body := post(t, app, "/v1/incidents/"+incidentID+"/streams/"+stream.ID+"/relay-session", "application/json", bytes.NewBufferString(`{}`))
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create relay session status = %d, want 201: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)
	for _, disallowed := range []string{
		app.authToken,
		"Authorization",
		"viewer_token",
		"incident_token",
		"object_key",
		"stored_path",
		"wrapped_key",
		"plaintext",
		"raw_key",
		"latitude",
		"longitude",
	} {
		if disallowed != "" && bytes.Contains(body, []byte(disallowed)) {
			t.Fatalf("relay session response exposed %q: %s", disallowed, body)
		}
	}

	var decoded relaySessionTestResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode relay session response: %v", err)
	}
	if decoded.RelaySession.Role != relaycap.RoleUpload {
		t.Fatalf("role = %q, want upload", decoded.RelaySession.Role)
	}
	if decoded.RelaySession.FanoutCapability == "" || decoded.RelaySession.FanoutCapability == decoded.RelaySession.Capability {
		t.Fatalf("fanout capability was not issued separately")
	}
	if decoded.RelaySession.IncidentID != incidentID || decoded.RelaySession.StreamID != stream.ID {
		t.Fatalf("binding = incident %q stream %q, want %q %q", decoded.RelaySession.IncidentID, decoded.RelaySession.StreamID, incidentID, stream.ID)
	}
	if decoded.RelaySession.MaxChunkBytes != 4096 || decoded.RelaySession.MaxChunks != 7 {
		t.Fatalf("limits = bytes %d chunks %d, want 4096 and 7", decoded.RelaySession.MaxChunkBytes, decoded.RelaySession.MaxChunks)
	}
	if len(decoded.RelaySession.AllowedMediaTypes) != 1 || decoded.RelaySession.AllowedMediaTypes[0] != incidents.MediaTypeAudio {
		t.Fatalf("allowed media types = %v, want audio", decoded.RelaySession.AllowedMediaTypes)
	}

	secret, err := relaycap.SecretBytes(testRelayCapabilitySecret)
	if err != nil {
		t.Fatalf("SecretBytes: %v", err)
	}
	capability, err := relaycap.Validate(secret, decoded.RelaySession.Capability, relaycap.ValidationContext{
		Role:           relaycap.RoleUpload,
		RelaySessionID: decoded.RelaySession.RelaySessionID,
		IncidentID:     incidentID,
		StreamID:       stream.ID,
		Now:            time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("validate returned capability: %v", err)
	}
	if capability.MaxChunkBytes != decoded.RelaySession.MaxChunkBytes || capability.MaxChunks != decoded.RelaySession.MaxChunks {
		t.Fatalf("capability limits = bytes %d chunks %d, response = bytes %d chunks %d", capability.MaxChunkBytes, capability.MaxChunks, decoded.RelaySession.MaxChunkBytes, decoded.RelaySession.MaxChunks)
	}
	fanoutCapability, err := relaycap.Validate(secret, decoded.RelaySession.FanoutCapability, relaycap.ValidationContext{
		Role:           relaycap.RoleFanout,
		RelaySessionID: decoded.RelaySession.RelaySessionID,
		IncidentID:     incidentID,
		StreamID:       stream.ID,
		Now:            time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("validate fanout capability: %v", err)
	}
	if fanoutCapability.MaxChunkBytes != decoded.RelaySession.MaxChunkBytes || fanoutCapability.MaxChunks != decoded.RelaySession.MaxChunks {
		t.Fatalf("fanout capability limits = bytes %d chunks %d, response = bytes %d chunks %d", fanoutCapability.MaxChunkBytes, fanoutCapability.MaxChunks, decoded.RelaySession.MaxChunkBytes, decoded.RelaySession.MaxChunks)
	}
}

func TestCreateRelaySessionRequiresConfiguredSecret(t *testing.T) {
	app := newTestApp(t)
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "audio relay")

	response, body := post(t, app, "/v1/incidents/"+incidentID+"/streams/"+stream.ID+"/relay-session", "application/json", bytes.NewBufferString(`{}`))
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("create relay session without secret status = %d, want 503: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "relay_capability_not_configured")
	if bytes.Contains(body, []byte(app.authToken)) {
		t.Fatalf("unconfigured relay response exposed auth token: %s", body)
	}
}

func TestCreateRelaySessionRejectsUnauthenticatedAndNonOpenState(t *testing.T) {
	app := newTestAppWithOptions(t, httpapi.Options{
		RelayCapability: httpapi.RelayCapabilityConfig{
			Secret:    testRelayCapabilitySecret,
			TTL:       3 * time.Minute,
			MaxChunks: 7,
		},
	})
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "audio relay")

	response, body := request(t, app.privateHandler, http.MethodPost, "/v1/incidents/"+incidentID+"/streams/"+stream.ID+"/relay-session", "application/json", strings.NewReader(`{}`))
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated relay session status = %d, want 401: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "authentication_required")

	response, body = post(t, app, "/v1/incidents/"+incidentID+"/streams/"+stream.ID+"/fail", "application/json", strings.NewReader(`{"failure_reason":"test"}`))
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("fail stream status = %d, want 200: %s", response.StatusCode, body)
	}

	response, body = post(t, app, "/v1/incidents/"+incidentID+"/streams/"+stream.ID+"/relay-session", "application/json", strings.NewReader(`{}`))
	response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("failed stream relay session status = %d, want 409: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "stream_not_open")
}

func TestCreateRelaySessionRejectsClosedIncident(t *testing.T) {
	app := newTestAppWithOptions(t, httpapi.Options{
		RelayCapability: httpapi.RelayCapabilityConfig{
			Secret:    testRelayCapabilitySecret,
			TTL:       3 * time.Minute,
			MaxChunks: 7,
		},
	})
	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "audio relay")

	response, body := post(t, app, "/v1/incidents/"+incidentID+"/close", "application/json", strings.NewReader(`{}`))
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("close incident status = %d, want 200: %s", response.StatusCode, body)
	}

	response, body = post(t, app, "/v1/incidents/"+incidentID+"/streams/"+stream.ID+"/relay-session", "application/json", strings.NewReader(`{}`))
	response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("closed incident relay session status = %d, want 409: %s", response.StatusCode, body)
	}
	assertErrorCode(t, body, "incident_closed")
}
