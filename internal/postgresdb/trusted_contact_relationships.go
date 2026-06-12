package postgresdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/open-proofline/server/internal/auth"
	"github.com/open-proofline/server/internal/incidents"
)

// CreateTrustedContactRelationship stores an owner-created pending invite for
// an authenticated recipient account. It never grants evidence or key access.
func (r *Repository) CreateTrustedContactRelationship(ctx context.Context, params incidents.CreateTrustedContactRelationshipParams) (incidents.TrustedContactRelationship, error) {
	if params.RelationshipRole == "" {
		params.RelationshipRole = incidents.TrustedContactRelationshipRoleTrustedContact
	}
	if !incidents.ValidTrustedContactRelationshipRole(params.RelationshipRole) {
		return incidents.TrustedContactRelationship{}, incidents.ErrInvalidState
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return incidents.TrustedContactRelationship{}, fmt.Errorf("begin create postgres trusted contact relationship: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if params.OwnerAccountID == params.RecipientAccountID {
		return incidents.TrustedContactRelationship{}, incidents.ErrInvalidState
	}
	if err := requireActiveAccountForTrustedContactTx(ctx, tx, params.RecipientAccountID); err != nil {
		return incidents.TrustedContactRelationship{}, err
	}
	if err := requireNoOpenTrustedContactRelationshipTx(ctx, tx, params.OwnerAccountID, params.RecipientAccountID, params.RelationshipRole, ""); err != nil {
		return incidents.TrustedContactRelationship{}, err
	}

	relationship, err := newPostgresTrustedContactRelationship(params)
	if err != nil {
		return incidents.TrustedContactRelationship{}, err
	}
	if err := insertTrustedContactRelationshipTx(ctx, tx, relationship); err != nil {
		return incidents.TrustedContactRelationship{}, err
	}
	if err := tx.Commit(); err != nil {
		return incidents.TrustedContactRelationship{}, fmt.Errorf("commit create postgres trusted contact relationship: %w", err)
	}
	return relationship, nil
}

// ListTrustedContactRelationshipsForAccount returns relationships where the
// authenticated account is the owner or recipient.
func (r *Repository) ListTrustedContactRelationshipsForAccount(ctx context.Context, accountID string) ([]incidents.TrustedContactRelationship, error) {
	rows, err := r.db.QueryContext(ctx, trustedContactRelationshipSelect()+`
		WHERE owner_account_id = $1 OR recipient_account_id = $1
		ORDER BY updated_at DESC, created_at DESC, id`,
		accountID,
	)
	if err != nil {
		return nil, fmt.Errorf("list postgres trusted contact relationships: %w", err)
	}
	defer rows.Close()
	return scanTrustedContactRelationshipRows(rows)
}

// GetTrustedContactRelationshipForAccount returns a relationship visible to an
// authenticated owner or recipient account.
func (r *Repository) GetTrustedContactRelationshipForAccount(ctx context.Context, accountID, relationshipID string) (incidents.TrustedContactRelationship, error) {
	row := r.db.QueryRowContext(ctx, trustedContactRelationshipSelect()+`
		WHERE id = $1 AND (owner_account_id = $2 OR recipient_account_id = $2)`,
		relationshipID,
		accountID,
	)
	relationship, err := scanTrustedContactRelationship(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.TrustedContactRelationship{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.TrustedContactRelationship{}, fmt.Errorf("get postgres trusted contact relationship: %w", err)
	}
	return relationship, nil
}

// AcceptTrustedContactRelationship marks a pending invite active. Only the
// authenticated recipient account can accept.
func (r *Repository) AcceptTrustedContactRelationship(ctx context.Context, recipientAccountID, relationshipID string) (incidents.TrustedContactRelationship, error) {
	return r.setRecipientTrustedContactRelationshipState(ctx, recipientAccountID, relationshipID, incidents.TrustedContactRelationshipStateActive)
}

// DeclineTrustedContactRelationship marks a pending invite declined. Only the
// authenticated recipient account can decline.
func (r *Repository) DeclineTrustedContactRelationship(ctx context.Context, recipientAccountID, relationshipID string) (incidents.TrustedContactRelationship, error) {
	return r.setRecipientTrustedContactRelationshipState(ctx, recipientAccountID, relationshipID, incidents.TrustedContactRelationshipStateDeclined)
}

// RevokeTrustedContactRelationship marks a pending or active relationship
// revoked. Only the owner account can revoke.
func (r *Repository) RevokeTrustedContactRelationship(ctx context.Context, ownerAccountID, relationshipID, revokedByAccountID string) (incidents.TrustedContactRelationship, error) {
	current, err := r.GetTrustedContactRelationshipForAccount(ctx, ownerAccountID, relationshipID)
	if err != nil {
		return incidents.TrustedContactRelationship{}, err
	}
	if current.OwnerAccountID != ownerAccountID {
		return incidents.TrustedContactRelationship{}, incidents.ErrNotFound
	}
	if current.RelationshipState == incidents.TrustedContactRelationshipStateRevoked {
		return current, nil
	}
	if incidents.TerminalTrustedContactRelationshipState(current.RelationshipState) {
		return incidents.TrustedContactRelationship{}, incidents.ErrInvalidState
	}

	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE trusted_contact_relationships
		SET relationship_state = $1, updated_at = $2, revoked_at = $3, revoked_by_account_id = $4
		WHERE owner_account_id = $5 AND id = $6`,
		incidents.TrustedContactRelationshipStateRevoked,
		now,
		now,
		nullableString(revokedByAccountID),
		ownerAccountID,
		relationshipID,
	)
	if err != nil {
		return incidents.TrustedContactRelationship{}, fmt.Errorf("revoke postgres trusted contact relationship: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return incidents.TrustedContactRelationship{}, fmt.Errorf("revoke postgres trusted contact relationship rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return incidents.TrustedContactRelationship{}, incidents.ErrNotFound
	}
	return r.GetTrustedContactRelationshipForAccount(ctx, ownerAccountID, relationshipID)
}

// ReplaceTrustedContactRelationship creates a successor pending invite and
// marks the previous relationship replaced. It does not copy or create keys.
func (r *Repository) ReplaceTrustedContactRelationship(ctx context.Context, params incidents.ReplaceTrustedContactRelationshipParams) (incidents.TrustedContactRelationship, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return incidents.TrustedContactRelationship{}, fmt.Errorf("begin replace postgres trusted contact relationship: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := getTrustedContactRelationshipTx(ctx, tx, params.RelationshipID)
	if err != nil {
		return incidents.TrustedContactRelationship{}, err
	}
	if current.OwnerAccountID != params.OwnerAccountID {
		return incidents.TrustedContactRelationship{}, incidents.ErrNotFound
	}
	if incidents.TerminalTrustedContactRelationshipState(current.RelationshipState) {
		return incidents.TrustedContactRelationship{}, incidents.ErrInvalidState
	}

	recipientAccountID := params.RecipientAccountID
	if recipientAccountID == "" {
		recipientAccountID = current.RecipientAccountID
	}
	role := params.RelationshipRole
	if role == "" {
		role = current.RelationshipRole
	}
	if !incidents.ValidTrustedContactRelationshipRole(role) {
		return incidents.TrustedContactRelationship{}, incidents.ErrInvalidState
	}
	if params.OwnerAccountID == recipientAccountID {
		return incidents.TrustedContactRelationship{}, incidents.ErrInvalidState
	}
	if err := requireActiveAccountForTrustedContactTx(ctx, tx, recipientAccountID); err != nil {
		return incidents.TrustedContactRelationship{}, err
	}
	if err := requireNoOpenTrustedContactRelationshipTx(ctx, tx, params.OwnerAccountID, recipientAccountID, role, current.ID); err != nil {
		return incidents.TrustedContactRelationship{}, err
	}

	replacement, err := newPostgresTrustedContactRelationship(incidents.CreateTrustedContactRelationshipParams{
		OwnerAccountID:     params.OwnerAccountID,
		RecipientAccountID: recipientAccountID,
		RelationshipRole:   role,
		DisplayLabel:       params.DisplayLabel,
	})
	if err != nil {
		return incidents.TrustedContactRelationship{}, err
	}

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE trusted_contact_relationships
		SET relationship_state = $1, updated_at = $2, replaced_at = $3
		WHERE owner_account_id = $4 AND id = $5`,
		incidents.TrustedContactRelationshipStateReplaced,
		now,
		now,
		params.OwnerAccountID,
		current.ID,
	)
	if err != nil {
		return incidents.TrustedContactRelationship{}, fmt.Errorf("mark postgres trusted contact relationship replaced: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return incidents.TrustedContactRelationship{}, fmt.Errorf("replace postgres trusted contact relationship rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return incidents.TrustedContactRelationship{}, incidents.ErrNotFound
	}
	if err := insertTrustedContactRelationshipTx(ctx, tx, replacement); err != nil {
		return incidents.TrustedContactRelationship{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE trusted_contact_relationships
		SET replaced_by_relationship_id = $1
		WHERE owner_account_id = $2 AND id = $3`,
		replacement.ID,
		params.OwnerAccountID,
		current.ID,
	); err != nil {
		return incidents.TrustedContactRelationship{}, fmt.Errorf("link postgres replacement trusted contact relationship: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return incidents.TrustedContactRelationship{}, fmt.Errorf("commit replace postgres trusted contact relationship: %w", err)
	}
	return replacement, nil
}

func (r *Repository) setRecipientTrustedContactRelationshipState(ctx context.Context, recipientAccountID, relationshipID, state string) (incidents.TrustedContactRelationship, error) {
	now := time.Now().UTC()
	var acceptedAt, declinedAt *time.Time
	if state == incidents.TrustedContactRelationshipStateActive {
		acceptedAt = &now
	}
	if state == incidents.TrustedContactRelationshipStateDeclined {
		declinedAt = &now
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE trusted_contact_relationships
		SET relationship_state = $1, updated_at = $2, accepted_at = $3, declined_at = $4
		WHERE recipient_account_id = $5 AND id = $6 AND relationship_state = $7`,
		state,
		now,
		nullableTime(acceptedAt),
		nullableTime(declinedAt),
		recipientAccountID,
		relationshipID,
		incidents.TrustedContactRelationshipStatePendingInvite,
	)
	if err != nil {
		return incidents.TrustedContactRelationship{}, fmt.Errorf("set postgres trusted contact relationship state: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return incidents.TrustedContactRelationship{}, fmt.Errorf("set postgres trusted contact relationship state rows affected: %w", err)
	}
	if rowsAffected == 0 {
		relationship, getErr := r.GetTrustedContactRelationshipForAccount(ctx, recipientAccountID, relationshipID)
		if getErr == nil && relationship.RecipientAccountID == recipientAccountID {
			return incidents.TrustedContactRelationship{}, incidents.ErrInvalidState
		}
		return incidents.TrustedContactRelationship{}, incidents.ErrNotFound
	}
	return r.GetTrustedContactRelationshipForAccount(ctx, recipientAccountID, relationshipID)
}

func newPostgresTrustedContactRelationship(params incidents.CreateTrustedContactRelationshipParams) (incidents.TrustedContactRelationship, error) {
	id, err := newID("tcr")
	if err != nil {
		return incidents.TrustedContactRelationship{}, err
	}
	now := time.Now().UTC()
	return incidents.TrustedContactRelationship{
		ID:                 id,
		OwnerAccountID:     params.OwnerAccountID,
		RecipientAccountID: params.RecipientAccountID,
		RelationshipRole:   params.RelationshipRole,
		RelationshipState:  incidents.TrustedContactRelationshipStatePendingInvite,
		DisplayLabel:       params.DisplayLabel,
		CreatedAt:          now,
		UpdatedAt:          now,
		InvitedAt:          now,
	}, nil
}

func insertTrustedContactRelationshipTx(ctx context.Context, tx *sql.Tx, relationship incidents.TrustedContactRelationship) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO trusted_contact_relationships (
			id, owner_account_id, recipient_account_id, relationship_role,
			relationship_state, display_label, created_at, updated_at, invited_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		relationship.ID,
		relationship.OwnerAccountID,
		relationship.RecipientAccountID,
		relationship.RelationshipRole,
		relationship.RelationshipState,
		nullableString(relationship.DisplayLabel),
		relationship.CreatedAt,
		relationship.UpdatedAt,
		relationship.InvitedAt,
	)
	if err != nil {
		if isIntegrityConstraint(err) {
			return incidents.ErrDuplicate
		}
		return fmt.Errorf("insert postgres trusted contact relationship: %w", err)
	}
	return nil
}

func getTrustedContactRelationshipTx(ctx context.Context, tx *sql.Tx, relationshipID string) (incidents.TrustedContactRelationship, error) {
	row := tx.QueryRowContext(ctx, trustedContactRelationshipSelect()+`
		WHERE id = $1
		FOR UPDATE`,
		relationshipID,
	)
	relationship, err := scanTrustedContactRelationship(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.TrustedContactRelationship{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.TrustedContactRelationship{}, fmt.Errorf("get postgres trusted contact relationship in transaction: %w", err)
	}
	return relationship, nil
}

func requireActiveAccountForTrustedContactTx(ctx context.Context, tx *sql.Tx, accountID string) error {
	var found string
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM accounts
		WHERE id = $1 AND account_state = $2`,
		accountID,
		auth.AccountStateActive,
	).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read postgres active trusted contact account: %w", err)
	}
	return nil
}

func requireNoOpenTrustedContactRelationshipTx(ctx context.Context, tx *sql.Tx, ownerAccountID, recipientAccountID, role, excludeRelationshipID string) error {
	query := `
		SELECT id
		FROM trusted_contact_relationships
		WHERE owner_account_id = $1
			AND recipient_account_id = $2
			AND relationship_role = $3
			AND relationship_state IN ($4, $5)`
	args := []any{
		ownerAccountID,
		recipientAccountID,
		role,
		incidents.TrustedContactRelationshipStatePendingInvite,
		incidents.TrustedContactRelationshipStateActive,
	}
	if excludeRelationshipID != "" {
		query += ` AND id <> $6`
		args = append(args, excludeRelationshipID)
	}
	query += ` LIMIT 1`

	var found string
	err := tx.QueryRowContext(ctx, query, args...).Scan(&found)
	if err == nil {
		return incidents.ErrDuplicate
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return fmt.Errorf("read postgres open trusted contact relationship: %w", err)
}

func trustedContactRelationshipSelect() string {
	return `
		SELECT id, owner_account_id, recipient_account_id, relationship_role,
			relationship_state, display_label, created_at, updated_at, invited_at,
			accepted_at, declined_at, revoked_at, revoked_by_account_id,
			replaced_at, replaced_by_relationship_id
		FROM trusted_contact_relationships `
}

func scanTrustedContactRelationshipRows(rows *sql.Rows) ([]incidents.TrustedContactRelationship, error) {
	relationships := []incidents.TrustedContactRelationship{}
	for rows.Next() {
		relationship, err := scanTrustedContactRelationship(rows)
		if err != nil {
			return nil, err
		}
		relationships = append(relationships, relationship)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres trusted contact relationships: %w", err)
	}
	return relationships, nil
}
