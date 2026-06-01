package incidents

import (
	"encoding/json"
	"time"
)

const (
	// WrappedKeyStateActive means the record can be delivered under an active grant.
	WrappedKeyStateActive = "active"
	// WrappedKeyStateRevoked means the record must not be delivered.
	WrappedKeyStateRevoked = "revoked"
	// WrappedKeyStateRotated means a newer media-key generation supersedes this record.
	WrappedKeyStateRotated = "rotated"
)

// WrappedKeyRecord stores encrypted media-key material for an authorized
// recipient. It never contains raw media keys, contact private keys, or plaintext.
type WrappedKeyRecord struct {
	ID                       string          `json:"wrapped_key_id"`
	OwnerAccountID           string          `json:"owner_account_id"`
	IncidentID               string          `json:"incident_id"`
	StreamID                 string          `json:"stream_id,omitempty"`
	GrantID                  string          `json:"grant_id"`
	RecipientType            string          `json:"recipient_type"`
	ContactID                string          `json:"contact_id"`
	ContactPublicKeyID       string          `json:"contact_public_key_id"`
	ContactPublicKeyVersion  int             `json:"contact_public_key_version"`
	MediaKeyID               string          `json:"media_key_id"`
	WrappingAlgorithm        string          `json:"wrapping_algorithm"`
	WrappingAlgorithmVersion string          `json:"wrapping_algorithm_version"`
	WrappedKeyCiphertext     string          `json:"wrapped_key_ciphertext"`
	PublicWrappingMetadata   json.RawMessage `json:"public_wrapping_metadata"`
	WrappedKeyState          string          `json:"wrapped_key_state"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`
	RevokedAt                *time.Time      `json:"revoked_at,omitempty"`
	RevokedByAccountID       string          `json:"revoked_by_account_id,omitempty"`
	RotatedAt                *time.Time      `json:"rotated_at,omitempty"`
}

// CreateWrappedKeyRecordParams contains owner-scoped wrapped-key metadata.
type CreateWrappedKeyRecordParams struct {
	OwnerAccountID           string
	IncidentID               string
	StreamID                 string
	GrantID                  string
	MediaKeyID               string
	WrappingAlgorithm        string
	WrappingAlgorithmVersion string
	WrappedKeyCiphertext     string
	PublicWrappingMetadata   json.RawMessage
}

// ValidWrappedKeyState reports whether state is a known wrapped-key state.
func ValidWrappedKeyState(state string) bool {
	switch state {
	case WrappedKeyStateActive, WrappedKeyStateRevoked, WrappedKeyStateRotated:
		return true
	default:
		return false
	}
}
