package postgresdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/open-proofline/server/internal/incidents"
)

// CreateContactPublicKey stores a trusted-contact public key owned by one
// account. It never stores contact private keys or media keys.
func (r *Repository) CreateContactPublicKey(ctx context.Context, params incidents.CreateContactPublicKeyParams) (incidents.ContactPublicKey, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return incidents.ContactPublicKey{}, fmt.Errorf("begin create postgres contact public key: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	contactID := params.ContactID
	version := 1
	if contactID == "" {
		contactID, err = newID("ctc")
		if err != nil {
			return incidents.ContactPublicKey{}, err
		}
	} else {
		if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(version), 0) + 1
			FROM contact_public_keys
			WHERE owner_account_id = $1 AND contact_id = $2`,
			params.OwnerAccountID,
			contactID,
		).Scan(&version); err != nil {
			return incidents.ContactPublicKey{}, fmt.Errorf("read postgres contact public key version: %w", err)
		}
		if version == 1 {
			return incidents.ContactPublicKey{}, incidents.ErrNotFound
		}
	}

	id, err := newID("cpk")
	if err != nil {
		return incidents.ContactPublicKey{}, err
	}
	now := time.Now().UTC()
	contactKey := incidents.ContactPublicKey{
		ID:                   id,
		OwnerAccountID:       params.OwnerAccountID,
		ContactID:            contactID,
		Version:              version,
		DisplayLabel:         params.DisplayLabel,
		WrappingAlgorithm:    params.WrappingAlgorithm,
		PublicKey:            params.PublicKey,
		PublicKeyFingerprint: params.PublicKeyFingerprint,
		KeyState:             params.KeyState,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO contact_public_keys (
			id, owner_account_id, contact_id, version, display_label,
			wrapping_algorithm, public_key, public_key_fingerprint, key_state,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		contactKey.ID,
		contactKey.OwnerAccountID,
		contactKey.ContactID,
		contactKey.Version,
		nullableString(contactKey.DisplayLabel),
		contactKey.WrappingAlgorithm,
		contactKey.PublicKey,
		contactKey.PublicKeyFingerprint,
		contactKey.KeyState,
		contactKey.CreatedAt,
		contactKey.UpdatedAt,
	); err != nil {
		if isIntegrityConstraint(err) {
			return incidents.ContactPublicKey{}, incidents.ErrNotFound
		}
		return incidents.ContactPublicKey{}, fmt.Errorf("insert postgres contact public key: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return incidents.ContactPublicKey{}, fmt.Errorf("commit create postgres contact public key: %w", err)
	}
	return contactKey, nil
}

// ListContactPublicKeys returns public-key records owned by one account.
func (r *Repository) ListContactPublicKeys(ctx context.Context, ownerAccountID string) ([]incidents.ContactPublicKey, error) {
	rows, err := r.db.QueryContext(ctx, contactPublicKeySelect()+`
		WHERE owner_account_id = $1
		ORDER BY contact_id, version, created_at, id`,
		ownerAccountID,
	)
	if err != nil {
		return nil, fmt.Errorf("list postgres contact public keys: %w", err)
	}
	defer rows.Close()
	return scanContactPublicKeyRows(rows)
}

// GetContactPublicKey returns one owner-scoped public-key record.
func (r *Repository) GetContactPublicKey(ctx context.Context, ownerAccountID, publicKeyID string) (incidents.ContactPublicKey, error) {
	row := r.db.QueryRowContext(ctx, contactPublicKeySelect()+`
		WHERE owner_account_id = $1 AND id = $2`,
		ownerAccountID,
		publicKeyID,
	)
	contactKey, err := scanContactPublicKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.ContactPublicKey{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.ContactPublicKey{}, fmt.Errorf("get postgres contact public key: %w", err)
	}
	return contactKey, nil
}

// UpdateContactPublicKey updates owner-scoped mutable contact-key metadata.
func (r *Repository) UpdateContactPublicKey(ctx context.Context, params incidents.UpdateContactPublicKeyParams) (incidents.ContactPublicKey, error) {
	current, err := r.GetContactPublicKey(ctx, params.OwnerAccountID, params.PublicKeyID)
	if err != nil {
		return incidents.ContactPublicKey{}, err
	}
	if params.DisplayLabel != nil {
		current.DisplayLabel = *params.DisplayLabel
	}
	if params.KeyState != nil {
		if current.KeyState == incidents.ContactKeyStateRevoked && *params.KeyState != incidents.ContactKeyStateRevoked {
			return incidents.ContactPublicKey{}, incidents.ErrInvalidState
		}
		current.KeyState = *params.KeyState
		if current.KeyState == incidents.ContactKeyStateRevoked && current.RevokedAt == nil {
			revokedAt := time.Now().UTC()
			current.RevokedAt = &revokedAt
		}
	}
	current.UpdatedAt = time.Now().UTC()

	result, err := r.db.ExecContext(ctx, `
		UPDATE contact_public_keys
		SET display_label = $1, key_state = $2, updated_at = $3, revoked_at = $4
		WHERE owner_account_id = $5 AND id = $6`,
		nullableString(current.DisplayLabel),
		current.KeyState,
		current.UpdatedAt,
		nullableTime(current.RevokedAt),
		params.OwnerAccountID,
		params.PublicKeyID,
	)
	if err != nil {
		return incidents.ContactPublicKey{}, fmt.Errorf("update postgres contact public key: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return incidents.ContactPublicKey{}, fmt.Errorf("update postgres contact public key rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return incidents.ContactPublicKey{}, incidents.ErrNotFound
	}
	return r.GetContactPublicKey(ctx, params.OwnerAccountID, params.PublicKeyID)
}

// RevokeContactPublicKey marks a contact key revoked so it cannot receive new grants.
func (r *Repository) RevokeContactPublicKey(ctx context.Context, ownerAccountID, publicKeyID string) (incidents.ContactPublicKey, error) {
	state := incidents.ContactKeyStateRevoked
	return r.UpdateContactPublicKey(ctx, incidents.UpdateContactPublicKeyParams{
		OwnerAccountID: ownerAccountID,
		PublicKeyID:    publicKeyID,
		KeyState:       &state,
	})
}

// CreateSharingGrant creates an owner-scoped trusted-contact grant for an incident or stream.
func (r *Repository) CreateSharingGrant(ctx context.Context, params incidents.CreateSharingGrantParams) (incidents.SharingGrant, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return incidents.SharingGrant{}, fmt.Errorf("begin create postgres sharing grant: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := requireOwnedActiveIncidentTx(ctx, tx, params.OwnerAccountID, params.IncidentID); err != nil {
		return incidents.SharingGrant{}, err
	}
	if params.StreamID != "" {
		if err := requireIncidentStreamTx(ctx, tx, params.IncidentID, params.StreamID); err != nil {
			return incidents.SharingGrant{}, err
		}
	}

	contactKey, err := activeContactPublicKeyForGrant(ctx, tx, params)
	if err != nil {
		return incidents.SharingGrant{}, err
	}

	id, err := newID("sgr")
	if err != nil {
		return incidents.SharingGrant{}, err
	}
	now := time.Now().UTC()
	grant := incidents.SharingGrant{
		ID:                      id,
		OwnerAccountID:          params.OwnerAccountID,
		IncidentID:              params.IncidentID,
		StreamID:                params.StreamID,
		RecipientType:           params.RecipientType,
		ContactID:               contactKey.ContactID,
		ContactPublicKeyID:      contactKey.ID,
		ContactPublicKeyVersion: contactKey.Version,
		DataClass:               params.DataClass,
		GrantState:              incidents.SharingGrantStateActive,
		CreatedAt:               now,
		UpdatedAt:               now,
		ExpiresAt:               utcTimePtr(params.ExpiresAt),
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sharing_grants (
			id, owner_account_id, incident_id, stream_id, recipient_type,
			contact_id, contact_public_key_id, contact_public_key_version,
			data_class, grant_state, created_at, updated_at, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		grant.ID,
		grant.OwnerAccountID,
		grant.IncidentID,
		nullableString(grant.StreamID),
		grant.RecipientType,
		grant.ContactID,
		grant.ContactPublicKeyID,
		grant.ContactPublicKeyVersion,
		grant.DataClass,
		grant.GrantState,
		grant.CreatedAt,
		grant.UpdatedAt,
		nullableTime(grant.ExpiresAt),
	); err != nil {
		if isIntegrityConstraint(err) {
			return incidents.SharingGrant{}, incidents.ErrNotFound
		}
		return incidents.SharingGrant{}, fmt.Errorf("insert postgres sharing grant: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return incidents.SharingGrant{}, fmt.Errorf("commit create postgres sharing grant: %w", err)
	}
	return grant, nil
}

// ListSharingGrants returns owner-scoped grants for one incident.
func (r *Repository) ListSharingGrants(ctx context.Context, ownerAccountID, incidentID string) ([]incidents.SharingGrant, error) {
	rows, err := r.db.QueryContext(ctx, sharingGrantSelect()+`
		WHERE owner_account_id = $1 AND incident_id = $2
		ORDER BY created_at, id`,
		ownerAccountID,
		incidentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list postgres sharing grants: %w", err)
	}
	defer rows.Close()
	return scanSharingGrantRows(rows)
}

// GetSharingGrant returns one owner-scoped grant.
func (r *Repository) GetSharingGrant(ctx context.Context, ownerAccountID, grantID string) (incidents.SharingGrant, error) {
	row := r.db.QueryRowContext(ctx, sharingGrantSelect()+`
		WHERE owner_account_id = $1 AND id = $2`,
		ownerAccountID,
		grantID,
	)
	grant, err := scanSharingGrant(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.SharingGrant{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.SharingGrant{}, fmt.Errorf("get postgres sharing grant: %w", err)
	}
	return grant, nil
}

// RevokeSharingGrant marks a grant revoked without deleting its audit metadata.
func (r *Repository) RevokeSharingGrant(ctx context.Context, ownerAccountID, grantID, revokedByAccountID string) (incidents.SharingGrant, error) {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE sharing_grants
		SET grant_state = $1, updated_at = $2, revoked_at = $3, revoked_by_account_id = $4
		WHERE owner_account_id = $5 AND id = $6 AND revoked_at IS NULL`,
		incidents.SharingGrantStateRevoked,
		now,
		now,
		nullableString(revokedByAccountID),
		ownerAccountID,
		grantID,
	)
	if err != nil {
		return incidents.SharingGrant{}, fmt.Errorf("revoke postgres sharing grant: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return incidents.SharingGrant{}, fmt.Errorf("revoke postgres sharing grant rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return incidents.SharingGrant{}, incidents.ErrNotFound
	}
	return r.GetSharingGrant(ctx, ownerAccountID, grantID)
}

func requireOwnedActiveIncidentTx(ctx context.Context, tx *sql.Tx, ownerAccountID, incidentID string) error {
	var found string
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM incidents
		WHERE id = $1 AND owner_account_id = $2 AND deletion_state = $3
		FOR UPDATE`,
		incidentID,
		ownerAccountID,
		incidents.IncidentDeletionStateActive,
	).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read postgres owned active incident: %w", err)
	}
	return nil
}

func requireIncidentStreamTx(ctx context.Context, tx *sql.Tx, incidentID, streamID string) error {
	var found string
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM media_streams
		WHERE id = $1 AND incident_id = $2`,
		streamID,
		incidentID,
	).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read postgres incident stream: %w", err)
	}
	return nil
}

func activeContactPublicKeyForGrant(ctx context.Context, tx *sql.Tx, params incidents.CreateSharingGrantParams) (incidents.ContactPublicKey, error) {
	var row *sql.Row
	if params.ContactPublicKeyID != "" {
		row = tx.QueryRowContext(ctx, contactPublicKeySelect()+`
			WHERE owner_account_id = $1 AND contact_id = $2 AND id = $3 AND key_state = $4`,
			params.OwnerAccountID,
			params.ContactID,
			params.ContactPublicKeyID,
			incidents.ContactKeyStateActive,
		)
	} else {
		row = tx.QueryRowContext(ctx, contactPublicKeySelect()+`
			WHERE owner_account_id = $1 AND contact_id = $2 AND key_state = $3
			ORDER BY version DESC, created_at DESC, id DESC
			LIMIT 1`,
			params.OwnerAccountID,
			params.ContactID,
			incidents.ContactKeyStateActive,
		)
	}
	contactKey, err := scanContactPublicKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.ContactPublicKey{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.ContactPublicKey{}, fmt.Errorf("read postgres active contact public key: %w", err)
	}
	return contactKey, nil
}

func contactPublicKeySelect() string {
	return `
		SELECT id, owner_account_id, contact_id, version, display_label,
			wrapping_algorithm, public_key, public_key_fingerprint, key_state,
			created_at, updated_at, revoked_at
		FROM contact_public_keys `
}

func sharingGrantSelect() string {
	return `
		SELECT id, owner_account_id, incident_id, stream_id, recipient_type,
			contact_id, contact_public_key_id, contact_public_key_version,
			data_class, grant_state, created_at, updated_at, expires_at,
			revoked_at, revoked_by_account_id
		FROM sharing_grants `
}

func scanContactPublicKeyRows(rows *sql.Rows) ([]incidents.ContactPublicKey, error) {
	contactKeys := []incidents.ContactPublicKey{}
	for rows.Next() {
		contactKey, err := scanContactPublicKey(rows)
		if err != nil {
			return nil, err
		}
		contactKeys = append(contactKeys, contactKey)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres contact public keys: %w", err)
	}
	return contactKeys, nil
}

func scanSharingGrantRows(rows *sql.Rows) ([]incidents.SharingGrant, error) {
	grants := []incidents.SharingGrant{}
	for rows.Next() {
		grant, err := scanSharingGrant(rows)
		if err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres sharing grants: %w", err)
	}
	return grants, nil
}
