# Configuration

Proofline Server now treats a TOML config file as the recommended default
configuration shape, while preserving the existing `SAFE_*` environment
variables as compatibility inputs and deployment overrides.

Configuration is applied in this order:

```text
built-in defaults
  < TOML config file
  < SAFE_* environment variables
  < SAFE_*_FILE environment variables for secret-capable fields
  < CLI flags
```

The only current config CLI flag is `--config`, which selects the TOML file
path. It does not replace the existing environment override surface.

Config file path resolution:

1. `--config /path/to/proofline.toml`
2. `SAFE_CONFIG_FILE=/path/to/proofline.toml`
3. `./proofline.toml`, when present in the process working directory
4. `/etc/proofline/proofline.toml`, when present
5. built-in defaults plus environment variables only

Explicit config paths must exist and contain valid TOML. Discovered config
files are optional, but if a discovered file exists and is invalid startup
fails. Unknown TOML keys fail startup so misspelled config is not silently
accepted.

Run with a custom config file:

```bash
go run ./cmd/api --config /path/to/proofline.toml
```

or:

```bash
SAFE_CONFIG_FILE=/path/to/proofline.toml go run ./cmd/api
```

The repository root `proofline.toml` is a safe local-first example. It matches
the built-in local defaults and does not contain real secrets. Do not commit or
publish real config files that include private endpoints, secret-file paths, or
deployment credentials.

Examples in this document show TOML first because it is the recommended
configuration shape for repeatable deployments. Environment snippets are still
included for compatibility, CI, tests, or short-lived local overrides. When both
are set, remember that `SAFE_*` environment variables override TOML values.

Secret-bearing values support direct `SAFE_*` environment variables and
matching `SAFE_*_FILE` variables. File-backed secrets are read once at startup.
Missing files, empty files, and direct-secret plus secret-file conflicts within
one TOML config fail startup. A single trailing LF or CRLF is trimmed for
Docker, Kubernetes, Nomad, and systemd secret compatibility; internal
whitespace is preserved. Secret values and secret file contents must not be
logged or copied into public issues, PRs, screenshots, or support tickets.

Within environment configuration, `SAFE_*_FILE` values override direct `SAFE_*`
values for the same field. Within TOML, set either the direct secret key or the
`*_file` key, not both. Prefer `*_file` keys for deployments.

## TOML To Environment Mapping

| TOML key | Environment variable |
|---|---|
| `[server].main_bind_addrs` | `SAFE_MAIN_BIND_ADDRS` |
| `[server].admin_bind_addrs` | `SAFE_ADMIN_BIND_ADDRS` |
| `[paths].data_dir` | `SAFE_DATA_DIR` |
| `[paths].sqlite_db_path` | `SAFE_DB_PATH` |
| `[metadata].backend` | `SAFE_METADATA_BACKEND` |
| `[metadata].postgres_dsn` | `SAFE_POSTGRES_DSN` |
| `[metadata].postgres_dsn_file` | `SAFE_POSTGRES_DSN_FILE` |
| `[metadata].postgres_max_open_conns` | `SAFE_POSTGRES_MAX_OPEN_CONNS` |
| `[metadata].postgres_max_idle_conns` | `SAFE_POSTGRES_MAX_IDLE_CONNS` |
| `[metadata].postgres_conn_max_lifetime` | `SAFE_POSTGRES_CONN_MAX_LIFETIME` |
| `[blob_storage].backend` | `SAFE_BLOB_BACKEND` |
| `[blob_storage].s3_endpoint` | `SAFE_S3_ENDPOINT` |
| `[blob_storage].s3_region` | `SAFE_S3_REGION` |
| `[blob_storage].s3_bucket` | `SAFE_S3_BUCKET` |
| `[blob_storage].s3_prefix` | `SAFE_S3_PREFIX` |
| `[blob_storage].s3_access_key_id` | `SAFE_S3_ACCESS_KEY_ID` |
| `[blob_storage].s3_access_key_id_file` | `SAFE_S3_ACCESS_KEY_ID_FILE` |
| `[blob_storage].s3_secret_access_key` | `SAFE_S3_SECRET_ACCESS_KEY` |
| `[blob_storage].s3_secret_access_key_file` | `SAFE_S3_SECRET_ACCESS_KEY_FILE` |
| `[blob_storage].s3_session_token` | `SAFE_S3_SESSION_TOKEN` |
| `[blob_storage].s3_session_token_file` | `SAFE_S3_SESSION_TOKEN_FILE` |
| `[blob_storage].s3_force_path_style` | `SAFE_S3_FORCE_PATH_STYLE` |
| `[coordination].backend` | `SAFE_COORDINATION_BACKEND` |
| `[coordination].valkey_addr` | `SAFE_VALKEY_ADDR` |
| `[coordination].valkey_username` | `SAFE_VALKEY_USERNAME` |
| `[coordination].valkey_password` | `SAFE_VALKEY_PASSWORD` |
| `[coordination].valkey_password_file` | `SAFE_VALKEY_PASSWORD_FILE` |
| `[coordination].valkey_db` | `SAFE_VALKEY_DB` |
| `[coordination].valkey_tls` | `SAFE_VALKEY_TLS` |
| `[coordination].valkey_dial_timeout` | `SAFE_VALKEY_DIAL_TIMEOUT` |
| `[coordination].valkey_read_timeout` | `SAFE_VALKEY_READ_TIMEOUT` |
| `[coordination].valkey_write_timeout` | `SAFE_VALKEY_WRITE_TIMEOUT` |
| `[uploads].max_upload_bytes` | `SAFE_MAX_UPLOAD_BYTES` |
| `[uploads].account_default_blob_quota_bytes` | `SAFE_ACCOUNT_DEFAULT_BLOB_QUOTA_BYTES` |
| `[uploads].temp_upload_staging_quota_bytes` | `SAFE_TEMP_UPLOAD_STAGING_QUOTA_BYTES` |
| `[uploads].upload_coordination_lease_ttl` | `SAFE_UPLOAD_COORDINATION_LEASE_TTL` |
| `[uploads].temp_upload_cleanup_age` | `SAFE_TEMP_UPLOAD_CLEANUP_AGE` |
| `[uploads].temp_upload_cleanup_dry_run` | `SAFE_TEMP_UPLOAD_CLEANUP_DRY_RUN` |
| `[auth].session_ttl` | `SAFE_SESSION_TTL` |
| `[auth].bootstrap_secret` | `SAFE_AUTH_BOOTSTRAP_SECRET` |
| `[auth].bootstrap_secret_file` | `SAFE_AUTH_BOOTSTRAP_SECRET_FILE` |
| `[auth].second_factor_email_challenge_ttl` | `SAFE_SECOND_FACTOR_EMAIL_CHALLENGE_TTL` |
| `[account_registration].mode` | `SAFE_ACCOUNT_REGISTRATION_MODE` |
| `[account_registration].email_verification_ttl` | `SAFE_EMAIL_VERIFICATION_TTL` |
| `[account_registration].public_web_origin` | `SAFE_PUBLIC_WEB_ORIGIN` |
| `[email].backend` | `SAFE_EMAIL_BACKEND` |
| `[email].smtp_host` | `SAFE_SMTP_HOST` |
| `[email].smtp_port` | `SAFE_SMTP_PORT` |
| `[email].smtp_username` | `SAFE_SMTP_USERNAME` |
| `[email].smtp_password` | `SAFE_SMTP_PASSWORD` |
| `[email].smtp_password_file` | `SAFE_SMTP_PASSWORD_FILE` |
| `[email].smtp_from` | `SAFE_SMTP_FROM` |
| `[email].smtp_starttls` | `SAFE_SMTP_STARTTLS` |
| `[email].smtp_timeout` | `SAFE_SMTP_TIMEOUT` |
| `[web_auth].enabled` | `SAFE_WEB_AUTH_ENABLED` |
| `[web_auth].allowed_origins` | `SAFE_WEB_ALLOWED_ORIGINS` |
| `[web_auth].session_cookie_name` | `SAFE_WEB_SESSION_COOKIE_NAME` |
| `[web_auth].session_cookie_secure` | `SAFE_WEB_SESSION_COOKIE_SECURE` |
| `[web_auth].session_cookie_samesite` | `SAFE_WEB_SESSION_COOKIE_SAMESITE` |
| `[web_auth].csrf_header_name` | `SAFE_WEB_CSRF_HEADER_NAME` |
| `[retention].default_incident_token_ttl` | `SAFE_DEFAULT_INCIDENT_TOKEN_TTL` |
| `[retention].closed_incident_retention` | `SAFE_CLOSED_INCIDENT_RETENTION` |
| `[retention].token_metadata_retention` | `SAFE_TOKEN_METADATA_RETENTION` |
| `[retention].deletion_tombstone_retention` | `SAFE_DELETION_TOMBSTONE_RETENTION` |
| `[retention].deletion_worker_interval` | `SAFE_DELETION_WORKER_INTERVAL` |
| `[rate_limits.main_api].enabled` | `SAFE_MAIN_API_RATE_LIMIT_ENABLED` |
| `[rate_limits.main_api].window` | `SAFE_MAIN_API_RATE_LIMIT_WINDOW` |
| `[rate_limits.main_api].auth` | `SAFE_MAIN_API_RATE_LIMIT_AUTH` |
| `[rate_limits.main_api].auth_register` | `SAFE_MAIN_API_RATE_LIMIT_AUTH_REGISTER` |
| `[rate_limits.main_api].auth_email_verify` | `SAFE_MAIN_API_RATE_LIMIT_AUTH_EMAIL_VERIFY` |
| `[rate_limits.main_api].bootstrap` | `SAFE_MAIN_API_RATE_LIMIT_BOOTSTRAP` |
| `[rate_limits.main_api].account` | `SAFE_MAIN_API_RATE_LIMIT_ACCOUNT` |
| `[rate_limits.main_api].incident_read` | `SAFE_MAIN_API_RATE_LIMIT_INCIDENT_READ` |
| `[rate_limits.main_api].incident_write` | `SAFE_MAIN_API_RATE_LIMIT_INCIDENT_WRITE` |
| `[rate_limits.main_api].upload` | `SAFE_MAIN_API_RATE_LIMIT_UPLOAD` |
| `[rate_limits.main_api].reconcile` | `SAFE_MAIN_API_RATE_LIMIT_RECONCILE` |
| `[rate_limits.main_api].stream` | `SAFE_MAIN_API_RATE_LIMIT_STREAM` |
| `[rate_limits.main_api].token` | `SAFE_MAIN_API_RATE_LIMIT_TOKEN` |
| `[rate_limits.main_api].download` | `SAFE_MAIN_API_RATE_LIMIT_DOWNLOAD` |
| `[rate_limits.main_api].admin` | `SAFE_MAIN_API_RATE_LIMIT_ADMIN` |
| `[rate_limits.public_viewer].enabled` | `SAFE_PUBLIC_VIEWER_RATE_LIMIT_ENABLED` |
| `[rate_limits.public_viewer].window` | `SAFE_PUBLIC_VIEWER_RATE_LIMIT_WINDOW` |
| `[rate_limits.public_viewer].page` | `SAFE_PUBLIC_VIEWER_RATE_LIMIT_PAGE` |
| `[rate_limits.public_viewer].data` | `SAFE_PUBLIC_VIEWER_RATE_LIMIT_DATA` |
| `[rate_limits.public_viewer].download` | `SAFE_PUBLIC_VIEWER_RATE_LIMIT_DOWNLOAD` |
| `[rate_limits.public_viewer].static` | `SAFE_PUBLIC_VIEWER_RATE_LIMIT_STATIC` |
| `[http.main].read_header_timeout` | `SAFE_MAIN_READ_HEADER_TIMEOUT` |
| `[http.main].read_timeout` | `SAFE_MAIN_READ_TIMEOUT` |
| `[http.main].write_timeout` | `SAFE_MAIN_WRITE_TIMEOUT` |
| `[http.main].idle_timeout` | `SAFE_MAIN_IDLE_TIMEOUT` |
| `[http.admin].read_header_timeout` | `SAFE_ADMIN_READ_HEADER_TIMEOUT` |
| `[http.admin].read_timeout` | `SAFE_ADMIN_READ_TIMEOUT` |
| `[http.admin].write_timeout` | `SAFE_ADMIN_WRITE_TIMEOUT` |
| `[http.admin].idle_timeout` | `SAFE_ADMIN_IDLE_TIMEOUT` |

## Environment Variables

| Variable | Default | Notes |
|---|---|---|
| `SAFE_CONFIG_FILE` | unset | Optional TOML config path. Overridden by `--config`; otherwise checked before automatic `./proofline.toml` and `/etc/proofline/proofline.toml` discovery. |
| `SAFE_MAIN_BIND_ADDRS` | `127.0.0.1:8080` | Comma-separated main listener addresses for authenticated non-admin `/v1` routes and the read-only incident viewer. |
| `SAFE_ADMIN_BIND_ADDRS` | `127.0.0.1:8081` | Comma-separated private-admin listener addresses for admin-only `/v1/admin/...` JSON routes and the `/admin` dashboard route tree. |
| `SAFE_DATA_DIR` | `./data` | Local directory for SQLite, temp uploads, and encrypted blobs unless `SAFE_DB_PATH` points elsewhere. |
| `SAFE_DB_PATH` | `./data/proofline.db` | SQLite database path. |
| `SAFE_METADATA_BACKEND` | `sqlite` | Metadata backend selector. Supported values are `sqlite` and `postgresql`. |
| `SAFE_BLOB_BACKEND` | `local` | Encrypted blob backend selector. Supported values are `local` and `s3`. |
| `SAFE_COORDINATION_BACKEND` | `none` | Coordination backend selector. Supported values are `none`, `valkey`, and `redis`. |
| `SAFE_POSTGRES_DSN` | unset | PostgreSQL connection string. Required when `SAFE_METADATA_BACKEND=postgresql`; treat as secret-bearing. |
| `SAFE_POSTGRES_DSN_FILE` | unset | File containing the PostgreSQL connection string. Overrides `SAFE_POSTGRES_DSN` when set. |
| `SAFE_POSTGRES_MAX_OPEN_CONNS` | `10` | Maximum open PostgreSQL connections when the PostgreSQL metadata backend is selected. |
| `SAFE_POSTGRES_MAX_IDLE_CONNS` | `5` | Maximum idle PostgreSQL connections when the PostgreSQL metadata backend is selected. |
| `SAFE_POSTGRES_CONN_MAX_LIFETIME` | `30m` | Maximum lifetime for PostgreSQL connections. |
| `SAFE_S3_ENDPOINT` | unset | S3-compatible endpoint URL. Required when `SAFE_BLOB_BACKEND=s3`. |
| `SAFE_S3_REGION` | `us-east-1` | S3 signing region used when `SAFE_BLOB_BACKEND=s3`. |
| `SAFE_S3_BUCKET` | unset | S3 bucket for committed encrypted chunks. Required when `SAFE_BLOB_BACKEND=s3`. |
| `SAFE_S3_PREFIX` | unset | Optional server-controlled object key prefix for committed chunks. |
| `SAFE_S3_ACCESS_KEY_ID` | unset | Static S3 access key. Required when `SAFE_BLOB_BACKEND=s3`. |
| `SAFE_S3_ACCESS_KEY_ID_FILE` | unset | File containing the static S3 access key. Overrides `SAFE_S3_ACCESS_KEY_ID` when set. |
| `SAFE_S3_SECRET_ACCESS_KEY` | unset | Static S3 secret access key. Required when `SAFE_BLOB_BACKEND=s3`; treat as a secret. |
| `SAFE_S3_SECRET_ACCESS_KEY_FILE` | unset | File containing the static S3 secret access key. Overrides `SAFE_S3_SECRET_ACCESS_KEY` when set. |
| `SAFE_S3_SESSION_TOKEN` | unset | Optional static S3 session token. Requires static S3 credentials. |
| `SAFE_S3_SESSION_TOKEN_FILE` | unset | File containing an optional static S3 session token. Overrides `SAFE_S3_SESSION_TOKEN` when set. |
| `SAFE_S3_FORCE_PATH_STYLE` | `true` | Use path-style bucket addressing for S3-compatible services. Set to `false` for virtual-hosted-style services that require it. |
| `SAFE_VALKEY_ADDR` | unset | Valkey/Redis-compatible `host:port`. Required when `SAFE_COORDINATION_BACKEND=valkey` or `redis`. |
| `SAFE_VALKEY_USERNAME` | unset | Optional Valkey ACL username. |
| `SAFE_VALKEY_PASSWORD` | unset | Optional Valkey password; treat as a secret. |
| `SAFE_VALKEY_PASSWORD_FILE` | unset | File containing the optional Valkey password. Overrides `SAFE_VALKEY_PASSWORD` when set. |
| `SAFE_VALKEY_DB` | `0` | Non-negative Valkey database number. |
| `SAFE_VALKEY_TLS` | `false` | Use TLS for the Valkey connection. |
| `SAFE_VALKEY_DIAL_TIMEOUT` | `5s` | Valkey dial timeout. |
| `SAFE_VALKEY_READ_TIMEOUT` | `5s` | Valkey read timeout. |
| `SAFE_VALKEY_WRITE_TIMEOUT` | `5s` | Valkey write timeout. |
| `SAFE_UPLOAD_COORDINATION_LEASE_TTL` | `2m` | Short TTL for Valkey-backed complete-upload in-progress leases and retry hints. Must be positive. |
| `SAFE_MAX_UPLOAD_BYTES` | `250MB` | Maximum encrypted file bytes per upload. |
| `SAFE_ACCOUNT_DEFAULT_BLOB_QUOTA_BYTES` | `10GB` | Default committed encrypted blob quota per owner account. Counted from accepted chunk metadata across owned incidents for both local and S3-compatible blob backends. |
| `SAFE_TEMP_UPLOAD_STAGING_QUOTA_BYTES` | `1GB` | Maximum regular `upload-*` temp staging bytes under the local temp directory before new upload bytes fail closed with a generic staging-quota error. Applies to local blob storage and S3-compatible blob staging. |
| `SAFE_DEFAULT_INCIDENT_TOKEN_TTL` | `24h` | Default lifetime for viewer tokens created without `expires_at`. Set to `0` to disable the default for omitted `expires_at` values. |
| `SAFE_SESSION_TTL` | `12h` | Lifetime for local account sessions created by `/v1/auth/login`. |
| `SAFE_SECOND_FACTOR_EMAIL_CHALLENGE_TTL` | `10m` | Lifetime for single-use email second-factor setup challenge codes. Must be positive. |
| `SAFE_ACCOUNT_REGISTRATION_MODE` | `disabled` | Public account registration mode. Supported values are `disabled`, `admin_only`, `open`, and `paid`. `open` requires SMTP email verification; `paid` is a fail-closed placeholder. |
| `SAFE_EMAIL_VERIFICATION_TTL` | `24h` | Lifetime for single-use email verification tokens. Must be positive. |
| `SAFE_PUBLIC_WEB_ORIGIN` | unset | Web-client origin used to build email verification links such as `/verify-email#token=...`. Required when `SAFE_ACCOUNT_REGISTRATION_MODE=open`. |
| `SAFE_EMAIL_BACKEND` | `none` | Outbound email backend. Supported values are `none` and `smtp`. `none` cannot be used with open registration. |
| `SAFE_SMTP_HOST` | unset | SMTP host. Required when `SAFE_EMAIL_BACKEND=smtp`; treat private hostnames as deployment details. |
| `SAFE_SMTP_PORT` | `587` | SMTP TCP port. Must be positive. |
| `SAFE_SMTP_USERNAME` | unset | Optional SMTP username. Required when `SAFE_SMTP_PASSWORD` is set. |
| `SAFE_SMTP_PASSWORD` | unset | Optional SMTP password; treat as a secret. |
| `SAFE_SMTP_PASSWORD_FILE` | unset | File containing the optional SMTP password. Overrides `SAFE_SMTP_PASSWORD` when set. |
| `SAFE_SMTP_FROM` | unset | Sender email address for verification messages. Required when `SAFE_EMAIL_BACKEND=smtp`. |
| `SAFE_SMTP_STARTTLS` | `required` | SMTP STARTTLS behavior. Supported values are `required`, `opportunistic`, and `disabled`. |
| `SAFE_SMTP_TIMEOUT` | `10s` | SMTP dial timeout. Must be positive. |
| `SAFE_AUTH_BOOTSTRAP_SECRET` | unset | One-time bootstrap secret required to create the first admin account when no admin exists. Remove after bootstrap. |
| `SAFE_AUTH_BOOTSTRAP_SECRET_FILE` | unset | File containing the one-time bootstrap secret. Overrides `SAFE_AUTH_BOOTSTRAP_SECRET` when set. |
| `SAFE_WEB_AUTH_ENABLED` | `false` | Enables main `/v1` browser cookie-session routes for the future web client. Existing bearer-token routes continue to work. |
| `SAFE_WEB_ALLOWED_ORIGINS` | unset | Comma-separated exact web origins that may receive credentialed CORS responses. Wildcards are rejected. |
| `SAFE_WEB_SESSION_COOKIE_NAME` | `__Host-proofline_session` | Browser session cookie name. The default production name requires `SAFE_WEB_SESSION_COOKIE_SECURE=true`; local plain-HTTP development should use a non-`__Host-` name. |
| `SAFE_WEB_SESSION_COOKIE_SECURE` | `true` | Sets the browser session cookie `Secure` attribute. `false` is accepted only with local loopback web origins. |
| `SAFE_WEB_SESSION_COOKIE_SAMESITE` | `lax` | Browser session cookie SameSite policy. Supported values are `lax` and `strict`. |
| `SAFE_WEB_CSRF_HEADER_NAME` | `X-CSRF-Token` | Header required on unsafe browser-cookie-authenticated requests. |
| `SAFE_DELETION_WORKER_INTERVAL` | `1m` | Background deletion maintenance interval. Set to `0` to disable the automatic scheduler while keeping deletion decisions durable for a later run. |
| `SAFE_CLOSED_INCIDENT_RETENTION` | `0` | Retention window for closed incidents. `0` disables automatic retention deletion; positive Go durations delete closed incidents older than the window. |
| `SAFE_TOKEN_METADATA_RETENTION` | `0` | Audit window for pruning expired or revoked viewer-token metadata. `0` disables token metadata pruning. |
| `SAFE_DELETION_TOMBSTONE_RETENTION` | `0` | Retention window for minimal deleted-incident tombstones after deletion completion. `0` disables tombstone pruning. |
| `SAFE_TEMP_UPLOAD_CLEANUP_AGE` | `0` | Minimum age for startup cleanup of orphaned local temp upload files. `0` disables cleanup. |
| `SAFE_TEMP_UPLOAD_CLEANUP_DRY_RUN` | `false` | When temp cleanup is enabled, log safe counts without deleting eligible temp files. |
| `SAFE_MAIN_API_RATE_LIMIT_ENABLED` | `true` | Enables app-level rate limiting for main API route classes. Set to `false` to disable the app-level limiter. |
| `SAFE_MAIN_API_RATE_LIMIT_WINDOW` | `1m` | Fixed-window duration for app-level main API limits. |
| `SAFE_MAIN_API_RATE_LIMIT_AUTH` | `30` | Main API bearer login/logout and browser cookie login/logout/CSRF requests allowed per window per hashed socket peer. Set to `0` to disable this route-class limit. |
| `SAFE_MAIN_API_RATE_LIMIT_AUTH_REGISTER` | `10` | Public registration requests allowed per window per hashed socket peer. Set to `0` to disable this route-class limit. |
| `SAFE_MAIN_API_RATE_LIMIT_AUTH_EMAIL_VERIFY` | `30` | Registration email verification, email second-factor challenge/verify, and TOTP enroll/confirm/verify requests allowed per window per hashed socket peer. Set to `0` to disable this route-class limit. |
| `SAFE_MAIN_API_RATE_LIMIT_BOOTSTRAP` | `5` | Compatibility setting for the legacy JSON bootstrap route class. The current first-admin bootstrap flow is the private `/admin/bootstrap` form. |
| `SAFE_MAIN_API_RATE_LIMIT_ACCOUNT` | `120` | Account self-service, owner account/device recipient-key metadata, trusted-contact relationship metadata, and owner contact public-key metadata requests allowed per window per hashed socket peer. Set to `0` to disable this route-class limit. |
| `SAFE_MAIN_API_RATE_LIMIT_INCIDENT_READ` | `300` | Incident metadata, sharing-grant metadata, and wrapped-key metadata read requests allowed per window per hashed socket peer. Set to `0` to disable this route-class limit. |
| `SAFE_MAIN_API_RATE_LIMIT_INCIDENT_WRITE` | `120` | Incident create, close, owner-scoped deletion, sharing-grant metadata, and wrapped-key metadata write requests allowed per window per hashed socket peer. Set to `0` to disable this route-class limit. |
| `SAFE_MAIN_API_RATE_LIMIT_UPLOAD` | `120` | Complete encrypted chunk upload requests allowed per window per hashed socket peer. Set to `0` to disable this route-class limit. |
| `SAFE_MAIN_API_RATE_LIMIT_RECONCILE` | `120` | Duplicate chunk reconciliation requests allowed per window per hashed socket peer. Set to `0` to disable this route-class limit. |
| `SAFE_MAIN_API_RATE_LIMIT_STREAM` | `120` | Stream create/read/complete/fail requests allowed per window per hashed socket peer. Set to `0` to disable this route-class limit. |
| `SAFE_MAIN_API_RATE_LIMIT_TOKEN` | `60` | Incident-token create/revoke requests allowed per window per hashed socket peer. Set to `0` to disable this route-class limit. |
| `SAFE_MAIN_API_RATE_LIMIT_DOWNLOAD` | `30` | Private chunk and encrypted bundle download requests allowed per window per hashed socket peer. Set to `0` to disable this route-class limit. |
| `SAFE_MAIN_API_RATE_LIMIT_ADMIN` | `60` | Compatibility setting for older main-handler admin API rate-limit configuration. Current `/v1/admin/...` JSON routes are mounted on the private-admin listener and are not classified by the main API limiter. |
| `SAFE_PUBLIC_VIEWER_RATE_LIMIT_ENABLED` | `true` | Enables app-level rate limiting for public incident viewer route classes. Set to `false` to disable the app-level limiter. |
| `SAFE_PUBLIC_VIEWER_RATE_LIMIT_WINDOW` | `1m` | Fixed-window duration for app-level public viewer limits. |
| `SAFE_PUBLIC_VIEWER_RATE_LIMIT_PAGE` | `60` | Public viewer page lookup requests allowed per window per hashed socket peer. Set to `0` to disable this route-class limit. |
| `SAFE_PUBLIC_VIEWER_RATE_LIMIT_DATA` | `300` | Public viewer JSON polling requests allowed per window per hashed socket peer. Set to `0` to disable this route-class limit. |
| `SAFE_PUBLIC_VIEWER_RATE_LIMIT_DOWNLOAD` | `12` | Public viewer encrypted ZIP download starts allowed per window per hashed socket peer. Set to `0` to disable this route-class limit. |
| `SAFE_PUBLIC_VIEWER_RATE_LIMIT_STATIC` | `600` | Public viewer static asset requests allowed per window per hashed socket peer. Set to `0` to disable this route-class limit. |
| `SAFE_MAIN_READ_HEADER_TIMEOUT` | `10s` | Main API/viewer HTTP read-header timeout. |
| `SAFE_MAIN_READ_TIMEOUT` | `0s` | Main API/viewer HTTP read timeout. `0` disables it for large or slow uploads. |
| `SAFE_MAIN_WRITE_TIMEOUT` | `0s` | Main API/viewer HTTP write timeout. `0` disables it for large uploads, authenticated downloads, and viewer ZIP downloads. |
| `SAFE_MAIN_IDLE_TIMEOUT` | `120s` | Main API/viewer HTTP idle connection timeout. |
| `SAFE_ADMIN_READ_HEADER_TIMEOUT` | `10s` | Private-admin HTTP read-header timeout. |
| `SAFE_ADMIN_READ_TIMEOUT` | `30s` | Private-admin HTTP read timeout. |
| `SAFE_ADMIN_WRITE_TIMEOUT` | `300s` | Private-admin HTTP write timeout. |
| `SAFE_ADMIN_IDLE_TIMEOUT` | `120s` | Private-admin HTTP idle connection timeout. |

The older singular variables `SAFE_MAIN_BIND_ADDR` and `SAFE_ADMIN_BIND_ADDR`
are supported when the matching plural variable is unset. Plural variables
take precedence. `SAFE_PRIVATE_BIND_ADDRS` and `SAFE_PRIVATE_BIND_ADDR` remain
accepted as legacy aliases for the main listener only. `SAFE_PUBLIC_BIND_ADDRS`
and `SAFE_PUBLIC_BIND_ADDR` now fail startup so a previously public viewer bind
cannot silently become the private-admin listener.

There are no implemented regional stream-ingress relay configuration variables.
The future relay configuration and service identity shape is planning-only in
[regional-stream-ingress-relay.md](regional-stream-ingress-relay.md). Any later
relay settings should use a distinct namespace, keep the relay upload-only, and
avoid logging service credentials, token fingerprints, staging paths, object
keys, private endpoints, or other private deployment details.

## Backend Selection Scaffold

The backend selector variables are a startup validation scaffold for cluster support. Local-first values remain the defaults:

Using TOML:

```toml
[metadata]
backend = "sqlite"

[blob_storage]
backend = "local"

[coordination]
backend = "none"
```

Environment-only deployments remain supported:

```bash
SAFE_METADATA_BACKEND=sqlite \
SAFE_BLOB_BACKEND=local \
SAFE_COORDINATION_BACKEND=none \
go run ./cmd/api
```

Values are matched case-insensitively after trimming surrounding whitespace. Unsupported names fail startup with a clear configuration error.

PostgreSQL metadata is implemented as an optional backend for new deployments.
Prefer a secret file for the DSN:

```toml
[metadata]
backend = "postgresql"
postgres_dsn_file = "/run/secrets/proofline-postgres-dsn"
```

The equivalent environment-only shape remains supported:

```bash
SAFE_METADATA_BACKEND=postgresql \
SAFE_POSTGRES_DSN='postgres://proofline:example-password@db.example.invalid:5432/proofline?sslmode=require' \
SAFE_BLOB_BACKEND=local \
SAFE_COORDINATION_BACKEND=none \
go run ./cmd/api
```

`SAFE_POSTGRES_DSN` may contain credentials and private hostnames. Do not log it
or include it in public issues, support tickets, screenshots, shell history, or
deployment notes. `SAFE_DB_PATH` remains the SQLite database path and is ignored
by the PostgreSQL metadata backend.

S3-compatible object storage is implemented as an optional encrypted blob backend for committed chunks. Prefer secret files for static credentials:

```toml
[blob_storage]
backend = "s3"
s3_endpoint = "https://s3.example.invalid"
s3_region = "us-east-1"
s3_bucket = "proofline-evidence"
s3_prefix = "prod/server"
s3_access_key_id_file = "/run/secrets/proofline-s3-access-key-id"
s3_secret_access_key_file = "/run/secrets/proofline-s3-secret-access-key"
s3_force_path_style = true
```

The equivalent environment-only shape remains supported:

```bash
SAFE_METADATA_BACKEND=sqlite \
SAFE_BLOB_BACKEND=s3 \
SAFE_COORDINATION_BACKEND=none \
SAFE_S3_ENDPOINT=https://s3.example.invalid \
SAFE_S3_REGION=us-east-1 \
SAFE_S3_BUCKET=proofline-evidence \
SAFE_S3_PREFIX=prod/server \
SAFE_S3_ACCESS_KEY_ID=example-access-key \
SAFE_S3_SECRET_ACCESS_KEY=example-secret-key \
go run ./cmd/api
```

Valkey/Redis-compatible coordination is implemented as an optional, explicit
backend. The current server validates the configured service at startup.
Main API and public viewer app-level rate-limit counters use the configured
Valkey service when `SAFE_COORDINATION_BACKEND=valkey` or `redis`; otherwise
they use local in-memory process counters. Upload routes still use complete
encrypted chunk uploads. When Valkey is configured, the upload handler also
uses short-lived complete-upload leases to reduce duplicate final commit and
metadata work and return safe `upload_in_progress` retry hints. The lease is
acquired after the encrypted request body is staged and verified; it is not a
resumable-transfer or bandwidth-saving lease. Complete-upload idempotency keys
and final upload-operation state are stored in the selected metadata backend,
not Valkey. Resumable uploads and partial-upload sessions remain out of scope.

`SAFE_DB_PATH` and `SAFE_DATA_DIR` keep their current behavior for the supported `sqlite` and `local` backends. When `SAFE_METADATA_BACKEND=postgresql`, `SAFE_DB_PATH` is not used for metadata. When `SAFE_BLOB_BACKEND=s3`, `SAFE_DATA_DIR/tmp` is still used for local temporary upload staging before final object writes.

PostgreSQL schema, migration, test, and restore expectations are documented in
[PostgreSQL metadata migration path](postgresql-metadata-migration.md). Initial
PostgreSQL support is for new metadata deployments only. The server does not
automatically migrate an existing SQLite database to PostgreSQL at startup.

Cluster backup, restore, and failure-mode guidance for PostgreSQL metadata,
S3-compatible encrypted blobs, and Valkey/Redis-compatible coordination is
documented in
[Cluster backup, restore, and failure runbook](cluster-backup-restore-runbook.md).

## S3-Compatible Blob Storage

The S3-compatible backend stores only opaque encrypted chunk bytes. It does not add backend decryption, raw media keys, key escrow, browser decryption, broad public `/v1` exposure, public account workflows, or production-readiness guarantees.

Uploads are first staged as local temp files under the configured data
directory's `tmp` subdirectory while the server enforces the configured
`[uploads].max_upload_bytes` limit and computes SHA-256 over the uploaded
ciphertext. After the client-provided hash is verified, the server writes the
final object key with conditional no-overwrite behavior. The final object key
is derived from server-controlled incident, stream, media type, and chunk
index metadata:

```text
{s3_prefix}/incidents/{incident_id}/streams/{stream_id}/{media_type}_{zero_padded_chunk_index}.enc
{s3_prefix}/incidents/{incident_id}/{media_type}_{zero_padded_chunk_index}.enc
```

The optional prefix must be relative and must not contain empty, `.`, `..`, or backslash path segments. Client requests never provide final object keys or stored paths.

Account-scoped committed blob quota is enforced from metadata, not from object
key listing. `[uploads].account_default_blob_quota_bytes` counts accepted
encrypted chunk `byte_size` values for incidents owned by the account. Chunks
continue to count while deletion is pending or retrying and stop counting only
after durable deletion completes and chunk metadata is pruned. Failed, staged,
or orphan temp uploads are separate from committed quota and are bounded by the
local temp-upload staging quota.

Use HTTPS for S3-compatible endpoints unless the endpoint is limited to a local
or private test network. Plain HTTP object-storage traffic can expose
credentials, session tokens, object keys, and encrypted evidence bytes to the
network path. Before enabling a provider for evidence storage, run a small
no-overwrite smoke test that confirms conditional writes reject an existing
object instead of replacing it.

This implementation does not create S3 staging objects. Failed uploads and
hash mismatches clean up local temp files through the normal upload path. If
the process crashes, abandoned local temp files under the configured data
directory's `tmp` subdirectory may remain and should be cleaned only by a
conservative operator policy that never deletes committed objects.
`[uploads].temp_upload_staging_quota_bytes` applies to the same local staging
directory for both local and S3-compatible blob backends and rejects additional
upload bytes with a generic `507 upload_staging_quota_exceeded` response when
regular `upload-*` staging files reach the configured limit.
`[uploads].temp_upload_cleanup_age` applies to this local staging directory for
both local and S3-compatible blob backends. Object-store lifecycle cleanup for
staging prefixes is not needed unless a future resumable or multipart S3
staging design adds such prefixes.

S3 access key ID and secret access key settings are required when the S3
backend is selected. Prefer `[blob_storage].s3_access_key_id_file` and
`[blob_storage].s3_secret_access_key_file`; environment-only deployments can
use `SAFE_S3_ACCESS_KEY_ID_FILE` and `SAFE_S3_SECRET_ACCESS_KEY_FILE`, or the
direct secret variables for short-lived local overrides. The session token is
optional. Credentials, endpoints, bucket names, object keys, and private
deployment details should not be written to public issue drafts, logs, or
support tickets.

Bundle downloads continue to generate server-controlled ZIP entry names such as `chunks/audio_000001.enc`; they do not expose object-store URLs, bucket names, configured prefixes, or filesystem paths.

## Optional Valkey / Redis-Compatible Coordination

No coordination backend is used by default. To enable Valkey or another
Redis-compatible service for short-lived coordination, explicitly set the
coordination selector and connection settings:

Using TOML with a password file:

```toml
[coordination]
backend = "valkey"
valkey_addr = "valkey.example.invalid:6379"
valkey_username = "proofline"
valkey_password_file = "/run/secrets/proofline-valkey-password"
valkey_tls = true
```

Environment-only deployments remain supported:

```bash
SAFE_COORDINATION_BACKEND=valkey \
SAFE_VALKEY_ADDR=valkey.example.invalid:6379 \
SAFE_VALKEY_USERNAME=proofline \
SAFE_VALKEY_PASSWORD=example-password \
SAFE_VALKEY_TLS=true \
go run ./cmd/api
```

`SAFE_COORDINATION_BACKEND=redis` is accepted as an alias for Redis-compatible
deployments. `SAFE_VALKEY_ADDR` must be a `host:port`, not a URL, so passwords
and database numbers stay in their dedicated settings.

Treat Valkey passwords, private hostnames, private network details,
rate-limit counter keys, and any future coordination keys as private
deployment details. Do not put them in public issues, logs, dashboards,
screenshots, support tickets, or metrics labels.

Coordination is not durable evidence storage. Incident metadata and
viewer-token metadata remain in the selected metadata backend, and committed
encrypted bytes remain in the selected blob backend. If a configured Valkey
backend cannot be checked at startup, the server fails closed instead of
silently running with a misleading cluster configuration.

The current implementation stores only short-lived main API/public viewer
rate-limit counters and complete-upload lease keys in Valkey when coordination
is configured. Rate-limit keys are server-controlled route-class keys using a
hash of the socket peer identity. Upload lease keys use a server-controlled
hash of normalized chunk identity and expire after
`SAFE_UPLOAD_COORDINATION_LEASE_TTL`. Those keys do not include raw
`/i/{token}` paths, legacy `/e/{token}` paths, `/v1` incident paths, raw viewer
tokens, raw incident tokens, raw session tokens, Authorization headers, raw
idempotency keys, request bodies, uploaded bytes, plaintext, raw keys, stored
paths, staging paths, object keys, user safety data, or private deployment
details. Valkey does not store idempotency results or committed evidence
truth.

## Bind Address Lists

`SAFE_MAIN_BIND_ADDRS` and `SAFE_ADMIN_BIND_ADDRS` are comma-separated
`host:port` lists.

Empty entries are rejected. These values fail startup:

```text
,
127.0.0.1:8080,,10.66.0.1:8080
```

TOML accepts these as arrays:

```toml
[server]
main_bind_addrs = ["127.0.0.1:8080", "10.66.0.1:8080"]
admin_bind_addrs = ["127.0.0.1:8081"]
```

Environment-only deployments remain supported:

```bash
SAFE_MAIN_BIND_ADDRS=127.0.0.1:8080,10.66.0.1:8080 \
SAFE_ADMIN_BIND_ADDRS=127.0.0.1:8081 \
go run ./cmd/api
```

## Upload Size And Blob Quotas

`SAFE_MAX_UPLOAD_BYTES` accepts a positive byte count or binary unit suffix:

- `B`
- `K` / `KB`
- `M` / `MB`
- `G` / `GB`

Fractional unit values are allowed when they resolve to at least one byte, for example `0.5KB`. Non-positive, sub-byte, invalid, and oversized values are rejected during startup.

Using TOML:

```toml
[uploads]
max_upload_bytes = "250MB"
account_default_blob_quota_bytes = "10GB"
temp_upload_staging_quota_bytes = "1GB"
```

Environment override:

```bash
SAFE_MAX_UPLOAD_BYTES=250MB \
SAFE_ACCOUNT_DEFAULT_BLOB_QUOTA_BYTES=10GB \
SAFE_TEMP_UPLOAD_STAGING_QUOTA_BYTES=1GB \
go run ./cmd/api
```

`SAFE_ACCOUNT_DEFAULT_BLOB_QUOTA_BYTES` uses the same byte parser and defaults
to 10 GB. It limits committed encrypted chunk bytes per owner account across
all owned incidents. Equivalent duplicate or idempotent retries do not add new
committed bytes. Deletion frees quota only after blob deletion has completed
and the associated chunk metadata has been removed. The setting is an
abuse/cost control for preview deployments, not billing, subscription, account
plan, or payment-gating logic.

`SAFE_TEMP_UPLOAD_STAGING_QUOTA_BYTES` uses the same byte parser and defaults
to 1 GB. It limits regular local `upload-*` staging files under the configured
data directory's `tmp` subdirectory before final chunk commit. The limit applies
to both local blob storage and S3-compatible blob staging because both paths
stage upload bytes locally before hash verification and final commit. It is
separate from `SAFE_MAX_UPLOAD_BYTES`, committed account quota, and conservative
orphan temp cleanup. When staging pressure reaches the configured limit, chunk
upload returns a generic `507 upload_staging_quota_exceeded` response without
exposing temp paths, stored paths, object keys, bucket names, or uploaded bytes.

## Viewer Token Expiry

Viewer tokens created without an explicit `expires_at` default to expiring after `SAFE_DEFAULT_INCIDENT_TOKEN_TTL`, which is `24h` unless configured otherwise.

The value uses Go duration strings such as `12h` or `168h`.

Set `[retention].default_incident_token_ttl = "0"` only when you deliberately
want omitted `expires_at` values to create tokens that remain valid until
revoked. The `SAFE_DEFAULT_INCIDENT_TOKEN_TTL=0` environment override remains
supported.

Using TOML:

```toml
[retention]
default_incident_token_ttl = "24h"
```

Environment override:

```bash
SAFE_DEFAULT_INCIDENT_TOKEN_TTL=24h go run ./cmd/api
```

## Local Account Sessions

The main `/v1` API requires local account sessions. Sessions created by
`POST /v1/auth/login` expire after `SAFE_SESSION_TTL`, which defaults to `12h`.
The private `/admin` browser flow uses the same session store and TTL, with the
raw session token held in an HttpOnly SameSite cookie scoped to `/admin`. The
value uses Go duration strings such as `6h` or `30m`.

Public account registration is disabled by default:

```toml
[account_registration]
mode = "disabled"
```

Environment override:

```bash
SAFE_ACCOUNT_REGISTRATION_MODE=disabled
```

`admin_only` also rejects public registration while preserving existing
admin-created account flows. `open` enables public self-registration for
self-hosted deployments, but it requires email verification before login:

```toml
[account_registration]
mode = "open"
public_web_origin = "https://app.example.invalid"

[email]
backend = "smtp"
smtp_host = "smtp.example.invalid"
smtp_port = 587
smtp_from = "noreply@example.invalid"
smtp_starttls = "required"
smtp_password_file = "/run/secrets/proofline-smtp-password"
```

Environment-only deployments remain supported:

```bash
SAFE_ACCOUNT_REGISTRATION_MODE=open \
SAFE_PUBLIC_WEB_ORIGIN=https://app.example.invalid \
SAFE_EMAIL_BACKEND=smtp \
SAFE_SMTP_HOST=smtp.example.invalid \
SAFE_SMTP_PORT=587 \
SAFE_SMTP_FROM=noreply@example.invalid \
SAFE_SMTP_STARTTLS=required \
go run ./cmd/api
```

If `open` is selected without a usable SMTP backend and public web origin,
startup fails closed. Verification links use
`{SAFE_PUBLIC_WEB_ORIGIN}/verify-email#token=<raw-token>`, and the backend
stores only token hashes. The raw verification token must be treated as a
secret and must not appear in logs, metrics labels, docs examples, support
tickets, screenshots, shell history, or public issue drafts.

New admin-created, `/admin` bootstrap, and open-registration accounts are
created with `second_factor_setup_state=setup_required`; existing migrated
accounts default to `not_required` for preview compatibility. Password login
and browser-cookie login can create a primary-authenticated session for an
active setup-incomplete account, but main product routes fail closed until
email challenge or TOTP second-factor setup verifies the account and marks the
account `complete`. Email challenge uses the configured SMTP sender, stores
only challenge-code hashes, and remains distinct from registration email
verification. TOTP setup uses fixed six-digit SHA-1 codes with 30-second time
steps and one adjacent step of clock skew on either side; active TOTP factors
require each new session to verify TOTP before product-route access. There is
no `SAFE_*` setting in this foundation to choose WebAuthn, passkeys, recovery
codes, or lost-factor behavior.

`SAFE_ACCOUNT_REGISTRATION_MODE=paid` is accepted only as a future
hosted-service placeholder. `POST /v1/auth/register` returns
`503 registration_payment_unavailable`; it does not create an active account,
start checkout, contact a billing provider, or process subscriptions.

When `SAFE_WEB_AUTH_ENABLED=true`, the main `/v1` API also supports a dedicated
browser session cookie through `POST /v1/auth/web/login`,
`GET /v1/auth/web/csrf`, and `POST /v1/auth/web/logout`. Browser login creates
the same hashed server-side session records as bearer login, but it does not
return the raw session token in JSON. `GET /v1/account` and other authenticated
`/v1` routes can use the browser cookie when no bearer token is present.
Requests that send both bearer and browser-cookie credentials are rejected as
ambiguous.

Cookie-authenticated unsafe requests require the configured CSRF header. The
token returned by `/v1/auth/web/csrf` is bound to the server-side session with
HMAC and is not stored separately in SQLite or PostgreSQL. Bearer clients keep
their existing behavior and do not need the CSRF header.

Credentialed CORS is disabled unless `SAFE_WEB_ALLOWED_ORIGINS` is configured.
Origins must match exactly and `*` is rejected because credentials are allowed.
For local plain-HTTP web-client development, use a non-`__Host-` cookie name and
local origins only, for example:

```toml
[web_auth]
enabled = true
allowed_origins = ["http://127.0.0.1:5173"]
session_cookie_name = "proofline_session"
session_cookie_secure = false
```

Environment-only deployments remain supported:

```bash
SAFE_WEB_AUTH_ENABLED=true \
SAFE_WEB_ALLOWED_ORIGINS=http://127.0.0.1:5173 \
SAFE_WEB_SESSION_COOKIE_NAME=proofline_session \
SAFE_WEB_SESSION_COOKIE_SECURE=false \
go run ./cmd/api
```

Production deployments should keep the default `__Host-proofline_session`,
`Secure`, host-only, `Path=/` cookie shape and serve the web client over HTTPS.
Browser token persistence should not use localStorage in production.

For a new metadata database, startup fails until an admin account exists unless
a bootstrap secret is configured. Use that secret only long enough to create
the first admin through the private `/admin` bootstrap screen or
`POST /admin/bootstrap`, then remove it from TOML, the environment, or the
secret mount and restart.

For repeatable local or private deployments, prefer a secret file reference:

```toml
[auth]
bootstrap_secret_file = "/run/secrets/proofline-bootstrap-secret"
```

For a one-off local shell, an environment override remains supported:

```bash
SAFE_AUTH_BOOTSTRAP_SECRET='replace-with-local-bootstrap-secret' go run ./cmd/api
```

Treat the bootstrap secret, account passwords, session tokens, raw
idempotency keys, and Authorization headers as secrets. They must not appear in
public issues, logs, dashboards, screenshots, support tickets, or shell
history.

## Deletion And Retention

The server starts a background deletion worker by default. The worker processes
durable incident deletion decisions created through private owner-scoped or
admin routes, deletes encrypted blobs by server-controlled stored paths from
metadata, prunes sensitive child metadata after blob deletion, and leaves a
minimal tombstone.

Using TOML:

```toml
[retention]
deletion_worker_interval = "30s"
```

Environment override:

```bash
SAFE_DELETION_WORKER_INTERVAL=30s \
go run ./cmd/api
```

Set `[retention].deletion_worker_interval = "0"` to disable the automatic
scheduler. The `SAFE_DELETION_WORKER_INTERVAL=0` environment override remains
supported. This does not delete or discard pending deletion decisions; a later
process run with the worker enabled can resume them.

Closed-incident retention is disabled by default. To queue deletion decisions
for closed incidents older than a configured window, set a positive duration:

```toml
[retention]
closed_incident_retention = "720h"
```

Environment override:

```bash
SAFE_CLOSED_INCIDENT_RETENTION=720h \
go run ./cmd/api
```

Open incidents are not selected by automatic retention. Deleting an open
incident requires an explicit private deletion request with `allow_open: true`.
Mode-specific retention windows and backup expiry are not configured by these
variables.

Expired or revoked viewer-token metadata pruning is disabled by default. Set a
positive audit window only after reviewing whether token labels and token-hash
metadata must remain available for operational review:

```toml
[retention]
token_metadata_retention = "168h"
```

Environment override:

```bash
SAFE_TOKEN_METADATA_RETENTION=168h \
go run ./cmd/api
```

Token metadata pruning removes only incident-token rows whose `expires_at` or
`revoked_at` timestamp is older than the configured window. It does not delete
incidents, streams, chunks, checkins, blobs, backups, or raw tokens. Raw viewer
tokens are not stored.

Deleted-incident tombstone pruning is also disabled by default:

```toml
[retention]
deletion_tombstone_retention = "2160h"
```

Environment override:

```bash
SAFE_DELETION_TOMBSTONE_RETENTION=2160h \
go run ./cmd/api
```

Tombstone pruning removes only completed minimal tombstones after deletion retry
state is no longer needed and no sensitive child metadata remains. Backup
expiry, restore reconciliation, object-store versions, filesystem snapshots,
and downloaded bundles remain deployment responsibilities.

## Orphan Temp Upload Cleanup

Temp upload cleanup is disabled by default. To clean up abandoned local upload
staging files after a crash, set a positive age threshold and restart the
server:

```toml
[uploads]
temp_upload_cleanup_age = "24h"
```

Environment override:

```bash
SAFE_TEMP_UPLOAD_CLEANUP_AGE=24h \
go run ./cmd/api
```

Only regular files whose names match the server's `upload-*` temp-upload
pattern under `SAFE_DATA_DIR/tmp` are eligible. Active files newer than the
configured age are skipped. Directories, symlinks, unrelated temp files,
committed chunk blobs, stored object keys, SQLite or PostgreSQL metadata, and
evidence bundle contents are never cleanup targets.

To preview safe counts without deleting files:

```toml
[uploads]
temp_upload_cleanup_age = "24h"
temp_upload_cleanup_dry_run = true
```

Environment override:

```bash
SAFE_TEMP_UPLOAD_CLEANUP_AGE=24h \
SAFE_TEMP_UPLOAD_CLEANUP_DRY_RUN=true \
go run ./cmd/api
```

Cleanup logs only counts such as scanned, eligible, removed, skipped, and error
totals. Logs must not include temp paths, committed stored paths, object keys,
request bodies, uploaded bytes, raw tokens, plaintext, raw keys, or private
deployment details.

## HTTP Timeouts

Timeout values use Go duration strings such as `10s`, `30s`, or `5m`. `0` and `0s` disable a timeout.

Using TOML:

```toml
[http.main]
read_header_timeout = "10s"
read_timeout = "0"
write_timeout = "0"
idle_timeout = "120s"

[http.admin]
read_header_timeout = "10s"
read_timeout = "30s"
write_timeout = "300s"
idle_timeout = "120s"
```

Main read and write timeouts default to disabled so slow chunk uploads, private
downloads, and viewer ZIP downloads are not accidentally cut off. Private-admin
requests use finite defaults because they do not accept large evidence upload
bodies. `SAFE_PRIVATE_*` timeout variables remain accepted as legacy aliases
for `SAFE_MAIN_*` when the matching main timeout variable is unset.

## Data Directory Layout

By default:

```text
data/
  proofline.db
  proofline.db-wal
  proofline.db-shm
  tmp/
  incidents/{incident_id}/streams/{stream_id}/{media_type}_{zero_padded_chunk_index}.enc
  incidents/{incident_id}/{media_type}_{zero_padded_chunk_index}.enc
```

The `proofline.db-wal` and `proofline.db-shm` sidecar files appear while SQLite is
running in WAL mode. Keep them on the same local filesystem as the main
database and include them when making a direct live copy. See
[SQLite WAL operations](deployment.md#sqlite-wal-operations) for deployment,
backup, restore, and checkpoint-pressure guidance.

Uploaded chunks are staged in `tmp/`, hashed while streaming, and hard-linked into the final incident path only after SHA-256 verification. New streamed uploads use the stream-scoped path. Legacy unstreamed chunks keep the older incident-level path. Stored chunk paths are relative server-controlled paths, not client-provided paths.

SQLite schema changes are tracked in a `schema_migrations` table in the configured SQLite database. PostgreSQL schema changes use a separate PostgreSQL migration path and `schema_migrations` table in the configured PostgreSQL database.

With `SAFE_BLOB_BACKEND=s3`, committed encrypted chunks use the same stored path values in SQLite, but those values are resolved to S3 object keys under `SAFE_S3_PREFIX` instead of local files under `SAFE_DATA_DIR/incidents`.
