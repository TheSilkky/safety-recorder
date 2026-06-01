package postgresdb

import (
	"database/sql"
	"time"

	"github.com/open-proofline/server/internal/incidents"
)

type scanner interface {
	Scan(dest ...any) error
}

func scanIncident(s scanner) (incidents.Incident, error) {
	var incident incidents.Incident
	var ownerAccountID sql.NullString
	var clientLabel sql.NullString
	var notes sql.NullString
	var incidentMode sql.NullString
	var captureProfile sql.NullString
	var escalationPolicy sql.NullString
	var sharingState sql.NullString
	if err := s.Scan(
		&incident.ID,
		&ownerAccountID,
		&incident.CreatedAt,
		&incident.UpdatedAt,
		&incident.Status,
		&clientLabel,
		&notes,
		&incidentMode,
		&captureProfile,
		&escalationPolicy,
		&sharingState,
		&incident.DeletionState,
	); err != nil {
		return incidents.Incident{}, err
	}
	incident.CreatedAt = incident.CreatedAt.UTC()
	incident.UpdatedAt = incident.UpdatedAt.UTC()
	if ownerAccountID.Valid {
		incident.OwnerAccountID = ownerAccountID.String
	}
	if clientLabel.Valid {
		incident.ClientLabel = clientLabel.String
	}
	if notes.Valid {
		incident.Notes = notes.String
	}
	if incidentMode.Valid {
		incident.IncidentMode = incidentMode.String
	}
	if captureProfile.Valid {
		incident.CaptureProfile = captureProfile.String
	}
	if escalationPolicy.Valid {
		incident.EscalationPolicy = escalationPolicy.String
	}
	if sharingState.Valid {
		incident.SharingState = sharingState.String
	}
	if incident.DeletionState == "" {
		incident.DeletionState = incidents.IncidentDeletionStateActive
	}
	return incident, nil
}

func scanChunk(s scanner) (incidents.Chunk, error) {
	var chunk incidents.Chunk
	var streamID sql.NullString
	var originalFilename sql.NullString
	if err := s.Scan(
		&chunk.ID,
		&chunk.IncidentID,
		&streamID,
		&chunk.ChunkIndex,
		&chunk.MediaType,
		&chunk.StartedAt,
		&chunk.EndedAt,
		&originalFilename,
		&chunk.StoredPath,
		&chunk.ByteSize,
		&chunk.SHA256Hex,
		&chunk.CreatedAt,
	); err != nil {
		return incidents.Chunk{}, err
	}
	chunk.StartedAt = chunk.StartedAt.UTC()
	chunk.EndedAt = chunk.EndedAt.UTC()
	chunk.CreatedAt = chunk.CreatedAt.UTC()
	if streamID.Valid {
		chunk.StreamID = streamID.String
	}
	if originalFilename.Valid {
		chunk.OriginalFilename = originalFilename.String
	}
	return chunk, nil
}

func scanUploadOperation(s scanner) (incidents.UploadOperation, error) {
	var operation incidents.UploadOperation
	var streamID sql.NullString
	var originalFilename sql.NullString
	var chunkID sql.NullString
	var storedPath sql.NullString
	if err := s.Scan(
		&operation.ID,
		&operation.Operation,
		&operation.IdempotencyKeyHash,
		&operation.IncidentID,
		&streamID,
		&operation.ChunkIndex,
		&operation.MediaType,
		&operation.StartedAt,
		&operation.EndedAt,
		&originalFilename,
		&operation.ByteSize,
		&operation.SHA256Hex,
		&operation.FingerprintHash,
		&operation.State,
		&chunkID,
		&storedPath,
		&operation.CreatedAt,
		&operation.UpdatedAt,
	); err != nil {
		return incidents.UploadOperation{}, err
	}
	operation.StartedAt = operation.StartedAt.UTC()
	operation.EndedAt = operation.EndedAt.UTC()
	operation.CreatedAt = operation.CreatedAt.UTC()
	operation.UpdatedAt = operation.UpdatedAt.UTC()
	if streamID.Valid {
		operation.StreamID = streamID.String
	}
	if originalFilename.Valid {
		operation.OriginalFilename = originalFilename.String
	}
	if chunkID.Valid {
		operation.ChunkID = chunkID.String
	}
	if storedPath.Valid {
		operation.StoredPath = storedPath.String
	}
	return operation, nil
}

func scanMediaStream(s scanner) (incidents.MediaStream, error) {
	var stream incidents.MediaStream
	var label sql.NullString
	var expectedChunkCount sql.NullInt64
	var completedAt sql.NullTime
	var failedAt sql.NullTime
	var failureReason sql.NullString
	if err := s.Scan(
		&stream.ID,
		&stream.IncidentID,
		&stream.MediaType,
		&label,
		&stream.Status,
		&expectedChunkCount,
		&stream.CreatedAt,
		&stream.UpdatedAt,
		&completedAt,
		&failedAt,
		&failureReason,
	); err != nil {
		return incidents.MediaStream{}, err
	}
	stream.CreatedAt = stream.CreatedAt.UTC()
	stream.UpdatedAt = stream.UpdatedAt.UTC()
	if label.Valid {
		stream.Label = label.String
	}
	if expectedChunkCount.Valid {
		count := int(expectedChunkCount.Int64)
		stream.ExpectedChunkCount = &count
	}
	stream.CompletedAt = nullableDBTime(completedAt)
	stream.FailedAt = nullableDBTime(failedAt)
	if failureReason.Valid {
		stream.FailureReason = failureReason.String
	}
	return stream, nil
}

func scanCheckin(s scanner) (incidents.Checkin, error) {
	var checkin incidents.Checkin
	var battery sql.NullInt64
	var network sql.NullString
	var latitude sql.NullFloat64
	var longitude sql.NullFloat64
	var accuracy sql.NullFloat64
	if err := s.Scan(
		&checkin.ID,
		&checkin.IncidentID,
		&checkin.CreatedAt,
		&battery,
		&network,
		&latitude,
		&longitude,
		&accuracy,
	); err != nil {
		return incidents.Checkin{}, err
	}
	checkin.CreatedAt = checkin.CreatedAt.UTC()
	if battery.Valid {
		value := int(battery.Int64)
		checkin.DeviceBatteryPercent = &value
	}
	if network.Valid {
		value := network.String
		checkin.DeviceNetwork = &value
	}
	if latitude.Valid {
		value := latitude.Float64
		checkin.Latitude = &value
	}
	if longitude.Valid {
		value := longitude.Float64
		checkin.Longitude = &value
	}
	if accuracy.Valid {
		value := accuracy.Float64
		checkin.AccuracyMeters = &value
	}
	return checkin, nil
}

func scanIncidentToken(s scanner) (incidents.IncidentToken, error) {
	var token incidents.IncidentToken
	var label sql.NullString
	var expiresAt sql.NullTime
	var revokedAt sql.NullTime
	if err := s.Scan(
		&token.ID,
		&token.IncidentID,
		&token.TokenHash,
		&label,
		&token.CreatedAt,
		&expiresAt,
		&revokedAt,
	); err != nil {
		return incidents.IncidentToken{}, err
	}
	token.CreatedAt = token.CreatedAt.UTC()
	if label.Valid {
		token.Label = label.String
	}
	token.ExpiresAt = nullableDBTime(expiresAt)
	token.RevokedAt = nullableDBTime(revokedAt)
	return token, nil
}

func scanContactPublicKey(s scanner) (incidents.ContactPublicKey, error) {
	var contactKey incidents.ContactPublicKey
	var displayLabel sql.NullString
	var revokedAt sql.NullTime
	if err := s.Scan(
		&contactKey.ID,
		&contactKey.OwnerAccountID,
		&contactKey.ContactID,
		&contactKey.Version,
		&displayLabel,
		&contactKey.WrappingAlgorithm,
		&contactKey.PublicKey,
		&contactKey.PublicKeyFingerprint,
		&contactKey.KeyState,
		&contactKey.CreatedAt,
		&contactKey.UpdatedAt,
		&revokedAt,
	); err != nil {
		return incidents.ContactPublicKey{}, err
	}
	contactKey.CreatedAt = contactKey.CreatedAt.UTC()
	contactKey.UpdatedAt = contactKey.UpdatedAt.UTC()
	if displayLabel.Valid {
		contactKey.DisplayLabel = displayLabel.String
	}
	contactKey.RevokedAt = nullableDBTime(revokedAt)
	return contactKey, nil
}

func scanSharingGrant(s scanner) (incidents.SharingGrant, error) {
	var grant incidents.SharingGrant
	var streamID sql.NullString
	var expiresAt sql.NullTime
	var revokedAt sql.NullTime
	var revokedByAccountID sql.NullString
	if err := s.Scan(
		&grant.ID,
		&grant.OwnerAccountID,
		&grant.IncidentID,
		&streamID,
		&grant.RecipientType,
		&grant.ContactID,
		&grant.ContactPublicKeyID,
		&grant.ContactPublicKeyVersion,
		&grant.DataClass,
		&grant.GrantState,
		&grant.CreatedAt,
		&grant.UpdatedAt,
		&expiresAt,
		&revokedAt,
		&revokedByAccountID,
	); err != nil {
		return incidents.SharingGrant{}, err
	}
	grant.CreatedAt = grant.CreatedAt.UTC()
	grant.UpdatedAt = grant.UpdatedAt.UTC()
	if streamID.Valid {
		grant.StreamID = streamID.String
	}
	grant.ExpiresAt = nullableDBTime(expiresAt)
	grant.RevokedAt = nullableDBTime(revokedAt)
	if revokedByAccountID.Valid {
		grant.RevokedByAccountID = revokedByAccountID.String
	}
	return grant, nil
}

func scanWrappedKeyRecord(s scanner) (incidents.WrappedKeyRecord, error) {
	var record incidents.WrappedKeyRecord
	var streamID sql.NullString
	var publicWrappingMetadata string
	var revokedAt sql.NullTime
	var revokedByAccountID sql.NullString
	var rotatedAt sql.NullTime
	if err := s.Scan(
		&record.ID,
		&record.OwnerAccountID,
		&record.IncidentID,
		&streamID,
		&record.GrantID,
		&record.RecipientType,
		&record.ContactID,
		&record.ContactPublicKeyID,
		&record.ContactPublicKeyVersion,
		&record.MediaKeyID,
		&record.WrappingAlgorithm,
		&record.WrappingAlgorithmVersion,
		&record.WrappedKeyCiphertext,
		&publicWrappingMetadata,
		&record.WrappedKeyState,
		&record.CreatedAt,
		&record.UpdatedAt,
		&revokedAt,
		&revokedByAccountID,
		&rotatedAt,
	); err != nil {
		return incidents.WrappedKeyRecord{}, err
	}
	record.CreatedAt = record.CreatedAt.UTC()
	record.UpdatedAt = record.UpdatedAt.UTC()
	if streamID.Valid {
		record.StreamID = streamID.String
	}
	record.PublicWrappingMetadata = []byte(publicWrappingMetadata)
	record.RevokedAt = nullableDBTime(revokedAt)
	if revokedByAccountID.Valid {
		record.RevokedByAccountID = revokedByAccountID.String
	}
	record.RotatedAt = nullableDBTime(rotatedAt)
	return record, nil
}

func nullableDBTime(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	utc := value.Time.UTC()
	return &utc
}
