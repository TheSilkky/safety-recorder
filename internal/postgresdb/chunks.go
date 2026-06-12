package postgresdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/open-proofline/server/internal/incidents"
)

// ChunkExists reports whether an incident already has a chunk with the same
// stream-scoped or legacy unstreamed identity.
func (r *Repository) ChunkExists(ctx context.Context, incidentID, streamID, mediaType string, chunkIndex int) (bool, error) {
	var exists int
	if streamID != "" {
		err := r.db.QueryRowContext(ctx, `
			SELECT 1
			FROM chunks
			WHERE incident_id = $1 AND stream_id = $2 AND chunk_index = $3`,
			incidentID,
			streamID,
			chunkIndex,
		).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("check postgres stream chunk exists: %w", err)
		}
		return true, nil
	}

	err := r.db.QueryRowContext(ctx, `
		SELECT 1
		FROM chunks
		WHERE incident_id = $1 AND stream_id IS NULL AND media_type = $2 AND chunk_index = $3`,
		incidentID,
		mediaType,
		chunkIndex,
	).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check postgres chunk exists: %w", err)
	}
	return true, nil
}

// GetChunkByIdentity returns one chunk by its stream-scoped or legacy
// unstreamed immutable identity.
func (r *Repository) GetChunkByIdentity(ctx context.Context, incidentID, streamID, mediaType string, chunkIndex int) (incidents.Chunk, error) {
	if streamID != "" {
		row := r.db.QueryRowContext(ctx, `
			SELECT id, incident_id, stream_id, chunk_index, media_type, started_at, ended_at,
				original_filename, stored_path, byte_size, sha256_hex, created_at
			FROM chunks
			WHERE incident_id = $1 AND stream_id = $2 AND chunk_index = $3`,
			incidentID,
			streamID,
			chunkIndex,
		)
		chunk, err := scanChunk(row)
		if errors.Is(err, sql.ErrNoRows) {
			return incidents.Chunk{}, incidents.ErrNotFound
		}
		if err != nil {
			return incidents.Chunk{}, fmt.Errorf("get postgres stream chunk by identity: %w", err)
		}
		return chunk, nil
	}

	return r.GetChunkByKey(ctx, incidentID, mediaType, chunkIndex)
}

// AccountCommittedBlobBytes returns committed encrypted chunk bytes for all
// incidents owned by accountID. Chunks continue to count while deletion is
// pending or retrying because metadata rows are pruned only after durable blob
// deletion completes.
func (r *Repository) AccountCommittedBlobBytes(ctx context.Context, ownerAccountID string) (int64, error) {
	if ownerAccountID == "" {
		return 0, nil
	}
	var total int64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(chunks.byte_size), 0)
		FROM chunks
		JOIN incidents ON incidents.id = chunks.incident_id
		WHERE incidents.owner_account_id = $1`,
		ownerAccountID,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("read postgres account committed blob bytes: %w", err)
	}
	return total, nil
}

// CreateChunk inserts metadata for a chunk after the blob has been committed.
func (r *Repository) CreateChunk(ctx context.Context, params incidents.CreateChunkParams) (incidents.Chunk, error) {
	id, err := newID("chk")
	if err != nil {
		return incidents.Chunk{}, err
	}
	chunk := incidents.Chunk{
		ID:               id,
		IncidentID:       params.IncidentID,
		StreamID:         params.StreamID,
		ChunkIndex:       params.ChunkIndex,
		MediaType:        params.MediaType,
		StartedAt:        params.StartedAt.UTC(),
		EndedAt:          params.EndedAt.UTC(),
		OriginalFilename: params.OriginalFilename,
		StoredPath:       params.StoredPath,
		ByteSize:         params.ByteSize,
		SHA256Hex:        params.SHA256Hex,
		CreatedAt:        time.Now().UTC(),
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return incidents.Chunk{}, fmt.Errorf("begin create postgres chunk: %w", err)
	}
	ownerAccountID, err := validateChunkInsertState(ctx, tx, params)
	if err != nil {
		_ = tx.Rollback()
		return incidents.Chunk{}, err
	}
	exists, err := chunkIdentityExistsTx(ctx, tx, params)
	if err != nil {
		_ = tx.Rollback()
		return incidents.Chunk{}, err
	}
	if exists {
		_ = tx.Rollback()
		return incidents.Chunk{}, incidents.ErrDuplicate
	}
	if err := enforceAccountBlobQuotaTx(ctx, tx, ownerAccountID, params.ByteSize, params.AccountBlobQuotaBytes); err != nil {
		_ = tx.Rollback()
		return incidents.Chunk{}, err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO chunks (
			id, incident_id, stream_id, chunk_index, media_type, started_at, ended_at,
			original_filename, stored_path, byte_size, sha256_hex, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		chunk.ID,
		chunk.IncidentID,
		nullableString(chunk.StreamID),
		chunk.ChunkIndex,
		chunk.MediaType,
		chunk.StartedAt,
		chunk.EndedAt,
		nullableString(chunk.OriginalFilename),
		chunk.StoredPath,
		chunk.ByteSize,
		chunk.SHA256Hex,
		chunk.CreatedAt,
	)
	if err != nil {
		_ = tx.Rollback()
		if isUniqueViolation(err) {
			return incidents.Chunk{}, incidents.ErrDuplicate
		}
		if isForeignKeyViolation(err) {
			return incidents.Chunk{}, incidents.ErrNotFound
		}
		return incidents.Chunk{}, fmt.Errorf("insert postgres chunk: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return incidents.Chunk{}, fmt.Errorf("commit create postgres chunk: %w", err)
	}

	return chunk, nil
}

func validateChunkInsertState(ctx context.Context, tx *sql.Tx, params incidents.CreateChunkParams) (string, error) {
	var incidentStatus string
	var deletionState string
	var ownerAccountID string
	err := tx.QueryRowContext(ctx, `
		SELECT status, deletion_state, COALESCE(owner_account_id, '')
		FROM incidents
		WHERE id = $1
		FOR UPDATE`,
		params.IncidentID,
	).Scan(&incidentStatus, &deletionState, &ownerAccountID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", incidents.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read postgres incident status: %w", err)
	}
	if deletionState != incidents.IncidentDeletionStateActive {
		return "", incidents.ErrIncidentDeleting
	}
	if incidentStatus != incidents.StatusOpen {
		return "", incidents.ErrIncidentClosed
	}
	if params.StreamID == "" {
		return ownerAccountID, nil
	}

	var streamStatus string
	var streamMediaType string
	err = tx.QueryRowContext(ctx, `
		SELECT status, media_type
		FROM media_streams
		WHERE incident_id = $1 AND id = $2
		FOR UPDATE`,
		params.IncidentID,
		params.StreamID,
	).Scan(&streamStatus, &streamMediaType)
	if errors.Is(err, sql.ErrNoRows) {
		return "", incidents.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read postgres media stream state: %w", err)
	}
	if streamStatus != incidents.StreamStatusOpen || streamMediaType != params.MediaType {
		return "", incidents.ErrInvalidState
	}
	return ownerAccountID, nil
}

func chunkIdentityExistsTx(ctx context.Context, tx *sql.Tx, params incidents.CreateChunkParams) (bool, error) {
	var exists int
	var err error
	if params.StreamID != "" {
		err = tx.QueryRowContext(ctx, `
			SELECT 1
			FROM chunks
			WHERE incident_id = $1 AND stream_id = $2 AND chunk_index = $3`,
			params.IncidentID,
			params.StreamID,
			params.ChunkIndex,
		).Scan(&exists)
	} else {
		err = tx.QueryRowContext(ctx, `
			SELECT 1
			FROM chunks
			WHERE incident_id = $1 AND stream_id IS NULL AND media_type = $2 AND chunk_index = $3`,
			params.IncidentID,
			params.MediaType,
			params.ChunkIndex,
		).Scan(&exists)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check postgres chunk identity for insert: %w", err)
	}
	return true, nil
}

func enforceAccountBlobQuotaTx(ctx context.Context, tx *sql.Tx, ownerAccountID string, additionalBytes int64, quotaBytes int64) error {
	if ownerAccountID == "" || quotaBytes <= 0 || additionalBytes <= 0 {
		return nil
	}
	if additionalBytes > quotaBytes {
		return incidents.ErrAccountBlobQuotaExceeded
	}
	var lockedAccountID string
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM accounts
		WHERE id = $1
		FOR UPDATE`,
		ownerAccountID,
	).Scan(&lockedAccountID)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("lock postgres account for blob quota: %w", err)
	}

	var currentBytes int64
	err = tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(chunks.byte_size), 0)
		FROM chunks
		JOIN incidents ON incidents.id = chunks.incident_id
		WHERE incidents.owner_account_id = $1`,
		ownerAccountID,
	).Scan(&currentBytes)
	if err != nil {
		return fmt.Errorf("read postgres account committed blob bytes for quota: %w", err)
	}
	if currentBytes > quotaBytes-additionalBytes {
		return incidents.ErrAccountBlobQuotaExceeded
	}
	return nil
}

// ListChunks returns chunk metadata for an incident without loading file bytes.
func (r *Repository) ListChunks(ctx context.Context, incidentID string) ([]incidents.Chunk, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, incident_id, stream_id, chunk_index, media_type, started_at, ended_at,
			original_filename, stored_path, byte_size, sha256_hex, created_at
		FROM chunks
		WHERE incident_id = $1
		ORDER BY chunk_index ASC, media_type ASC`,
		incidentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list postgres chunks: %w", err)
	}
	defer rows.Close()

	chunks := []incidents.Chunk{}
	for rows.Next() {
		chunk, err := scanChunk(rows)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres chunks: %w", err)
	}
	return chunks, nil
}

// GetChunkByKey returns one legacy unstreamed chunk by incident, media type,
// and chunk index. Streamed chunks are addressed through stream-aware routes.
func (r *Repository) GetChunkByKey(ctx context.Context, incidentID, mediaType string, chunkIndex int) (incidents.Chunk, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, incident_id, stream_id, chunk_index, media_type, started_at, ended_at,
			original_filename, stored_path, byte_size, sha256_hex, created_at
		FROM chunks
		WHERE incident_id = $1 AND stream_id IS NULL AND media_type = $2 AND chunk_index = $3`,
		incidentID,
		mediaType,
		chunkIndex,
	)

	chunk, err := scanChunk(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.Chunk{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.Chunk{}, fmt.Errorf("get postgres chunk: %w", err)
	}
	return chunk, nil
}
