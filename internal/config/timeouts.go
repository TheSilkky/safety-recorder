package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

func mainTimeoutsFromEnv() (HTTPTimeouts, error) {
	return timeoutsFromEnvWithLegacy("SAFE_MAIN", "SAFE_PRIVATE", HTTPTimeouts{
		ReadHeaderTimeout: defaultMainReadHeaderTimeout,
		ReadTimeout:       defaultMainReadTimeout,
		WriteTimeout:      defaultMainWriteTimeout,
		IdleTimeout:       defaultMainIdleTimeout,
	})
}

func adminTimeoutsFromEnv() (HTTPTimeouts, error) {
	return timeoutsFromEnv("SAFE_ADMIN", HTTPTimeouts{
		ReadHeaderTimeout: defaultAdminReadHeaderTimeout,
		ReadTimeout:       defaultAdminReadTimeout,
		WriteTimeout:      defaultAdminWriteTimeout,
		IdleTimeout:       defaultAdminIdleTimeout,
	})
}

func timeoutsFromEnv(prefix string, defaults HTTPTimeouts) (HTTPTimeouts, error) {
	readHeaderTimeout, err := durationFromEnv(prefix+"_READ_HEADER_TIMEOUT", defaults.ReadHeaderTimeout)
	if err != nil {
		return HTTPTimeouts{}, err
	}
	readTimeout, err := durationFromEnv(prefix+"_READ_TIMEOUT", defaults.ReadTimeout)
	if err != nil {
		return HTTPTimeouts{}, err
	}
	writeTimeout, err := durationFromEnv(prefix+"_WRITE_TIMEOUT", defaults.WriteTimeout)
	if err != nil {
		return HTTPTimeouts{}, err
	}
	idleTimeout, err := durationFromEnv(prefix+"_IDLE_TIMEOUT", defaults.IdleTimeout)
	if err != nil {
		return HTTPTimeouts{}, err
	}
	return HTTPTimeouts{
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}, nil
}

func timeoutsFromEnvWithLegacy(prefix, legacyPrefix string, defaults HTTPTimeouts) (HTTPTimeouts, error) {
	readHeaderTimeout, err := durationFromEnvWithLegacy(prefix+"_READ_HEADER_TIMEOUT", legacyPrefix+"_READ_HEADER_TIMEOUT", defaults.ReadHeaderTimeout)
	if err != nil {
		return HTTPTimeouts{}, err
	}
	readTimeout, err := durationFromEnvWithLegacy(prefix+"_READ_TIMEOUT", legacyPrefix+"_READ_TIMEOUT", defaults.ReadTimeout)
	if err != nil {
		return HTTPTimeouts{}, err
	}
	writeTimeout, err := durationFromEnvWithLegacy(prefix+"_WRITE_TIMEOUT", legacyPrefix+"_WRITE_TIMEOUT", defaults.WriteTimeout)
	if err != nil {
		return HTTPTimeouts{}, err
	}
	idleTimeout, err := durationFromEnvWithLegacy(prefix+"_IDLE_TIMEOUT", legacyPrefix+"_IDLE_TIMEOUT", defaults.IdleTimeout)
	if err != nil {
		return HTTPTimeouts{}, err
	}
	return HTTPTimeouts{
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}, nil
}

func durationFromEnvWithLegacy(name, legacyName string, fallback time.Duration) (time.Duration, error) {
	if _, ok := os.LookupEnv(name); ok {
		return durationFromEnv(name, fallback)
	}
	return durationFromEnv(legacyName, fallback)
}

func durationFromEnv(name string, fallback time.Duration) (time.Duration, error) {
	raw, ok := os.LookupEnv(name)
	if !ok {
		return fallback, nil
	}
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("parse %s: empty duration", name)
	}
	if value == "0" {
		return 0, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("parse %s: duration must be non-negative", name)
	}
	return parsed, nil
}
