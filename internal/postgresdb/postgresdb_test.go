package postgresdb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/incidents"
	"github.com/open-proofline/server/internal/incidents/contracttest"
	"golang.org/x/crypto/bcrypt"
)

func TestPostgresMigrateCreatesSchemaAndRejectsChecksumMismatch(t *testing.T) {
	ctx := context.Background()
	conn := openPostgresTestDB(t, ctx)

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	assertPostgresTable(t, ctx, conn, "schema_migrations")
	assertPostgresTable(t, ctx, conn, "incidents")
	assertPostgresTable(t, ctx, conn, "media_streams")
	assertPostgresTable(t, ctx, conn, "chunks")
	assertPostgresTable(t, ctx, conn, "checkins")
	assertPostgresTable(t, ctx, conn, "incident_tokens")
	assertPostgresTable(t, ctx, conn, "accounts")
	assertPostgresTable(t, ctx, conn, "auth_sessions")
	assertPostgresTable(t, ctx, conn, "upload_operations")
	assertPostgresTable(t, ctx, conn, "incident_deletion_decisions")
	assertPostgresTable(t, ctx, conn, "incident_deletion_items")
	assertPostgresTable(t, ctx, conn, "contact_public_keys")
	assertPostgresTable(t, ctx, conn, "sharing_grants")
	assertPostgresTable(t, ctx, conn, "wrapped_key_records")

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if _, err := conn.ExecContext(ctx, `
		UPDATE schema_migrations
		SET checksum = 'bad'
		WHERE id = '001_init.sql'`); err != nil {
		t.Fatalf("update checksum: %v", err)
	}
	err := Migrate(ctx, conn)
	if err == nil {
		t.Fatal("expected checksum mismatch")
	}
	if !strings.Contains(err.Error(), "001_init.sql checksum mismatch") {
		t.Fatalf("expected checksum mismatch for 001_init.sql, got %v", err)
	}
}

func TestPostgresMigrateSerializesConcurrentCalls(t *testing.T) {
	ctx := context.Background()
	conns := openPostgresTestDBsInOneSchema(t, ctx, 4)

	start := make(chan struct{})
	errCh := make(chan error, len(conns))
	var wg sync.WaitGroup
	for _, conn := range conns {
		conn := conn
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errCh <- Migrate(ctx, conn)
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent Migrate: %v", err)
		}
	}
	assertPostgresTable(t, ctx, conns[0], "schema_migrations")
	assertPostgresTable(t, ctx, conns[0], "incidents")

	var records int
	if err := conns[0].QueryRowContext(ctx, `
		SELECT count(*)
		FROM schema_migrations
		WHERE id = '001_init.sql'`,
	).Scan(&records); err != nil {
		t.Fatalf("count migration records: %v", err)
	}
	if records != 1 {
		t.Fatalf("migration record count = %d, want 1", records)
	}
}

func TestPostgresSchemaConstraints(t *testing.T) {
	ctx := context.Background()
	conn := openPostgresTestDB(t, ctx)
	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	now := time.Now().UTC()
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO incidents (id, created_at, updated_at, status)
		VALUES ($1, $2, $3, $4)`,
		"inc_valid",
		now,
		now,
		incidents.StatusOpen,
	); err != nil {
		t.Fatalf("insert valid incident: %v", err)
	}
	assertPostgresConstraint(t, execErr(conn.ExecContext(ctx, `
		INSERT INTO incidents (id, created_at, updated_at, status)
		VALUES ($1, $2, $3, $4)`,
		"inc_bad_status",
		now,
		now,
		"paused",
	)))
	assertPostgresConstraint(t, execErr(conn.ExecContext(ctx, `
		INSERT INTO incidents (id, created_at, updated_at, status, incident_mode)
		VALUES ($1, $2, $3, $4, $5)`,
		"inc_bad_mode",
		now,
		now,
		incidents.StatusOpen,
		"panic",
	)))
	assertPostgresConstraint(t, execErr(conn.ExecContext(ctx, `
		INSERT INTO media_streams (id, incident_id, media_type, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		"str_bad_media",
		"inc_valid",
		"image",
		incidents.StreamStatusOpen,
		now,
		now,
	)))
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO media_streams (id, incident_id, media_type, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		"str_audio",
		"inc_valid",
		incidents.MediaTypeAudio,
		incidents.StreamStatusOpen,
		now,
		now,
	); err != nil {
		t.Fatalf("insert valid stream: %v", err)
	}
	assertPostgresConstraint(t, execErr(conn.ExecContext(ctx, `
		INSERT INTO chunks (
			id, incident_id, stream_id, chunk_index, media_type, started_at, ended_at,
			stored_path, byte_size, sha256_hex, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		"chk_bad_index",
		"inc_valid",
		"str_audio",
		0,
		incidents.MediaTypeAudio,
		now,
		now,
		"incidents/inc_valid/streams/str_audio/audio_000000.enc",
		int64(1),
		strings.Repeat("a", 64),
		now,
	)))
	assertPostgresConstraint(t, execErr(conn.ExecContext(ctx, `
		INSERT INTO chunks (
			id, incident_id, stream_id, chunk_index, media_type, started_at, ended_at,
			stored_path, byte_size, sha256_hex, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		"chk_bad_sha",
		"inc_valid",
		"str_audio",
		1,
		incidents.MediaTypeAudio,
		now,
		now,
		"incidents/inc_valid/streams/str_audio/audio_000001.enc",
		int64(1),
		"not-a-sha",
		now,
	)))
	assertPostgresConstraint(t, execErr(conn.ExecContext(ctx, `
		INSERT INTO chunks (
			id, incident_id, stream_id, chunk_index, media_type, started_at, ended_at,
			stored_path, byte_size, sha256_hex, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		"chk_wrong_stream_media",
		"inc_valid",
		"str_audio",
		1,
		incidents.MediaTypeVideo,
		now,
		now,
		"incidents/inc_valid/streams/str_audio/video_000001.enc",
		int64(1),
		strings.Repeat("a", 64),
		now,
	)))
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO incident_tokens (id, incident_id, token_hash, created_at)
		VALUES ($1, $2, $3, $4)`,
		"itk_valid",
		"inc_valid",
		strings.Repeat("b", 64),
		now,
	); err != nil {
		t.Fatalf("insert valid token: %v", err)
	}
	assertPostgresConstraint(t, execErr(conn.ExecContext(ctx, `
		INSERT INTO incident_tokens (id, incident_id, token_hash, created_at)
		VALUES ($1, $2, $3, $4)`,
		"itk_duplicate",
		"inc_valid",
		strings.Repeat("b", 64),
		now,
	)))
}

func TestPostgresRepositoryPreservesCoreSemantics(t *testing.T) {
	ctx := context.Background()
	conn := openPostgresTestDB(t, ctx)
	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := NewRepository(conn)

	incident, err := repo.CreateIncident(ctx, "phone", "")
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	firstStream, err := repo.CreateMediaStream(ctx, incident.ID, incidents.MediaTypeAudio, "first audio")
	if err != nil {
		t.Fatalf("create first stream: %v", err)
	}
	secondStream, err := repo.CreateMediaStream(ctx, incident.ID, incidents.MediaTypeAudio, "second audio")
	if err != nil {
		t.Fatalf("create second stream: %v", err)
	}

	if _, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, firstStream.ID, incidents.MediaTypeAudio, 1)); err != nil {
		t.Fatalf("create first stream chunk: %v", err)
	}
	operationParams := testUploadOperationParams(incident.ID, firstStream.ID)
	reserved, err := repo.ReserveUploadOperation(ctx, operationParams)
	if err != nil {
		t.Fatalf("reserve upload operation: %v", err)
	}
	if reserved.State != incidents.UploadOperationStateReserved {
		t.Fatalf("reserved operation state = %q, want %q", reserved.State, incidents.UploadOperationStateReserved)
	}
	conflictingOperation := operationParams
	conflictingOperation.OriginalFilename = "other.enc"
	conflictingOperation.FingerprintHash = strings.Repeat("c", 64)
	if _, err := repo.ReserveUploadOperation(ctx, conflictingOperation); !errors.Is(err, incidents.ErrIdempotencyConflict) {
		t.Fatalf("conflicting upload operation error = %v, want ErrIdempotencyConflict", err)
	}
	firstChunk, err := repo.GetChunkByIdentity(ctx, incident.ID, firstStream.ID, incidents.MediaTypeAudio, 1)
	if err != nil {
		t.Fatalf("get first stream chunk by identity: %v", err)
	}
	completedOperation, err := repo.CompleteUploadOperation(ctx, operationParams, firstChunk)
	if err != nil {
		t.Fatalf("complete upload operation: %v", err)
	}
	if completedOperation.State != incidents.UploadOperationStateMetadataCommitted || completedOperation.ChunkID != firstChunk.ID {
		t.Fatalf("unexpected completed operation: %+v", completedOperation)
	}
	replayedOperation, err := repo.ReserveUploadOperation(ctx, operationParams)
	if err != nil {
		t.Fatalf("reserve completed upload operation: %v", err)
	}
	if replayedOperation.State != incidents.UploadOperationStateMetadataCommitted || replayedOperation.ChunkID != firstChunk.ID {
		t.Fatalf("expected completed operation replay, got %+v", replayedOperation)
	}
	if _, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, secondStream.ID, incidents.MediaTypeAudio, 1)); err != nil {
		t.Fatalf("create second stream chunk: %v", err)
	}
	legacy, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, "", incidents.MediaTypeAudio, 1))
	if err != nil {
		t.Fatalf("create legacy chunk: %v", err)
	}
	if _, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, firstStream.ID, incidents.MediaTypeAudio, 1)); !errors.Is(err, incidents.ErrDuplicate) {
		t.Fatalf("duplicate stream chunk error = %v, want ErrDuplicate", err)
	}
	got, err := repo.GetChunkByKey(ctx, incident.ID, incidents.MediaTypeAudio, 1)
	if err != nil {
		t.Fatalf("get legacy chunk: %v", err)
	}
	if got.ID != legacy.ID || got.StreamID != "" {
		t.Fatalf("expected legacy chunk %+v, got %+v", legacy, got)
	}
	if _, err := repo.CompleteMediaStream(ctx, incident.ID, firstStream.ID, 1); err != nil {
		t.Fatalf("complete stream: %v", err)
	}
	if _, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, firstStream.ID, incidents.MediaTypeAudio, 2)); !errors.Is(err, incidents.ErrInvalidState) {
		t.Fatalf("create chunk on completed stream error = %v, want ErrInvalidState", err)
	}
	if _, err := repo.CloseIncident(ctx, incident.ID); err != nil {
		t.Fatalf("close incident: %v", err)
	}
	if _, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, "", incidents.MediaTypeAudio, 2)); !errors.Is(err, incidents.ErrIncidentClosed) {
		t.Fatalf("create chunk on closed incident error = %v, want ErrIncidentClosed", err)
	}
	deletionStatus, err := repo.RequestIncidentDeletion(ctx, incidents.IncidentDeletionRequest{
		IncidentID: incident.ID,
		Source:     incidents.IncidentDeletionSourceAdminRequest,
	})
	if err != nil {
		t.Fatalf("request deletion: %v", err)
	}
	if deletionStatus.State != incidents.IncidentDeletionStatePending || deletionStatus.ItemCount != 3 {
		t.Fatalf("unexpected postgres deletion status: %+v", deletionStatus)
	}
	items, err := repo.ListIncidentDeletionItems(ctx, deletionStatus.DecisionID)
	if err != nil {
		t.Fatalf("list deletion items: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("postgres deletion item count = %d, want 3", len(items))
	}
	if _, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, "", incidents.MediaTypeAudio, 2)); !errors.Is(err, incidents.ErrIncidentDeleting) {
		t.Fatalf("create chunk during deletion error = %v, want ErrIncidentDeleting", err)
	}
	if _, err := repo.CreateCheckin(ctx, incident.ID, incidents.CreateCheckinParams{}); !errors.Is(err, incidents.ErrIncidentDeleting) {
		t.Fatalf("create checkin during deletion error = %v, want ErrIncidentDeleting", err)
	}
	if _, err := repo.CreateMediaStream(ctx, incident.ID, incidents.MediaTypeAudio, "late audio"); !errors.Is(err, incidents.ErrIncidentDeleting) {
		t.Fatalf("create stream during deletion error = %v, want ErrIncidentDeleting", err)
	}
	if _, err := repo.FailMediaStream(ctx, incident.ID, secondStream.ID, "late failure"); !errors.Is(err, incidents.ErrIncidentDeleting) {
		t.Fatalf("fail stream during deletion error = %v, want ErrIncidentDeleting", err)
	}
	if _, err := repo.CloseIncident(ctx, incident.ID); !errors.Is(err, incidents.ErrIncidentDeleting) {
		t.Fatalf("close incident during deletion error = %v, want ErrIncidentDeleting", err)
	}
	deletedOperationParams := testUploadOperationParams(incident.ID, secondStream.ID)
	deletedOperationParams.IdempotencyKeyHash = strings.Repeat("d", 64)
	deletedOperationParams.FingerprintHash = strings.Repeat("e", 64)
	if _, err := repo.ReserveUploadOperation(ctx, deletedOperationParams); !errors.Is(err, incidents.ErrIncidentDeleting) {
		t.Fatalf("reserve upload operation during deletion error = %v, want ErrIncidentDeleting", err)
	}
	if _, _, err := repo.CreateIncidentToken(ctx, incident.ID, "trusted contact", nil); !errors.Is(err, incidents.ErrNotFound) {
		t.Fatalf("create token during deletion error = %v, want ErrNotFound", err)
	}
}

func TestPostgresDeletionOperatorStatusAndPreview(t *testing.T) {
	ctx := context.Background()
	conn := openPostgresTestDB(t, ctx)
	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := NewRepository(conn)

	closedIncident, err := repo.CreateIncident(ctx, "closed", "")
	if err != nil {
		t.Fatalf("create closed incident: %v", err)
	}
	failedIncident, err := repo.CreateIncident(ctx, "failed", "")
	if err != nil {
		t.Fatalf("create failed incident: %v", err)
	}
	if _, err := repo.CloseIncident(ctx, closedIncident.ID); err != nil {
		t.Fatalf("close candidate incident: %v", err)
	}
	if _, err := repo.CreateChunk(ctx, testChunkParams(failedIncident.ID, "", incidents.MediaTypeAudio, 1)); err != nil {
		t.Fatalf("create chunk: %v", err)
	}
	status, err := repo.RequestIncidentDeletion(ctx, incidents.IncidentDeletionRequest{
		IncidentID: failedIncident.ID,
		Source:     incidents.IncidentDeletionSourceAdminRequest,
		AllowOpen:  true,
	})
	if err != nil {
		t.Fatalf("request deletion: %v", err)
	}
	items, err := repo.ListIncidentDeletionItems(ctx, status.DecisionID)
	if err != nil {
		t.Fatalf("list deletion items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("deletion items = %+v, want one", items)
	}
	if err := repo.MarkIncidentDeletionItemFailed(ctx, items[0].ID, "unsafe_stored_path"); err != nil {
		t.Fatalf("mark item failed: %v", err)
	}
	if _, err := repo.FailIncidentDeletion(ctx, status.DecisionID, "blob_delete_failed"); err != nil {
		t.Fatalf("fail deletion: %v", err)
	}

	candidates, err := repo.ListRetentionDeletionCandidates(ctx, time.Now().UTC().Add(time.Minute), 10)
	if err != nil {
		t.Fatalf("list retention candidates: %v", err)
	}
	if len(candidates) != 1 || candidates[0].IncidentID != closedIncident.ID {
		t.Fatalf("retention candidates = %+v, want only %s", candidates, closedIncident.ID)
	}

	report, err := repo.GetIncidentDeletionJobStatus(ctx, 10, time.Now().UTC().Add(time.Minute))
	if err != nil {
		t.Fatalf("get deletion job status: %v", err)
	}
	assertPostgresDecisionStateCount(t, report.DecisionStateCounts, incidents.IncidentDeletionStateFailed, 1)
	assertPostgresDecisionErrorCount(t, report.DecisionErrorCounts, incidents.IncidentDeletionStateFailed, "blob_delete_failed", 1)
	assertPostgresItemStateCount(t, report.ItemStateCounts, incidents.IncidentDeletionItemStateFailed, "unsafe_stored_path", 1)
	if len(report.RunnableJobs) != 1 || report.RunnableJobs[0].DecisionID != status.DecisionID {
		t.Fatalf("unexpected runnable jobs: %+v", report.RunnableJobs)
	}
}

func TestPostgresRetentionPruning(t *testing.T) {
	ctx := context.Background()
	conn := openPostgresTestDB(t, ctx)
	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := NewRepository(conn)

	incident, err := repo.CreateIncident(ctx, "phone", "")
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	now := time.Now().UTC()
	expiredAt := now.Add(-2 * time.Hour)
	futureExpiresAt := now.Add(2 * time.Hour)
	expiredToken, _, err := repo.CreateIncidentToken(ctx, incident.ID, "expired token label", &expiredAt)
	if err != nil {
		t.Fatalf("create expired token: %v", err)
	}
	futureToken, _, err := repo.CreateIncidentToken(ctx, incident.ID, "future token label", &futureExpiresAt)
	if err != nil {
		t.Fatalf("create future token: %v", err)
	}
	revokedToken, _, err := repo.CreateIncidentToken(ctx, incident.ID, "revoked token label", nil)
	if err != nil {
		t.Fatalf("create revoked token: %v", err)
	}
	if err := repo.RevokeIncidentToken(ctx, revokedToken.ID); err != nil {
		t.Fatalf("revoke token: %v", err)
	}

	pruned, err := repo.PruneIncidentTokenMetadata(ctx, now.Add(-time.Hour), 25)
	if err != nil {
		t.Fatalf("prune expired token metadata: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("expired token metadata pruned = %d, want 1", pruned)
	}
	if _, err := repo.GetIncidentToken(ctx, expiredToken.ID); !errors.Is(err, incidents.ErrNotFound) {
		t.Fatalf("expired token lookup error = %v, want ErrNotFound", err)
	}
	if _, err := repo.GetIncidentToken(ctx, futureToken.ID); err != nil {
		t.Fatalf("future token was pruned: %v", err)
	}

	deletedIncident, err := repo.CreateIncident(ctx, "deleted", "")
	if err != nil {
		t.Fatalf("create deleted incident: %v", err)
	}
	if _, err := repo.CreateChunk(ctx, testChunkParams(deletedIncident.ID, "", incidents.MediaTypeAudio, 1)); err != nil {
		t.Fatalf("create chunk: %v", err)
	}
	status, err := repo.RequestIncidentDeletion(ctx, incidents.IncidentDeletionRequest{
		IncidentID: deletedIncident.ID,
		Source:     incidents.IncidentDeletionSourceAdminRequest,
		AllowOpen:  true,
	})
	if err != nil {
		t.Fatalf("request deletion: %v", err)
	}
	items, err := repo.ListIncidentDeletionItems(ctx, status.DecisionID)
	if err != nil {
		t.Fatalf("list deletion items: %v", err)
	}
	for _, item := range items {
		if err := repo.MarkIncidentDeletionItemDeleted(ctx, item.ID); err != nil {
			t.Fatalf("mark deletion item deleted: %v", err)
		}
	}
	if _, err := repo.CompleteIncidentDeletion(ctx, status.DecisionID); err != nil {
		t.Fatalf("complete deletion: %v", err)
	}

	pruned, err = repo.PruneIncidentDeletionTombstones(ctx, time.Now().UTC().Add(time.Minute), 25)
	if err != nil {
		t.Fatalf("prune tombstone: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("tombstones pruned = %d, want 1", pruned)
	}
	if _, err := repo.GetIncident(ctx, deletedIncident.ID); !errors.Is(err, incidents.ErrNotFound) {
		t.Fatalf("deleted tombstone lookup error = %v, want ErrNotFound", err)
	}
	if _, err := repo.GetIncident(ctx, incident.ID); err != nil {
		t.Fatalf("active incident was pruned: %v", err)
	}
}

func TestPostgresContactPublicKeysAndSharingGrants(t *testing.T) {
	ctx := context.Background()
	conn := openPostgresTestDB(t, ctx)
	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := NewRepository(conn)
	owner, err := repo.CreateAccount(ctx, auth.CreateAccountParams{
		Username:     "grant-owner",
		PasswordHash: "hash",
		Role:         auth.RoleUser,
	})
	if err != nil {
		t.Fatalf("create owner account: %v", err)
	}
	incident, err := repo.CreateIncidentForAccount(ctx, owner.ID, incidents.CreateIncidentParams{})
	if err != nil {
		t.Fatalf("create owner incident: %v", err)
	}
	contactKey, err := repo.CreateContactPublicKey(ctx, incidents.CreateContactPublicKeyParams{
		OwnerAccountID:       owner.ID,
		DisplayLabel:         "contact",
		WrappingAlgorithm:    "age-v1-x25519",
		PublicKey:            "age1first",
		PublicKeyFingerprint: "fingerprint-1",
		KeyState:             incidents.ContactKeyStateActive,
	})
	if err != nil {
		t.Fatalf("create contact public key: %v", err)
	}
	grant, err := repo.CreateSharingGrant(ctx, incidents.CreateSharingGrantParams{
		OwnerAccountID: owner.ID,
		IncidentID:     incident.ID,
		RecipientType:  incidents.SharingGrantRecipientTrustedContact,
		ContactID:      contactKey.ContactID,
		DataClass:      incidents.SharingGrantDataClassMetadataCiphertext,
	})
	if err != nil {
		t.Fatalf("create sharing grant: %v", err)
	}
	if grant.ContactPublicKeyID != contactKey.ID || grant.ContactPublicKeyVersion != 1 {
		t.Fatalf("unexpected grant key binding: %+v", grant)
	}
	record, err := repo.CreateWrappedKeyRecord(ctx, incidents.CreateWrappedKeyRecordParams{
		OwnerAccountID:           owner.ID,
		IncidentID:               incident.ID,
		GrantID:                  grant.ID,
		MediaKeyID:               "media-key-postgres",
		WrappingAlgorithm:        "age-v1-x25519",
		WrappingAlgorithmVersion: "1",
		WrappedKeyCiphertext:     "wrapped-ciphertext",
		PublicWrappingMetadata:   []byte(`{"profile":"age-v1-x25519"}`),
	})
	if err != nil {
		t.Fatalf("create wrapped key record: %v", err)
	}
	records, err := repo.ListWrappedKeyRecords(ctx, owner.ID, incident.ID)
	if err != nil {
		t.Fatalf("list wrapped key records: %v", err)
	}
	if len(records) != 1 || records[0].ID != record.ID {
		t.Fatalf("unexpected wrapped key records: %+v", records)
	}
	if _, err := repo.RevokeSharingGrant(ctx, owner.ID, grant.ID, owner.ID); err != nil {
		t.Fatalf("revoke sharing grant: %v", err)
	}
	if _, err := repo.GetWrappedKeyRecord(ctx, owner.ID, record.ID); !errors.Is(err, incidents.ErrNotFound) {
		t.Fatalf("revoked grant get wrapped key error = %v, want ErrNotFound", err)
	}
}

func TestPostgresUploadOperationRaceAndBackendParity(t *testing.T) {
	contracttest.RunUploadOperationRaceAndParity(t, func(t *testing.T, ctx context.Context) contracttest.Repository {
		t.Helper()
		conn := openPostgresTestDB(t, ctx)
		if err := Migrate(ctx, conn); err != nil {
			t.Fatalf("Migrate: %v", err)
		}
		return NewRepository(conn)
	})
}

func TestPostgresRepositoryHashesAndRevokesIncidentTokens(t *testing.T) {
	ctx := context.Background()
	conn := openPostgresTestDB(t, ctx)
	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := NewRepository(conn)
	incident, err := repo.CreateIncident(ctx, "phone", "")
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}

	token, rawToken, err := repo.CreateIncidentToken(ctx, incident.ID, "trusted contact", nil)
	if err != nil {
		t.Fatalf("create incident token: %v", err)
	}
	if rawToken == "" {
		t.Fatal("raw token was empty")
	}
	var storedHash string
	if err := conn.QueryRowContext(ctx, `
		SELECT token_hash
		FROM incident_tokens
		WHERE id = $1`,
		token.ID,
	).Scan(&storedHash); err != nil {
		t.Fatalf("read token hash: %v", err)
	}
	if storedHash == rawToken || len(storedHash) != 64 {
		t.Fatalf("token storage did not use a 64-character hash")
	}
	lookedUp, err := repo.LookupIncidentToken(ctx, rawToken)
	if err != nil {
		t.Fatalf("lookup token: %v", err)
	}
	if lookedUp.ID != token.ID {
		t.Fatalf("looked up token id = %q, want %q", lookedUp.ID, token.ID)
	}
	if err := repo.RevokeIncidentToken(ctx, token.ID); err != nil {
		t.Fatalf("revoke token: %v", err)
	}
	if _, err := repo.LookupIncidentToken(ctx, rawToken); !errors.Is(err, incidents.ErrNotFound) {
		t.Fatalf("lookup revoked token error = %v, want ErrNotFound", err)
	}
}

func TestPostgresRepositoryStoresIncidentModeFields(t *testing.T) {
	ctx := context.Background()
	conn := openPostgresTestDB(t, ctx)
	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := NewRepository(conn)

	incident, err := repo.CreateIncidentForAccount(ctx, "", incidents.CreateIncidentParams{
		ClientLabel:      "phone",
		IncidentMode:     incidents.IncidentModeEmergency,
		CaptureProfile:   incidents.CaptureProfileAudioVideoLocation,
		EscalationPolicy: incidents.EscalationPolicyTrustedContactsOnStart,
		SharingState:     incidents.SharingStatePrivate,
	})
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	got, err := repo.GetIncident(ctx, incident.ID)
	if err != nil {
		t.Fatalf("get incident: %v", err)
	}
	if got.IncidentMode != incidents.IncidentModeEmergency ||
		got.CaptureProfile != incidents.CaptureProfileAudioVideoLocation ||
		got.EscalationPolicy != incidents.EscalationPolicyTrustedContactsOnStart ||
		got.SharingState != incidents.SharingStatePrivate {
		t.Fatalf("incident mode fields were not preserved: %+v", got)
	}
}

func TestPostgresRepositoryAccountsAndSessions(t *testing.T) {
	ctx := context.Background()
	conn := openPostgresTestDB(t, ctx)
	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	repo := NewRepository(conn)

	hasAccounts, err := repo.HasAccounts(ctx)
	if err != nil {
		t.Fatalf("has accounts: %v", err)
	}
	if hasAccounts {
		t.Fatal("expected fresh schema to have no accounts")
	}
	hasAdmin, err := repo.HasAdminAccount(ctx)
	if err != nil {
		t.Fatalf("has admin: %v", err)
	}
	if hasAdmin {
		t.Fatal("expected fresh schema to have no admin accounts")
	}

	adminPasswordHash, err := auth.HashPassword("test-password", bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash admin password: %v", err)
	}
	admin, err := repo.CreateAccount(ctx, auth.CreateAccountParams{
		Username:     "Admin.User",
		PasswordHash: adminPasswordHash,
		Role:         auth.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("create admin account: %v", err)
	}
	if admin.Username != "admin.user" || admin.Role != auth.RoleAdmin {
		t.Fatalf("unexpected admin account: %+v", admin)
	}
	if _, err := repo.CreateAccount(ctx, auth.CreateAccountParams{
		Username:     "admin.user",
		PasswordHash: adminPasswordHash,
		Role:         auth.RoleAdmin,
	}); !errors.Is(err, auth.ErrDuplicate) {
		t.Fatalf("duplicate account error = %v, want ErrDuplicate", err)
	}

	hasAccounts, err = repo.HasAccounts(ctx)
	if err != nil {
		t.Fatalf("has accounts after create: %v", err)
	}
	if !hasAccounts {
		t.Fatal("expected account existence after create")
	}
	hasAdmin, err = repo.HasAdminAccount(ctx)
	if err != nil {
		t.Fatalf("has admin after create: %v", err)
	}
	if !hasAdmin {
		t.Fatal("expected admin existence after create")
	}

	gotAdmin, err := repo.GetAccountByUsername(ctx, " ADMIN.USER ")
	if err != nil {
		t.Fatalf("get admin by username: %v", err)
	}
	if gotAdmin.ID != admin.ID || gotAdmin.PasswordHash != adminPasswordHash {
		t.Fatalf("got admin %+v, want id %q and stored hash", gotAdmin, admin.ID)
	}

	updatedHash, err := auth.HashPassword("updated-password", bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash updated password: %v", err)
	}
	updatedAdmin, err := repo.UpdateAccountPassword(ctx, admin.ID, updatedHash)
	if err != nil {
		t.Fatalf("update admin password: %v", err)
	}
	if updatedAdmin.PasswordHash != updatedHash {
		t.Fatal("updated account did not return new password hash")
	}
	if updatedAdmin.PasswordChangedAt.Before(admin.PasswordChangedAt) {
		t.Fatalf("password_changed_at moved backward: before=%s after=%s", admin.PasswordChangedAt, updatedAdmin.PasswordChangedAt)
	}

	userPasswordHash, err := auth.HashPassword("regular-password", bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash user password: %v", err)
	}
	user, err := repo.CreateAccount(ctx, auth.CreateAccountParams{
		Username:     "regular-user",
		PasswordHash: userPasswordHash,
		Role:         auth.RoleUser,
	})
	if err != nil {
		t.Fatalf("create user account: %v", err)
	}
	incident, err := repo.CreateIncidentForAccount(ctx, user.ID, incidents.CreateIncidentParams{ClientLabel: "phone"})
	if err != nil {
		t.Fatalf("create owned incident: %v", err)
	}
	if incident.OwnerAccountID != user.ID {
		t.Fatalf("incident owner = %q, want %q", incident.OwnerAccountID, user.ID)
	}

	session, rawToken, err := repo.CreateSession(ctx, user.ID, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if rawToken == "" {
		t.Fatal("raw session token was empty")
	}
	var storedHash string
	if err := conn.QueryRowContext(ctx, `
		SELECT token_hash
		FROM auth_sessions
		WHERE id = $1`,
		session.ID,
	).Scan(&storedHash); err != nil {
		t.Fatalf("read session token hash: %v", err)
	}
	if storedHash == rawToken || len(storedHash) != 64 {
		t.Fatalf("session storage did not use a 64-character hash")
	}
	lookedUp, err := repo.LookupSession(ctx, rawToken)
	if err != nil {
		t.Fatalf("lookup session: %v", err)
	}
	if lookedUp.ID != session.ID || lookedUp.AccountID != user.ID {
		t.Fatalf("looked up session %+v, want session %q for account %q", lookedUp, session.ID, user.ID)
	}
	if err := repo.RevokeSession(ctx, session.ID); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	if _, err := repo.LookupSession(ctx, rawToken); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("lookup revoked session error = %v, want ErrNotFound", err)
	}

	expiredSession, expiredRawToken, err := repo.CreateSession(ctx, user.ID, time.Now().UTC().Add(-time.Second))
	if err != nil {
		t.Fatalf("create expired session: %v", err)
	}
	if _, err := repo.LookupSession(ctx, expiredRawToken); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("lookup expired session %q error = %v, want ErrNotFound", expiredSession.ID, err)
	}

	keptSession, keptRawToken, err := repo.CreateSession(ctx, user.ID, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("create kept session: %v", err)
	}
	revokedSession, revokedRawToken, err := repo.CreateSession(ctx, user.ID, time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("create revokable session: %v", err)
	}
	revoked, err := repo.RevokeAccountSessions(ctx, user.ID, keptSession.ID)
	if err != nil {
		t.Fatalf("revoke account sessions: %v", err)
	}
	if revoked != 2 {
		t.Fatalf("revoked sessions = %d, want 2", revoked)
	}
	if _, err := repo.LookupSession(ctx, keptRawToken); err != nil {
		t.Fatalf("kept session lookup after account revoke: %v", err)
	}
	if _, err := repo.LookupSession(ctx, revokedRawToken); !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("lookup revoked account session %q error = %v, want ErrNotFound", revokedSession.ID, err)
	}
}

func openPostgresTestDB(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()
	conn := openPostgresConnection(t, ctx, postgresTestDSN(t))
	schema := "proofline_test_" + randomHex(t, 8)
	quotedSchema := quotePostgresIdentifier(schema)
	if _, err := conn.ExecContext(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		_ = conn.Close()
		t.Fatalf("create test schema: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "SET search_path TO "+quotedSchema); err != nil {
		_ = conn.Close()
		t.Fatalf("set test schema search path: %v", err)
	}
	t.Cleanup(func() {
		_, _ = conn.ExecContext(context.Background(), "DROP SCHEMA "+quotedSchema+" CASCADE")
		_ = conn.Close()
	})
	return conn
}

func openPostgresTestDBsInOneSchema(t *testing.T, ctx context.Context, count int) []*sql.DB {
	t.Helper()
	if count <= 0 {
		t.Fatal("postgres test database count must be positive")
	}

	dsn := postgresTestDSN(t)
	admin := openPostgresConnection(t, ctx, dsn)
	schema := "proofline_test_" + randomHex(t, 8)
	quotedSchema := quotePostgresIdentifier(schema)
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		_ = admin.Close()
		t.Fatalf("create shared test schema: %v", err)
	}

	conns := make([]*sql.DB, 0, count)
	t.Cleanup(func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
		_, _ = admin.ExecContext(context.Background(), "DROP SCHEMA "+quotedSchema+" CASCADE")
		_ = admin.Close()
	})

	for range count {
		conn := openPostgresConnection(t, ctx, dsn)
		if _, err := conn.ExecContext(ctx, "SET search_path TO "+quotedSchema); err != nil {
			_ = conn.Close()
			t.Fatalf("set shared test schema search path: %v", err)
		}
		conns = append(conns, conn)
	}
	return conns
}

func postgresTestDSN(t *testing.T) string {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("SAFE_POSTGRES_TEST_DSN"))
	if dsn == "" {
		t.Skip("SAFE_POSTGRES_TEST_DSN is not set")
	}
	return dsn
}

func openPostgresConnection(t *testing.T, ctx context.Context, dsn string) *sql.DB {
	t.Helper()
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal("open postgres test database failed")
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		t.Fatal("connect postgres test database failed; verify SAFE_POSTGRES_TEST_DSN")
	}
	return conn
}

func quotePostgresIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func randomHex(t *testing.T, byteCount int) string {
	t.Helper()
	buf := make([]byte, byteCount)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("generate random suffix: %v", err)
	}
	return hex.EncodeToString(buf)
}

func assertPostgresTable(t *testing.T, ctx context.Context, conn *sql.DB, tableName string) {
	t.Helper()
	var exists bool
	if err := conn.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", tableName).Scan(&exists); err != nil {
		t.Fatalf("check table %s: %v", tableName, err)
	}
	if !exists {
		t.Fatalf("expected table %s to exist", tableName)
	}
}

func execErr(_ sql.Result, err error) error {
	return err
}

func assertPostgresConstraint(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected postgres constraint error")
	}
	if !isIntegrityConstraint(err) {
		t.Fatalf("expected postgres integrity constraint error, got %v", err)
	}
}

func testChunkParams(incidentID, streamID, mediaType string, chunkIndex int) incidents.CreateChunkParams {
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

func testUploadOperationParams(incidentID, streamID string) incidents.UploadOperationParams {
	chunk := testChunkParams(incidentID, streamID, incidents.MediaTypeAudio, 1)
	return incidents.UploadOperationParams{
		Operation:          incidents.UploadOperationUploadChunk,
		IdempotencyKeyHash: strings.Repeat("b", 64),
		IncidentID:         incidentID,
		StreamID:           streamID,
		ChunkIndex:         chunk.ChunkIndex,
		MediaType:          chunk.MediaType,
		StartedAt:          chunk.StartedAt,
		EndedAt:            chunk.EndedAt,
		OriginalFilename:   chunk.OriginalFilename,
		ByteSize:           chunk.ByteSize,
		SHA256Hex:          chunk.SHA256Hex,
		FingerprintHash:    strings.Repeat("a", 64),
	}
}

func assertPostgresDecisionStateCount(t *testing.T, counts []incidents.IncidentDeletionStateCount, state string, want int) {
	t.Helper()
	for _, count := range counts {
		if count.State == state {
			if count.Count != want {
				t.Fatalf("decision state count for %q = %d, want %d", state, count.Count, want)
			}
			return
		}
	}
	t.Fatalf("decision state count for %q missing in %+v", state, counts)
}

func assertPostgresDecisionErrorCount(t *testing.T, counts []incidents.IncidentDeletionErrorCount, state, errorCode string, want int) {
	t.Helper()
	for _, count := range counts {
		if count.State == state && count.ErrorCode == errorCode {
			if count.Count != want {
				t.Fatalf("decision error count for %q/%q = %d, want %d", state, errorCode, count.Count, want)
			}
			return
		}
	}
	t.Fatalf("decision error count for %q/%q missing in %+v", state, errorCode, counts)
}

func assertPostgresItemStateCount(t *testing.T, counts []incidents.IncidentDeletionItemStateCount, state, errorCode string, want int) {
	t.Helper()
	for _, count := range counts {
		if count.State == state && count.ErrorCode == errorCode {
			if count.Count != want {
				t.Fatalf("item state count for %q/%q = %d, want %d", state, errorCode, count.Count, want)
			}
			return
		}
	}
	t.Fatalf("item state count for %q/%q missing in %+v", state, errorCode, counts)
}
