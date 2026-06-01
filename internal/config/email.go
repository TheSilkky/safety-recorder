package config

import (
	"fmt"
	"net"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultEmailBackend = EmailBackendNone
	defaultSMTPPort     = 587
	defaultSMTPStartTLS = SMTPStartTLSRequired
	defaultSMTPTimeout  = 10 * time.Second
)

func emailConfigFromEnv() (EmailConfig, error) {
	backend := strings.ToLower(strings.TrimSpace(envOrDefault("SAFE_EMAIL_BACKEND", defaultEmailBackend)))
	switch backend {
	case EmailBackendNone, EmailBackendSMTP:
	default:
		return EmailConfig{}, fmt.Errorf("parse SAFE_EMAIL_BACKEND: value must be none or smtp")
	}

	cfg := EmailConfig{
		Backend: backend,
		SMTP: SMTPConfig{
			Port:     defaultSMTPPort,
			StartTLS: defaultSMTPStartTLS,
			Timeout:  defaultSMTPTimeout,
		},
	}
	if backend != EmailBackendSMTP {
		return cfg, nil
	}

	smtpCfg, err := smtpConfigFromEnv()
	if err != nil {
		return EmailConfig{}, err
	}
	cfg.SMTP = smtpCfg
	return cfg, nil
}

func smtpConfigFromEnv() (SMTPConfig, error) {
	port, err := positiveIntFromEnv("SAFE_SMTP_PORT", defaultSMTPPort)
	if err != nil {
		return SMTPConfig{}, err
	}
	startTLS := strings.ToLower(strings.TrimSpace(envOrDefault("SAFE_SMTP_STARTTLS", defaultSMTPStartTLS)))
	switch startTLS {
	case SMTPStartTLSRequired, SMTPStartTLSOpportunistic, SMTPStartTLSDisabled:
	default:
		return SMTPConfig{}, fmt.Errorf("parse SAFE_SMTP_STARTTLS: value must be required, opportunistic, or disabled")
	}
	timeout, err := durationFromEnv("SAFE_SMTP_TIMEOUT", defaultSMTPTimeout)
	if err != nil {
		return SMTPConfig{}, err
	}
	if timeout <= 0 {
		return SMTPConfig{}, fmt.Errorf("parse SAFE_SMTP_TIMEOUT: duration must be positive")
	}

	cfg := SMTPConfig{
		Host:     strings.TrimSpace(os.Getenv("SAFE_SMTP_HOST")),
		Port:     port,
		Username: strings.TrimSpace(os.Getenv("SAFE_SMTP_USERNAME")),
		Password: secretFromEnv("SAFE_SMTP_PASSWORD"),
		From:     strings.TrimSpace(os.Getenv("SAFE_SMTP_FROM")),
		StartTLS: startTLS,
		Timeout:  timeout,
	}
	if cfg.Host == "" {
		return SMTPConfig{}, fmt.Errorf("parse SAFE_SMTP_HOST: required when SAFE_EMAIL_BACKEND=smtp")
	}
	if strings.Contains(cfg.Host, "://") || net.ParseIP(cfg.Host) == nil && strings.ContainsAny(cfg.Host, "/?#") {
		return SMTPConfig{}, fmt.Errorf("parse SAFE_SMTP_HOST: host must not be a URL")
	}
	if cfg.From == "" {
		return SMTPConfig{}, fmt.Errorf("parse SAFE_SMTP_FROM: required when SAFE_EMAIL_BACKEND=smtp")
	}
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return SMTPConfig{}, fmt.Errorf("parse SAFE_SMTP_FROM: invalid email address")
	}
	cfg.From = from.Address
	if cfg.Password != "" && cfg.Username == "" {
		return SMTPConfig{}, fmt.Errorf("parse SAFE_SMTP_USERNAME: required when SAFE_SMTP_PASSWORD is set")
	}
	return cfg, nil
}

func positiveIntFromEnv(name string, fallback int) (int, error) {
	raw, ok := os.LookupEnv(name)
	if !ok {
		return fallback, nil
	}
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("parse %s: empty integer", name)
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: invalid integer", name)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("parse %s: integer must be positive", name)
	}
	return parsed, nil
}
