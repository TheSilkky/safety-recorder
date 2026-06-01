package config

import (
	"fmt"
	"time"
)

const (
	defaultMainBindAddr                       = "127.0.0.1:8080"
	defaultAdminBindAddr                      = "127.0.0.1:8081"
	defaultDataDir                            = "./data"
	defaultDBPath                             = "./data/safety.db"
	defaultMaxUploadBytes                     = int64(250 * 1024 * 1024)
	defaultIncidentTokenTTL                   = 24 * time.Hour
	defaultSessionTTL                         = 12 * time.Hour
	defaultDeletionInterval                   = time.Minute
	defaultTempUploadCleanupAge               = 0
	defaultTempUploadCleanupDryRun            = false
	defaultUploadCoordinationLeaseTTL         = 2 * time.Minute
	defaultMainAPIRateLimitEnabled            = true
	defaultMainAPIRateLimitWindow             = time.Minute
	defaultMainAPIRateLimitAuthLimit          = 30
	defaultMainAPIRateLimitBootstrapLimit     = 5
	defaultMainAPIRateLimitAccountLimit       = 120
	defaultMainAPIRateLimitIncidentReadLimit  = 300
	defaultMainAPIRateLimitIncidentWriteLimit = 120
	defaultMainAPIRateLimitUploadLimit        = 120
	defaultMainAPIRateLimitReconcileLimit     = 120
	defaultMainAPIRateLimitStreamLimit        = 120
	defaultMainAPIRateLimitTokenLimit         = 60
	defaultMainAPIRateLimitDownloadLimit      = 30
	defaultMainAPIRateLimitAdminLimit         = 60
	defaultPublicViewerRateLimitEnabled       = true
	defaultPublicViewerRateLimitWindow        = time.Minute
	defaultPublicViewerRateLimitPageLimit     = 60
	defaultPublicViewerRateLimitDataLimit     = 300
	defaultPublicViewerRateLimitDownloadLimit = 12
	defaultPublicViewerRateLimitStaticLimit   = 600
	// Leave room for the multipart envelope added by the HTTP upload handler
	// so configured upload limits cannot overflow request-size arithmetic.
	maxConfiguredUploadBytes = int64(1<<63 - 1 - 1024*1024)

	defaultMainReadHeaderTimeout = 10 * time.Second
	defaultMainReadTimeout       = 0
	defaultMainWriteTimeout      = 0
	defaultMainIdleTimeout       = 120 * time.Second

	defaultAdminReadHeaderTimeout = 10 * time.Second
	defaultAdminReadTimeout       = 30 * time.Second
	defaultAdminWriteTimeout      = 300 * time.Second
	defaultAdminIdleTimeout       = 120 * time.Second
)

// Config contains the runtime settings needed by the API server.
type Config struct {
	MainBindAddrs              []string
	AdminBindAddrs             []string
	Backends                   BackendSelection
	Postgres                   PostgresConfig
	S3Blob                     S3BlobConfig
	Valkey                     ValkeyConfig
	DataDir                    string
	DBPath                     string
	MaxUploadBytes             int64
	DefaultIncidentTokenTTL    time.Duration
	SessionTTL                 time.Duration
	AuthBootstrapSecret        string
	DeletionWorkerInterval     time.Duration
	ClosedIncidentRetention    time.Duration
	TokenMetadataRetention     time.Duration
	TombstoneRetention         time.Duration
	TempUploadCleanupAge       time.Duration
	TempUploadCleanupDryRun    bool
	UploadCoordinationLeaseTTL time.Duration
	MainAPIRateLimit           MainAPIRateLimitConfig
	PublicViewerRateLimit      PublicViewerRateLimitConfig
	MainTimeouts               HTTPTimeouts
	AdminTimeouts              HTTPTimeouts
}

// BackendSelection records the configured storage and coordination backends.
type BackendSelection struct {
	Metadata     string
	Blob         string
	Coordination string
}

// S3BlobConfig contains the optional S3-compatible blob backend settings.
type S3BlobConfig struct {
	Endpoint        string
	Region          string
	Bucket          string
	Prefix          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	ForcePathStyle  bool
}

// ValkeyConfig contains optional Valkey/Redis-compatible coordination settings.
type ValkeyConfig struct {
	Addr         string
	Username     string
	Password     string
	DB           int
	UseTLS       bool
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// PostgresConfig contains optional PostgreSQL metadata backend settings.
type PostgresConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// MainAPIRateLimitConfig contains app-level rate limits for main API route
// classes that must be controlled before public exposure.
type MainAPIRateLimitConfig struct {
	Enabled            bool
	Window             time.Duration
	AuthLimit          int
	BootstrapLimit     int
	AccountLimit       int
	IncidentReadLimit  int
	IncidentWriteLimit int
	UploadLimit        int
	ReconcileLimit     int
	StreamLimit        int
	TokenLimit         int
	DownloadLimit      int
	AdminLimit         int
}

// PublicViewerRateLimitConfig contains app-level rate limits for public viewer
// route classes.
type PublicViewerRateLimitConfig struct {
	Enabled       bool
	Window        time.Duration
	PageLimit     int
	DataLimit     int
	DownloadLimit int
	StaticLimit   int
}

// HTTPTimeouts groups net/http server timeout settings.
type HTTPTimeouts struct {
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

// Load reads configuration from environment variables and applies defaults for
// unset values.
func Load() (Config, error) {
	mainBindAddrs, err := mainBindAddrsFromEnv()
	if err != nil {
		return Config{}, err
	}
	adminBindAddrs, err := adminBindAddrsFromEnv()
	if err != nil {
		return Config{}, err
	}

	backends, err := backendSelectionFromEnv()
	if err != nil {
		return Config{}, err
	}
	postgres, err := postgresConfigFromEnv(backends.Metadata)
	if err != nil {
		return Config{}, err
	}
	s3Blob, err := s3BlobConfigFromEnv(backends.Blob)
	if err != nil {
		return Config{}, err
	}
	valkey, err := valkeyConfigFromEnv(backends.Coordination)
	if err != nil {
		return Config{}, err
	}

	maxUploadBytes, err := maxUploadBytesFromEnv()
	if err != nil {
		return Config{}, err
	}
	incidentTokenTTL, err := durationFromEnv("SAFE_DEFAULT_INCIDENT_TOKEN_TTL", defaultIncidentTokenTTL)
	if err != nil {
		return Config{}, err
	}
	sessionTTL, err := durationFromEnv("SAFE_SESSION_TTL", defaultSessionTTL)
	if err != nil {
		return Config{}, err
	}
	deletionWorkerInterval, err := durationFromEnv("SAFE_DELETION_WORKER_INTERVAL", defaultDeletionInterval)
	if err != nil {
		return Config{}, err
	}
	closedIncidentRetention, err := durationFromEnv("SAFE_CLOSED_INCIDENT_RETENTION", 0)
	if err != nil {
		return Config{}, err
	}
	tokenMetadataRetention, err := durationFromEnv("SAFE_TOKEN_METADATA_RETENTION", 0)
	if err != nil {
		return Config{}, err
	}
	tombstoneRetention, err := durationFromEnv("SAFE_DELETION_TOMBSTONE_RETENTION", 0)
	if err != nil {
		return Config{}, err
	}
	tempUploadCleanupAge, err := durationFromEnv("SAFE_TEMP_UPLOAD_CLEANUP_AGE", defaultTempUploadCleanupAge)
	if err != nil {
		return Config{}, err
	}
	tempUploadCleanupDryRun, err := boolFromEnv("SAFE_TEMP_UPLOAD_CLEANUP_DRY_RUN", defaultTempUploadCleanupDryRun)
	if err != nil {
		return Config{}, err
	}
	uploadCoordinationLeaseTTL, err := durationFromEnv("SAFE_UPLOAD_COORDINATION_LEASE_TTL", defaultUploadCoordinationLeaseTTL)
	if err != nil {
		return Config{}, err
	}
	if uploadCoordinationLeaseTTL <= 0 {
		return Config{}, fmt.Errorf("parse SAFE_UPLOAD_COORDINATION_LEASE_TTL: duration must be positive")
	}

	mainAPIRateLimit, err := mainAPIRateLimitConfigFromEnv()
	if err != nil {
		return Config{}, err
	}
	publicViewerRateLimit, err := publicViewerRateLimitConfigFromEnv()
	if err != nil {
		return Config{}, err
	}

	mainTimeouts, err := mainTimeoutsFromEnv()
	if err != nil {
		return Config{}, err
	}
	adminTimeouts, err := adminTimeoutsFromEnv()
	if err != nil {
		return Config{}, err
	}

	return Config{
		MainBindAddrs:              mainBindAddrs,
		AdminBindAddrs:             adminBindAddrs,
		Backends:                   backends,
		Postgres:                   postgres,
		S3Blob:                     s3Blob,
		Valkey:                     valkey,
		DataDir:                    envOrDefault("SAFE_DATA_DIR", defaultDataDir),
		DBPath:                     envOrDefault("SAFE_DB_PATH", defaultDBPath),
		MaxUploadBytes:             maxUploadBytes,
		DefaultIncidentTokenTTL:    incidentTokenTTL,
		SessionTTL:                 sessionTTL,
		AuthBootstrapSecret:        secretFromEnv("SAFE_AUTH_BOOTSTRAP_SECRET"),
		DeletionWorkerInterval:     deletionWorkerInterval,
		ClosedIncidentRetention:    closedIncidentRetention,
		TokenMetadataRetention:     tokenMetadataRetention,
		TombstoneRetention:         tombstoneRetention,
		TempUploadCleanupAge:       tempUploadCleanupAge,
		TempUploadCleanupDryRun:    tempUploadCleanupDryRun,
		UploadCoordinationLeaseTTL: uploadCoordinationLeaseTTL,
		MainAPIRateLimit:           mainAPIRateLimit,
		PublicViewerRateLimit:      publicViewerRateLimit,
		MainTimeouts:               mainTimeouts,
		AdminTimeouts:              adminTimeouts,
	}, nil
}
