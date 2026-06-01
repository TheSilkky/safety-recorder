package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

const (
	defaultWebAuthEnabled           = false
	defaultWebSessionCookieName     = "__Host-proofline_session"
	defaultWebSessionCookieSecure   = true
	defaultWebSessionCookieSameSite = "lax"
	defaultWebCSRFHeaderName        = "X-CSRF-Token"
	webSessionCookieSameSiteLax     = "lax"
	webSessionCookieSameSiteStrict  = "strict"
)

func webAuthConfigFromEnv() (WebAuthConfig, error) {
	enabled, err := boolFromEnv("SAFE_WEB_AUTH_ENABLED", defaultWebAuthEnabled)
	if err != nil {
		return WebAuthConfig{}, err
	}
	allowedOrigins, err := webAllowedOriginsFromEnv()
	if err != nil {
		return WebAuthConfig{}, err
	}
	cookieSecure, err := boolFromEnv("SAFE_WEB_SESSION_COOKIE_SECURE", defaultWebSessionCookieSecure)
	if err != nil {
		return WebAuthConfig{}, err
	}
	cookieName := strings.TrimSpace(envOrDefault("SAFE_WEB_SESSION_COOKIE_NAME", defaultWebSessionCookieName))
	if cookieName == "" {
		return WebAuthConfig{}, fmt.Errorf("parse SAFE_WEB_SESSION_COOKIE_NAME: cookie name is required")
	}
	if !isHTTPToken(cookieName) {
		return WebAuthConfig{}, fmt.Errorf("parse SAFE_WEB_SESSION_COOKIE_NAME: cookie name contains invalid characters")
	}
	if strings.HasPrefix(cookieName, "__Host-") && !cookieSecure {
		return WebAuthConfig{}, fmt.Errorf("parse SAFE_WEB_SESSION_COOKIE_NAME: __Host- cookies require SAFE_WEB_SESSION_COOKIE_SECURE=true")
	}

	sameSite := strings.ToLower(strings.TrimSpace(envOrDefault("SAFE_WEB_SESSION_COOKIE_SAMESITE", defaultWebSessionCookieSameSite)))
	switch sameSite {
	case webSessionCookieSameSiteLax, webSessionCookieSameSiteStrict:
	default:
		return WebAuthConfig{}, fmt.Errorf("parse SAFE_WEB_SESSION_COOKIE_SAMESITE: value must be lax or strict")
	}

	csrfHeaderName := strings.TrimSpace(envOrDefault("SAFE_WEB_CSRF_HEADER_NAME", defaultWebCSRFHeaderName))
	if csrfHeaderName == "" {
		return WebAuthConfig{}, fmt.Errorf("parse SAFE_WEB_CSRF_HEADER_NAME: header name is required")
	}
	if !isHTTPToken(csrfHeaderName) {
		return WebAuthConfig{}, fmt.Errorf("parse SAFE_WEB_CSRF_HEADER_NAME: header name contains invalid characters")
	}

	if !cookieSecure {
		if len(allowedOrigins) == 0 {
			return WebAuthConfig{}, fmt.Errorf("parse SAFE_WEB_SESSION_COOKIE_SECURE: false requires at least one local web origin")
		}
		for _, origin := range allowedOrigins {
			if !webOriginIsLocal(origin) {
				return WebAuthConfig{}, fmt.Errorf("parse SAFE_WEB_SESSION_COOKIE_SECURE: false is allowed only for local web origins")
			}
		}
	}

	return WebAuthConfig{
		Enabled:               enabled,
		AllowedOrigins:        allowedOrigins,
		SessionCookieName:     cookieName,
		SessionCookieSecure:   cookieSecure,
		SessionCookieSameSite: sameSite,
		CSRFHeaderName:        csrfHeaderName,
	}, nil
}

func webAllowedOriginsFromEnv() ([]string, error) {
	raw := strings.TrimSpace(os.Getenv("SAFE_WEB_ALLOWED_ORIGINS"))
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		origin, err := normalizeWebOrigin(part)
		if err != nil {
			return nil, fmt.Errorf("parse SAFE_WEB_ALLOWED_ORIGINS: %w", err)
		}
		if seen[origin] {
			continue
		}
		seen[origin] = true
		origins = append(origins, origin)
	}
	return origins, nil
}

func normalizeWebOrigin(raw string) (string, error) {
	origin := strings.TrimSpace(raw)
	if origin == "" {
		return "", fmt.Errorf("empty origin")
	}
	if origin == "*" {
		return "", fmt.Errorf("wildcard origin is not allowed with credentials")
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("origin %q must use http or https", origin)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("origin %q must include a host", origin)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("origin %q must not include credentials, query, or fragment", origin)
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("origin %q must not include a path", origin)
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func webOriginIsLocal(origin string) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isHTTPToken(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch > 127 {
			return false
		}
		switch ch {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
			continue
		}
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch >= 'A' && ch <= 'Z' {
			continue
		}
		if ch >= 'a' && ch <= 'z' {
			continue
		}
		return false
	}
	return true
}
