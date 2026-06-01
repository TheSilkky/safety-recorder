package auth

import (
	"errors"
	"time"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

const (
	AccountStatePendingEmailVerification = "pending_email_verification"
	AccountStateActive                   = "active"
	AccountStateDisabled                 = "disabled"
	AccountStateSuspended                = "suspended"
	AccountStatePendingPayment           = "pending_payment"
)

const VerificationPurposeEmail = "email_verification"

var (
	ErrDuplicate = errors.New("duplicate auth row")
	ErrNotFound  = errors.New("auth row not found")
)

type Account struct {
	ID                string     `json:"id"`
	Username          string     `json:"username"`
	EmailNormalized   string     `json:"-"`
	EmailVerifiedAt   *time.Time `json:"-"`
	AccountState      string     `json:"account_state"`
	PasswordHash      string     `json:"-"`
	Role              string     `json:"role"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	PasswordChangedAt time.Time  `json:"password_changed_at"`
}

type Session struct {
	ID        string     `json:"id"`
	AccountID string     `json:"account_id"`
	TokenHash string     `json:"-"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

type CreateAccountParams struct {
	Username        string
	EmailNormalized string
	EmailVerifiedAt *time.Time
	AccountState    string
	PasswordHash    string
	Role            string
}

type AccountVerificationToken struct {
	ID         string
	AccountID  string
	Purpose    string
	TokenHash  string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	ConsumedAt *time.Time
}

type CreateAccountVerificationTokenParams struct {
	AccountID string
	Purpose   string
	ExpiresAt time.Time
}

func ValidRole(role string) bool {
	return role == RoleUser || role == RoleAdmin
}

func ValidAccountState(state string) bool {
	switch state {
	case AccountStatePendingEmailVerification,
		AccountStateActive,
		AccountStateDisabled,
		AccountStateSuspended,
		AccountStatePendingPayment:
		return true
	default:
		return false
	}
}

func CanAuthenticate(account Account) bool {
	return account.AccountState == "" || account.AccountState == AccountStateActive
}
