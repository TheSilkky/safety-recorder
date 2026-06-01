package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPostgresMaxOpenConns    = 10
	defaultPostgresMaxIdleConns    = 5
	defaultPostgresConnMaxLifetime = 30 * time.Minute
)

func postgresConfigFromSource(source configSource, metadataBackend string) (PostgresConfig, error) {
	cfg := PostgresConfig{
		MaxOpenConns:    defaultPostgresMaxOpenConns,
		MaxIdleConns:    defaultPostgresMaxIdleConns,
		ConnMaxLifetime: defaultPostgresConnMaxLifetime,
	}
	if metadataBackend != MetadataBackendPostgres {
		return cfg, nil
	}

	dsn, err := secretFromSource(source, "SAFE_POSTGRES_DSN", "SAFE_POSTGRES_DSN_FILE")
	if err != nil {
		return PostgresConfig{}, err
	}
	cfg.DSN = dsn
	if cfg.DSN == "" {
		return PostgresConfig{}, fmt.Errorf("parse SAFE_POSTGRES_DSN: required when SAFE_METADATA_BACKEND=postgresql")
	}

	maxOpenConns, err := nonNegativeIntFromSource(source, "SAFE_POSTGRES_MAX_OPEN_CONNS", defaultPostgresMaxOpenConns)
	if err != nil {
		return PostgresConfig{}, err
	}
	maxIdleConns, err := nonNegativeIntFromSource(source, "SAFE_POSTGRES_MAX_IDLE_CONNS", defaultPostgresMaxIdleConns)
	if err != nil {
		return PostgresConfig{}, err
	}
	connMaxLifetime, err := durationFromSource(source, "SAFE_POSTGRES_CONN_MAX_LIFETIME", defaultPostgresConnMaxLifetime)
	if err != nil {
		return PostgresConfig{}, err
	}

	cfg.MaxOpenConns = maxOpenConns
	cfg.MaxIdleConns = maxIdleConns
	cfg.ConnMaxLifetime = connMaxLifetime
	return cfg, nil
}

func nonNegativeIntFromSource(source configSource, name string, fallback int) (int, error) {
	raw, ok := source.Lookup(name)
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
	if parsed < 0 {
		return 0, fmt.Errorf("parse %s: integer must be non-negative", name)
	}
	return parsed, nil
}
