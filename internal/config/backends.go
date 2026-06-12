package config

import (
	"fmt"
	"strings"
)

const (
	MetadataBackendSQLite     = "sqlite"
	MetadataBackendPostgres   = "postgresql"
	BlobBackendLocal          = "local"
	BlobBackendS3             = "s3"
	CoordinationBackendNone   = "none"
	CoordinationBackendValkey = "valkey"
	CoordinationBackendRedis  = "redis"
)

func backendSelectionFromSource(source configSource) (BackendSelection, error) {
	metadata, err := backendFromSource(
		source,
		"SAFE_METADATA_BACKEND",
		MetadataBackendSQLite,
		[]string{MetadataBackendSQLite, MetadataBackendPostgres},
	)
	if err != nil {
		return BackendSelection{}, err
	}
	blob, err := backendFromSource(
		source,
		"SAFE_BLOB_BACKEND",
		BlobBackendLocal,
		[]string{BlobBackendLocal, BlobBackendS3},
	)
	if err != nil {
		return BackendSelection{}, err
	}
	coordination, err := backendFromSource(
		source,
		"SAFE_COORDINATION_BACKEND",
		CoordinationBackendNone,
		[]string{CoordinationBackendNone, CoordinationBackendValkey, CoordinationBackendRedis},
	)
	if err != nil {
		return BackendSelection{}, err
	}

	return BackendSelection{
		Metadata:     metadata,
		Blob:         blob,
		Coordination: coordination,
	}, nil
}

func backendFromSource(source configSource, name, fallback string, supported []string) (string, error) {
	raw, ok := source.Lookup(name)
	if !ok {
		return fallback, nil
	}
	value := strings.ToLower(strings.TrimSpace(raw))
	for _, candidate := range supported {
		if value == candidate {
			return value, nil
		}
	}
	return "", UnsupportedBackendError{
		EnvName:   name,
		Supported: supported,
	}
}

// UnsupportedBackendError reports a rejected backend selector without exposing
// the rejected value.
type UnsupportedBackendError struct {
	EnvName   string
	Supported []string
}

func (e UnsupportedBackendError) Error() string {
	return fmt.Sprintf("parse %s: unsupported backend; supported values: %s",
		e.EnvName,
		strings.Join(e.Supported, ", "),
	)
}
