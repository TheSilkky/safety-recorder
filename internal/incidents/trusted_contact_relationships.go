package incidents

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/open-proofline/server/internal/auth"
)

// CreateTrustedContactRelationship stores an owner-created pending invite for
// an authenticated recipient account. It never grants evidence or key access.
func (r *Repository) CreateTrustedContactRelationship(ctx context.Context, params CreateTrustedContactRelationshipParams) (TrustedContactRelationship, error) {
	if params.RelationshipRole == "" {
		params.RelationshipRole = TrustedContactRelationshipRoleTrustedContact
	}
	if !ValidTrustedContactRelationshipRole(params.RelationshipRole) {
		return TrustedContactRelationship{}, ErrInvalidState
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return TrustedContactRelationship{}, fmt.Errorf("begin create trusted contact relationship: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if params.OwnerAccountID == params.RecipientAccountID {
		return TrustedContactRelationship{}, ErrInvalidState
	}
	if err := requireActiveAccountForTrustedContactTx(ctx, tx, params.RecipientAccountID); err != nil {
		return TrustedContactRelationship{}, err
	}
	if err := requireNoOpenTrustedContactRelationshipTx(ctx, tx, params.OwnerAccountID, params.RecipientAccountID, params.RelationshipRole, ""); err != nil {
		return TrustedContactRelationship{}, err
	}

	relationship, err := newTrustedContactRelationship(params)
	if err != nil {
		return TrustedContactRelationship{}, err
	}
	if err := insertTrustedContactRelationshipTx(ctx, tx, relationship); err != nil {
		return TrustedContactRelationship{}, err
	}
	if err := tx.Commit(); err != nil {
		return TrustedContactRelationship{}, fmt.Errorf("commit create trusted contact relationship: %w", err)
	}
	return relationship, nil
}

// ListTrustedContactRelationshipsForAccount returns relationships where the
// authenticated account is the owner or recipient.
func (r *Repository) ListTrustedContactRelationshipsForAccount(ctx context.Context, accountID string) ([]TrustedContactRelationship, error) {
	rows, err := r.db.QueryContext(ctx, trustedContactRelationshipSelect()+`
		WHERE owner_account_id = ? OR recipient_account_id = ?
		ORDER BY updated_at DESC, created_at DESC, id`,
		accountID,
		accountID,
	)
	if err != nil {
		return nil, fmt.Errorf("list trusted contact relationships: %w", err)
	}
	defer rows.Close()
	return scanTrustedContactRelationshipRows(rows)
}

// GetTrustedContactRelationshipForAccount returns a relationship visible to an
// authenticated owner or recipient account.
func (r *Repository) GetTrustedContactRelationshipForAccount(ctx context.Context, accountID, relationshipID string) (TrustedContactRelationship, error) {
	row := r.db.QueryRowContext(ctx, trustedContactRelationshipSelect()+`
		WHERE id = ? AND (owner_account_id = ? OR recipient_account_id = ?)`,
		relationshipID,
		accountID,
		accountID,
	)
	relationship, err := scanTrustedContactRelationship(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TrustedContactRelationship{}, ErrNotFound
	}
	if err != nil {
		return TrustedContactRelationship{}, fmt.Errorf("get trusted contact relationship: %w", err)
	}
	return relationship, nil
}

// AcceptTrustedContactRelationship marks a pending invite active. Only the
// authenticated recipient account can accept.
func (r *Repository) AcceptTrustedContactRelationship(ctx context.Context, recipientAccountID, relationshipID string) (TrustedContactRelationship, error) {
	return r.setRecipientTrustedContactRelationshipState(ctx, recipientAccountID, relationshipID, TrustedContactRelationshipStateActive)
}

// DeclineTrustedContactRelationship marks a pending invite declined. Only the
// authenticated recipient account can decline.
func (r *Repository) DeclineTrustedContactRelationship(ctx context.Context, recipientAccountID, relationshipID string) (TrustedContactRelationship, error) {
	return r.setRecipientTrustedContactRelationshipState(ctx, recipientAccountID, relationshipID, TrustedContactRelationshipStateDeclined)
}

// RevokeTrustedContactRelationship marks a pending or active relationship
// revoked. Only the owner account can revoke.
func (r *Repository) RevokeTrustedContactRelationship(ctx context.Context, ownerAccountID, relationshipID, revokedByAccountID string) (TrustedContactRelationship, error) {
	current, err := r.GetTrustedContactRelationshipForAccount(ctx, ownerAccountID, relationshipID)
	if err != nil {
		return TrustedContactRelationship{}, err
	}
	if current.OwnerAccountID != ownerAccountID {
		return TrustedContactRelationship{}, ErrNotFound
	}
	if current.RelationshipState == TrustedContactRelationshipStateRevoked {
		return current, nil
	}
	if TerminalTrustedContactRelationshipState(current.RelationshipState) {
		return TrustedContactRelationship{}, ErrInvalidState
	}

	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE trusted_contact_relationships
		SET relationship_state = ?, updated_at = ?, revoked_at = ?, revoked_by_account_id = ?
		WHERE owner_account_id = ? AND id = ?`,
		TrustedContactRelationshipStateRevoked,
		formatDBTime(now),
		formatDBTime(now),
		nullableString(revokedByAccountID),
		ownerAccountID,
		relationshipID,
	)
	if err != nil {
		return TrustedContactRelationship{}, fmt.Errorf("revoke trusted contact relationship: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return TrustedContactRelationship{}, fmt.Errorf("revoke trusted contact relationship rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return TrustedContactRelationship{}, ErrNotFound
	}
	return r.GetTrustedContactRelationshipForAccount(ctx, ownerAccountID, relationshipID)
}

// ReplaceTrustedContactRelationship creates a successor pending invite and
// marks the previous relationship replaced. It does not copy or create keys.
func (r *Repository) ReplaceTrustedContactRelationship(ctx context.Context, params ReplaceTrustedContactRelationshipParams) (TrustedContactRelationship, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return TrustedContactRelationship{}, fmt.Errorf("begin replace trusted contact relationship: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := getTrustedContactRelationshipTx(ctx, tx, params.RelationshipID)
	if err != nil {
		return TrustedContactRelationship{}, err
	}
	if current.OwnerAccountID != params.OwnerAccountID {
		return TrustedContactRelationship{}, ErrNotFound
	}
	if TerminalTrustedContactRelationshipState(current.RelationshipState) {
		return TrustedContactRelationship{}, ErrInvalidState
	}

	recipientAccountID := params.RecipientAccountID
	if recipientAccountID == "" {
		recipientAccountID = current.RecipientAccountID
	}
	role := params.RelationshipRole
	if role == "" {
		role = current.RelationshipRole
	}
	if !ValidTrustedContactRelationshipRole(role) {
		return TrustedContactRelationship{}, ErrInvalidState
	}
	if params.OwnerAccountID == recipientAccountID {
		return TrustedContactRelationship{}, ErrInvalidState
	}
	if err := requireActiveAccountForTrustedContactTx(ctx, tx, recipientAccountID); err != nil {
		return TrustedContactRelationship{}, err
	}
	if err := requireNoOpenTrustedContactRelationshipTx(ctx, tx, params.OwnerAccountID, recipientAccountID, role, current.ID); err != nil {
		return TrustedContactRelationship{}, err
	}

	replacement, err := newTrustedContactRelationship(CreateTrustedContactRelationshipParams{
		OwnerAccountID:     params.OwnerAccountID,
		RecipientAccountID: recipientAccountID,
		RelationshipRole:   role,
		DisplayLabel:       params.DisplayLabel,
	})
	if err != nil {
		return TrustedContactRelationship{}, err
	}

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE trusted_contact_relationships
		SET relationship_state = ?, updated_at = ?, replaced_at = ?
		WHERE owner_account_id = ? AND id = ?`,
		TrustedContactRelationshipStateReplaced,
		formatDBTime(now),
		formatDBTime(now),
		params.OwnerAccountID,
		current.ID,
	)
	if err != nil {
		return TrustedContactRelationship{}, fmt.Errorf("mark trusted contact relationship replaced: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return TrustedContactRelationship{}, fmt.Errorf("replace trusted contact relationship rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return TrustedContactRelationship{}, ErrNotFound
	}
	if err := insertTrustedContactRelationshipTx(ctx, tx, replacement); err != nil {
		return TrustedContactRelationship{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE trusted_contact_relationships
		SET replaced_by_relationship_id = ?
		WHERE owner_account_id = ? AND id = ?`,
		replacement.ID,
		params.OwnerAccountID,
		current.ID,
	); err != nil {
		return TrustedContactRelationship{}, fmt.Errorf("link replacement trusted contact relationship: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return TrustedContactRelationship{}, fmt.Errorf("commit replace trusted contact relationship: %w", err)
	}
	return replacement, nil
}

func (r *Repository) setRecipientTrustedContactRelationshipState(ctx context.Context, recipientAccountID, relationshipID, state string) (TrustedContactRelationship, error) {
	now := time.Now().UTC()
	var acceptedAt, declinedAt *time.Time
	if state == TrustedContactRelationshipStateActive {
		acceptedAt = &now
	}
	if state == TrustedContactRelationshipStateDeclined {
		declinedAt = &now
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE trusted_contact_relationships
		SET relationship_state = ?, updated_at = ?, accepted_at = ?, declined_at = ?
		WHERE recipient_account_id = ? AND id = ? AND relationship_state = ?`,
		state,
		formatDBTime(now),
		nullableTime(acceptedAt),
		nullableTime(declinedAt),
		recipientAccountID,
		relationshipID,
		TrustedContactRelationshipStatePendingInvite,
	)
	if err != nil {
		return TrustedContactRelationship{}, fmt.Errorf("set trusted contact relationship state: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return TrustedContactRelationship{}, fmt.Errorf("set trusted contact relationship state rows affected: %w", err)
	}
	if rowsAffected == 0 {
		relationship, getErr := r.GetTrustedContactRelationshipForAccount(ctx, recipientAccountID, relationshipID)
		if getErr == nil && relationship.RecipientAccountID == recipientAccountID {
			return TrustedContactRelationship{}, ErrInvalidState
		}
		return TrustedContactRelationship{}, ErrNotFound
	}
	return r.GetTrustedContactRelationshipForAccount(ctx, recipientAccountID, relationshipID)
}

func newTrustedContactRelationship(params CreateTrustedContactRelationshipParams) (TrustedContactRelationship, error) {
	id, err := newID("tcr")
	if err != nil {
		return TrustedContactRelationship{}, err
	}
	now := time.Now().UTC()
	return TrustedContactRelationship{
		ID:                 id,
		OwnerAccountID:     params.OwnerAccountID,
		RecipientAccountID: params.RecipientAccountID,
		RelationshipRole:   params.RelationshipRole,
		RelationshipState:  TrustedContactRelationshipStatePendingInvite,
		DisplayLabel:       params.DisplayLabel,
		CreatedAt:          now,
		UpdatedAt:          now,
		InvitedAt:          now,
	}, nil
}

func insertTrustedContactRelationshipTx(ctx context.Context, tx *sql.Tx, relationship TrustedContactRelationship) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO trusted_contact_relationships (
			id, owner_account_id, recipient_account_id, relationship_role,
			relationship_state, display_label, created_at, updated_at, invited_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		relationship.ID,
		relationship.OwnerAccountID,
		relationship.RecipientAccountID,
		relationship.RelationshipRole,
		relationship.RelationshipState,
		nullableString(relationship.DisplayLabel),
		formatDBTime(relationship.CreatedAt),
		formatDBTime(relationship.UpdatedAt),
		formatDBTime(relationship.InvitedAt),
	)
	if err != nil {
		if isConstraint(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("insert trusted contact relationship: %w", err)
	}
	return nil
}

func getTrustedContactRelationshipTx(ctx context.Context, tx *sql.Tx, relationshipID string) (TrustedContactRelationship, error) {
	row := tx.QueryRowContext(ctx, trustedContactRelationshipSelect()+`
		WHERE id = ?`,
		relationshipID,
	)
	relationship, err := scanTrustedContactRelationship(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TrustedContactRelationship{}, ErrNotFound
	}
	if err != nil {
		return TrustedContactRelationship{}, fmt.Errorf("get trusted contact relationship in transaction: %w", err)
	}
	return relationship, nil
}

func requireActiveAccountForTrustedContactTx(ctx context.Context, tx *sql.Tx, accountID string) error {
	var found string
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM accounts
		WHERE id = ? AND account_state = ?`,
		accountID,
		auth.AccountStateActive,
	).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read active trusted contact account: %w", err)
	}
	return nil
}

func requireNoOpenTrustedContactRelationshipTx(ctx context.Context, tx *sql.Tx, ownerAccountID, recipientAccountID, role, excludeRelationshipID string) error {
	query := `
		SELECT id
		FROM trusted_contact_relationships
		WHERE owner_account_id = ?
			AND recipient_account_id = ?
			AND relationship_role = ?
			AND relationship_state IN (?, ?)`
	args := []any{
		ownerAccountID,
		recipientAccountID,
		role,
		TrustedContactRelationshipStatePendingInvite,
		TrustedContactRelationshipStateActive,
	}
	if excludeRelationshipID != "" {
		query += ` AND id <> ?`
		args = append(args, excludeRelationshipID)
	}
	query += ` LIMIT 1`

	var found string
	err := tx.QueryRowContext(ctx, query, args...).Scan(&found)
	if err == nil {
		return ErrDuplicate
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return fmt.Errorf("read open trusted contact relationship: %w", err)
}

func trustedContactRelationshipSelect() string {
	return `
		SELECT id, owner_account_id, recipient_account_id, relationship_role,
			relationship_state, display_label, created_at, updated_at, invited_at,
			accepted_at, declined_at, revoked_at, revoked_by_account_id,
			replaced_at, replaced_by_relationship_id
		FROM trusted_contact_relationships `
}

func scanTrustedContactRelationshipRows(rows *sql.Rows) ([]TrustedContactRelationship, error) {
	relationships := []TrustedContactRelationship{}
	for rows.Next() {
		relationship, err := scanTrustedContactRelationship(rows)
		if err != nil {
			return nil, err
		}
		relationships = append(relationships, relationship)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trusted contact relationships: %w", err)
	}
	return relationships, nil
}
