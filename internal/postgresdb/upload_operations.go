package postgresdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/open-proofline/server/internal/incidents"
)

const uploadOperationColumns = `
	id, operation, idempotency_key_hash, incident_id, stream_id, chunk_index,
	media_type, started_at, ended_at, original_filename, byte_size, sha256_hex,
	fingerprint_hash, state, chunk_id, stored_path, created_at, updated_at`

// ReserveUploadOperation binds an idempotency-key hash to immutable upload
// inputs. Reusing the same key with different inputs returns ErrIdempotencyConflict.
func (r *Repository) ReserveUploadOperation(ctx context.Context, params incidents.UploadOperationParams) (incidents.UploadOperation, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return incidents.UploadOperation{}, fmt.Errorf("begin reserve postgres upload operation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockActiveIncident(ctx, tx, params.IncidentID); err != nil {
		return incidents.UploadOperation{}, err
	}

	now := time.Now().UTC()
	id, err := newID("uop")
	if err != nil {
		return incidents.UploadOperation{}, err
	}
	operation := incidents.UploadOperation{
		ID:                 id,
		Operation:          params.Operation,
		IdempotencyKeyHash: params.IdempotencyKeyHash,
		IncidentID:         params.IncidentID,
		StreamID:           params.StreamID,
		ChunkIndex:         params.ChunkIndex,
		MediaType:          params.MediaType,
		StartedAt:          params.StartedAt.UTC(),
		EndedAt:            params.EndedAt.UTC(),
		OriginalFilename:   params.OriginalFilename,
		ByteSize:           params.ByteSize,
		SHA256Hex:          params.SHA256Hex,
		FingerprintHash:    params.FingerprintHash,
		State:              incidents.UploadOperationStateReserved,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO upload_operations (
			id, operation, idempotency_key_hash, incident_id, stream_id, chunk_index,
			media_type, started_at, ended_at, original_filename, byte_size, sha256_hex,
			fingerprint_hash, state, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		operation.ID,
		operation.Operation,
		operation.IdempotencyKeyHash,
		operation.IncidentID,
		nullableString(operation.StreamID),
		operation.ChunkIndex,
		operation.MediaType,
		operation.StartedAt,
		operation.EndedAt,
		nullableString(operation.OriginalFilename),
		operation.ByteSize,
		operation.SHA256Hex,
		operation.FingerprintHash,
		operation.State,
		operation.CreatedAt,
		operation.UpdatedAt,
	)
	if err == nil {
		if err := tx.Commit(); err != nil {
			return incidents.UploadOperation{}, fmt.Errorf("commit reserve postgres upload operation: %w", err)
		}
		return operation, nil
	}
	if isForeignKeyViolation(err) {
		return incidents.UploadOperation{}, incidents.ErrNotFound
	}
	if !isUniqueViolation(err) {
		return incidents.UploadOperation{}, fmt.Errorf("insert postgres upload operation: %w", err)
	}
	_ = tx.Rollback()

	existing, getErr := r.getUploadOperationByKey(ctx, params.Operation, params.IdempotencyKeyHash)
	if errors.Is(getErr, incidents.ErrNotFound) {
		return incidents.UploadOperation{}, incidents.ErrNotFound
	}
	if getErr != nil {
		return incidents.UploadOperation{}, getErr
	}
	if !uploadOperationMatchesParams(existing, params) {
		return incidents.UploadOperation{}, incidents.ErrIdempotencyConflict
	}
	return existing, nil
}

// CompleteUploadOperation records the final chunk row confirmed for a reserved
// idempotent upload operation.
func (r *Repository) CompleteUploadOperation(ctx context.Context, params incidents.UploadOperationParams, chunk incidents.Chunk) (incidents.UploadOperation, error) {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE upload_operations
		SET state = $1, chunk_id = $2, stored_path = $3, updated_at = $4
		WHERE operation = $5 AND idempotency_key_hash = $6 AND fingerprint_hash = $7`,
		incidents.UploadOperationStateMetadataCommitted,
		chunk.ID,
		chunk.StoredPath,
		now,
		params.Operation,
		params.IdempotencyKeyHash,
		params.FingerprintHash,
	)
	if err != nil {
		return incidents.UploadOperation{}, fmt.Errorf("complete postgres upload operation: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return incidents.UploadOperation{}, fmt.Errorf("complete postgres upload operation rows affected: %w", err)
	}
	if rowsAffected == 0 {
		existing, getErr := r.getUploadOperationByKey(ctx, params.Operation, params.IdempotencyKeyHash)
		if errors.Is(getErr, incidents.ErrNotFound) {
			return incidents.UploadOperation{}, incidents.ErrNotFound
		}
		if getErr != nil {
			return incidents.UploadOperation{}, getErr
		}
		if !uploadOperationMatchesParams(existing, params) {
			return incidents.UploadOperation{}, incidents.ErrIdempotencyConflict
		}
		return incidents.UploadOperation{}, incidents.ErrNotFound
	}
	return r.getUploadOperationByKey(ctx, params.Operation, params.IdempotencyKeyHash)
}

func (r *Repository) getUploadOperationByKey(ctx context.Context, operation, keyHash string) (incidents.UploadOperation, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+uploadOperationColumns+`
		FROM upload_operations
		WHERE operation = $1 AND idempotency_key_hash = $2`,
		operation,
		keyHash,
	)
	uploadOperation, err := scanUploadOperation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.UploadOperation{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.UploadOperation{}, fmt.Errorf("get postgres upload operation: %w", err)
	}
	return uploadOperation, nil
}

func uploadOperationMatchesParams(operation incidents.UploadOperation, params incidents.UploadOperationParams) bool {
	return operation.Operation == params.Operation &&
		operation.IdempotencyKeyHash == params.IdempotencyKeyHash &&
		operation.IncidentID == params.IncidentID &&
		operation.StreamID == params.StreamID &&
		operation.ChunkIndex == params.ChunkIndex &&
		operation.MediaType == params.MediaType &&
		postgresTimeEqual(operation.StartedAt, params.StartedAt) &&
		postgresTimeEqual(operation.EndedAt, params.EndedAt) &&
		operation.OriginalFilename == params.OriginalFilename &&
		operation.ByteSize == params.ByteSize &&
		operation.SHA256Hex == params.SHA256Hex &&
		operation.FingerprintHash == params.FingerprintHash
}

func postgresTimeEqual(stored, input time.Time) bool {
	delta := stored.UTC().Sub(input.UTC())
	if delta < 0 {
		delta = -delta
	}
	return delta < time.Microsecond
}
