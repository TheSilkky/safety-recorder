package incidents

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CreateContactPublicKey stores a trusted-contact public key owned by one
// account. It never stores contact private keys or media keys.
func (r *Repository) CreateContactPublicKey(ctx context.Context, params CreateContactPublicKeyParams) (ContactPublicKey, error) {
	if !ValidContactKeyState(params.KeyState) || TerminalContactKeyState(params.KeyState) {
		return ContactPublicKey{}, ErrInvalidState
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ContactPublicKey{}, fmt.Errorf("begin create contact public key: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	contactID := params.ContactID
	version := 1
	if contactID == "" {
		contactID, err = newID("ctc")
		if err != nil {
			return ContactPublicKey{}, err
		}
	} else {
		if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(version), 0) + 1
			FROM contact_public_keys
			WHERE owner_account_id = ? AND contact_id = ?`,
			params.OwnerAccountID,
			contactID,
		).Scan(&version); err != nil {
			return ContactPublicKey{}, fmt.Errorf("read contact public key version: %w", err)
		}
		if version == 1 {
			return ContactPublicKey{}, ErrNotFound
		}
	}

	contactKey, err := newContactPublicKey(params, contactID, version)
	if err != nil {
		return ContactPublicKey{}, err
	}
	if err := insertContactPublicKeyTx(ctx, tx, contactKey); err != nil {
		return ContactPublicKey{}, err
	}
	if params.ContactID != "" {
		if _, err := markOpenContactKeysReplacedTx(ctx, tx, params.OwnerAccountID, contactID, contactKey.ID, contactKey.CreatedAt); err != nil {
			return ContactPublicKey{}, err
		}
		if _, err := createSharingAuditEventTx(ctx, tx, SharingAuditEventParams{
			OwnerAccountID:     params.OwnerAccountID,
			ActorAccountID:     params.OwnerAccountID,
			Action:             SharingAuditActionContactKeyReplaced,
			OutcomeCategory:    SharingAuditOutcomeReplaced,
			ContactID:          contactID,
			ContactPublicKeyID: contactKey.ID,
		}, contactKey.CreatedAt); err != nil {
			return ContactPublicKey{}, err
		}
	} else {
		if _, err := createSharingAuditEventTx(ctx, tx, SharingAuditEventParams{
			OwnerAccountID:     params.OwnerAccountID,
			ActorAccountID:     params.OwnerAccountID,
			Action:             SharingAuditActionContactKeyRegistered,
			OutcomeCategory:    SharingAuditOutcomeCreated,
			ContactID:          contactID,
			ContactPublicKeyID: contactKey.ID,
		}, contactKey.CreatedAt); err != nil {
			return ContactPublicKey{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return ContactPublicKey{}, fmt.Errorf("commit create contact public key: %w", err)
	}
	return contactKey, nil
}

// ListContactPublicKeys returns public-key records owned by one account.
func (r *Repository) ListContactPublicKeys(ctx context.Context, ownerAccountID string) ([]ContactPublicKey, error) {
	rows, err := r.db.QueryContext(ctx, contactPublicKeySelect()+`
		WHERE owner_account_id = ?
		ORDER BY contact_id, version, created_at, id`,
		ownerAccountID,
	)
	if err != nil {
		return nil, fmt.Errorf("list contact public keys: %w", err)
	}
	defer rows.Close()
	return scanContactPublicKeyRows(rows)
}

// GetContactPublicKey returns one owner-scoped public-key record.
func (r *Repository) GetContactPublicKey(ctx context.Context, ownerAccountID, publicKeyID string) (ContactPublicKey, error) {
	row := r.db.QueryRowContext(ctx, contactPublicKeySelect()+`
		WHERE owner_account_id = ? AND id = ?`,
		ownerAccountID,
		publicKeyID,
	)
	contactKey, err := scanContactPublicKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ContactPublicKey{}, ErrNotFound
	}
	if err != nil {
		return ContactPublicKey{}, fmt.Errorf("get contact public key: %w", err)
	}
	return contactKey, nil
}

// UpdateContactPublicKey updates owner-scoped mutable contact-key metadata.
func (r *Repository) UpdateContactPublicKey(ctx context.Context, params UpdateContactPublicKeyParams) (ContactPublicKey, error) {
	current, err := r.GetContactPublicKey(ctx, params.OwnerAccountID, params.PublicKeyID)
	if err != nil {
		return ContactPublicKey{}, err
	}
	if params.DisplayLabel != nil {
		current.DisplayLabel = *params.DisplayLabel
	}
	if params.KeyState != nil {
		if !allowedContactKeyUpdate(current.KeyState, *params.KeyState) {
			return ContactPublicKey{}, ErrInvalidState
		}
		current.KeyState = *params.KeyState
	}
	current.UpdatedAt = time.Now().UTC()

	result, err := r.db.ExecContext(ctx, `
		UPDATE contact_public_keys
		SET display_label = ?, key_state = ?, updated_at = ?
		WHERE owner_account_id = ? AND id = ?`,
		nullableString(current.DisplayLabel),
		current.KeyState,
		formatDBTime(current.UpdatedAt),
		params.OwnerAccountID,
		params.PublicKeyID,
	)
	if err != nil {
		return ContactPublicKey{}, fmt.Errorf("update contact public key: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return ContactPublicKey{}, fmt.Errorf("update contact public key rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ContactPublicKey{}, ErrNotFound
	}
	return r.GetContactPublicKey(ctx, params.OwnerAccountID, params.PublicKeyID)
}

// RevokeContactPublicKey marks a contact key revoked so it cannot receive new grants.
func (r *Repository) RevokeContactPublicKey(ctx context.Context, ownerAccountID, publicKeyID string) (ContactPublicKey, error) {
	return r.setContactKeyTerminalState(ctx, ownerAccountID, publicKeyID, ContactKeyStateRevoked)
}

// MarkContactPublicKeyLost marks a contact key lost so it cannot receive new grants.
func (r *Repository) MarkContactPublicKeyLost(ctx context.Context, ownerAccountID, publicKeyID string) (ContactPublicKey, error) {
	return r.setContactKeyTerminalState(ctx, ownerAccountID, publicKeyID, ContactKeyStateLost)
}

// ReplaceContactPublicKey creates a successor key version and marks the
// previous trusted-contact public key replaced in one transaction.
func (r *Repository) ReplaceContactPublicKey(ctx context.Context, params ReplaceContactPublicKeyParams) (ContactPublicKey, error) {
	if !ValidContactKeyState(params.KeyState) || TerminalContactKeyState(params.KeyState) {
		return ContactPublicKey{}, ErrInvalidState
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ContactPublicKey{}, fmt.Errorf("begin replace contact public key: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := getContactPublicKeyTx(ctx, tx, params.OwnerAccountID, params.PublicKeyID)
	if err != nil {
		return ContactPublicKey{}, err
	}
	if TerminalContactKeyState(current.KeyState) {
		return ContactPublicKey{}, ErrInvalidState
	}

	var version int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM contact_public_keys
		WHERE owner_account_id = ? AND contact_id = ?`,
		params.OwnerAccountID,
		current.ContactID,
	).Scan(&version); err != nil {
		return ContactPublicKey{}, fmt.Errorf("read replacement contact public key version: %w", err)
	}

	replacement, err := newContactPublicKey(CreateContactPublicKeyParams{
		OwnerAccountID:       params.OwnerAccountID,
		ContactID:            current.ContactID,
		DisplayLabel:         params.DisplayLabel,
		WrappingAlgorithm:    params.WrappingAlgorithm,
		PublicKey:            params.PublicKey,
		PublicKeyFingerprint: params.PublicKeyFingerprint,
		KeyState:             params.KeyState,
	}, current.ContactID, version)
	if err != nil {
		return ContactPublicKey{}, err
	}
	if err := insertContactPublicKeyTx(ctx, tx, replacement); err != nil {
		return ContactPublicKey{}, err
	}

	now := time.Now().UTC()
	rowsAffected, err := markOpenContactKeysReplacedTx(ctx, tx, params.OwnerAccountID, current.ContactID, replacement.ID, now)
	if err != nil {
		return ContactPublicKey{}, err
	}
	if rowsAffected == 0 {
		return ContactPublicKey{}, ErrNotFound
	}
	if _, err := createSharingAuditEventTx(ctx, tx, SharingAuditEventParams{
		OwnerAccountID:     params.OwnerAccountID,
		ActorAccountID:     params.OwnerAccountID,
		Action:             SharingAuditActionContactKeyReplaced,
		OutcomeCategory:    SharingAuditOutcomeReplaced,
		ContactID:          current.ContactID,
		ContactPublicKeyID: replacement.ID,
	}, now); err != nil {
		return ContactPublicKey{}, err
	}
	if err := tx.Commit(); err != nil {
		return ContactPublicKey{}, fmt.Errorf("commit replace contact public key: %w", err)
	}
	return replacement, nil
}

func (r *Repository) setContactKeyTerminalState(ctx context.Context, ownerAccountID, publicKeyID, state string) (ContactPublicKey, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ContactPublicKey{}, fmt.Errorf("begin set contact public key terminal state: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := getContactPublicKeyTx(ctx, tx, ownerAccountID, publicKeyID)
	if err != nil {
		return ContactPublicKey{}, err
	}
	if TerminalContactKeyState(current.KeyState) && current.KeyState != state {
		return ContactPublicKey{}, ErrInvalidState
	}
	if current.KeyState == state {
		return current, nil
	}

	now := time.Now().UTC()
	var revokedAt, lostAt *time.Time
	if state == ContactKeyStateRevoked {
		revokedAt = &now
	}
	if state == ContactKeyStateLost {
		lostAt = &now
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE contact_public_keys
		SET key_state = ?, updated_at = ?, revoked_at = ?, lost_at = ?
		WHERE owner_account_id = ? AND id = ?`,
		state,
		formatDBTime(now),
		nullableTime(revokedAt),
		nullableTime(lostAt),
		ownerAccountID,
		publicKeyID,
	)
	if err != nil {
		return ContactPublicKey{}, fmt.Errorf("set contact public key terminal state: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return ContactPublicKey{}, fmt.Errorf("terminal contact public key rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ContactPublicKey{}, ErrNotFound
	}
	action := SharingAuditActionContactKeyRevoked
	outcome := SharingAuditOutcomeRevoked
	if state == ContactKeyStateLost {
		action = SharingAuditActionContactKeyLost
		outcome = SharingAuditOutcomeLost
	}
	if _, err := createSharingAuditEventTx(ctx, tx, SharingAuditEventParams{
		OwnerAccountID:     ownerAccountID,
		ActorAccountID:     ownerAccountID,
		Action:             action,
		OutcomeCategory:    outcome,
		ContactID:          current.ContactID,
		ContactPublicKeyID: current.ID,
	}, now); err != nil {
		return ContactPublicKey{}, err
	}
	contactKey, err := getContactPublicKeyTx(ctx, tx, ownerAccountID, publicKeyID)
	if err != nil {
		return ContactPublicKey{}, err
	}
	if err := tx.Commit(); err != nil {
		return ContactPublicKey{}, fmt.Errorf("commit set contact public key terminal state: %w", err)
	}
	return contactKey, nil
}

// CreateSharingGrant creates an owner-scoped trusted-contact grant for an incident or stream.
func (r *Repository) CreateSharingGrant(ctx context.Context, params CreateSharingGrantParams) (SharingGrant, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return SharingGrant{}, fmt.Errorf("begin create sharing grant: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := requireOwnedActiveIncidentTx(ctx, tx, params.OwnerAccountID, params.IncidentID); err != nil {
		return SharingGrant{}, err
	}
	if params.StreamID != "" {
		if err := requireIncidentStreamTx(ctx, tx, params.IncidentID, params.StreamID); err != nil {
			return SharingGrant{}, err
		}
	}

	contactKey, err := activeContactPublicKeyForGrant(ctx, tx, params)
	if err != nil {
		return SharingGrant{}, err
	}

	id, err := newID("sgr")
	if err != nil {
		return SharingGrant{}, err
	}
	now := time.Now().UTC()
	grant := SharingGrant{
		ID:                      id,
		OwnerAccountID:          params.OwnerAccountID,
		IncidentID:              params.IncidentID,
		StreamID:                params.StreamID,
		RecipientType:           params.RecipientType,
		ContactID:               contactKey.ContactID,
		ContactPublicKeyID:      contactKey.ID,
		ContactPublicKeyVersion: contactKey.Version,
		DataClass:               params.DataClass,
		GrantState:              SharingGrantStateActive,
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
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
		formatDBTime(grant.CreatedAt),
		formatDBTime(grant.UpdatedAt),
		nullableTime(grant.ExpiresAt),
	); err != nil {
		if isConstraint(err) {
			return SharingGrant{}, ErrNotFound
		}
		return SharingGrant{}, fmt.Errorf("insert sharing grant: %w", err)
	}
	if _, err := createSharingAuditEventTx(ctx, tx, SharingAuditEventParams{
		OwnerAccountID:     grant.OwnerAccountID,
		ActorAccountID:     grant.OwnerAccountID,
		Action:             SharingAuditActionSharingGrantCreated,
		OutcomeCategory:    SharingAuditOutcomeCreated,
		IncidentID:         grant.IncidentID,
		StreamID:           grant.StreamID,
		GrantID:            grant.ID,
		ContactID:          grant.ContactID,
		ContactPublicKeyID: grant.ContactPublicKeyID,
	}, now); err != nil {
		return SharingGrant{}, err
	}
	if err := tx.Commit(); err != nil {
		return SharingGrant{}, fmt.Errorf("commit create sharing grant: %w", err)
	}
	return grant, nil
}

// ListSharingGrants returns owner-scoped grants for one incident.
func (r *Repository) ListSharingGrants(ctx context.Context, ownerAccountID, incidentID string) ([]SharingGrant, error) {
	rows, err := r.db.QueryContext(ctx, sharingGrantSelect()+`
		WHERE owner_account_id = ? AND incident_id = ?
		ORDER BY created_at, id`,
		ownerAccountID,
		incidentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list sharing grants: %w", err)
	}
	defer rows.Close()
	return scanSharingGrantRows(rows)
}

// GetSharingGrant returns one owner-scoped grant.
func (r *Repository) GetSharingGrant(ctx context.Context, ownerAccountID, grantID string) (SharingGrant, error) {
	row := r.db.QueryRowContext(ctx, sharingGrantSelect()+`
		WHERE owner_account_id = ? AND id = ?`,
		ownerAccountID,
		grantID,
	)
	grant, err := scanSharingGrant(row)
	if errors.Is(err, sql.ErrNoRows) {
		return SharingGrant{}, ErrNotFound
	}
	if err != nil {
		return SharingGrant{}, fmt.Errorf("get sharing grant: %w", err)
	}
	return grant, nil
}

func getSharingGrantTx(ctx context.Context, tx *sql.Tx, ownerAccountID, grantID string) (SharingGrant, error) {
	row := tx.QueryRowContext(ctx, sharingGrantSelect()+`
		WHERE owner_account_id = ? AND id = ?`,
		ownerAccountID,
		grantID,
	)
	grant, err := scanSharingGrant(row)
	if errors.Is(err, sql.ErrNoRows) {
		return SharingGrant{}, ErrNotFound
	}
	if err != nil {
		return SharingGrant{}, fmt.Errorf("get sharing grant in transaction: %w", err)
	}
	return grant, nil
}

// RevokeSharingGrant marks a grant revoked without deleting its audit metadata.
func (r *Repository) RevokeSharingGrant(ctx context.Context, ownerAccountID, grantID, revokedByAccountID string) (SharingGrant, error) {
	now := time.Now().UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return SharingGrant{}, fmt.Errorf("begin revoke sharing grant: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		UPDATE sharing_grants
		SET grant_state = ?, updated_at = ?, revoked_at = ?, revoked_by_account_id = ?
		WHERE owner_account_id = ? AND id = ? AND revoked_at IS NULL`,
		SharingGrantStateRevoked,
		formatDBTime(now),
		formatDBTime(now),
		nullableString(revokedByAccountID),
		ownerAccountID,
		grantID,
	)
	if err != nil {
		return SharingGrant{}, fmt.Errorf("revoke sharing grant: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return SharingGrant{}, fmt.Errorf("revoke sharing grant rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return SharingGrant{}, ErrNotFound
	}
	grant, err := getSharingGrantTx(ctx, tx, ownerAccountID, grantID)
	if err != nil {
		return SharingGrant{}, err
	}
	if _, err := createSharingAuditEventTx(ctx, tx, SharingAuditEventParams{
		OwnerAccountID:     ownerAccountID,
		ActorAccountID:     revokedByAccountID,
		Action:             SharingAuditActionSharingGrantRevoked,
		OutcomeCategory:    SharingAuditOutcomeRevoked,
		IncidentID:         grant.IncidentID,
		StreamID:           grant.StreamID,
		GrantID:            grant.ID,
		ContactID:          grant.ContactID,
		ContactPublicKeyID: grant.ContactPublicKeyID,
	}, now); err != nil {
		return SharingGrant{}, err
	}
	if err := tx.Commit(); err != nil {
		return SharingGrant{}, fmt.Errorf("commit revoke sharing grant: %w", err)
	}
	return grant, nil
}

func requireOwnedActiveIncidentTx(ctx context.Context, tx *sql.Tx, ownerAccountID, incidentID string) error {
	var found string
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM incidents
		WHERE id = ? AND owner_account_id = ? AND deletion_state = ?`,
		incidentID,
		ownerAccountID,
		IncidentDeletionStateActive,
	).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read owned active incident: %w", err)
	}
	return nil
}

func requireIncidentStreamTx(ctx context.Context, tx *sql.Tx, incidentID, streamID string) error {
	var found string
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM media_streams
		WHERE id = ? AND incident_id = ?`,
		streamID,
		incidentID,
	).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read incident stream: %w", err)
	}
	return nil
}

func activeContactPublicKeyForGrant(ctx context.Context, tx *sql.Tx, params CreateSharingGrantParams) (ContactPublicKey, error) {
	var row *sql.Row
	if params.ContactPublicKeyID != "" {
		row = tx.QueryRowContext(ctx, contactPublicKeySelect()+`
			WHERE owner_account_id = ? AND contact_id = ? AND id = ? AND key_state = ?`,
			params.OwnerAccountID,
			params.ContactID,
			params.ContactPublicKeyID,
			ContactKeyStateActive,
		)
	} else {
		row = tx.QueryRowContext(ctx, contactPublicKeySelect()+`
			WHERE owner_account_id = ? AND contact_id = ? AND key_state = ?
			ORDER BY version DESC, created_at DESC, id DESC
			LIMIT 1`,
			params.OwnerAccountID,
			params.ContactID,
			ContactKeyStateActive,
		)
	}
	contactKey, err := scanContactPublicKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ContactPublicKey{}, ErrNotFound
	}
	if err != nil {
		return ContactPublicKey{}, fmt.Errorf("read active contact public key: %w", err)
	}
	return contactKey, nil
}

func contactPublicKeySelect() string {
	return `
		SELECT id, owner_account_id, contact_id, version, display_label,
			wrapping_algorithm, public_key, public_key_fingerprint, key_state,
			created_at, updated_at, revoked_at, replaced_at, lost_at,
			replaced_by_public_key_id
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

func scanContactPublicKeyRows(rows *sql.Rows) ([]ContactPublicKey, error) {
	contactKeys := []ContactPublicKey{}
	for rows.Next() {
		contactKey, err := scanContactPublicKey(rows)
		if err != nil {
			return nil, err
		}
		contactKeys = append(contactKeys, contactKey)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contact public keys: %w", err)
	}
	return contactKeys, nil
}

func scanSharingGrantRows(rows *sql.Rows) ([]SharingGrant, error) {
	grants := []SharingGrant{}
	for rows.Next() {
		grant, err := scanSharingGrant(rows)
		if err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sharing grants: %w", err)
	}
	return grants, nil
}

func newContactPublicKey(params CreateContactPublicKeyParams, contactID string, version int) (ContactPublicKey, error) {
	id, err := newID("cpk")
	if err != nil {
		return ContactPublicKey{}, err
	}
	now := time.Now().UTC()
	return ContactPublicKey{
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
	}, nil
}

func insertContactPublicKeyTx(ctx context.Context, tx *sql.Tx, contactKey ContactPublicKey) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO contact_public_keys (
			id, owner_account_id, contact_id, version, display_label,
			wrapping_algorithm, public_key, public_key_fingerprint, key_state,
			created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		contactKey.ID,
		contactKey.OwnerAccountID,
		contactKey.ContactID,
		contactKey.Version,
		nullableString(contactKey.DisplayLabel),
		contactKey.WrappingAlgorithm,
		contactKey.PublicKey,
		contactKey.PublicKeyFingerprint,
		contactKey.KeyState,
		formatDBTime(contactKey.CreatedAt),
		formatDBTime(contactKey.UpdatedAt),
	)
	if err != nil {
		if isConstraint(err) {
			return ErrNotFound
		}
		return fmt.Errorf("insert contact public key: %w", err)
	}
	return nil
}

func getContactPublicKeyTx(ctx context.Context, tx *sql.Tx, ownerAccountID, publicKeyID string) (ContactPublicKey, error) {
	row := tx.QueryRowContext(ctx, contactPublicKeySelect()+`
		WHERE owner_account_id = ? AND id = ?`,
		ownerAccountID,
		publicKeyID,
	)
	contactKey, err := scanContactPublicKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ContactPublicKey{}, ErrNotFound
	}
	if err != nil {
		return ContactPublicKey{}, fmt.Errorf("get contact public key in transaction: %w", err)
	}
	return contactKey, nil
}

func markOpenContactKeysReplacedTx(ctx context.Context, tx *sql.Tx, ownerAccountID, contactID, replacementID string, replacedAt time.Time) (int64, error) {
	result, err := tx.ExecContext(ctx, `
		UPDATE contact_public_keys
		SET key_state = ?, updated_at = ?, replaced_at = ?, replaced_by_public_key_id = ?
		WHERE owner_account_id = ?
			AND contact_id = ?
			AND id <> ?
			AND key_state IN (?, ?)`,
		ContactKeyStateReplaced,
		formatDBTime(replacedAt),
		formatDBTime(replacedAt),
		replacementID,
		ownerAccountID,
		contactID,
		replacementID,
		ContactKeyStatePendingVerification,
		ContactKeyStateActive,
	)
	if err != nil {
		return 0, fmt.Errorf("mark contact public keys replaced: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("replace contact public key rows affected: %w", err)
	}
	return rowsAffected, nil
}

func allowedContactKeyUpdate(currentState, nextState string) bool {
	if currentState == nextState {
		return true
	}
	if TerminalContactKeyState(currentState) || TerminalContactKeyState(nextState) {
		return false
	}
	return nextState == ContactKeyStatePendingVerification || nextState == ContactKeyStateActive
}
