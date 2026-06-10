package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/go-webauthn/webauthn/protocol"
)

func webAuthnConfigFromSource(source configSource) (WebAuthnConfig, error) {
	enabled, err := boolFromSource(source, "SAFE_WEBAUTHN_ENABLED", defaultWebAuthnEnabled)
	if err != nil {
		return WebAuthnConfig{}, err
	}
	rpID := strings.TrimSpace(source.Get("SAFE_WEBAUTHN_RP_ID"))
	if rpID != "" {
		if err := protocol.ValidateRPID(rpID); err != nil {
			return WebAuthnConfig{}, fmt.Errorf("parse SAFE_WEBAUTHN_RP_ID: %w", err)
		}
	}
	displayName := strings.TrimSpace(envOrDefault(source, "SAFE_WEBAUTHN_RP_DISPLAY_NAME", defaultWebAuthnRPDisplayName))
	if displayName == "" {
		return WebAuthnConfig{}, fmt.Errorf("parse SAFE_WEBAUTHN_RP_DISPLAY_NAME: display name is required")
	}
	allowedOrigins, err := webAuthnAllowedOriginsFromSource(source)
	if err != nil {
		return WebAuthnConfig{}, err
	}
	userVerification := strings.ToLower(strings.TrimSpace(envOrDefault(source, "SAFE_WEBAUTHN_USER_VERIFICATION", defaultWebAuthnUserVerification)))
	switch protocol.UserVerificationRequirement(userVerification) {
	case protocol.VerificationRequired, protocol.VerificationPreferred, protocol.VerificationDiscouraged:
	default:
		return WebAuthnConfig{}, fmt.Errorf("parse SAFE_WEBAUTHN_USER_VERIFICATION: value must be required, preferred, or discouraged")
	}
	challengeTTL, err := durationFromSource(source, "SAFE_WEBAUTHN_CHALLENGE_TTL", defaultWebAuthnChallengeTTL)
	if err != nil {
		return WebAuthnConfig{}, err
	}
	if challengeTTL <= 0 {
		return WebAuthnConfig{}, fmt.Errorf("parse SAFE_WEBAUTHN_CHALLENGE_TTL: duration must be positive")
	}

	if enabled {
		if rpID == "" {
			return WebAuthnConfig{}, fmt.Errorf("parse SAFE_WEBAUTHN_RP_ID: required when SAFE_WEBAUTHN_ENABLED=true")
		}
		if len(allowedOrigins) == 0 {
			return WebAuthnConfig{}, fmt.Errorf("parse SAFE_WEBAUTHN_ALLOWED_ORIGINS: required when SAFE_WEBAUTHN_ENABLED=true")
		}
	}

	return WebAuthnConfig{
		Enabled:          enabled,
		RPID:             rpID,
		RPDisplayName:    displayName,
		AllowedOrigins:   allowedOrigins,
		UserVerification: userVerification,
		ChallengeTTL:     challengeTTL,
	}, nil
}

func webAuthnAllowedOriginsFromSource(source configSource) ([]string, error) {
	raw := strings.TrimSpace(source.Get("SAFE_WEBAUTHN_ALLOWED_ORIGINS"))
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		origin, err := normalizeWebAuthnOrigin(part)
		if err != nil {
			return nil, fmt.Errorf("parse SAFE_WEBAUTHN_ALLOWED_ORIGINS: %w", err)
		}
		if seen[origin] {
			continue
		}
		seen[origin] = true
		origins = append(origins, origin)
	}
	return origins, nil
}

func normalizeWebAuthnOrigin(raw string) (string, error) {
	if strings.Contains(raw, "*") {
		return "", fmt.Errorf("wildcard origin is not allowed")
	}
	origin, err := normalizeWebOrigin(raw)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "http" && !webOriginIsLocal(origin) {
		return "", fmt.Errorf("origin %q must use https unless it is an explicit local development origin", origin)
	}
	return origin, nil
}
