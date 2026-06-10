package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	defaultLocalConfigFile  = "./proofline.toml"
	defaultSystemConfigFile = "/etc/proofline/proofline.toml"
)

type LoadOptions struct {
	ConfigFilePath string
}

type configFile struct {
	Server              configFileServer              `toml:"server"`
	Paths               configFilePaths               `toml:"paths"`
	Metadata            configFileMetadata            `toml:"metadata"`
	BlobStorage         configFileBlobStorage         `toml:"blob_storage"`
	Coordination        configFileCoordination        `toml:"coordination"`
	Uploads             configFileUploads             `toml:"uploads"`
	Auth                configFileAuth                `toml:"auth"`
	AccountRegistration configFileAccountRegistration `toml:"account_registration"`
	Email               configFileEmail               `toml:"email"`
	WebAuth             configFileWebAuth             `toml:"web_auth"`
	Retention           configFileRetention           `toml:"retention"`
	RateLimits          configFileRateLimits          `toml:"rate_limits"`
	HTTP                configFileHTTP                `toml:"http"`
}

type configFileServer struct {
	MainBindAddrs  *[]string `toml:"main_bind_addrs"`
	AdminBindAddrs *[]string `toml:"admin_bind_addrs"`
}

type configFilePaths struct {
	DataDir      *string `toml:"data_dir"`
	SQLiteDBPath *string `toml:"sqlite_db_path"`
}

type configFileMetadata struct {
	Backend                 *string `toml:"backend"`
	PostgresDSN             *string `toml:"postgres_dsn"`
	PostgresDSNFile         *string `toml:"postgres_dsn_file"`
	PostgresMaxOpenConns    *int    `toml:"postgres_max_open_conns"`
	PostgresMaxIdleConns    *int    `toml:"postgres_max_idle_conns"`
	PostgresConnMaxLifetime *string `toml:"postgres_conn_max_lifetime"`
}

type configFileBlobStorage struct {
	Backend               *string `toml:"backend"`
	S3Endpoint            *string `toml:"s3_endpoint"`
	S3Region              *string `toml:"s3_region"`
	S3Bucket              *string `toml:"s3_bucket"`
	S3Prefix              *string `toml:"s3_prefix"`
	S3AccessKeyID         *string `toml:"s3_access_key_id"`
	S3AccessKeyIDFile     *string `toml:"s3_access_key_id_file"`
	S3SecretAccessKey     *string `toml:"s3_secret_access_key"`
	S3SecretAccessKeyFile *string `toml:"s3_secret_access_key_file"`
	S3SessionToken        *string `toml:"s3_session_token"`
	S3SessionTokenFile    *string `toml:"s3_session_token_file"`
	S3ForcePathStyle      *bool   `toml:"s3_force_path_style"`
}

type configFileCoordination struct {
	Backend            *string `toml:"backend"`
	ValkeyAddr         *string `toml:"valkey_addr"`
	ValkeyUsername     *string `toml:"valkey_username"`
	ValkeyPassword     *string `toml:"valkey_password"`
	ValkeyPasswordFile *string `toml:"valkey_password_file"`
	ValkeyDB           *int    `toml:"valkey_db"`
	ValkeyTLS          *bool   `toml:"valkey_tls"`
	ValkeyDialTimeout  *string `toml:"valkey_dial_timeout"`
	ValkeyReadTimeout  *string `toml:"valkey_read_timeout"`
	ValkeyWriteTimeout *string `toml:"valkey_write_timeout"`
}

type configFileUploads struct {
	MaxUploadBytes               *string `toml:"max_upload_bytes"`
	AccountDefaultBlobQuotaBytes *string `toml:"account_default_blob_quota_bytes"`
	TempUploadStagingQuotaBytes  *string `toml:"temp_upload_staging_quota_bytes"`
	UploadCoordinationLeaseTTL   *string `toml:"upload_coordination_lease_ttl"`
	TempUploadCleanupAge         *string `toml:"temp_upload_cleanup_age"`
	TempUploadCleanupDryRun      *bool   `toml:"temp_upload_cleanup_dry_run"`
}

type configFileAuth struct {
	SessionTTL          *string `toml:"session_ttl"`
	BootstrapSecret     *string `toml:"bootstrap_secret"`
	BootstrapSecretFile *string `toml:"bootstrap_secret_file"`
}

type configFileAccountRegistration struct {
	Mode                 *string `toml:"mode"`
	EmailVerificationTTL *string `toml:"email_verification_ttl"`
	PublicWebOrigin      *string `toml:"public_web_origin"`
}

type configFileEmail struct {
	Backend          *string `toml:"backend"`
	SMTPHost         *string `toml:"smtp_host"`
	SMTPPort         *int    `toml:"smtp_port"`
	SMTPUsername     *string `toml:"smtp_username"`
	SMTPPassword     *string `toml:"smtp_password"`
	SMTPPasswordFile *string `toml:"smtp_password_file"`
	SMTPFrom         *string `toml:"smtp_from"`
	SMTPStartTLS     *string `toml:"smtp_starttls"`
	SMTPTimeout      *string `toml:"smtp_timeout"`
}

type configFileWebAuth struct {
	Enabled               *bool     `toml:"enabled"`
	AllowedOrigins        *[]string `toml:"allowed_origins"`
	SessionCookieName     *string   `toml:"session_cookie_name"`
	SessionCookieSecure   *bool     `toml:"session_cookie_secure"`
	SessionCookieSameSite *string   `toml:"session_cookie_samesite"`
	CSRFHeaderName        *string   `toml:"csrf_header_name"`
}

type configFileRetention struct {
	DefaultIncidentTokenTTL    *string `toml:"default_incident_token_ttl"`
	ClosedIncidentRetention    *string `toml:"closed_incident_retention"`
	TokenMetadataRetention     *string `toml:"token_metadata_retention"`
	DeletionTombstoneRetention *string `toml:"deletion_tombstone_retention"`
	DeletionWorkerInterval     *string `toml:"deletion_worker_interval"`
}

type configFileRateLimits struct {
	MainAPI      configFileMainAPIRateLimit      `toml:"main_api"`
	PublicViewer configFilePublicViewerRateLimit `toml:"public_viewer"`
}

type configFileMainAPIRateLimit struct {
	Enabled         *bool   `toml:"enabled"`
	Window          *string `toml:"window"`
	Auth            *int    `toml:"auth"`
	AuthRegister    *int    `toml:"auth_register"`
	AuthEmailVerify *int    `toml:"auth_email_verify"`
	Bootstrap       *int    `toml:"bootstrap"`
	Account         *int    `toml:"account"`
	IncidentRead    *int    `toml:"incident_read"`
	IncidentWrite   *int    `toml:"incident_write"`
	Upload          *int    `toml:"upload"`
	Reconcile       *int    `toml:"reconcile"`
	Stream          *int    `toml:"stream"`
	Token           *int    `toml:"token"`
	Download        *int    `toml:"download"`
	Admin           *int    `toml:"admin"`
}

type configFilePublicViewerRateLimit struct {
	Enabled  *bool   `toml:"enabled"`
	Window   *string `toml:"window"`
	Page     *int    `toml:"page"`
	Data     *int    `toml:"data"`
	Download *int    `toml:"download"`
	Static   *int    `toml:"static"`
}

type configFileHTTP struct {
	Main  configFileHTTPTimeouts `toml:"main"`
	Admin configFileHTTPTimeouts `toml:"admin"`
}

type configFileHTTPTimeouts struct {
	ReadHeaderTimeout *string `toml:"read_header_timeout"`
	ReadTimeout       *string `toml:"read_timeout"`
	WriteTimeout      *string `toml:"write_timeout"`
	IdleTimeout       *string `toml:"idle_timeout"`
}

func resolveConfigFilePath(explicitPath string) (string, bool, error) {
	if path := strings.TrimSpace(explicitPath); path != "" {
		return checkedConfigFilePath("--config", path)
	}
	if path := strings.TrimSpace(os.Getenv("SAFE_CONFIG_FILE")); path != "" {
		return checkedConfigFilePath("SAFE_CONFIG_FILE", path)
	}
	for _, candidate := range []string{defaultLocalConfigFile, defaultSystemConfigFile} {
		ok, err := discoveredConfigFileExists(candidate)
		if err != nil {
			return "", false, err
		}
		if ok {
			return candidate, true, nil
		}
	}
	return "", false, nil
}

func checkedConfigFilePath(name, path string) (string, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", false, newConfigParseError(name, "config file cannot be read")
	}
	if !info.Mode().IsRegular() {
		return "", false, newConfigParseError(name, "config file must be a regular file")
	}
	return path, true, nil
}

func discoveredConfigFileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, newConfigParseError("config file", "discovered config file cannot be inspected")
	}
	if !info.Mode().IsRegular() {
		return false, newConfigParseError("config file", "discovered config file must be a regular file")
	}
	return true, nil
}

func configValuesFromFile(path string) (map[string]string, error) {
	var file configFile
	meta, err := toml.DecodeFile(path, &file)
	if err != nil {
		return nil, newConfigParseError("config file", "invalid TOML")
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		return nil, newConfigParseError("config file", fmt.Sprintf("unknown TOML key %q", formatTOMLKey(undecoded[0])))
	}
	return file.toValues()
}

func formatTOMLKey(key toml.Key) string {
	return strings.Join(key, ".")
}

func (file configFile) toValues() (map[string]string, error) {
	values := make(map[string]string)

	setStringList(values, "SAFE_MAIN_BIND_ADDRS", file.Server.MainBindAddrs)
	setStringList(values, "SAFE_ADMIN_BIND_ADDRS", file.Server.AdminBindAddrs)

	setString(values, "SAFE_DATA_DIR", file.Paths.DataDir)
	setString(values, "SAFE_DB_PATH", file.Paths.SQLiteDBPath)

	setString(values, "SAFE_METADATA_BACKEND", file.Metadata.Backend)
	if err := setSecret(values, "SAFE_POSTGRES_DSN", "SAFE_POSTGRES_DSN_FILE", file.Metadata.PostgresDSN, file.Metadata.PostgresDSNFile); err != nil {
		return nil, err
	}
	setInt(values, "SAFE_POSTGRES_MAX_OPEN_CONNS", file.Metadata.PostgresMaxOpenConns)
	setInt(values, "SAFE_POSTGRES_MAX_IDLE_CONNS", file.Metadata.PostgresMaxIdleConns)
	setString(values, "SAFE_POSTGRES_CONN_MAX_LIFETIME", file.Metadata.PostgresConnMaxLifetime)

	setString(values, "SAFE_BLOB_BACKEND", file.BlobStorage.Backend)
	setString(values, "SAFE_S3_ENDPOINT", file.BlobStorage.S3Endpoint)
	setString(values, "SAFE_S3_REGION", file.BlobStorage.S3Region)
	setString(values, "SAFE_S3_BUCKET", file.BlobStorage.S3Bucket)
	setString(values, "SAFE_S3_PREFIX", file.BlobStorage.S3Prefix)
	if err := setSecret(values, "SAFE_S3_ACCESS_KEY_ID", "SAFE_S3_ACCESS_KEY_ID_FILE", file.BlobStorage.S3AccessKeyID, file.BlobStorage.S3AccessKeyIDFile); err != nil {
		return nil, err
	}
	if err := setSecret(values, "SAFE_S3_SECRET_ACCESS_KEY", "SAFE_S3_SECRET_ACCESS_KEY_FILE", file.BlobStorage.S3SecretAccessKey, file.BlobStorage.S3SecretAccessKeyFile); err != nil {
		return nil, err
	}
	if err := setSecret(values, "SAFE_S3_SESSION_TOKEN", "SAFE_S3_SESSION_TOKEN_FILE", file.BlobStorage.S3SessionToken, file.BlobStorage.S3SessionTokenFile); err != nil {
		return nil, err
	}
	setBool(values, "SAFE_S3_FORCE_PATH_STYLE", file.BlobStorage.S3ForcePathStyle)

	setString(values, "SAFE_COORDINATION_BACKEND", file.Coordination.Backend)
	setString(values, "SAFE_VALKEY_ADDR", file.Coordination.ValkeyAddr)
	setString(values, "SAFE_VALKEY_USERNAME", file.Coordination.ValkeyUsername)
	if err := setSecret(values, "SAFE_VALKEY_PASSWORD", "SAFE_VALKEY_PASSWORD_FILE", file.Coordination.ValkeyPassword, file.Coordination.ValkeyPasswordFile); err != nil {
		return nil, err
	}
	setInt(values, "SAFE_VALKEY_DB", file.Coordination.ValkeyDB)
	setBool(values, "SAFE_VALKEY_TLS", file.Coordination.ValkeyTLS)
	setString(values, "SAFE_VALKEY_DIAL_TIMEOUT", file.Coordination.ValkeyDialTimeout)
	setString(values, "SAFE_VALKEY_READ_TIMEOUT", file.Coordination.ValkeyReadTimeout)
	setString(values, "SAFE_VALKEY_WRITE_TIMEOUT", file.Coordination.ValkeyWriteTimeout)

	setString(values, "SAFE_MAX_UPLOAD_BYTES", file.Uploads.MaxUploadBytes)
	setString(values, "SAFE_ACCOUNT_DEFAULT_BLOB_QUOTA_BYTES", file.Uploads.AccountDefaultBlobQuotaBytes)
	setString(values, "SAFE_TEMP_UPLOAD_STAGING_QUOTA_BYTES", file.Uploads.TempUploadStagingQuotaBytes)
	setString(values, "SAFE_UPLOAD_COORDINATION_LEASE_TTL", file.Uploads.UploadCoordinationLeaseTTL)
	setString(values, "SAFE_TEMP_UPLOAD_CLEANUP_AGE", file.Uploads.TempUploadCleanupAge)
	setBool(values, "SAFE_TEMP_UPLOAD_CLEANUP_DRY_RUN", file.Uploads.TempUploadCleanupDryRun)

	setString(values, "SAFE_SESSION_TTL", file.Auth.SessionTTL)
	if err := setSecret(values, "SAFE_AUTH_BOOTSTRAP_SECRET", "SAFE_AUTH_BOOTSTRAP_SECRET_FILE", file.Auth.BootstrapSecret, file.Auth.BootstrapSecretFile); err != nil {
		return nil, err
	}

	setString(values, "SAFE_ACCOUNT_REGISTRATION_MODE", file.AccountRegistration.Mode)
	setString(values, "SAFE_EMAIL_VERIFICATION_TTL", file.AccountRegistration.EmailVerificationTTL)
	setString(values, "SAFE_PUBLIC_WEB_ORIGIN", file.AccountRegistration.PublicWebOrigin)

	setString(values, "SAFE_EMAIL_BACKEND", file.Email.Backend)
	setString(values, "SAFE_SMTP_HOST", file.Email.SMTPHost)
	setInt(values, "SAFE_SMTP_PORT", file.Email.SMTPPort)
	setString(values, "SAFE_SMTP_USERNAME", file.Email.SMTPUsername)
	if err := setSecret(values, "SAFE_SMTP_PASSWORD", "SAFE_SMTP_PASSWORD_FILE", file.Email.SMTPPassword, file.Email.SMTPPasswordFile); err != nil {
		return nil, err
	}
	setString(values, "SAFE_SMTP_FROM", file.Email.SMTPFrom)
	setString(values, "SAFE_SMTP_STARTTLS", file.Email.SMTPStartTLS)
	setString(values, "SAFE_SMTP_TIMEOUT", file.Email.SMTPTimeout)

	setBool(values, "SAFE_WEB_AUTH_ENABLED", file.WebAuth.Enabled)
	setStringList(values, "SAFE_WEB_ALLOWED_ORIGINS", file.WebAuth.AllowedOrigins)
	setString(values, "SAFE_WEB_SESSION_COOKIE_NAME", file.WebAuth.SessionCookieName)
	setBool(values, "SAFE_WEB_SESSION_COOKIE_SECURE", file.WebAuth.SessionCookieSecure)
	setString(values, "SAFE_WEB_SESSION_COOKIE_SAMESITE", file.WebAuth.SessionCookieSameSite)
	setString(values, "SAFE_WEB_CSRF_HEADER_NAME", file.WebAuth.CSRFHeaderName)

	setString(values, "SAFE_DEFAULT_INCIDENT_TOKEN_TTL", file.Retention.DefaultIncidentTokenTTL)
	setString(values, "SAFE_CLOSED_INCIDENT_RETENTION", file.Retention.ClosedIncidentRetention)
	setString(values, "SAFE_TOKEN_METADATA_RETENTION", file.Retention.TokenMetadataRetention)
	setString(values, "SAFE_DELETION_TOMBSTONE_RETENTION", file.Retention.DeletionTombstoneRetention)
	setString(values, "SAFE_DELETION_WORKER_INTERVAL", file.Retention.DeletionWorkerInterval)

	setBool(values, "SAFE_MAIN_API_RATE_LIMIT_ENABLED", file.RateLimits.MainAPI.Enabled)
	setString(values, "SAFE_MAIN_API_RATE_LIMIT_WINDOW", file.RateLimits.MainAPI.Window)
	setInt(values, "SAFE_MAIN_API_RATE_LIMIT_AUTH", file.RateLimits.MainAPI.Auth)
	setInt(values, "SAFE_MAIN_API_RATE_LIMIT_AUTH_REGISTER", file.RateLimits.MainAPI.AuthRegister)
	setInt(values, "SAFE_MAIN_API_RATE_LIMIT_AUTH_EMAIL_VERIFY", file.RateLimits.MainAPI.AuthEmailVerify)
	setInt(values, "SAFE_MAIN_API_RATE_LIMIT_BOOTSTRAP", file.RateLimits.MainAPI.Bootstrap)
	setInt(values, "SAFE_MAIN_API_RATE_LIMIT_ACCOUNT", file.RateLimits.MainAPI.Account)
	setInt(values, "SAFE_MAIN_API_RATE_LIMIT_INCIDENT_READ", file.RateLimits.MainAPI.IncidentRead)
	setInt(values, "SAFE_MAIN_API_RATE_LIMIT_INCIDENT_WRITE", file.RateLimits.MainAPI.IncidentWrite)
	setInt(values, "SAFE_MAIN_API_RATE_LIMIT_UPLOAD", file.RateLimits.MainAPI.Upload)
	setInt(values, "SAFE_MAIN_API_RATE_LIMIT_RECONCILE", file.RateLimits.MainAPI.Reconcile)
	setInt(values, "SAFE_MAIN_API_RATE_LIMIT_STREAM", file.RateLimits.MainAPI.Stream)
	setInt(values, "SAFE_MAIN_API_RATE_LIMIT_TOKEN", file.RateLimits.MainAPI.Token)
	setInt(values, "SAFE_MAIN_API_RATE_LIMIT_DOWNLOAD", file.RateLimits.MainAPI.Download)
	setInt(values, "SAFE_MAIN_API_RATE_LIMIT_ADMIN", file.RateLimits.MainAPI.Admin)

	setBool(values, "SAFE_PUBLIC_VIEWER_RATE_LIMIT_ENABLED", file.RateLimits.PublicViewer.Enabled)
	setString(values, "SAFE_PUBLIC_VIEWER_RATE_LIMIT_WINDOW", file.RateLimits.PublicViewer.Window)
	setInt(values, "SAFE_PUBLIC_VIEWER_RATE_LIMIT_PAGE", file.RateLimits.PublicViewer.Page)
	setInt(values, "SAFE_PUBLIC_VIEWER_RATE_LIMIT_DATA", file.RateLimits.PublicViewer.Data)
	setInt(values, "SAFE_PUBLIC_VIEWER_RATE_LIMIT_DOWNLOAD", file.RateLimits.PublicViewer.Download)
	setInt(values, "SAFE_PUBLIC_VIEWER_RATE_LIMIT_STATIC", file.RateLimits.PublicViewer.Static)

	setString(values, "SAFE_MAIN_READ_HEADER_TIMEOUT", file.HTTP.Main.ReadHeaderTimeout)
	setString(values, "SAFE_MAIN_READ_TIMEOUT", file.HTTP.Main.ReadTimeout)
	setString(values, "SAFE_MAIN_WRITE_TIMEOUT", file.HTTP.Main.WriteTimeout)
	setString(values, "SAFE_MAIN_IDLE_TIMEOUT", file.HTTP.Main.IdleTimeout)
	setString(values, "SAFE_ADMIN_READ_HEADER_TIMEOUT", file.HTTP.Admin.ReadHeaderTimeout)
	setString(values, "SAFE_ADMIN_READ_TIMEOUT", file.HTTP.Admin.ReadTimeout)
	setString(values, "SAFE_ADMIN_WRITE_TIMEOUT", file.HTTP.Admin.WriteTimeout)
	setString(values, "SAFE_ADMIN_IDLE_TIMEOUT", file.HTTP.Admin.IdleTimeout)

	return values, nil
}

func setString(values map[string]string, name string, value *string) {
	if value == nil {
		return
	}
	values[name] = strings.TrimSpace(*value)
}

func setStringList(values map[string]string, name string, value *[]string) {
	if value == nil {
		return
	}
	values[name] = strings.Join(*value, ",")
}

func setInt(values map[string]string, name string, value *int) {
	if value == nil {
		return
	}
	values[name] = strconv.Itoa(*value)
}

func setBool(values map[string]string, name string, value *bool) {
	if value == nil {
		return
	}
	values[name] = strconv.FormatBool(*value)
}

func setSecret(values map[string]string, directName, fileName string, directValue, fileValue *string) error {
	direct := ""
	if directValue != nil {
		direct = strings.TrimSpace(*directValue)
	}
	file := ""
	if fileValue != nil {
		file = strings.TrimSpace(*fileValue)
	}
	if direct != "" && file != "" {
		return newConfigParseError(directName, "direct secret and secret file are both configured")
	}
	if direct != "" {
		values[directName] = direct
	}
	if file != "" {
		values[fileName] = file
	}
	return nil
}
