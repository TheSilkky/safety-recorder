package auth

import (
	"errors"
	"net/mail"
	"strings"
	"unicode"
)

const MaxEmailLength = 254

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func ValidateEmail(email string) error {
	if email == "" {
		return errors.New("email is required")
	}
	if len(email) > MaxEmailLength {
		return errors.New("email must be at most 254 bytes")
	}
	for _, ch := range email {
		if ch > unicode.MaxASCII || unicode.IsSpace(ch) {
			return errors.New("email must be a single ASCII address")
		}
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || addr.Name != "" {
		return errors.New("email must be a valid address")
	}
	local, domain, ok := strings.Cut(addr.Address, "@")
	if !ok || local == "" || domain == "" || strings.Contains(domain, "..") {
		return errors.New("email must be a valid address")
	}
	return nil
}
