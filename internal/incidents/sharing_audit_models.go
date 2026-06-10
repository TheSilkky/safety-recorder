package incidents

import "time"

const (
	SharingAuditActionContactKeyRegistered   = "contact_key_registered"
	SharingAuditActionContactKeyReplaced     = "contact_key_replaced"
	SharingAuditActionContactKeyRevoked      = "contact_key_revoked"
	SharingAuditActionContactKeyLost         = "contact_key_lost"
	SharingAuditActionSharingGrantCreated    = "sharing_grant_created"
	SharingAuditActionSharingGrantRevoked    = "sharing_grant_revoked"
	SharingAuditActionWrappedKeyCreated      = "wrapped_key_created"
	SharingAuditActionWrappedKeyRevoked      = "wrapped_key_revoked"
	SharingAuditActionIncidentMetadataPruned = "incident_sharing_metadata_pruned"
	SharingAuditOutcomeCreated               = "created"
	SharingAuditOutcomeReplaced              = "replaced"
	SharingAuditOutcomeRevoked               = "revoked"
	SharingAuditOutcomeLost                  = "lost"
	SharingAuditOutcomeDeleted               = "deleted"
)

// SharingAuditEvent records safe lifecycle metadata for contact keys, sharing
// grants, wrapped-key records, and deletion pruning. It intentionally omits
// tokens, request bodies, paths, object keys, plaintext, raw keys, wrapped-key
// ciphertext, public wrapping metadata, and user safety narratives.
type SharingAuditEvent struct {
	ID                 string    `json:"audit_event_id"`
	OwnerAccountID     string    `json:"owner_account_id,omitempty"`
	ActorAccountID     string    `json:"actor_account_id,omitempty"`
	Action             string    `json:"action"`
	OutcomeCategory    string    `json:"outcome_category"`
	IncidentID         string    `json:"incident_id,omitempty"`
	StreamID           string    `json:"stream_id,omitempty"`
	GrantID            string    `json:"grant_id,omitempty"`
	ContactID          string    `json:"contact_id,omitempty"`
	ContactPublicKeyID string    `json:"contact_public_key_id,omitempty"`
	WrappedKeyID       string    `json:"wrapped_key_id,omitempty"`
	DeletionDecisionID string    `json:"deletion_decision_id,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

type SharingAuditEventParams struct {
	OwnerAccountID     string
	ActorAccountID     string
	Action             string
	OutcomeCategory    string
	IncidentID         string
	StreamID           string
	GrantID            string
	ContactID          string
	ContactPublicKeyID string
	WrappedKeyID       string
	DeletionDecisionID string
}

func ValidSharingAuditAction(action string) bool {
	switch action {
	case SharingAuditActionContactKeyRegistered,
		SharingAuditActionContactKeyReplaced,
		SharingAuditActionContactKeyRevoked,
		SharingAuditActionContactKeyLost,
		SharingAuditActionSharingGrantCreated,
		SharingAuditActionSharingGrantRevoked,
		SharingAuditActionWrappedKeyCreated,
		SharingAuditActionWrappedKeyRevoked,
		SharingAuditActionIncidentMetadataPruned:
		return true
	default:
		return false
	}
}

func ValidSharingAuditOutcome(outcome string) bool {
	switch outcome {
	case SharingAuditOutcomeCreated,
		SharingAuditOutcomeReplaced,
		SharingAuditOutcomeRevoked,
		SharingAuditOutcomeLost,
		SharingAuditOutcomeDeleted:
		return true
	default:
		return false
	}
}
