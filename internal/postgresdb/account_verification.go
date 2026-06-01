package postgresdb

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
		VALUES ($1, $2, $3, $4, $5, $6)`,
		token.ID,
		token.AccountID,
		token.Purpose,
		token.TokenHash,
		token.CreatedAt,
		token.ExpiresAt,
	)
	if err != nil {
		if isIntegrityConstraint(err) {
			return auth.AccountVerificationToken{}, "", auth.ErrNotFound
		}
		return auth.AccountVerificationToken{}, "", fmt.Errorf("insert postgres account verification token: %w", err)
	}
	return token, rawToken, nil
}

func (r *Repository) ConsumeAccountVerificationToken(ctx context.Context, rawToken, purpose string, now time.Time) (auth.Account, error) {
	tokenHash := auth.SessionTokenHash(rawToken)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.Account{}, fmt.Errorf("begin postgres account verification token consume: %w", err)
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
		SET consumed_at = $1
		WHERE id = $2 AND consumed_at IS NULL`,
		now,
		token.ID,
	)
	if err != nil {
		return auth.Account{}, fmt.Errorf("consume postgres account verification token: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return auth.Account{}, fmt.Errorf("consume postgres account verification token rows affected: %w", err)
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
			email_verified_at = COALESCE(email_verified_at, $1),
			updated_at = $1
		WHERE id = $2`,
		now,
		token.AccountID,
	)
	if err != nil {
		return auth.Account{}, fmt.Errorf("mark postgres account email verified: %w", err)
	}

	account, err := getAccountByIDTx(ctx, tx, token.AccountID)
	if err != nil {
		return auth.Account{}, err
	}
	if err := tx.Commit(); err != nil {
		return auth.Account{}, fmt.Errorf("commit postgres account verification token consume: %w", err)
	}
	return account, nil
}

func getAccountVerificationTokenTx(ctx context.Context, tx *sql.Tx, tokenHash, purpose string) (auth.AccountVerificationToken, error) {
	var token auth.AccountVerificationToken
	var consumedAt sql.NullTime
	err := tx.QueryRowContext(ctx, `
		SELECT id, account_id, purpose, token_hash, created_at, expires_at, consumed_at
		FROM account_verification_tokens
		WHERE token_hash = $1 AND purpose = $2`,
		tokenHash,
		purpose,
	).Scan(
		&token.ID,
		&token.AccountID,
		&token.Purpose,
		&token.TokenHash,
		&token.CreatedAt,
		&token.ExpiresAt,
		&consumedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.AccountVerificationToken{}, auth.ErrNotFound
		}
		return auth.AccountVerificationToken{}, err
	}
	token.CreatedAt = token.CreatedAt.UTC()
	token.ExpiresAt = token.ExpiresAt.UTC()
	token.ConsumedAt = nullableDBTime(consumedAt)
	return token, nil
}

func getAccountByIDTx(ctx context.Context, tx *sql.Tx, accountID string) (auth.Account, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, username, email_normalized, email_verified_at, account_state, password_hash, role, created_at, updated_at, password_changed_at
		FROM accounts
		WHERE id = $1`,
		accountID,
	)
	return scanAccount(row)
}
