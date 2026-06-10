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

const (
	SecondFactorSetupStateNotRequired   = "not_required"
	SecondFactorSetupStateSetupRequired = "setup_required"
	SecondFactorSetupStateComplete      = "complete"
)

const (
	SecondFactorTypeEmailChallenge = "email_challenge"
	SecondFactorTypeTOTP           = "totp"
	SecondFactorTypeWebAuthn       = "webauthn"
)

const (
	SecondFactorStatePending = "pending"
	SecondFactorStateActive  = "active"
)

const (
	SecondFactorChallengeTypeEmailSetup           = "email_setup"
	SecondFactorChallengeTypeWebAuthnRegistration = "webauthn_registration"
	SecondFactorChallengeTypeWebAuthnAssertion    = "webauthn_assertion"
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
	SecondFactorSetup string     `json:"second_factor_setup_state"`
	PasswordHash      string     `json:"-"`
	Role              string     `json:"role"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	PasswordChangedAt time.Time  `json:"password_changed_at"`
}

type Session struct {
	ID                     string     `json:"id"`
	AccountID              string     `json:"account_id"`
	TokenHash              string     `json:"-"`
	SecondFactorVerifiedAt *time.Time `json:"second_factor_verified_at,omitempty"`
	SecondFactorFactorID   string     `json:"-"`
	SecondFactorMethod     string     `json:"second_factor_method,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	ExpiresAt              time.Time  `json:"expires_at"`
	RevokedAt              *time.Time `json:"revoked_at,omitempty"`
}

type CreateAccountParams struct {
	Username          string
	EmailNormalized   string
	EmailVerifiedAt   *time.Time
	AccountState      string
	SecondFactorSetup string
	PasswordHash      string
	Role              string
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

type SecondFactor struct {
	ID                   string
	AccountID            string
	FactorType           string
	EmailNormalized      string
	FactorState          string
	TOTPSecret           string
	TOTPPeriodSeconds    int
	TOTPDigits           int
	TOTPAlgorithm        string
	TOTPLastUsedTimeStep *int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
	VerifiedAt           *time.Time
}

type SecondFactorChallenge struct {
	ID              string
	AccountID       string
	FactorID        string
	ChallengeType   string
	TokenHash       string
	EmailNormalized string
	CreatedAt       time.Time
	ExpiresAt       time.Time
	ConsumedAt      *time.Time
}

type CreateEmailSecondFactorChallengeParams struct {
	AccountID       string
	EmailNormalized string
	ExpiresAt       time.Time
}

type CreateTOTPSecondFactorEnrollmentParams struct {
	AccountID     string
	Secret        string
	PeriodSeconds int
	Digits        int
	Algorithm     string
}

type WebAuthnUser struct {
	AccountID  string
	RPID       string
	UserHandle []byte
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type WebAuthnCredential struct {
	ID                string
	AccountID         string
	RPID              string
	CredentialID      []byte
	PublicKey         []byte
	AttestationType   string
	AttestationFormat string
	Transports        []string
	AAGUID            []byte
	SignCount         uint32
	CloneWarning      bool
	Attachment        string
	UserPresent       bool
	UserVerified      bool
	BackupEligible    bool
	BackupState       bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
	VerifiedAt        *time.Time
	LastUsedAt        *time.Time
}

type WebAuthnChallenge struct {
	ID              string
	AccountID       string
	SessionID       string
	RPID            string
	ChallengeType   string
	SessionDataJSON []byte
	CreatedAt       time.Time
	ExpiresAt       time.Time
	ConsumedAt      *time.Time
}

type CreateWebAuthnChallengeParams struct {
	AccountID       string
	SessionID       string
	RPID            string
	ChallengeType   string
	SessionDataJSON []byte
	ExpiresAt       time.Time
}

type CreateWebAuthnCredentialParams struct {
	AccountID         string
	RPID              string
	CredentialID      []byte
	PublicKey         []byte
	AttestationType   string
	AttestationFormat string
	Transports        []string
	AAGUID            []byte
	SignCount         uint32
	CloneWarning      bool
	Attachment        string
	UserPresent       bool
	UserVerified      bool
	BackupEligible    bool
	BackupState       bool
	VerifiedAt        time.Time
}

type UpdateWebAuthnCredentialParams struct {
	ID             string
	AccountID      string
	RPID           string
	SignCount      uint32
	CloneWarning   bool
	UserPresent    bool
	UserVerified   bool
	BackupEligible bool
	BackupState    bool
	VerifiedAt     time.Time
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

func ValidSecondFactorSetupState(state string) bool {
	switch state {
	case SecondFactorSetupStateNotRequired,
		SecondFactorSetupStateSetupRequired,
		SecondFactorSetupStateComplete:
		return true
	default:
		return false
	}
}

func ValidSecondFactorState(state string) bool {
	switch state {
	case SecondFactorStatePending,
		SecondFactorStateActive:
		return true
	default:
		return false
	}
}

func ValidSecondFactorMethod(method string) bool {
	switch method {
	case SecondFactorTypeTOTP, SecondFactorTypeWebAuthn:
		return true
	default:
		return false
	}
}

func CanAuthenticate(account Account) bool {
	return account.AccountState == "" || account.AccountState == AccountStateActive
}

func RequiresSecondFactorSetup(account Account) bool {
	return account.SecondFactorSetup == SecondFactorSetupStateSetupRequired
}

func CanAccessProductRoutes(account Account) bool {
	return account.SecondFactorSetup == "" ||
		account.SecondFactorSetup == SecondFactorSetupStateNotRequired ||
		account.SecondFactorSetup == SecondFactorSetupStateComplete
}
