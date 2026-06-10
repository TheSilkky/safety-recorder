package auth

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

const WebAuthnUserHandleBytes = 32

type WebAuthnAccountUser struct {
	Account     Account
	User        WebAuthnUser
	Credentials []WebAuthnCredential
}

var _ webauthn.User = (*WebAuthnAccountUser)(nil)

func GenerateWebAuthnUserHandle() ([]byte, error) {
	handle := make([]byte, WebAuthnUserHandleBytes)
	if _, err := rand.Read(handle); err != nil {
		return nil, fmt.Errorf("generate WebAuthn user handle: %w", err)
	}
	return handle, nil
}

func (u WebAuthnAccountUser) WebAuthnID() []byte {
	return cloneBytes(u.User.UserHandle)
}

func (u WebAuthnAccountUser) WebAuthnName() string {
	return u.Account.Username
}

func (u WebAuthnAccountUser) WebAuthnDisplayName() string {
	return u.Account.Username
}

func (u WebAuthnAccountUser) WebAuthnCredentials() []webauthn.Credential {
	credentials := make([]webauthn.Credential, 0, len(u.Credentials))
	for _, stored := range u.Credentials {
		credentials = append(credentials, WebAuthnCredentialToLibrary(stored))
	}
	return credentials
}

func WebAuthnCredentialToLibrary(stored WebAuthnCredential) webauthn.Credential {
	return webauthn.Credential{
		ID:                cloneBytes(stored.CredentialID),
		PublicKey:         cloneBytes(stored.PublicKey),
		AttestationType:   stored.AttestationType,
		AttestationFormat: stored.AttestationFormat,
		Transport:         webauthnTransportStringsToProtocol(stored.Transports),
		Flags: webauthn.CredentialFlags{
			UserPresent:    stored.UserPresent,
			UserVerified:   stored.UserVerified,
			BackupEligible: stored.BackupEligible,
			BackupState:    stored.BackupState,
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:       cloneBytes(stored.AAGUID),
			SignCount:    stored.SignCount,
			CloneWarning: stored.CloneWarning,
			Attachment:   protocol.AuthenticatorAttachment(stored.Attachment),
		},
	}
}

func WebAuthnCredentialParamsFromLibrary(accountID, rpID string, credential *webauthn.Credential, verifiedAtTime time.Time) CreateWebAuthnCredentialParams {
	return CreateWebAuthnCredentialParams{
		AccountID:         accountID,
		RPID:              rpID,
		CredentialID:      cloneBytes(credential.ID),
		PublicKey:         cloneBytes(credential.PublicKey),
		AttestationType:   credential.AttestationType,
		AttestationFormat: credential.AttestationFormat,
		Transports:        WebAuthnTransportsToStrings(credential.Transport),
		AAGUID:            cloneBytes(credential.Authenticator.AAGUID),
		SignCount:         credential.Authenticator.SignCount,
		CloneWarning:      credential.Authenticator.CloneWarning,
		Attachment:        string(credential.Authenticator.Attachment),
		UserPresent:       credential.Flags.UserPresent,
		UserVerified:      credential.Flags.UserVerified,
		BackupEligible:    credential.Flags.BackupEligible,
		BackupState:       credential.Flags.BackupState,
		VerifiedAt:        verifiedAtTime.UTC(),
	}
}

func WebAuthnCredentialUpdateFromLibrary(stored WebAuthnCredential, credential *webauthn.Credential, verifiedAtTime time.Time) UpdateWebAuthnCredentialParams {
	return UpdateWebAuthnCredentialParams{
		ID:             stored.ID,
		AccountID:      stored.AccountID,
		RPID:           stored.RPID,
		SignCount:      credential.Authenticator.SignCount,
		CloneWarning:   credential.Authenticator.CloneWarning,
		UserPresent:    credential.Flags.UserPresent,
		UserVerified:   credential.Flags.UserVerified,
		BackupEligible: credential.Flags.BackupEligible,
		BackupState:    credential.Flags.BackupState,
		VerifiedAt:     verifiedAtTime.UTC(),
	}
}

func WebAuthnTransportsToStrings(transports []protocol.AuthenticatorTransport) []string {
	if len(transports) == 0 {
		return nil
	}
	values := make([]string, 0, len(transports))
	for _, transport := range transports {
		if transport == "" {
			continue
		}
		values = append(values, string(transport))
	}
	return values
}

func webauthnTransportStringsToProtocol(values []string) []protocol.AuthenticatorTransport {
	if len(values) == 0 {
		return nil
	}
	transports := make([]protocol.AuthenticatorTransport, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		transports = append(transports, protocol.AuthenticatorTransport(value))
	}
	return transports
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	cloned := make([]byte, len(value))
	copy(cloned, value)
	return cloned
}
