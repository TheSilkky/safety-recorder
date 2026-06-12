package incidents

import "time"

const (
	// AccountRecipientTypeAccount identifies an account-level recipient key.
	AccountRecipientTypeAccount = "account"
	// AccountRecipientTypeDevice identifies one owner device recipient key.
	AccountRecipientTypeDevice = "device"

	// AccountRecipientKeyStatePendingVerification means the owner has not yet
	// verified the public key out of band.
	AccountRecipientKeyStatePendingVerification = "pending_verification"
	// AccountRecipientKeyStateActive means the key may be used for future
	// account/device wrapped-key records.
	AccountRecipientKeyStateActive = "active"
	// AccountRecipientKeyStateReplaced means a newer key version supersedes this key.
	AccountRecipientKeyStateReplaced = "replaced"
	// AccountRecipientKeyStateRevoked means the key must not receive new wraps.
	AccountRecipientKeyStateRevoked = "revoked"
	// AccountRecipientKeyStateLost means the owner reported device/private-key loss.
	AccountRecipientKeyStateLost = "lost"
)

// AccountRecipientKey records account-owned public recipient-key metadata. It
// never contains private keys, media keys, plaintext, or derived wrapping secrets.
type AccountRecipientKey struct {
	ID                       string     `json:"recipient_key_id"`
	OwnerAccountID           string     `json:"owner_account_id"`
	RecipientID              string     `json:"recipient_id"`
	RecipientType            string     `json:"recipient_type"`
	KeyID                    string     `json:"key_id"`
	Version                  int        `json:"version"`
	DisplayLabel             string     `json:"display_label,omitempty"`
	Scheme                   string     `json:"scheme"`
	SuiteID                  string     `json:"suite_id"`
	PublicKey                string     `json:"public_key"`
	PublicKeyFingerprint     string     `json:"public_key_fingerprint"`
	KeyState                 string     `json:"key_state"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
	RevokedAt                *time.Time `json:"revoked_at,omitempty"`
	ReplacedAt               *time.Time `json:"replaced_at,omitempty"`
	LostAt                   *time.Time `json:"lost_at,omitempty"`
	ReplacedByRecipientKeyID string     `json:"replaced_by_recipient_key_id,omitempty"`
}

// CreateAccountRecipientKeyParams contains owner-scoped recipient-key metadata.
type CreateAccountRecipientKeyParams struct {
	OwnerAccountID       string
	RecipientID          string
	RecipientType        string
	KeyID                string
	DisplayLabel         string
	Scheme               string
	SuiteID              string
	PublicKey            string
	PublicKeyFingerprint string
	KeyState             string
}

// UpdateAccountRecipientKeyParams contains owner-scoped mutable recipient-key metadata.
type UpdateAccountRecipientKeyParams struct {
	OwnerAccountID string
	RecipientKeyID string
	DisplayLabel   *string
	KeyState       *string
}

// ReplaceAccountRecipientKeyParams creates a successor key version for one
// account/device recipient identity.
type ReplaceAccountRecipientKeyParams struct {
	OwnerAccountID       string
	RecipientKeyID       string
	KeyID                string
	DisplayLabel         string
	Scheme               string
	SuiteID              string
	PublicKey            string
	PublicKeyFingerprint string
	KeyState             string
}

// ValidAccountRecipientType reports whether recipientType is supported.
func ValidAccountRecipientType(recipientType string) bool {
	switch recipientType {
	case AccountRecipientTypeAccount, AccountRecipientTypeDevice:
		return true
	default:
		return false
	}
}

// ValidAccountRecipientKeyState reports whether state is a known state.
func ValidAccountRecipientKeyState(state string) bool {
	switch state {
	case AccountRecipientKeyStatePendingVerification,
		AccountRecipientKeyStateActive,
		AccountRecipientKeyStateReplaced,
		AccountRecipientKeyStateRevoked,
		AccountRecipientKeyStateLost:
		return true
	default:
		return false
	}
}

// TerminalAccountRecipientKeyState reports whether state prevents future wrapping.
func TerminalAccountRecipientKeyState(state string) bool {
	switch state {
	case AccountRecipientKeyStateReplaced, AccountRecipientKeyStateRevoked, AccountRecipientKeyStateLost:
		return true
	default:
		return false
	}
}
