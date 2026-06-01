package incidents

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// PruneIncidentTokenMetadata deletes expired or revoked viewer-token metadata
// selected by an operator-reviewed audit cutoff. It never deletes incidents or
// evidence metadata.
func (r *Repository) PruneIncidentTokenMetadata(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = 25
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin prune incident token metadata: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	tokenIDs, err := pruneTokenIDs(ctx, tx, formatDBTime(cutoff.UTC()), limit)
	if err != nil {
		return 0, err
	}
	pruned := 0
	for _, tokenID := range tokenIDs {
		result, err := tx.ExecContext(ctx, `
			DELETE FROM incident_tokens
			WHERE id = ?`,
			tokenID,
		)
		if err != nil {
			return pruned, fmt.Errorf("delete incident token metadata: %w", err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return pruned, fmt.Errorf("delete incident token metadata rows affected: %w", err)
		}
		pruned += int(rowsAffected)
	}
	if err := tx.Commit(); err != nil {
		return pruned, fmt.Errorf("commit prune incident token metadata: %w", err)
	}
	return pruned, nil
}

// PruneIncidentDeletionTombstones deletes only completed minimal tombstones
// after the configured cutoff. It refuses incidents that still have child
// metadata or deletion retry items.
func (r *Repository) PruneIncidentDeletionTombstones(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = 25
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin prune incident deletion tombstones: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	incidentIDs, err := pruneTombstoneIncidentIDs(ctx, tx, formatDBTime(cutoff.UTC()), limit)
	if err != nil {
		return 0, err
	}
	pruned := 0
	for _, incidentID := range incidentIDs {
		result, err := tx.ExecContext(ctx, `
			DELETE FROM incidents
			WHERE id = ? AND deletion_state = ?`,
			incidentID,
			IncidentDeletionStateDeleted,
		)
		if err != nil {
			return pruned, fmt.Errorf("delete incident deletion tombstone: %w", err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return pruned, fmt.Errorf("delete incident deletion tombstone rows affected: %w", err)
		}
		pruned += int(rowsAffected)
	}
	if err := tx.Commit(); err != nil {
		return pruned, fmt.Errorf("commit prune incident deletion tombstones: %w", err)
	}
	return pruned, nil
}

func pruneTokenIDs(ctx context.Context, tx *sql.Tx, cutoff string, limit int) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id
		FROM incident_tokens
		WHERE (expires_at IS NOT NULL AND expires_at <= ?)
			OR (revoked_at IS NOT NULL AND revoked_at <= ?)
		ORDER BY id ASC
		LIMIT ?`,
		cutoff,
		cutoff,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("select incident token metadata for pruning: %w", err)
	}
	defer rows.Close()

	tokenIDs := []string{}
	for rows.Next() {
		var tokenID string
		if err := rows.Scan(&tokenID); err != nil {
			return nil, fmt.Errorf("scan incident token metadata for pruning: %w", err)
		}
		tokenIDs = append(tokenIDs, tokenID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate incident token metadata for pruning: %w", err)
	}
	return tokenIDs, nil
}

func pruneTombstoneIncidentIDs(ctx context.Context, tx *sql.Tx, cutoff string, limit int) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT incidents.id
		FROM incidents
		JOIN incident_deletion_decisions
			ON incident_deletion_decisions.incident_id = incidents.id
		WHERE incidents.deletion_state = ?
			AND incident_deletion_decisions.state = ?
			AND incident_deletion_decisions.completed_at IS NOT NULL
			AND incident_deletion_decisions.completed_at <= ?
			AND NOT EXISTS (
				SELECT 1 FROM incident_deletion_items
				WHERE incident_deletion_items.incident_id = incidents.id
			)
			AND NOT EXISTS (
				SELECT 1 FROM upload_operations
				WHERE upload_operations.incident_id = incidents.id
			)
			AND NOT EXISTS (
				SELECT 1 FROM incident_tokens
				WHERE incident_tokens.incident_id = incidents.id
			)
			AND NOT EXISTS (
				SELECT 1 FROM checkins
				WHERE checkins.incident_id = incidents.id
			)
			AND NOT EXISTS (
				SELECT 1 FROM chunks
				WHERE chunks.incident_id = incidents.id
			)
			AND NOT EXISTS (
				SELECT 1 FROM media_streams
				WHERE media_streams.incident_id = incidents.id
			)
		ORDER BY incident_deletion_decisions.completed_at ASC, incidents.id ASC
		LIMIT ?`,
		IncidentDeletionStateDeleted,
		IncidentDeletionStateDeleted,
		cutoff,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("select incident deletion tombstones for pruning: %w", err)
	}
	defer rows.Close()

	incidentIDs := []string{}
	for rows.Next() {
		var incidentID string
		if err := rows.Scan(&incidentID); err != nil {
			return nil, fmt.Errorf("scan incident deletion tombstone for pruning: %w", err)
		}
		incidentIDs = append(incidentIDs, incidentID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate incident deletion tombstones for pruning: %w", err)
	}
	return incidentIDs, nil
}
