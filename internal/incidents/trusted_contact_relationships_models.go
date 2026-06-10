package incidents

import "time"

const (
	// TrustedContactRelationshipRoleTrustedContact identifies a general trusted
	// contact relationship. It does not grant key, plaintext, or notification
	// access by itself.
	TrustedContactRelationshipRoleTrustedContact = "trusted_contact"

	// TrustedContactRelationshipStatePendingInvite means the owner invited the
	// recipient account and the recipient has not accepted or declined.
	TrustedContactRelationshipStatePendingInvite = "pending_invite"
	// TrustedContactRelationshipStateActive means the recipient accepted the relationship.
	TrustedContactRelationshipStateActive = "active"
	// TrustedContactRelationshipStateDeclined means the recipient declined the invite.
	TrustedContactRelationshipStateDeclined = "declined"
	// TrustedContactRelationshipStateRevoked means the owner revoked the relationship.
	TrustedContactRelationshipStateRevoked = "revoked"
	// TrustedContactRelationshipStateReplaced means a newer relationship supersedes this one.
	TrustedContactRelationshipStateReplaced = "replaced"
)

// TrustedContactRelationship records account-to-account trusted-contact
// lifecycle metadata. It never stores raw keys, wrapped-key ciphertext,
// plaintext, notification payloads, or emergency-dispatch state.
type TrustedContactRelationship struct {
	ID                       string     `json:"relationship_id"`
	OwnerAccountID           string     `json:"owner_account_id"`
	RecipientAccountID       string     `json:"recipient_account_id"`
	RelationshipRole         string     `json:"relationship_role"`
	RelationshipState        string     `json:"relationship_state"`
	DisplayLabel             string     `json:"display_label,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
	InvitedAt                time.Time  `json:"invited_at"`
	AcceptedAt               *time.Time `json:"accepted_at,omitempty"`
	DeclinedAt               *time.Time `json:"declined_at,omitempty"`
	RevokedAt                *time.Time `json:"revoked_at,omitempty"`
	RevokedByAccountID       string     `json:"revoked_by_account_id,omitempty"`
	ReplacedAt               *time.Time `json:"replaced_at,omitempty"`
	ReplacedByRelationshipID string     `json:"replaced_by_relationship_id,omitempty"`
}

// CreateTrustedContactRelationshipParams contains owner-scoped relationship invite metadata.
type CreateTrustedContactRelationshipParams struct {
	OwnerAccountID     string
	RecipientAccountID string
	RelationshipRole   string
	DisplayLabel       string
}

// ReplaceTrustedContactRelationshipParams creates a successor pending invite
// while marking an existing owner-scoped relationship replaced.
type ReplaceTrustedContactRelationshipParams struct {
	OwnerAccountID     string
	RelationshipID     string
	RecipientAccountID string
	RelationshipRole   string
	DisplayLabel       string
}

// ValidTrustedContactRelationshipRole reports whether role is supported.
func ValidTrustedContactRelationshipRole(role string) bool {
	return role == TrustedContactRelationshipRoleTrustedContact
}

// ValidTrustedContactRelationshipState reports whether state is a known state.
func ValidTrustedContactRelationshipState(state string) bool {
	switch state {
	case TrustedContactRelationshipStatePendingInvite,
		TrustedContactRelationshipStateActive,
		TrustedContactRelationshipStateDeclined,
		TrustedContactRelationshipStateRevoked,
		TrustedContactRelationshipStateReplaced:
		return true
	default:
		return false
	}
}

// TerminalTrustedContactRelationshipState reports whether state closes the lifecycle.
func TerminalTrustedContactRelationshipState(state string) bool {
	switch state {
	case TrustedContactRelationshipStateDeclined,
		TrustedContactRelationshipStateRevoked,
		TrustedContactRelationshipStateReplaced:
		return true
	default:
		return false
	}
}
