package config

import "fmt"

func mainAPIRateLimitConfigFromSource(source configSource) (MainAPIRateLimitConfig, error) {
	enabled, err := boolFromSource(source, "SAFE_MAIN_API_RATE_LIMIT_ENABLED", defaultMainAPIRateLimitEnabled)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	window, err := durationFromSource(source, "SAFE_MAIN_API_RATE_LIMIT_WINDOW", defaultMainAPIRateLimitWindow)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	if enabled && window <= 0 {
		return MainAPIRateLimitConfig{}, fmt.Errorf("parse SAFE_MAIN_API_RATE_LIMIT_WINDOW: duration must be positive when rate limiting is enabled")
	}

	authLimit, err := nonNegativeIntFromSource(source, "SAFE_MAIN_API_RATE_LIMIT_AUTH", defaultMainAPIRateLimitAuthLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	authRegisterLimit, err := nonNegativeIntFromSource(source, "SAFE_MAIN_API_RATE_LIMIT_AUTH_REGISTER", defaultMainAPIRateLimitAuthRegisterLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	authEmailVerifyLimit, err := nonNegativeIntFromSource(source, "SAFE_MAIN_API_RATE_LIMIT_AUTH_EMAIL_VERIFY", defaultMainAPIRateLimitAuthEmailVerify)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	bootstrapLimit, err := nonNegativeIntFromSource(source, "SAFE_MAIN_API_RATE_LIMIT_BOOTSTRAP", defaultMainAPIRateLimitBootstrapLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	accountLimit, err := nonNegativeIntFromSource(source, "SAFE_MAIN_API_RATE_LIMIT_ACCOUNT", defaultMainAPIRateLimitAccountLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	incidentReadLimit, err := nonNegativeIntFromSource(source, "SAFE_MAIN_API_RATE_LIMIT_INCIDENT_READ", defaultMainAPIRateLimitIncidentReadLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	incidentWriteLimit, err := nonNegativeIntFromSource(source, "SAFE_MAIN_API_RATE_LIMIT_INCIDENT_WRITE", defaultMainAPIRateLimitIncidentWriteLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	uploadLimit, err := nonNegativeIntFromSource(source, "SAFE_MAIN_API_RATE_LIMIT_UPLOAD", defaultMainAPIRateLimitUploadLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	reconcileLimit, err := nonNegativeIntFromSource(source, "SAFE_MAIN_API_RATE_LIMIT_RECONCILE", defaultMainAPIRateLimitReconcileLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	streamLimit, err := nonNegativeIntFromSource(source, "SAFE_MAIN_API_RATE_LIMIT_STREAM", defaultMainAPIRateLimitStreamLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	tokenLimit, err := nonNegativeIntFromSource(source, "SAFE_MAIN_API_RATE_LIMIT_TOKEN", defaultMainAPIRateLimitTokenLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	downloadLimit, err := nonNegativeIntFromSource(source, "SAFE_MAIN_API_RATE_LIMIT_DOWNLOAD", defaultMainAPIRateLimitDownloadLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}
	adminLimit, err := nonNegativeIntFromSource(source, "SAFE_MAIN_API_RATE_LIMIT_ADMIN", defaultMainAPIRateLimitAdminLimit)
	if err != nil {
		return MainAPIRateLimitConfig{}, err
	}

	return MainAPIRateLimitConfig{
		Enabled:            enabled,
		Window:             window,
		AuthLimit:          authLimit,
		AuthRegisterLimit:  authRegisterLimit,
		AuthEmailVerify:    authEmailVerifyLimit,
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
