package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/open-proofline/server/internal/storage"
)

const (
	relayFanoutSessionHeader    = "X-Proofline-Relay-Session-ID"
	relayFanoutCapabilityHeader = "X-Proofline-Relay-Fanout-Capability"
	relayFanoutIncidentHeader   = "X-Proofline-Relay-Incident-ID"
	relayFanoutStreamHeader     = "X-Proofline-Relay-Stream-ID"
)

type relayFanoutAuthorizeInput struct {
	RelaySessionID string
	Capability     string
	IncidentID     string
	StreamID       string
}

type relayFanoutEvent struct {
	Type       string `json:"type"`
	State      string `json:"state"`
	IncidentID string `json:"incident_id"`
	StreamID   string `json:"stream_id"`
	ChunkIndex int    `json:"chunk_index"`
	MediaType  string `json:"media_type"`
	ByteSize   int64  `json:"byte_size"`
	SHA256Hex  string `json:"sha256_hex"`
	PayloadB64 string `json:"payload_b64"`
}

type relayFanoutHub struct {
	mu          sync.Mutex
	nextID      uint64
	subscribers map[string]map[uint64]chan relayFanoutEvent
}

func newRelayFanoutHub() *relayFanoutHub {
	return &relayFanoutHub{
		subscribers: make(map[string]map[uint64]chan relayFanoutEvent),
	}
}

func (u *relayUploader) fanoutSubscribe(w http.ResponseWriter, r *http.Request) {
	if !u.configured() {
		writeError(w, http.StatusServiceUnavailable, "relay_core_not_configured", "relay core forwarding is not configured")
		return
	}
	input, ok := parseRelayFanoutHeaders(w, r)
	if !ok {
		return
	}
	if !u.coreFanoutAuthorize(w, r.Context(), input) {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "relay_fanout_unavailable", "relay fanout streaming is unavailable")
		return
	}
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})

	events, release := u.fanout.register(input)
	defer release()

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if !writeSSEEvent(w, "relay_ready", map[string]string{
		"status":      "subscribed",
		"state":       "near_live_unconfirmed",
		"incident_id": input.IncidentID,
		"stream_id":   input.StreamID,
	}) {
		return
	}
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-events:
			if !writeSSEEvent(w, "relay_chunk", event) {
				return
			}
			flusher.Flush()
		}
	}
}

func parseRelayFanoutHeaders(w http.ResponseWriter, r *http.Request) (relayFanoutAuthorizeInput, bool) {
	input := relayFanoutAuthorizeInput{
		RelaySessionID: strings.TrimSpace(r.Header.Get(relayFanoutSessionHeader)),
		Capability:     strings.TrimSpace(r.Header.Get(relayFanoutCapabilityHeader)),
		IncidentID:     strings.TrimSpace(r.Header.Get(relayFanoutIncidentHeader)),
		StreamID:       strings.TrimSpace(r.Header.Get(relayFanoutStreamHeader)),
	}
	if input.RelaySessionID == "" || input.Capability == "" || input.IncidentID == "" || input.StreamID == "" {
		writeError(w, http.StatusBadRequest, "invalid_relay_fanout_request", "relay fanout session headers are required")
		return relayFanoutAuthorizeInput{}, false
	}
	return input, true
}

func (u *relayUploader) coreFanoutAuthorize(w http.ResponseWriter, ctx context.Context, input relayFanoutAuthorizeInput) bool {
	body, err := json.Marshal(input.coreFields())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "relay_internal_error", "relay fanout failed")
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.cfg.CoreBaseURL+"/v1/relay/fanout-authorize", bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "core_fanout_unavailable", "core fanout authorization is unavailable")
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(relayCoreServiceAuthHeader, u.cfg.CoreServiceAuthToken)

	resp, err := u.client.Do(req)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "core_fanout_unavailable", "core fanout authorization is unavailable")
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeCoreRejected(w, resp, "core_fanout_rejected", "core rejected relay fanout")
		return false
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
	return true
}

func (input relayFanoutAuthorizeInput) coreFields() map[string]any {
	return map[string]any{
		"relay_session_id": input.RelaySessionID,
		"capability":       input.Capability,
		"incident_id":      input.IncidentID,
		"stream_id":        input.StreamID,
	}
}

func (u *relayUploader) publishRelayFanout(input relayUploadMetadata, temp *storage.TempUpload) {
	if u == nil || u.fanout == nil || temp == nil {
		return
	}
	key := u.fanoutKey(input)
	if !u.fanout.hasSubscribers(key) {
		return
	}
	payloadB64, byteSize, err := relayFanoutPayloadB64(temp.Path)
	if err != nil || byteSize != input.ByteSize {
		return
	}
	u.fanout.broadcast(key, relayFanoutEventFromUpload(input, payloadB64, byteSize))
}

func (u *relayUploader) fanoutKey(input relayUploadMetadata) string {
	return relayFanoutKey(input.RelaySessionID, input.IncidentID, input.StreamID)
}

func relayFanoutKey(relaySessionID, incidentID, streamID string) string {
	return relaySessionID + "\x00" + incidentID + "\x00" + streamID
}

func (h *relayFanoutHub) register(input relayFanoutAuthorizeInput) (<-chan relayFanoutEvent, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextID++
	id := h.nextID
	key := relayFanoutKey(input.RelaySessionID, input.IncidentID, input.StreamID)
	ch := make(chan relayFanoutEvent, 8)
	if h.subscribers[key] == nil {
		h.subscribers[key] = make(map[uint64]chan relayFanoutEvent)
	}
	h.subscribers[key][id] = ch
	return ch, func() {
		h.unregister(key, id)
	}
}

func (h *relayFanoutHub) unregister(key string, id uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subscribers[key], id)
	if len(h.subscribers[key]) == 0 {
		delete(h.subscribers, key)
	}
}

func (h *relayFanoutHub) hasSubscribers(key string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subscribers[key]) > 0
}

func (h *relayFanoutHub) broadcast(key string, event relayFanoutEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.subscribers[key] {
		select {
		case ch <- event:
		default:
		}
	}
}

func writeSSEEvent(w http.ResponseWriter, eventName string, payload any) bool {
	data, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", eventName); err != nil {
		return false
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return false
	}
	return true
}

func relayFanoutPayloadB64(path string) (string, int64, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return "", 0, err
	}
	return base64.StdEncoding.EncodeToString(payload), int64(len(payload)), nil
}

func relayFanoutEventFromUpload(input relayUploadMetadata, payloadB64 string, byteSize int64) relayFanoutEvent {
	return relayFanoutEvent{
		Type:       "relay_chunk",
		State:      "near_live_unconfirmed",
		IncidentID: input.IncidentID,
		StreamID:   input.StreamID,
		ChunkIndex: input.ChunkIndex,
		MediaType:  input.MediaType,
		ByteSize:   byteSize,
		SHA256Hex:  input.SHA256Hex,
		PayloadB64: payloadB64,
	}
}
