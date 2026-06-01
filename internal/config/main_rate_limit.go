package config

import "fmt"

func mainAPIRateLimitConfigFromEnv() (MainAPIRateLimitConfig, error) {
	enabled, err := boolFromEnv("SAFE_MAIN_API_RATE_LIMIT_ENABLED", defaultMainAPIRateLimitEnabled)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	window, err := durationFromEnv("SAFE_MAIN_API_RATE_LIMIT_WINDOW", defaultMainAPIRateLimitWindow)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	if enabled && window <= 0 {
		return MainAPIRateLimitConfig{}, fmt.Errorf("parse SAFE_MAIN_API_RATE_LIMIT_WINDOW: duration must be positive when rate limiting is enabled")
	}

	authLimit, err := nonNegativeIntFromEnv("SAFE_MAIN_API_RATE_LIMIT_AUTH", defaultMainAPIRateLimitAuthLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	bootstrapLimit, err := nonNegativeIntFromEnv("SAFE_MAIN_API_RATE_LIMIT_BOOTSTRAP", defaultMainAPIRateLimitBootstrapLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	accountLimit, err := nonNegativeIntFromEnv("SAFE_MAIN_API_RATE_LIMIT_ACCOUNT", defaultMainAPIRateLimitAccountLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	incidentReadLimit, err := nonNegativeIntFromEnv("SAFE_MAIN_API_RATE_LIMIT_INCIDENT_READ", defaultMainAPIRateLimitIncidentReadLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	incidentWriteLimit, err := nonNegativeIntFromEnv("SAFE_MAIN_API_RATE_LIMIT_INCIDENT_WRITE", defaultMainAPIRateLimitIncidentWriteLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	uploadLimit, err := nonNegativeIntFromEnv("SAFE_MAIN_API_RATE_LIMIT_UPLOAD", defaultMainAPIRateLimitUploadLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	reconcileLimit, err := nonNegativeIntFromEnv("SAFE_MAIN_API_RATE_LIMIT_RECONCILE", defaultMainAPIRateLimitReconcileLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	streamLimit, err := nonNegativeIntFromEnv("SAFE_MAIN_API_RATE_LIMIT_STREAM", defaultMainAPIRateLimitStreamLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	tokenLimit, err := nonNegativeIntFromEnv("SAFE_MAIN_API_RATE_LIMIT_TOKEN", defaultMainAPIRateLimitTokenLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	downloadLimit, err := nonNegativeIntFromEnv("SAFE_MAIN_API_RATE_LIMIT_DOWNLOAD", defaultMainAPIRateLimitDownloadLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	adminLimit, err := nonNegativeIntFromEnv("SAFE_MAIN_API_RATE_LIMIT_ADMIN", defaultMainAPIRateLimitAdminLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}

	return MainAPIRateLimitConfig{
		Enabled:            enabled,
		Window:             window,
		AuthLimit:          authLimit,
		BootstrapLimit:     bootstrapLimit,
		AccountLimit:       accountLimit,
		IncidentReadLimit:  incidentReadLimit,
		IncidentWriteLimit: incidentWriteLimit,
		UploadLimit:        uploadLimit,
		ReconcileLimit:     reconcileLimit,
		StreamLimit:        streamLimit,
		TokenLimit:         tokenLimit,
		DownloadLimit:      downloadLimit,
		AdminLimit:         adminLimit,
	}, nil
}
