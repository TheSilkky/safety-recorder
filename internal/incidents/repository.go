package incidents

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrNotFound indicates that a requested incident, chunk, or related row
	// does not exist.
	ErrNotFound = errors.New("not found")
	// ErrDuplicate indicates that metadata storage rejected a duplicate chunk identity.
	ErrDuplicate = errors.New("duplicate")
	// ErrIdempotencyConflict indicates that an idempotency key was reused with
	// different immutable upload inputs.
	ErrIdempotencyConflict = errors.New("idempotency conflict")
	// ErrInvalidState indicates that a requested state transition is not
	// allowed for the current stream state.
	ErrInvalidState = errors.New("invalid state")
	// ErrIncidentClosed indicates that a chunk insert raced with an incident
	// close and the incident no longer accepts uploads.
	ErrIncidentClosed = errors.New("incident closed")
	// ErrIncidentDeleting indicates that a write raced with an incident deletion
	// decision and the incident no longer accepts metadata or ciphertext writes.
	ErrIncidentDeleting = errors.New("incident deleting")
)

// Repository stores incident metadata and related rows in SQLite.
type Repository struct {
	db *sql.DB
}

// NewRepository wraps db with incident-specific query methods.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Check verifies that the SQLite metadata handle is reachable.
func (r *Repository) Check(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func requireActiveIncidentTx(ctx context.Context, tx *sql.Tx, incidentID string) error {
	var deletionState string
	err := tx.QueryRowContext(ctx, `
		SELECT deletion_state
		FROM incidents
		WHERE id = ?`,
		incidentID,
	).Scan(&deletionState)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read incident deletion state: %w", err)
	}
	if deletionState != IncidentDeletionStateActive {
		return ErrIncidentDeleting
	}
	return nil
}

// CreateIncident inserts a new open legacy incident without an owner account.
func (r *Repository) CreateIncident(ctx context.Context, clientLabel, notes string) (Incident, error) {
	return r.CreateIncidentForAccount(ctx, "", CreateIncidentParams{
		ClientLabel: clientLabel,
		Notes:       notes,
	})
}

// CreateIncidentForAccount inserts a new open incident owned by accountID.
func (r *Repository) CreateIncidentForAccount(ctx context.Context, accountID string, params CreateIncidentParams) (Incident, error) {
	now := time.Now().UTC()
	id, err := newID("inc")
	if err != nil {
		return Incident{}, err
	}
	incident := Incident{
		ID:               id,
		OwnerAccountID:   accountID,
		CreatedAt:        now,
		UpdatedAt:        now,
		Status:           StatusOpen,
		ClientLabel:      params.ClientLabel,
		Notes:            params.Notes,
		IncidentMode:     params.IncidentMode,
		CaptureProfile:   params.CaptureProfile,
		EscalationPolicy: params.EscalationPolicy,
		SharingState:     params.SharingState,
		DeletionState:    IncidentDeletionStateActive,
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO incidents (
			id, owner_account_id, created_at, updated_at, status, client_label, notes,
			incident_mode, capture_profile, escalation_policy, sharing_state
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		incident.ID,
		nullableString(incident.OwnerAccountID),
		formatDBTime(incident.CreatedAt),
		formatDBTime(incident.UpdatedAt),
		incident.Status,
		nullableString(incident.ClientLabel),
		nullableString(incident.Notes),
		nullableString(incident.IncidentMode),
		nullableString(incident.CaptureProfile),
		nullableString(incident.EscalationPolicy),
		nullableString(incident.SharingState),
	)
	if err != nil {
		if isConstraint(err) {
			return Incident{}, ErrNotFound
		}
		return Incident{}, fmt.Errorf("insert incident: %w", err)
	}

	return incident, nil
}

// GetIncident returns one incident by ID.
func (r *Repository) GetIncident(ctx context.Context, id string) (Incident, error) {
	row := r.db.QueryRowContext(ctx, `
			SELECT id, owner_account_id, created_at, updated_at, status, client_label, notes,
				incident_mode, capture_profile, escalation_policy, sharing_state, deletion_state
			FROM incidents
			WHERE id = ?`, id)

	incident, err := scanIncident(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Incident{}, ErrNotFound
	}
	if err != nil {
		return Incident{}, fmt.Errorf("get incident: %w", err)
	}
	return incident, nil
}

// ListIncidentsForAccount returns non-deleted incidents owned by accountID.
func (r *Repository) ListIncidentsForAccount(ctx context.Context, accountID string) ([]Incident, error) {
	rows, err := r.db.QueryContext(ctx, `
			SELECT id, owner_account_id, created_at, updated_at, status, client_label, notes,
				incident_mode, capture_profile, escalation_policy, sharing_state, deletion_state
			FROM incidents
			WHERE owner_account_id = ? AND deletion_state <> ?
			ORDER BY updated_at DESC, created_at DESC, id ASC`,
		accountID,
		IncidentDeletionStateDeleted,
	)
	if err != nil {
		return nil, fmt.Errorf("list account incidents: %w", err)
	}
	defer rows.Close()

	incidents, err := scanIncidents(rows)
	if err != nil {
		return nil, fmt.Errorf("scan account incidents: %w", err)
	}
	return incidents, nil
}

// GetIncidentDetail returns an incident with its chunk and checkin metadata.
func (r *Repository) GetIncidentDetail(ctx context.Context, id string) (IncidentDetail, error) {
	incident, err := r.GetIncident(ctx, id)
	if err != nil {
		return IncidentDetail{}, err
	}
	streams, err := r.ListMediaStreams(ctx, id)
	if err != nil {
		return IncidentDetail{}, err
	}
	chunks, err := r.ListChunks(ctx, id)
	if err != nil {
		return IncidentDetail{}, err
	}
	checkins, err := r.ListCheckins(ctx, id)
	if err != nil {
		return IncidentDetail{}, err
	}

	return IncidentDetail{
		Incident: incident,
		Streams:  streams,
		Chunks:   chunks,
		Checkins: checkins,
	}, nil
}

// CloseIncident marks an incident closed so the HTTP layer rejects later chunk
// uploads.
func (r *Repository) CloseIncident(ctx context.Context, id string) (Incident, error) {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE incidents
		SET status = ?, updated_at = ?
		WHERE id = ? AND deletion_state = ?`,
		StatusClosed,
		formatDBTime(now),
		id,
		IncidentDeletionStateActive,
	)
	if err != nil {
		return Incident{}, fmt.Errorf("close incident: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return Incident{}, fmt.Errorf("close incident rows affected: %w", err)
	}
	if rowsAffected == 0 {
		incident, getErr := r.GetIncident(ctx, id)
		if errors.Is(getErr, ErrNotFound) {
			return Incident{}, ErrNotFound
		}
		if getErr != nil {
			return Incident{}, getErr
		}
		if incident.DeletionState != IncidentDeletionStateActive {
			return Incident{}, ErrIncidentDeleting
		}
		return Incident{}, ErrNotFound
	}
	return r.GetIncident(ctx, id)
}
