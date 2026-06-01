package postgresdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/open-proofline/server/internal/incidents"
)

// CreateWrappedKeyRecord stores encrypted media-key material for an active
// owner grant. It never stores raw media keys, contact private keys, or plaintext.
func (r *Repository) CreateWrappedKeyRecord(ctx context.Context, params incidents.CreateWrappedKeyRecordParams) (incidents.WrappedKeyRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return incidents.WrappedKeyRecord{}, fmt.Errorf("begin create postgres wrapped key record: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := requireOwnedActiveIncidentTx(ctx, tx, params.OwnerAccountID, params.IncidentID); err != nil {
		return incidents.WrappedKeyRecord{}, err
	}
	if params.StreamID != "" {
		if err := requireIncidentStreamTx(ctx, tx, params.IncidentID, params.StreamID); err != nil {
			return incidents.WrappedKeyRecord{}, err
		}
	}

	grant, err := activeGrantForWrappedKey(ctx, tx, params)
	if err != nil {
		return incidents.WrappedKeyRecord{}, err
	}
	if grant.DataClass != incidents.SharingGrantDataClassCiphertext && grant.DataClass != incidents.SharingGrantDataClassMetadataCiphertext {
		return incidents.WrappedKeyRecord{}, incidents.ErrInvalidState
	}
	if grant.StreamID != "" && grant.StreamID != params.StreamID {
		return incidents.WrappedKeyRecord{}, incidents.ErrNotFound
	}

	id, err := newID("wkey")
	if err != nil {
		return incidents.WrappedKeyRecord{}, err
	}
	now := time.Now().UTC()
	record := incidents.WrappedKeyRecord{
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
		WrappedKeyState:          incidents.WrappedKeyStateActive,
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
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
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
		record.CreatedAt,
		record.UpdatedAt,
	); err != nil {
		if isIntegrityConstraint(err) {
			return incidents.WrappedKeyRecord{}, incidents.ErrDuplicate
		}
		return incidents.WrappedKeyRecord{}, fmt.Errorf("insert postgres wrapped key record: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return incidents.WrappedKeyRecord{}, fmt.Errorf("commit create postgres wrapped key record: %w", err)
	}
	return record, nil
}

// ListWrappedKeyRecords returns active, grant-deliverable records for one owner
// incident. Revoked grants, expired grants, revoked contact keys, and revoked
// wrapped-key records are excluded.
func (r *Repository) ListWrappedKeyRecords(ctx context.Context, ownerAccountID, incidentID string) ([]incidents.WrappedKeyRecord, error) {
	rows, err := r.db.QueryContext(ctx, wrappedKeyRecordSelect()+activeWrappedKeyJoins()+`
		WHERE w.owner_account_id = $1 AND w.incident_id = $2
			AND w.wrapped_key_state = $3
			AND sg.grant_state = $4
			AND (sg.expires_at IS NULL OR sg.expires_at > $5)
			AND cpk.key_state = $6
		ORDER BY w.created_at, w.id`,
		ownerAccountID,
		incidentID,
		incidents.WrappedKeyStateActive,
		incidents.SharingGrantStateActive,
		time.Now().UTC(),
		incidents.ContactKeyStateActive,
	)
	if err != nil {
		return nil, fmt.Errorf("list postgres wrapped key records: %w", err)
	}
	defer rows.Close()
	return scanWrappedKeyRecordRows(rows)
}

// GetWrappedKeyRecord returns one active, grant-deliverable wrapped-key record.
func (r *Repository) GetWrappedKeyRecord(ctx context.Context, ownerAccountID, wrappedKeyID string) (incidents.WrappedKeyRecord, error) {
	row := r.db.QueryRowContext(ctx, wrappedKeyRecordSelect()+activeWrappedKeyJoins()+`
		WHERE w.owner_account_id = $1 AND w.id = $2
			AND w.wrapped_key_state = $3
			AND sg.grant_state = $4
			AND (sg.expires_at IS NULL OR sg.expires_at > $5)
			AND cpk.key_state = $6`,
		ownerAccountID,
		wrappedKeyID,
		incidents.WrappedKeyStateActive,
		incidents.SharingGrantStateActive,
		time.Now().UTC(),
		incidents.ContactKeyStateActive,
	)
	record, err := scanWrappedKeyRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.WrappedKeyRecord{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.WrappedKeyRecord{}, fmt.Errorf("get postgres wrapped key record: %w", err)
	}
	return record, nil
}

// RevokeWrappedKeyRecord marks one owner-scoped wrapped-key record revoked.
func (r *Repository) RevokeWrappedKeyRecord(ctx context.Context, ownerAccountID, wrappedKeyID, revokedByAccountID string) (incidents.WrappedKeyRecord, error) {
	now := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE wrapped_key_records
		SET wrapped_key_state = $1, updated_at = $2, revoked_at = $3, revoked_by_account_id = $4
		WHERE owner_account_id = $5 AND id = $6 AND wrapped_key_state = $7`,
		incidents.WrappedKeyStateRevoked,
		now,
		now,
		nullableString(revokedByAccountID),
		ownerAccountID,
		wrappedKeyID,
		incidents.WrappedKeyStateActive,
	)
	if err != nil {
		return incidents.WrappedKeyRecord{}, fmt.Errorf("revoke postgres wrapped key record: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return incidents.WrappedKeyRecord{}, fmt.Errorf("revoke postgres wrapped key record rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return incidents.WrappedKeyRecord{}, incidents.ErrNotFound
	}
	return r.getWrappedKeyRecordForOwner(ctx, ownerAccountID, wrappedKeyID)
}

func activeGrantForWrappedKey(ctx context.Context, tx *sql.Tx, params incidents.CreateWrappedKeyRecordParams) (incidents.SharingGrant, error) {
	row := tx.QueryRowContext(ctx, sharingGrantSelect()+`
		WHERE sharing_grants.owner_account_id = $1
			AND sharing_grants.incident_id = $2
			AND sharing_grants.id = $3
			AND sharing_grants.grant_state = $4
			AND (sharing_grants.expires_at IS NULL OR sharing_grants.expires_at > $5)`,
		params.OwnerAccountID,
		params.IncidentID,
		params.GrantID,
		incidents.SharingGrantStateActive,
		time.Now().UTC(),
	)
	grant, err := scanSharingGrant(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.SharingGrant{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.SharingGrant{}, fmt.Errorf("read postgres active sharing grant for wrapped key: %w", err)
	}
	var contactKeyID string
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM contact_public_keys
		WHERE owner_account_id = $1 AND id = $2 AND key_state = $3`,
		params.OwnerAccountID,
		grant.ContactPublicKeyID,
		incidents.ContactKeyStateActive,
	).Scan(&contactKeyID)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.SharingGrant{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.SharingGrant{}, fmt.Errorf("read postgres active contact key for wrapped key: %w", err)
	}
	return grant, nil
}

func (r *Repository) getWrappedKeyRecordForOwner(ctx context.Context, ownerAccountID, wrappedKeyID string) (incidents.WrappedKeyRecord, error) {
	row := r.db.QueryRowContext(ctx, wrappedKeyRecordSelect()+`
		WHERE w.owner_account_id = $1 AND w.id = $2`,
		ownerAccountID,
		wrappedKeyID,
	)
	record, err := scanWrappedKeyRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return incidents.WrappedKeyRecord{}, incidents.ErrNotFound
	}
	if err != nil {
		return incidents.WrappedKeyRecord{}, fmt.Errorf("get postgres owner wrapped key record: %w", err)
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

func scanWrappedKeyRecordRows(rows *sql.Rows) ([]incidents.WrappedKeyRecord, error) {
	records := []incidents.WrappedKeyRecord{}
	for rows.Next() {
		record, err := scanWrappedKeyRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate postgres wrapped key records: %w", err)
	}
	return records, nil
}
