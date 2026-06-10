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
		return auth.SecondFactorChallenge{}, "", fmt.Errorf("begin postgres email second factor challenge: %w", err)
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
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			factor.ID,
			factor.AccountID,
			factor.FactorType,
			factor.EmailNormalized,
			factor.FactorState,
			factor.CreatedAt,
			factor.UpdatedAt,
		)
		if err != nil {
			if isIntegrityConstraint(err) {
				return auth.SecondFactorChallenge{}, "", auth.ErrNotFound
			}
			return auth.SecondFactorChallenge{}, "", fmt.Errorf("insert postgres email second factor: %w", err)
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
			SET email_normalized = $1, factor_state = $2, verified_at = NULL, updated_at = $3
			WHERE id = $4`,
			factor.EmailNormalized,
			auth.SecondFactorStatePending,
			factor.UpdatedAt,
			factor.ID,
		)
		if err != nil {
			return auth.SecondFactorChallenge{}, "", fmt.Errorf("update postgres pending email second factor: %w", err)
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
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		challenge.ID,
		challenge.AccountID,
		challenge.FactorID,
		challenge.ChallengeType,
		challenge.TokenHash,
		challenge.EmailNormalized,
		challenge.CreatedAt,
		challenge.ExpiresAt,
	)
	if err != nil {
		if isIntegrityConstraint(err) {
			return auth.SecondFactorChallenge{}, "", auth.ErrNotFound
		}
		return auth.SecondFactorChallenge{}, "", fmt.Errorf("insert postgres email second factor challenge: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return auth.SecondFactorChallenge{}, "", fmt.Errorf("commit postgres email second factor challenge: %w", err)
	}
	return challenge, rawToken, nil
}

func (r *Repository) ConsumeEmailSecondFactorChallenge(ctx context.Context, accountID, rawToken string, now time.Time) (auth.SecondFactor, auth.Account, error) {
	tokenHash := auth.SessionTokenHash(rawToken)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, fmt.Errorf("begin postgres email second factor consume: %w", err)
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
		SET consumed_at = $1
		WHERE id = $2 AND consumed_at IS NULL`,
		now,
		challenge.ID,
	)
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, fmt.Errorf("consume postgres email second factor challenge: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, fmt.Errorf("consume postgres email second factor rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return auth.SecondFactor{}, auth.Account{}, auth.ErrNotFound
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE account_second_factor_challenges
		SET consumed_at = COALESCE(consumed_at, $1)
		WHERE account_id = $2 AND factor_id = $3 AND consumed_at IS NULL`,
		now,
		accountID,
		challenge.FactorID,
	)
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, fmt.Errorf("consume postgres sibling email second factor challenges: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE account_second_factors
		SET email_normalized = $1, factor_state = $2, verified_at = $3, updated_at = $3
		WHERE id = $4 AND account_id = $5`,
		challenge.EmailNormalized,
		auth.SecondFactorStateActive,
		now,
		challenge.FactorID,
		accountID,
	)
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, fmt.Errorf("activate postgres email second factor: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE accounts
		SET second_factor_setup_state = $1, updated_at = $2
		WHERE id = $3`,
		auth.SecondFactorSetupStateComplete,
		now,
		accountID,
	)
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, fmt.Errorf("mark postgres second factor setup complete: %w", err)
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
		return auth.SecondFactor{}, auth.Account{}, fmt.Errorf("commit postgres email second factor consume: %w", err)
	}
	return factor, account, nil
}

func getEmailSecondFactorForAccountTx(ctx context.Context, tx *sql.Tx, accountID string) (auth.SecondFactor, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, account_id, factor_type, email_normalized, factor_state, created_at, updated_at, verified_at
		FROM account_second_factors
		WHERE account_id = $1 AND factor_type = $2`,
		accountID,
		auth.SecondFactorTypeEmailChallenge,
	)
	return scanSecondFactor(row)
}

func getSecondFactorByIDTx(ctx context.Context, tx *sql.Tx, factorID string) (auth.SecondFactor, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, account_id, factor_type, email_normalized, factor_state, created_at, updated_at, verified_at
		FROM account_second_factors
		WHERE id = $1`,
		factorID,
	)
	return scanSecondFactor(row)
}

func getEmailSecondFactorChallengeTx(ctx context.Context, tx *sql.Tx, accountID, tokenHash string) (auth.SecondFactorChallenge, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, account_id, factor_id, challenge_type, token_hash,
			email_normalized, created_at, expires_at, consumed_at
		FROM account_second_factor_challenges
		WHERE account_id = $1 AND token_hash = $2 AND challenge_type = $3`,
		accountID,
		tokenHash,
		auth.SecondFactorChallengeTypeEmailSetup,
	)
	return scanSecondFactorChallenge(row)
}

func scanSecondFactor(s scanner) (auth.SecondFactor, error) {
	var factor auth.SecondFactor
	var verifiedAt sql.NullTime
	if err := s.Scan(
		&factor.ID,
		&factor.AccountID,
		&factor.FactorType,
		&factor.EmailNormalized,
		&factor.FactorState,
		&factor.CreatedAt,
		&factor.UpdatedAt,
		&verifiedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.SecondFactor{}, auth.ErrNotFound
		}
		return auth.SecondFactor{}, err
	}
	factor.CreatedAt = factor.CreatedAt.UTC()
	factor.UpdatedAt = factor.UpdatedAt.UTC()
	factor.VerifiedAt = nullableDBTime(verifiedAt)
	return factor, nil
}

func scanSecondFactorChallenge(s scanner) (auth.SecondFactorChallenge, error) {
	var challenge auth.SecondFactorChallenge
	var consumedAt sql.NullTime
	if err := s.Scan(
		&challenge.ID,
		&challenge.AccountID,
		&challenge.FactorID,
		&challenge.ChallengeType,
		&challenge.TokenHash,
		&challenge.EmailNormalized,
		&challenge.CreatedAt,
		&challenge.ExpiresAt,
		&consumedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.SecondFactorChallenge{}, auth.ErrNotFound
		}
		return auth.SecondFactorChallenge{}, err
	}
	challenge.CreatedAt = challenge.CreatedAt.UTC()
	challenge.ExpiresAt = challenge.ExpiresAt.UTC()
	challenge.ConsumedAt = nullableDBTime(consumedAt)
	return challenge, nil
}
