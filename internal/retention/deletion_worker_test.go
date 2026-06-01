package retention_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/db"
	"github.com/open-proofline/server/internal/incidents"
	"github.com/open-proofline/server/internal/retention"
	"github.com/open-proofline/server/internal/storage"
)

func TestWorkerRunOnceCompletesDeletionAndPrunesMetadata(t *testing.T) {
	ctx := context.Background()
	repo, conn := newDeletionTestRepository(t, ctx)
	incident, err := repo.CreateIncidentForAccount(ctx, "", incidents.CreateIncidentParams{
		ClientLabel:      "phone",
		Notes:            "sensitive note",
		IncidentMode:     incidents.IncidentModeEmergency,
		CaptureProfile:   incidents.CaptureProfileAudioVideoLocation,
		EscalationPolicy: incidents.EscalationPolicyNone,
		SharingState:     incidents.SharingStatePrivate,
	})
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	stream, err := repo.CreateMediaStream(ctx, incident.ID, incidents.MediaTypeAudio, "audio")
	if err != nil {
		t.Fatalf("create media stream: %v", err)
	}
	firstChunk, err := repo.CreateChunk(ctx, deletionTestChunkParams(incident.ID, stream.ID, incidents.MediaTypeAudio, 1))
	if err != nil {
		t.Fatalf("create first chunk: %v", err)
	}
	secondChunk, err := repo.CreateChunk(ctx, deletionTestChunkParams(incident.ID, "", incidents.MediaTypeMetadata, 1))
	if err != nil {
		t.Fatalf("create second chunk: %v", err)
	}
	if _, _, err := repo.CreateIncidentToken(ctx, incident.ID, "trusted contact", nil); err != nil {
		t.Fatalf("create incident token: %v", err)
	}
	if _, err := repo.CreateCheckin(ctx, incident.ID, incidents.CreateCheckinParams{}); err != nil {
		t.Fatalf("create checkin: %v", err)
	}
	status, err := repo.RequestIncidentDeletion(ctx, incidents.IncidentDeletionRequest{
		IncidentID: incident.ID,
		Source:     incidents.IncidentDeletionSourceAdminRequest,
		AllowOpen:  true,
	})
	if err != nil {
		t.Fatalf("request deletion: %v", err)
	}

	store := &fakeBlobStore{}
	worker := retention.NewWorker(repo, store, retention.Options{})
	summary, err := worker.RunOnce(ctx)
	if err != nil {
		t.Fatalf("run worker: %v", err)
	}
	if summary.Processed != 1 || summary.Completed != 1 || summary.Failed != 0 {
		t.Fatalf("unexpected worker summary: %+v", summary)
	}
	assertRemovedPaths(t, store.removed, firstChunk.StoredPath, secondChunk.StoredPath)

	detail, err := repo.GetIncidentDetail(ctx, incident.ID)
	if err != nil {
		t.Fatalf("get incident detail: %v", err)
	}
	if detail.Incident.DeletionState != incidents.IncidentDeletionStateDeleted ||
		detail.Incident.ClientLabel != "" ||
		detail.Incident.Notes != "" ||
		detail.Incident.IncidentMode != "" ||
		len(detail.Chunks) != 0 ||
		len(detail.Streams) != 0 ||
		len(detail.Checkins) != 0 {
		t.Fatalf("deleted incident kept sensitive metadata: %+v", detail)
	}
	completed, err := repo.GetIncidentDeletionStatus(ctx, incident.ID)
	if err != nil {
		t.Fatalf("get deletion status: %v", err)
	}
	if completed.DecisionID != status.DecisionID ||
		completed.State != incidents.IncidentDeletionStateDeleted ||
		completed.CompletedAt == nil ||
		completed.ItemCount != 2 {
		t.Fatalf("unexpected completed deletion status: %+v", completed)
	}
	items, err := repo.ListIncidentDeletionItems(ctx, status.DecisionID)
	if err != nil {
		t.Fatalf("list deletion items: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected internal deletion items to be pruned, got %+v", items)
	}
	assertTableCount(t, ctx, conn, "incident_tokens", 0)
}

func TestWorkerTreatsMissingDeletionItemBlobAsSuccess(t *testing.T) {
	ctx := context.Background()
	repo, _ := newDeletionTestRepository(t, ctx)
	incident, err := repo.CreateIncident(ctx, "phone", "")
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	chunk, err := repo.CreateChunk(ctx, deletionTestChunkParams(incident.ID, "", incidents.MediaTypeAudio, 1))
	if err != nil {
		t.Fatalf("create chunk: %v", err)
	}
	status, err := repo.RequestIncidentDeletion(ctx, incidents.IncidentDeletionRequest{
		IncidentID: incident.ID,
		Source:     incidents.IncidentDeletionSourceAdminRequest,
		AllowOpen:  true,
	})
	if err != nil {
		t.Fatalf("request deletion: %v", err)
	}

	store := &fakeBlobStore{removeErrByPath: map[string]error{chunk.StoredPath: os.ErrNotExist}}
	summary, err := retention.NewWorker(repo, store, retention.Options{}).RunOnce(ctx)
	if err != nil {
		t.Fatalf("run worker: %v", err)
	}
	if summary.Completed != 1 || summary.Failed != 0 {
		t.Fatalf("unexpected worker summary: %+v", summary)
	}
	completed, err := repo.GetIncidentDeletionStatus(ctx, incident.ID)
	if err != nil {
		t.Fatalf("get deletion status: %v", err)
	}
	if completed.DecisionID != status.DecisionID || completed.State != incidents.IncidentDeletionStateDeleted {
		t.Fatalf("unexpected deletion status: %+v", completed)
	}
}

func TestWorkerResumesClaimedDeletionAfterRestart(t *testing.T) {
	ctx := context.Background()
	repo, _ := newDeletionTestRepository(t, ctx)
	incident, err := repo.CreateIncident(ctx, "phone", "")
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	chunk, err := repo.CreateChunk(ctx, deletionTestChunkParams(incident.ID, "", incidents.MediaTypeAudio, 1))
	if err != nil {
		t.Fatalf("create chunk: %v", err)
	}
	status, err := repo.RequestIncidentDeletion(ctx, incidents.IncidentDeletionRequest{
		IncidentID: incident.ID,
		Source:     incidents.IncidentDeletionSourceAdminRequest,
		AllowOpen:  true,
	})
	if err != nil {
		t.Fatalf("request deletion: %v", err)
	}
	if _, err := repo.MarkIncidentDeletionDeleting(ctx, status.DecisionID, time.Now().UTC()); err != nil {
		t.Fatalf("preclaim deletion: %v", err)
	}

	store := &fakeBlobStore{}
	summary, err := retention.NewWorker(repo, store, retention.Options{}).RunOnce(ctx)
	if err != nil {
		t.Fatalf("run worker: %v", err)
	}
	if summary.Processed != 0 || summary.Completed != 0 || summary.Failed != 0 {
		t.Fatalf("fresh claimed deletion should not be processed: %+v", summary)
	}
	if len(store.removed) != 0 {
		t.Fatalf("fresh claimed deletion removed blobs: %v", store.removed)
	}

	time.Sleep(time.Millisecond)
	summary, err = retention.NewWorker(repo, store, retention.Options{
		DeletingRetryAfter: time.Nanosecond,
	}).RunOnce(ctx)
	if err != nil {
		t.Fatalf("run stale worker: %v", err)
	}
	if summary.Processed != 1 || summary.Completed != 1 || summary.Failed != 0 {
		t.Fatalf("unexpected worker summary: %+v", summary)
	}
	assertRemovedPaths(t, store.removed, chunk.StoredPath)
	completed, err := repo.GetIncidentDeletionStatus(ctx, incident.ID)
	if err != nil {
		t.Fatalf("get deletion status: %v", err)
	}
	if completed.State != incidents.IncidentDeletionStateDeleted {
		t.Fatalf("deletion did not resume to completion: %+v", completed)
	}
}

func TestWorkerKeepsDeletionRetryableAfterBlobFailure(t *testing.T) {
	ctx := context.Background()
	repo, _ := newDeletionTestRepository(t, ctx)
	incident, err := repo.CreateIncident(ctx, "phone", "")
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	chunk, err := repo.CreateChunk(ctx, deletionTestChunkParams(incident.ID, "", incidents.MediaTypeAudio, 1))
	if err != nil {
		t.Fatalf("create chunk: %v", err)
	}
	status, err := repo.RequestIncidentDeletion(ctx, incidents.IncidentDeletionRequest{
		IncidentID: incident.ID,
		Source:     incidents.IncidentDeletionSourceAdminRequest,
		AllowOpen:  true,
	})
	if err != nil {
		t.Fatalf("request deletion: %v", err)
	}

	store := &fakeBlobStore{removeErrByPath: map[string]error{chunk.StoredPath: storage.ErrUnsafePath}}
	summary, err := retention.NewWorker(repo, store, retention.Options{}).RunOnce(ctx)
	if err != nil {
		t.Fatalf("run worker: %v", err)
	}
	if summary.Processed != 1 || summary.Completed != 0 || summary.Failed != 1 {
		t.Fatalf("unexpected worker summary: %+v", summary)
	}
	failed, err := repo.GetIncidentDeletionStatus(ctx, incident.ID)
	if err != nil {
		t.Fatalf("get failed deletion status: %v", err)
	}
	if failed.State != incidents.IncidentDeletionStateFailed || failed.ErrorCode != "blob_delete_failed" {
		t.Fatalf("unexpected failed deletion status: %+v", failed)
	}
	items, err := repo.ListIncidentDeletionItems(ctx, status.DecisionID)
	if err != nil {
		t.Fatalf("list failed deletion items: %v", err)
	}
	if len(items) != 1 || items[0].State != incidents.IncidentDeletionItemStateFailed || items[0].ErrorCode != "unsafe_stored_path" {
		t.Fatalf("unexpected failed deletion item: %+v", items)
	}

	delete(store.removeErrByPath, chunk.StoredPath)
	summary, err = retention.NewWorker(repo, store, retention.Options{}).RunOnce(ctx)
	if err != nil {
		t.Fatalf("rerun worker: %v", err)
	}
	if summary.Completed != 1 || summary.Failed != 0 {
		t.Fatalf("unexpected retry worker summary: %+v", summary)
	}
	completed, err := repo.GetIncidentDeletionStatus(ctx, incident.ID)
	if err != nil {
		t.Fatalf("get completed deletion status: %v", err)
	}
	if completed.State != incidents.IncidentDeletionStateDeleted {
		t.Fatalf("retry did not complete deletion: %+v", completed)
	}
}

func TestWorkerQueuesClosedIncidentRetention(t *testing.T) {
	ctx := context.Background()
	repo, _ := newDeletionTestRepository(t, ctx)
	openIncident, err := repo.CreateIncident(ctx, "open", "")
	if err != nil {
		t.Fatalf("create open incident: %v", err)
	}
	closedIncident, err := repo.CreateIncident(ctx, "closed", "")
	if err != nil {
		t.Fatalf("create closed incident: %v", err)
	}
	if _, err := repo.CloseIncident(ctx, closedIncident.ID); err != nil {
		t.Fatalf("close incident: %v", err)
	}
	time.Sleep(time.Millisecond)

	summary, err := retention.NewWorker(repo, &fakeBlobStore{}, retention.Options{
		ClosedIncidentRetention: time.Nanosecond,
	}).RunOnce(ctx)
	if err != nil {
		t.Fatalf("run retention worker: %v", err)
	}
	if summary.RetentionQueued != 1 || summary.Completed != 1 {
		t.Fatalf("unexpected retention worker summary: %+v", summary)
	}
	if _, err := repo.GetIncidentDeletionStatus(ctx, openIncident.ID); !errors.Is(err, incidents.ErrNotFound) {
		t.Fatalf("open incident deletion status error = %v, want ErrNotFound", err)
	}
	status, err := repo.GetIncidentDeletionStatus(ctx, closedIncident.ID)
	if err != nil {
		t.Fatalf("closed incident deletion status: %v", err)
	}
	if status.Source != incidents.IncidentDeletionSourceRetentionPolicy ||
		status.State != incidents.IncidentDeletionStateDeleted {
		t.Fatalf("unexpected retention deletion status: %+v", status)
	}
}

func TestWorkerPrunesTokenMetadataAndTombstones(t *testing.T) {
	ctx := context.Background()
	repo, _ := newDeletionTestRepository(t, ctx)
	incident, err := repo.CreateIncident(ctx, "phone", "")
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	expiredAt := time.Now().UTC().Add(-2 * time.Hour)
	expiredToken, _, err := repo.CreateIncidentToken(ctx, incident.ID, "expired token label", &expiredAt)
	if err != nil {
		t.Fatalf("create expired token: %v", err)
	}
	futureExpiresAt := time.Now().UTC().Add(2 * time.Hour)
	futureToken, _, err := repo.CreateIncidentToken(ctx, incident.ID, "future token label", &futureExpiresAt)
	if err != nil {
		t.Fatalf("create future token: %v", err)
	}

	deletedIncident, err := repo.CreateIncident(ctx, "deleted", "")
	if err != nil {
		t.Fatalf("create deleted incident: %v", err)
	}
	chunk, err := repo.CreateChunk(ctx, deletionTestChunkParams(deletedIncident.ID, "", incidents.MediaTypeAudio, 1))
	if err != nil {
		t.Fatalf("create deletion chunk: %v", err)
	}
	status, err := repo.RequestIncidentDeletion(ctx, incidents.IncidentDeletionRequest{
		IncidentID: deletedIncident.ID,
		Source:     incidents.IncidentDeletionSourceAdminRequest,
		AllowOpen:  true,
	})
	if err != nil {
		t.Fatalf("request deletion: %v", err)
	}
	store := &fakeBlobStore{}
	summary, err := retention.NewWorker(repo, store, retention.Options{}).RunOnce(ctx)
	if err != nil {
		t.Fatalf("run deletion worker: %v", err)
	}
	if summary.Completed != 1 {
		t.Fatalf("deletion worker summary = %+v, want one completed", summary)
	}
	assertRemovedPaths(t, store.removed, chunk.StoredPath)
	if _, err := repo.GetIncidentDeletionStatus(ctx, deletedIncident.ID); err != nil {
		t.Fatalf("deletion status before tombstone pruning: %v", err)
	}
	time.Sleep(time.Millisecond)

	summary, err = retention.NewWorker(repo, &fakeBlobStore{}, retention.Options{
		TokenMetadataRetention: time.Hour,
		TombstoneRetention:     time.Nanosecond,
	}).RunOnce(ctx)
	if err != nil {
		t.Fatalf("run pruning worker: %v", err)
	}
	if summary.TokenMetadataPruned != 1 || summary.TombstonesPruned != 1 {
		t.Fatalf("unexpected pruning summary: %+v", summary)
	}
	if _, err := repo.GetIncidentToken(ctx, expiredToken.ID); !errors.Is(err, incidents.ErrNotFound) {
		t.Fatalf("expired token lookup error = %v, want ErrNotFound", err)
	}
	if _, err := repo.GetIncidentToken(ctx, futureToken.ID); err != nil {
		t.Fatalf("future token was pruned: %v", err)
	}
	if _, err := repo.GetIncident(ctx, deletedIncident.ID); !errors.Is(err, incidents.ErrNotFound) {
		t.Fatalf("deleted tombstone lookup error = %v, want ErrNotFound", err)
	}
	if _, err := repo.GetIncidentDeletionStatus(ctx, deletedIncident.ID); !errors.Is(err, incidents.ErrNotFound) {
		t.Fatalf("deleted tombstone status error = %v, want ErrNotFound", err)
	}
	if _, err := repo.GetIncident(ctx, incident.ID); err != nil {
		t.Fatalf("active incident was pruned: %v", err)
	}
	if status.State != incidents.IncidentDeletionStatePending {
		t.Fatalf("unexpected original deletion status: %+v", status)
	}
}

type fakeBlobStore struct {
	removed         []string
	removeErrByPath map[string]error
}

func (s *fakeBlobStore) Check(context.Context) error {
	return nil
}

func (s *fakeBlobStore) SaveTemp(context.Context, io.Reader, int64) (*storage.TempUpload, error) {
	return nil, nil
}

func (s *fakeBlobStore) CommitTemp(context.Context, *storage.TempUpload, string, string, string, int) (string, error) {
	return "", nil
}

func (s *fakeBlobStore) Open(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func (s *fakeBlobStore) Remove(_ context.Context, storedPath string) error {
	s.removed = append(s.removed, storedPath)
	if s.removeErrByPath != nil {
		return s.removeErrByPath[storedPath]
	}
	return nil
}

func newDeletionTestRepository(t *testing.T, ctx context.Context) (*incidents.Repository, *sql.DB) {
	t.Helper()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "safety.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})
	return incidents.NewRepository(conn), conn
}

func deletionTestChunkParams(incidentID, streamID, mediaType string, chunkIndex int) incidents.CreateChunkParams {
	startedAt := time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC)
	storedPath := fmt.Sprintf("incidents/%s/%s_%06d.enc", incidentID, mediaType, chunkIndex)
	if streamID != "" {
		storedPath = fmt.Sprintf("incidents/%s/streams/%s/%s_%06d.enc", incidentID, streamID, mediaType, chunkIndex)
	}
	return incidents.CreateChunkParams{
		IncidentID:       incidentID,
		StreamID:         streamID,
		ChunkIndex:       chunkIndex,
		MediaType:        mediaType,
		StartedAt:        startedAt,
		EndedAt:          startedAt.Add(time.Second),
		OriginalFilename: "chunk.enc",
		StoredPath:       storedPath,
		ByteSize:         4,
		SHA256Hex:        strings.Repeat("a", 64),
	}
}

func assertRemovedPaths(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("removed paths = %v, want %v", got, want)
	}
	seen := map[string]bool{}
	for _, path := range got {
		seen[path] = true
	}
	for _, path := range want {
		if !seen[path] {
			t.Fatalf("removed paths = %v, missing %s", got, path)
		}
	}
}

func assertTableCount(t *testing.T, ctx context.Context, conn *sql.DB, tableName string, want int) {
	t.Helper()
	var count int
	if err := conn.QueryRowContext(ctx, "SELECT count(*) FROM "+tableName).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", tableName, err)
	}
	if count != want {
		t.Fatalf("%s count = %d, want %d", tableName, count, want)
	}
}
