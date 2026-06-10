package config

import (
	"fmt"
	"time"
)

const (
	defaultMainBindAddr                       = "127.0.0.1:8080"
	defaultAdminBindAddr                      = "127.0.0.1:8081"
	defaultDataDir                            = "./data"
	defaultDBPath                             = "./data/proofline.db"
	defaultMaxUploadBytes                     = int64(250 * 1024 * 1024)
	defaultAccountDefaultBlobQuotaBytes       = int64(10 * 1024 * 1024 * 1024)
	defaultTempUploadStagingQuotaBytes        = int64(1024 * 1024 * 1024)
	defaultIncidentTokenTTL                   = 24 * time.Hour
	defaultSessionTTL                         = 12 * time.Hour
	defaultEmailVerificationTTL               = 24 * time.Hour
	defaultSecondFactorEmailChallengeTTL      = 10 * time.Minute
	defaultWebAuthnEnabled                    = false
	defaultWebAuthnRPDisplayName              = "Proofline"
	defaultWebAuthnUserVerification           = "required"
	defaultWebAuthnChallengeTTL               = 5 * time.Minute
	defaultDeletionInterval                   = time.Minute
	defaultTempUploadCleanupAge               = 0
	defaultTempUploadCleanupDryRun            = false
	defaultUploadCoordinationLeaseTTL         = 2 * time.Minute
	defaultMainAPIRateLimitEnabled            = true
	defaultMainAPIRateLimitWindow             = time.Minute
	defaultMainAPIRateLimitAuthLimit          = 30
	defaultMainAPIRateLimitAuthRegisterLimit  = 10
	defaultMainAPIRateLimitAuthEmailVerify    = 30
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

const (
	AccountRegistrationModeDisabled  = "disabled"
	AccountRegistrationModeAdminOnly = "admin_only"
	AccountRegistrationModeOpen      = "open"
	AccountRegistrationModePaid      = "paid"

	EmailBackendNone = "none"
	EmailBackendSMTP = "smtp"

	SMTPStartTLSRequired      = "required"
	SMTPStartTLSOpportunistic = "opportunistic"
	SMTPStartTLSDisabled      = "disabled"
)

// Config contains the runtime settings needed by the API server.
type Config struct {
	MainBindAddrs                 []string
	AdminBindAddrs                []string
	Backends                      BackendSelection
	Postgres                      PostgresConfig
	S3Blob                        S3BlobConfig
	Valkey                        ValkeyConfig
	DataDir                       string
	DBPath                        string
	MaxUploadBytes                int64
	AccountDefaultBlobQuotaBytes  int64
	TempUploadStagingQuotaBytes   int64
	DefaultIncidentTokenTTL       time.Duration
	SessionTTL                    time.Duration
	AccountRegistration           AccountRegistrationConfig
	SecondFactorEmailChallengeTTL time.Duration
	Email                         EmailConfig
	AuthBootstrapSecret           string
	DeletionWorkerInterval        time.Duration
	ClosedIncidentRetention       time.Duration
	TokenMetadataRetention        time.Duration
	TombstoneRetention            time.Duration
	TempUploadCleanupAge          time.Duration
	TempUploadCleanupDryRun       bool
	UploadCoordinationLeaseTTL    time.Duration
	MainAPIRateLimit              MainAPIRateLimitConfig
	PublicViewerRateLimit         PublicViewerRateLimitConfig
	WebAuth                       WebAuthConfig
	WebAuthn                      WebAuthnConfig
	MainTimeouts                  HTTPTimeouts
	AdminTimeouts                 HTTPTimeouts
}

// AccountRegistrationConfig controls public self-registration behavior.
type AccountRegistrationConfig struct {
	Mode                 string
	EmailVerificationTTL time.Duration
	PublicWebOrigin      string
}

// EmailConfig contains outbound verification email settings.
type EmailConfig struct {
	Backend string
	SMTP    SMTPConfig
}

// SMTPConfig contains SMTP settings for verification email delivery.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	StartTLS string
	Timeout  time.Duration
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
	AuthRegisterLimit  int
	AuthEmailVerify    int
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

// WebAuthConfig contains optional browser cookie-session settings for the main
// API. It is disabled by default so existing bearer clients keep their current
// behavior unless a deployment explicitly opts in.
type WebAuthConfig struct {
	Enabled               bool
	AllowedOrigins        []string
	SessionCookieName     string
	SessionCookieSecure   bool
	SessionCookieSameSite string
	CSRFHeaderName        string
}

// WebAuthnConfig contains optional WebAuthn passkey/security-key second-factor
// settings. It is disabled by default and fails closed when enabled without an
// explicit relying-party ID and exact origin allow-list.
type WebAuthnConfig struct {
	Enabled          bool
	RPID             string
	RPDisplayName    string
	AllowedOrigins   []string
	UserVerification string
	ChallengeTTL     time.Duration
}

// HTTPTimeouts groups net/http server timeout settings.
type HTTPTimeouts struct {
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

// Load reads configuration from the discovered config file and environment
// variables, then applies defaults for unset values.
func Load() (Config, error) {
	return LoadWithOptions(LoadOptions{})
}

// LoadWithOptions reads configuration from the selected config file and
// environment variables, then applies defaults for unset values.
func LoadWithOptions(opts LoadOptions) (Config, error) {
	configFilePath, ok, err := resolveConfigFilePath(opts.ConfigFilePath)
	if err != nil {
		return Config{}, err
	}
	fileValues := map[string]string{}
	if ok {
		fileValues, err = configValuesFromFile(configFilePath)
		if err != nil {
			return Config{}, err
		}
	}
	return loadFromSource(newConfigSource(fileValues))
}

func loadFromSource(source configSource) (Config, error) {
	mainBindAddrs, err := mainBindAddrsFromSource(source)
	if err != nil {
		return Config{}, err
	}
	adminBindAddrs, err := adminBindAddrsFromSource(source)
	if err != nil {
		return Config{}, err
	}

	backends, err := backendSelectionFromSource(source)
	if err != nil {
		return Config{}, err
	}
	postgres, err := postgresConfigFromSource(source, backends.Metadata)
	if err != nil {
		return Config{}, err
	}
	s3Blob, err := s3BlobConfigFromSource(source, backends.Blob)
	if err != nil {
		return Config{}, err
	}
	valkey, err := valkeyConfigFromSource(source, backends.Coordination)
	if err != nil {
		return Config{}, err
	}

	maxUploadBytes, err := maxUploadBytesFromSource(source)
	if err != nil {
		return Config{}, err
	}
	accountDefaultBlobQuotaBytes, err := accountDefaultBlobQuotaBytesFromSource(source)
	if err != nil {
		return Config{}, err
	}
	tempUploadStagingQuotaBytes, err := tempUploadStagingQuotaBytesFromSource(source)
	if err != nil {
		return Config{}, err
	}
	incidentTokenTTL, err := durationFromSource(source, "SAFE_DEFAULT_INCIDENT_TOKEN_TTL", defaultIncidentTokenTTL)
	if err != nil {
		return Config{}, err
	}
	sessionTTL, err := durationFromSource(source, "SAFE_SESSION_TTL", defaultSessionTTL)
	if err != nil {
		return Config{}, err
	}
	accountRegistration, err := accountRegistrationConfigFromSource(source)
	if err != nil {
		return Config{}, err
	}
	email, err := emailConfigFromSource(source)
	if err != nil {
		return Config{}, err
	}
	accountRegistration, err = validateAccountRegistrationConfig(accountRegistration, email)
	if err != nil {
		return Config{}, err
	}
	secondFactorEmailChallengeTTL, err := durationFromSource(source, "SAFE_SECOND_FACTOR_EMAIL_CHALLENGE_TTL", defaultSecondFactorEmailChallengeTTL)
	if err != nil {
		return Config{}, err
	}
	if secondFactorEmailChallengeTTL <= 0 {
		return Config{}, fmt.Errorf("parse SAFE_SECOND_FACTOR_EMAIL_CHALLENGE_TTL: duration must be positive")
	}
	authBootstrapSecret, err := secretFromSource(source, "SAFE_AUTH_BOOTSTRAP_SECRET", "SAFE_AUTH_BOOTSTRAP_SECRET_FILE")
	if err != nil {
		return Config{}, err
	}
	deletionWorkerInterval, err := durationFromSource(source, "SAFE_DELETION_WORKER_INTERVAL", defaultDeletionInterval)
	if err != nil {
		return Config{}, err
	}
	closedIncidentRetention, err := durationFromSource(source, "SAFE_CLOSED_INCIDENT_RETENTION", 0)
	if err != nil {
		return Config{}, err
	}
	tokenMetadataRetention, err := durationFromSource(source, "SAFE_TOKEN_METADATA_RETENTION", 0)
	if err != nil {
		return Config{}, err
	}
	tombstoneRetention, err := durationFromSource(source, "SAFE_DELETION_TOMBSTONE_RETENTION", 0)
	if err != nil {
		return Config{}, err
	}
	tempUploadCleanupAge, err := durationFromSource(source, "SAFE_TEMP_UPLOAD_CLEANUP_AGE", defaultTempUploadCleanupAge)
	if err != nil {
		return Config{}, err
	}
	tempUploadCleanupDryRun, err := boolFromSource(source, "SAFE_TEMP_UPLOAD_CLEANUP_DRY_RUN", defaultTempUploadCleanupDryRun)
	if err != nil {
		return Config{}, err
	}
	uploadCoordinationLeaseTTL, err := durationFromSource(source, "SAFE_UPLOAD_COORDINATION_LEASE_TTL", defaultUploadCoordinationLeaseTTL)
	if err != nil {
		return Config{}, err
	}
	if uploadCoordinationLeaseTTL <= 0 {
		return Config{}, fmt.Errorf("parse SAFE_UPLOAD_COORDINATION_LEASE_TTL: duration must be positive")
	}

	mainAPIRateLimit, err := mainAPIRateLimitConfigFromSource(source)
	if err != nil {
		return Config{}, err
	}
	publicViewerRateLimit, err := publicViewerRateLimitConfigFromSource(source)
	if err != nil {
		return Config{}, err
	}
	webAuth, err := webAuthConfigFromSource(source)
	if err != nil {
		return Config{}, err
	}
	webAuthn, err := webAuthnConfigFromSource(source)
	if err != nil {
		return Config{}, err
	}

	mainTimeouts, err := mainTimeoutsFromSource(source)
	if err != nil {
		return Config{}, err
	}
	adminTimeouts, err := adminTimeoutsFromSource(source)
	if err != nil {
		return Config{}, err
	}

	return Config{
		MainBindAddrs:                 mainBindAddrs,
		AdminBindAddrs:                adminBindAddrs,
		Backends:                      backends,
		Postgres:                      postgres,
		S3Blob:                        s3Blob,
		Valkey:                        valkey,
		DataDir:                       envOrDefault(source, "SAFE_DATA_DIR", defaultDataDir),
		DBPath:                        envOrDefault(source, "SAFE_DB_PATH", defaultDBPath),
		MaxUploadBytes:                maxUploadBytes,
		AccountDefaultBlobQuotaBytes:  accountDefaultBlobQuotaBytes,
		TempUploadStagingQuotaBytes:   tempUploadStagingQuotaBytes,
		DefaultIncidentTokenTTL:       incidentTokenTTL,
		SessionTTL:                    sessionTTL,
		AccountRegistration:           accountRegistration,
		SecondFactorEmailChallengeTTL: secondFactorEmailChallengeTTL,
		Email:                         email,
		AuthBootstrapSecret:           authBootstrapSecret,
		DeletionWorkerInterval:        deletionWorkerInterval,
		ClosedIncidentRetention:       closedIncidentRetention,
		TokenMetadataRetention:        tokenMetadataRetention,
		TombstoneRetention:            tombstoneRetention,
		TempUploadCleanupAge:          tempUploadCleanupAge,
		TempUploadCleanupDryRun:       tempUploadCleanupDryRun,
		UploadCoordinationLeaseTTL:    uploadCoordinationLeaseTTL,
		MainAPIRateLimit:              mainAPIRateLimit,
		PublicViewerRateLimit:         publicViewerRateLimit,
		WebAuth:                       webAuth,
		WebAuthn:                      webAuthn,
		MainTimeouts:                  mainTimeouts,
		AdminTimeouts:                 adminTimeouts,
	}, nil
}
