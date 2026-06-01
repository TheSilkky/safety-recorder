package incidents

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const deletionStatusSelect = `
	SELECT id, incident_id, source, reason_code, actor_account_id, allow_open,
		state, item_count, error_code, requested_at, updated_at, started_at, completed_at
	FROM incident_deletion_decisions`

// RequestIncidentDeletion creates or returns a durable deletion decision for one
// incident. Stored paths are snapshotted from chunk metadata inside the same
// transaction before the incident is marked deletion-pending.
func (r *Repository) RequestIncidentDeletion(ctx context.Context, params IncidentDeletionRequest) (IncidentDeletionStatus, error) {
	if !validDeletionSource(params.Source) {
		return IncidentDeletionStatus{}, ErrInvalidState
	}
	now := time.Now().UTC()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("begin request incident deletion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	existing, err := getIncidentDeletionStatusTx(ctx, tx, params.IncidentID)
	if err == nil {
		if params.RequireOwnerID != "" {
			if err := requireIncidentOwnerTx(ctx, tx, params.IncidentID, params.RequireOwnerID); err != nil {
				return IncidentDeletionStatus{}, err
			}
		}
		if commitErr := tx.Commit(); commitErr != nil {
			return IncidentDeletionStatus{}, fmt.Errorf("commit existing incident deletion: %w", commitErr)
		}
		return existing, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return IncidentDeletionStatus{}, err
	}

	var status string
	var ownerAccountID sql.NullString
	var updatedAt string
	var deletionState string
	err = tx.QueryRowContext(ctx, `
		SELECT status, owner_account_id, updated_at, deletion_state
		FROM incidents
		WHERE id = ?`,
		params.IncidentID,
	).Scan(&status, &ownerAccountID, &updatedAt, &deletionState)
	if errors.Is(err, sql.ErrNoRows) {
		return IncidentDeletionStatus{}, ErrNotFound
	}
	if err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("read incident for deletion: %w", err)
	}
	if params.RequireOwnerID != "" && (!ownerAccountID.Valid || ownerAccountID.String != params.RequireOwnerID) {
		return IncidentDeletionStatus{}, ErrNotFound
	}
	if deletionState != IncidentDeletionStateActive {
		return IncidentDeletionStatus{}, ErrInvalidState
	}
	if status == StatusOpen && !params.AllowOpen {
		return IncidentDeletionStatus{}, ErrInvalidState
	}
	if params.RetentionCutoff != nil {
		parsedUpdatedAt, parseErr := parseDBTime(updatedAt)
		if parseErr != nil {
			return IncidentDeletionStatus{}, parseErr
		}
		if status != StatusClosed || parsedUpdatedAt.After(params.RetentionCutoff.UTC()) {
			return IncidentDeletionStatus{}, ErrInvalidState
		}
	}

	decisionID, err := newID("del")
	if err != nil {
		return IncidentDeletionStatus{}, err
	}
	statusResult := IncidentDeletionStatus{
		DecisionID:     decisionID,
		IncidentID:     params.IncidentID,
		Source:         params.Source,
		ReasonCode:     params.ReasonCode,
		ActorAccountID: params.ActorAccountID,
		AllowOpen:      params.AllowOpen,
		State:          IncidentDeletionStatePending,
		RequestedAt:    now,
		UpdatedAt:      now,
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO incident_deletion_decisions (
			id, incident_id, source, reason_code, actor_account_id, allow_open,
			state, item_count, requested_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		statusResult.DecisionID,
		statusResult.IncidentID,
		statusResult.Source,
		nullableString(statusResult.ReasonCode),
		nullableString(statusResult.ActorAccountID),
		boolInt(statusResult.AllowOpen),
		statusResult.State,
		formatDBTime(statusResult.RequestedAt),
		formatDBTime(statusResult.UpdatedAt),
	); err != nil {
		if isConstraint(err) {
			return IncidentDeletionStatus{}, ErrInvalidState
		}
		return IncidentDeletionStatus{}, fmt.Errorf("insert incident deletion decision: %w", err)
	}

	storedPaths, err := deletionStoredPaths(ctx, tx, params.IncidentID)
	if err != nil {
		return IncidentDeletionStatus{}, err
	}
	for _, storedPath := range storedPaths {
		itemID, idErr := newID("dli")
		if idErr != nil {
			return IncidentDeletionStatus{}, idErr
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO incident_deletion_items (
				id, decision_id, incident_id, stored_path, state, created_at, updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			itemID,
			statusResult.DecisionID,
			statusResult.IncidentID,
			storedPath,
			IncidentDeletionItemStatePending,
			formatDBTime(now),
			formatDBTime(now),
		); err != nil {
			return IncidentDeletionStatus{}, fmt.Errorf("insert incident deletion item: %w", err)
		}
	}
	statusResult.ItemCount = len(storedPaths)

	if _, err = tx.ExecContext(ctx, `
		UPDATE incident_deletion_decisions
		SET item_count = ?, updated_at = ?
		WHERE id = ?`,
		statusResult.ItemCount,
		formatDBTime(now),
		statusResult.DecisionID,
	); err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("update incident deletion item count: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE incidents
		SET deletion_state = ?, updated_at = ?
		WHERE id = ?`,
		IncidentDeletionStatePending,
		formatDBTime(now),
		statusResult.IncidentID,
	); err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("mark incident deletion pending: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("commit request incident deletion: %w", err)
	}
	return statusResult, nil
}

// GetIncidentDeletionStatus returns the non-sensitive deletion decision status.
func (r *Repository) GetIncidentDeletionStatus(ctx context.Context, incidentID string) (IncidentDeletionStatus, error) {
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
		WHERE status = ? AND deletion_state = ? AND updated_at <= ?
		ORDER BY updated_at ASC, id ASC
		LIMIT ?`,
		StatusClosed,
		IncidentDeletionStateActive,
		formatDBTime(cutoff.UTC()),
		limit,
	)
	if err != nil {
		return 0, fmt.Errorf("select retention deletion candidates: %w", err)
	}
	defer rows.Close()

	incidentIDs := []string{}
	for rows.Next() {
		var incidentID string
		if err := rows.Scan(&incidentID); err != nil {
			return 0, fmt.Errorf("scan retention deletion candidate: %w", err)
		}
		incidentIDs = append(incidentIDs, incidentID)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate retention deletion candidates: %w", err)
	}

	queued := 0
	for _, incidentID := range incidentIDs {
		if _, err := r.RequestIncidentDeletion(ctx, IncidentDeletionRequest{
			IncidentID:      incidentID,
			Source:          IncidentDeletionSourceRetentionPolicy,
			ReasonCode:      "closed_incident_retention",
			RetentionCutoff: &cutoff,
		}); err != nil {
			if errors.Is(err, ErrInvalidState) || errors.Is(err, ErrNotFound) {
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
func (r *Repository) ListRetentionDeletionCandidates(ctx context.Context, cutoff time.Time, limit int) ([]RetentionDeletionCandidate, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, updated_at
		FROM incidents
		WHERE status = ? AND deletion_state = ? AND updated_at <= ?
		ORDER BY updated_at ASC, id ASC
		LIMIT ?`,
		StatusClosed,
		IncidentDeletionStateActive,
		formatDBTime(cutoff.UTC()),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("select retention deletion preview candidates: %w", err)
	}
	defer rows.Close()

	candidates := []RetentionDeletionCandidate{}
	for rows.Next() {
		var candidate RetentionDeletionCandidate
		var updatedAt string
		if err := rows.Scan(&candidate.IncidentID, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan retention deletion preview candidate: %w", err)
		}
		parsedUpdatedAt, err := parseDBTime(updatedAt)
		if err != nil {
			return nil, err
		}
		candidate.UpdatedAt = parsedUpdatedAt
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate retention deletion preview candidates: %w", err)
	}
	return candidates, nil
}

// GetIncidentDeletionJobStatus returns local-operator deletion job status using
// only safe aggregates and deletion decision fields.
func (r *Repository) GetIncidentDeletionJobStatus(ctx context.Context, limit int, staleDeletingBefore time.Time) (IncidentDeletionJobStatus, error) {
	status := IncidentDeletionJobStatus{}
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
func (r *Repository) ListRunnableIncidentDeletions(ctx context.Context, limit int, staleDeletingBefore time.Time) ([]IncidentDeletionStatus, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := r.db.QueryContext(ctx, deletionStatusSelect+`
		WHERE state IN (?, ?)
			OR (state = ? AND updated_at <= ?)
		ORDER BY updated_at ASC, id ASC
		LIMIT ?`,
		IncidentDeletionStatePending,
		IncidentDeletionStateFailed,
		IncidentDeletionStateDeleting,
		formatDBTime(staleDeletingBefore.UTC()),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list runnable incident deletions: %w", err)
	}
	defer rows.Close()
	return scanDeletionStatuses(rows)
}

// MarkIncidentDeletionDeleting claims one deletion decision for processing.
func (r *Repository) MarkIncidentDeletionDeleting(ctx context.Context, decisionID string, staleDeletingBefore time.Time) (IncidentDeletionStatus, error) {
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("begin mark incident deleting: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		UPDATE incident_deletion_decisions
		SET state = ?, error_code = NULL, updated_at = ?, started_at = COALESCE(started_at, ?)
		WHERE id = ?
			AND (state IN (?, ?)
				OR (state = ? AND updated_at <= ?))`,
		IncidentDeletionStateDeleting,
		formatDBTime(now),
		formatDBTime(now),
		decisionID,
		IncidentDeletionStatePending,
		IncidentDeletionStateFailed,
		IncidentDeletionStateDeleting,
		formatDBTime(staleDeletingBefore.UTC()),
	)
	if err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("mark deletion decision deleting: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("mark deletion decision deleting rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return IncidentDeletionStatus{}, ErrNotFound
	}

	var incidentID string
	if err = tx.QueryRowContext(ctx, `
		SELECT incident_id
		FROM incident_deletion_decisions
		WHERE id = ?`,
		decisionID,
	).Scan(&incidentID); err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("read incident deletion decision: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE incidents
		SET deletion_state = ?, updated_at = ?
		WHERE id = ?`,
		IncidentDeletionStateDeleting,
		formatDBTime(now),
		incidentID,
	); err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("mark incident deleting: %w", err)
	}
	status, err := getIncidentDeletionStatusByIDTx(ctx, tx, decisionID)
	if err != nil {
		return IncidentDeletionStatus{}, err
	}
	if err = tx.Commit(); err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("commit mark incident deleting: %w", err)
	}
	return status, nil
}

// ListIncidentDeletionItems returns pending or failed internal stored-path items.
func (r *Repository) ListIncidentDeletionItems(ctx context.Context, decisionID string) ([]IncidentDeletionItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, decision_id, incident_id, stored_path, state, attempts, error_code,
			created_at, updated_at, last_attempt_at, completed_at
		FROM incident_deletion_items
		WHERE decision_id = ? AND state IN (?, ?)
		ORDER BY created_at ASC, id ASC`,
		decisionID,
		IncidentDeletionItemStatePending,
		IncidentDeletionItemStateFailed,
	)
	if err != nil {
		return nil, fmt.Errorf("list incident deletion items: %w", err)
	}
	defer rows.Close()
	return scanDeletionItems(rows)
}

// MarkIncidentDeletionItemDeleted records idempotent success for one stored path.
func (r *Repository) MarkIncidentDeletionItemDeleted(ctx context.Context, itemID string) error {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE incident_deletion_items
		SET state = ?, attempts = attempts + 1, error_code = NULL, updated_at = ?,
			last_attempt_at = ?, completed_at = ?
		WHERE id = ?`,
		IncidentDeletionItemStateDeleted,
		formatDBTime(now),
		formatDBTime(now),
		formatDBTime(now),
		itemID,
	)
	if err != nil {
		return fmt.Errorf("mark incident deletion item deleted: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark incident deletion item rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkIncidentDeletionItemFailed records a retryable blob deletion failure.
func (r *Repository) MarkIncidentDeletionItemFailed(ctx context.Context, itemID, errorCode string) error {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE incident_deletion_items
		SET state = ?, attempts = attempts + 1, error_code = ?, updated_at = ?,
			last_attempt_at = ?
		WHERE id = ?`,
		IncidentDeletionItemStateFailed,
		nullableString(errorCode),
		formatDBTime(now),
		formatDBTime(now),
		itemID,
	)
	if err != nil {
		return fmt.Errorf("mark incident deletion item failed: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark incident deletion item failed rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// CompleteIncidentDeletion prunes sensitive child metadata, removes internal
// stored-path retry items, and leaves a minimal incident tombstone.
func (r *Repository) CompleteIncidentDeletion(ctx context.Context, decisionID string) (IncidentDeletionStatus, error) {
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("begin complete incident deletion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	status, err := getIncidentDeletionStatusByIDTx(ctx, tx, decisionID)
	if err != nil {
		return IncidentDeletionStatus{}, err
	}
	var remaining int
	if err = tx.QueryRowContext(ctx, `
		SELECT count(*)
		FROM incident_deletion_items
		WHERE decision_id = ? AND state != ?`,
		decisionID,
		IncidentDeletionItemStateDeleted,
	).Scan(&remaining); err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("count remaining deletion items: %w", err)
	}
	if remaining != 0 {
		return IncidentDeletionStatus{}, ErrInvalidState
	}

	for _, query := range []string{
		"DELETE FROM wrapped_key_records WHERE incident_id = ?",
		"DELETE FROM sharing_grants WHERE incident_id = ?",
		"DELETE FROM upload_operations WHERE incident_id = ?",
		"DELETE FROM incident_tokens WHERE incident_id = ?",
		"DELETE FROM checkins WHERE incident_id = ?",
		"DELETE FROM chunks WHERE incident_id = ?",
		"DELETE FROM media_streams WHERE incident_id = ?",
	} {
		if _, err = tx.ExecContext(ctx, query, status.IncidentID); err != nil {
			return IncidentDeletionStatus{}, fmt.Errorf("prune incident deletion metadata: %w", err)
		}
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM incident_deletion_items WHERE decision_id = ?", decisionID); err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("prune incident deletion items: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE incident_deletion_decisions
		SET state = ?, error_code = NULL, updated_at = ?, completed_at = ?
		WHERE id = ?`,
		IncidentDeletionStateDeleted,
		formatDBTime(now),
		formatDBTime(now),
		decisionID,
	); err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("mark deletion decision deleted: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE incidents
		SET status = ?, updated_at = ?, client_label = NULL, notes = NULL,
			incident_mode = NULL, capture_profile = NULL, escalation_policy = NULL,
			sharing_state = NULL, deletion_state = ?
		WHERE id = ?`,
		StatusClosed,
		formatDBTime(now),
		IncidentDeletionStateDeleted,
		status.IncidentID,
	); err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("mark incident deleted: %w", err)
	}
	status, err = getIncidentDeletionStatusByIDTx(ctx, tx, decisionID)
	if err != nil {
		return IncidentDeletionStatus{}, err
	}
	if err = tx.Commit(); err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("commit complete incident deletion: %w", err)
	}
	return status, nil
}

// FailIncidentDeletion marks a deletion decision retryable without exposing
// storage paths or backend details in the status.
func (r *Repository) FailIncidentDeletion(ctx context.Context, decisionID, errorCode string) (IncidentDeletionStatus, error) {
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("begin fail incident deletion: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	status, err := getIncidentDeletionStatusByIDTx(ctx, tx, decisionID)
	if err != nil {
		return IncidentDeletionStatus{}, err
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE incident_deletion_decisions
		SET state = ?, error_code = ?, updated_at = ?
		WHERE id = ?`,
		IncidentDeletionStateFailed,
		nullableString(errorCode),
		formatDBTime(now),
		decisionID,
	); err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("mark deletion decision failed: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE incidents
		SET deletion_state = ?, updated_at = ?
		WHERE id = ?`,
		IncidentDeletionStateFailed,
		formatDBTime(now),
		status.IncidentID,
	); err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("mark incident deletion failed: %w", err)
	}
	status, err = getIncidentDeletionStatusByIDTx(ctx, tx, decisionID)
	if err != nil {
		return IncidentDeletionStatus{}, err
	}
	if err = tx.Commit(); err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("commit fail incident deletion: %w", err)
	}
	return status, nil
}

func deletionStoredPaths(ctx context.Context, tx *sql.Tx, incidentID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT stored_path
		FROM chunks
		WHERE incident_id = ?
		ORDER BY stored_path ASC`,
		incidentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list deletion stored paths: %w", err)
	}
	defer rows.Close()

	paths := []string{}
	for rows.Next() {
		var storedPath string
		if err := rows.Scan(&storedPath); err != nil {
			return nil, fmt.Errorf("scan deletion stored path: %w", err)
		}
		paths = append(paths, storedPath)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deletion stored paths: %w", err)
	}
	return paths, nil
}

func getIncidentDeletionStatus(ctx context.Context, db *sql.DB, incidentID string) (IncidentDeletionStatus, error) {
	row := db.QueryRowContext(ctx, deletionStatusSelect+" WHERE incident_id = ?", incidentID)
	status, err := scanDeletionStatus(row)
	if errors.Is(err, sql.ErrNoRows) {
		return IncidentDeletionStatus{}, ErrNotFound
	}
	if err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("get incident deletion status: %w", err)
	}
	return status, nil
}

func getIncidentDeletionStatusTx(ctx context.Context, tx *sql.Tx, incidentID string) (IncidentDeletionStatus, error) {
	row := tx.QueryRowContext(ctx, deletionStatusSelect+" WHERE incident_id = ?", incidentID)
	status, err := scanDeletionStatus(row)
	if errors.Is(err, sql.ErrNoRows) {
		return IncidentDeletionStatus{}, ErrNotFound
	}
	if err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("get incident deletion status: %w", err)
	}
	return status, nil
}

func getIncidentDeletionStatusByIDTx(ctx context.Context, tx *sql.Tx, decisionID string) (IncidentDeletionStatus, error) {
	row := tx.QueryRowContext(ctx, deletionStatusSelect+" WHERE id = ?", decisionID)
	status, err := scanDeletionStatus(row)
	if errors.Is(err, sql.ErrNoRows) {
		return IncidentDeletionStatus{}, ErrNotFound
	}
	if err != nil {
		return IncidentDeletionStatus{}, fmt.Errorf("get incident deletion status by id: %w", err)
	}
	return status, nil
}

func requireIncidentOwnerTx(ctx context.Context, tx *sql.Tx, incidentID, accountID string) error {
	var ownerAccountID sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT owner_account_id
		FROM incidents
		WHERE id = ?`,
		incidentID,
	).Scan(&ownerAccountID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read incident owner: %w", err)
	}
	if !ownerAccountID.Valid || ownerAccountID.String != accountID {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) listDeletionDecisionStateCounts(ctx context.Context) ([]IncidentDeletionStateCount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT state, count(*)
		FROM incident_deletion_decisions
		GROUP BY state
		ORDER BY state ASC`)
	if err != nil {
		return nil, fmt.Errorf("count incident deletion decisions by state: %w", err)
	}
	defer rows.Close()

	counts := []IncidentDeletionStateCount{}
	for rows.Next() {
		var count IncidentDeletionStateCount
		if err := rows.Scan(&count.State, &count.Count); err != nil {
			return nil, fmt.Errorf("scan incident deletion decision state count: %w", err)
		}
		counts = append(counts, count)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate incident deletion decision state counts: %w", err)
	}
	return counts, nil
}

func (r *Repository) listDeletionDecisionErrorCounts(ctx context.Context) ([]IncidentDeletionErrorCount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT state, error_code, count(*)
		FROM incident_deletion_decisions
		WHERE error_code IS NOT NULL AND error_code != ''
		GROUP BY state, error_code
		ORDER BY state ASC, error_code ASC`)
	if err != nil {
		return nil, fmt.Errorf("count incident deletion decisions by error: %w", err)
	}
	defer rows.Close()

	counts := []IncidentDeletionErrorCount{}
	for rows.Next() {
		var count IncidentDeletionErrorCount
		if err := rows.Scan(&count.State, &count.ErrorCode, &count.Count); err != nil {
			return nil, fmt.Errorf("scan incident deletion decision error count: %w", err)
		}
		counts = append(counts, count)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate incident deletion decision error counts: %w", err)
	}
	return counts, nil
}

func (r *Repository) listDeletionItemStateCounts(ctx context.Context) ([]IncidentDeletionItemStateCount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT state, COALESCE(error_code, ''), count(*)
		FROM incident_deletion_items
		GROUP BY state, error_code
		ORDER BY state ASC, error_code ASC`)
	if err != nil {
		return nil, fmt.Errorf("count incident deletion items by state: %w", err)
	}
	defer rows.Close()

	counts := []IncidentDeletionItemStateCount{}
	for rows.Next() {
		var count IncidentDeletionItemStateCount
		if err := rows.Scan(&count.State, &count.ErrorCode, &count.Count); err != nil {
			return nil, fmt.Errorf("scan incident deletion item state count: %w", err)
		}
		counts = append(counts, count)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate incident deletion item state counts: %w", err)
	}
	return counts, nil
}

func scanDeletionStatuses(rows *sql.Rows) ([]IncidentDeletionStatus, error) {
	statuses := []IncidentDeletionStatus{}
	for rows.Next() {
		status, err := scanDeletionStatus(rows)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deletion statuses: %w", err)
	}
	return statuses, nil
}

func scanDeletionStatus(s scanner) (IncidentDeletionStatus, error) {
	var status IncidentDeletionStatus
	var reasonCode sql.NullString
	var actorAccountID sql.NullString
	var allowOpen int
	var errorCode sql.NullString
	var requestedAt string
	var updatedAt string
	var startedAt sql.NullString
	var completedAt sql.NullString
	if err := s.Scan(
		&status.DecisionID,
		&status.IncidentID,
		&status.Source,
		&reasonCode,
		&actorAccountID,
		&allowOpen,
		&status.State,
		&status.ItemCount,
		&errorCode,
		&requestedAt,
		&updatedAt,
		&startedAt,
		&completedAt,
	); err != nil {
		return IncidentDeletionStatus{}, err
	}
	parsedRequestedAt, err := parseDBTime(requestedAt)
	if err != nil {
		return IncidentDeletionStatus{}, err
	}
	parsedUpdatedAt, err := parseDBTime(updatedAt)
	if err != nil {
		return IncidentDeletionStatus{}, err
	}
	parsedStartedAt, err := nullableDBTime(startedAt)
	if err != nil {
		return IncidentDeletionStatus{}, err
	}
	parsedCompletedAt, err := nullableDBTime(completedAt)
	if err != nil {
		return IncidentDeletionStatus{}, err
	}
	status.RequestedAt = parsedRequestedAt
	status.UpdatedAt = parsedUpdatedAt
	status.StartedAt = parsedStartedAt
	status.CompletedAt = parsedCompletedAt
	status.AllowOpen = allowOpen == 1
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

func scanDeletionItems(rows *sql.Rows) ([]IncidentDeletionItem, error) {
	items := []IncidentDeletionItem{}
	for rows.Next() {
		item, err := scanDeletionItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deletion items: %w", err)
	}
	return items, nil
}

func scanDeletionItem(s scanner) (IncidentDeletionItem, error) {
	var item IncidentDeletionItem
	var errorCode sql.NullString
	var createdAt string
	var updatedAt string
	var lastAttemptAt sql.NullString
	var completedAt sql.NullString
	if err := s.Scan(
		&item.ID,
		&item.DecisionID,
		&item.IncidentID,
		&item.StoredPath,
		&item.State,
		&item.Attempts,
		&errorCode,
		&createdAt,
		&updatedAt,
		&lastAttemptAt,
		&completedAt,
	); err != nil {
		return IncidentDeletionItem{}, err
	}
	parsedCreatedAt, err := parseDBTime(createdAt)
	if err != nil {
		return IncidentDeletionItem{}, err
	}
	parsedUpdatedAt, err := parseDBTime(updatedAt)
	if err != nil {
		return IncidentDeletionItem{}, err
	}
	parsedLastAttemptAt, err := nullableDBTime(lastAttemptAt)
	if err != nil {
		return IncidentDeletionItem{}, err
	}
	parsedCompletedAt, err := nullableDBTime(completedAt)
	if err != nil {
		return IncidentDeletionItem{}, err
	}
	item.CreatedAt = parsedCreatedAt
	item.UpdatedAt = parsedUpdatedAt
	item.LastAttemptAt = parsedLastAttemptAt
	item.CompletedAt = parsedCompletedAt
	if errorCode.Valid {
		item.ErrorCode = errorCode.String
	}
	return item, nil
}

func validDeletionSource(source string) bool {
	switch source {
	case IncidentDeletionSourceAccountRequest, IncidentDeletionSourceAdminRequest, IncidentDeletionSourceRetentionPolicy:
		return true
	default:
		return false
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
