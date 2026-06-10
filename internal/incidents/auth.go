package incidents

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/open-proofline/server/internal/auth"
)

func (r *Repository) HasAccounts(ctx context.Context) (bool, error) {
	return r.exists(ctx, `SELECT 1 FROM accounts LIMIT 1`)
}

func (r *Repository) HasAdminAccount(ctx context.Context) (bool, error) {
	return r.exists(ctx, `SELECT 1 FROM accounts WHERE role = 'admin' LIMIT 1`)
}

func (r *Repository) exists(ctx context.Context, query string, args ...any) (bool, error) {
	var value int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&value)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func (r *Repository) CreateAccount(ctx context.Context, params auth.CreateAccountParams) (auth.Account, error) {
	id, err := newID("acct")
	if err != nil {
		return auth.Account{}, err
	}
	now := time.Now().UTC()
	account := auth.Account{
		ID:                id,
		Username:          auth.NormalizeUsername(params.Username),
		EmailNormalized:   auth.NormalizeEmail(params.EmailNormalized),
		EmailVerifiedAt:   params.EmailVerifiedAt,
		AccountState:      params.AccountState,
		SecondFactorSetup: params.SecondFactorSetup,
		PasswordHash:      params.PasswordHash,
		Role:              params.Role,
		CreatedAt:         now,
		UpdatedAt:         now,
		PasswordChangedAt: now,
	}
	if account.AccountState == "" {
		account.AccountState = auth.AccountStateActive
	}
	if account.SecondFactorSetup == "" {
		account.SecondFactorSetup = auth.SecondFactorSetupStateNotRequired
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO accounts (
			id, username, email_normalized, email_verified_at, account_state, second_factor_setup_state, password_hash, role, created_at, updated_at, password_changed_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		account.ID,
		account.Username,
		nullableString(account.EmailNormalized),
		nullableTime(account.EmailVerifiedAt),
		account.AccountState,
		account.SecondFactorSetup,
		account.PasswordHash,
		account.Role,
		formatDBTime(account.CreatedAt),
		formatDBTime(account.UpdatedAt),
		formatDBTime(account.PasswordChangedAt),
	)
	if err != nil {
		if isConstraint(err) {
			return auth.Account{}, auth.ErrDuplicate
		}
		return auth.Account{}, fmt.Errorf("insert account: %w", err)
	}
	return account, nil
}

func (r *Repository) GetAccountByUsername(ctx context.Context, username string) (auth.Account, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, username, email_normalized, email_verified_at, account_state, second_factor_setup_state, password_hash, role, created_at, updated_at, password_changed_at
		FROM accounts
		WHERE username = ?`,
		auth.NormalizeUsername(username),
	)
	return scanAccount(row)
}

func (r *Repository) GetAccountByID(ctx context.Context, accountID string) (auth.Account, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, username, email_normalized, email_verified_at, account_state, second_factor_setup_state, password_hash, role, created_at, updated_at, password_changed_at
		FROM accounts
		WHERE id = ?`,
		accountID,
	)
	return scanAccount(row)
}

func (r *Repository) ListAccounts(ctx context.Context) ([]auth.Account, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, username, email_normalized, email_verified_at, account_state, second_factor_setup_state, password_hash, role, created_at, updated_at, password_changed_at
		FROM accounts
		ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	accounts := []auth.Account{}
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate accounts: %w", err)
	}
	return accounts, nil
}

func (r *Repository) UpdateAccountPassword(ctx context.Context, accountID, passwordHash string) (auth.Account, error) {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE accounts
		SET password_hash = ?, updated_at = ?, password_changed_at = ?
		WHERE id = ?`,
		passwordHash,
		formatDBTime(now),
		formatDBTime(now),
		accountID,
	)
	if err != nil {
		return auth.Account{}, fmt.Errorf("update account password: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return auth.Account{}, fmt.Errorf("update account password rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return auth.Account{}, auth.ErrNotFound
	}
	return r.GetAccountByID(ctx, accountID)
}

func (r *Repository) CreateSession(ctx context.Context, accountID string, expiresAt time.Time) (auth.Session, string, error) {
	rawToken, err := newRawAuthToken()
	if err != nil {
		return auth.Session{}, "", err
	}
	tokenHash := auth.SessionTokenHash(rawToken)
	id, err := newID("ses")
	if err != nil {
		return auth.Session{}, "", err
	}
	now := time.Now().UTC()
	session := auth.Session{
		ID:        id,
		AccountID: accountID,
		TokenHash: tokenHash,
		CreatedAt: now,
		ExpiresAt: expiresAt.UTC(),
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO auth_sessions (
			id, account_id, token_hash, created_at, expires_at
		)
		VALUES (?, ?, ?, ?, ?)`,
		session.ID,
		session.AccountID,
		session.TokenHash,
		formatDBTime(session.CreatedAt),
		formatDBTime(session.ExpiresAt),
	)
	if err != nil {
		if isConstraint(err) {
			return auth.Session{}, "", auth.ErrNotFound
		}
		return auth.Session{}, "", fmt.Errorf("insert auth session: %w", err)
	}
	return session, rawToken, nil
}

func (r *Repository) LookupSession(ctx context.Context, rawToken string) (auth.Session, error) {
	tokenHash := auth.SessionTokenHash(rawToken)
	row := r.db.QueryRowContext(ctx, `
		SELECT id, account_id, token_hash, created_at, expires_at, revoked_at
		FROM auth_sessions
		WHERE token_hash = ?`,
		tokenHash,
	)
	session, err := scanSession(row)
	if err != nil {
		return auth.Session{}, err
	}
	if subtle.ConstantTimeCompare([]byte(session.TokenHash), []byte(tokenHash)) != 1 {
		return auth.Session{}, auth.ErrNotFound
	}
	if session.RevokedAt != nil || !session.ExpiresAt.After(time.Now().UTC()) {
		return auth.Session{}, auth.ErrNotFound
	}
	return session, nil
}

func (r *Repository) RevokeSession(ctx context.Context, sessionID string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE auth_sessions
		SET revoked_at = ?
		WHERE id = ? AND revoked_at IS NULL`,
		formatDBTime(time.Now().UTC()),
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("revoke auth session: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke auth session rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return auth.ErrNotFound
	}
	return nil
}

func (r *Repository) RevokeAccountSessions(ctx context.Context, accountID, exceptSessionID string) (int64, error) {
	query := `
		UPDATE auth_sessions
		SET revoked_at = ?
		WHERE account_id = ? AND revoked_at IS NULL`
	args := []any{formatDBTime(time.Now().UTC()), accountID}
	if exceptSessionID != "" {
		query += ` AND id <> ?`
		args = append(args, exceptSessionID)
	}
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("revoke account sessions: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("revoke account sessions rows affected: %w", err)
	}
	return rowsAffected, nil
}

func scanAccount(s scanner) (auth.Account, error) {
	var account auth.Account
	var emailNormalized sql.NullString
	var emailVerifiedAt sql.NullString
	var createdAt string
	var updatedAt string
	var passwordChangedAt string
	if err := s.Scan(
		&account.ID,
		&account.Username,
		&emailNormalized,
		&emailVerifiedAt,
		&account.AccountState,
		&account.SecondFactorSetup,
		&account.PasswordHash,
		&account.Role,
		&createdAt,
		&updatedAt,
		&passwordChangedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.Account{}, auth.ErrNotFound
		}
		return auth.Account{}, err
	}
	if emailNormalized.Valid {
		account.EmailNormalized = emailNormalized.String
	}
	var err error
	if account.EmailVerifiedAt, err = nullableDBTime(emailVerifiedAt); err != nil {
		return auth.Account{}, err
	}
	if account.CreatedAt, err = parseDBTime(createdAt); err != nil {
		return auth.Account{}, err
	}
	if account.UpdatedAt, err = parseDBTime(updatedAt); err != nil {
		return auth.Account{}, err
	}
	if account.PasswordChangedAt, err = parseDBTime(passwordChangedAt); err != nil {
		return auth.Account{}, err
	}
	return account, nil
}

func scanSession(s scanner) (auth.Session, error) {
	var session auth.Session
	var createdAt string
	var expiresAt string
	var revokedAt sql.NullString
	if err := s.Scan(
		&session.ID,
		&session.AccountID,
		&session.TokenHash,
		&createdAt,
		&expiresAt,
		&revokedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.Session{}, auth.ErrNotFound
		}
		return auth.Session{}, err
	}
	var err error
	if session.CreatedAt, err = parseDBTime(createdAt); err != nil {
		return auth.Session{}, err
	}
	if session.ExpiresAt, err = parseDBTime(expiresAt); err != nil {
		return auth.Session{}, err
	}
	if session.RevokedAt, err = nullableDBTime(revokedAt); err != nil {
		return auth.Session{}, err
	}
	return session, nil
}

func newRawAuthToken() (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generate auth token: %w", err)
	}
	return auth.EncodeToken(tokenBytes), nil
}
