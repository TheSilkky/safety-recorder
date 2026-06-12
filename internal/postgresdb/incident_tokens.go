package postgresdb

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/open-proofline/server/internal/incidents"
)

// CreateIncidentToken creates a read-only token scoped to one incident and
// returns the raw token once for the caller to share.
func (r *Repository) CreateIncidentToken(ctx context.Context, incidentID, label string, expiresAt *time.Time) (incidents.IncidentToken, string, error) {
	rawToken, err := newRawIncidentToken()
	if err != nil {
		return incidents.IncidentToken{}, "", err
	}
	tokenHash := hashIncidentToken(rawToken)

	id, err := newID("itk")
	if err != nil {
		return incidents.IncidentToken{}, "", err
	}
	now := time.Now().UTC()
	token := incidents.IncidentToken{
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
		SELECT $1, id, $2, $3, $4, $5
		FROM incidents
		WHERE id = $6 AND deletion_state = $7`,
		token.ID,
		token.TokenHash,
		nullableString(token.Label),
		token.CreatedAt,
		nullableTime(token.ExpiresAt),
		token.IncidentID,
		incidents.IncidentDeletionStateActive,
	)
	if err != nil {
		if isIntegrityConstraint(err) {
			return incidents.IncidentToken{}, "", incidents.ErrNotFound
		}
		return incidents.IncidentToken{}, "", fmt.Errorf("insert postgres incident token: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return incidents.IncidentToken{}, "", fmt.Errorf("insert postgres incident token rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return incidents.IncidentToken{}, "", incidents.ErrNotFound
	}

	return token, rawToken, nil
}

// LookupIncidentToken returns token metadata when rawToken is valid, unexpired,
// and not revoked.
func (r *Repository) LookupIncidentToken(ctx context.Context, rawToken string) (incidents.IncidentToken, error) {
	tokenHash := hashIncidentToken(rawToken)
	row := r.db.QueryRowContext(ctx, `
			SELECT incident_tokens.id, incident_tokens.incident_id, incident_tokens.token_hash,
				incident_tokens.label, incident_tokens.created_at, incident_tokens.expires_at,
				incident_tokens.revoked_at
			FROM incident_tokens
			JOIN incidents ON incidents.id = incident_tokens.incident_id
			WHERE incident_tokens.token_hash = $1 AND incidents.deletion_state = $2`,
		tokenHash,
		incidents.IncidentDeletionStateActive,
	)

	token, err := scanIncidentToken(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.IncidentToken{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.IncidentToken{}, fmt.Errorf("lookup postgres incident token: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(token.TokenHash), []byte(tokenHash)) != 1 {
		return incidents.IncidentToken{}, incidents.ErrNotFound
	}
	if token.RevokedAt != nil {
		return incidents.IncidentToken{}, incidents.ErrNotFound
	}
	if token.ExpiresAt != nil && !token.ExpiresAt.After(time.Now().UTC()) {
		return incidents.IncidentToken{}, incidents.ErrNotFound
	}

	return token, nil
}

// GetIncidentToken returns token metadata by server-generated token ID.
func (r *Repository) GetIncidentToken(ctx context.Context, tokenID string) (incidents.IncidentToken, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, incident_id, token_hash, label, created_at, expires_at, revoked_at
		FROM incident_tokens
		WHERE id = $1`,
		tokenID,
	)
	token, err := scanIncidentToken(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.IncidentToken{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.IncidentToken{}, fmt.Errorf("get postgres incident token: %w", err)
	}
	return token, nil
}

// ListIncidentTokens returns non-secret token metadata for one incident.
func (r *Repository) ListIncidentTokens(ctx context.Context, incidentID string) ([]incidents.IncidentToken, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT incident_tokens.id, incident_tokens.incident_id,
			incident_tokens.token_hash, incident_tokens.label,
			incident_tokens.created_at, incident_tokens.expires_at,
			incident_tokens.revoked_at
		FROM incident_tokens
		JOIN incidents ON incidents.id = incident_tokens.incident_id
		WHERE incident_tokens.incident_id = $1 AND incidents.deletion_state = $2
		ORDER BY incident_tokens.created_at, incident_tokens.id`,
		incidentID,
		incidents.IncidentDeletionStateActive,
	)
	if err != nil {
		return nil, fmt.Errorf("list postgres incident tokens: %w", err)
	}
	defer rows.Close()

	tokens := []incidents.IncidentToken{}
	for rows.Next() {
		token, err := scanIncidentToken(rows)
		if err != nil {
			return nil, fmt.Errorf("scan postgres incident token metadata: %w", err)
		}
		tokens = append(tokens, token)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres incident token metadata: %w", err)
	}
	return tokens, nil
}

// GetIncidentTokenForIncident returns non-secret token metadata for one incident.
func (r *Repository) GetIncidentTokenForIncident(ctx context.Context, incidentID, tokenID string) (incidents.IncidentToken, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT incident_tokens.id, incident_tokens.incident_id,
			incident_tokens.token_hash, incident_tokens.label,
			incident_tokens.created_at, incident_tokens.expires_at,
			incident_tokens.revoked_at
		FROM incident_tokens
		JOIN incidents ON incidents.id = incident_tokens.incident_id
		WHERE incident_tokens.incident_id = $1
			AND incident_tokens.id = $2
			AND incidents.deletion_state = $3`,
		incidentID,
		tokenID,
		incidents.IncidentDeletionStateActive,
	)
	token, err := scanIncidentToken(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.IncidentToken{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.IncidentToken{}, fmt.Errorf("get postgres incident token metadata: %w", err)
	}
	return token, nil
}

// RevokeIncidentToken revokes a token so it can no longer read incident viewer data.
func (r *Repository) RevokeIncidentToken(ctx context.Context, tokenID string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE incident_tokens
		SET revoked_at = $1
		WHERE id = $2 AND revoked_at IS NULL`,
		time.Now().UTC(),
		tokenID,
	)
	if err != nil {
		return fmt.Errorf("revoke postgres incident token: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke postgres incident token rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return incidents.ErrNotFound
	}
	return nil
}
