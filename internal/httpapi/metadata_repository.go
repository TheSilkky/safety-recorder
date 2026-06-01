package httpapi

import (
	"context"
	"time"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/incidents"
)

// MetadataRepository is the incident metadata boundary required by the HTTP
// handlers. SQLite remains the default implementation, and optional PostgreSQL
// support must preserve token hashing, duplicate guards, state checks, and
// stream completion validation.
type MetadataRepository interface {
	Check(ctx context.Context) error

	CreateIncidentForAccount(ctx context.Context, accountID string, params incidents.CreateIncidentParams) (incidents.Incident, error)
	GetIncident(ctx context.Context, id string) (incidents.Incident, error)
	GetIncidentDetail(ctx context.Context, id string) (incidents.IncidentDetail, error)
	CloseIncident(ctx context.Context, id string) (incidents.Incident, error)
	RequestIncidentDeletion(ctx context.Context, params incidents.IncidentDeletionRequest) (incidents.IncidentDeletionStatus, error)
	GetIncidentDeletionStatus(ctx context.Context, incidentID string) (incidents.IncidentDeletionStatus, error)
	QueueRetentionIncidentDeletions(ctx context.Context, cutoff time.Time, limit int) (int, error)
	PruneIncidentTokenMetadata(ctx context.Context, cutoff time.Time, limit int) (int, error)
	PruneIncidentDeletionTombstones(ctx context.Context, cutoff time.Time, limit int) (int, error)
	ListRunnableIncidentDeletions(ctx context.Context, limit int, staleDeletingBefore time.Time) ([]incidents.IncidentDeletionStatus, error)
	MarkIncidentDeletionDeleting(ctx context.Context, decisionID string, staleDeletingBefore time.Time) (incidents.IncidentDeletionStatus, error)
	ListIncidentDeletionItems(ctx context.Context, decisionID string) ([]incidents.IncidentDeletionItem, error)
	MarkIncidentDeletionItemDeleted(ctx context.Context, itemID string) error
	MarkIncidentDeletionItemFailed(ctx context.Context, itemID, errorCode string) error
	CompleteIncidentDeletion(ctx context.Context, decisionID string) (incidents.IncidentDeletionStatus, error)
	FailIncidentDeletion(ctx context.Context, decisionID, errorCode string) (incidents.IncidentDeletionStatus, error)

	CreateCheckin(ctx context.Context, incidentID string, params incidents.CreateCheckinParams) (incidents.Checkin, error)

	ChunkExists(ctx context.Context, incidentID, streamID, mediaType string, chunkIndex int) (bool, error)
	GetChunkByIdentity(ctx context.Context, incidentID, streamID, mediaType string, chunkIndex int) (incidents.Chunk, error)
	CreateChunk(ctx context.Context, params incidents.CreateChunkParams) (incidents.Chunk, error)
	ListChunks(ctx context.Context, incidentID string) ([]incidents.Chunk, error)
	GetChunkByKey(ctx context.Context, incidentID, mediaType string, chunkIndex int) (incidents.Chunk, error)
	ReserveUploadOperation(ctx context.Context, params incidents.UploadOperationParams) (incidents.UploadOperation, error)
	CompleteUploadOperation(ctx context.Context, params incidents.UploadOperationParams, chunk incidents.Chunk) (incidents.UploadOperation, error)

	CreateMediaStream(ctx context.Context, incidentID, mediaType, label string) (incidents.MediaStream, error)
	ListMediaStreams(ctx context.Context, incidentID string) ([]incidents.MediaStream, error)
	ListCompletedMediaStreams(ctx context.Context, incidentID string) ([]incidents.MediaStream, error)
	GetMediaStream(ctx context.Context, incidentID, streamID string) (incidents.MediaStream, error)
	ListStreamChunks(ctx context.Context, incidentID, streamID string) ([]incidents.Chunk, error)
	CompleteMediaStream(ctx context.Context, incidentID, streamID string, expectedChunkCount int) (incidents.MediaStream, error)
	FailMediaStream(ctx context.Context, incidentID, streamID, reason string) (incidents.MediaStream, error)

	CreateIncidentToken(ctx context.Context, incidentID, label string, expiresAt *time.Time) (incidents.IncidentToken, string, error)
	GetIncidentToken(ctx context.Context, tokenID string) (incidents.IncidentToken, error)
	LookupIncidentToken(ctx context.Context, rawToken string) (incidents.IncidentToken, error)
	RevokeIncidentToken(ctx context.Context, tokenID string) error

	HasAccounts(ctx context.Context) (bool, error)
	HasAdminAccount(ctx context.Context) (bool, error)
	CreateAccount(ctx context.Context, params auth.CreateAccountParams) (auth.Account, error)
	GetAccountByUsername(ctx context.Context, username string) (auth.Account, error)
	GetAccountByID(ctx context.Context, accountID string) (auth.Account, error)
	ListAccounts(ctx context.Context) ([]auth.Account, error)
	UpdateAccountPassword(ctx context.Context, accountID, passwordHash string) (auth.Account, error)
	CreateSession(ctx context.Context, accountID string, expiresAt time.Time) (auth.Session, string, error)
	LookupSession(ctx context.Context, rawToken string) (auth.Session, error)
	RevokeSession(ctx context.Context, sessionID string) error
	RevokeAccountSessions(ctx context.Context, accountID, exceptSessionID string) (int64, error)

	CreateContactPublicKey(ctx context.Context, params incidents.CreateContactPublicKeyParams) (incidents.ContactPublicKey, error)
	ListContactPublicKeys(ctx context.Context, ownerAccountID string) ([]incidents.ContactPublicKey, error)
	GetContactPublicKey(ctx context.Context, ownerAccountID, publicKeyID string) (incidents.ContactPublicKey, error)
	UpdateContactPublicKey(ctx context.Context, params incidents.UpdateContactPublicKeyParams) (incidents.ContactPublicKey, error)
	RevokeContactPublicKey(ctx context.Context, ownerAccountID, publicKeyID string) (incidents.ContactPublicKey, error)
	CreateSharingGrant(ctx context.Context, params incidents.CreateSharingGrantParams) (incidents.SharingGrant, error)
	ListSharingGrants(ctx context.Context, ownerAccountID, incidentID string) ([]incidents.SharingGrant, error)
	GetSharingGrant(ctx context.Context, ownerAccountID, grantID string) (incidents.SharingGrant, error)
	RevokeSharingGrant(ctx context.Context, ownerAccountID, grantID, revokedByAccountID string) (incidents.SharingGrant, error)
	CreateWrappedKeyRecord(ctx context.Context, params incidents.CreateWrappedKeyRecordParams) (incidents.WrappedKeyRecord, error)
	ListWrappedKeyRecords(ctx context.Context, ownerAccountID, incidentID string) ([]incidents.WrappedKeyRecord, error)
	GetWrappedKeyRecord(ctx context.Context, ownerAccountID, wrappedKeyID string) (incidents.WrappedKeyRecord, error)
	RevokeWrappedKeyRecord(ctx context.Context, ownerAccountID, wrappedKeyID, revokedByAccountID string) (incidents.WrappedKeyRecord, error)
}

var _ MetadataRepository = (*incidents.Repository)(nil)
