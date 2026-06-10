package postgresdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/open-proofline/server/internal/auth"
)

func (r *Repository) CreateTOTPSecondFactorEnrollment(ctx context.Context, params auth.CreateTOTPSecondFactorEnrollmentParams) (auth.SecondFactor, error) {
	now := time.Now().UTC()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.SecondFactor{}, fmt.Errorf("begin postgres TOTP second factor enrollment: %w", err)
	}
	defer tx.Rollback()

	factor, err := getTOTPSecondFactorForAccountTx(ctx, tx, params.AccountID)
	if errors.Is(err, auth.ErrNotFound) {
		factorID, idErr := newID("sf")
		if idErr != nil {
			return auth.SecondFactor{}, idErr
		}
		factor = auth.SecondFactor{
			ID:                factorID,
			AccountID:         params.AccountID,
			FactorType:        auth.SecondFactorTypeTOTP,
			FactorState:       auth.SecondFactorStatePending,
			TOTPSecret:        params.Secret,
			TOTPPeriodSeconds: params.PeriodSeconds,
			TOTPDigits:        params.Digits,
			TOTPAlgorithm:     params.Algorithm,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO account_totp_second_factors (
				id, account_id, factor_state, secret, period_seconds, digits, algorithm, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			factor.ID,
			factor.AccountID,
			factor.FactorState,
			factor.TOTPSecret,
			factor.TOTPPeriodSeconds,
			factor.TOTPDigits,
			factor.TOTPAlgorithm,
			factor.CreatedAt,
			factor.UpdatedAt,
		)
		if err != nil {
			if isIntegrityConstraint(err) {
				return auth.SecondFactor{}, auth.ErrNotFound
			}
			return auth.SecondFactor{}, fmt.Errorf("insert postgres TOTP second factor: %w", err)
		}
	} else if err != nil {
		return auth.SecondFactor{}, err
	} else if factor.FactorState == auth.SecondFactorStateActive {
		return auth.SecondFactor{}, auth.ErrDuplicate
	} else {
		_, err = tx.ExecContext(ctx, `
			UPDATE account_totp_second_factors
			SET secret = $1, period_seconds = $2, digits = $3, algorithm = $4,
				last_used_time_step = NULL, verified_at = NULL, updated_at = $5
			WHERE id = $6`,
			params.Secret,
			params.PeriodSeconds,
			params.Digits,
			params.Algorithm,
			now,
			factor.ID,
		)
		if err != nil {
			return auth.SecondFactor{}, fmt.Errorf("refresh postgres pending TOTP second factor: %w", err)
		}
		factor.TOTPSecret = params.Secret
		factor.TOTPPeriodSeconds = params.PeriodSeconds
		factor.TOTPDigits = params.Digits
		factor.TOTPAlgorithm = params.Algorithm
		factor.TOTPLastUsedTimeStep = nil
		factor.VerifiedAt = nil
		factor.UpdatedAt = now
	}

	if err := tx.Commit(); err != nil {
		return auth.SecondFactor{}, fmt.Errorf("commit postgres TOTP second factor enrollment: %w", err)
	}
	return factor, nil
}

func (r *Repository) GetPendingTOTPSecondFactor(ctx context.Context, accountID string) (auth.SecondFactor, error) {
	factor, err := r.getTOTPSecondFactorForAccount(ctx, accountID)
	if err != nil {
		return auth.SecondFactor{}, err
	}
	if factor.FactorState != auth.SecondFactorStatePending {
		return auth.SecondFactor{}, auth.ErrNotFound
	}
	return factor, nil
}

func (r *Repository) GetActiveTOTPSecondFactor(ctx context.Context, accountID string) (auth.SecondFactor, error) {
	factor, err := r.getTOTPSecondFactorForAccount(ctx, accountID)
	if err != nil {
		return auth.SecondFactor{}, err
	}
	if factor.FactorState != auth.SecondFactorStateActive {
		return auth.SecondFactor{}, auth.ErrNotFound
	}
	return factor, nil
}

func (r *Repository) ActivateTOTPSecondFactor(ctx context.Context, accountID, factorID string, verifiedAt time.Time, lastUsedTimeStep int64) (auth.SecondFactor, auth.Account, error) {
	verifiedAt = verifiedAt.UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, fmt.Errorf("begin postgres TOTP second factor activation: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE account_totp_second_factors
		SET factor_state = $1, last_used_time_step = $2, verified_at = $3, updated_at = $3
		WHERE id = $4 AND account_id = $5 AND factor_state = $6`,
		auth.SecondFactorStateActive,
		lastUsedTimeStep,
		verifiedAt,
		factorID,
		accountID,
		auth.SecondFactorStatePending,
	)
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, fmt.Errorf("activate postgres TOTP second factor: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, fmt.Errorf("activate postgres TOTP second factor rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return auth.SecondFactor{}, auth.Account{}, auth.ErrNotFound
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE accounts
		SET second_factor_setup_state = $1, updated_at = $2
		WHERE id = $3`,
		auth.SecondFactorSetupStateComplete,
		verifiedAt,
		accountID,
	)
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, fmt.Errorf("mark postgres TOTP second factor setup complete: %w", err)
	}

	factor, err := getTOTPSecondFactorByIDTx(ctx, tx, factorID)
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, err
	}
	account, err := getAccountByIDTx(ctx, tx, accountID)
	if err != nil {
		return auth.SecondFactor{}, auth.Account{}, err
	}
	if err := tx.Commit(); err != nil {
		return auth.SecondFactor{}, auth.Account{}, fmt.Errorf("commit postgres TOTP second factor activation: %w", err)
	}
	return factor, account, nil
}

func (r *Repository) MarkTOTPSecondFactorUsed(ctx context.Context, factorID string, verifiedAt time.Time, lastUsedTimeStep int64) (auth.SecondFactor, error) {
	verifiedAt = verifiedAt.UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE account_totp_second_factors
		SET last_used_time_step = $1, updated_at = $2
		WHERE id = $3
			AND factor_state = $4
			AND (last_used_time_step IS NULL OR last_used_time_step < $1)`,
		lastUsedTimeStep,
		verifiedAt,
		factorID,
		auth.SecondFactorStateActive,
	)
	if err != nil {
		return auth.SecondFactor{}, fmt.Errorf("mark postgres TOTP second factor used: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return auth.SecondFactor{}, fmt.Errorf("mark postgres TOTP second factor used rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return auth.SecondFactor{}, auth.ErrNotFound
	}
	return r.getTOTPSecondFactorByID(ctx, factorID)
}

func (r *Repository) getTOTPSecondFactorForAccount(ctx context.Context, accountID string) (auth.SecondFactor, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, account_id, factor_state, secret, period_seconds, digits, algorithm,
			last_used_time_step, created_at, updated_at, verified_at
		FROM account_totp_second_factors
		WHERE account_id = $1`,
		accountID,
	)
	return scanTOTPSecondFactor(row)
}

func (r *Repository) getTOTPSecondFactorByID(ctx context.Context, factorID string) (auth.SecondFactor, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, account_id, factor_state, secret, period_seconds, digits, algorithm,
			last_used_time_step, created_at, updated_at, verified_at
		FROM account_totp_second_factors
		WHERE id = $1`,
		factorID,
	)
	return scanTOTPSecondFactor(row)
}

func getTOTPSecondFactorForAccountTx(ctx context.Context, tx *sql.Tx, accountID string) (auth.SecondFactor, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, account_id, factor_state, secret, period_seconds, digits, algorithm,
			last_used_time_step, created_at, updated_at, verified_at
		FROM account_totp_second_factors
		WHERE account_id = $1`,
		accountID,
	)
	return scanTOTPSecondFactor(row)
}

func getTOTPSecondFactorByIDTx(ctx context.Context, tx *sql.Tx, factorID string) (auth.SecondFactor, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, account_id, factor_state, secret, period_seconds, digits, algorithm,
			last_used_time_step, created_at, updated_at, verified_at
		FROM account_totp_second_factors
		WHERE id = $1`,
		factorID,
	)
	return scanTOTPSecondFactor(row)
}

func scanTOTPSecondFactor(s scanner) (auth.SecondFactor, error) {
	factor := auth.SecondFactor{FactorType: auth.SecondFactorTypeTOTP}
	var lastUsedTimeStep sql.NullInt64
	var verifiedAt sql.NullTime
	if err := s.Scan(
		&factor.ID,
		&factor.AccountID,
		&factor.FactorState,
		&factor.TOTPSecret,
		&factor.TOTPPeriodSeconds,
		&factor.TOTPDigits,
		&factor.TOTPAlgorithm,
		&lastUsedTimeStep,
		&factor.CreatedAt,
		&factor.UpdatedAt,
		&verifiedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.SecondFactor{}, auth.ErrNotFound
		}
		return auth.SecondFactor{}, err
	}
	if lastUsedTimeStep.Valid {
		factor.TOTPLastUsedTimeStep = &lastUsedTimeStep.Int64
	}
	factor.CreatedAt = factor.CreatedAt.UTC()
	factor.UpdatedAt = factor.UpdatedAt.UTC()
	factor.VerifiedAt = nullableDBTime(verifiedAt)
	return factor, nil
}
