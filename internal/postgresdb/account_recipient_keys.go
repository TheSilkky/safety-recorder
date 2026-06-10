package postgresdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/open-proofline/server/internal/incidents"
)

// CreateAccountRecipientKey stores owner account/device public-key metadata.
// It never stores recipient private keys, raw media keys, or plaintext.
func (r *Repository) CreateAccountRecipientKey(ctx context.Context, params incidents.CreateAccountRecipientKeyParams) (incidents.AccountRecipientKey, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return incidents.AccountRecipientKey{}, fmt.Errorf("begin create postgres account recipient key: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	recipientID := params.RecipientID
	if recipientID == "" {
		recipientID, err = newID("arcp")
		if err != nil {
			return incidents.AccountRecipientKey{}, err
		}
	}

	var existing int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM account_recipient_keys
		WHERE owner_account_id = $1 AND recipient_id = $2`,
		params.OwnerAccountID,
		recipientID,
	).Scan(&existing); err != nil {
		return incidents.AccountRecipientKey{}, fmt.Errorf("read postgres account recipient key count: %w", err)
	}
	if existing > 0 {
		return incidents.AccountRecipientKey{}, incidents.ErrDuplicate
	}

	key, err := newPostgresAccountRecipientKey(params, recipientID, 1)
	if err != nil {
		return incidents.AccountRecipientKey{}, err
	}
	if err := insertAccountRecipientKeyTx(ctx, tx, key); err != nil {
		return incidents.AccountRecipientKey{}, err
	}
	if err := tx.Commit(); err != nil {
		return incidents.AccountRecipientKey{}, fmt.Errorf("commit create postgres account recipient key: %w", err)
	}
	return key, nil
}

// ListAccountRecipientKeys returns account/device public-key metadata for one owner.
func (r *Repository) ListAccountRecipientKeys(ctx context.Context, ownerAccountID string) ([]incidents.AccountRecipientKey, error) {
	rows, err := r.db.QueryContext(ctx, accountRecipientKeySelect()+`
		WHERE owner_account_id = $1
		ORDER BY recipient_type, recipient_id, version, created_at, id`,
		ownerAccountID,
	)
	if err != nil {
		return nil, fmt.Errorf("list postgres account recipient keys: %w", err)
	}
	defer rows.Close()
	return scanAccountRecipientKeyRows(rows)
}

// GetAccountRecipientKey returns one owner-scoped recipient-key record.
func (r *Repository) GetAccountRecipientKey(ctx context.Context, ownerAccountID, recipientKeyID string) (incidents.AccountRecipientKey, error) {
	row := r.db.QueryRowContext(ctx, accountRecipientKeySelect()+`
		WHERE owner_account_id = $1 AND id = $2`,
		ownerAccountID,
		recipientKeyID,
	)
	key, err := scanAccountRecipientKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.AccountRecipientKey{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.AccountRecipientKey{}, fmt.Errorf("get postgres account recipient key: %w", err)
	}
	return key, nil
}

// GetActiveAccountRecipientKey returns one owner-scoped key only when it is
// eligible for future account/device wrapped-key delivery.
func (r *Repository) GetActiveAccountRecipientKey(ctx context.Context, ownerAccountID, recipientKeyID string) (incidents.AccountRecipientKey, error) {
	row := r.db.QueryRowContext(ctx, accountRecipientKeySelect()+`
		WHERE owner_account_id = $1 AND id = $2 AND key_state = $3`,
		ownerAccountID,
		recipientKeyID,
		incidents.AccountRecipientKeyStateActive,
	)
	key, err := scanAccountRecipientKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.AccountRecipientKey{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.AccountRecipientKey{}, fmt.Errorf("get active postgres account recipient key: %w", err)
	}
	return key, nil
}

// UpdateAccountRecipientKey updates mutable owner-scoped recipient-key metadata.
func (r *Repository) UpdateAccountRecipientKey(ctx context.Context, params incidents.UpdateAccountRecipientKeyParams) (incidents.AccountRecipientKey, error) {
	current, err := r.GetAccountRecipientKey(ctx, params.OwnerAccountID, params.RecipientKeyID)
	if err != nil {
		return incidents.AccountRecipientKey{}, err
	}
	if params.DisplayLabel != nil {
		current.DisplayLabel = *params.DisplayLabel
	}
	if params.KeyState != nil {
		if !allowedAccountRecipientKeyUpdate(current.KeyState, *params.KeyState) {
			return incidents.AccountRecipientKey{}, incidents.ErrInvalidState
		}
		current.KeyState = *params.KeyState
	}
	current.UpdatedAt = time.Now().UTC()

	result, err := r.db.ExecContext(ctx, `
		UPDATE account_recipient_keys
		SET display_label = $1, key_state = $2, updated_at = $3
		WHERE owner_account_id = $4 AND id = $5`,
		nullableString(current.DisplayLabel),
		current.KeyState,
		current.UpdatedAt,
		params.OwnerAccountID,
		params.RecipientKeyID,
	)
	if err != nil {
		return incidents.AccountRecipientKey{}, fmt.Errorf("update postgres account recipient key: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return incidents.AccountRecipientKey{}, fmt.Errorf("update postgres account recipient key rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return incidents.AccountRecipientKey{}, incidents.ErrNotFound
	}
	return r.GetAccountRecipientKey(ctx, params.OwnerAccountID, params.RecipientKeyID)
}

// RevokeAccountRecipientKey marks a key revoked so future wrapping cannot use it.
func (r *Repository) RevokeAccountRecipientKey(ctx context.Context, ownerAccountID, recipientKeyID string) (incidents.AccountRecipientKey, error) {
	return r.setAccountRecipientKeyTerminalState(ctx, ownerAccountID, recipientKeyID, incidents.AccountRecipientKeyStateRevoked)
}

// MarkAccountRecipientKeyLost marks a key lost so future wrapping cannot use it.
func (r *Repository) MarkAccountRecipientKeyLost(ctx context.Context, ownerAccountID, recipientKeyID string) (incidents.AccountRecipientKey, error) {
	return r.setAccountRecipientKeyTerminalState(ctx, ownerAccountID, recipientKeyID, incidents.AccountRecipientKeyStateLost)
}

// ReplaceAccountRecipientKey creates a new key version and marks the previous
// owner account/device key replaced in one transaction.
func (r *Repository) ReplaceAccountRecipientKey(ctx context.Context, params incidents.ReplaceAccountRecipientKeyParams) (incidents.AccountRecipientKey, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return incidents.AccountRecipientKey{}, fmt.Errorf("begin replace postgres account recipient key: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := getAccountRecipientKeyTx(ctx, tx, params.OwnerAccountID, params.RecipientKeyID)
	if err != nil {
		return incidents.AccountRecipientKey{}, err
	}
	if incidents.TerminalAccountRecipientKeyState(current.KeyState) {
		return incidents.AccountRecipientKey{}, incidents.ErrInvalidState
	}

	var version int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM account_recipient_keys
		WHERE owner_account_id = $1 AND recipient_id = $2`,
		params.OwnerAccountID,
		current.RecipientID,
	).Scan(&version); err != nil {
		return incidents.AccountRecipientKey{}, fmt.Errorf("read postgres replacement account recipient key version: %w", err)
	}

	replacementParams := incidents.CreateAccountRecipientKeyParams{
		OwnerAccountID:       params.OwnerAccountID,
		RecipientID:          current.RecipientID,
		RecipientType:        current.RecipientType,
		KeyID:                params.KeyID,
		DisplayLabel:         params.DisplayLabel,
		Scheme:               params.Scheme,
		SuiteID:              params.SuiteID,
		PublicKey:            params.PublicKey,
		PublicKeyFingerprint: params.PublicKeyFingerprint,
		KeyState:             params.KeyState,
	}
	replacement, err := newPostgresAccountRecipientKey(replacementParams, current.RecipientID, version)
	if err != nil {
		return incidents.AccountRecipientKey{}, err
	}
	if err := insertAccountRecipientKeyTx(ctx, tx, replacement); err != nil {
		return incidents.AccountRecipientKey{}, err
	}

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE account_recipient_keys
		SET key_state = $1, updated_at = $2, replaced_at = $3, replaced_by_recipient_key_id = $4
		WHERE owner_account_id = $5 AND id = $6`,
		incidents.AccountRecipientKeyStateReplaced,
		now,
		now,
		replacement.ID,
		params.OwnerAccountID,
		params.RecipientKeyID,
	)
	if err != nil {
		return incidents.AccountRecipientKey{}, fmt.Errorf("mark postgres account recipient key replaced: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return incidents.AccountRecipientKey{}, fmt.Errorf("replace postgres account recipient key rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return incidents.AccountRecipientKey{}, incidents.ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return incidents.AccountRecipientKey{}, fmt.Errorf("commit replace postgres account recipient key: %w", err)
	}
	return replacement, nil
}

func (r *Repository) setAccountRecipientKeyTerminalState(ctx context.Context, ownerAccountID, recipientKeyID, state string) (incidents.AccountRecipientKey, error) {
	current, err := r.GetAccountRecipientKey(ctx, ownerAccountID, recipientKeyID)
	if err != nil {
		return incidents.AccountRecipientKey{}, err
	}
	if incidents.TerminalAccountRecipientKeyState(current.KeyState) && current.KeyState != state {
		return incidents.AccountRecipientKey{}, incidents.ErrInvalidState
	}
	if current.KeyState == state {
		return current, nil
	}

	now := time.Now().UTC()
	var revokedAt, lostAt *time.Time
	if state == incidents.AccountRecipientKeyStateRevoked {
		revokedAt = &now
	}
	if state == incidents.AccountRecipientKeyStateLost {
		lostAt = &now
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE account_recipient_keys
		SET key_state = $1, updated_at = $2, revoked_at = $3, lost_at = $4
		WHERE owner_account_id = $5 AND id = $6`,
		state,
		now,
		nullableTime(revokedAt),
		nullableTime(lostAt),
		ownerAccountID,
		recipientKeyID,
	)
	if err != nil {
		return incidents.AccountRecipientKey{}, fmt.Errorf("set postgres account recipient key terminal state: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return incidents.AccountRecipientKey{}, fmt.Errorf("terminal postgres account recipient key rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return incidents.AccountRecipientKey{}, incidents.ErrNotFound
	}
	return r.GetAccountRecipientKey(ctx, ownerAccountID, recipientKeyID)
}

func newPostgresAccountRecipientKey(params incidents.CreateAccountRecipientKeyParams, recipientID string, version int) (incidents.AccountRecipientKey, error) {
	id, err := newID("ark")
	if err != nil {
		return incidents.AccountRecipientKey{}, err
	}
	now := time.Now().UTC()
	return incidents.AccountRecipientKey{
		ID:                   id,
		OwnerAccountID:       params.OwnerAccountID,
		RecipientID:          recipientID,
		RecipientType:        params.RecipientType,
		KeyID:                params.KeyID,
		Version:              version,
		DisplayLabel:         params.DisplayLabel,
		Scheme:               params.Scheme,
		SuiteID:              params.SuiteID,
		PublicKey:            params.PublicKey,
		PublicKeyFingerprint: params.PublicKeyFingerprint,
		KeyState:             params.KeyState,
		CreatedAt:            now,
		UpdatedAt:            now,
	}, nil
}

func insertAccountRecipientKeyTx(ctx context.Context, tx *sql.Tx, key incidents.AccountRecipientKey) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO account_recipient_keys (
			id, owner_account_id, recipient_id, recipient_type, key_id, version,
			display_label, scheme, suite_id, public_key, public_key_fingerprint,
			key_state, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		key.ID,
		key.OwnerAccountID,
		key.RecipientID,
		key.RecipientType,
		key.KeyID,
		key.Version,
		nullableString(key.DisplayLabel),
		key.Scheme,
		key.SuiteID,
		key.PublicKey,
		key.PublicKeyFingerprint,
		key.KeyState,
		key.CreatedAt,
		key.UpdatedAt,
	)
	if err != nil {
		if isIntegrityConstraint(err) {
			return incidents.ErrDuplicate
		}
		return fmt.Errorf("insert postgres account recipient key: %w", err)
	}
	return nil
}

func getAccountRecipientKeyTx(ctx context.Context, tx *sql.Tx, ownerAccountID, recipientKeyID string) (incidents.AccountRecipientKey, error) {
	row := tx.QueryRowContext(ctx, accountRecipientKeySelect()+`
		WHERE owner_account_id = $1 AND id = $2
		FOR UPDATE`,
		ownerAccountID,
		recipientKeyID,
	)
	key, err := scanAccountRecipientKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.AccountRecipientKey{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.AccountRecipientKey{}, fmt.Errorf("get postgres account recipient key in transaction: %w", err)
	}
	return key, nil
}

func allowedAccountRecipientKeyUpdate(currentState, nextState string) bool {
	if currentState == nextState {
		return true
	}
	if incidents.TerminalAccountRecipientKeyState(currentState) || incidents.TerminalAccountRecipientKeyState(nextState) {
		return false
	}
	return currentState == incidents.AccountRecipientKeyStatePendingVerification && nextState == incidents.AccountRecipientKeyStateActive
}

func accountRecipientKeySelect() string {
	return `
		SELECT id, owner_account_id, recipient_id, recipient_type, key_id, version,
			display_label, scheme, suite_id, public_key, public_key_fingerprint,
			key_state, created_at, updated_at, revoked_at, replaced_at, lost_at,
			replaced_by_recipient_key_id
		FROM account_recipient_keys `
}

func scanAccountRecipientKeyRows(rows *sql.Rows) ([]incidents.AccountRecipientKey, error) {
	keys := []incidents.AccountRecipientKey{}
	for rows.Next() {
		key, err := scanAccountRecipientKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres account recipient keys: %w", err)
	}
	return keys, nil
}
