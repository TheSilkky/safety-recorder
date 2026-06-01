package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type client struct {
	httpClient   *http.Client
	apiBase      string
	viewerBase   string
	sessionToken string
}

type createIncidentResponse struct {
	IncidentID string `json:"incident_id"`
	Status     string `json:"status"`
}

type loginResponse struct {
	Token string `json:"token"`
}

type createIncidentTokenResponse struct {
	TokenID    string     `json:"token_id"`
	IncidentID string     `json:"incident_id"`
	Token      string     `json:"token"`
	Label      string     `json:"label,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

type createMediaStreamResponse struct {
	Stream mediaStream `json:"stream"`
}

type mediaStreamResponse struct {
	Stream mediaStream `json:"stream"`
}

type mediaStream struct {
	ID                 string     `json:"id"`
	IncidentID         string     `json:"incident_id"`
	MediaType          string     `json:"media_type"`
	Label              string     `json:"label,omitempty"`
	Status             string     `json:"status"`
	ExpectedChunkCount *int       `json:"expected_chunk_count,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
}

type apiErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c client) createIncident(ctx context.Context) (string, error) {
	request := map[string]string{
		"client_label": "simclient",
		"notes":        "simulated incident",
	}
	var response createIncidentResponse
	if err := c.postJSON(ctx, "/v1/incidents", request, http.StatusCreated, &response); err != nil {
		return "", fmt.Errorf("create incident: %w", err)
	}
	if response.IncidentID == "" {
		return "", fmt.Errorf("create incident: empty incident_id in response")
	}
	return response.IncidentID, nil
}

func (c client) login(ctx context.Context, username, password string) (string, error) {
	request := map[string]string{
		"username": username,
		"password": password,
	}
	var response loginResponse
	if err := c.postJSON(ctx, "/v1/auth/login", request, http.StatusCreated, &response); err != nil {
		return "", fmt.Errorf("login: %w", err)
	}
	if response.Token == "" {
		return "", fmt.Errorf("login: empty token in response")
	}
	return response.Token, nil
}

func (c client) createIncidentToken(ctx context.Context, incidentID string) (string, error) {
	request := map[string]string{"label": "simclient"}
	var response createIncidentTokenResponse
	path := "/v1/incidents/" + url.PathEscape(incidentID) + "/incident-tokens"
	if err := c.postJSON(ctx, path, request, http.StatusCreated, &response); err != nil {
		return "", fmt.Errorf("create incident token: %w", err)
	}
	if response.Token == "" {
		return "", fmt.Errorf("create incident token: empty token in response")
	}
	return response.Token, nil
}

func (c client) createMediaStream(ctx context.Context, incidentID, mediaType string) (string, error) {
	request := map[string]string{
		"media_type": mediaType,
		"label":      mediaType + " recording",
	}
	var response createMediaStreamResponse
	path := "/v1/incidents/" + url.PathEscape(incidentID) + "/streams"
	if err := c.postJSON(ctx, path, request, http.StatusCreated, &response); err != nil {
		return "", fmt.Errorf("create media stream: %w", err)
	}
	if response.Stream.ID == "" {
		return "", fmt.Errorf("create media stream: empty stream id in response")
	}
	return response.Stream.ID, nil
}

func (c client) createCheckin(ctx context.Context, incidentID string, chunkIndex int) error {
	battery := 100 - chunkIndex
	if battery < 1 {
		battery = 1
	}
	request := map[string]any{
		"device_battery_percent": battery,
		"device_network":         "simulated",
		"latitude":               37.7749,
		"longitude":              -122.4194,
		"accuracy_meters":        15,
	}
	path := "/v1/incidents/" + url.PathEscape(incidentID) + "/checkins"
	if err := c.postJSON(ctx, path, request, http.StatusCreated, nil); err != nil {
		return fmt.Errorf("create checkin: %w", err)
	}
	return nil
}

func (c client) completeMediaStream(ctx context.Context, incidentID, streamID string, expectedChunkCount int) error {
	request := map[string]int{"expected_chunk_count": expectedChunkCount}
	var response mediaStreamResponse
	path := "/v1/incidents/" + url.PathEscape(incidentID) + "/streams/" + url.PathEscape(streamID) + "/complete"
	if err := c.postJSON(ctx, path, request, http.StatusOK, &response); err != nil {
		return fmt.Errorf("complete media stream: %w", err)
	}
	if response.Stream.Status != "complete" {
		return fmt.Errorf("complete media stream: expected complete status, got %q", response.Stream.Status)
	}
	return nil
}

func (c client) failMediaStream(ctx context.Context, incidentID, streamID, reason string) error {
	request := map[string]string{"failure_reason": reason}
	var response mediaStreamResponse
	path := "/v1/incidents/" + url.PathEscape(incidentID) + "/streams/" + url.PathEscape(streamID) + "/fail"
	if err := c.postJSON(ctx, path, request, http.StatusOK, &response); err != nil {
		return fmt.Errorf("fail media stream: %w", err)
	}
	if response.Stream.Status != "failed" {
		return fmt.Errorf("fail media stream: expected failed status, got %q", response.Stream.Status)
	}
	return nil
}

func (c client) closeIncident(ctx context.Context, incidentID string) error {
	path := "/v1/incidents/" + url.PathEscape(incidentID) + "/close"
	if err := c.postJSON(ctx, path, map[string]any{}, http.StatusOK, nil); err != nil {
		return fmt.Errorf("close incident: %w", err)
	}
	return nil
}

func (c client) downloadStreamBundle(ctx context.Context, token, streamID string) ([]byte, error) {
	path := "/i/" + url.PathEscape(token) + "/streams/" + url.PathEscape(streamID) + "/download"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, joinURL(c.viewerBase, path), nil)
	if err != nil {
		return nil, fmt.Errorf("build bundle download request: %w", err)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download bundle request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 64*1024))
		if readErr != nil {
			return nil, readErr
		}
		return nil, fmt.Errorf("download bundle: %w", statusError(http.StatusOK, response.StatusCode, responseBody))
	}
	if response.Header.Get("Content-Type") != "application/zip" {
		return nil, fmt.Errorf("download bundle: expected application/zip, got %q", response.Header.Get("Content-Type"))
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("download bundle body: %w", err)
	}
	return body, nil
}

func (c client) uploadChunk(ctx context.Context, upload chunkUpload) error {
	status, headers, body, err := c.postChunk(ctx, upload)
	if err != nil {
		return err
	}
	if status == http.StatusCreated {
		return nil
	}
	if status == http.StatusOK && headers.Get("Idempotency-Replayed") == "true" {
		return nil
	}
	return fmt.Errorf("upload chunk: %w", uploadStatusError(status, body))
}

func (c client) expectIdempotentReplay(ctx context.Context, upload chunkUpload) error {
	status, headers, body, err := c.postChunk(ctx, upload)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("expected idempotent replay: %w", statusError(http.StatusOK, status, body))
	}
	if headers.Get("Idempotency-Replayed") != "true" {
		return fmt.Errorf("expected idempotent replay header")
	}
	return nil
}

func (c client) expectHashMismatch(ctx context.Context, upload chunkUpload) error {
	status, _, body, err := c.postChunk(ctx, upload)
	if err != nil {
		return err
	}
	if status != http.StatusBadRequest {
		return fmt.Errorf("expected hash mismatch: %w", statusError(http.StatusBadRequest, status, body))
	}
	code := errorCode(body)
	if code != "hash_mismatch" {
		return fmt.Errorf("expected hash_mismatch error code, got %q: %s", code, responseErrorSummary(body))
	}
	return nil
}

func (c client) postJSON(ctx context.Context, path string, payload any, wantStatus int, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL(c.apiBase, path), bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	c.authorize(request)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	if err != nil {
		return err
	}
	if response.StatusCode != wantStatus {
		return statusError(wantStatus, response.StatusCode, responseBody)
	}
	if target == nil {
		return nil
	}
	if err := json.Unmarshal(responseBody, target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c client) postChunk(ctx context.Context, upload chunkUpload) (int, http.Header, []byte, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	filePart, err := writer.CreateFormFile("file", upload.filename)
	if err != nil {
		return 0, nil, nil, err
	}
	if _, err := filePart.Write(upload.body); err != nil {
		return 0, nil, nil, err
	}
	fields := map[string]string{
		"chunk_index":       strconv.Itoa(upload.chunkIndex),
		"media_type":        upload.mediaType,
		"started_at":        upload.startedAt.Format(time.RFC3339Nano),
		"ended_at":          upload.endedAt.Format(time.RFC3339Nano),
		"sha256_hex":        upload.sha256Hex,
		"original_filename": upload.filename,
	}
	if upload.streamID != "" {
		fields["stream_id"] = upload.streamID
	}
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			return 0, nil, nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return 0, nil, nil, err
	}

	path := "/v1/incidents/" + url.PathEscape(upload.incidentID) + "/chunks"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL(c.apiBase, path), &body)
	if err != nil {
		return 0, nil, nil, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if upload.idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", upload.idempotencyKey)
	}
	c.authorize(request)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return 0, nil, nil, err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	if err != nil {
		return response.StatusCode, response.Header.Clone(), nil, err
	}
	return response.StatusCode, response.Header.Clone(), responseBody, nil
}

func (c client) authorize(request *http.Request) {
	if c.sessionToken != "" {
		request.Header.Set("Authorization", "Bearer "+c.sessionToken)
	}
}

func errorCode(body []byte) string {
	var apiError apiErrorResponse
	if err := json.Unmarshal(body, &apiError); err != nil {
		return ""
	}
	return apiError.Error.Code
}

func statusError(wantStatus, gotStatus int, body []byte) error {
	summary := responseErrorSummary(body)
	if summary == "" {
		return fmt.Errorf("expected status %d, got %d", wantStatus, gotStatus)
	}
	return fmt.Errorf("expected status %d, got %d: %s", wantStatus, gotStatus, summary)
}

func uploadStatusError(gotStatus int, body []byte) error {
	if gotStatus == http.StatusConflict {
		if code := errorCode(body); code != "" {
			return fmt.Errorf("expected status %d, got %d: %s", http.StatusCreated, gotStatus, code)
		}
	}
	return statusError(http.StatusCreated, gotStatus, body)
}

func responseErrorSummary(body []byte) string {
	var apiError apiErrorResponse
	if err := json.Unmarshal(body, &apiError); err == nil {
		code := strings.TrimSpace(apiError.Error.Code)
		message := strings.TrimSpace(apiError.Error.Message)
		switch {
		case code != "" && message != "":
			return code + ": " + message
		case code != "":
			return code
		case message != "":
			return message
		}
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return "empty response body"
	}
	return "response body omitted"
}
