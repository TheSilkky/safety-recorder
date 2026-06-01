package incidents

import "time"

const (
	// ContactKeyStatePendingVerification means the owner has not yet verified
	// the contact key out of band.
	ContactKeyStatePendingVerification = "pending_verification"
	// ContactKeyStateActive means the key may be used for new sharing grants.
	ContactKeyStateActive = "active"
	// ContactKeyStateReplaced means a newer key version supersedes this key.
	ContactKeyStateReplaced = "replaced"
	// ContactKeyStateRevoked means the key must not receive new grants.
	ContactKeyStateRevoked = "revoked"
	// ContactKeyStateLost means the contact reported private-key loss.
	ContactKeyStateLost = "lost"

	// SharingGrantRecipientTrustedContact identifies a trusted-contact grant.
	SharingGrantRecipientTrustedContact = "trusted_contact"

	// SharingGrantDataClassMetadata allows incident metadata access.
	SharingGrantDataClassMetadata = "metadata"
	// SharingGrantDataClassCiphertext allows encrypted evidence access.
	SharingGrantDataClassCiphertext = "ciphertext"
	// SharingGrantDataClassMetadataCiphertext allows metadata and ciphertext access.
	SharingGrantDataClassMetadataCiphertext = "metadata_ciphertext"

	// SharingGrantStateActive means a grant can authorize future requests.
	SharingGrantStateActive = "active"
	// SharingGrantStateRevoked means a grant no longer authorizes future requests.
	SharingGrantStateRevoked = "revoked"
)

// ContactPublicKey records an account-owner controlled trusted-contact public
// key. It never contains contact private keys or media keys.
type ContactPublicKey struct {
	ID                   string     `json:"public_key_id"`
	OwnerAccountID       string     `json:"owner_account_id"`
	ContactID            string     `json:"contact_id"`
	Version              int        `json:"version"`
	DisplayLabel         string     `json:"display_label,omitempty"`
	WrappingAlgorithm    string     `json:"wrapping_algorithm"`
	PublicKey            string     `json:"public_key"`
	PublicKeyFingerprint string     `json:"public_key_fingerprint"`
	KeyState             string     `json:"key_state"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	RevokedAt            *time.Time `json:"revoked_at,omitempty"`
}

// CreateContactPublicKeyParams contains owner-scoped public-key metadata.
type CreateContactPublicKeyParams struct {
	OwnerAccountID       string
	ContactID            string
	DisplayLabel         string
	WrappingAlgorithm    string
	PublicKey            string
	PublicKeyFingerprint string
	KeyState             string
}

// UpdateContactPublicKeyParams contains owner-scoped mutable public-key metadata.
type UpdateContactPublicKeyParams struct {
	OwnerAccountID string
	PublicKeyID    string
	DisplayLabel   *string
	KeyState       *string
}

// SharingGrant records an account-owner grant for trusted-contact access to
// metadata and/or encrypted evidence. It does not contain decryption material.
type SharingGrant struct {
	ID                      string     `json:"grant_id"`
	OwnerAccountID          string     `json:"owner_account_id"`
	IncidentID              string     `json:"incident_id"`
	StreamID                string     `json:"stream_id,omitempty"`
	RecipientType           string     `json:"recipient_type"`
	ContactID               string     `json:"contact_id"`
	ContactPublicKeyID      string     `json:"contact_public_key_id"`
	ContactPublicKeyVersion int        `json:"contact_public_key_version"`
	DataClass               string     `json:"data_class"`
	GrantState              string     `json:"grant_state"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
	ExpiresAt               *time.Time `json:"expires_at,omitempty"`
	RevokedAt               *time.Time `json:"revoked_at,omitempty"`
	RevokedByAccountID      string     `json:"revoked_by_account_id,omitempty"`
}

// CreateSharingGrantParams contains owner-scoped sharing grant metadata.
type CreateSharingGrantParams struct {
	OwnerAccountID     string
	IncidentID         string
	StreamID           string
	RecipientType      string
	ContactID          string
	ContactPublicKeyID string
	DataClass          string
	ExpiresAt          *time.Time
}

// ValidContactKeyState reports whether state is a known contact key state.
func ValidContactKeyState(state string) bool {
	switch state {
	case ContactKeyStatePendingVerification, ContactKeyStateActive, ContactKeyStateReplaced, ContactKeyStateRevoked, ContactKeyStateLost:
		return true
	default:
		return false
	}
}

// ValidSharingGrantRecipientType reports whether recipientType is supported.
func ValidSharingGrantRecipientType(recipientType string) bool {
	return recipientType == SharingGrantRecipientTrustedContact
}

// ValidSharingGrantDataClass reports whether dataClass is supported.
func ValidSharingGrantDataClass(dataClass string) bool {
	switch dataClass {
	case SharingGrantDataClassMetadata, SharingGrantDataClassCiphertext, SharingGrantDataClassMetadataCiphertext:
		return true
	default:
		return false
	}
}
