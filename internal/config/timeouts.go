package config

import (
	"fmt"
	"strings"
	"time"
)

func mainTimeoutsFromSource(source configSource) (HTTPTimeouts, error) {
	return timeoutsFromSourceWithLegacy(source, "SAFE_MAIN", "SAFE_PRIVATE", HTTPTimeouts{
		ReadHeaderTimeout: defaultMainReadHeaderTimeout,
		ReadTimeout:       defaultMainReadTimeout,
		WriteTimeout:      defaultMainWriteTimeout,
		IdleTimeout:       defaultMainIdleTimeout,
	})
}

func adminTimeoutsFromSource(source configSource) (HTTPTimeouts, error) {
	return timeoutsFromSource(source, "SAFE_ADMIN", HTTPTimeouts{
		ReadHeaderTimeout: defaultAdminReadHeaderTimeout,
		ReadTimeout:       defaultAdminReadTimeout,
		WriteTimeout:      defaultAdminWriteTimeout,
		IdleTimeout:       defaultAdminIdleTimeout,
	})
}

func timeoutsFromSource(source configSource, prefix string, defaults HTTPTimeouts) (HTTPTimeouts, error) {
	readHeaderTimeout, err := durationFromSource(source, prefix+"_READ_HEADER_TIMEOUT", defaults.ReadHeaderTimeout)
	if err != nil {
		return HTTPTimeouts{}, err
	}
	readTimeout, err := durationFromSource(source, prefix+"_READ_TIMEOUT", defaults.ReadTimeout)
	if err != nil {
		return HTTPTimeouts{}, err
	}
	writeTimeout, err := durationFromSource(source, prefix+"_WRITE_TIMEOUT", defaults.WriteTimeout)
	if err != nil {
		return HTTPTimeouts{}, err
	}
	idleTimeout, err := durationFromSource(source, prefix+"_IDLE_TIMEOUT", defaults.IdleTimeout)
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

func timeoutsFromSourceWithLegacy(source configSource, prefix, legacyPrefix string, defaults HTTPTimeouts) (HTTPTimeouts, error) {
	readHeaderTimeout, err := durationFromSourceWithLegacy(source, prefix+"_READ_HEADER_TIMEOUT", legacyPrefix+"_READ_HEADER_TIMEOUT", defaults.ReadHeaderTimeout)
	if err != nil {
		return HTTPTimeouts{}, err
	}
	readTimeout, err := durationFromSourceWithLegacy(source, prefix+"_READ_TIMEOUT", legacyPrefix+"_READ_TIMEOUT", defaults.ReadTimeout)
	if err != nil {
		return HTTPTimeouts{}, err
	}
	writeTimeout, err := durationFromSourceWithLegacy(source, prefix+"_WRITE_TIMEOUT", legacyPrefix+"_WRITE_TIMEOUT", defaults.WriteTimeout)
	if err != nil {
		return HTTPTimeouts{}, err
	}
	idleTimeout, err := durationFromSourceWithLegacy(source, prefix+"_IDLE_TIMEOUT", legacyPrefix+"_IDLE_TIMEOUT", defaults.IdleTimeout)
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

func durationFromSourceWithLegacy(source configSource, name, legacyName string, fallback time.Duration) (time.Duration, error) {
	if _, ok := source.Lookup(name); ok {
		return durationFromSource(source, name, fallback)
	}
	return durationFromSource(source, legacyName, fallback)
}

func durationFromSource(source configSource, name string, fallback time.Duration) (time.Duration, error) {
	raw, ok := source.Lookup(name)
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
