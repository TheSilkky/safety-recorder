package incidents_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/db"
	"github.com/open-proofline/server/internal/incidents"
	"github.com/open-proofline/server/internal/incidents/contracttest"
)

func TestCreateIncidentStoresModeFields(t *testing.T) {
	ctx := context.Background()
	repo := newRepository(t, ctx)

	incident, err := repo.CreateIncidentForAccount(ctx, "", incidents.CreateIncidentParams{
		ClientLabel:      "phone",
		IncidentMode:     incidents.IncidentModeSafetyCheck,
		CaptureProfile:   incidents.CaptureProfileLocationCheckin,
		EscalationPolicy: incidents.EscalationPolicyTrustedContactsOnMissedCheckin,
		SharingState:     incidents.SharingStatePrivate,
	})
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}

	got, err := repo.GetIncident(ctx, incident.ID)
	if err != nil {
		t.Fatalf("get incident: %v", err)
	}
	if got.IncidentMode != incidents.IncidentModeSafetyCheck ||
		got.CaptureProfile != incidents.CaptureProfileLocationCheckin ||
		got.EscalationPolicy != incidents.EscalationPolicyTrustedContactsOnMissedCheckin ||
		got.SharingState != incidents.SharingStatePrivate {
		t.Fatalf("incident mode fields were not preserved: %+v", got)
	}
}

func TestContactPublicKeysAndSharingGrants(t *testing.T) {
	ctx := context.Background()
	repo := newRepository(t, ctx)
	owner, err := repo.CreateAccount(ctx, auth.CreateAccountParams{
		Username:     "grant-owner",
		PasswordHash: "hash",
		Role:         auth.RoleUser,
	})
	if err != nil {
		t.Fatalf("create owner account: %v", err)
	}
	other, err := repo.CreateAccount(ctx, auth.CreateAccountParams{
		Username:     "grant-other",
		PasswordHash: "hash",
		Role:         auth.RoleUser,
	})
	if err != nil {
		t.Fatalf("create other account: %v", err)
	}
	incident, err := repo.CreateIncidentForAccount(ctx, owner.ID, incidents.CreateIncidentParams{})
	if err != nil {
		t.Fatalf("create owner incident: %v", err)
	}
	stream, err := repo.CreateMediaStream(ctx, incident.ID, incidents.MediaTypeAudio, "audio")
	if err != nil {
		t.Fatalf("create media stream: %v", err)
	}
	firstKey, err := repo.CreateContactPublicKey(ctx, incidents.CreateContactPublicKeyParams{
		OwnerAccountID:       owner.ID,
		DisplayLabel:         "contact",
		WrappingAlgorithm:    "age-v1-x25519",
		PublicKey:            "age1first",
		PublicKeyFingerprint: "fingerprint-1",
		KeyState:             incidents.ContactKeyStateActive,
	})
	if err != nil {
		t.Fatalf("create first contact key: %v", err)
	}
	secondKey, err := repo.CreateContactPublicKey(ctx, incidents.CreateContactPublicKeyParams{
		OwnerAccountID:       owner.ID,
		ContactID:            firstKey.ContactID,
		DisplayLabel:         "contact replacement",
		WrappingAlgorithm:    "age-v1-x25519",
		PublicKey:            "age1second",
		PublicKeyFingerprint: "fingerprint-2",
		KeyState:             incidents.ContactKeyStateActive,
	})
	if err != nil {
		t.Fatalf("create replacement contact key: %v", err)
	}
	if secondKey.Version != 2 || secondKey.ContactID != firstKey.ContactID {
		t.Fatalf("replacement key = %+v, want same contact version 2", secondKey)
	}

	if _, err := repo.GetContactPublicKey(ctx, other.ID, firstKey.ID); !errors.Is(err, incidents.ErrNotFound) {
		t.Fatalf("other account get contact key error = %v, want ErrNotFound", err)
	}
	grant, err := repo.CreateSharingGrant(ctx, incidents.CreateSharingGrantParams{
		OwnerAccountID: owner.ID,
		IncidentID:     incident.ID,
		StreamID:       stream.ID,
		RecipientType:  incidents.SharingGrantRecipientTrustedContact,
		ContactID:      firstKey.ContactID,
		DataClass:      incidents.SharingGrantDataClassMetadataCiphertext,
	})
	if err != nil {
		t.Fatalf("create sharing grant: %v", err)
	}
	if grant.ContactPublicKeyID != secondKey.ID || grant.ContactPublicKeyVersion != 2 {
		t.Fatalf("grant used key %q version %d, want %q version 2", grant.ContactPublicKeyID, grant.ContactPublicKeyVersion, secondKey.ID)
	}
	if _, err := repo.CreateSharingGrant(ctx, incidents.CreateSharingGrantParams{
		OwnerAccountID: other.ID,
		IncidentID:     incident.ID,
		RecipientType:  incidents.SharingGrantRecipientTrustedContact,
		ContactID:      firstKey.ContactID,
		DataClass:      incidents.SharingGrantDataClassMetadataCiphertext,
	}); !errors.Is(err, incidents.ErrNotFound) {
		t.Fatalf("other account create grant error = %v, want ErrNotFound", err)
	}

	revokedGrant, err := repo.RevokeSharingGrant(ctx, owner.ID, grant.ID, owner.ID)
	if err != nil {
		t.Fatalf("revoke sharing grant: %v", err)
	}
	if revokedGrant.GrantState != incidents.SharingGrantStateRevoked || revokedGrant.RevokedAt == nil {
		t.Fatalf("grant not revoked: %+v", revokedGrant)
	}
	revokedKey, err := repo.RevokeContactPublicKey(ctx, owner.ID, secondKey.ID)
	if err != nil {
		t.Fatalf("revoke contact public key: %v", err)
	}
	if revokedKey.KeyState != incidents.ContactKeyStateRevoked || revokedKey.RevokedAt == nil {
		t.Fatalf("contact key not revoked: %+v", revokedKey)
	}
	active := incidents.ContactKeyStateActive
	if _, err := repo.UpdateContactPublicKey(ctx, incidents.UpdateContactPublicKeyParams{
		OwnerAccountID: owner.ID,
		PublicKeyID:    secondKey.ID,
		KeyState:       &active,
	}); !errors.Is(err, incidents.ErrInvalidState) {
		t.Fatalf("reactivate revoked key error = %v, want ErrInvalidState", err)
	}
}

func TestWrappedKeyRecords(t *testing.T) {
	ctx := context.Background()
	repo := newRepository(t, ctx)
	owner, err := repo.CreateAccount(ctx, auth.CreateAccountParams{
		Username:     "wrapped-owner",
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
	stream, err := repo.CreateMediaStream(ctx, incident.ID, incidents.MediaTypeAudio, "audio")
	if err != nil {
		t.Fatalf("create media stream: %v", err)
	}
	contactKey, err := repo.CreateContactPublicKey(ctx, incidents.CreateContactPublicKeyParams{
		OwnerAccountID:       owner.ID,
		DisplayLabel:         "contact",
		WrappingAlgorithm:    "age-v1-x25519",
		PublicKey:            "age1wrapped",
		PublicKeyFingerprint: "fingerprint-wrapped",
		KeyState:             incidents.ContactKeyStateActive,
	})
	if err != nil {
		t.Fatalf("create contact key: %v", err)
	}
	grant, err := repo.CreateSharingGrant(ctx, incidents.CreateSharingGrantParams{
		OwnerAccountID: owner.ID,
		IncidentID:     incident.ID,
		StreamID:       stream.ID,
		RecipientType:  incidents.SharingGrantRecipientTrustedContact,
		ContactID:      contactKey.ContactID,
		DataClass:      incidents.SharingGrantDataClassMetadataCiphertext,
	})
	if err != nil {
		t.Fatalf("create sharing grant: %v", err)
	}
	record, err := repo.CreateWrappedKeyRecord(ctx, incidents.CreateWrappedKeyRecordParams{
		OwnerAccountID:           owner.ID,
		IncidentID:               incident.ID,
		StreamID:                 stream.ID,
		GrantID:                  grant.ID,
		MediaKeyID:               "media-key-1",
		WrappingAlgorithm:        "age-v1-x25519",
		WrappingAlgorithmVersion: "1",
		WrappedKeyCiphertext:     "wrapped-ciphertext",
		PublicWrappingMetadata:   []byte(`{"profile":"age-v1-x25519"}`),
	})
	if err != nil {
		t.Fatalf("create wrapped key: %v", err)
	}
	if record.ContactPublicKeyID != contactKey.ID || record.ContactPublicKeyVersion != contactKey.Version {
		t.Fatalf("wrapped key contact binding = %+v, want key %q version %d", record, contactKey.ID, contactKey.Version)
	}
	if _, err := repo.CreateWrappedKeyRecord(ctx, incidents.CreateWrappedKeyRecordParams{
		OwnerAccountID:           owner.ID,
		IncidentID:               incident.ID,
		StreamID:                 stream.ID,
		GrantID:                  grant.ID,
		MediaKeyID:               "media-key-1",
		WrappingAlgorithm:        "age-v1-x25519",
		WrappingAlgorithmVersion: "1",
		WrappedKeyCiphertext:     "wrapped-ciphertext",
		PublicWrappingMetadata:   []byte(`{"profile":"age-v1-x25519"}`),
	}); !errors.Is(err, incidents.ErrDuplicate) {
		t.Fatalf("duplicate wrapped key error = %v, want ErrDuplicate", err)
	}

	records, err := repo.ListWrappedKeyRecords(ctx, owner.ID, incident.ID)
	if err != nil {
		t.Fatalf("list wrapped keys: %v", err)
	}
	if len(records) != 1 || records[0].ID != record.ID || string(records[0].PublicWrappingMetadata) == "" {
		t.Fatalf("unexpected wrapped key list: %+v", records)
	}
	if _, err := repo.GetWrappedKeyRecord(ctx, owner.ID, record.ID); err != nil {
		t.Fatalf("get wrapped key: %v", err)
	}

	if _, err := repo.RevokeSharingGrant(ctx, owner.ID, grant.ID, owner.ID); err != nil {
		t.Fatalf("revoke sharing grant: %v", err)
	}
	if _, err := repo.GetWrappedKeyRecord(ctx, owner.ID, record.ID); !errors.Is(err, incidents.ErrNotFound) {
		t.Fatalf("revoked grant get wrapped key error = %v, want ErrNotFound", err)
	}
	records, err = repo.ListWrappedKeyRecords(ctx, owner.ID, incident.ID)
	if err != nil {
		t.Fatalf("list wrapped keys after grant revoke: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("revoked grant still delivered wrapped keys: %+v", records)
	}
}

func TestIncidentDeletionPrunesSharingAndWrappedKeyMetadata(t *testing.T) {
	ctx := context.Background()
	repo := newRepository(t, ctx)
	owner, err := repo.CreateAccount(ctx, auth.CreateAccountParams{
		Username:     "delete-wrapped-owner",
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
		PublicKey:            "age1delete",
		PublicKeyFingerprint: "fingerprint-delete",
		KeyState:             incidents.ContactKeyStateActive,
	})
	if err != nil {
		t.Fatalf("create contact key: %v", err)
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
	record, err := repo.CreateWrappedKeyRecord(ctx, incidents.CreateWrappedKeyRecordParams{
		OwnerAccountID:           owner.ID,
		IncidentID:               incident.ID,
		GrantID:                  grant.ID,
		MediaKeyID:               "media-key-delete",
		WrappingAlgorithm:        "age-v1-x25519",
		WrappingAlgorithmVersion: "1",
		WrappedKeyCiphertext:     "wrapped-ciphertext",
		PublicWrappingMetadata:   []byte(`{"profile":"age-v1-x25519"}`),
	})
	if err != nil {
		t.Fatalf("create wrapped key: %v", err)
	}

	status, err := repo.RequestIncidentDeletion(ctx, incidents.IncidentDeletionRequest{
		IncidentID:     incident.ID,
		Source:         incidents.IncidentDeletionSourceAccountRequest,
		ActorAccountID: owner.ID,
		AllowOpen:      true,
	})
	if err != nil {
		t.Fatalf("request incident deletion: %v", err)
	}
	if _, err := repo.CompleteIncidentDeletion(ctx, status.DecisionID); err != nil {
		t.Fatalf("complete incident deletion: %v", err)
	}
	if grants, err := repo.ListSharingGrants(ctx, owner.ID, incident.ID); err != nil || len(grants) != 0 {
		t.Fatalf("sharing grants after deletion = %+v, err %v", grants, err)
	}
	if _, err := repo.GetWrappedKeyRecord(ctx, owner.ID, record.ID); !errors.Is(err, incidents.ErrNotFound) {
		t.Fatalf("wrapped key after deletion error = %v, want ErrNotFound", err)
	}
	if _, err := repo.GetContactPublicKey(ctx, owner.ID, contactKey.ID); err != nil {
		t.Fatalf("contact key should remain after incident deletion: %v", err)
	}
}

func TestSQLiteUploadOperationRaceAndBackendParity(t *testing.T) {
	contracttest.RunUploadOperationRaceAndParity(t, func(t *testing.T, ctx context.Context) contracttest.Repository {
		t.Helper()
		return newRepository(t, ctx)
	})
}

func TestCreateChunkRejectsClosedIncident(t *testing.T) {
	ctx := context.Background()
	repo := newRepository(t, ctx)
	incident, err := repo.CreateIncident(ctx, "phone", "")
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	if _, err := repo.CloseIncident(ctx, incident.ID); err != nil {
		t.Fatalf("close incident: %v", err)
	}

	_, err = repo.CreateChunk(ctx, testChunkParams(incident.ID, "", incidents.MediaTypeAudio, 1))

	if !errors.Is(err, incidents.ErrIncidentClosed) {
		t.Fatalf("expected ErrIncidentClosed, got %v", err)
	}
	chunks, err := repo.ListChunks(ctx, incident.ID)
	if err != nil {
		t.Fatalf("list chunks: %v", err)
	}
	if len(chunks) != 0 {
		t.Fatalf("expected no chunks, got %+v", chunks)
	}
}

func TestRequestIncidentDeletionSnapshotsItemsAndBlocksWrites(t *testing.T) {
	ctx := context.Background()
	repo := newRepository(t, ctx)
	incident, err := repo.CreateIncidentForAccount(ctx, "", incidents.CreateIncidentParams{
		ClientLabel: "phone",
		Notes:       "delete me",
	})
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	stream, err := repo.CreateMediaStream(ctx, incident.ID, incidents.MediaTypeAudio, "audio")
	if err != nil {
		t.Fatalf("create media stream: %v", err)
	}
	if _, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, stream.ID, incidents.MediaTypeAudio, 1)); err != nil {
		t.Fatalf("create stream chunk: %v", err)
	}
	if _, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, "", incidents.MediaTypeMetadata, 1)); err != nil {
		t.Fatalf("create metadata chunk: %v", err)
	}

	status, err := repo.RequestIncidentDeletion(ctx, incidents.IncidentDeletionRequest{
		IncidentID:     incident.ID,
		Source:         incidents.IncidentDeletionSourceAccountRequest,
		ReasonCode:     "account_delete",
		ActorAccountID: "acct_owner",
		AllowOpen:      true,
	})
	if err != nil {
		t.Fatalf("request deletion: %v", err)
	}
	if status.State != incidents.IncidentDeletionStatePending || status.ItemCount != 2 {
		t.Fatalf("unexpected deletion status: %+v", status)
	}

	got, err := repo.GetIncident(ctx, incident.ID)
	if err != nil {
		t.Fatalf("get incident: %v", err)
	}
	if got.DeletionState != incidents.IncidentDeletionStatePending {
		t.Fatalf("deletion state = %q, want pending", got.DeletionState)
	}
	items, err := repo.ListIncidentDeletionItems(ctx, status.DecisionID)
	if err != nil {
		t.Fatalf("list deletion items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("deletion item count = %d, want 2", len(items))
	}
	for _, item := range items {
		if item.StoredPath == "" || item.State != incidents.IncidentDeletionItemStatePending {
			t.Fatalf("unexpected deletion item: %+v", item)
		}
	}

	repeated, err := repo.RequestIncidentDeletion(ctx, incidents.IncidentDeletionRequest{
		IncidentID:     incident.ID,
		Source:         incidents.IncidentDeletionSourceAccountRequest,
		ActorAccountID: "acct_owner",
		AllowOpen:      true,
	})
	if err != nil {
		t.Fatalf("repeat deletion request: %v", err)
	}
	if repeated.DecisionID != status.DecisionID || repeated.ItemCount != status.ItemCount {
		t.Fatalf("repeat request returned %+v, want existing %+v", repeated, status)
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
	if _, err := repo.CompleteMediaStream(ctx, incident.ID, stream.ID, 1); !errors.Is(err, incidents.ErrIncidentDeleting) {
		t.Fatalf("complete stream during deletion error = %v, want ErrIncidentDeleting", err)
	}
	if _, err := repo.FailMediaStream(ctx, incident.ID, stream.ID, "late failure"); !errors.Is(err, incidents.ErrIncidentDeleting) {
		t.Fatalf("fail stream during deletion error = %v, want ErrIncidentDeleting", err)
	}
	if _, err := repo.CloseIncident(ctx, incident.ID); !errors.Is(err, incidents.ErrIncidentDeleting) {
		t.Fatalf("close incident during deletion error = %v, want ErrIncidentDeleting", err)
	}
	if _, err := repo.ReserveUploadOperation(ctx, testUploadOperationParams(incident.ID, stream.ID)); !errors.Is(err, incidents.ErrIncidentDeleting) {
		t.Fatalf("reserve upload operation during deletion error = %v, want ErrIncidentDeleting", err)
	}
	if _, _, err := repo.CreateIncidentToken(ctx, incident.ID, "trusted contact", nil); !errors.Is(err, incidents.ErrNotFound) {
		t.Fatalf("create token during deletion error = %v, want ErrNotFound", err)
	}
}

func TestRequestIncidentDeletionRejectsOpenIncidentWithoutAllowOpen(t *testing.T) {
	ctx := context.Background()
	repo := newRepository(t, ctx)
	incident, err := repo.CreateIncident(ctx, "phone", "")
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}

	_, err = repo.RequestIncidentDeletion(ctx, incidents.IncidentDeletionRequest{
		IncidentID: incident.ID,
		Source:     incidents.IncidentDeletionSourceAccountRequest,
	})
	if !errors.Is(err, incidents.ErrInvalidState) {
		t.Fatalf("open deletion error = %v, want ErrInvalidState", err)
	}
}

func TestQueueRetentionIncidentDeletionsSelectsClosedIncidentsOnly(t *testing.T) {
	ctx := context.Background()
	repo := newRepository(t, ctx)
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

	queued, err := repo.QueueRetentionIncidentDeletions(ctx, time.Now().UTC().Add(time.Minute), 10)
	if err != nil {
		t.Fatalf("queue retention deletions: %v", err)
	}
	if queued != 1 {
		t.Fatalf("queued retention deletions = %d, want 1", queued)
	}
	status, err := repo.GetIncidentDeletionStatus(ctx, closedIncident.ID)
	if err != nil {
		t.Fatalf("get closed incident deletion: %v", err)
	}
	if status.Source != incidents.IncidentDeletionSourceRetentionPolicy || status.ReasonCode != "closed_incident_retention" {
		t.Fatalf("unexpected retention deletion status: %+v", status)
	}
	if _, err := repo.GetIncidentDeletionStatus(ctx, openIncident.ID); !errors.Is(err, incidents.ErrNotFound) {
		t.Fatalf("open incident deletion status error = %v, want ErrNotFound", err)
	}
}

func TestListRetentionDeletionCandidatesPreviewsClosedActiveOnly(t *testing.T) {
	ctx := context.Background()
	repo := newRepository(t, ctx)
	openIncident, err := repo.CreateIncident(ctx, "open", "")
	if err != nil {
		t.Fatalf("create open incident: %v", err)
	}
	closedIncident, err := repo.CreateIncident(ctx, "closed", "")
	if err != nil {
		t.Fatalf("create closed incident: %v", err)
	}
	deletingIncident, err := repo.CreateIncident(ctx, "deleting", "")
	if err != nil {
		t.Fatalf("create deleting incident: %v", err)
	}
	if _, err := repo.CloseIncident(ctx, closedIncident.ID); err != nil {
		t.Fatalf("close candidate incident: %v", err)
	}
	if _, err := repo.CloseIncident(ctx, deletingIncident.ID); err != nil {
		t.Fatalf("close deleting incident: %v", err)
	}
	if _, err := repo.RequestIncidentDeletion(ctx, incidents.IncidentDeletionRequest{
		IncidentID: deletingIncident.ID,
		Source:     incidents.IncidentDeletionSourceAdminRequest,
	}); err != nil {
		t.Fatalf("request deletion: %v", err)
	}

	candidates, err := repo.ListRetentionDeletionCandidates(ctx, time.Now().UTC().Add(time.Minute), 10)
	if err != nil {
		t.Fatalf("list retention candidates: %v", err)
	}
	if len(candidates) != 1 || candidates[0].IncidentID != closedIncident.ID {
		t.Fatalf("retention candidates = %+v, want only %s", candidates, closedIncident.ID)
	}
	for _, candidate := range candidates {
		if candidate.IncidentID == openIncident.ID || candidate.IncidentID == deletingIncident.ID {
			t.Fatalf("candidate included ineligible incident: %+v", candidate)
		}
	}
}

func TestIncidentDeletionJobStatusSummarizesSafeRetryCategories(t *testing.T) {
	ctx := context.Background()
	repo := newRepository(t, ctx)
	incident, err := repo.CreateIncident(ctx, "phone", "")
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	if _, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, "", incidents.MediaTypeAudio, 1)); err != nil {
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

	report, err := repo.GetIncidentDeletionJobStatus(ctx, 10, time.Now().UTC().Add(time.Minute))
	if err != nil {
		t.Fatalf("get deletion job status: %v", err)
	}
	assertDecisionStateCount(t, report.DecisionStateCounts, incidents.IncidentDeletionStateFailed, 1)
	assertDecisionErrorCount(t, report.DecisionErrorCounts, incidents.IncidentDeletionStateFailed, "blob_delete_failed", 1)
	assertItemStateCount(t, report.ItemStateCounts, incidents.IncidentDeletionItemStateFailed, "unsafe_stored_path", 1)
	if len(report.RunnableJobs) != 1 ||
		report.RunnableJobs[0].DecisionID != status.DecisionID ||
		report.RunnableJobs[0].IncidentID != incident.ID ||
		report.RunnableJobs[0].ErrorCode != "blob_delete_failed" {
		t.Fatalf("unexpected runnable jobs: %+v", report.RunnableJobs)
	}
}

func TestPruneIncidentTokenMetadataRemovesOnlyExpiredOrRevokedTokens(t *testing.T) {
	ctx := context.Background()
	repo := newRepository(t, ctx)
	incident, err := repo.CreateIncident(ctx, "phone", "")
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	if _, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, "", incidents.MediaTypeAudio, 1)); err != nil {
		t.Fatalf("create chunk: %v", err)
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
	if _, err := repo.GetIncidentToken(ctx, revokedToken.ID); err != nil {
		t.Fatalf("recent revoked token was pruned: %v", err)
	}

	pruned, err = repo.PruneIncidentTokenMetadata(ctx, time.Now().UTC().Add(time.Minute), 25)
	if err != nil {
		t.Fatalf("prune revoked token metadata: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("revoked token metadata pruned = %d, want 1", pruned)
	}
	if _, err := repo.GetIncidentToken(ctx, revokedToken.ID); !errors.Is(err, incidents.ErrNotFound) {
		t.Fatalf("revoked token lookup error = %v, want ErrNotFound", err)
	}
	if _, err := repo.GetIncident(ctx, incident.ID); err != nil {
		t.Fatalf("incident was pruned by token metadata pruning: %v", err)
	}
	chunks, err := repo.ListChunks(ctx, incident.ID)
	if err != nil {
		t.Fatalf("list chunks after token pruning: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("chunks after token pruning = %+v, want original chunk", chunks)
	}
}

func TestPruneIncidentDeletionTombstonesRemovesOnlyCompletedTombstones(t *testing.T) {
	ctx := context.Background()
	repo := newRepository(t, ctx)
	activeIncident, err := repo.CreateIncident(ctx, "active", "")
	if err != nil {
		t.Fatalf("create active incident: %v", err)
	}
	deletedIncident, err := repo.CreateIncident(ctx, "deleted", "sensitive note")
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

	pruned, err := repo.PruneIncidentDeletionTombstones(ctx, time.Now().UTC().Add(-time.Hour), 25)
	if err != nil {
		t.Fatalf("prune recent tombstone: %v", err)
	}
	if pruned != 0 {
		t.Fatalf("recent tombstones pruned = %d, want 0", pruned)
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
	if _, err := repo.GetIncidentDeletionStatus(ctx, deletedIncident.ID); !errors.Is(err, incidents.ErrNotFound) {
		t.Fatalf("deleted tombstone status error = %v, want ErrNotFound", err)
	}
	if _, err := repo.GetIncident(ctx, activeIncident.ID); err != nil {
		t.Fatalf("active incident was pruned: %v", err)
	}
}

func TestCreateChunkRejectsCompletedStream(t *testing.T) {
	ctx := context.Background()
	repo := newRepository(t, ctx)
	incident, err := repo.CreateIncident(ctx, "phone", "")
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	stream, err := repo.CreateMediaStream(ctx, incident.ID, incidents.MediaTypeAudio, "audio")
	if err != nil {
		t.Fatalf("create media stream: %v", err)
	}
	if _, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, stream.ID, incidents.MediaTypeAudio, 1)); err != nil {
		t.Fatalf("create stream chunk: %v", err)
	}
	if _, err := repo.CompleteMediaStream(ctx, incident.ID, stream.ID, 1); err != nil {
		t.Fatalf("complete media stream: %v", err)
	}

	_, err = repo.CreateChunk(ctx, testChunkParams(incident.ID, stream.ID, incidents.MediaTypeAudio, 2))

	if !errors.Is(err, incidents.ErrInvalidState) {
		t.Fatalf("expected ErrInvalidState, got %v", err)
	}
	chunks, err := repo.ListStreamChunks(ctx, incident.ID, stream.ID)
	if err != nil {
		t.Fatalf("list stream chunks: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected only the original stream chunk, got %+v", chunks)
	}
}

func TestCreateChunkUsesStreamScopedDuplicateIdentity(t *testing.T) {
	ctx := context.Background()
	repo := newRepository(t, ctx)
	incident, err := repo.CreateIncident(ctx, "phone", "")
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	firstStream, err := repo.CreateMediaStream(ctx, incident.ID, incidents.MediaTypeAudio, "first audio")
	if err != nil {
		t.Fatalf("create first media stream: %v", err)
	}
	secondStream, err := repo.CreateMediaStream(ctx, incident.ID, incidents.MediaTypeAudio, "second audio")
	if err != nil {
		t.Fatalf("create second media stream: %v", err)
	}

	if _, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, firstStream.ID, incidents.MediaTypeAudio, 1)); err != nil {
		t.Fatalf("create first stream chunk: %v", err)
	}
	if _, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, secondStream.ID, incidents.MediaTypeAudio, 1)); err != nil {
		t.Fatalf("create second stream chunk with same index: %v", err)
	}
	if _, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, "", incidents.MediaTypeAudio, 1)); err != nil {
		t.Fatalf("create legacy chunk with same media/index: %v", err)
	}

	_, err = repo.CreateChunk(ctx, testChunkParams(incident.ID, firstStream.ID, incidents.MediaTypeAudio, 1))
	if !errors.Is(err, incidents.ErrDuplicate) {
		t.Fatalf("expected duplicate stream chunk to return ErrDuplicate, got %v", err)
	}
}

func TestGetChunkByKeyReturnsLegacyUnstreamedChunk(t *testing.T) {
	ctx := context.Background()
	repo := newRepository(t, ctx)
	incident, err := repo.CreateIncident(ctx, "phone", "")
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	stream, err := repo.CreateMediaStream(ctx, incident.ID, incidents.MediaTypeAudio, "audio")
	if err != nil {
		t.Fatalf("create media stream: %v", err)
	}
	if _, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, stream.ID, incidents.MediaTypeAudio, 1)); err != nil {
		t.Fatalf("create stream chunk: %v", err)
	}
	legacy, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, "", incidents.MediaTypeAudio, 1))
	if err != nil {
		t.Fatalf("create legacy chunk: %v", err)
	}

	got, err := repo.GetChunkByKey(ctx, incident.ID, incidents.MediaTypeAudio, 1)
	if err != nil {
		t.Fatalf("get legacy chunk by key: %v", err)
	}
	if got.ID != legacy.ID || got.StreamID != "" {
		t.Fatalf("expected legacy chunk %+v, got %+v", legacy, got)
	}
}

func TestUploadOperationReservationCompletionAndConflict(t *testing.T) {
	ctx := context.Background()
	repo := newRepository(t, ctx)
	incident, err := repo.CreateIncident(ctx, "phone", "")
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	stream, err := repo.CreateMediaStream(ctx, incident.ID, incidents.MediaTypeAudio, "audio")
	if err != nil {
		t.Fatalf("create media stream: %v", err)
	}
	params := testUploadOperationParams(incident.ID, stream.ID)

	reserved, err := repo.ReserveUploadOperation(ctx, params)
	if err != nil {
		t.Fatalf("reserve upload operation: %v", err)
	}
	if reserved.State != incidents.UploadOperationStateReserved {
		t.Fatalf("state = %q, want reserved", reserved.State)
	}
	same, err := repo.ReserveUploadOperation(ctx, params)
	if err != nil {
		t.Fatalf("reserve same upload operation: %v", err)
	}
	if same.ID != reserved.ID {
		t.Fatalf("same idempotency key created a new operation: first=%q second=%q", reserved.ID, same.ID)
	}

	conflicting := params
	conflicting.OriginalFilename = "other.enc"
	conflicting.FingerprintHash = strings.Repeat("c", 64)
	if _, err := repo.ReserveUploadOperation(ctx, conflicting); !errors.Is(err, incidents.ErrIdempotencyConflict) {
		t.Fatalf("conflicting reservation error = %v, want ErrIdempotencyConflict", err)
	}

	chunk, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, stream.ID, incidents.MediaTypeAudio, 1))
	if err != nil {
		t.Fatalf("create chunk: %v", err)
	}
	completed, err := repo.CompleteUploadOperation(ctx, params, chunk)
	if err != nil {
		t.Fatalf("complete upload operation: %v", err)
	}
	if completed.State != incidents.UploadOperationStateMetadataCommitted || completed.ChunkID != chunk.ID {
		t.Fatalf("unexpected completed operation: %+v", completed)
	}
	replayed, err := repo.ReserveUploadOperation(ctx, params)
	if err != nil {
		t.Fatalf("reserve completed operation: %v", err)
	}
	if replayed.State != incidents.UploadOperationStateMetadataCommitted || replayed.ChunkID != chunk.ID {
		t.Fatalf("expected completed operation replay, got %+v", replayed)
	}
}

func TestCompleteMediaStreamRejectsUnexpectedChunkRows(t *testing.T) {
	ctx := context.Background()
	repo := newRepository(t, ctx)
	incident, err := repo.CreateIncident(ctx, "phone", "")
	if err != nil {
		t.Fatalf("create incident: %v", err)
	}
	stream, err := repo.CreateMediaStream(ctx, incident.ID, incidents.MediaTypeAudio, "audio")
	if err != nil {
		t.Fatalf("create media stream: %v", err)
	}
	if _, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, stream.ID, incidents.MediaTypeAudio, 1)); err != nil {
		t.Fatalf("create first stream chunk: %v", err)
	}
	if _, err := repo.CreateChunk(ctx, testChunkParams(incident.ID, stream.ID, incidents.MediaTypeAudio, 2)); err != nil {
		t.Fatalf("create second stream chunk: %v", err)
	}

	_, err = repo.CompleteMediaStream(ctx, incident.ID, stream.ID, 1)

	if !errors.Is(err, incidents.ErrInvalidState) {
		t.Fatalf("expected ErrInvalidState, got %v", err)
	}
	stream, err = repo.GetMediaStream(ctx, incident.ID, stream.ID)
	if err != nil {
		t.Fatalf("get media stream: %v", err)
	}
	if stream.Status != incidents.StreamStatusOpen {
		t.Fatalf("expected stream to remain open, got %+v", stream)
	}
}

func newRepository(t *testing.T, ctx context.Context) *incidents.Repository {
	t.Helper()

	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "safety.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})
	return incidents.NewRepository(conn)
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

func assertDecisionStateCount(t *testing.T, counts []incidents.IncidentDeletionStateCount, state string, want int) {
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

func assertDecisionErrorCount(t *testing.T, counts []incidents.IncidentDeletionErrorCount, state, errorCode string, want int) {
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

func assertItemStateCount(t *testing.T, counts []incidents.IncidentDeletionItemStateCount, state, errorCode string, want int) {
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
