package postgresdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/open-proofline/server/internal/incidents"
)

const defaultLegacyUnownedIncidentLimit = 100

// ListLegacyUnownedIncidentCandidates returns safe private review metadata for
// incidents that still have no owner account.
func (r *Repository) ListLegacyUnownedIncidentCandidates(ctx context.Context, limit int) ([]incidents.LegacyUnownedIncidentCandidate, error) {
	if limit <= 0 {
		limit = defaultLegacyUnownedIncidentLimit
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT i.id, i.created_at, i.updated_at, i.status, i.deletion_state,
			COALESCE(streams.stream_count, 0),
			COALESCE(chunks.chunk_count, 0),
			COALESCE(checkins.checkin_count, 0),
			COALESCE(tokens.token_count, 0),
			COALESCE(active_tokens.active_token_count, 0),
			i.incident_mode, i.capture_profile, i.escalation_policy, i.sharing_state
		FROM incidents i
		LEFT JOIN (
			SELECT incident_id, COUNT(*) AS stream_count
			FROM media_streams
			GROUP BY incident_id
		) streams ON streams.incident_id = i.id
		LEFT JOIN (
			SELECT incident_id, COUNT(*) AS chunk_count
			FROM chunks
			GROUP BY incident_id
		) chunks ON chunks.incident_id = i.id
		LEFT JOIN (
			SELECT incident_id, COUNT(*) AS checkin_count
			FROM checkins
			GROUP BY incident_id
		) checkins ON checkins.incident_id = i.id
		LEFT JOIN (
			SELECT incident_id, COUNT(*) AS token_count
			FROM incident_tokens
			GROUP BY incident_id
		) tokens ON tokens.incident_id = i.id
		LEFT JOIN (
			SELECT incident_id, COUNT(*) AS active_token_count
			FROM incident_tokens
			WHERE revoked_at IS NULL AND (expires_at IS NULL OR expires_at > $1)
			GROUP BY incident_id
		) active_tokens ON active_tokens.incident_id = i.id
		WHERE i.owner_account_id IS NULL
		ORDER BY i.updated_at DESC, i.created_at DESC, i.id ASC
		LIMIT $2`,
		time.Now().UTC(),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list postgres legacy unowned incident candidates: %w", err)
	}
	defer rows.Close()

	candidates, err := scanLegacyUnownedIncidentCandidates(rows)
	if err != nil {
		return nil, fmt.Errorf("scan postgres legacy unowned incident candidates: %w", err)
	}
	return candidates, nil
}

// ReassignLegacyUnownedIncident records one private assignment or keep-unowned
// decision for an active legacy unowned incident.
func (r *Repository) ReassignLegacyUnownedIncident(ctx context.Context, params incidents.LegacyIncidentReassignmentParams) (incidents.LegacyIncidentReassignmentEvent, error) {
	eventID, err := newID("lra")
	if err != nil {
		return incidents.LegacyIncidentReassignmentEvent{}, err
	}
	now := time.Now().UTC()
	event := incidents.LegacyIncidentReassignmentEvent{
		ID:                eventID,
		IncidentID:        params.IncidentID,
		NewOwnerAccountID: params.NewOwnerAccountID,
		ActorAccountID:    params.ActorAccountID,
		Action:            params.Action,
		ReasonCode:        params.ReasonCode,
		Source:            params.Source,
		CreatedAt:         now,
		CompletedAt:       now,
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return incidents.LegacyIncidentReassignmentEvent{}, fmt.Errorf("begin postgres legacy reassignment: %w", err)
	}
	defer tx.Rollback()

	if params.Action == incidents.LegacyIncidentReassignmentActionAssignOwner {
		if err := requireAccountTx(ctx, tx, params.NewOwnerAccountID); err != nil {
			return incidents.LegacyIncidentReassignmentEvent{}, err
		}
	}
	if err := requireActiveUnownedIncidentTx(ctx, tx, params.IncidentID); err != nil {
		return incidents.LegacyIncidentReassignmentEvent{}, err
	}
	if params.Action == incidents.LegacyIncidentReassignmentActionAssignOwner {
		result, err := tx.ExecContext(ctx, `
			UPDATE incidents
			SET owner_account_id = $1
			WHERE id = $2 AND owner_account_id IS NULL AND deletion_state = $3`,
			params.NewOwnerAccountID,
			params.IncidentID,
			incidents.IncidentDeletionStateActive,
		)
		if err != nil {
			return incidents.LegacyIncidentReassignmentEvent{}, fmt.Errorf("assign postgres legacy incident owner: %w", err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return incidents.LegacyIncidentReassignmentEvent{}, fmt.Errorf("assign postgres legacy incident owner rows affected: %w", err)
		}
		if rowsAffected != 1 {
			return incidents.LegacyIncidentReassignmentEvent{}, incidents.ErrInvalidState
		}
	}
	if err := insertLegacyIncidentReassignmentEventTx(ctx, tx, event); err != nil {
		return incidents.LegacyIncidentReassignmentEvent{}, err
	}
	if err := tx.Commit(); err != nil {
		return incidents.LegacyIncidentReassignmentEvent{}, fmt.Errorf("commit postgres legacy incident reassignment: %w", err)
	}
	return event, nil
}

func requireAccountTx(ctx context.Context, tx *sql.Tx, accountID string) error {
	var id string
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM accounts
		WHERE id = $1`,
		accountID,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read postgres reassignment account: %w", err)
	}
	return nil
}

func requireActiveUnownedIncidentTx(ctx context.Context, tx *sql.Tx, incidentID string) error {
	var ownerAccountID sql.NullString
	var deletionState string
	err := tx.QueryRowContext(ctx, `
		SELECT owner_account_id, deletion_state
		FROM incidents
		WHERE id = $1`,
		incidentID,
	).Scan(&ownerAccountID, &deletionState)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read postgres legacy incident owner state: %w", err)
	}
	if ownerAccountID.Valid || deletionState != incidents.IncidentDeletionStateActive {
		return incidents.ErrInvalidState
	}
	return nil
}

func insertLegacyIncidentReassignmentEventTx(ctx context.Context, tx *sql.Tx, event incidents.LegacyIncidentReassignmentEvent) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO legacy_incident_reassignment_events (
			id, incident_id, previous_owner_account_id, new_owner_account_id,
			actor_account_id, action, reason_code, source, created_at, completed_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		event.ID,
		event.IncidentID,
		nullableString(event.PreviousOwnerAccountID),
		nullableString(event.NewOwnerAccountID),
		event.ActorAccountID,
		event.Action,
		event.ReasonCode,
		event.Source,
		event.CreatedAt,
		event.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("insert postgres legacy reassignment event: %w", err)
	}
	return nil
}
