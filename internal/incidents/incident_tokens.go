package incidents

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

// CreateIncidentToken creates a read-only token scoped to one incident and
// returns the raw token once for the caller to share.
func (r *Repository) CreateIncidentToken(ctx context.Context, incidentID, label string, expiresAt *time.Time) (IncidentToken, string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return IncidentToken{}, "", fmt.Errorf("generate incident token: %w", err)
	}
	// Generate a URL-safe 256-bit bearer token and persist only its SHA-256
	// hash so database disclosure does not reveal usable incident viewer links.
	rawToken := base64.RawURLEncoding.EncodeToString(tokenBytes)
	tokenHash := hashIncidentToken(rawToken)

	id, err := newID("itk")
	if err != nil {
		return IncidentToken{}, "", err
	}
	now := time.Now().UTC()
	token := IncidentToken{
		ID:         id,
		IncidentID: incidentID,
		TokenHash:  tokenHash,
		Label:      label,
		CreatedAt:  now,
		ExpiresAt:  utcTimePtr(expiresAt),
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO incident_tokens (
			id, incident_id, token_hash, label, created_at, expires_at
		)
		SELECT ?, id, ?, ?, ?, ?
		FROM incidents
		WHERE id = ? AND deletion_state = ?`,
		token.ID,
		token.TokenHash,
		nullableString(token.Label),
		formatDBTime(token.CreatedAt),
		nullableTime(token.ExpiresAt),
		token.IncidentID,
		IncidentDeletionStateActive,
	)
	if err != nil {
		// Constraint failures include missing incident foreign keys and the
		// vanishingly unlikely token-hash collision; callers treat both as a
		// failed token creation.
		if isConstraint(err) {
			return IncidentToken{}, "", ErrNotFound
		}
		return IncidentToken{}, "", fmt.Errorf("insert incident token: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return IncidentToken{}, "", fmt.Errorf("insert incident token rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return IncidentToken{}, "", ErrNotFound
	}

	return token, rawToken, nil
}

// LookupIncidentToken returns token metadata when rawToken is valid, unexpired,
// and not revoked.
func (r *Repository) LookupIncidentToken(ctx context.Context, rawToken string) (IncidentToken, error) {
	tokenHash := hashIncidentToken(rawToken)
	row := r.db.QueryRowContext(ctx, `
			SELECT incident_tokens.id, incident_tokens.incident_id, incident_tokens.token_hash,
				incident_tokens.label, incident_tokens.created_at, incident_tokens.expires_at,
				incident_tokens.revoked_at
			FROM incident_tokens
			JOIN incidents ON incidents.id = incident_tokens.incident_id
			WHERE incident_tokens.token_hash = ? AND incidents.deletion_state = ?`,
		tokenHash,
		IncidentDeletionStateActive,
	)

	token, err := scanIncidentToken(row)
	if errors.Is(err, sql.ErrNoRows) {
		return IncidentToken{}, ErrNotFound
	}
	if err != nil {
		return IncidentToken{}, fmt.Errorf("lookup incident token: %w", err)
	}
	// The indexed lookup should already match the hash; keep a constant-time
	// comparison as a final equality check before considering token state.
	if subtle.ConstantTimeCompare([]byte(token.TokenHash), []byte(tokenHash)) != 1 {
		return IncidentToken{}, ErrNotFound
	}
	if token.RevokedAt != nil {
		return IncidentToken{}, ErrNotFound
	}
	if token.ExpiresAt != nil && !token.ExpiresAt.After(time.Now().UTC()) {
		return IncidentToken{}, ErrNotFound
	}

	return token, nil
}

// GetIncidentToken returns token metadata by server-generated token ID.
func (r *Repository) GetIncidentToken(ctx context.Context, tokenID string) (IncidentToken, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, incident_id, token_hash, label, created_at, expires_at, revoked_at
		FROM incident_tokens
		WHERE id = ?`,
		tokenID,
	)
	token, err := scanIncidentToken(row)
	if errors.Is(err, sql.ErrNoRows) {
		return IncidentToken{}, ErrNotFound
	}
	if err != nil {
		return IncidentToken{}, fmt.Errorf("get incident token: %w", err)
	}
	return token, nil
}

// ListIncidentTokens returns non-secret token metadata for one incident.
func (r *Repository) ListIncidentTokens(ctx context.Context, incidentID string) ([]IncidentToken, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT incident_tokens.id, incident_tokens.incident_id,
			incident_tokens.token_hash, incident_tokens.label,
			incident_tokens.created_at, incident_tokens.expires_at,
			incident_tokens.revoked_at
		FROM incident_tokens
		JOIN incidents ON incidents.id = incident_tokens.incident_id
		WHERE incident_tokens.incident_id = ? AND incidents.deletion_state = ?
		ORDER BY incident_tokens.created_at, incident_tokens.id`,
		incidentID,
		IncidentDeletionStateActive,
	)
	if err != nil {
		return nil, fmt.Errorf("list incident tokens: %w", err)
	}
	defer rows.Close()

	tokens := []IncidentToken{}
	for rows.Next() {
		token, err := scanIncidentToken(rows)
		if err != nil {
			return nil, fmt.Errorf("scan incident token metadata: %w", err)
		}
		tokens = append(tokens, token)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate incident token metadata: %w", err)
	}
	return tokens, nil
}

// GetIncidentTokenForIncident returns non-secret token metadata for one incident.
func (r *Repository) GetIncidentTokenForIncident(ctx context.Context, incidentID, tokenID string) (IncidentToken, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT incident_tokens.id, incident_tokens.incident_id,
			incident_tokens.token_hash, incident_tokens.label,
			incident_tokens.created_at, incident_tokens.expires_at,
			incident_tokens.revoked_at
		FROM incident_tokens
		JOIN incidents ON incidents.id = incident_tokens.incident_id
		WHERE incident_tokens.incident_id = ?
			AND incident_tokens.id = ?
			AND incidents.deletion_state = ?`,
		incidentID,
		tokenID,
		IncidentDeletionStateActive,
	)
	token, err := scanIncidentToken(row)
	if errors.Is(err, sql.ErrNoRows) {
		return IncidentToken{}, ErrNotFound
	}
	if err != nil {
		return IncidentToken{}, fmt.Errorf("get incident token metadata: %w", err)
	}
	return token, nil
}

// RevokeIncidentToken revokes a token so it can no longer read incident viewer data.
func (r *Repository) RevokeIncidentToken(ctx context.Context, tokenID string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE incident_tokens
		SET revoked_at = ?
		WHERE id = ? AND revoked_at IS NULL`,
		formatDBTime(time.Now().UTC()),
		tokenID,
	)
	if err != nil {
		return fmt.Errorf("revoke incident token: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke incident token rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
