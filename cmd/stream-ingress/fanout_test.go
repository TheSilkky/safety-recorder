package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRelayFanoutDeliversEncryptedUnconfirmedChunk(t *testing.T) {
	payload := []byte("opaque encrypted relay fanout chunk")
	input := relayUploadTestMetadata("relay-session", "upload-capability-token", "inc_test", "str_test", 1, "audio", payload)
	fanoutCapability := "fanout-capability-token"
	var sawFanoutAuthorize atomic.Bool
	var sawPreflight atomic.Bool
	var sawCommit atomic.Bool

	handler := newRelayUploadTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertCoreServiceAuth(t, r)
		switch r.URL.Path {
		case "/v1/relay/fanout-authorize":
			sawFanoutAuthorize.Store(true)
			assertCoreFanoutAuthorizeBody(t, r, input, fanoutCapability)
			relayUploadWriteJSON(w, http.StatusOK, map[string]any{
				"relay_fanout": map[string]any{"status": "authorized"},
			})
		case "/v1/relay/preflight":
			sawPreflight.Store(true)
			assertCorePreflightBody(t, r, input)
			relayUploadWriteJSON(w, http.StatusOK, map[string]any{
				"relay_preflight": map[string]any{"status": "accepted"},
			})
		case "/v1/relay/commit":
			sawCommit.Store(true)
			assertCoreCommitMultipart(t, r, input, payload)
			relayUploadWriteJSON(w, http.StatusCreated, relayUploadCoreCommitResponse(input))
		default:
			t.Fatalf("unexpected core path %s", r.URL.Path)
		}
	}), streamIngressConfig{})
	relayServer := httptest.NewServer(handler.Handler)
	t.Cleanup(relayServer.Close)

	client := &http.Client{}
	subscribeRequest, err := http.NewRequest(http.MethodGet, relayServer.URL+"/fanout/subscribe", nil)
	if err != nil {
		t.Fatalf("new fanout request: %v", err)
	}
	setRelayFanoutHeaders(subscribeRequest, input, fanoutCapability)
	subscribeResponse, err := client.Do(subscribeRequest)
	if err != nil {
		t.Fatalf("subscribe fanout: %v", err)
	}
	defer subscribeResponse.Body.Close()
	if subscribeResponse.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(subscribeResponse.Body)
		t.Fatalf("fanout subscribe status = %d, want 200: %s", subscribeResponse.StatusCode, body)
	}
	reader := bufio.NewReader(subscribeResponse.Body)
	ready := readRelaySSEEventWithin(t, reader)
	if ready.Event != "relay_ready" {
		t.Fatalf("ready event = %q, want relay_ready", ready.Event)
	}

	uploadResponse, uploadBody := postRelayUploadHTTP(t, client, relayServer.URL, input, payload)
	defer uploadResponse.Body.Close()
	if uploadResponse.StatusCode != http.StatusCreated {
		t.Fatalf("relay upload status = %d, want 201: %s", uploadResponse.StatusCode, uploadBody)
	}

	chunkEvent := readRelaySSEEventWithin(t, reader)
	if chunkEvent.Event != "relay_chunk" {
		t.Fatalf("chunk event = %q, want relay_chunk", chunkEvent.Event)
	}
	var decoded relayFanoutEvent
	if err := json.Unmarshal(chunkEvent.Data, &decoded); err != nil {
		t.Fatalf("decode relay fanout event: %v", err)
	}
	if decoded.State != "near_live_unconfirmed" || decoded.Type != "relay_chunk" {
		t.Fatalf("fanout state/type = %q/%q, want near_live_unconfirmed/relay_chunk", decoded.State, decoded.Type)
	}
	if decoded.IncidentID != input.IncidentID || decoded.StreamID != input.StreamID ||
		decoded.ChunkIndex != input.ChunkIndex || decoded.MediaType != input.MediaType ||
		decoded.ByteSize != input.ByteSize || decoded.SHA256Hex != input.SHA256Hex {
		t.Fatalf("fanout metadata = %+v, want upload metadata", decoded)
	}
	gotPayload, err := base64.StdEncoding.DecodeString(decoded.PayloadB64)
	if err != nil {
		t.Fatalf("decode fanout payload: %v", err)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Fatalf("fanout payload = %q, want original encrypted chunk bytes", gotPayload)
	}
	assertRedactedRelayUploadBody(t, chunkEvent.Data, testCoreServiceAuthToken, input.Capability, fanoutCapability, input.RelaySessionID)
	if bytes.Contains(chunkEvent.Data, payload) {
		t.Fatalf("fanout event exposed raw encrypted bytes outside payload_b64: %s", chunkEvent.Data)
	}
	if !sawFanoutAuthorize.Load() || !sawPreflight.Load() || !sawCommit.Load() {
		t.Fatalf("core fanout/preflight/commit seen = %v/%v/%v, want true/true/true", sawFanoutAuthorize.Load(), sawPreflight.Load(), sawCommit.Load())
	}
	assertRelayTempDirEmpty(t, handler)
}

func TestRelayFanoutRejectsUnauthorizedSubscriber(t *testing.T) {
	payload := []byte("opaque encrypted relay fanout chunk")
	input := relayUploadTestMetadata("relay-session", "upload-capability-token", "inc_test", "str_test", 1, "audio", payload)
	fanoutCapability := "wrong-role-capability-token"
	handler := newRelayUploadTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertCoreServiceAuth(t, r)
		if r.URL.Path != "/v1/relay/fanout-authorize" {
			t.Fatalf("unexpected core path %s", r.URL.Path)
		}
		assertCoreFanoutAuthorizeBody(t, r, input, fanoutCapability)
		relayUploadWriteJSON(w, http.StatusForbidden, relayUploadError("relay_capability_wrong_role"))
	}), streamIngressConfig{})

	request := httptest.NewRequest(http.MethodGet, "/fanout/subscribe", nil)
	setRelayFanoutHeaders(request, input, fanoutCapability)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("fanout subscribe status = %d, want 403: %s", recorder.Code, recorder.Body.String())
	}
	assertErrorCode(t, recorder.Body.Bytes(), "core_fanout_rejected")
	if !strings.Contains(recorder.Body.String(), `"core_error_code":"relay_capability_wrong_role"`) {
		t.Fatalf("fanout rejection omitted safe core error code: %s", recorder.Body.String())
	}
	assertRedactedRelayUploadBody(t, recorder.Body.Bytes(), testCoreServiceAuthToken, input.Capability, fanoutCapability, input.RelaySessionID)
}

func TestRelayFanoutHubDoesNotReplayToLateSubscriber(t *testing.T) {
	payload := []byte("opaque encrypted relay fanout chunk")
	input := relayUploadTestMetadata("relay-session", "upload-capability-token", "inc_test", "str_test", 1, "audio", payload)
	hub := newRelayFanoutHub()
	hub.broadcast(relayFanoutKey(input.RelaySessionID, input.IncidentID, input.StreamID), relayFanoutEventFromUpload(input, base64.StdEncoding.EncodeToString(payload), int64(len(payload))))

	events, release := hub.register(relayFanoutAuthorizeInput{
		RelaySessionID: input.RelaySessionID,
		IncidentID:     input.IncidentID,
		StreamID:       input.StreamID,
	})
	defer release()

	select {
	case event := <-events:
		t.Fatalf("late subscriber received replayed event: %+v", event)
	case <-time.After(25 * time.Millisecond):
	}
}

func setRelayFanoutHeaders(r *http.Request, input relayUploadMetadata, fanoutCapability string) {
	r.Header.Set(relayFanoutSessionHeader, input.RelaySessionID)
	r.Header.Set(relayFanoutCapabilityHeader, fanoutCapability)
	r.Header.Set(relayFanoutIncidentHeader, input.IncidentID)
	r.Header.Set(relayFanoutStreamHeader, input.StreamID)
}

func postRelayUploadHTTP(t *testing.T, client *http.Client, relayBaseURL string, input relayUploadMetadata, payload []byte) (*http.Response, []byte) {
	t.Helper()
	contentType, body, err := relayUploadMultipartBody(input, payload)
	if err != nil {
		t.Fatalf("relay upload multipart body: %v", err)
	}
	request, err := http.NewRequest(http.MethodPost, relayBaseURL+"/upload/complete-chunk", body)
	if err != nil {
		t.Fatalf("new relay upload request: %v", err)
	}
	request.Header.Set("Content-Type", contentType)
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("relay upload request: %v", err)
	}
	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read relay upload response: %v", err)
	}
	return response, bodyBytes
}

func assertCoreFanoutAuthorizeBody(t *testing.T, r *http.Request, want relayUploadMetadata, wantCapability string) {
	t.Helper()
	var got map[string]string
	if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
		t.Fatalf("decode core fanout authorize body: %v", err)
	}
	for field, wantValue := range map[string]string{
		"relay_session_id": want.RelaySessionID,
		"capability":       wantCapability,
		"incident_id":      want.IncidentID,
		"stream_id":        want.StreamID,
	} {
		if got[field] != wantValue {
			t.Fatalf("core fanout authorize %s = %v, want %q", field, got[field], wantValue)
		}
	}
}

type relaySSEEvent struct {
	Event string
	Data  []byte
}

func readRelaySSEEventWithin(t *testing.T, reader *bufio.Reader) relaySSEEvent {
	t.Helper()
	type result struct {
		event relaySSEEvent
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		event, err := readRelaySSEEvent(reader)
		ch <- result{event: event, err: err}
	}()
	select {
	case got := <-ch:
		if got.err != nil {
			t.Fatalf("read SSE event: %v", got.err)
		}
		return got.event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for SSE event")
		return relaySSEEvent{}
	}
}

func readRelaySSEEvent(reader *bufio.Reader) (relaySSEEvent, error) {
	var event relaySSEEvent
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return relaySSEEvent{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if event.Event == "" && len(event.Data) == 0 {
				return relaySSEEvent{}, errors.New("empty SSE event")
			}
			return event, nil
		}
		if value, ok := strings.CutPrefix(line, "event: "); ok {
			event.Event = value
			continue
		}
		if value, ok := strings.CutPrefix(line, "data: "); ok {
			event.Data = []byte(value)
		}
	}
}
