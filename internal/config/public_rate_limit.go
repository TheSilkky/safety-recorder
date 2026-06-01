package config

import "fmt"

func publicViewerRateLimitConfigFromSource(source configSource) (PublicViewerRateLimitConfig, error) {
	enabled, err := boolFromSource(source, "SAFE_PUBLIC_VIEWER_RATE_LIMIT_ENABLED", defaultPublicViewerRateLimitEnabled)
	if err != nil {
		return PublicViewerRateLimitConfig{}, err
	}
	window, err := durationFromSource(source, "SAFE_PUBLIC_VIEWER_RATE_LIMIT_WINDOW", defaultPublicViewerRateLimitWindow)
	if err != nil {
		return PublicViewerRateLimitConfig{}, err
	}
	if enabled && window <= 0 {
		return PublicViewerRateLimitConfig{}, fmt.Errorf("parse SAFE_PUBLIC_VIEWER_RATE_LIMIT_WINDOW: duration must be positive when rate limiting is enabled")
	}

	pageLimit, err := nonNegativeIntFromSource(source, "SAFE_PUBLIC_VIEWER_RATE_LIMIT_PAGE", defaultPublicViewerRateLimitPageLimit)
	if err != nil {
		return PublicViewerRateLimitConfig{}, err
	}
	dataLimit, err := nonNegativeIntFromSource(source, "SAFE_PUBLIC_VIEWER_RATE_LIMIT_DATA", defaultPublicViewerRateLimitDataLimit)
	if err != nil {
		return PublicViewerRateLimitConfig{}, err
	}
	downloadLimit, err := nonNegativeIntFromSource(source, "SAFE_PUBLIC_VIEWER_RATE_LIMIT_DOWNLOAD", defaultPublicViewerRateLimitDownloadLimit)
	if err != nil {
		return PublicViewerRateLimitConfig{}, err
	}
	staticLimit, err := nonNegativeIntFromSource(source, "SAFE_PUBLIC_VIEWER_RATE_LIMIT_STATIC", defaultPublicViewerRateLimitStaticLimit)
	if err != nil {
		return PublicViewerRateLimitConfig{}, err
	}

	return PublicViewerRateLimitConfig{
		Enabled:       enabled,
		Window:        window,
		PageLimit:     pageLimit,
		DataLimit:     dataLimit,
		DownloadLimit: downloadLimit,
		StaticLimit:   staticLimit,
	}, nil
}
