package incidents

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/open-proofline/server/internal/auth"
)

func (r *Repository) CreateAccountVerificationToken(ctx context.Context, params auth.CreateAccountVerificationTokenParams) (auth.AccountVerificationToken, string, error) {
	rawToken, err := newRawAuthToken()
	if err != nil {
		return auth.AccountVerificationToken{}, "", err
	}
	tokenHash := auth.SessionTokenHash(rawToken)
	id, err := newID("avt")
	if err != nil {
		return auth.AccountVerificationToken{}, "", err
	}
	now := time.Now().UTC()
	token := auth.AccountVerificationToken{
		ID:        id,
		AccountID: params.AccountID,
		Purpose:   params.Purpose,
		TokenHash: tokenHash,
		CreatedAt: now,
		ExpiresAt: params.ExpiresAt.UTC(),
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO account_verification_tokens (
			id, account_id, purpose, token_hash, created_at, expires_at
		)
		VALUES (?, ?, ?, ?, ?, ?)`,
		token.ID,
		token.AccountID,
		token.Purpose,
		token.TokenHash,
		formatDBTime(token.CreatedAt),
		formatDBTime(token.ExpiresAt),
	)
	if err != nil {
		if isConstraint(err) {
			return auth.AccountVerificationToken{}, "", auth.ErrNotFound
		}
		return auth.AccountVerificationToken{}, "", fmt.Errorf("insert account verification token: %w", err)
	}
	return token, rawToken, nil
}

func (r *Repository) ConsumeAccountVerificationToken(ctx context.Context, rawToken, purpose string, now time.Time) (auth.Account, error) {
	tokenHash := auth.SessionTokenHash(rawToken)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.Account{}, fmt.Errorf("begin account verification token consume: %w", err)
	}
	defer tx.Rollback()

	token, err := getAccountVerificationTokenTx(ctx, tx, tokenHash, purpose)
	if err != nil {
		return auth.Account{}, err
	}
	if subtle.ConstantTimeCompare([]byte(token.TokenHash), []byte(tokenHash)) != 1 {
		return auth.Account{}, auth.ErrNotFound
	}
	now = now.UTC()
	if token.ConsumedAt != nil || !token.ExpiresAt.After(now) {
		return auth.Account{}, auth.ErrNotFound
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE account_verification_tokens
		SET consumed_at = ?
		WHERE id = ? AND consumed_at IS NULL`,
		formatDBTime(now),
		token.ID,
	)
	if err != nil {
		return auth.Account{}, fmt.Errorf("consume account verification token: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return auth.Account{}, fmt.Errorf("consume account verification token rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return auth.Account{}, auth.ErrNotFound
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE accounts
		SET account_state = CASE
				WHEN account_state = 'pending_email_verification' THEN 'active'
				ELSE account_state
			END,
			email_verified_at = COALESCE(email_verified_at, ?),
			updated_at = ?
		WHERE id = ?`,
		formatDBTime(now),
		formatDBTime(now),
		token.AccountID,
	)
	if err != nil {
		return auth.Account{}, fmt.Errorf("mark account email verified: %w", err)
	}

	account, err := getAccountByIDTx(ctx, tx, token.AccountID)
	if err != nil {
		return auth.Account{}, err
	}
	if err := tx.Commit(); err != nil {
		return auth.Account{}, fmt.Errorf("commit account verification token consume: %w", err)
	}
	return account, nil
}

func getAccountVerificationTokenTx(ctx context.Context, tx *sql.Tx, tokenHash, purpose string) (auth.AccountVerificationToken, error) {
	var token auth.AccountVerificationToken
	var createdAt string
	var expiresAt string
	var consumedAt sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT id, account_id, purpose, token_hash, created_at, expires_at, consumed_at
		FROM account_verification_tokens
		WHERE token_hash = ? AND purpose = ?`,
		tokenHash,
		purpose,
	).Scan(
		&token.ID,
		&token.AccountID,
		&token.Purpose,
		&token.TokenHash,
		&createdAt,
		&expiresAt,
		&consumedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.AccountVerificationToken{}, auth.ErrNotFound
		}
		return auth.AccountVerificationToken{}, err
	}
	var parseErr error
	if token.CreatedAt, parseErr = parseDBTime(createdAt); parseErr != nil {
		return auth.AccountVerificationToken{}, parseErr
	}
	if token.ExpiresAt, parseErr = parseDBTime(expiresAt); parseErr != nil {
		return auth.AccountVerificationToken{}, parseErr
	}
	if token.ConsumedAt, parseErr = nullableDBTime(consumedAt); parseErr != nil {
		return auth.AccountVerificationToken{}, parseErr
	}
	return token, nil
}

func getAccountByIDTx(ctx context.Context, tx *sql.Tx, accountID string) (auth.Account, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, username, email_normalized, email_verified_at, account_state, password_hash, role, created_at, updated_at, password_changed_at
		FROM accounts
		WHERE id = ?`,
		accountID,
	)
	return scanAccount(row)
}
