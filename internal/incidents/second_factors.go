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

func (r *Repository) CreateEmailSecondFactorChallenge(ctx context.Context, params auth.CreateEmailSecondFactorChallengeParams) (auth.SecondFactorChallenge, string, error) {
	rawToken, err := newRawAuthToken()
	if err != nil {
		return auth.SecondFactorChallenge{}, "", err
	}
	tokenHash := auth.SessionTokenHash(rawToken)
	challengeID, err := newID("sfc")
	if err != nil {
		return auth.SecondFactorChallenge{}, "", err
	}
	emailAddress := auth.NormalizeEmail(params.EmailNormalized)
	now := time.Now().UTC()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.SecondFactorChallenge{}, "", fmt.Errorf("begin email second factor challenge: %w", err)
	}
	defer tx.Rollback()

	factor, err := getEmailSecondFactorForAccountTx(ctx, tx, params.AccountID)
	if errors.Is(err, auth.ErrNotFound) {
		factorID, idErr := newID("sf")
		if idErr != nil {
			return auth.SecondFactorChallenge{}, "", idErr
		}
		factor = auth.SecondFactor{
			ID:              factorID,
			AccountID:       params.AccountID,
			FactorType:      auth.SecondFactorTypeEmailChallenge,
			EmailNormalized: emailAddress,
			FactorState:     auth.SecondFactorStatePending,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO account_second_factors (
				id, account_id, factor_type, email_normalized, factor_state, created_at, updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			factor.ID,
			factor.AccountID,
			factor.FactorType,
			factor.EmailNormalized,
			factor.FactorState,
			formatDBTime(factor.CreatedAt),
			formatDBTime(factor.UpdatedAt),
		)
		if err != nil {
			if isConstraint(err) {
				return auth.SecondFactorChallenge{}, "", auth.ErrNotFound
			}
			return auth.SecondFactorChallenge{}, "", fmt.Errorf("insert email second factor: %w", err)
		}
	} else if err != nil {
		return auth.SecondFactorChallenge{}, "", err
	} else if factor.FactorState == auth.SecondFactorStateActive {
		return auth.SecondFactorChallenge{}, "", auth.ErrDuplicate
	} else {
		factor.EmailNormalized = emailAddress
		factor.UpdatedAt = now
		_, err = tx.ExecContext(ctx, `
			UPDATE account_second_factors
			SET email_normalized = ?, factor_state = ?, verified_at = NULL, updated_at = ?
			WHERE id = ?`,
			factor.EmailNormalized,
			auth.SecondFactorStatePending,
			formatDBTime(factor.UpdatedAt),
			factor.ID,
		)
		if err != nil {
			return auth.SecondFactorChallenge{}, "", fmt.Errorf("update pending email second factor: %w", err)
		}
	}

	challenge := auth.SecondFactorChallenge{
		ID:              challengeID,
		AccountID:       params.AccountID,
		FactorID:        factor.ID,
		ChallengeType:   auth.SecondFactorChallengeTypeEmailSetup,
		TokenHash:       tokenHash,
		EmailNormalized: emailAddress,
		CreatedAt:       now,
		ExpiresAt:       params.ExpiresAt.UTC(),
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO account_second_factor_challenges (
			id, account_id, factor_id, challenge_type, token_hash,
			email_normalized, created_at, expires_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		challenge.ID,
		challenge.AccountID,
		challenge.FactorID,
		challenge.ChallengeType,
		challenge.TokenHash,
		challenge.EmailNormalized,
		formatDBTime(challenge.CreatedAt),
		formatDBTime(challenge.ExpiresAt),
	)
	if err != nil {
		if isConstraint(err) {
			return auth.SecondFactorChallenge{}, "", auth.ErrNotFound
		}
		return auth.SecondFactorChallenge{}, "", fmt.Errorf("insert email second factor challenge: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return auth.SecondFactorChallenge{}, "", fmt.Errorf("commit email second factor challenge: %w", err)
	}
	return challenge, rawToken, nil
}

func (r *Repository) GetActiveEmailSecondFactor(ctx context.Context, accountID string) (auth.SecondFactor, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, account_id, factor_type, email_normalized, factor_state, created_at, updated_at, verified_at
		FROM account_second_factors
		WHERE account_id = ? AND factor_type = ? AND factor_state = ?`,
		accountID,
		auth.SecondFactorTypeEmailChallenge,
		auth.SecondFactorStateActive,
	)
	return scanSecondFactor(row)
}

func (r *Repository) CreateActiveEmailSecondFactorChallenge(ctx context.Context, accountID string, expiresAt time.Time) (auth.SecondFactorChallenge, string, error) {
	rawToken, err := newRawAuthToken()
	if err != nil {
		return auth.SecondFactorChallenge{}, "", err
	}
	tokenHash := auth.SessionTokenHash(rawToken)
	challengeID, err := newID("sfc")
	if err != nil {
		return auth.SecondFactorChallenge{}, "", err
	}
	now := time.Now().UTC()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.SecondFactorChallenge{}, "", fmt.Errorf("begin active email second factor challenge: %w", err)
	}
	defer tx.Rollback()

	factor, err := getActiveEmailSecondFactorForAccountTx(ctx, tx, accountID)
	if err != nil {
		return auth.SecondFactorChallenge{}, "", err
	}

	challenge := auth.SecondFactorChallenge{
		ID:              challengeID,
		AccountID:       accountID,
		FactorID:        factor.ID,
		ChallengeType:   auth.SecondFactorChallengeTypeEmailSetup,
		TokenHash:       tokenHash,
		EmailNormalized: factor.EmailNormalized,
		CreatedAt:       now,
		ExpiresAt:       expiresAt.UTC(),
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO account_second_factor_challenges (
			id, account_id, factor_id, challenge_type, token_hash,
			email_normalized, created_at, expires_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		challenge.ID,
		challenge.AccountID,
		challenge.FactorID,
		challenge.ChallengeType,
		challenge.TokenHash,
		challenge.EmailNormalized,
		formatDBTime(challenge.CreatedAt),
		formatDBTime(challenge.ExpiresAt),
	)
	if err != nil {
		if isConstraint(err) {
			return auth.SecondFactorChallenge{}, "", auth.ErrNotFound
		}
		return auth.SecondFactorChallenge{}, "", fmt.Errorf("insert active email second factor challenge: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return auth.SecondFactorChallenge{}, "", fmt.Errorf("commit active email second factor challenge: %w", err)
	}
	return challenge, rawToken, nil
}

func (r *Repository) ConsumeEmailSecondFactorChallenge(ctx context.Context, accountID, rawToken string, now time.Time) (auth.SecondFactor, auth.Account, error) {
	tokenHash := auth.SessionTokenHash(rawToken)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, fmt.Errorf("begin email second factor consume: %w", err)
	}
	defer tx.Rollback()

	challenge, err := getEmailSecondFactorChallengeTx(ctx, tx, accountID, tokenHash)
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, err
	}
	if subtle.ConstantTimeCompare([]byte(challenge.TokenHash), []byte(tokenHash)) != 1 {
		return auth.SecondFactor{}, auth.Account{}, auth.ErrNotFound
	}
	now = now.UTC()
	if challenge.ConsumedAt != nil || !challenge.ExpiresAt.After(now) {
		return auth.SecondFactor{}, auth.Account{}, auth.ErrNotFound
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE account_second_factor_challenges
		SET consumed_at = ?
		WHERE id = ? AND consumed_at IS NULL`,
		formatDBTime(now),
		challenge.ID,
	)
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, fmt.Errorf("consume email second factor challenge: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, fmt.Errorf("consume email second factor rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return auth.SecondFactor{}, auth.Account{}, auth.ErrNotFound
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE account_second_factor_challenges
		SET consumed_at = COALESCE(consumed_at, ?)
		WHERE account_id = ? AND factor_id = ? AND consumed_at IS NULL`,
		formatDBTime(now),
		accountID,
		challenge.FactorID,
	)
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, fmt.Errorf("consume sibling email second factor challenges: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE account_second_factors
		SET email_normalized = ?, factor_state = ?, verified_at = ?, updated_at = ?
		WHERE id = ? AND account_id = ?`,
		challenge.EmailNormalized,
		auth.SecondFactorStateActive,
		formatDBTime(now),
		formatDBTime(now),
		challenge.FactorID,
		accountID,
	)
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, fmt.Errorf("activate email second factor: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE accounts
		SET second_factor_setup_state = ?, updated_at = ?
		WHERE id = ?`,
		auth.SecondFactorSetupStateComplete,
		formatDBTime(now),
		accountID,
	)
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, fmt.Errorf("mark second factor setup complete: %w", err)
	}

	factor, err := getSecondFactorByIDTx(ctx, tx, challenge.FactorID)
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, err
	}
	account, err := getAccountByIDTx(ctx, tx, accountID)
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, err
	}
	if err := tx.Commit(); err != nil {
		return auth.SecondFactor{}, auth.Account{}, fmt.Errorf("commit email second factor consume: %w", err)
	}
	return factor, account, nil
}

func getEmailSecondFactorForAccountTx(ctx context.Context, tx *sql.Tx, accountID string) (auth.SecondFactor, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, account_id, factor_type, email_normalized, factor_state, created_at, updated_at, verified_at
		FROM account_second_factors
		WHERE account_id = ? AND factor_type = ?`,
		accountID,
		auth.SecondFactorTypeEmailChallenge,
	)
	return scanSecondFactor(row)
}

func getActiveEmailSecondFactorForAccountTx(ctx context.Context, tx *sql.Tx, accountID string) (auth.SecondFactor, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, account_id, factor_type, email_normalized, factor_state, created_at, updated_at, verified_at
		FROM account_second_factors
		WHERE account_id = ? AND factor_type = ? AND factor_state = ?`,
		accountID,
		auth.SecondFactorTypeEmailChallenge,
		auth.SecondFactorStateActive,
	)
	return scanSecondFactor(row)
}

func getSecondFactorByIDTx(ctx context.Context, tx *sql.Tx, factorID string) (auth.SecondFactor, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, account_id, factor_type, email_normalized, factor_state, created_at, updated_at, verified_at
		FROM account_second_factors
		WHERE id = ?`,
		factorID,
	)
	return scanSecondFactor(row)
}

func getEmailSecondFactorChallengeTx(ctx context.Context, tx *sql.Tx, accountID, tokenHash string) (auth.SecondFactorChallenge, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, account_id, factor_id, challenge_type, token_hash,
			email_normalized, created_at, expires_at, consumed_at
		FROM account_second_factor_challenges
		WHERE account_id = ? AND token_hash = ? AND challenge_type = ?`,
		accountID,
		tokenHash,
		auth.SecondFactorChallengeTypeEmailSetup,
	)
	return scanSecondFactorChallenge(row)
}

func scanSecondFactor(s scanner) (auth.SecondFactor, error) {
	var factor auth.SecondFactor
	var createdAt string
	var updatedAt string
	var verifiedAt sql.NullString
	if err := s.Scan(
		&factor.ID,
		&factor.AccountID,
		&factor.FactorType,
		&factor.EmailNormalized,
		&factor.FactorState,
		&createdAt,
		&updatedAt,
		&verifiedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.SecondFactor{}, auth.ErrNotFound
		}
		return auth.SecondFactor{}, err
	}
	var err error
	if factor.CreatedAt, err = parseDBTime(createdAt); err != nil {
		return auth.SecondFactor{}, err
	}
	if factor.UpdatedAt, err = parseDBTime(updatedAt); err != nil {
		return auth.SecondFactor{}, err
	}
	if factor.VerifiedAt, err = nullableDBTime(verifiedAt); err != nil {
		return auth.SecondFactor{}, err
	}
	return factor, nil
}

func scanSecondFactorChallenge(s scanner) (auth.SecondFactorChallenge, error) {
	var challenge auth.SecondFactorChallenge
	var createdAt string
	var expiresAt string
	var consumedAt sql.NullString
	if err := s.Scan(
		&challenge.ID,
		&challenge.AccountID,
		&challenge.FactorID,
		&challenge.ChallengeType,
		&challenge.TokenHash,
		&challenge.EmailNormalized,
		&createdAt,
		&expiresAt,
		&consumedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.SecondFactorChallenge{}, auth.ErrNotFound
		}
		return auth.SecondFactorChallenge{}, err
	}
	var err error
	if challenge.CreatedAt, err = parseDBTime(createdAt); err != nil {
		return auth.SecondFactorChallenge{}, err
	}
	if challenge.ExpiresAt, err = parseDBTime(expiresAt); err != nil {
		return auth.SecondFactorChallenge{}, err
	}
	if challenge.ConsumedAt, err = nullableDBTime(consumedAt); err != nil {
		return auth.SecondFactorChallenge{}, err
	}
	return challenge, nil
}
