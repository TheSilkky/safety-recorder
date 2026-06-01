package config

import (
	"fmt"
	"net/url"
	"strings"
)

func accountRegistrationConfigFromSource(source configSource) (AccountRegistrationConfig, error) {
	mode := strings.ToLower(strings.TrimSpace(envOrDefault(source, "SAFE_ACCOUNT_REGISTRATION_MODE", AccountRegistrationModeDisabled)))
	switch mode {
	case AccountRegistrationModeDisabled,
		AccountRegistrationModeAdminOnly,
		AccountRegistrationModeOpen,
		AccountRegistrationModePaid:
	default:
		return AccountRegistrationConfig{}, fmt.Errorf("parse SAFE_ACCOUNT_REGISTRATION_MODE: value must be disabled, admin_only, open, or paid")
	}

	ttl, err := durationFromSource(source, "SAFE_EMAIL_VERIFICATION_TTL", defaultEmailVerificationTTL)
	if err != nil {
		return AccountRegistrationConfig{}, err
	}
	if ttl <= 0 {
		return AccountRegistrationConfig{}, fmt.Errorf("parse SAFE_EMAIL_VERIFICATION_TTL: duration must be positive")
	}

	origin := strings.TrimSpace(source.Get("SAFE_PUBLIC_WEB_ORIGIN"))
	if origin != "" {
		origin, err = normalizePublicWebOrigin(origin)
		if err != nil {
			return AccountRegistrationConfig{}, fmt.Errorf("parse SAFE_PUBLIC_WEB_ORIGIN: %w", err)
		}
	}

	return AccountRegistrationConfig{
		Mode:                 mode,
		EmailVerificationTTL: ttl,
		PublicWebOrigin:      origin,
	}, nil
}

func validateAccountRegistrationConfig(cfg AccountRegistrationConfig, email EmailConfig) (AccountRegistrationConfig, error) {
	if cfg.Mode != AccountRegistrationModeOpen {
		return cfg, nil
	}
	if email.Backend == EmailBackendNone {
		return AccountRegistrationConfig{}, fmt.Errorf("parse SAFE_EMAIL_BACKEND: smtp is required when SAFE_ACCOUNT_REGISTRATION_MODE=open")
	}
	if cfg.PublicWebOrigin == "" {
		return AccountRegistrationConfig{}, fmt.Errorf("parse SAFE_PUBLIC_WEB_ORIGIN: required when SAFE_ACCOUNT_REGISTRATION_MODE=open")
	}
	return cfg, nil
}

func normalizePublicWebOrigin(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("invalid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("origin must use http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("origin must include a host")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("origin must not include credentials, query, or fragment")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("origin must not include a path")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}
