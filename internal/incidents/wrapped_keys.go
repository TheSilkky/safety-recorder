package incidents

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CreateWrappedKeyRecord stores encrypted media-key material for an active
// owner grant. It never stores raw media keys, contact private keys, or plaintext.
func (r *Repository) CreateWrappedKeyRecord(ctx context.Context, params CreateWrappedKeyRecordParams) (WrappedKeyRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return WrappedKeyRecord{}, fmt.Errorf("begin create wrapped key record: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := requireOwnedActiveIncidentTx(ctx, tx, params.OwnerAccountID, params.IncidentID); err != nil {
		return WrappedKeyRecord{}, err
	}
	if params.StreamID != "" {
		if err := requireIncidentStreamTx(ctx, tx, params.IncidentID, params.StreamID); err != nil {
			return WrappedKeyRecord{}, err
		}
	}

	grant, err := activeGrantForWrappedKey(ctx, tx, params)
	if err != nil {
		return WrappedKeyRecord{}, err
	}
	if grant.DataClass != SharingGrantDataClassCiphertext && grant.DataClass != SharingGrantDataClassMetadataCiphertext {
		return WrappedKeyRecord{}, ErrInvalidState
	}
	if grant.StreamID != "" && grant.StreamID != params.StreamID {
		return WrappedKeyRecord{}, ErrNotFound
	}

	id, err := newID("wkey")
	if err != nil {
		return WrappedKeyRecord{}, err
	}
	now := time.Now().UTC()
	record := WrappedKeyRecord{
		ID:                       id,
		OwnerAccountID:           params.OwnerAccountID,
		IncidentID:               params.IncidentID,
		StreamID:                 params.StreamID,
		GrantID:                  grant.ID,
		RecipientType:            grant.RecipientType,
		ContactID:                grant.ContactID,
		ContactPublicKeyID:       grant.ContactPublicKeyID,
		ContactPublicKeyVersion:  grant.ContactPublicKeyVersion,
		MediaKeyID:               params.MediaKeyID,
		WrappingAlgorithm:        params.WrappingAlgorithm,
		WrappingAlgorithmVersion: params.WrappingAlgorithmVersion,
		WrappedKeyCiphertext:     params.WrappedKeyCiphertext,
		PublicWrappingMetadata:   append([]byte(nil), params.PublicWrappingMetadata...),
		WrappedKeyState:          WrappedKeyStateActive,
		CreatedAt:                now,
		UpdatedAt:                now,
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO wrapped_key_records (
			id, owner_account_id, incident_id, stream_id, grant_id,
			recipient_type, contact_id, contact_public_key_id,
			contact_public_key_version, media_key_id, wrapping_algorithm,
			wrapping_algorithm_version, wrapped_key_ciphertext,
			public_wrapping_metadata, wrapped_key_state, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ID,
		record.OwnerAccountID,
		record.IncidentID,
		nullableString(record.StreamID),
		record.GrantID,
		record.RecipientType,
		record.ContactID,
		record.ContactPublicKeyID,
		record.ContactPublicKeyVersion,
		record.MediaKeyID,
		record.WrappingAlgorithm,
		record.WrappingAlgorithmVersion,
		record.WrappedKeyCiphertext,
		string(record.PublicWrappingMetadata),
		record.WrappedKeyState,
		formatDBTime(record.CreatedAt),
		formatDBTime(record.UpdatedAt),
	); err != nil {
		if isConstraint(err) {
			return WrappedKeyRecord{}, ErrDuplicate
		}
		return WrappedKeyRecord{}, fmt.Errorf("insert wrapped key record: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return WrappedKeyRecord{}, fmt.Errorf("commit create wrapped key record: %w", err)
	}
	return record, nil
}

// ListWrappedKeyRecords returns active, grant-deliverable records for one owner
// incident. Revoked grants, expired grants, revoked contact keys, and revoked
// wrapped-key records are excluded.
func (r *Repository) ListWrappedKeyRecords(ctx context.Context, ownerAccountID, incidentID string) ([]WrappedKeyRecord, error) {
	rows, err := r.db.QueryContext(ctx, wrappedKeyRecordSelect()+activeWrappedKeyJoins()+`
		WHERE w.owner_account_id = ? AND w.incident_id = ?
			AND w.wrapped_key_state = ?
			AND sg.grant_state = ?
			AND (sg.expires_at IS NULL OR sg.expires_at > ?)
			AND cpk.key_state = ?
		ORDER BY w.created_at, w.id`,
		ownerAccountID,
		incidentID,
		WrappedKeyStateActive,
		SharingGrantStateActive,
		formatDBTime(time.Now().UTC()),
		ContactKeyStateActive,
	)
	if err != nil {
		return nil, fmt.Errorf("list wrapped key records: %w", err)
	}
	defer rows.Close()
	return scanWrappedKeyRecordRows(rows)
}

// GetWrappedKeyRecord returns one active, grant-deliverable wrapped-key record.
func (r *Repository) GetWrappedKeyRecord(ctx context.Context, ownerAccountID, wrappedKeyID string) (WrappedKeyRecord, error) {
	row := r.db.QueryRowContext(ctx, wrappedKeyRecordSelect()+activeWrappedKeyJoins()+`
		WHERE w.owner_account_id = ? AND w.id = ?
			AND w.wrapped_key_state = ?
			AND sg.grant_state = ?
			AND (sg.expires_at IS NULL OR sg.expires_at > ?)
			AND cpk.key_state = ?`,
		ownerAccountID,
		wrappedKeyID,
		WrappedKeyStateActive,
		SharingGrantStateActive,
		formatDBTime(time.Now().UTC()),
		ContactKeyStateActive,
	)
	record, err := scanWrappedKeyRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return WrappedKeyRecord{}, ErrNotFound
	}
	if err != nil {
		return WrappedKeyRecord{}, fmt.Errorf("get wrapped key record: %w", err)
	}
	return record, nil
}

// RevokeWrappedKeyRecord marks one owner-scoped wrapped-key record revoked.
func (r *Repository) RevokeWrappedKeyRecord(ctx context.Context, ownerAccountID, wrappedKeyID, revokedByAccountID string) (WrappedKeyRecord, error) {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE wrapped_key_records
		SET wrapped_key_state = ?, updated_at = ?, revoked_at = ?, revoked_by_account_id = ?
		WHERE owner_account_id = ? AND id = ? AND wrapped_key_state = ?`,
		WrappedKeyStateRevoked,
		formatDBTime(now),
		formatDBTime(now),
		nullableString(revokedByAccountID),
		ownerAccountID,
		wrappedKeyID,
		WrappedKeyStateActive,
	)
	if err != nil {
		return WrappedKeyRecord{}, fmt.Errorf("revoke wrapped key record: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return WrappedKeyRecord{}, fmt.Errorf("revoke wrapped key record rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return WrappedKeyRecord{}, ErrNotFound
	}
	return r.getWrappedKeyRecordForOwner(ctx, ownerAccountID, wrappedKeyID)
}

func activeGrantForWrappedKey(ctx context.Context, tx *sql.Tx, params CreateWrappedKeyRecordParams) (SharingGrant, error) {
	row := tx.QueryRowContext(ctx, sharingGrantSelect()+`
		WHERE sharing_grants.owner_account_id = ?
			AND sharing_grants.incident_id = ?
			AND sharing_grants.id = ?
			AND sharing_grants.grant_state = ?
			AND (sharing_grants.expires_at IS NULL OR sharing_grants.expires_at > ?)`,
		params.OwnerAccountID,
		params.IncidentID,
		params.GrantID,
		SharingGrantStateActive,
		formatDBTime(time.Now().UTC()),
	)
	grant, err := scanSharingGrant(row)
	if errors.Is(err, sql.ErrNoRows) {
		return SharingGrant{}, ErrNotFound
	}
	if err != nil {
		return SharingGrant{}, fmt.Errorf("read active sharing grant for wrapped key: %w", err)
	}
	var contactKeyID string
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM contact_public_keys
		WHERE owner_account_id = ? AND id = ? AND key_state = ?`,
		params.OwnerAccountID,
		grant.ContactPublicKeyID,
		ContactKeyStateActive,
	).Scan(&contactKeyID)
	if errors.Is(err, sql.ErrNoRows) {
		return SharingGrant{}, ErrNotFound
	}
	if err != nil {
		return SharingGrant{}, fmt.Errorf("read active contact key for wrapped key: %w", err)
	}
	return grant, nil
}

func (r *Repository) getWrappedKeyRecordForOwner(ctx context.Context, ownerAccountID, wrappedKeyID string) (WrappedKeyRecord, error) {
	row := r.db.QueryRowContext(ctx, wrappedKeyRecordSelect()+`
		WHERE w.owner_account_id = ? AND w.id = ?`,
		ownerAccountID,
		wrappedKeyID,
	)
	record, err := scanWrappedKeyRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return WrappedKeyRecord{}, ErrNotFound
	}
	if err != nil {
		return WrappedKeyRecord{}, fmt.Errorf("get owner wrapped key record: %w", err)
	}
	return record, nil
}

func wrappedKeyRecordSelect() string {
	return `
		SELECT w.id, w.owner_account_id, w.incident_id, w.stream_id, w.grant_id,
			w.recipient_type, w.contact_id, w.contact_public_key_id,
			w.contact_public_key_version, w.media_key_id, w.wrapping_algorithm,
			w.wrapping_algorithm_version, w.wrapped_key_ciphertext,
			w.public_wrapping_metadata, w.wrapped_key_state, w.created_at,
			w.updated_at, w.revoked_at, w.revoked_by_account_id, w.rotated_at
		FROM wrapped_key_records w `
}

func activeWrappedKeyJoins() string {
	return `
		JOIN sharing_grants sg
			ON sg.id = w.grant_id
			AND sg.owner_account_id = w.owner_account_id
		JOIN contact_public_keys cpk
			ON cpk.id = w.contact_public_key_id
			AND cpk.owner_account_id = w.owner_account_id `
}

func scanWrappedKeyRecordRows(rows *sql.Rows) ([]WrappedKeyRecord, error) {
	records := []WrappedKeyRecord{}
	for rows.Next() {
		record, err := scanWrappedKeyRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate wrapped key records: %w", err)
	}
	return records, nil
}
