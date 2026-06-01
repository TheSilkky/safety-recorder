package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoadDefaultBindAddrs(t *testing.T) {
	cfg := loadConfigForTest(t, nil)

	assertStringsEqual(t, cfg.MainBindAddrs, []string{"127.0.0.1:8080"})
	assertStringsEqual(t, cfg.AdminBindAddrs, []string{"127.0.0.1:8081"})
}

func TestLoadDefaultHTTPTimeouts(t *testing.T) {
	cfg := loadConfigForTest(t, nil)

	assertTimeoutsEqual(t, cfg.MainTimeouts, HTTPTimeouts{
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       0,
		WriteTimeout:      0,
		IdleTimeout:       120 * time.Second,
	})
	assertTimeoutsEqual(t, cfg.AdminTimeouts, HTTPTimeouts{
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      300 * time.Second,
		IdleTimeout:       120 * time.Second,
	})
}

func TestLoadDefaultIncidentTokenTTL(t *testing.T) {
	cfg := loadConfigForTest(t, nil)

	if cfg.DefaultIncidentTokenTTL != 24*time.Hour {
		t.Fatalf("default incident token ttl = %s, want 24h", cfg.DefaultIncidentTokenTTL)
	}
}

func TestLoadDefaultSessionTTL(t *testing.T) {
	cfg := loadConfigForTest(t, nil)

	if cfg.SessionTTL != 12*time.Hour {
		t.Fatalf("default session ttl = %s, want 12h", cfg.SessionTTL)
	}
}

func TestLoadDefaultDeletionRetentionConfig(t *testing.T) {
	cfg := loadConfigForTest(t, nil)

	if cfg.DeletionWorkerInterval != time.Minute {
		t.Fatalf("deletion worker interval = %s, want 1m", cfg.DeletionWorkerInterval)
	}
	if cfg.ClosedIncidentRetention != 0 {
		t.Fatalf("closed incident retention = %s, want disabled", cfg.ClosedIncidentRetention)
	}
	if cfg.TokenMetadataRetention != 0 {
		t.Fatalf("token metadata retention = %s, want disabled", cfg.TokenMetadataRetention)
	}
	if cfg.TombstoneRetention != 0 {
		t.Fatalf("tombstone retention = %s, want disabled", cfg.TombstoneRetention)
	}
	if cfg.TempUploadCleanupAge != 0 {
		t.Fatalf("temp upload cleanup age = %s, want disabled", cfg.TempUploadCleanupAge)
	}
	if cfg.TempUploadCleanupDryRun {
		t.Fatal("temp upload cleanup dry run should default to false")
	}
}

func TestLoadTempUploadCleanupConfig(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_TEMP_UPLOAD_CLEANUP_AGE":     "24h",
		"SAFE_TEMP_UPLOAD_CLEANUP_DRY_RUN": "true",
	})

	if cfg.TempUploadCleanupAge != 24*time.Hour {
		t.Fatalf("temp upload cleanup age = %s, want 24h", cfg.TempUploadCleanupAge)
	}
	if !cfg.TempUploadCleanupDryRun {
		t.Fatal("temp upload cleanup dry run was not enabled")
	}
}

func TestLoadUploadCoordinationLeaseTTL(t *testing.T) {
	cfg := loadConfigForTest(t, nil)
	if cfg.UploadCoordinationLeaseTTL != 2*time.Minute {
		t.Fatalf("upload coordination lease ttl = %s, want 2m", cfg.UploadCoordinationLeaseTTL)
	}

	cfg = loadConfigForTest(t, map[string]string{
		"SAFE_UPLOAD_COORDINATION_LEASE_TTL": "45s",
	})
	if cfg.UploadCoordinationLeaseTTL != 45*time.Second {
		t.Fatalf("upload coordination lease ttl = %s, want 45s", cfg.UploadCoordinationLeaseTTL)
	}
}

func TestLoadUploadCoordinationLeaseTTLRejectsNonPositive(t *testing.T) {
	for _, value := range []string{"0", "-1s"} {
		t.Run(value, func(t *testing.T) {
			_, err := loadConfigForTestErr(t, map[string]string{
				"SAFE_UPLOAD_COORDINATION_LEASE_TTL": value,
			})
			if err == nil {
				t.Fatal("expected upload coordination ttl error")
			}
			if !strings.Contains(err.Error(), "SAFE_UPLOAD_COORDINATION_LEASE_TTL") {
				t.Fatalf("expected SAFE_UPLOAD_COORDINATION_LEASE_TTL error, got %v", err)
			}
		})
	}
}

func TestLoadDefaultPublicViewerRateLimitConfig(t *testing.T) {
	cfg := loadConfigForTest(t, nil)

	want := PublicViewerRateLimitConfig{
		Enabled:       true,
		Window:        time.Minute,
		PageLimit:     60,
		DataLimit:     300,
		DownloadLimit: 12,
		StaticLimit:   600,
	}
	if cfg.PublicViewerRateLimit != want {
		t.Fatalf("public viewer rate limit = %+v, want %+v", cfg.PublicViewerRateLimit, want)
	}
}

func TestLoadDefaultMainAPIRateLimitConfig(t *testing.T) {
	cfg := loadConfigForTest(t, nil)

	want := MainAPIRateLimitConfig{
		Enabled:            true,
		Window:             time.Minute,
		AuthLimit:          30,
		BootstrapLimit:     5,
		AccountLimit:       120,
		IncidentReadLimit:  300,
		IncidentWriteLimit: 120,
		UploadLimit:        120,
		ReconcileLimit:     120,
		StreamLimit:        120,
		TokenLimit:         60,
		DownloadLimit:      30,
		AdminLimit:         60,
	}
	if cfg.MainAPIRateLimit != want {
		t.Fatalf("main api rate limit = %+v, want %+v", cfg.MainAPIRateLimit, want)
	}
}

func TestLoadAuthBootstrapSecret(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_AUTH_BOOTSTRAP_SECRET": " bootstrap-secret ",
	})

	if cfg.AuthBootstrapSecret != "bootstrap-secret" {
		t.Fatalf("bootstrap secret was not trimmed")
	}
}

func TestLoadDefaultBackends(t *testing.T) {
	cfg := loadConfigForTest(t, nil)

	want := BackendSelection{
		Metadata:     MetadataBackendSQLite,
		Blob:         BlobBackendLocal,
		Coordination: CoordinationBackendNone,
	}
	if cfg.Backends != want {
		t.Fatalf("backends = %+v, want %+v", cfg.Backends, want)
	}
}

func TestLoadExplicitSupportedBackends(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_METADATA_BACKEND":     "SQLite",
		"SAFE_BLOB_BACKEND":         " local ",
		"SAFE_COORDINATION_BACKEND": "NONE",
	})

	want := BackendSelection{
		Metadata:     MetadataBackendSQLite,
		Blob:         BlobBackendLocal,
		Coordination: CoordinationBackendNone,
	}
	if cfg.Backends != want {
		t.Fatalf("backends = %+v, want %+v", cfg.Backends, want)
	}
}

func TestLoadValkeyCoordinationBackendConfig(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_COORDINATION_BACKEND": " Valkey ",
		"SAFE_VALKEY_ADDR":          "127.0.0.1:6379",
		"SAFE_VALKEY_USERNAME":      "proofline",
		"SAFE_VALKEY_PASSWORD":      "secret-password",
		"SAFE_VALKEY_DB":            "2",
		"SAFE_VALKEY_TLS":           "true",
		"SAFE_VALKEY_DIAL_TIMEOUT":  "2s",
		"SAFE_VALKEY_READ_TIMEOUT":  "3s",
		"SAFE_VALKEY_WRITE_TIMEOUT": "4s",
	})

	if cfg.Backends.Coordination != CoordinationBackendValkey {
		t.Fatalf("coordination backend = %q, want valkey", cfg.Backends.Coordination)
	}
	want := ValkeyConfig{
		Addr:         "127.0.0.1:6379",
		Username:     "proofline",
		Password:     "secret-password",
		DB:           2,
		UseTLS:       true,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 4 * time.Second,
	}
	if cfg.Valkey != want {
		t.Fatalf("valkey config = %+v, want %+v", cfg.Valkey, want)
	}
}

func TestLoadRedisCoordinationBackendAlias(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_COORDINATION_BACKEND": "redis",
		"SAFE_VALKEY_ADDR":          "localhost:6379",
	})

	if cfg.Backends.Coordination != CoordinationBackendRedis {
		t.Fatalf("coordination backend = %q, want redis", cfg.Backends.Coordination)
	}
	if cfg.Valkey.Addr != "localhost:6379" {
		t.Fatalf("valkey addr = %q, want localhost:6379", cfg.Valkey.Addr)
	}
}

func TestLoadValkeyCoordinationBackendRequiresExplicitConfig(t *testing.T) {
	tests := map[string]map[string]string{
		"addr": {
			"SAFE_COORDINATION_BACKEND": "valkey",
		},
		"url addr": {
			"SAFE_COORDINATION_BACKEND": "valkey",
			"SAFE_VALKEY_ADDR":          "redis://user:secret@example.invalid:6379/0",
		},
		"host without port": {
			"SAFE_COORDINATION_BACKEND": "valkey",
			"SAFE_VALKEY_ADDR":          "example.invalid",
		},
		"empty db": {
			"SAFE_COORDINATION_BACKEND": "valkey",
			"SAFE_VALKEY_ADDR":          "localhost:6379",
			"SAFE_VALKEY_DB":            "",
		},
		"negative db": {
			"SAFE_COORDINATION_BACKEND": "valkey",
			"SAFE_VALKEY_ADDR":          "localhost:6379",
			"SAFE_VALKEY_DB":            "-1",
		},
		"invalid tls": {
			"SAFE_COORDINATION_BACKEND": "valkey",
			"SAFE_VALKEY_ADDR":          "localhost:6379",
			"SAFE_VALKEY_TLS":           "sometimes",
		},
		"invalid dial timeout": {
			"SAFE_COORDINATION_BACKEND": "valkey",
			"SAFE_VALKEY_ADDR":          "localhost:6379",
			"SAFE_VALKEY_DIAL_TIMEOUT":  "soon",
		},
	}

	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := loadConfigForTestErr(t, env)
			if err == nil {
				t.Fatal("expected valkey config error")
			}
			if !strings.Contains(err.Error(), "SAFE_VALKEY_") {
				t.Fatalf("expected SAFE_VALKEY error, got %v", err)
			}
			if strings.Contains(err.Error(), "example.invalid") || strings.Contains(err.Error(), "secret") {
				t.Fatalf("valkey config error exposed deployment detail: %v", err)
			}
		})
	}
}

func TestLoadNoCoordinationBackendIgnoresValkeyConfig(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_COORDINATION_BACKEND": "none",
		"SAFE_VALKEY_ADDR":          "not-a-host-port",
		"SAFE_VALKEY_DB":            "not-an-int",
		"SAFE_VALKEY_DIAL_TIMEOUT":  "soon",
		"SAFE_VALKEY_READ_TIMEOUT":  "",
		"SAFE_VALKEY_WRITE_TIMEOUT": "-1s",
	})

	if cfg.Backends.Coordination != CoordinationBackendNone {
		t.Fatalf("coordination backend = %q, want none", cfg.Backends.Coordination)
	}
}

func TestLoadPostgresMetadataBackendConfig(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_METADATA_BACKEND":              " PostgreSQL ",
		"SAFE_POSTGRES_DSN":                  "postgres://proofline:secret@example.invalid/proofline",
		"SAFE_POSTGRES_MAX_OPEN_CONNS":       "20",
		"SAFE_POSTGRES_MAX_IDLE_CONNS":       "8",
		"SAFE_POSTGRES_CONN_MAX_LIFETIME":    "45m",
		"SAFE_POSTGRES_UNUSED_IGNORED_VALUE": "ignored",
	})

	if cfg.Backends.Metadata != MetadataBackendPostgres {
		t.Fatalf("metadata backend = %q, want postgresql", cfg.Backends.Metadata)
	}
	want := PostgresConfig{
		DSN:             "postgres://proofline:secret@example.invalid/proofline",
		MaxOpenConns:    20,
		MaxIdleConns:    8,
		ConnMaxLifetime: 45 * time.Minute,
	}
	if cfg.Postgres != want {
		t.Fatalf("postgres config = %+v, want %+v", cfg.Postgres, want)
	}
}

func TestLoadPostgresMetadataBackendRequiresExplicitConfig(t *testing.T) {
	tests := map[string]map[string]string{
		"dsn": {
			"SAFE_METADATA_BACKEND": "postgresql",
		},
		"empty max open conns": {
			"SAFE_METADATA_BACKEND":        "postgresql",
			"SAFE_POSTGRES_DSN":            "postgres://example.invalid/proofline",
			"SAFE_POSTGRES_MAX_OPEN_CONNS": "",
		},
		"negative max idle conns": {
			"SAFE_METADATA_BACKEND":        "postgresql",
			"SAFE_POSTGRES_DSN":            "postgres://example.invalid/proofline",
			"SAFE_POSTGRES_MAX_IDLE_CONNS": "-1",
		},
		"invalid lifetime": {
			"SAFE_METADATA_BACKEND":           "postgresql",
			"SAFE_POSTGRES_DSN":               "postgres://example.invalid/proofline",
			"SAFE_POSTGRES_CONN_MAX_LIFETIME": "soon",
		},
	}

	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := loadConfigForTestErr(t, env)
			if err == nil {
				t.Fatal("expected postgres config error")
			}
			if !strings.Contains(err.Error(), "SAFE_POSTGRES_") {
				t.Fatalf("expected SAFE_POSTGRES error, got %v", err)
			}
			if strings.Contains(err.Error(), "example.invalid") {
				t.Fatalf("postgres config error exposed DSN: %v", err)
			}
		})
	}
}

func TestLoadSQLiteMetadataBackendIgnoresPostgresConfig(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_METADATA_BACKEND":           "sqlite",
		"SAFE_POSTGRES_MAX_OPEN_CONNS":    "",
		"SAFE_POSTGRES_CONN_MAX_LIFETIME": "not-a-duration",
	})

	if cfg.Backends.Metadata != MetadataBackendSQLite {
		t.Fatalf("metadata backend = %q, want sqlite", cfg.Backends.Metadata)
	}
}

func TestLoadS3BlobBackendConfig(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_BLOB_BACKEND":         "S3",
		"SAFE_S3_ENDPOINT":          "https://s3.example.test",
		"SAFE_S3_REGION":            "us-test-1",
		"SAFE_S3_BUCKET":            "proofline-evidence",
		"SAFE_S3_PREFIX":            "prod/server",
		"SAFE_S3_ACCESS_KEY_ID":     "test-access-key",
		"SAFE_S3_SECRET_ACCESS_KEY": "test-secret-key",
		"SAFE_S3_SESSION_TOKEN":     "test-session-token",
		"SAFE_S3_FORCE_PATH_STYLE":  "false",
	})

	if cfg.Backends.Blob != BlobBackendS3 {
		t.Fatalf("blob backend = %q, want s3", cfg.Backends.Blob)
	}
	want := S3BlobConfig{
		Endpoint:        "https://s3.example.test",
		Region:          "us-test-1",
		Bucket:          "proofline-evidence",
		Prefix:          "prod/server",
		AccessKeyID:     "test-access-key",
		SecretAccessKey: "test-secret-key",
		SessionToken:    "test-session-token",
		ForcePathStyle:  false,
	}
	if cfg.S3Blob != want {
		t.Fatalf("s3 config = %+v, want %+v", cfg.S3Blob, want)
	}
}

func TestLoadS3BlobBackendRequiresExplicitConfig(t *testing.T) {
	tests := map[string]map[string]string{
		"endpoint": {
			"SAFE_BLOB_BACKEND":         "s3",
			"SAFE_S3_BUCKET":            "proofline-evidence",
			"SAFE_S3_ACCESS_KEY_ID":     "test-access-key",
			"SAFE_S3_SECRET_ACCESS_KEY": "test-secret-key",
		},
		"bucket": {
			"SAFE_BLOB_BACKEND":         "s3",
			"SAFE_S3_ENDPOINT":          "https://s3.example.test",
			"SAFE_S3_ACCESS_KEY_ID":     "test-access-key",
			"SAFE_S3_SECRET_ACCESS_KEY": "test-secret-key",
		},
		"missing access key": {
			"SAFE_BLOB_BACKEND":         "s3",
			"SAFE_S3_ENDPOINT":          "https://s3.example.test",
			"SAFE_S3_BUCKET":            "proofline-evidence",
			"SAFE_S3_SECRET_ACCESS_KEY": "test-secret-key",
		},
		"missing secret key": {
			"SAFE_BLOB_BACKEND":     "s3",
			"SAFE_S3_ENDPOINT":      "https://s3.example.test",
			"SAFE_S3_BUCKET":        "proofline-evidence",
			"SAFE_S3_ACCESS_KEY_ID": "test-access-key",
		},
		"invalid path style flag": {
			"SAFE_BLOB_BACKEND":         "s3",
			"SAFE_S3_ENDPOINT":          "https://s3.example.test",
			"SAFE_S3_BUCKET":            "proofline-evidence",
			"SAFE_S3_ACCESS_KEY_ID":     "test-access-key",
			"SAFE_S3_SECRET_ACCESS_KEY": "test-secret-key",
			"SAFE_S3_FORCE_PATH_STYLE":  "sometimes",
		},
	}

	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := loadConfigForTestErr(t, env)
			if err == nil {
				t.Fatal("expected s3 config error")
			}
			if !strings.Contains(err.Error(), "SAFE_S3_") {
				t.Fatalf("expected SAFE_S3 error, got %v", err)
			}
		})
	}
}

func TestLoadLocalBlobBackendIgnoresS3Config(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_BLOB_BACKEND":        "local",
		"SAFE_S3_FORCE_PATH_STYLE": "not-a-bool",
	})

	if cfg.Backends.Blob != BlobBackendLocal {
		t.Fatalf("blob backend = %q, want local", cfg.Backends.Blob)
	}
}

func TestLoadBackendsPreservesLocalPaths(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_METADATA_BACKEND":     "sqlite",
		"SAFE_BLOB_BACKEND":         "local",
		"SAFE_COORDINATION_BACKEND": "none",
		"SAFE_DB_PATH":              "/tmp/proofline-test.db",
		"SAFE_DATA_DIR":             "/tmp/proofline-test-data",
	})

	if cfg.DBPath != "/tmp/proofline-test.db" {
		t.Fatalf("db path = %q, want configured path", cfg.DBPath)
	}
	if cfg.DataDir != "/tmp/proofline-test-data" {
		t.Fatalf("data dir = %q, want configured path", cfg.DataDir)
	}
}

func TestLoadRejectsUnsupportedBackends(t *testing.T) {
	tests := map[string]map[string]string{
		"metadata": {
			"SAFE_METADATA_BACKEND": "postgres",
		},
		"blob": {
			"SAFE_BLOB_BACKEND": "filesystem",
		},
		"coordination": {
			"SAFE_COORDINATION_BACKEND": "memcached",
		},
		"empty": {
			"SAFE_METADATA_BACKEND": "",
		},
	}

	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := loadConfigForTestErr(t, env)
			if err == nil {
				t.Fatal("expected backend config error")
			}
			if !strings.Contains(err.Error(), "unsupported backend") {
				t.Fatalf("expected unsupported backend error, got %v", err)
			}
			if !strings.Contains(err.Error(), "supported values") {
				t.Fatalf("expected supported values in error, got %v", err)
			}
		})
	}
}

func TestLoadIncidentTokenTTLFromEnv(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_DEFAULT_INCIDENT_TOKEN_TTL": "12h",
	})

	if cfg.DefaultIncidentTokenTTL != 12*time.Hour {
		t.Fatalf("default incident token ttl = %s, want 12h", cfg.DefaultIncidentTokenTTL)
	}
}

func TestLoadCanDisableDefaultIncidentTokenTTL(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_DEFAULT_INCIDENT_TOKEN_TTL": "0",
	})

	if cfg.DefaultIncidentTokenTTL != 0 {
		t.Fatalf("default incident token ttl = %s, want disabled", cfg.DefaultIncidentTokenTTL)
	}
}

func TestLoadSessionTTLFromEnv(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_SESSION_TTL": "6h",
	})

	if cfg.SessionTTL != 6*time.Hour {
		t.Fatalf("session ttl = %s, want 6h", cfg.SessionTTL)
	}
}

func TestLoadDeletionRetentionConfigFromEnv(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_DELETION_WORKER_INTERVAL":     "30s",
		"SAFE_CLOSED_INCIDENT_RETENTION":    "720h",
		"SAFE_TOKEN_METADATA_RETENTION":     "168h",
		"SAFE_DELETION_TOMBSTONE_RETENTION": "2160h",
	})

	if cfg.DeletionWorkerInterval != 30*time.Second {
		t.Fatalf("deletion worker interval = %s, want 30s", cfg.DeletionWorkerInterval)
	}
	if cfg.ClosedIncidentRetention != 720*time.Hour {
		t.Fatalf("closed incident retention = %s, want 720h", cfg.ClosedIncidentRetention)
	}
	if cfg.TokenMetadataRetention != 168*time.Hour {
		t.Fatalf("token metadata retention = %s, want 168h", cfg.TokenMetadataRetention)
	}
	if cfg.TombstoneRetention != 2160*time.Hour {
		t.Fatalf("tombstone retention = %s, want 2160h", cfg.TombstoneRetention)
	}
}

func TestLoadCanDisableDeletionWorkerAndRetention(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_DELETION_WORKER_INTERVAL":     "0",
		"SAFE_CLOSED_INCIDENT_RETENTION":    "0",
		"SAFE_TOKEN_METADATA_RETENTION":     "0",
		"SAFE_DELETION_TOMBSTONE_RETENTION": "0",
	})

	if cfg.DeletionWorkerInterval != 0 {
		t.Fatalf("deletion worker interval = %s, want disabled", cfg.DeletionWorkerInterval)
	}
	if cfg.ClosedIncidentRetention != 0 {
		t.Fatalf("closed incident retention = %s, want disabled", cfg.ClosedIncidentRetention)
	}
	if cfg.TokenMetadataRetention != 0 {
		t.Fatalf("token metadata retention = %s, want disabled", cfg.TokenMetadataRetention)
	}
	if cfg.TombstoneRetention != 0 {
		t.Fatalf("tombstone retention = %s, want disabled", cfg.TombstoneRetention)
	}
}

func TestLoadPublicViewerRateLimitConfigFromEnv(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_PUBLIC_VIEWER_RATE_LIMIT_ENABLED":  "false",
		"SAFE_PUBLIC_VIEWER_RATE_LIMIT_WINDOW":   "30s",
		"SAFE_PUBLIC_VIEWER_RATE_LIMIT_PAGE":     "10",
		"SAFE_PUBLIC_VIEWER_RATE_LIMIT_DATA":     "20",
		"SAFE_PUBLIC_VIEWER_RATE_LIMIT_DOWNLOAD": "3",
		"SAFE_PUBLIC_VIEWER_RATE_LIMIT_STATIC":   "100",
	})

	want := PublicViewerRateLimitConfig{
		Enabled:       false,
		Window:        30 * time.Second,
		PageLimit:     10,
		DataLimit:     20,
		DownloadLimit: 3,
		StaticLimit:   100,
	}
	if cfg.PublicViewerRateLimit != want {
		t.Fatalf("public viewer rate limit = %+v, want %+v", cfg.PublicViewerRateLimit, want)
	}
}

func TestLoadMainAPIRateLimitConfigFromEnv(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_MAIN_API_RATE_LIMIT_ENABLED":        "false",
		"SAFE_MAIN_API_RATE_LIMIT_WINDOW":         "30s",
		"SAFE_MAIN_API_RATE_LIMIT_AUTH":           "11",
		"SAFE_MAIN_API_RATE_LIMIT_BOOTSTRAP":      "12",
		"SAFE_MAIN_API_RATE_LIMIT_ACCOUNT":        "13",
		"SAFE_MAIN_API_RATE_LIMIT_INCIDENT_READ":  "14",
		"SAFE_MAIN_API_RATE_LIMIT_INCIDENT_WRITE": "15",
		"SAFE_MAIN_API_RATE_LIMIT_UPLOAD":         "16",
		"SAFE_MAIN_API_RATE_LIMIT_RECONCILE":      "17",
		"SAFE_MAIN_API_RATE_LIMIT_STREAM":         "18",
		"SAFE_MAIN_API_RATE_LIMIT_TOKEN":          "19",
		"SAFE_MAIN_API_RATE_LIMIT_DOWNLOAD":       "20",
		"SAFE_MAIN_API_RATE_LIMIT_ADMIN":          "21",
	})

	want := MainAPIRateLimitConfig{
		Enabled:            false,
		Window:             30 * time.Second,
		AuthLimit:          11,
		BootstrapLimit:     12,
		AccountLimit:       13,
		IncidentReadLimit:  14,
		IncidentWriteLimit: 15,
		UploadLimit:        16,
		ReconcileLimit:     17,
		StreamLimit:        18,
		TokenLimit:         19,
		DownloadLimit:      20,
		AdminLimit:         21,
	}
	if cfg.MainAPIRateLimit != want {
		t.Fatalf("main api rate limit = %+v, want %+v", cfg.MainAPIRateLimit, want)
	}
}

func TestLoadRejectsInvalidPublicViewerRateLimitConfig(t *testing.T) {
	tests := map[string]map[string]string{
		"invalid enabled": {
			"SAFE_PUBLIC_VIEWER_RATE_LIMIT_ENABLED": "sometimes",
		},
		"empty window": {
			"SAFE_PUBLIC_VIEWER_RATE_LIMIT_WINDOW": "",
		},
		"zero enabled window": {
			"SAFE_PUBLIC_VIEWER_RATE_LIMIT_WINDOW": "0",
		},
		"invalid page": {
			"SAFE_PUBLIC_VIEWER_RATE_LIMIT_PAGE": "many",
		},
		"negative data": {
			"SAFE_PUBLIC_VIEWER_RATE_LIMIT_DATA": "-1",
		},
		"empty download": {
			"SAFE_PUBLIC_VIEWER_RATE_LIMIT_DOWNLOAD": "",
		},
		"invalid static": {
			"SAFE_PUBLIC_VIEWER_RATE_LIMIT_STATIC": "lots",
		},
	}

	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := loadConfigForTestErr(t, env)
			if err == nil {
				t.Fatal("expected public viewer rate limit config error")
			}
			if !strings.Contains(err.Error(), "SAFE_PUBLIC_VIEWER_RATE_LIMIT_") {
				t.Fatalf("expected SAFE_PUBLIC_VIEWER_RATE_LIMIT error, got %v", err)
			}
		})
	}
}

func TestLoadRejectsInvalidMainAPIRateLimitConfig(t *testing.T) {
	tests := map[string]map[string]string{
		"invalid enabled": {
			"SAFE_MAIN_API_RATE_LIMIT_ENABLED": "sometimes",
		},
		"empty window": {
			"SAFE_MAIN_API_RATE_LIMIT_WINDOW": "",
		},
		"zero enabled window": {
			"SAFE_MAIN_API_RATE_LIMIT_WINDOW": "0",
		},
		"invalid auth": {
			"SAFE_MAIN_API_RATE_LIMIT_AUTH": "many",
		},
		"negative bootstrap": {
			"SAFE_MAIN_API_RATE_LIMIT_BOOTSTRAP": "-1",
		},
		"empty account": {
			"SAFE_MAIN_API_RATE_LIMIT_ACCOUNT": "",
		},
		"invalid incident read": {
			"SAFE_MAIN_API_RATE_LIMIT_INCIDENT_READ": "lots",
		},
		"invalid incident write": {
			"SAFE_MAIN_API_RATE_LIMIT_INCIDENT_WRITE": "lots",
		},
		"invalid upload": {
			"SAFE_MAIN_API_RATE_LIMIT_UPLOAD": "lots",
		},
		"invalid reconcile": {
			"SAFE_MAIN_API_RATE_LIMIT_RECONCILE": "lots",
		},
		"invalid stream": {
			"SAFE_MAIN_API_RATE_LIMIT_STREAM": "lots",
		},
		"invalid token": {
			"SAFE_MAIN_API_RATE_LIMIT_TOKEN": "lots",
		},
		"invalid download": {
			"SAFE_MAIN_API_RATE_LIMIT_DOWNLOAD": "lots",
		},
		"invalid admin": {
			"SAFE_MAIN_API_RATE_LIMIT_ADMIN": "lots",
		},
	}

	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := loadConfigForTestErr(t, env)
			if err == nil {
				t.Fatal("expected main api rate limit config error")
			}
			if !strings.Contains(err.Error(), "SAFE_MAIN_API_RATE_LIMIT_") {
				t.Fatalf("expected SAFE_MAIN_API_RATE_LIMIT error, got %v", err)
			}
		})
	}
}

func TestLoadRejectsInvalidIncidentTokenTTL(t *testing.T) {
	tests := map[string]string{
		"negative": "-1s",
		"invalid":  "forever",
		"empty":    "",
	}

	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := loadConfigForTestErr(t, map[string]string{
				"SAFE_DEFAULT_INCIDENT_TOKEN_TTL": value,
			})
			if err == nil {
				t.Fatal("expected incident token ttl config error")
			}
			if !strings.Contains(err.Error(), "parse SAFE_DEFAULT_INCIDENT_TOKEN_TTL") {
				t.Fatalf("expected SAFE_DEFAULT_INCIDENT_TOKEN_TTL parse context, got %v", err)
			}
		})
	}
}

func TestLoadRejectsInvalidSessionTTL(t *testing.T) {
	tests := map[string]string{
		"negative": "-1s",
		"invalid":  "forever",
		"empty":    "",
	}

	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := loadConfigForTestErr(t, map[string]string{
				"SAFE_SESSION_TTL": value,
			})
			if err == nil {
				t.Fatal("expected session ttl config error")
			}
			if !strings.Contains(err.Error(), "parse SAFE_SESSION_TTL") {
				t.Fatalf("expected SAFE_SESSION_TTL parse context, got %v", err)
			}
		})
	}
}

func TestLoadRejectsInvalidDeletionRetentionConfig(t *testing.T) {
	tests := map[string]map[string]string{
		"negative worker": {
			"SAFE_DELETION_WORKER_INTERVAL": "-1s",
		},
		"invalid worker": {
			"SAFE_DELETION_WORKER_INTERVAL": "soon",
		},
		"empty retention": {
			"SAFE_CLOSED_INCIDENT_RETENTION": "",
		},
		"invalid token metadata retention": {
			"SAFE_TOKEN_METADATA_RETENTION": "later",
		},
		"negative tombstone retention": {
			"SAFE_DELETION_TOMBSTONE_RETENTION": "-1h",
		},
	}

	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := loadConfigForTestErr(t, env)
			if err == nil {
				t.Fatal("expected deletion retention config error")
			}
			if !strings.Contains(err.Error(), "SAFE_DELETION_WORKER_INTERVAL") &&
				!strings.Contains(err.Error(), "SAFE_CLOSED_INCIDENT_RETENTION") &&
				!strings.Contains(err.Error(), "SAFE_TOKEN_METADATA_RETENTION") &&
				!strings.Contains(err.Error(), "SAFE_DELETION_TOMBSTONE_RETENTION") {
				t.Fatalf("expected deletion env var parse context, got %v", err)
			}
		})
	}
}

func TestLoadHTTPTimeoutsFromEnv(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_MAIN_READ_HEADER_TIMEOUT":  "11s",
		"SAFE_MAIN_READ_TIMEOUT":         "0",
		"SAFE_MAIN_WRITE_TIMEOUT":        "0s",
		"SAFE_MAIN_IDLE_TIMEOUT":         "2m",
		"SAFE_ADMIN_READ_HEADER_TIMEOUT": "12s",
		"SAFE_ADMIN_READ_TIMEOUT":        "31s",
		"SAFE_ADMIN_WRITE_TIMEOUT":       "5m",
		"SAFE_ADMIN_IDLE_TIMEOUT":        "3m",
	})

	assertTimeoutsEqual(t, cfg.MainTimeouts, HTTPTimeouts{
		ReadHeaderTimeout: 11 * time.Second,
		ReadTimeout:       0,
		WriteTimeout:      0,
		IdleTimeout:       2 * time.Minute,
	})
	assertTimeoutsEqual(t, cfg.AdminTimeouts, HTTPTimeouts{
		ReadHeaderTimeout: 12 * time.Second,
		ReadTimeout:       31 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       3 * time.Minute,
	})
}

func TestLoadRejectsInvalidHTTPTimeouts(t *testing.T) {
	tests := map[string]map[string]string{
		"negative": {
			"SAFE_MAIN_READ_TIMEOUT": "-1s",
		},
		"invalid": {
			"SAFE_ADMIN_WRITE_TIMEOUT": "soon",
		},
		"empty": {
			"SAFE_ADMIN_IDLE_TIMEOUT": "",
		},
	}

	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := loadConfigForTestErr(t, env)
			if err == nil {
				t.Fatal("expected timeout config error")
			}
			if !strings.Contains(err.Error(), "parse SAFE_") {
				t.Fatalf("expected env var parse context, got %v", err)
			}
		})
	}
}

func TestLoadSingularBindAddrs(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_MAIN_BIND_ADDR":  "10.66.0.1:8080",
		"SAFE_ADMIN_BIND_ADDR": "192.168.1.20:8081",
	})

	assertStringsEqual(t, cfg.MainBindAddrs, []string{"10.66.0.1:8080"})
	assertStringsEqual(t, cfg.AdminBindAddrs, []string{"192.168.1.20:8081"})
}

func TestLoadPluralBindAddrs(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_MAIN_BIND_ADDRS":  "127.0.0.1:8080,10.66.0.1:8080",
		"SAFE_ADMIN_BIND_ADDRS": "127.0.0.1:8081,192.168.1.20:8081",
	})

	assertStringsEqual(t, cfg.MainBindAddrs, []string{"127.0.0.1:8080", "10.66.0.1:8080"})
	assertStringsEqual(t, cfg.AdminBindAddrs, []string{"127.0.0.1:8081", "192.168.1.20:8081"})
}

func TestLoadPluralBindAddrsTakePrecedenceOverSingular(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_MAIN_BIND_ADDR":   "10.0.0.1:8080",
		"SAFE_MAIN_BIND_ADDRS":  "127.0.0.1:8080,10.66.0.1:8080",
		"SAFE_ADMIN_BIND_ADDR":  "10.0.0.2:8081",
		"SAFE_ADMIN_BIND_ADDRS": "127.0.0.1:8081",
	})

	assertStringsEqual(t, cfg.MainBindAddrs, []string{"127.0.0.1:8080", "10.66.0.1:8080"})
	assertStringsEqual(t, cfg.AdminBindAddrs, []string{"127.0.0.1:8081"})
}

func TestLoadBindAddrsTrimWhitespace(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_MAIN_BIND_ADDRS":  " 127.0.0.1:8080 , 10.66.0.1:8080 ",
		"SAFE_ADMIN_BIND_ADDRS": " 127.0.0.1:8081 ",
	})

	assertStringsEqual(t, cfg.MainBindAddrs, []string{"127.0.0.1:8080", "10.66.0.1:8080"})
	assertStringsEqual(t, cfg.AdminBindAddrs, []string{"127.0.0.1:8081"})
}

func TestLoadLegacyPrivateBindAddrsConfigureMainListener(t *testing.T) {
	cfg := loadConfigForTest(t, map[string]string{
		"SAFE_PRIVATE_BIND_ADDRS": "127.0.0.1:8080,10.66.0.1:8080",
	})

	assertStringsEqual(t, cfg.MainBindAddrs, []string{"127.0.0.1:8080", "10.66.0.1:8080"})
	assertStringsEqual(t, cfg.AdminBindAddrs, []string{"127.0.0.1:8081"})
}

func TestLoadRejectsLegacyPublicBindAddrs(t *testing.T) {
	tests := map[string]map[string]string{
		"plural":   {"SAFE_PUBLIC_BIND_ADDRS": "0.0.0.0:8081"},
		"singular": {"SAFE_PUBLIC_BIND_ADDR": "0.0.0.0:8081"},
	}

	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := loadConfigForTestErr(t, env)
			if err == nil {
				t.Fatal("expected legacy public bind config error")
			}
			if !strings.Contains(err.Error(), "SAFE_PUBLIC_BIND_ADDR") ||
				!strings.Contains(err.Error(), "SAFE_MAIN_BIND_ADDRS") ||
				!strings.Contains(err.Error(), "SAFE_ADMIN_BIND_ADDRS") {
				t.Fatalf("expected migration guidance, got %v", err)
			}
		})
	}
}

func TestLoadBindAddrsRejectEmptyEntries(t *testing.T) {
	tests := map[string]map[string]string{
		"fully empty private list": {
			"SAFE_MAIN_BIND_ADDRS": "",
		},
		"comma-only admin list": {
			"SAFE_ADMIN_BIND_ADDRS": ",",
		},
		"middle empty entry": {
			"SAFE_MAIN_BIND_ADDRS": "127.0.0.1:8080,,10.66.0.1:8080",
		},
		"singular empty entry": {
			"SAFE_MAIN_BIND_ADDR": "",
		},
	}

	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := loadConfigForTestErr(t, env)
			if err == nil {
				t.Fatal("expected config error")
			}
			if !strings.Contains(err.Error(), "empty entry") {
				t.Fatalf("expected empty-entry error, got %v", err)
			}
		})
	}
}

func TestParseBindAddrsKeepsAddressStringsForHTTPValidation(t *testing.T) {
	addrs, err := parseBindAddrs("not-a-net-addr")
	if err != nil {
		t.Fatalf("parseBindAddrs returned error: %v", err)
	}
	assertStringsEqual(t, addrs, []string{"not-a-net-addr"})
}

func TestParseBytesAcceptsUnitValues(t *testing.T) {
	tests := map[string]int64{
		"42":    42,
		"1B":    1,
		"0.5KB": 512,
		"1.5MB": 1572864,
		"2 G":   2 * 1024 * 1024 * 1024,
	}

	for raw, want := range tests {
		t.Run(raw, func(t *testing.T) {
			got, err := parseBytes(raw)
			if err != nil {
				t.Fatalf("parseBytes returned error: %v", err)
			}
			if got != want {
				t.Fatalf("got %d, want %d", got, want)
			}
		})
	}
}

func TestParseBytesRejectsUnsafeUnitValues(t *testing.T) {
	tests := map[string]string{
		"0B":                    "positive",
		"0.0001B":               "at least one byte",
		"9223372036853727232":   "too large",
		"9223372036853727232B":  "too large",
		"9999999999999999999GB": "too large",
		"NaNMB":                 "invalid byte value",
		"InfMB":                 "invalid byte value",
	}

	for raw, wantError := range tests {
		t.Run(raw, func(t *testing.T) {
			_, err := parseBytes(raw)
			if err == nil {
				t.Fatal("expected parseBytes error")
			}
			if !strings.Contains(err.Error(), wantError) {
				t.Fatalf("expected error containing %q, got %v", wantError, err)
			}
		})
	}
}

func TestLoadRejectsUnsafeMaxUploadBytes(t *testing.T) {
	_, err := loadConfigForTestErr(t, map[string]string{
		"SAFE_MAX_UPLOAD_BYTES": "0.0001B",
	})
	if err == nil {
		t.Fatal("expected Load error")
	}
	if !strings.Contains(err.Error(), "parse SAFE_MAX_UPLOAD_BYTES") {
		t.Fatalf("expected SAFE_MAX_UPLOAD_BYTES context, got %v", err)
	}
}

func loadConfigForTest(t *testing.T, env map[string]string) Config {
	t.Helper()
	cfg, err := loadConfigForTestErr(t, env)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func loadConfigForTestErr(t *testing.T, env map[string]string) (Config, error) {
	t.Helper()
	names := []string{
		"SAFE_MAIN_BIND_ADDRS",
		"SAFE_ADMIN_BIND_ADDRS",
		"SAFE_MAIN_BIND_ADDR",
		"SAFE_ADMIN_BIND_ADDR",
		"SAFE_PRIVATE_BIND_ADDRS",
		"SAFE_PUBLIC_BIND_ADDRS",
		"SAFE_PRIVATE_BIND_ADDR",
		"SAFE_PUBLIC_BIND_ADDR",
		"SAFE_DATA_DIR",
		"SAFE_DB_PATH",
		"SAFE_METADATA_BACKEND",
		"SAFE_BLOB_BACKEND",
		"SAFE_COORDINATION_BACKEND",
		"SAFE_MAX_UPLOAD_BYTES",
		"SAFE_DEFAULT_INCIDENT_TOKEN_TTL",
		"SAFE_SESSION_TTL",
		"SAFE_AUTH_BOOTSTRAP_SECRET",
		"SAFE_DELETION_WORKER_INTERVAL",
		"SAFE_CLOSED_INCIDENT_RETENTION",
		"SAFE_TOKEN_METADATA_RETENTION",
		"SAFE_DELETION_TOMBSTONE_RETENTION",
		"SAFE_TEMP_UPLOAD_CLEANUP_AGE",
		"SAFE_TEMP_UPLOAD_CLEANUP_DRY_RUN",
		"SAFE_UPLOAD_COORDINATION_LEASE_TTL",
		"SAFE_MAIN_API_RATE_LIMIT_ENABLED",
		"SAFE_MAIN_API_RATE_LIMIT_WINDOW",
		"SAFE_MAIN_API_RATE_LIMIT_AUTH",
		"SAFE_MAIN_API_RATE_LIMIT_BOOTSTRAP",
		"SAFE_MAIN_API_RATE_LIMIT_ACCOUNT",
		"SAFE_MAIN_API_RATE_LIMIT_INCIDENT_READ",
		"SAFE_MAIN_API_RATE_LIMIT_INCIDENT_WRITE",
		"SAFE_MAIN_API_RATE_LIMIT_UPLOAD",
		"SAFE_MAIN_API_RATE_LIMIT_RECONCILE",
		"SAFE_MAIN_API_RATE_LIMIT_STREAM",
		"SAFE_MAIN_API_RATE_LIMIT_TOKEN",
		"SAFE_MAIN_API_RATE_LIMIT_DOWNLOAD",
		"SAFE_MAIN_API_RATE_LIMIT_ADMIN",
		"SAFE_PUBLIC_VIEWER_RATE_LIMIT_ENABLED",
		"SAFE_PUBLIC_VIEWER_RATE_LIMIT_WINDOW",
		"SAFE_PUBLIC_VIEWER_RATE_LIMIT_PAGE",
		"SAFE_PUBLIC_VIEWER_RATE_LIMIT_DATA",
		"SAFE_PUBLIC_VIEWER_RATE_LIMIT_DOWNLOAD",
		"SAFE_PUBLIC_VIEWER_RATE_LIMIT_STATIC",
		"SAFE_MAIN_READ_HEADER_TIMEOUT",
		"SAFE_MAIN_READ_TIMEOUT",
		"SAFE_MAIN_WRITE_TIMEOUT",
		"SAFE_MAIN_IDLE_TIMEOUT",
		"SAFE_ADMIN_READ_HEADER_TIMEOUT",
		"SAFE_ADMIN_READ_TIMEOUT",
		"SAFE_ADMIN_WRITE_TIMEOUT",
		"SAFE_ADMIN_IDLE_TIMEOUT",
		"SAFE_PRIVATE_READ_HEADER_TIMEOUT",
		"SAFE_PRIVATE_READ_TIMEOUT",
		"SAFE_PRIVATE_WRITE_TIMEOUT",
		"SAFE_PRIVATE_IDLE_TIMEOUT",
		"SAFE_PUBLIC_READ_HEADER_TIMEOUT",
		"SAFE_PUBLIC_READ_TIMEOUT",
		"SAFE_PUBLIC_WRITE_TIMEOUT",
		"SAFE_PUBLIC_IDLE_TIMEOUT",
		"SAFE_POSTGRES_DSN",
		"SAFE_POSTGRES_MAX_OPEN_CONNS",
		"SAFE_POSTGRES_MAX_IDLE_CONNS",
		"SAFE_POSTGRES_CONN_MAX_LIFETIME",
		"SAFE_POSTGRES_UNUSED_IGNORED_VALUE",
		"SAFE_S3_ENDPOINT",
		"SAFE_S3_REGION",
		"SAFE_S3_BUCKET",
		"SAFE_S3_PREFIX",
		"SAFE_S3_ACCESS_KEY_ID",
		"SAFE_S3_SECRET_ACCESS_KEY",
		"SAFE_S3_SESSION_TOKEN",
		"SAFE_S3_FORCE_PATH_STYLE",
		"SAFE_VALKEY_ADDR",
		"SAFE_VALKEY_USERNAME",
		"SAFE_VALKEY_PASSWORD",
		"SAFE_VALKEY_DB",
		"SAFE_VALKEY_TLS",
		"SAFE_VALKEY_DIAL_TIMEOUT",
		"SAFE_VALKEY_READ_TIMEOUT",
		"SAFE_VALKEY_WRITE_TIMEOUT",
	}
	restoreEnv(t, names)
	for name, value := range env {
		if err := os.Setenv(name, value); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}
	return Load()
}

func restoreEnv(t *testing.T, names []string) {
	t.Helper()
	originals := make(map[string]string, len(names))
	present := make(map[string]bool, len(names))
	for _, name := range names {
		value, ok := os.LookupEnv(name)
		originals[name] = value
		present[name] = ok
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("unset %s: %v", name, err)
		}
	}
	t.Cleanup(func() {
		for _, name := range names {
			if present[name] {
				_ = os.Setenv(name, originals[name])
				continue
			}
			_ = os.Unsetenv(name)
		}
	})
}

func assertStringsEqual(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func assertTimeoutsEqual(t *testing.T, got, want HTTPTimeouts) {
	t.Helper()
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
