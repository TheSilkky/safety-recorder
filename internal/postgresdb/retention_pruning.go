package postgresdb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/open-proofline/server/internal/incidents"
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
		return 0, fmt.Errorf("begin prune postgres incident token metadata: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	tokenIDs, err := pruneTokenIDs(ctx, tx, cutoff.UTC(), limit)
	if err != nil {
		return 0, err
	}
	pruned := 0
	for _, tokenID := range tokenIDs {
		result, err := tx.ExecContext(ctx, `
			DELETE FROM incident_tokens
			WHERE id = $1`,
			tokenID,
		)
		if err != nil {
			return pruned, fmt.Errorf("delete postgres incident token metadata: %w", err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return pruned, fmt.Errorf("delete postgres incident token metadata rows affected: %w", err)
		}
		pruned += int(rowsAffected)
	}
	if err := tx.Commit(); err != nil {
		return pruned, fmt.Errorf("commit prune postgres incident token metadata: %w", err)
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
		return 0, fmt.Errorf("begin prune postgres incident deletion tombstones: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	incidentIDs, err := pruneTombstoneIncidentIDs(ctx, tx, cutoff.UTC(), limit)
	if err != nil {
		return 0, err
	}
	pruned := 0
	for _, incidentID := range incidentIDs {
		result, err := tx.ExecContext(ctx, `
			DELETE FROM incidents
			WHERE id = $1 AND deletion_state = $2`,
			incidentID,
			incidents.IncidentDeletionStateDeleted,
		)
		if err != nil {
			return pruned, fmt.Errorf("delete postgres incident deletion tombstone: %w", err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return pruned, fmt.Errorf("delete postgres incident deletion tombstone rows affected: %w", err)
		}
		pruned += int(rowsAffected)
	}
	if err := tx.Commit(); err != nil {
		return pruned, fmt.Errorf("commit prune postgres incident deletion tombstones: %w", err)
	}
	return pruned, nil
}

func pruneTokenIDs(ctx context.Context, tx *sql.Tx, cutoff time.Time, limit int) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id
		FROM incident_tokens
		WHERE (expires_at IS NOT NULL AND expires_at <= $1)
			OR (revoked_at IS NOT NULL AND revoked_at <= $1)
		ORDER BY id ASC
		LIMIT $2`,
		cutoff,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("select postgres incident token metadata for pruning: %w", err)
	}
	defer rows.Close()

	tokenIDs := []string{}
	for rows.Next() {
		var tokenID string
		if err := rows.Scan(&tokenID); err != nil {
			return nil, fmt.Errorf("scan postgres incident token metadata for pruning: %w", err)
		}
		tokenIDs = append(tokenIDs, tokenID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres incident token metadata for pruning: %w", err)
	}
	return tokenIDs, nil
}

func pruneTombstoneIncidentIDs(ctx context.Context, tx *sql.Tx, cutoff time.Time, limit int) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT incidents.id
		FROM incidents
		JOIN incident_deletion_decisions
			ON incident_deletion_decisions.incident_id = incidents.id
		WHERE incidents.deletion_state = $1
			AND incident_deletion_decisions.state = $2
			AND incident_deletion_decisions.completed_at IS NOT NULL
			AND incident_deletion_decisions.completed_at <= $3
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
		LIMIT $4`,
		incidents.IncidentDeletionStateDeleted,
		incidents.IncidentDeletionStateDeleted,
		cutoff,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("select postgres incident deletion tombstones for pruning: %w", err)
	}
	defer rows.Close()

	incidentIDs := []string{}
	for rows.Next() {
		var incidentID string
		if err := rows.Scan(&incidentID); err != nil {
			return nil, fmt.Errorf("scan postgres incident deletion tombstone for pruning: %w", err)
		}
		incidentIDs = append(incidentIDs, incidentID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres incident deletion tombstones for pruning: %w", err)
	}
	return incidentIDs, nil
}
