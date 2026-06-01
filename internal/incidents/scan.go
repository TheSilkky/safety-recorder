package incidents

import "database/sql"

type scanner interface {
	Scan(dest ...any) error
}

func scanIncident(s scanner) (Incident, error) {
	var incident Incident
	var createdAt string
	var updatedAt string
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
		&createdAt,
		&updatedAt,
		&incident.Status,
		&clientLabel,
		&notes,
		&incidentMode,
		&captureProfile,
		&escalationPolicy,
		&sharingState,
		&incident.DeletionState,
	); err != nil {
		return Incident{}, err
	}
	parsedCreatedAt, err := parseDBTime(createdAt)
	if err != nil {
		return Incident{}, err
	}
	parsedUpdatedAt, err := parseDBTime(updatedAt)
	if err != nil {
		return Incident{}, err
	}
	incident.CreatedAt = parsedCreatedAt
	incident.UpdatedAt = parsedUpdatedAt
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
		incident.DeletionState = IncidentDeletionStateActive
	}
	return incident, nil
}

func scanChunk(s scanner) (Chunk, error) {
	var chunk Chunk
	var startedAt string
	var endedAt string
	var createdAt string
	var streamID sql.NullString
	var originalFilename sql.NullString
	if err := s.Scan(
		&chunk.ID,
		&chunk.IncidentID,
		&streamID,
		&chunk.ChunkIndex,
		&chunk.MediaType,
		&startedAt,
		&endedAt,
		&originalFilename,
		&chunk.StoredPath,
		&chunk.ByteSize,
		&chunk.SHA256Hex,
		&createdAt,
	); err != nil {
		return Chunk{}, err
	}
	parsedStartedAt, err := parseDBTime(startedAt)
	if err != nil {
		return Chunk{}, err
	}
	parsedEndedAt, err := parseDBTime(endedAt)
	if err != nil {
		return Chunk{}, err
	}
	parsedCreatedAt, err := parseDBTime(createdAt)
	if err != nil {
		return Chunk{}, err
	}
	chunk.StartedAt = parsedStartedAt
	chunk.EndedAt = parsedEndedAt
	chunk.CreatedAt = parsedCreatedAt
	if streamID.Valid {
		chunk.StreamID = streamID.String
	}
	if originalFilename.Valid {
		chunk.OriginalFilename = originalFilename.String
	}
	return chunk, nil
}

func scanUploadOperation(s scanner) (UploadOperation, error) {
	var operation UploadOperation
	var streamID sql.NullString
	var originalFilename sql.NullString
	var chunkID sql.NullString
	var storedPath sql.NullString
	var startedAt string
	var endedAt string
	var createdAt string
	var updatedAt string
	if err := s.Scan(
		&operation.ID,
		&operation.Operation,
		&operation.IdempotencyKeyHash,
		&operation.IncidentID,
		&streamID,
		&operation.ChunkIndex,
		&operation.MediaType,
		&startedAt,
		&endedAt,
		&originalFilename,
		&operation.ByteSize,
		&operation.SHA256Hex,
		&operation.FingerprintHash,
		&operation.State,
		&chunkID,
		&storedPath,
		&createdAt,
		&updatedAt,
	); err != nil {
		return UploadOperation{}, err
	}
	parsedStartedAt, err := parseDBTime(startedAt)
	if err != nil {
		return UploadOperation{}, err
	}
	parsedEndedAt, err := parseDBTime(endedAt)
	if err != nil {
		return UploadOperation{}, err
	}
	parsedCreatedAt, err := parseDBTime(createdAt)
	if err != nil {
		return UploadOperation{}, err
	}
	parsedUpdatedAt, err := parseDBTime(updatedAt)
	if err != nil {
		return UploadOperation{}, err
	}
	operation.StartedAt = parsedStartedAt
	operation.EndedAt = parsedEndedAt
	operation.CreatedAt = parsedCreatedAt
	operation.UpdatedAt = parsedUpdatedAt
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

func scanMediaStream(s scanner) (MediaStream, error) {
	var stream MediaStream
	var label sql.NullString
	var expectedChunkCount sql.NullInt64
	var createdAt string
	var updatedAt string
	var completedAt sql.NullString
	var failedAt sql.NullString
	var failureReason sql.NullString
	if err := s.Scan(
		&stream.ID,
		&stream.IncidentID,
		&stream.MediaType,
		&label,
		&stream.Status,
		&expectedChunkCount,
		&createdAt,
		&updatedAt,
		&completedAt,
		&failedAt,
		&failureReason,
	); err != nil {
		return MediaStream{}, err
	}
	parsedCreatedAt, err := parseDBTime(createdAt)
	if err != nil {
		return MediaStream{}, err
	}
	parsedUpdatedAt, err := parseDBTime(updatedAt)
	if err != nil {
		return MediaStream{}, err
	}
	stream.CreatedAt = parsedCreatedAt
	stream.UpdatedAt = parsedUpdatedAt
	if label.Valid {
		stream.Label = label.String
	}
	if expectedChunkCount.Valid {
		count := int(expectedChunkCount.Int64)
		stream.ExpectedChunkCount = &count
	}
	if stream.CompletedAt, err = nullableDBTime(completedAt); err != nil {
		return MediaStream{}, err
	}
	if stream.FailedAt, err = nullableDBTime(failedAt); err != nil {
		return MediaStream{}, err
	}
	if failureReason.Valid {
		stream.FailureReason = failureReason.String
	}
	return stream, nil
}

func scanCheckin(s scanner) (Checkin, error) {
	var checkin Checkin
	var createdAt string
	var battery sql.NullInt64
	var network sql.NullString
	var latitude sql.NullFloat64
	var longitude sql.NullFloat64
	var accuracy sql.NullFloat64
	if err := s.Scan(
		&checkin.ID,
		&checkin.IncidentID,
		&createdAt,
		&battery,
		&network,
		&latitude,
		&longitude,
		&accuracy,
	); err != nil {
		return Checkin{}, err
	}
	parsedCreatedAt, err := parseDBTime(createdAt)
	if err != nil {
		return Checkin{}, err
	}
	checkin.CreatedAt = parsedCreatedAt
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

func scanIncidentToken(s scanner) (IncidentToken, error) {
	var token IncidentToken
	var label sql.NullString
	var createdAt string
	var expiresAt sql.NullString
	var revokedAt sql.NullString
	if err := s.Scan(
		&token.ID,
		&token.IncidentID,
		&token.TokenHash,
		&label,
		&createdAt,
		&expiresAt,
		&revokedAt,
	); err != nil {
		return IncidentToken{}, err
	}
	parsedCreatedAt, err := parseDBTime(createdAt)
	if err != nil {
		return IncidentToken{}, err
	}
	token.CreatedAt = parsedCreatedAt
	if label.Valid {
		token.Label = label.String
	}
	if token.ExpiresAt, err = nullableDBTime(expiresAt); err != nil {
		return IncidentToken{}, err
	}
	if token.RevokedAt, err = nullableDBTime(revokedAt); err != nil {
		return IncidentToken{}, err
	}
	return token, nil
}

func scanContactPublicKey(s scanner) (ContactPublicKey, error) {
	var contactKey ContactPublicKey
	var displayLabel sql.NullString
	var createdAt string
	var updatedAt string
	var revokedAt sql.NullString
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
		&createdAt,
		&updatedAt,
		&revokedAt,
	); err != nil {
		return ContactPublicKey{}, err
	}
	parsedCreatedAt, err := parseDBTime(createdAt)
	if err != nil {
		return ContactPublicKey{}, err
	}
	parsedUpdatedAt, err := parseDBTime(updatedAt)
	if err != nil {
		return ContactPublicKey{}, err
	}
	contactKey.CreatedAt = parsedCreatedAt
	contactKey.UpdatedAt = parsedUpdatedAt
	if displayLabel.Valid {
		contactKey.DisplayLabel = displayLabel.String
	}
	if contactKey.RevokedAt, err = nullableDBTime(revokedAt); err != nil {
		return ContactPublicKey{}, err
	}
	return contactKey, nil
}

func scanSharingGrant(s scanner) (SharingGrant, error) {
	var grant SharingGrant
	var streamID sql.NullString
	var expiresAt sql.NullString
	var revokedAt sql.NullString
	var revokedByAccountID sql.NullString
	var createdAt string
	var updatedAt string
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
		&createdAt,
		&updatedAt,
		&expiresAt,
		&revokedAt,
		&revokedByAccountID,
	); err != nil {
		return SharingGrant{}, err
	}
	parsedCreatedAt, err := parseDBTime(createdAt)
	if err != nil {
		return SharingGrant{}, err
	}
	parsedUpdatedAt, err := parseDBTime(updatedAt)
	if err != nil {
		return SharingGrant{}, err
	}
	grant.CreatedAt = parsedCreatedAt
	grant.UpdatedAt = parsedUpdatedAt
	if streamID.Valid {
		grant.StreamID = streamID.String
	}
	if grant.ExpiresAt, err = nullableDBTime(expiresAt); err != nil {
		return SharingGrant{}, err
	}
	if grant.RevokedAt, err = nullableDBTime(revokedAt); err != nil {
		return SharingGrant{}, err
	}
	if revokedByAccountID.Valid {
		grant.RevokedByAccountID = revokedByAccountID.String
	}
	return grant, nil
}

func scanWrappedKeyRecord(s scanner) (WrappedKeyRecord, error) {
	var record WrappedKeyRecord
	var streamID sql.NullString
	var publicWrappingMetadata string
	var createdAt string
	var updatedAt string
	var revokedAt sql.NullString
	var revokedByAccountID sql.NullString
	var rotatedAt sql.NullString
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
		&createdAt,
		&updatedAt,
		&revokedAt,
		&revokedByAccountID,
		&rotatedAt,
	); err != nil {
		return WrappedKeyRecord{}, err
	}
	parsedCreatedAt, err := parseDBTime(createdAt)
	if err != nil {
		return WrappedKeyRecord{}, err
	}
	parsedUpdatedAt, err := parseDBTime(updatedAt)
	if err != nil {
		return WrappedKeyRecord{}, err
	}
	record.CreatedAt = parsedCreatedAt
	record.UpdatedAt = parsedUpdatedAt
	if streamID.Valid {
		record.StreamID = streamID.String
	}
	record.PublicWrappingMetadata = []byte(publicWrappingMetadata)
	if record.RevokedAt, err = nullableDBTime(revokedAt); err != nil {
		return WrappedKeyRecord{}, err
	}
	if revokedByAccountID.Valid {
		record.RevokedByAccountID = revokedByAccountID.String
	}
	if record.RotatedAt, err = nullableDBTime(rotatedAt); err != nil {
		return WrappedKeyRecord{}, err
	}
	return record, nil
}
