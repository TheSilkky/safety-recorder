package postgresdb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/open-proofline/server/internal/auth"
)

func (r *Repository) ResetAccountSecondFactorRecovery(ctx context.Context, params auth.ResetAccountSecondFactorRecoveryParams) (auth.AccountRecoveryEvent, auth.Account, error) {
	if !auth.ValidAccountRecoveryReason(params.Reason) {
		return auth.AccountRecoveryEvent{}, auth.Account{}, fmt.Errorf("invalid account recovery reason")
	}
	id, err := newID("are")
	if err != nil {
		return auth.AccountRecoveryEvent{}, auth.Account{}, err
	}
	now := time.Now().UTC()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.AccountRecoveryEvent{}, auth.Account{}, fmt.Errorf("begin postgres account recovery reset: %w", err)
	}
	defer tx.Rollback()

	account, err := getAccountByIDTx(ctx, tx, params.AccountID)
	if err != nil {
		return auth.AccountRecoveryEvent{}, auth.Account{}, err
	}
	if _, err := getAccountByIDTx(ctx, tx, params.AdminAccountID); err != nil {
		return auth.AccountRecoveryEvent{}, auth.Account{}, err
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM account_second_factor_challenges
		WHERE account_id = $1`,
		params.AccountID,
	); err != nil {
		return auth.AccountRecoveryEvent{}, auth.Account{}, fmt.Errorf("delete postgres email second factor challenges for recovery: %w", err)
	}
	emailFactorsRemoved, err := execRowsAffected(tx.ExecContext(ctx, `
		DELETE FROM account_second_factors
		WHERE account_id = $1`,
		params.AccountID,
	))
	if err != nil {
		return auth.AccountRecoveryEvent{}, auth.Account{}, fmt.Errorf("delete postgres email second factors for recovery: %w", err)
	}
	totpFactorsRemoved, err := execRowsAffected(tx.ExecContext(ctx, `
		DELETE FROM account_totp_second_factors
		WHERE account_id = $1`,
		params.AccountID,
	))
	if err != nil {
		return auth.AccountRecoveryEvent{}, auth.Account{}, fmt.Errorf("delete postgres TOTP second factors for recovery: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM account_webauthn_challenges
		WHERE account_id = $1`,
		params.AccountID,
	); err != nil {
		return auth.AccountRecoveryEvent{}, auth.Account{}, fmt.Errorf("delete postgres WebAuthn challenges for recovery: %w", err)
	}
	webAuthnCredentialsRemoved, err := execRowsAffected(tx.ExecContext(ctx, `
		DELETE FROM account_webauthn_credentials
		WHERE account_id = $1`,
		params.AccountID,
	))
	if err != nil {
		return auth.AccountRecoveryEvent{}, auth.Account{}, fmt.Errorf("delete postgres WebAuthn credentials for recovery: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE accounts
		SET second_factor_setup_state = $1, updated_at = $2
		WHERE id = $3`,
		auth.SecondFactorSetupStateSetupRequired,
		now,
		params.AccountID,
	)
	if err != nil {
		return auth.AccountRecoveryEvent{}, auth.Account{}, fmt.Errorf("mark postgres account recovery setup required: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return auth.AccountRecoveryEvent{}, auth.Account{}, fmt.Errorf("mark postgres account recovery setup required rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return auth.AccountRecoveryEvent{}, auth.Account{}, auth.ErrNotFound
	}

	sessionsRevoked, err := execRowsAffected(tx.ExecContext(ctx, `
		UPDATE auth_sessions
		SET revoked_at = $1
		WHERE account_id = $2 AND revoked_at IS NULL`,
		now,
		params.AccountID,
	))
	if err != nil {
		return auth.AccountRecoveryEvent{}, auth.Account{}, fmt.Errorf("revoke postgres account recovery sessions: %w", err)
	}

	event := auth.AccountRecoveryEvent{
		ID:                             id,
		AccountID:                      params.AccountID,
		AdminAccountID:                 params.AdminAccountID,
		Action:                         auth.AccountRecoveryActionSecondFactorReset,
		Reason:                         params.Reason,
		PreviousSecondFactorSetupState: account.SecondFactorSetup,
		NewSecondFactorSetupState:      auth.SecondFactorSetupStateSetupRequired,
		SessionsRevoked:                sessionsRevoked,
		EmailFactorsRemoved:            emailFactorsRemoved,
		TOTPFactorsRemoved:             totpFactorsRemoved,
		WebAuthnCredentialsRemoved:     webAuthnCredentialsRemoved,
		CreatedAt:                      now,
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO account_recovery_events (
			id, account_id, admin_account_id, action, reason,
			previous_second_factor_setup_state, new_second_factor_setup_state,
			sessions_revoked, email_factors_removed, totp_factors_removed,
			webauthn_credentials_removed, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		event.ID,
		event.AccountID,
		event.AdminAccountID,
		event.Action,
		event.Reason,
		event.PreviousSecondFactorSetupState,
		event.NewSecondFactorSetupState,
		event.SessionsRevoked,
		event.EmailFactorsRemoved,
		event.TOTPFactorsRemoved,
		event.WebAuthnCredentialsRemoved,
		event.CreatedAt,
	)
	if err != nil {
		return auth.AccountRecoveryEvent{}, auth.Account{}, fmt.Errorf("insert postgres account recovery event: %w", err)
	}

	updated, err := getAccountByIDTx(ctx, tx, params.AccountID)
	if err != nil {
		return auth.AccountRecoveryEvent{}, auth.Account{}, err
	}
	if err := tx.Commit(); err != nil {
		return auth.AccountRecoveryEvent{}, auth.Account{}, fmt.Errorf("commit postgres account recovery reset: %w", err)
	}
	return event, updated, nil
}

func execRowsAffected(result sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
