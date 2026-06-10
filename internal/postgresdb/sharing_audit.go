package postgresdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/open-proofline/server/internal/incidents"
)

// ListSharingAuditEvents returns safe owner-scoped lifecycle audit metadata.
// It is a repository helper for private operational review and tests; no public
// route exposes these events.
func (r *Repository) ListSharingAuditEvents(ctx context.Context, ownerAccountID string) ([]incidents.SharingAuditEvent, error) {
	rows, err := r.db.QueryContext(ctx, sharingAuditEventSelect()+`
		WHERE owner_account_id = $1
		ORDER BY created_at, id`,
		ownerAccountID,
	)
	if err != nil {
		return nil, fmt.Errorf("list postgres sharing audit events: %w", err)
	}
	defer rows.Close()
	return scanSharingAuditEventRows(rows)
}

func createSharingAuditEventTx(ctx context.Context, tx *sql.Tx, params incidents.SharingAuditEventParams, createdAt time.Time) (incidents.SharingAuditEvent, error) {
	if !incidents.ValidSharingAuditAction(params.Action) || !incidents.ValidSharingAuditOutcome(params.OutcomeCategory) {
		return incidents.SharingAuditEvent{}, incidents.ErrInvalidState
	}
	if params.OwnerAccountID == "" || params.ActorAccountID == "" {
		return incidents.SharingAuditEvent{}, incidents.ErrInvalidState
	}
	id, err := newID("sae")
	if err != nil {
		return incidents.SharingAuditEvent{}, err
	}
	event := incidents.SharingAuditEvent{
		ID:                 id,
		OwnerAccountID:     params.OwnerAccountID,
		ActorAccountID:     params.ActorAccountID,
		Action:             params.Action,
		OutcomeCategory:    params.OutcomeCategory,
		IncidentID:         params.IncidentID,
		StreamID:           params.StreamID,
		GrantID:            params.GrantID,
		ContactID:          params.ContactID,
		ContactPublicKeyID: params.ContactPublicKeyID,
		WrappedKeyID:       params.WrappedKeyID,
		DeletionDecisionID: params.DeletionDecisionID,
		CreatedAt:          createdAt.UTC(),
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO sharing_audit_events (
			id, owner_account_id, actor_account_id, action, outcome_category,
			incident_id, stream_id, grant_id, contact_id, contact_public_key_id,
			wrapped_key_id, deletion_decision_id, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		event.ID,
		nullableString(event.OwnerAccountID),
		nullableString(event.ActorAccountID),
		event.Action,
		event.OutcomeCategory,
		nullableString(event.IncidentID),
		nullableString(event.StreamID),
		nullableString(event.GrantID),
		nullableString(event.ContactID),
		nullableString(event.ContactPublicKeyID),
		nullableString(event.WrappedKeyID),
		nullableString(event.DeletionDecisionID),
		event.CreatedAt,
	)
	if err != nil {
		return incidents.SharingAuditEvent{}, fmt.Errorf("insert postgres sharing audit event: %w", err)
	}
	return event, nil
}

func sharingAuditActorAccountID(actorAccountID, ownerAccountID string) string {
	if actorAccountID != "" {
		return actorAccountID
	}
	return ownerAccountID
}

func sharingAuditEventSelect() string {
	return `
		SELECT id, owner_account_id, actor_account_id, action, outcome_category,
			incident_id, stream_id, grant_id, contact_id, contact_public_key_id,
			wrapped_key_id, deletion_decision_id, created_at
		FROM sharing_audit_events `
}

func scanSharingAuditEvent(s scanner) (incidents.SharingAuditEvent, error) {
	var event incidents.SharingAuditEvent
	var ownerAccountID sql.NullString
	var actorAccountID sql.NullString
	var incidentID sql.NullString
	var streamID sql.NullString
	var grantID sql.NullString
	var contactID sql.NullString
	var contactPublicKeyID sql.NullString
	var wrappedKeyID sql.NullString
	var deletionDecisionID sql.NullString
	if err := s.Scan(
		&event.ID,
		&ownerAccountID,
		&actorAccountID,
		&event.Action,
		&event.OutcomeCategory,
		&incidentID,
		&streamID,
		&grantID,
		&contactID,
		&contactPublicKeyID,
		&wrappedKeyID,
		&deletionDecisionID,
		&event.CreatedAt,
	); err != nil {
		return incidents.SharingAuditEvent{}, err
	}
	event.CreatedAt = event.CreatedAt.UTC()
	if ownerAccountID.Valid {
		event.OwnerAccountID = ownerAccountID.String
	}
	if actorAccountID.Valid {
		event.ActorAccountID = actorAccountID.String
	}
	if incidentID.Valid {
		event.IncidentID = incidentID.String
	}
	if streamID.Valid {
		event.StreamID = streamID.String
	}
	if grantID.Valid {
		event.GrantID = grantID.String
	}
	if contactID.Valid {
		event.ContactID = contactID.String
	}
	if contactPublicKeyID.Valid {
		event.ContactPublicKeyID = contactPublicKeyID.String
	}
	if wrappedKeyID.Valid {
		event.WrappedKeyID = wrappedKeyID.String
	}
	if deletionDecisionID.Valid {
		event.DeletionDecisionID = deletionDecisionID.String
	}
	return event, nil
}

func scanSharingAuditEventRows(rows *sql.Rows) ([]incidents.SharingAuditEvent, error) {
	events := []incidents.SharingAuditEvent{}
	for rows.Next() {
		event, err := scanSharingAuditEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres sharing audit events: %w", err)
	}
	return events, nil
}

func incidentOwnerAccountIDTx(ctx context.Context, tx *sql.Tx, incidentID string) (string, error) {
	var ownerAccountID sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT owner_account_id
		FROM incidents
		WHERE id = $1`,
		incidentID,
	).Scan(&ownerAccountID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", incidents.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read postgres incident owner for sharing audit: %w", err)
	}
	if ownerAccountID.Valid {
		return ownerAccountID.String, nil
	}
	return "", nil
}
