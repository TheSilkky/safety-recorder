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
	ListIncidentsForAccount(ctx context.Context, accountID string) ([]incidents.Incident, error)
	ListLegacyUnownedIncidentCandidates(ctx context.Context, limit int) ([]incidents.LegacyUnownedIncidentCandidate, error)
	ReassignLegacyUnownedIncident(ctx context.Context, params incidents.LegacyIncidentReassignmentParams) (incidents.LegacyIncidentReassignmentEvent, error)
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
	AccountCommittedBlobBytes(ctx context.Context, ownerAccountID string) (int64, error)
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
	ListIncidentTokens(ctx context.Context, incidentID string) ([]incidents.IncidentToken, error)
	GetIncidentTokenForIncident(ctx context.Context, incidentID, tokenID string) (incidents.IncidentToken, error)
	LookupIncidentToken(ctx context.Context, rawToken string) (incidents.IncidentToken, error)
	RevokeIncidentToken(ctx context.Context, tokenID string) error

	HasAccounts(ctx context.Context) (bool, error)
	HasAdminAccount(ctx context.Context) (bool, error)
	CreateAccount(ctx context.Context, params auth.CreateAccountParams) (auth.Account, error)
	GetAccountByUsername(ctx context.Context, username string) (auth.Account, error)
	GetAccountByID(ctx context.Context, accountID string) (auth.Account, error)
	ListAccounts(ctx context.Context) ([]auth.Account, error)
	UpdateAccountPassword(ctx context.Context, accountID, passwordHash string) (auth.Account, error)
	ResetAccountSecondFactorRecovery(ctx context.Context, params auth.ResetAccountSecondFactorRecoveryParams) (auth.AccountRecoveryEvent, auth.Account, error)
	CreateAccountVerificationToken(ctx context.Context, params auth.CreateAccountVerificationTokenParams) (auth.AccountVerificationToken, string, error)
	ConsumeAccountVerificationToken(ctx context.Context, rawToken, purpose string, now time.Time) (auth.Account, error)
	CreateEmailSecondFactorChallenge(ctx context.Context, params auth.CreateEmailSecondFactorChallengeParams) (auth.SecondFactorChallenge, string, error)
	ConsumeEmailSecondFactorChallenge(ctx context.Context, accountID, rawToken string, now time.Time) (auth.SecondFactor, auth.Account, error)
	CreateTOTPSecondFactorEnrollment(ctx context.Context, params auth.CreateTOTPSecondFactorEnrollmentParams) (auth.SecondFactor, error)
	GetPendingTOTPSecondFactor(ctx context.Context, accountID string) (auth.SecondFactor, error)
	GetActiveTOTPSecondFactor(ctx context.Context, accountID string) (auth.SecondFactor, error)
	ActivateTOTPSecondFactor(ctx context.Context, accountID, factorID string, verifiedAt time.Time, lastUsedTimeStep int64) (auth.SecondFactor, auth.Account, error)
	MarkTOTPSecondFactorUsed(ctx context.Context, factorID string, verifiedAt time.Time, lastUsedTimeStep int64) (auth.SecondFactor, error)
	GetOrCreateWebAuthnUser(ctx context.Context, accountID, rpID string) (auth.WebAuthnUser, error)
	ListWebAuthnCredentials(ctx context.Context, accountID, rpID string) ([]auth.WebAuthnCredential, error)
	HasActiveWebAuthnCredential(ctx context.Context, accountID string) (bool, error)
	CreateWebAuthnChallenge(ctx context.Context, params auth.CreateWebAuthnChallengeParams) (auth.WebAuthnChallenge, error)
	ConsumeWebAuthnChallenge(ctx context.Context, accountID, sessionID, rpID, challengeType string, now time.Time) (auth.WebAuthnChallenge, error)
	CreateWebAuthnCredential(ctx context.Context, params auth.CreateWebAuthnCredentialParams) (auth.WebAuthnCredential, auth.Account, error)
	UpdateWebAuthnCredentialAfterAssertion(ctx context.Context, params auth.UpdateWebAuthnCredentialParams) (auth.WebAuthnCredential, error)
	CreateSession(ctx context.Context, accountID string, expiresAt time.Time) (auth.Session, string, error)
	LookupSession(ctx context.Context, rawToken string) (auth.Session, error)
	MarkSessionSecondFactorVerified(ctx context.Context, sessionID, factorID, method string, verifiedAt time.Time) (auth.Session, error)
	RevokeSession(ctx context.Context, sessionID string) error
	RevokeAccountSessions(ctx context.Context, accountID, exceptSessionID string) (int64, error)

	CreateContactPublicKey(ctx context.Context, params incidents.CreateContactPublicKeyParams) (incidents.ContactPublicKey, error)
	ListContactPublicKeys(ctx context.Context, ownerAccountID string) ([]incidents.ContactPublicKey, error)
	GetContactPublicKey(ctx context.Context, ownerAccountID, publicKeyID string) (incidents.ContactPublicKey, error)
	UpdateContactPublicKey(ctx context.Context, params incidents.UpdateContactPublicKeyParams) (incidents.ContactPublicKey, error)
	RevokeContactPublicKey(ctx context.Context, ownerAccountID, publicKeyID string) (incidents.ContactPublicKey, error)
	MarkContactPublicKeyLost(ctx context.Context, ownerAccountID, publicKeyID string) (incidents.ContactPublicKey, error)
	ReplaceContactPublicKey(ctx context.Context, params incidents.ReplaceContactPublicKeyParams) (incidents.ContactPublicKey, error)
	CreateAccountRecipientKey(ctx context.Context, params incidents.CreateAccountRecipientKeyParams) (incidents.AccountRecipientKey, error)
	ListAccountRecipientKeys(ctx context.Context, ownerAccountID string) ([]incidents.AccountRecipientKey, error)
	GetAccountRecipientKey(ctx context.Context, ownerAccountID, recipientKeyID string) (incidents.AccountRecipientKey, error)
	GetActiveAccountRecipientKey(ctx context.Context, ownerAccountID, recipientKeyID string) (incidents.AccountRecipientKey, error)
	UpdateAccountRecipientKey(ctx context.Context, params incidents.UpdateAccountRecipientKeyParams) (incidents.AccountRecipientKey, error)
	RevokeAccountRecipientKey(ctx context.Context, ownerAccountID, recipientKeyID string) (incidents.AccountRecipientKey, error)
	MarkAccountRecipientKeyLost(ctx context.Context, ownerAccountID, recipientKeyID string) (incidents.AccountRecipientKey, error)
	ReplaceAccountRecipientKey(ctx context.Context, params incidents.ReplaceAccountRecipientKeyParams) (incidents.AccountRecipientKey, error)
	CreateTrustedContactRelationship(ctx context.Context, params incidents.CreateTrustedContactRelationshipParams) (incidents.TrustedContactRelationship, error)
	ListTrustedContactRelationshipsForAccount(ctx context.Context, accountID string) ([]incidents.TrustedContactRelationship, error)
	GetTrustedContactRelationshipForAccount(ctx context.Context, accountID, relationshipID string) (incidents.TrustedContactRelationship, error)
	AcceptTrustedContactRelationship(ctx context.Context, recipientAccountID, relationshipID string) (incidents.TrustedContactRelationship, error)
	DeclineTrustedContactRelationship(ctx context.Context, recipientAccountID, relationshipID string) (incidents.TrustedContactRelationship, error)
	RevokeTrustedContactRelationship(ctx context.Context, ownerAccountID, relationshipID, revokedByAccountID string) (incidents.TrustedContactRelationship, error)
	ReplaceTrustedContactRelationship(ctx context.Context, params incidents.ReplaceTrustedContactRelationshipParams) (incidents.TrustedContactRelationship, error)
	CreateSharingGrant(ctx context.Context, params incidents.CreateSharingGrantParams) (incidents.SharingGrant, error)
	ListSharingGrants(ctx context.Context, ownerAccountID, incidentID string) ([]incidents.SharingGrant, error)
	GetSharingGrant(ctx context.Context, ownerAccountID, grantID string) (incidents.SharingGrant, error)
	RevokeSharingGrant(ctx context.Context, ownerAccountID, grantID, revokedByAccountID string) (incidents.SharingGrant, error)
	CreateWrappedKeyRecord(ctx context.Context, params incidents.CreateWrappedKeyRecordParams) (incidents.WrappedKeyRecord, error)
	ListWrappedKeyRecords(ctx context.Context, ownerAccountID, incidentID string) ([]incidents.WrappedKeyRecord, error)
	GetWrappedKeyRecord(ctx context.Context, ownerAccountID, wrappedKeyID string) (incidents.WrappedKeyRecord, error)
	ListTrustedContactWrappedKeyRecords(ctx context.Context, recipientAccountID, incidentID string) ([]incidents.WrappedKeyRecord, error)
	GetTrustedContactWrappedKeyRecord(ctx context.Context, recipientAccountID, wrappedKeyID string) (incidents.WrappedKeyRecord, error)
	RevokeWrappedKeyRecord(ctx context.Context, ownerAccountID, wrappedKeyID, revokedByAccountID string) (incidents.WrappedKeyRecord, error)
}

var _ MetadataRepository = (*incidents.Repository)(nil)
