package httpapi_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/db"
	"github.com/open-proofline/server/internal/httpapi"
	"github.com/open-proofline/server/internal/incidents"
	"github.com/open-proofline/server/internal/retention"
	"github.com/open-proofline/server/internal/storage"
	"golang.org/x/crypto/bcrypt"
)

func TestS3DeletionSmokeRemovesObjectsAndHidesViewer(t *testing.T) {
	opts := s3DeletionSmokeOptions(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	store, err := storage.NewS3(opts)
	if err != nil {
		t.Fatalf("create s3 deletion smoke store: %v", err)
	}
	if err := store.Check(ctx); err != nil {
		t.Fatal("s3 deletion smoke store was not ready; verify SAFE_S3_* points at a disposable test bucket")
	}

	app, repo := newS3DeletionSmokeApp(t, store)
	incidentID := createIncident(t, app, `{"client_label":"s3 deletion smoke"}`)
	stream := createMediaStream(t, app, incidentID, incidents.MediaTypeAudio, "audio smoke")
	viewerToken := createIncidentToken(t, app, incidentID, "smoke viewer", nil)

	for index := 1; index <= 2; index++ {
		payload := s3DeletionSmokePayload(index)
		response, _ := uploadChunkWithStream(t, app, incidentID, stream.ID, index, incidents.MediaTypeAudio, payload, sha256Hex(payload))
		response.Body.Close()
		if response.StatusCode != http.StatusCreated {
			t.Fatalf("expected s3 smoke upload status 201, got %d", response.StatusCode)
		}
	}

	response, _ := getPublic(t, app, "/i/"+viewerToken.Token+"/data")
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected viewer token to work before deletion, got %d", response.StatusCode)
	}

	detail, err := repo.GetIncidentDetail(ctx, incidentID)
	if err != nil {
		t.Fatalf("get s3 smoke incident detail: %v", err)
	}
	if len(detail.Chunks) != 2 {
		t.Fatalf("s3 smoke chunk count = %d, want 2", len(detail.Chunks))
	}
	storedPaths := make([]string, 0, len(detail.Chunks))
	for _, chunk := range detail.Chunks {
		storedPaths = append(storedPaths, chunk.StoredPath)
		assertS3SmokeObjectExists(t, ctx, store, chunk.StoredPath)
	}
	t.Cleanup(func() {
		for _, storedPath := range storedPaths {
			_ = store.Remove(context.Background(), storedPath)
		}
	})

	response, body := post(t, app, "/v1/incidents/"+incidentID+"/deletion", "application/json", bytes.NewBufferString(`{"reason_code":"s3_deletion_smoke","allow_open":true}`))
	response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("expected s3 smoke deletion status 202, got %d", response.StatusCode)
	}
	deletion := decodeDeletionResponse(t, body)
	if deletion.State != incidents.IncidentDeletionStatePending || deletion.ItemCount != 2 {
		t.Fatalf("unexpected s3 smoke deletion status: state=%q item_count=%d", deletion.State, deletion.ItemCount)
	}
	assertDeletionItemsMatchStoredPaths(t, repo, deletion.DecisionID, storedPaths)

	if err := store.Remove(ctx, storedPaths[1]); err != nil {
		t.Fatal("pre-delete of metadata-snapshotted s3 smoke object failed")
	}

	worker := retention.NewWorker(repo, store, retention.Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	summary, err := worker.RunOnce(ctx)
	if err != nil {
		t.Fatal("s3 deletion smoke worker failed; verify the disposable object store is reachable")
	}
	if summary.Processed != 1 || summary.Completed != 1 || summary.Failed != 0 {
		t.Fatalf("unexpected s3 deletion smoke worker summary: %+v", summary)
	}

	completed, err := repo.GetIncidentDeletionStatus(ctx, incidentID)
	if err != nil {
		t.Fatalf("get completed s3 smoke deletion status: %v", err)
	}
	if completed.State != incidents.IncidentDeletionStateDeleted || completed.ItemCount != 2 {
		t.Fatalf("unexpected completed s3 smoke deletion status: state=%q item_count=%d", completed.State, completed.ItemCount)
	}
	for _, storedPath := range storedPaths {
		assertS3SmokeObjectMissing(t, ctx, store, storedPath)
	}

	retrySummary, err := worker.RunOnce(ctx)
	if err != nil {
		t.Fatal("s3 deletion smoke worker retry failed")
	}
	if retrySummary.Processed != 0 || retrySummary.Completed != 0 || retrySummary.Failed != 0 {
		t.Fatalf("unexpected s3 deletion smoke retry summary: %+v", retrySummary)
	}

	for _, target := range []string{
		"/i/" + viewerToken.Token,
		"/i/" + viewerToken.Token + "/data",
		"/i/" + viewerToken.Token + "/incident/download",
		"/e/" + viewerToken.Token + "/data",
	} {
		assertS3SmokePublicRouteFailsClosed(t, app, target)
	}

	items, err := repo.ListIncidentDeletionItems(ctx, deletion.DecisionID)
	if err != nil {
		t.Fatalf("list completed s3 smoke deletion items: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("completed s3 smoke deletion kept %d deletion items", len(items))
	}
}

func s3DeletionSmokeOptions(t *testing.T) storage.S3Options {
	t.Helper()
	if !s3DeletionSmokeEnabled() {
		t.Skip("set SAFE_S3_DELETION_SMOKE=1 with SAFE_S3_* for optional S3-compatible deletion smoke coverage")
	}

	required := []string{
		"SAFE_S3_ENDPOINT",
		"SAFE_S3_BUCKET",
		"SAFE_S3_ACCESS_KEY_ID",
		"SAFE_S3_SECRET_ACCESS_KEY",
	}
	missing := make([]string, 0)
	values := make(map[string]string, len(required))
	for _, name := range required {
		value := strings.TrimSpace(os.Getenv(name))
		if value == "" {
			missing = append(missing, name)
			continue
		}
		values[name] = value
	}
	if len(missing) > 0 {
		t.Fatalf("SAFE_S3_DELETION_SMOKE requires %s", strings.Join(missing, ", "))
	}

	region := strings.TrimSpace(os.Getenv("SAFE_S3_REGION"))
	if region == "" {
		region = "us-east-1"
	}
	forcePathStyle := true
	if raw := strings.TrimSpace(os.Getenv("SAFE_S3_FORCE_PATH_STYLE")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			t.Fatal("SAFE_S3_FORCE_PATH_STYLE must be a boolean")
		}
		forcePathStyle = parsed
	}

	prefix := path.Join(strings.Trim(strings.TrimSpace(os.Getenv("SAFE_S3_PREFIX")), "/"), "deletion-smoke", strconv.FormatInt(time.Now().UnixNano(), 10))
	return storage.S3Options{
		Endpoint:        values["SAFE_S3_ENDPOINT"],
		Region:          region,
		Bucket:          values["SAFE_S3_BUCKET"],
		Prefix:          prefix,
		AccessKeyID:     values["SAFE_S3_ACCESS_KEY_ID"],
		SecretAccessKey: values["SAFE_S3_SECRET_ACCESS_KEY"],
		SessionToken:    strings.TrimSpace(os.Getenv("SAFE_S3_SESSION_TOKEN")),
		ForcePathStyle:  forcePathStyle,
		TempDir:         filepath.Join(t.TempDir(), "tmp"),
	}
}

func s3DeletionSmokeEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SAFE_S3_DELETION_SMOKE"))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func newS3DeletionSmokeApp(t *testing.T, store storage.BlobStore) (*testApp, *incidents.Repository) {
	t.Helper()

	ctx := context.Background()
	dataDir := t.TempDir()
	conn, err := db.Open(ctx, filepath.Join(dataDir, "safety.db"))
	if err != nil {
		t.Fatalf("open s3 deletion smoke db: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	repo := incidents.NewRepository(conn)
	passwordHash, err := auth.HashPassword("test-password", bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash s3 deletion smoke account password: %v", err)
	}
	account, err := repo.CreateAccount(ctx, auth.CreateAccountParams{
		Username:     "s3-smoke-admin",
		PasswordHash: passwordHash,
		Role:         auth.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("create s3 deletion smoke account: %v", err)
	}
	_, authToken, err := repo.CreateSession(ctx, account.ID, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("create s3 deletion smoke session: %v", err)
	}

	options := httpapi.Options{
		MaxUploadBytes: 1024 * 1024,
		PasswordCost:   bcrypt.MinCost,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	mainHandler := httpapi.NewMain(repo, store, options)
	adminHandler := httpapi.NewAdmin(repo, store, options)
	app := &testApp{
		mainHandler:    mainHandler,
		adminHandler:   adminHandler,
		privateHandler: mainHandler,
		publicHandler:  mainHandler,
		dataDir:        dataDir,
		db:             conn,
		authToken:      authToken,
	}
	return app, repo
}

func s3DeletionSmokePayload(index int) []byte {
	payload := make([]byte, 128)
	for offset := range payload {
		payload[offset] = byte((index + offset) % 251)
	}
	return payload
}

func assertS3SmokeObjectExists(t *testing.T, ctx context.Context, store storage.BlobStore, storedPath string) {
	t.Helper()
	reader, err := store.Open(ctx, storedPath)
	if err != nil {
		t.Fatal("expected metadata-backed s3 smoke object to exist")
	}
	if err := reader.Close(); err != nil {
		t.Fatal("close metadata-backed s3 smoke object")
	}
}

func assertS3SmokeObjectMissing(t *testing.T, ctx context.Context, store storage.BlobStore, storedPath string) {
	t.Helper()
	reader, err := store.Open(ctx, storedPath)
	if err == nil {
		_ = reader.Close()
		t.Fatal("expected metadata-backed s3 smoke object to be deleted")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatal("expected deleted metadata-backed s3 smoke object to be absent")
	}
}

func assertDeletionItemsMatchStoredPaths(t *testing.T, repo *incidents.Repository, decisionID string, storedPaths []string) {
	t.Helper()
	items, err := repo.ListIncidentDeletionItems(context.Background(), decisionID)
	if err != nil {
		t.Fatalf("list s3 smoke deletion items: %v", err)
	}
	if len(items) != len(storedPaths) {
		t.Fatalf("s3 smoke deletion item count = %d, want %d", len(items), len(storedPaths))
	}
	want := make(map[string]struct{}, len(storedPaths))
	for _, storedPath := range storedPaths {
		want[storedPath] = struct{}{}
	}
	for _, item := range items {
		if item.State != incidents.IncidentDeletionItemStatePending {
			t.Fatalf("s3 smoke deletion item state = %q, want pending", item.State)
		}
		if _, ok := want[item.StoredPath]; !ok {
			t.Fatal("s3 smoke deletion item did not come from uploaded chunk metadata")
		}
	}
}

func assertS3SmokePublicRouteFailsClosed(t *testing.T, app *testApp, target string) {
	t.Helper()
	response, body := getPublic(t, app, target)
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected public route to fail closed after s3 deletion, got %d", response.StatusCode)
	}
	assertErrorCode(t, body, "incident_token_invalid")
	if bytes.Contains(body, []byte("deletion")) || bytes.Contains(body, []byte("deleted")) {
		t.Fatal("public fail-closed response exposed deletion state")
	}
}
