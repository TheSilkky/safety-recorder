package incidents

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CreateAccountRecipientKey stores owner account/device public-key metadata.
// It never stores recipient private keys, raw media keys, or plaintext.
func (r *Repository) CreateAccountRecipientKey(ctx context.Context, params CreateAccountRecipientKeyParams) (AccountRecipientKey, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AccountRecipientKey{}, fmt.Errorf("begin create account recipient key: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	recipientID := params.RecipientID
	if recipientID == "" {
		recipientID, err = newID("arcp")
		if err != nil {
			return AccountRecipientKey{}, err
		}
	}

	var existing int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM account_recipient_keys
		WHERE owner_account_id = ? AND recipient_id = ?`,
		params.OwnerAccountID,
		recipientID,
	).Scan(&existing); err != nil {
		return AccountRecipientKey{}, fmt.Errorf("read account recipient key count: %w", err)
	}
	if existing > 0 {
		return AccountRecipientKey{}, ErrDuplicate
	}

	key, err := newAccountRecipientKey(params, recipientID, 1)
	if err != nil {
		return AccountRecipientKey{}, err
	}
	if err := insertAccountRecipientKeyTx(ctx, tx, key); err != nil {
		return AccountRecipientKey{}, err
	}
	if err := tx.Commit(); err != nil {
		return AccountRecipientKey{}, fmt.Errorf("commit create account recipient key: %w", err)
	}
	return key, nil
}

// ListAccountRecipientKeys returns account/device public-key metadata for one owner.
func (r *Repository) ListAccountRecipientKeys(ctx context.Context, ownerAccountID string) ([]AccountRecipientKey, error) {
	rows, err := r.db.QueryContext(ctx, accountRecipientKeySelect()+`
		WHERE owner_account_id = ?
		ORDER BY recipient_type, recipient_id, version, created_at, id`,
		ownerAccountID,
	)
	if err != nil {
		return nil, fmt.Errorf("list account recipient keys: %w", err)
	}
	defer rows.Close()
	return scanAccountRecipientKeyRows(rows)
}

// GetAccountRecipientKey returns one owner-scoped recipient-key record.
func (r *Repository) GetAccountRecipientKey(ctx context.Context, ownerAccountID, recipientKeyID string) (AccountRecipientKey, error) {
	row := r.db.QueryRowContext(ctx, accountRecipientKeySelect()+`
		WHERE owner_account_id = ? AND id = ?`,
		ownerAccountID,
		recipientKeyID,
	)
	key, err := scanAccountRecipientKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return AccountRecipientKey{}, ErrNotFound
	}
	if err != nil {
		return AccountRecipientKey{}, fmt.Errorf("get account recipient key: %w", err)
	}
	return key, nil
}

// GetActiveAccountRecipientKey returns one owner-scoped key only when it is
// eligible for future account/device wrapped-key delivery.
func (r *Repository) GetActiveAccountRecipientKey(ctx context.Context, ownerAccountID, recipientKeyID string) (AccountRecipientKey, error) {
	row := r.db.QueryRowContext(ctx, accountRecipientKeySelect()+`
		WHERE owner_account_id = ? AND id = ? AND key_state = ?`,
		ownerAccountID,
		recipientKeyID,
		AccountRecipientKeyStateActive,
	)
	key, err := scanAccountRecipientKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return AccountRecipientKey{}, ErrNotFound
	}
	if err != nil {
		return AccountRecipientKey{}, fmt.Errorf("get active account recipient key: %w", err)
	}
	return key, nil
}

// UpdateAccountRecipientKey updates mutable owner-scoped recipient-key metadata.
func (r *Repository) UpdateAccountRecipientKey(ctx context.Context, params UpdateAccountRecipientKeyParams) (AccountRecipientKey, error) {
	current, err := r.GetAccountRecipientKey(ctx, params.OwnerAccountID, params.RecipientKeyID)
	if err != nil {
		return AccountRecipientKey{}, err
	}
	if params.DisplayLabel != nil {
		current.DisplayLabel = *params.DisplayLabel
	}
	if params.KeyState != nil {
		if !allowedAccountRecipientKeyUpdate(current.KeyState, *params.KeyState) {
			return AccountRecipientKey{}, ErrInvalidState
		}
		current.KeyState = *params.KeyState
	}
	current.UpdatedAt = time.Now().UTC()

	result, err := r.db.ExecContext(ctx, `
		UPDATE account_recipient_keys
		SET display_label = ?, key_state = ?, updated_at = ?
		WHERE owner_account_id = ? AND id = ?`,
		nullableString(current.DisplayLabel),
		current.KeyState,
		formatDBTime(current.UpdatedAt),
		params.OwnerAccountID,
		params.RecipientKeyID,
	)
	if err != nil {
		return AccountRecipientKey{}, fmt.Errorf("update account recipient key: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return AccountRecipientKey{}, fmt.Errorf("update account recipient key rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return AccountRecipientKey{}, ErrNotFound
	}
	return r.GetAccountRecipientKey(ctx, params.OwnerAccountID, params.RecipientKeyID)
}

// RevokeAccountRecipientKey marks a key revoked so future wrapping cannot use it.
func (r *Repository) RevokeAccountRecipientKey(ctx context.Context, ownerAccountID, recipientKeyID string) (AccountRecipientKey, error) {
	return r.setAccountRecipientKeyTerminalState(ctx, ownerAccountID, recipientKeyID, AccountRecipientKeyStateRevoked)
}

// MarkAccountRecipientKeyLost marks a key lost so future wrapping cannot use it.
func (r *Repository) MarkAccountRecipientKeyLost(ctx context.Context, ownerAccountID, recipientKeyID string) (AccountRecipientKey, error) {
	return r.setAccountRecipientKeyTerminalState(ctx, ownerAccountID, recipientKeyID, AccountRecipientKeyStateLost)
}

// ReplaceAccountRecipientKey creates a new key version and marks the previous
// owner account/device key replaced in one transaction.
func (r *Repository) ReplaceAccountRecipientKey(ctx context.Context, params ReplaceAccountRecipientKeyParams) (AccountRecipientKey, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AccountRecipientKey{}, fmt.Errorf("begin replace account recipient key: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := getAccountRecipientKeyTx(ctx, tx, params.OwnerAccountID, params.RecipientKeyID)
	if err != nil {
		return AccountRecipientKey{}, err
	}
	if TerminalAccountRecipientKeyState(current.KeyState) {
		return AccountRecipientKey{}, ErrInvalidState
	}

	var version int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM account_recipient_keys
		WHERE owner_account_id = ? AND recipient_id = ?`,
		params.OwnerAccountID,
		current.RecipientID,
	).Scan(&version); err != nil {
		return AccountRecipientKey{}, fmt.Errorf("read replacement account recipient key version: %w", err)
	}

	replacementParams := CreateAccountRecipientKeyParams{
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
	replacement, err := newAccountRecipientKey(replacementParams, current.RecipientID, version)
	if err != nil {
		return AccountRecipientKey{}, err
	}
	if err := insertAccountRecipientKeyTx(ctx, tx, replacement); err != nil {
		return AccountRecipientKey{}, err
	}

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `
		UPDATE account_recipient_keys
		SET key_state = ?, updated_at = ?, replaced_at = ?, replaced_by_recipient_key_id = ?
		WHERE owner_account_id = ? AND id = ?`,
		AccountRecipientKeyStateReplaced,
		formatDBTime(now),
		formatDBTime(now),
		replacement.ID,
		params.OwnerAccountID,
		params.RecipientKeyID,
	)
	if err != nil {
		return AccountRecipientKey{}, fmt.Errorf("mark account recipient key replaced: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return AccountRecipientKey{}, fmt.Errorf("replace account recipient key rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return AccountRecipientKey{}, ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return AccountRecipientKey{}, fmt.Errorf("commit replace account recipient key: %w", err)
	}
	return replacement, nil
}

func (r *Repository) setAccountRecipientKeyTerminalState(ctx context.Context, ownerAccountID, recipientKeyID, state string) (AccountRecipientKey, error) {
	current, err := r.GetAccountRecipientKey(ctx, ownerAccountID, recipientKeyID)
	if err != nil {
		return AccountRecipientKey{}, err
	}
	if TerminalAccountRecipientKeyState(current.KeyState) && current.KeyState != state {
		return AccountRecipientKey{}, ErrInvalidState
	}
	if current.KeyState == state {
		return current, nil
	}

	now := time.Now().UTC()
	var revokedAt, lostAt *time.Time
	if state == AccountRecipientKeyStateRevoked {
		revokedAt = &now
	}
	if state == AccountRecipientKeyStateLost {
		lostAt = &now
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE account_recipient_keys
		SET key_state = ?, updated_at = ?, revoked_at = ?, lost_at = ?
		WHERE owner_account_id = ? AND id = ?`,
		state,
		formatDBTime(now),
		nullableTime(revokedAt),
		nullableTime(lostAt),
		ownerAccountID,
		recipientKeyID,
	)
	if err != nil {
		return AccountRecipientKey{}, fmt.Errorf("set account recipient key terminal state: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return AccountRecipientKey{}, fmt.Errorf("terminal account recipient key rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return AccountRecipientKey{}, ErrNotFound
	}
	return r.GetAccountRecipientKey(ctx, ownerAccountID, recipientKeyID)
}

func newAccountRecipientKey(params CreateAccountRecipientKeyParams, recipientID string, version int) (AccountRecipientKey, error) {
	id, err := newID("ark")
	if err != nil {
		return AccountRecipientKey{}, err
	}
	now := time.Now().UTC()
	return AccountRecipientKey{
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

func insertAccountRecipientKeyTx(ctx context.Context, tx *sql.Tx, key AccountRecipientKey) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO account_recipient_keys (
			id, owner_account_id, recipient_id, recipient_type, key_id, version,
			display_label, scheme, suite_id, public_key, public_key_fingerprint,
			key_state, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
		formatDBTime(key.CreatedAt),
		formatDBTime(key.UpdatedAt),
	)
	if err != nil {
		if isConstraint(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("insert account recipient key: %w", err)
	}
	return nil
}

func getAccountRecipientKeyTx(ctx context.Context, tx *sql.Tx, ownerAccountID, recipientKeyID string) (AccountRecipientKey, error) {
	row := tx.QueryRowContext(ctx, accountRecipientKeySelect()+`
		WHERE owner_account_id = ? AND id = ?`,
		ownerAccountID,
		recipientKeyID,
	)
	key, err := scanAccountRecipientKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return AccountRecipientKey{}, ErrNotFound
	}
	if err != nil {
		return AccountRecipientKey{}, fmt.Errorf("get account recipient key in transaction: %w", err)
	}
	return key, nil
}

func allowedAccountRecipientKeyUpdate(currentState, nextState string) bool {
	if currentState == nextState {
		return true
	}
	if TerminalAccountRecipientKeyState(currentState) || TerminalAccountRecipientKeyState(nextState) {
		return false
	}
	return currentState == AccountRecipientKeyStatePendingVerification && nextState == AccountRecipientKeyStateActive
}

func accountRecipientKeySelect() string {
	return `
		SELECT id, owner_account_id, recipient_id, recipient_type, key_id, version,
			display_label, scheme, suite_id, public_key, public_key_fingerprint,
			key_state, created_at, updated_at, revoked_at, replaced_at, lost_at,
			replaced_by_recipient_key_id
		FROM account_recipient_keys `
}

func scanAccountRecipientKeyRows(rows *sql.Rows) ([]AccountRecipientKey, error) {
	keys := []AccountRecipientKey{}
	for rows.Next() {
		key, err := scanAccountRecipientKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate account recipient keys: %w", err)
	}
	return keys, nil
}
