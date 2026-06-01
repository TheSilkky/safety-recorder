package postgresdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/open-proofline/server/internal/incidents"
)

const deletionStatusSelect = `
	SELECT id, incident_id, source, reason_code, actor_account_id, allow_open,
		state, item_count, error_code, requested_at, updated_at, started_at, completed_at
	FROM incident_deletion_decisions`

// RequestIncidentDeletion creates or returns a durable deletion decision for one
// incident. Stored paths are snapshotted from chunk metadata inside the same
// transaction before the incident is marked deletion-pending.
func (r *Repository) RequestIncidentDeletion(ctx context.Context, params incidents.IncidentDeletionRequest) (incidents.IncidentDeletionStatus, error) {
	if !validDeletionSource(params.Source) {
		return incidents.IncidentDeletionStatus{}, incidents.ErrInvalidState
	}
	now := time.Now().UTC()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("begin request postgres incident deletion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	existing, err := getIncidentDeletionStatusTx(ctx, tx, params.IncidentID)
	if err == nil {
		if params.RequireOwnerID != "" {
			if err := requireIncidentOwnerTx(ctx, tx, params.IncidentID, params.RequireOwnerID); err != nil {
				return incidents.IncidentDeletionStatus{}, err
			}
		}
		if err := tx.Commit(); err != nil {
			return incidents.IncidentDeletionStatus{}, fmt.Errorf("commit existing postgres incident deletion: %w", err)
		}
		return existing, nil
	}
	if !errors.Is(err, incidents.ErrNotFound) {
		return incidents.IncidentDeletionStatus{}, err
	}

	var status string
	var ownerAccountID sql.NullString
	var updatedAt time.Time
	var deletionState string
	err = tx.QueryRowContext(ctx, `
		SELECT status, owner_account_id, updated_at, deletion_state
		FROM incidents
		WHERE id = $1
		FOR UPDATE`,
		params.IncidentID,
	).Scan(&status, &ownerAccountID, &updatedAt, &deletionState)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.IncidentDeletionStatus{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("read postgres incident for deletion: %w", err)
	}
	if params.RequireOwnerID != "" && (!ownerAccountID.Valid || ownerAccountID.String != params.RequireOwnerID) {
		return incidents.IncidentDeletionStatus{}, incidents.ErrNotFound
	}
	if deletionState != incidents.IncidentDeletionStateActive {
		return incidents.IncidentDeletionStatus{}, incidents.ErrInvalidState
	}
	if status == incidents.StatusOpen && !params.AllowOpen {
		return incidents.IncidentDeletionStatus{}, incidents.ErrInvalidState
	}
	if params.RetentionCutoff != nil {
		if status != incidents.StatusClosed || updatedAt.UTC().After(params.RetentionCutoff.UTC()) {
			return incidents.IncidentDeletionStatus{}, incidents.ErrInvalidState
		}
	}

	decisionID, err := newID("del")
	if err != nil {
		return incidents.IncidentDeletionStatus{}, err
	}
	statusResult := incidents.IncidentDeletionStatus{
		DecisionID:     decisionID,
		IncidentID:     params.IncidentID,
		Source:         params.Source,
		ReasonCode:     params.ReasonCode,
		ActorAccountID: params.ActorAccountID,
		AllowOpen:      params.AllowOpen,
		State:          incidents.IncidentDeletionStatePending,
		RequestedAt:    now,
		UpdatedAt:      now,
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO incident_deletion_decisions (
			id, incident_id, source, reason_code, actor_account_id, allow_open,
			state, item_count, requested_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 0, $8, $9)`,
		statusResult.DecisionID,
		statusResult.IncidentID,
		statusResult.Source,
		nullableString(statusResult.ReasonCode),
		nullableString(statusResult.ActorAccountID),
		statusResult.AllowOpen,
		statusResult.State,
		statusResult.RequestedAt,
		statusResult.UpdatedAt,
	); err != nil {
		if isIntegrityConstraint(err) {
			return incidents.IncidentDeletionStatus{}, incidents.ErrInvalidState
		}
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("insert postgres incident deletion decision: %w", err)
	}

	storedPaths, err := deletionStoredPaths(ctx, tx, params.IncidentID)
	if err != nil {
		return incidents.IncidentDeletionStatus{}, err
	}
	for _, storedPath := range storedPaths {
		itemID, idErr := newID("dli")
		if idErr != nil {
			return incidents.IncidentDeletionStatus{}, idErr
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO incident_deletion_items (
				id, decision_id, incident_id, stored_path, state, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			itemID,
			statusResult.DecisionID,
			statusResult.IncidentID,
			storedPath,
			incidents.IncidentDeletionItemStatePending,
			now,
			now,
		); err != nil {
			return incidents.IncidentDeletionStatus{}, fmt.Errorf("insert postgres incident deletion item: %w", err)
		}
	}
	statusResult.ItemCount = len(storedPaths)

	if _, err = tx.ExecContext(ctx, `
		UPDATE incident_deletion_decisions
		SET item_count = $1, updated_at = $2
		WHERE id = $3`,
		statusResult.ItemCount,
		now,
		statusResult.DecisionID,
	); err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("update postgres incident deletion item count: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE incidents
		SET deletion_state = $1, updated_at = $2
		WHERE id = $3`,
		incidents.IncidentDeletionStatePending,
		now,
		statusResult.IncidentID,
	); err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("mark postgres incident deletion pending: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("commit request postgres incident deletion: %w", err)
	}
	return statusResult, nil
}

// GetIncidentDeletionStatus returns the non-sensitive deletion decision status.
func (r *Repository) GetIncidentDeletionStatus(ctx context.Context, incidentID string) (incidents.IncidentDeletionStatus, error) {
	return getIncidentDeletionStatus(ctx, r.db, incidentID)
}

// QueueRetentionIncidentDeletions creates retention-policy deletion decisions for
// closed incidents older than cutoff. Open incidents are intentionally excluded.
func (r *Repository) QueueRetentionIncidentDeletions(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id
		FROM incidents
		WHERE status = $1 AND deletion_state = $2 AND updated_at <= $3
		ORDER BY updated_at ASC, id ASC
		LIMIT $4`,
		incidents.StatusClosed,
		incidents.IncidentDeletionStateActive,
		cutoff.UTC(),
		limit,
	)
	if err != nil {
		return 0, fmt.Errorf("select postgres retention deletion candidates: %w", err)
	}
	defer rows.Close()

	incidentIDs := []string{}
	for rows.Next() {
		var incidentID string
		if err := rows.Scan(&incidentID); err != nil {
			return 0, fmt.Errorf("scan postgres retention deletion candidate: %w", err)
		}
		incidentIDs = append(incidentIDs, incidentID)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate postgres retention deletion candidates: %w", err)
	}

	queued := 0
	for _, incidentID := range incidentIDs {
		if _, err := r.RequestIncidentDeletion(ctx, incidents.IncidentDeletionRequest{
			IncidentID:      incidentID,
			Source:          incidents.IncidentDeletionSourceRetentionPolicy,
			ReasonCode:      "closed_incident_retention",
			RetentionCutoff: &cutoff,
		}); err != nil {
			if errors.Is(err, incidents.ErrInvalidState) || errors.Is(err, incidents.ErrNotFound) {
				continue
			}
			return queued, err
		}
		queued++
	}
	return queued, nil
}

// ListRetentionDeletionCandidates previews closed incidents that retention
// policy would queue without creating deletion decisions.
func (r *Repository) ListRetentionDeletionCandidates(ctx context.Context, cutoff time.Time, limit int) ([]incidents.RetentionDeletionCandidate, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, updated_at
		FROM incidents
		WHERE status = $1 AND deletion_state = $2 AND updated_at <= $3
		ORDER BY updated_at ASC, id ASC
		LIMIT $4`,
		incidents.StatusClosed,
		incidents.IncidentDeletionStateActive,
		cutoff.UTC(),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("select postgres retention deletion preview candidates: %w", err)
	}
	defer rows.Close()

	candidates := []incidents.RetentionDeletionCandidate{}
	for rows.Next() {
		var candidate incidents.RetentionDeletionCandidate
		if err := rows.Scan(&candidate.IncidentID, &candidate.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan postgres retention deletion preview candidate: %w", err)
		}
		candidate.UpdatedAt = candidate.UpdatedAt.UTC()
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres retention deletion preview candidates: %w", err)
	}
	return candidates, nil
}

// GetIncidentDeletionJobStatus returns local-operator deletion job status using
// only safe aggregates and deletion decision fields.
func (r *Repository) GetIncidentDeletionJobStatus(ctx context.Context, limit int, staleDeletingBefore time.Time) (incidents.IncidentDeletionJobStatus, error) {
	status := incidents.IncidentDeletionJobStatus{}
	decisionCounts, err := r.listDeletionDecisionStateCounts(ctx)
	if err != nil {
		return status, err
	}
	status.DecisionStateCounts = decisionCounts
	decisionErrors, err := r.listDeletionDecisionErrorCounts(ctx)
	if err != nil {
		return status, err
	}
	status.DecisionErrorCounts = decisionErrors
	itemCounts, err := r.listDeletionItemStateCounts(ctx)
	if err != nil {
		return status, err
	}
	status.ItemStateCounts = itemCounts
	runnable, err := r.ListRunnableIncidentDeletions(ctx, limit, staleDeletingBefore)
	if err != nil {
		return status, err
	}
	status.RunnableJobs = runnable
	return status, nil
}

// ListRunnableIncidentDeletions returns retryable deletion decisions for the
// background worker.
func (r *Repository) ListRunnableIncidentDeletions(ctx context.Context, limit int, staleDeletingBefore time.Time) ([]incidents.IncidentDeletionStatus, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := r.db.QueryContext(ctx, deletionStatusSelect+`
		WHERE state IN ($1, $2)
			OR (state = $3 AND updated_at <= $4)
		ORDER BY updated_at ASC, id ASC
		LIMIT $5`,
		incidents.IncidentDeletionStatePending,
		incidents.IncidentDeletionStateFailed,
		incidents.IncidentDeletionStateDeleting,
		staleDeletingBefore.UTC(),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list runnable postgres incident deletions: %w", err)
	}
	defer rows.Close()
	return scanDeletionStatuses(rows)
}

// MarkIncidentDeletionDeleting claims one deletion decision for processing.
func (r *Repository) MarkIncidentDeletionDeleting(ctx context.Context, decisionID string, staleDeletingBefore time.Time) (incidents.IncidentDeletionStatus, error) {
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("begin mark postgres incident deleting: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		UPDATE incident_deletion_decisions
		SET state = $1, error_code = NULL, updated_at = $2, started_at = COALESCE(started_at, $3)
		WHERE id = $4
			AND (state IN ($5, $6)
				OR (state = $7 AND updated_at <= $8))`,
		incidents.IncidentDeletionStateDeleting,
		now,
		now,
		decisionID,
		incidents.IncidentDeletionStatePending,
		incidents.IncidentDeletionStateFailed,
		incidents.IncidentDeletionStateDeleting,
		staleDeletingBefore.UTC(),
	)
	if err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("mark postgres deletion decision deleting: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("mark postgres deletion decision deleting rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return incidents.IncidentDeletionStatus{}, incidents.ErrNotFound
	}

	var incidentID string
	if err = tx.QueryRowContext(ctx, `
		SELECT incident_id
		FROM incident_deletion_decisions
		WHERE id = $1`,
		decisionID,
	).Scan(&incidentID); err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("read postgres incident deletion decision: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE incidents
		SET deletion_state = $1, updated_at = $2
		WHERE id = $3`,
		incidents.IncidentDeletionStateDeleting,
		now,
		incidentID,
	); err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("mark postgres incident deleting: %w", err)
	}
	status, err := getIncidentDeletionStatusByIDTx(ctx, tx, decisionID)
	if err != nil {
		return incidents.IncidentDeletionStatus{}, err
	}
	if err = tx.Commit(); err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("commit mark postgres incident deleting: %w", err)
	}
	return status, nil
}

// ListIncidentDeletionItems returns pending or failed internal stored-path items.
func (r *Repository) ListIncidentDeletionItems(ctx context.Context, decisionID string) ([]incidents.IncidentDeletionItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, decision_id, incident_id, stored_path, state, attempts, error_code,
			created_at, updated_at, last_attempt_at, completed_at
		FROM incident_deletion_items
		WHERE decision_id = $1 AND state IN ($2, $3)
		ORDER BY created_at ASC, id ASC`,
		decisionID,
		incidents.IncidentDeletionItemStatePending,
		incidents.IncidentDeletionItemStateFailed,
	)
	if err != nil {
		return nil, fmt.Errorf("list postgres incident deletion items: %w", err)
	}
	defer rows.Close()
	return scanDeletionItems(rows)
}

// MarkIncidentDeletionItemDeleted records idempotent success for one stored path.
func (r *Repository) MarkIncidentDeletionItemDeleted(ctx context.Context, itemID string) error {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE incident_deletion_items
		SET state = $1, attempts = attempts + 1, error_code = NULL, updated_at = $2,
			last_attempt_at = $3, completed_at = $4
		WHERE id = $5`,
		incidents.IncidentDeletionItemStateDeleted,
		now,
		now,
		now,
		itemID,
	)
	if err != nil {
		return fmt.Errorf("mark postgres incident deletion item deleted: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark postgres incident deletion item rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return incidents.ErrNotFound
	}
	return nil
}

// MarkIncidentDeletionItemFailed records a retryable blob deletion failure.
func (r *Repository) MarkIncidentDeletionItemFailed(ctx context.Context, itemID, errorCode string) error {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE incident_deletion_items
		SET state = $1, attempts = attempts + 1, error_code = $2, updated_at = $3,
			last_attempt_at = $4
		WHERE id = $5`,
		incidents.IncidentDeletionItemStateFailed,
		nullableString(errorCode),
		now,
		now,
		itemID,
	)
	if err != nil {
		return fmt.Errorf("mark postgres incident deletion item failed: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark postgres incident deletion item failed rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return incidents.ErrNotFound
	}
	return nil
}

// CompleteIncidentDeletion prunes sensitive child metadata, removes internal
// stored-path retry items, and leaves a minimal incident tombstone.
func (r *Repository) CompleteIncidentDeletion(ctx context.Context, decisionID string) (incidents.IncidentDeletionStatus, error) {
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("begin complete postgres incident deletion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	status, err := getIncidentDeletionStatusByIDTx(ctx, tx, decisionID)
	if err != nil {
		return incidents.IncidentDeletionStatus{}, err
	}
	var remaining int
	if err = tx.QueryRowContext(ctx, `
		SELECT count(*)
		FROM incident_deletion_items
		WHERE decision_id = $1 AND state != $2`,
		decisionID,
		incidents.IncidentDeletionItemStateDeleted,
	).Scan(&remaining); err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("count remaining postgres deletion items: %w", err)
	}
	if remaining != 0 {
		return incidents.IncidentDeletionStatus{}, incidents.ErrInvalidState
	}

	for _, query := range []string{
		"DELETE FROM wrapped_key_records WHERE incident_id = $1",
		"DELETE FROM sharing_grants WHERE incident_id = $1",
		"DELETE FROM upload_operations WHERE incident_id = $1",
		"DELETE FROM incident_tokens WHERE incident_id = $1",
		"DELETE FROM checkins WHERE incident_id = $1",
		"DELETE FROM chunks WHERE incident_id = $1",
		"DELETE FROM media_streams WHERE incident_id = $1",
	} {
		if _, err = tx.ExecContext(ctx, query, status.IncidentID); err != nil {
			return incidents.IncidentDeletionStatus{}, fmt.Errorf("prune postgres incident deletion metadata: %w", err)
		}
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM incident_deletion_items WHERE decision_id = $1", decisionID); err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("prune postgres incident deletion items: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE incident_deletion_decisions
		SET state = $1, error_code = NULL, updated_at = $2, completed_at = $3
		WHERE id = $4`,
		incidents.IncidentDeletionStateDeleted,
		now,
		now,
		decisionID,
	); err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("mark postgres deletion decision deleted: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE incidents
		SET status = $1, updated_at = $2, client_label = NULL, notes = NULL,
			incident_mode = NULL, capture_profile = NULL, escalation_policy = NULL,
			sharing_state = NULL, deletion_state = $3
		WHERE id = $4`,
		incidents.StatusClosed,
		now,
		incidents.IncidentDeletionStateDeleted,
		status.IncidentID,
	); err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("mark postgres incident deleted: %w", err)
	}
	status, err = getIncidentDeletionStatusByIDTx(ctx, tx, decisionID)
	if err != nil {
		return incidents.IncidentDeletionStatus{}, err
	}
	if err = tx.Commit(); err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("commit complete postgres incident deletion: %w", err)
	}
	return status, nil
}

// FailIncidentDeletion marks a deletion decision retryable without exposing
// storage paths or backend details in the status.
func (r *Repository) FailIncidentDeletion(ctx context.Context, decisionID, errorCode string) (incidents.IncidentDeletionStatus, error) {
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("begin fail postgres incident deletion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	status, err := getIncidentDeletionStatusByIDTx(ctx, tx, decisionID)
	if err != nil {
		return incidents.IncidentDeletionStatus{}, err
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE incident_deletion_decisions
		SET state = $1, error_code = $2, updated_at = $3
		WHERE id = $4`,
		incidents.IncidentDeletionStateFailed,
		nullableString(errorCode),
		now,
		decisionID,
	); err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("mark postgres deletion decision failed: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE incidents
		SET deletion_state = $1, updated_at = $2
		WHERE id = $3`,
		incidents.IncidentDeletionStateFailed,
		now,
		status.IncidentID,
	); err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("mark postgres incident deletion failed: %w", err)
	}
	status, err = getIncidentDeletionStatusByIDTx(ctx, tx, decisionID)
	if err != nil {
		return incidents.IncidentDeletionStatus{}, err
	}
	if err = tx.Commit(); err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("commit fail postgres incident deletion: %w", err)
	}
	return status, nil
}

func deletionStoredPaths(ctx context.Context, tx *sql.Tx, incidentID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT stored_path
		FROM chunks
		WHERE incident_id = $1
		ORDER BY stored_path ASC`,
		incidentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list postgres deletion stored paths: %w", err)
	}
	defer rows.Close()

	paths := []string{}
	for rows.Next() {
		var storedPath string
		if err := rows.Scan(&storedPath); err != nil {
			return nil, fmt.Errorf("scan postgres deletion stored path: %w", err)
		}
		paths = append(paths, storedPath)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres deletion stored paths: %w", err)
	}
	return paths, nil
}

func getIncidentDeletionStatus(ctx context.Context, db *sql.DB, incidentID string) (incidents.IncidentDeletionStatus, error) {
	row := db.QueryRowContext(ctx, deletionStatusSelect+" WHERE incident_id = $1", incidentID)
	status, err := scanDeletionStatus(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.IncidentDeletionStatus{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("get postgres incident deletion status: %w", err)
	}
	return status, nil
}

func getIncidentDeletionStatusTx(ctx context.Context, tx *sql.Tx, incidentID string) (incidents.IncidentDeletionStatus, error) {
	row := tx.QueryRowContext(ctx, deletionStatusSelect+" WHERE incident_id = $1", incidentID)
	status, err := scanDeletionStatus(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.IncidentDeletionStatus{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("get postgres incident deletion status: %w", err)
	}
	return status, nil
}

func getIncidentDeletionStatusByIDTx(ctx context.Context, tx *sql.Tx, decisionID string) (incidents.IncidentDeletionStatus, error) {
	row := tx.QueryRowContext(ctx, deletionStatusSelect+" WHERE id = $1", decisionID)
	status, err := scanDeletionStatus(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.IncidentDeletionStatus{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.IncidentDeletionStatus{}, fmt.Errorf("get postgres incident deletion status by id: %w", err)
	}
	return status, nil
}

func requireIncidentOwnerTx(ctx context.Context, tx *sql.Tx, incidentID, accountID string) error {
	var ownerAccountID sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT owner_account_id
		FROM incidents
		WHERE id = $1`,
		incidentID,
	).Scan(&ownerAccountID)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read postgres incident owner: %w", err)
	}
	if !ownerAccountID.Valid || ownerAccountID.String != accountID {
		return incidents.ErrNotFound
	}
	return nil
}

func (r *Repository) listDeletionDecisionStateCounts(ctx context.Context) ([]incidents.IncidentDeletionStateCount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT state, count(*)
		FROM incident_deletion_decisions
		GROUP BY state
		ORDER BY state ASC`)
	if err != nil {
		return nil, fmt.Errorf("count postgres incident deletion decisions by state: %w", err)
	}
	defer rows.Close()

	counts := []incidents.IncidentDeletionStateCount{}
	for rows.Next() {
		var count incidents.IncidentDeletionStateCount
		if err := rows.Scan(&count.State, &count.Count); err != nil {
			return nil, fmt.Errorf("scan postgres incident deletion decision state count: %w", err)
		}
		counts = append(counts, count)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres incident deletion decision state counts: %w", err)
	}
	return counts, nil
}

func (r *Repository) listDeletionDecisionErrorCounts(ctx context.Context) ([]incidents.IncidentDeletionErrorCount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT state, error_code, count(*)
		FROM incident_deletion_decisions
		WHERE error_code IS NOT NULL AND error_code != ''
		GROUP BY state, error_code
		ORDER BY state ASC, error_code ASC`)
	if err != nil {
		return nil, fmt.Errorf("count postgres incident deletion decisions by error: %w", err)
	}
	defer rows.Close()

	counts := []incidents.IncidentDeletionErrorCount{}
	for rows.Next() {
		var count incidents.IncidentDeletionErrorCount
		if err := rows.Scan(&count.State, &count.ErrorCode, &count.Count); err != nil {
			return nil, fmt.Errorf("scan postgres incident deletion decision error count: %w", err)
		}
		counts = append(counts, count)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres incident deletion decision error counts: %w", err)
	}
	return counts, nil
}

func (r *Repository) listDeletionItemStateCounts(ctx context.Context) ([]incidents.IncidentDeletionItemStateCount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT state, COALESCE(error_code, ''), count(*)
		FROM incident_deletion_items
		GROUP BY state, error_code
		ORDER BY state ASC, error_code ASC`)
	if err != nil {
		return nil, fmt.Errorf("count postgres incident deletion items by state: %w", err)
	}
	defer rows.Close()

	counts := []incidents.IncidentDeletionItemStateCount{}
	for rows.Next() {
		var count incidents.IncidentDeletionItemStateCount
		if err := rows.Scan(&count.State, &count.ErrorCode, &count.Count); err != nil {
			return nil, fmt.Errorf("scan postgres incident deletion item state count: %w", err)
		}
		counts = append(counts, count)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres incident deletion item state counts: %w", err)
	}
	return counts, nil
}

func scanDeletionStatuses(rows *sql.Rows) ([]incidents.IncidentDeletionStatus, error) {
	statuses := []incidents.IncidentDeletionStatus{}
	for rows.Next() {
		status, err := scanDeletionStatus(rows)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres deletion statuses: %w", err)
	}
	return statuses, nil
}

func scanDeletionStatus(s scanner) (incidents.IncidentDeletionStatus, error) {
	var status incidents.IncidentDeletionStatus
	var reasonCode sql.NullString
	var actorAccountID sql.NullString
	var errorCode sql.NullString
	var startedAt sql.NullTime
	var completedAt sql.NullTime
	if err := s.Scan(
		&status.DecisionID,
		&status.IncidentID,
		&status.Source,
		&reasonCode,
		&actorAccountID,
		&status.AllowOpen,
		&status.State,
		&status.ItemCount,
		&errorCode,
		&status.RequestedAt,
		&status.UpdatedAt,
		&startedAt,
		&completedAt,
	); err != nil {
		return incidents.IncidentDeletionStatus{}, err
	}
	status.RequestedAt = status.RequestedAt.UTC()
	status.UpdatedAt = status.UpdatedAt.UTC()
	status.StartedAt = nullableDBTime(startedAt)
	status.CompletedAt = nullableDBTime(completedAt)
	if reasonCode.Valid {
		status.ReasonCode = reasonCode.String
	}
	if actorAccountID.Valid {
		status.ActorAccountID = actorAccountID.String
	}
	if errorCode.Valid {
		status.ErrorCode = errorCode.String
	}
	return status, nil
}

func scanDeletionItems(rows *sql.Rows) ([]incidents.IncidentDeletionItem, error) {
	items := []incidents.IncidentDeletionItem{}
	for rows.Next() {
		item, err := scanDeletionItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres deletion items: %w", err)
	}
	return items, nil
}

func scanDeletionItem(s scanner) (incidents.IncidentDeletionItem, error) {
	var item incidents.IncidentDeletionItem
	var errorCode sql.NullString
	var lastAttemptAt sql.NullTime
	var completedAt sql.NullTime
	if err := s.Scan(
		&item.ID,
		&item.DecisionID,
		&item.IncidentID,
		&item.StoredPath,
		&item.State,
		&item.Attempts,
		&errorCode,
		&item.CreatedAt,
		&item.UpdatedAt,
		&lastAttemptAt,
		&completedAt,
	); err != nil {
		return incidents.IncidentDeletionItem{}, err
	}
	item.CreatedAt = item.CreatedAt.UTC()
	item.UpdatedAt = item.UpdatedAt.UTC()
	item.LastAttemptAt = nullableDBTime(lastAttemptAt)
	item.CompletedAt = nullableDBTime(completedAt)
	if errorCode.Valid {
		item.ErrorCode = errorCode.String
	}
	return item, nil
}

func validDeletionSource(source string) bool {
	switch source {
	case incidents.IncidentDeletionSourceAccountRequest, incidents.IncidentDeletionSourceAdminRequest, incidents.IncidentDeletionSourceRetentionPolicy:
		return true
	default:
		return false
	}
}
