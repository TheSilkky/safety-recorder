package httpapi_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/db"
	"github.com/open-proofline/server/internal/httpapi"
	"github.com/open-proofline/server/internal/incidents"
	"github.com/open-proofline/server/internal/storage"
	"golang.org/x/crypto/bcrypt"
)

type testApp struct {
	mainHandler    http.Handler
	adminHandler   http.Handler
	privateHandler http.Handler
	publicHandler  http.Handler
	dataDir        string
	db             *sql.DB
	authToken      string
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()

	return newTestAppWithMaxUploadBytes(t, 1024*1024)
}

func newTestAppWithMaxUploadBytes(t *testing.T, maxUploadBytes int64) *testApp {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return newTestAppWithMaxUploadBytesAndLogger(t, maxUploadBytes, logger)
}

func newTestAppWithMaxUploadBytesAndLogger(t *testing.T, maxUploadBytes int64, logger *slog.Logger) *testApp {
	t.Helper()

	return newTestAppWithOptions(t, httpapi.Options{
		MaxUploadBytes: maxUploadBytes,
		Logger:         logger,
	})
}

func newTestAppWithDefaultIncidentTokenTTL(t *testing.T, ttl time.Duration) *testApp {
	t.Helper()

	return newTestAppWithOptions(t, httpapi.Options{
		MaxUploadBytes:          1024 * 1024,
		DefaultIncidentTokenTTL: &ttl,
		Logger:                  slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

func newTestAppWithOptions(t *testing.T, options httpapi.Options) *testApp {
	t.Helper()
	if options.PasswordCost == 0 {
		options.PasswordCost = bcrypt.MinCost
	}

	return newTestAppWithOptionsAndTestAccount(t, options, true)
}

func newTestAppWithoutTestAccount(t *testing.T, options httpapi.Options) *testApp {
	t.Helper()
	if options.PasswordCost == 0 {
		options.PasswordCost = bcrypt.MinCost
	}

	return newTestAppWithOptionsAndTestAccount(t, options, false)
}

func newTestAppWithOptionsAndTestAccount(t *testing.T, options httpapi.Options, createTestAccount bool) *testApp {
	t.Helper()

	dataDir := t.TempDir()
	conn, err := db.Open(context.Background(), filepath.Join(dataDir, "safety.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	blobStore, err := storage.New(dataDir)
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}
	repo := incidents.NewRepository(conn)
	authToken := ""
	if createTestAccount {
		passwordHash, err := auth.HashPassword("test-password", bcrypt.MinCost)
		if err != nil {
			t.Fatalf("hash test account password: %v", err)
		}
		account, err := repo.CreateAccount(context.Background(), auth.CreateAccountParams{
			Username:     "test-admin",
			PasswordHash: passwordHash,
			Role:         auth.RoleAdmin,
		})
		if err != nil {
			t.Fatalf("create test account: %v", err)
		}
		_, authToken, err = repo.CreateSession(context.Background(), account.ID, time.Now().UTC().Add(24*time.Hour))
		if err != nil {
			t.Fatalf("create test auth session: %v", err)
		}
	}
	var metadataRepo httpapi.MetadataRepository = repo
	mainHandler := httpapi.NewMain(metadataRepo, blobStore, options)
	adminHandler := httpapi.NewAdmin(metadataRepo, blobStore, options)
	publicHandler := httpapi.NewPublic(metadataRepo, blobStore, options)
	return &testApp{
		mainHandler:    mainHandler,
		adminHandler:   adminHandler,
		privateHandler: mainHandler,
		publicHandler:  publicHandler,
		dataDir:        dataDir,
		db:             conn,
		authToken:      authToken,
	}
}

func createIncident(t *testing.T, app *testApp, requestBody string) string {
	t.Helper()

	response, body := post(t, app, "/v1/incidents", "application/json", bytes.NewBufferString(requestBody))
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create incident status 201, got %d: %s", response.StatusCode, body)
	}

	var result struct {
		IncidentID string `json:"incident_id"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode create incident response: %v", err)
	}
	if result.Status != incidents.StatusOpen {
		t.Fatalf("expected status open, got %q", result.Status)
	}
	return result.IncidentID
}

type incidentTokenResponse struct {
	TokenID    string     `json:"token_id"`
	IncidentID string     `json:"incident_id"`
	Token      string     `json:"token"`
	Label      string     `json:"label"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
}

func createIncidentToken(t *testing.T, app *testApp, incidentID string, label string, expiresAt *time.Time) incidentTokenResponse {
	t.Helper()

	requestBody, err := json.Marshal(struct {
		Label     string     `json:"label"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
	}{
		Label:     label,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		t.Fatalf("marshal incident token request: %v", err)
	}

	response, body := post(t, app, "/v1/incidents/"+incidentID+"/incident-tokens", "application/json", bytes.NewReader(requestBody))
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create incident token status 201, got %d: %s", response.StatusCode, body)
	}
	assertMainJSONSecurityHeaders(t, response)

	var result incidentTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode create incident token response: %v", err)
	}
	return result
}

func createCheckin(t *testing.T, app *testApp, incidentID string) {
	t.Helper()

	checkinBody := bytes.NewBufferString(`{"device_battery_percent":82,"device_network":"wifi","latitude":-37,"longitude":145,"accuracy_meters":20}`)
	response, body := post(t, app, "/v1/incidents/"+incidentID+"/checkins", "application/json", checkinBody)
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected checkin status 201, got %d: %s", response.StatusCode, body)
	}
}

func getIncidentDetail(t *testing.T, app *testApp, incidentID string) incidents.IncidentDetail {
	t.Helper()

	response, body := get(t, app, "/v1/incidents/"+incidentID)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected incident status 200, got %d: %s", response.StatusCode, body)
	}
	var detail incidents.IncidentDetail
	if err := json.Unmarshal(body, &detail); err != nil {
		t.Fatalf("decode incident detail: %v", err)
	}
	return detail
}

func createMediaStream(t *testing.T, app *testApp, incidentID, mediaType, label string) incidents.MediaStream {
	t.Helper()

	requestBody, err := json.Marshal(struct {
		MediaType string `json:"media_type"`
		Label     string `json:"label"`
	}{
		MediaType: mediaType,
		Label:     label,
	})
	if err != nil {
		t.Fatalf("marshal media stream request: %v", err)
	}
	response, body := post(t, app, "/v1/incidents/"+incidentID+"/streams", "application/json", bytes.NewReader(requestBody))
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected create media stream status 201, got %d: %s", response.StatusCode, body)
	}

	var result struct {
		Stream incidents.MediaStream `json:"stream"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode create stream response: %v", err)
	}
	return result.Stream
}

func completeMediaStream(t *testing.T, app *testApp, incidentID, streamID string, expectedChunkCount int) incidents.MediaStream {
	t.Helper()

	requestBody, err := json.Marshal(struct {
		ExpectedChunkCount int `json:"expected_chunk_count"`
	}{ExpectedChunkCount: expectedChunkCount})
	if err != nil {
		t.Fatalf("marshal complete stream request: %v", err)
	}
	response, body := post(t, app, "/v1/incidents/"+incidentID+"/streams/"+streamID+"/complete", "application/json", bytes.NewReader(requestBody))
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected complete media stream status 200, got %d: %s", response.StatusCode, body)
	}

	var result struct {
		Stream incidents.MediaStream `json:"stream"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode complete stream response: %v", err)
	}
	return result.Stream
}

func createIncidentStreamWithChunks(t *testing.T, app *testApp, chunkCount int) (string, incidents.MediaStream) {
	t.Helper()

	incidentID := createIncident(t, app, `{}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "audio recording")
	for index := 1; index <= chunkCount; index++ {
		payload := []byte("encrypted audio data " + strconv.Itoa(index))
		response, body := uploadChunkWithStream(t, app, incidentID, stream.ID, index, incidents.MediaTypeAudio, payload, sha256Hex(payload))
		response.Body.Close()
		if response.StatusCode != http.StatusCreated {
			t.Fatalf("expected stream chunk upload status 201, got %d: %s", response.StatusCode, body)
		}
	}
	return incidentID, stream
}

func removeStoredStreamChunkFile(t *testing.T, app *testApp, incidentID, streamID, mediaType string, chunkIndex int) {
	t.Helper()

	chunkPath := filepath.Join(app.dataDir, "incidents", incidentID, "streams", streamID, fmt.Sprintf("%s_%06d.enc", mediaType, chunkIndex))
	if err := os.Remove(chunkPath); err != nil {
		t.Fatalf("remove stored stream chunk file: %v", err)
	}
}

func updateStoredStreamChunkIndex(t *testing.T, app *testApp, incidentID, streamID string, currentIndex, nextIndex int) {
	t.Helper()

	result, err := app.db.ExecContext(context.Background(), `
		UPDATE chunks
		SET chunk_index = ?
		WHERE incident_id = ? AND stream_id = ? AND chunk_index = ?`,
		nextIndex,
		incidentID,
		streamID,
		currentIndex,
	)
	if err != nil {
		t.Fatalf("update stored stream chunk index: %v", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("stored stream chunk rows affected: %v", err)
	}
	if rowsAffected != 1 {
		t.Fatalf("expected to update 1 stored stream chunk, updated %d", rowsAffected)
	}
}

func uploadChunk(t *testing.T, app *testApp, incidentID string, index int, mediaType string, payload []byte, hash string) (*http.Response, []byte) {
	t.Helper()

	return uploadChunkWithStream(t, app, incidentID, "", index, mediaType, payload, hash)
}

func uploadChunkWithStream(t *testing.T, app *testApp, incidentID string, streamID string, index int, mediaType string, payload []byte, hash string) (*http.Response, []byte) {
	t.Helper()

	return uploadChunkWithOptions(t, app, incidentID, streamID, index, mediaType, payload, hash, "chunk.enc", "")
}

func uploadChunkWithIdempotencyKey(t *testing.T, app *testApp, incidentID string, streamID string, index int, mediaType string, payload []byte, hash string, originalFilename string, idempotencyKey string) (*http.Response, []byte) {
	t.Helper()

	return uploadChunkWithOptions(t, app, incidentID, streamID, index, mediaType, payload, hash, originalFilename, idempotencyKey)
}

func uploadChunkWithOptions(t *testing.T, app *testApp, incidentID string, streamID string, index int, mediaType string, payload []byte, hash string, originalFilename string, idempotencyKey string) (*http.Response, []byte) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if streamID != "" {
		must(t, writer.WriteField("stream_id", streamID))
	}
	must(t, writer.WriteField("chunk_index", strconv.Itoa(index)))
	must(t, writer.WriteField("media_type", mediaType))
	startedAt := testChunkStartedAt()
	must(t, writer.WriteField("started_at", startedAt.Format(time.RFC3339Nano)))
	must(t, writer.WriteField("ended_at", startedAt.Add(time.Second).Format(time.RFC3339Nano)))
	must(t, writer.WriteField("sha256_hex", hash))
	must(t, writer.WriteField("original_filename", originalFilename))
	fileWriter, err := writer.CreateFormFile("file", "upload.enc")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fileWriter.Write(payload); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	must(t, writer.Close())

	headers := map[string]string{}
	if idempotencyKey != "" {
		headers["Idempotency-Key"] = idempotencyKey
	}
	return postWithHeaders(t, app, "/v1/incidents/"+incidentID+"/chunks", writer.FormDataContentType(), &body, headers)
}

func testChunkStartedAt() time.Time {
	return time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
}

func testChunkEndedAt() time.Time {
	return testChunkStartedAt().Add(time.Second)
}

func testChunkStartedAtString() string {
	return testChunkStartedAt().Format(time.RFC3339Nano)
}

func testChunkEndedAtString() string {
	return testChunkEndedAt().Format(time.RFC3339Nano)
}

func post(t *testing.T, app *testApp, target string, contentType string, body io.Reader) (*http.Response, []byte) {
	t.Helper()

	return requestWithAuth(t, app.privateHandler, http.MethodPost, target, contentType, body, app.authToken)
}

func postWithHeaders(t *testing.T, app *testApp, target string, contentType string, body io.Reader, headers map[string]string) (*http.Response, []byte) {
	t.Helper()

	return requestWithAuthAndHeaders(t, app.privateHandler, http.MethodPost, target, contentType, body, app.authToken, headers)
}

func postUnauthenticated(t *testing.T, app *testApp, target string, contentType string, body io.Reader) (*http.Response, []byte) {
	t.Helper()

	return request(t, app.privateHandler, http.MethodPost, target, contentType, body)
}

func postPublic(t *testing.T, app *testApp, target string, contentType string, body io.Reader) (*http.Response, []byte) {
	t.Helper()

	return request(t, app.publicHandler, http.MethodPost, target, contentType, body)
}

func get(t *testing.T, app *testApp, target string) (*http.Response, []byte) {
	t.Helper()

	return requestWithAuth(t, app.privateHandler, http.MethodGet, target, "", nil, app.authToken)
}

func getUnauthenticated(t *testing.T, app *testApp, target string) (*http.Response, []byte) {
	t.Helper()

	return request(t, app.privateHandler, http.MethodGet, target, "", nil)
}

func getPublic(t *testing.T, app *testApp, target string) (*http.Response, []byte) {
	t.Helper()

	return request(t, app.publicHandler, http.MethodGet, target, "", nil)
}

func request(t *testing.T, handler http.Handler, method string, target string, contentType string, body io.Reader) (*http.Response, []byte) {
	t.Helper()

	return requestWithAuth(t, handler, method, target, contentType, body, "")
}

func requestWithAuth(t *testing.T, handler http.Handler, method string, target string, contentType string, body io.Reader, token string) (*http.Response, []byte) {
	t.Helper()

	return requestWithAuthAndHeaders(t, handler, method, target, contentType, body, token, nil)
}

func requestWithAuthAndHeaders(t *testing.T, handler http.Handler, method string, target string, contentType string, body io.Reader, token string, headers map[string]string) (*http.Response, []byte) {
	t.Helper()

	request := httptest.NewRequest(method, target, body)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	response := recorder.Result()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return response, responseBody
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func stringsOf(value string, count int) string {
	var builder bytes.Buffer
	for range count {
		builder.WriteString(value)
	}
	return builder.String()
}

func assertIncidentViewerPrivacyHeaders(t *testing.T, response *http.Response) {
	t.Helper()

	assertPublicBrowserSecurityHeaders(t, response)
	assertNoStore(t, response)
}

func assertPublicBrowserSecurityHeaders(t *testing.T, response *http.Response) {
	t.Helper()

	assertNoSniff(t, response)
	if response.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("expected no-referrer policy, got %q", response.Header.Get("Referrer-Policy"))
	}
	csp := response.Header.Get("Content-Security-Policy")
	for _, directive := range []string{"default-src 'self'", "base-uri 'none'", "frame-ancestors 'none'", "form-action 'self'", "object-src 'none'"} {
		if !strings.Contains(csp, directive) {
			t.Fatalf("expected CSP directive %q in %q", directive, csp)
		}
	}
	if response.Header.Get("Permissions-Policy") != "geolocation=(), microphone=(), camera=()" {
		t.Fatalf("expected restricted permissions policy, got %q", response.Header.Get("Permissions-Policy"))
	}
	if response.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("expected X-Frame-Options DENY, got %q", response.Header.Get("X-Frame-Options"))
	}
	assertNoStrictTransportSecurity(t, response)
}

func assertMainJSONSecurityHeaders(t *testing.T, response *http.Response) {
	t.Helper()

	if response.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("expected application/json, got %q", response.Header.Get("Content-Type"))
	}
	assertNoSniff(t, response)
	assertNoStore(t, response)
	assertNoStrictTransportSecurity(t, response)
}

func assertContentTypePrefix(t *testing.T, response *http.Response, prefix string) {
	t.Helper()

	contentType := response.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, prefix) {
		t.Fatalf("expected Content-Type prefix %q, got %q", prefix, contentType)
	}
}

func assertContentTypeContains(t *testing.T, response *http.Response, value string) {
	t.Helper()

	contentType := response.Header.Get("Content-Type")
	if !strings.Contains(contentType, value) {
		t.Fatalf("expected Content-Type containing %q, got %q", value, contentType)
	}
}

func assertNoSniff(t *testing.T, response *http.Response) {
	t.Helper()

	if response.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("expected nosniff, got %q", response.Header.Get("X-Content-Type-Options"))
	}
}

func assertNoStore(t *testing.T, response *http.Response) {
	t.Helper()

	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("expected no-store cache policy, got %q", response.Header.Get("Cache-Control"))
	}
}

func assertNoStrictTransportSecurity(t *testing.T, response *http.Response) {
	t.Helper()

	if value := response.Header.Get("Strict-Transport-Security"); value != "" {
		t.Fatalf("expected no app-level Strict-Transport-Security header in local/dev HTTP mode, got %q", value)
	}
}

func assertBundleHeaders(t *testing.T, response *http.Response) {
	t.Helper()

	if response.Header.Get("Content-Type") != "application/zip" {
		t.Fatalf("expected application/zip, got %q", response.Header.Get("Content-Type"))
	}
	if !strings.HasPrefix(response.Header.Get("Content-Disposition"), `attachment; filename="incident_`) {
		t.Fatalf("expected attachment content disposition, got %q", response.Header.Get("Content-Disposition"))
	}
	assertIncidentViewerPrivacyHeaders(t, response)
}

func assertErrorCode(t *testing.T, body []byte, expected string) {
	t.Helper()

	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response.Error.Code != expected {
		t.Fatalf("expected error code %q, got %q", expected, response.Error.Code)
	}
}

func readZipEntries(t *testing.T, body []byte) map[string][]byte {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("open zip response: %v", err)
	}
	entries := make(map[string][]byte)
	for _, file := range reader.File {
		handle, err := file.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", file.Name, err)
		}
		entryBody, err := io.ReadAll(handle)
		if closeErr := handle.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			t.Fatalf("read zip entry %s: %v", file.Name, err)
		}
		entries[file.Name] = entryBody
	}
	return entries
}

func assertZipEntry(t *testing.T, entries map[string][]byte, name string) {
	t.Helper()

	if _, ok := entries[name]; !ok {
		t.Fatalf("expected zip entry %q, got entries %+v", name, entries)
	}
}

type incidentTokenDBRow struct {
	ID         string
	IncidentID string
	TokenHash  string
	Label      string
	CreatedAt  string
	ExpiresAt  string
	RevokedAt  string
}

func incidentTokenRows(t *testing.T, app *testApp) []incidentTokenDBRow {
	t.Helper()

	rows, err := app.db.QueryContext(context.Background(), `
		SELECT id, incident_id, token_hash, COALESCE(label, ''),
			created_at, COALESCE(expires_at, ''), COALESCE(revoked_at, '')
		FROM incident_tokens
		ORDER BY id`)
	if err != nil {
		t.Fatalf("query incident token rows: %v", err)
	}
	defer rows.Close()

	tokenRows := []incidentTokenDBRow{}
	for rows.Next() {
		var row incidentTokenDBRow
		if err := rows.Scan(
			&row.ID,
			&row.IncidentID,
			&row.TokenHash,
			&row.Label,
			&row.CreatedAt,
			&row.ExpiresAt,
			&row.RevokedAt,
		); err != nil {
			t.Fatalf("scan incident token row: %v", err)
		}
		tokenRows = append(tokenRows, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate incident token rows: %v", err)
	}
	return tokenRows
}

func assertIncidentTokenColumnMissing(t *testing.T, app *testApp, column string) {
	t.Helper()

	rows, err := app.db.QueryContext(context.Background(), `PRAGMA table_info(incident_tokens)`)
	if err != nil {
		t.Fatalf("query incident token schema: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan incident token schema: %v", err)
		}
		if name == column {
			t.Fatalf("incident_tokens still has removed column %q", column)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate incident token schema: %v", err)
	}
}

func assertNoStoredFile(t *testing.T, app *testApp, incidentID, filename string) {
	t.Helper()

	storedPath := filepath.Join(app.dataDir, "incidents", incidentID, filename)
	if _, err := os.Stat(storedPath); !os.IsNotExist(err) {
		t.Fatalf("expected no stored file at %s, got err %v", storedPath, err)
	}
}

func assertTempDirEmpty(t *testing.T, app *testApp) {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(app.dataDir, "tmp"))
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected temp dir to be empty, found %d entries", len(entries))
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
