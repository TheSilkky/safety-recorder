package postgresdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/open-proofline/server/internal/auth"
)

func (r *Repository) GetOrCreateWebAuthnUser(ctx context.Context, accountID, rpID string) (auth.WebAuthnUser, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.WebAuthnUser{}, fmt.Errorf("begin postgres WebAuthn user: %w", err)
	}
	defer tx.Rollback()

	user, err := getWebAuthnUserTx(ctx, tx, accountID, rpID)
	if err == nil {
		return user, tx.Commit()
	}
	if !errors.Is(err, auth.ErrNotFound) {
		return auth.WebAuthnUser{}, err
	}

	handle, err := auth.GenerateWebAuthnUserHandle()
	if err != nil {
		return auth.WebAuthnUser{}, err
	}
	now := time.Now().UTC()
	user = auth.WebAuthnUser{
		AccountID:  accountID,
		RPID:       rpID,
		UserHandle: handle,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO account_webauthn_users (
			account_id, rp_id, user_handle, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5)`,
		user.AccountID,
		user.RPID,
		user.UserHandle,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		if isIntegrityConstraint(err) {
			if existing, getErr := getWebAuthnUserTx(ctx, tx, accountID, rpID); getErr == nil {
				return existing, tx.Commit()
			}
			return auth.WebAuthnUser{}, auth.ErrNotFound
		}
		return auth.WebAuthnUser{}, fmt.Errorf("insert postgres WebAuthn user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return auth.WebAuthnUser{}, fmt.Errorf("commit postgres WebAuthn user: %w", err)
	}
	return user, nil
}

func (r *Repository) ListWebAuthnCredentials(ctx context.Context, accountID, rpID string) ([]auth.WebAuthnCredential, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, account_id, rp_id, credential_id, public_key, attestation_type,
			attestation_format, transports, aaguid, sign_count, clone_warning,
			attachment, user_present, user_verified, backup_eligible, backup_state,
			created_at, updated_at, verified_at, last_used_at
		FROM account_webauthn_credentials
		WHERE account_id = $1 AND rp_id = $2
		ORDER BY created_at, id`,
		accountID,
		rpID,
	)
	if err != nil {
		return nil, fmt.Errorf("list postgres WebAuthn credentials: %w", err)
	}
	defer rows.Close()

	credentials := []auth.WebAuthnCredential{}
	for rows.Next() {
		credential, err := scanWebAuthnCredential(rows)
		if err != nil {
			return nil, err
		}
		credentials = append(credentials, credential)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres WebAuthn credentials: %w", err)
	}
	return credentials, nil
}

func (r *Repository) HasActiveWebAuthnCredential(ctx context.Context, accountID string) (bool, error) {
	return r.exists(ctx, `
		SELECT 1
		FROM account_webauthn_credentials
		WHERE account_id = $1
		LIMIT 1`,
		accountID,
	)
}

func (r *Repository) CreateWebAuthnChallenge(ctx context.Context, params auth.CreateWebAuthnChallengeParams) (auth.WebAuthnChallenge, error) {
	if params.ChallengeType != auth.SecondFactorChallengeTypeWebAuthnRegistration &&
		params.ChallengeType != auth.SecondFactorChallengeTypeWebAuthnAssertion {
		return auth.WebAuthnChallenge{}, auth.ErrNotFound
	}
	id, err := newID("wac")
	if err != nil {
		return auth.WebAuthnChallenge{}, err
	}
	now := time.Now().UTC()
	challenge := auth.WebAuthnChallenge{
		ID:              id,
		AccountID:       params.AccountID,
		SessionID:       params.SessionID,
		RPID:            params.RPID,
		ChallengeType:   params.ChallengeType,
		SessionDataJSON: append([]byte(nil), params.SessionDataJSON...),
		CreatedAt:       now,
		ExpiresAt:       params.ExpiresAt.UTC(),
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.WebAuthnChallenge{}, fmt.Errorf("begin postgres WebAuthn challenge: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		UPDATE account_webauthn_challenges
		SET consumed_at = COALESCE(consumed_at, $1)
		WHERE account_id = $2 AND auth_session_id = $3 AND rp_id = $4 AND challenge_type = $5 AND consumed_at IS NULL`,
		now,
		challenge.AccountID,
		challenge.SessionID,
		challenge.RPID,
		challenge.ChallengeType,
	)
	if err != nil {
		return auth.WebAuthnChallenge{}, fmt.Errorf("consume previous postgres WebAuthn challenges: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO account_webauthn_challenges (
			id, account_id, auth_session_id, rp_id, challenge_type,
			session_data_json, created_at, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		challenge.ID,
		challenge.AccountID,
		challenge.SessionID,
		challenge.RPID,
		challenge.ChallengeType,
		challenge.SessionDataJSON,
		challenge.CreatedAt,
		challenge.ExpiresAt,
	)
	if err != nil {
		if isIntegrityConstraint(err) {
			return auth.WebAuthnChallenge{}, auth.ErrNotFound
		}
		return auth.WebAuthnChallenge{}, fmt.Errorf("insert postgres WebAuthn challenge: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return auth.WebAuthnChallenge{}, fmt.Errorf("commit postgres WebAuthn challenge: %w", err)
	}
	return challenge, nil
}

func (r *Repository) ConsumeWebAuthnChallenge(ctx context.Context, accountID, sessionID, rpID, challengeType string, now time.Time) (auth.WebAuthnChallenge, error) {
	now = now.UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.WebAuthnChallenge{}, fmt.Errorf("begin postgres WebAuthn challenge consume: %w", err)
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		SELECT id, account_id, auth_session_id, rp_id, challenge_type, session_data_json,
			created_at, expires_at, consumed_at
		FROM account_webauthn_challenges
		WHERE account_id = $1 AND auth_session_id = $2 AND rp_id = $3 AND challenge_type = $4 AND consumed_at IS NULL
		ORDER BY created_at DESC, id DESC
		LIMIT 1`,
		accountID,
		sessionID,
		rpID,
		challengeType,
	)
	challenge, err := scanWebAuthnChallenge(row)
	if err != nil {
		return auth.WebAuthnChallenge{}, err
	}
	if !challenge.ExpiresAt.After(now) {
		return auth.WebAuthnChallenge{}, auth.ErrNotFound
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE account_webauthn_challenges
		SET consumed_at = $1
		WHERE id = $2 AND consumed_at IS NULL`,
		now,
		challenge.ID,
	)
	if err != nil {
		return auth.WebAuthnChallenge{}, fmt.Errorf("consume postgres WebAuthn challenge: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return auth.WebAuthnChallenge{}, fmt.Errorf("consume postgres WebAuthn challenge rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return auth.WebAuthnChallenge{}, auth.ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return auth.WebAuthnChallenge{}, fmt.Errorf("commit postgres WebAuthn challenge consume: %w", err)
	}
	challenge.ConsumedAt = &now
	return challenge, nil
}

func (r *Repository) CreateWebAuthnCredential(ctx context.Context, params auth.CreateWebAuthnCredentialParams) (auth.WebAuthnCredential, auth.Account, error) {
	id, err := newID("sf")
	if err != nil {
		return auth.WebAuthnCredential{}, auth.Account{}, err
	}
	transports, err := marshalWebAuthnTransports(params.Transports)
	if err != nil {
		return auth.WebAuthnCredential{}, auth.Account{}, err
	}
	now := time.Now().UTC()
	verifiedAt := params.VerifiedAt.UTC()
	credential := auth.WebAuthnCredential{
		ID:                id,
		AccountID:         params.AccountID,
		RPID:              params.RPID,
		CredentialID:      append([]byte(nil), params.CredentialID...),
		PublicKey:         append([]byte(nil), params.PublicKey...),
		AttestationType:   params.AttestationType,
		AttestationFormat: params.AttestationFormat,
		Transports:        append([]string(nil), params.Transports...),
		AAGUID:            append([]byte(nil), params.AAGUID...),
		SignCount:         params.SignCount,
		CloneWarning:      params.CloneWarning,
		Attachment:        params.Attachment,
		UserPresent:       params.UserPresent,
		UserVerified:      params.UserVerified,
		BackupEligible:    params.BackupEligible,
		BackupState:       params.BackupState,
		CreatedAt:         now,
		UpdatedAt:         now,
		VerifiedAt:        &verifiedAt,
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.WebAuthnCredential{}, auth.Account{}, fmt.Errorf("begin postgres WebAuthn credential: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO account_webauthn_credentials (
			id, account_id, rp_id, credential_id, public_key, attestation_type,
			attestation_format, transports, aaguid, sign_count, clone_warning,
			attachment, user_present, user_verified, backup_eligible, backup_state,
			created_at, updated_at, verified_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)`,
		credential.ID,
		credential.AccountID,
		credential.RPID,
		credential.CredentialID,
		credential.PublicKey,
		credential.AttestationType,
		credential.AttestationFormat,
		transports,
		nullableBytes(credential.AAGUID),
		credential.SignCount,
		credential.CloneWarning,
		credential.Attachment,
		credential.UserPresent,
		credential.UserVerified,
		credential.BackupEligible,
		credential.BackupState,
		credential.CreatedAt,
		credential.UpdatedAt,
		credential.VerifiedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return auth.WebAuthnCredential{}, auth.Account{}, auth.ErrDuplicate
		}
		if isIntegrityConstraint(err) {
			return auth.WebAuthnCredential{}, auth.Account{}, auth.ErrNotFound
		}
		return auth.WebAuthnCredential{}, auth.Account{}, fmt.Errorf("insert postgres WebAuthn credential: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE accounts
		SET second_factor_setup_state = $1, updated_at = $2
		WHERE id = $3`,
		auth.SecondFactorSetupStateComplete,
		verifiedAt,
		params.AccountID,
	)
	if err != nil {
		return auth.WebAuthnCredential{}, auth.Account{}, fmt.Errorf("mark postgres WebAuthn setup complete: %w", err)
	}

	account, err := getAccountByIDTx(ctx, tx, params.AccountID)
	if err != nil {
		return auth.WebAuthnCredential{}, auth.Account{}, err
	}
	if err := tx.Commit(); err != nil {
		return auth.WebAuthnCredential{}, auth.Account{}, fmt.Errorf("commit postgres WebAuthn credential: %w", err)
	}
	return credential, account, nil
}

func (r *Repository) UpdateWebAuthnCredentialAfterAssertion(ctx context.Context, params auth.UpdateWebAuthnCredentialParams) (auth.WebAuthnCredential, error) {
	verifiedAt := params.VerifiedAt.UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE account_webauthn_credentials
		SET sign_count = $1, clone_warning = $2, user_present = $3, user_verified = $4,
			backup_eligible = $5, backup_state = $6, updated_at = $7, last_used_at = $8
		WHERE id = $9 AND account_id = $10 AND rp_id = $11`,
		params.SignCount,
		params.CloneWarning,
		params.UserPresent,
		params.UserVerified,
		params.BackupEligible,
		params.BackupState,
		verifiedAt,
		verifiedAt,
		params.ID,
		params.AccountID,
		params.RPID,
	)
	if err != nil {
		return auth.WebAuthnCredential{}, fmt.Errorf("update postgres WebAuthn credential: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return auth.WebAuthnCredential{}, fmt.Errorf("update postgres WebAuthn credential rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return auth.WebAuthnCredential{}, auth.ErrNotFound
	}
	return r.getWebAuthnCredentialByID(ctx, params.ID)
}

func (r *Repository) getWebAuthnCredentialByID(ctx context.Context, id string) (auth.WebAuthnCredential, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, account_id, rp_id, credential_id, public_key, attestation_type,
			attestation_format, transports, aaguid, sign_count, clone_warning,
			attachment, user_present, user_verified, backup_eligible, backup_state,
			created_at, updated_at, verified_at, last_used_at
		FROM account_webauthn_credentials
		WHERE id = $1`,
		id,
	)
	return scanWebAuthnCredential(row)
}

func getWebAuthnUserTx(ctx context.Context, tx *sql.Tx, accountID, rpID string) (auth.WebAuthnUser, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT account_id, rp_id, user_handle, created_at, updated_at
		FROM account_webauthn_users
		WHERE account_id = $1 AND rp_id = $2`,
		accountID,
		rpID,
	)
	return scanWebAuthnUser(row)
}

func scanWebAuthnUser(s scanner) (auth.WebAuthnUser, error) {
	var user auth.WebAuthnUser
	if err := s.Scan(
		&user.AccountID,
		&user.RPID,
		&user.UserHandle,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.WebAuthnUser{}, auth.ErrNotFound
		}
		return auth.WebAuthnUser{}, err
	}
	user.CreatedAt = user.CreatedAt.UTC()
	user.UpdatedAt = user.UpdatedAt.UTC()
	return user, nil
}

func scanWebAuthnCredential(s scanner) (auth.WebAuthnCredential, error) {
	var credential auth.WebAuthnCredential
	var transports string
	var signCount int64
	var aaguid []byte
	var verifiedAt sql.NullTime
	var lastUsedAt sql.NullTime
	if err := s.Scan(
		&credential.ID,
		&credential.AccountID,
		&credential.RPID,
		&credential.CredentialID,
		&credential.PublicKey,
		&credential.AttestationType,
		&credential.AttestationFormat,
		&transports,
		&aaguid,
		&signCount,
		&credential.CloneWarning,
		&credential.Attachment,
		&credential.UserPresent,
		&credential.UserVerified,
		&credential.BackupEligible,
		&credential.BackupState,
		&credential.CreatedAt,
		&credential.UpdatedAt,
		&verifiedAt,
		&lastUsedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.WebAuthnCredential{}, auth.ErrNotFound
		}
		return auth.WebAuthnCredential{}, err
	}
	if signCount < 0 || signCount > math.MaxUint32 {
		return auth.WebAuthnCredential{}, fmt.Errorf("invalid postgres WebAuthn sign count")
	}
	parsedTransports, err := unmarshalWebAuthnTransports(transports)
	if err != nil {
		return auth.WebAuthnCredential{}, err
	}
	credential.Transports = parsedTransports
	credential.AAGUID = aaguid
	credential.SignCount = uint32(signCount)
	credential.CreatedAt = credential.CreatedAt.UTC()
	credential.UpdatedAt = credential.UpdatedAt.UTC()
	credential.VerifiedAt = nullableDBTime(verifiedAt)
	credential.LastUsedAt = nullableDBTime(lastUsedAt)
	return credential, nil
}

func scanWebAuthnChallenge(s scanner) (auth.WebAuthnChallenge, error) {
	var challenge auth.WebAuthnChallenge
	var consumedAt sql.NullTime
	if err := s.Scan(
		&challenge.ID,
		&challenge.AccountID,
		&challenge.SessionID,
		&challenge.RPID,
		&challenge.ChallengeType,
		&challenge.SessionDataJSON,
		&challenge.CreatedAt,
		&challenge.ExpiresAt,
		&consumedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.WebAuthnChallenge{}, auth.ErrNotFound
		}
		return auth.WebAuthnChallenge{}, err
	}
	challenge.CreatedAt = challenge.CreatedAt.UTC()
	challenge.ExpiresAt = challenge.ExpiresAt.UTC()
	challenge.ConsumedAt = nullableDBTime(consumedAt)
	return challenge, nil
}

func marshalWebAuthnTransports(transports []string) (string, error) {
	if len(transports) == 0 {
		return "", nil
	}
	data, err := json.Marshal(transports)
	if err != nil {
		return "", fmt.Errorf("marshal postgres WebAuthn transports: %w", err)
	}
	return string(data), nil
}

func unmarshalWebAuthnTransports(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	var transports []string
	if err := json.Unmarshal([]byte(raw), &transports); err != nil {
		return nil, fmt.Errorf("parse postgres WebAuthn transports: %w", err)
	}
	return transports, nil
}

func nullableBytes(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return value
}
