# Logging Requirements

This document defines the required logging posture for Proofline Server. It is
documentation only. It does not change runtime logging, API behavior,
configuration behavior, storage behavior, migrations, key custody, decryption,
or deployment exposure.

Proofline logs must help maintainers diagnose startup, configuration, request,
storage, coordination, email, deletion, and operator workflows without exposing
secrets, user safety data, private deployment details, or evidence contents.

## Summary

Proofline Server logs must be structured, low-cardinality, and safe by default.
Logs should identify the component, operation, stage, route class, and stable
error category where that information is useful and safe. Logs must not include
raw request data, uploaded bytes, plaintext, raw keys, raw tokens,
Authorization headers, TOTP codes, TOTP seeds, `otpauth_url` values, object
keys, stored paths, private filesystem paths, database DSNs, SMTP credentials,
secret file paths, secret file contents, wrapped-key ciphertext, private
deployment details, or user safety data.

Raw `err.Error()` is forbidden by default in startup, request, upload, storage,
token, key, auth, object-store, email, and user-safety paths. A log may include
only a controlled category and a reviewed safe detail string unless the local
code proves the raw error type cannot contain sensitive data.

## Goals

- Make startup and operator failures diagnosable without exposing secrets.
- Use stable structured fields so logs can be searched and reviewed.
- Keep log field values low-cardinality and safe for metrics backends.
- Preserve the current redaction posture for tokens, paths, object keys,
  request bodies, uploaded bytes, plaintext, raw keys, TOTP credential
  material, wrapped-key ciphertext, and private deployment details.
- Give future code reviews a concrete checklist for logging changes.
- Require tests when code changes alter logging behavior.

## Non-goals

- No third-party logging dependency.
- No broad request-body logging.
- No uploaded byte, plaintext, media, key, token, TOTP credential material, or
  wrapped-key logging.
- No path, object-key, private endpoint, DSN, or secret-file-path logging.
- No public production-readiness claim.
- No observability backend, metrics system, tracing system, log shipper, or
  deployment automation requirement.
- No change to API behavior, auth/session behavior, upload semantics, storage
  semantics, migrations, key custody, browser decryption, backend decryption,
  or emergency-access behavior.

## Standard Structured Log Fields

Use structured fields from this table for new or changed logs. Values must be
stable and low-cardinality unless the table says otherwise.

| Field | Scope | Requirement |
|---|---|---|
| `component` | Startup, worker, operator, background, request-adjacent logs | Required for new non-request logs when the component is not obvious from the logger. Use values such as `startup`, `httpapi`, `storage`, `coordination`, `retention`, `operator`, or `email`. |
| `operation` | Errors and meaningful state transitions | Required for internal errors and useful for workers/operators. Use controlled operation names, not raw route paths or user input. |
| `startup_stage` | Startup only | Required for startup failures when the failing stage is known. |
| `listener` | HTTP server startup/listen logs only | Optional. Use safe values such as `main_api_viewer` or `private_admin`, not private bind addresses unless the deployment has explicitly accepted address logging. |
| `route_class` | Request, rate-limit, and route-class logs | Required for rate-limit failures and useful for request summaries. Use safe route classes such as `auth`, `account`, `incident_read`, `incident_write`, `upload`, `stream`, `token`, `download`, `public_viewer`, or `static`. |
| `error_category` | Errors | Required for logged errors. Must come from the finite taxonomy below. |
| `safe_error_detail` | Errors | Optional. Must be a controlled phrase that is safe by review, never raw `err.Error()` by default. |
| `config_key` | Safe non-secret config errors | Optional. Only log allowlisted non-secret key names. |
| `config_key_class` | Secret or secret-adjacent config errors | Required instead of `config_key` when the key is secret-bearing or the key name should not be exposed. Use values such as `secret_config`, `secret_file_config`, or `private_endpoint_config`. |
| `backend` | Backend selector summaries | Optional. Safe only for implemented selector values such as `sqlite`, `postgresql`, `local`, `s3`, `none`, `valkey`, `redis`, or `smtp`. |
| `status` | Worker/operator summaries | Optional. Use controlled values such as `completed`, `failed`, `skipped`, `dry_run`, or `disabled`. |
| `eligible`, `removed`, `failed`, `skipped`, `scanned`, `processed`, `completed` | Worker/operator counts | Optional. Counts are safe when they do not identify users, incidents, object keys, paths, tokens, or private deployment details. |
| `duration_ms` | Request summaries and bounded operations | Optional. Safe numeric duration only. |
| `bytes` | Request response summaries | Optional. Response byte count only. Do not log request body size when it could identify uploaded evidence or sensitive input. |

Do not introduce ad hoc fields that duplicate the same concept under different
names. If a new recurring field is needed, add it to this document first or in
the same change.

## Startup Logging Requirements

Startup logs should identify the safe failing stage, a stable error category,
and a safe detail when one is available. They must not include raw config
values, secret file paths, database DSNs, object-store credentials, private
filesystem paths, object keys, SMTP credentials, raw tokens, plaintext, raw
keys, or private deployment details.

Standard startup stages:

| `startup_stage` | Meaning |
|---|---|
| `args_parse` | CLI argument or command selection parsing. |
| `config_load` | TOML and environment configuration loading. |
| `config_validate` | Configuration validation after values are loaded. |
| `coordination_init` | Optional Valkey/Redis-compatible client construction. |
| `coordination_check` | Optional coordination startup health check. |
| `metadata_open` | SQLite or PostgreSQL metadata repository open/check. |
| `auth_bootstrap_check` | Admin account/bootstrap-secret startup gate. |
| `blob_store_open` | Local or S3-compatible blob storage initialization/check. |
| `temp_upload_cleanup` | Optional startup cleanup of old local temp uploads. |
| `email_sender_init` | SMTP sender construction or email backend setup. |
| `http_server_config` | Main/private-admin server construction. |
| `http_listen` | Listening on configured HTTP server sockets. |
| `shutdown` | Graceful shutdown after signal or server error. |

Useful startup failure example:

```text
component=startup startup_stage=config_load error_category=unsupported_backend config_key=SAFE_METADATA_BACKEND safe_error_detail="unsupported backend; supported values: sqlite, postgresql"
```

Secret-related startup failure example:

```text
component=startup startup_stage=config_load error_category=secret_file_config config_key_class=secret_file_config safe_error_detail="secret file cannot be read"
```

The second example does not include the secret-bearing key name, secret file
path, file contents, or underlying raw filesystem error.

## Configuration Error Logging Requirements

Configuration errors should be helpful only where the detail is safe.

Allowed for non-secret enum selectors:

- safe config key names such as `SAFE_METADATA_BACKEND`, `SAFE_BLOB_BACKEND`,
  `SAFE_COORDINATION_BACKEND`, `SAFE_EMAIL_BACKEND`, and
  `SAFE_ACCOUNT_REGISTRATION_MODE`
- supported values for finite public selectors
- safe backend values after validation, such as `sqlite`, `postgresql`,
  `local`, `s3`, `none`, `valkey`, `redis`, or `smtp`

Forbidden for configuration logs:

- raw configured values for DSNs, secrets, addresses, paths, object-store
  prefixes, private endpoints, SMTP credentials, or CORS origins
- secret file paths
- secret file contents
- private filesystem paths
- raw TOML snippets when the failed setting may contain a secret or private
  deployment detail
- raw parser messages if they quote secret-adjacent input

For secret-bearing or private-deployment configuration, prefer
`config_key_class` and `error_category` over `config_key`.

## Request and Handler Logging Requirements

Request logs must remain metadata-only. They may include:

- method
- redacted route pattern
- route class
- status code
- response byte count
- duration

Request logs must not include:

- raw URL paths for token-bearing viewer routes such as `/i/{token}` or legacy
  `/e/{token}` paths
- query strings unless the route is reviewed as query-safe
- request bodies
- uploaded bytes
- Authorization headers
- cookies or session identifiers
- raw viewer, incident, session, verification, CSRF, or idempotency tokens
- TOTP codes, TOTP seeds, or `otpauth_url` values
- usernames, emails, notes, original filenames, location values, or user safety
  narratives
- full GPS, speed, heading, route history, or location freshness values
- plaintext, raw keys, raw media keys, wrapped-key ciphertext, browser fragment
  secrets, stored paths, staging paths, or object keys

Handler error logs should use `operation` and `error_category`. They should not
include raw errors by default. Panic recovery logs may include only a safe panic
type or category, not the panic value.

Template render failures must not log raw template errors unless the specific
error type and rendered context are proven safe. Prefer `operation` and
`error_category`.

## Operator Command Logging Requirements

Operator command output and logs must be safe for local review and support
handoff. Operator output may include safe IDs and controlled state only when the
specific command is documented to return them. Logs should go to stderr when the
operator command writes machine-readable JSON to stdout.

Operator summaries should prefer:

- `component=operator`
- `operation`
- `backend`
- `status`
- safe counts such as `candidate_count`, `runnable_job_count`, `failed`, or
  `skipped`
- controlled error codes or categories

Operator logs and JSON output must not include stored paths, object keys,
private filesystem paths, raw tokens, token hashes unless explicitly required
and reviewed, request bodies, uploaded bytes, plaintext, raw keys,
wrapped-key ciphertext, original filenames, notes, location values, private
endpoints, or private deployment details.

## Background Worker Logging Requirements

Background worker logs should use count summaries and stable status fields.
They must not log row contents, stored paths, object keys, incident notes,
location values, original filenames, token hashes, private endpoint details, or
raw backend errors.

Deletion and retention worker logs may include:

- `component=retention`
- `operation`
- `status`
- `error_category`
- `retention_queued`
- `token_metadata_pruned`
- `tombstones_pruned`
- `processed`
- `completed`
- `failed`

Workers should log only when there is useful action, a reviewed state
transition, or a safe error summary. Noisy success logs with zero counts should
be avoided unless needed for an operator command or a one-shot maintenance run.

## Rate-limit, Auth, Token, Upload, Deletion, and Storage Logging Rules

Rate-limit logs:

- use `route_class`, `operation`, and `error_category`
- use server-controlled class names and safe peer-identity hashes only if a
  deployment has reviewed that signal
- never include raw IP addresses in public artifacts unless explicitly
  deployment-reviewed
- never include token-bearing route paths, raw tokens, Authorization headers,
  emails, usernames, request bodies, or uploaded bytes

Auth and token logs:

- never log raw session tokens, viewer tokens, incident tokens, verification
  tokens, CSRF tokens, idempotency keys, bearer tokens, or cookies
- never log TOTP codes, TOTP seeds, or `otpauth_url` values
- do not log password input, password hashes, reset material, verification
  credentials, or browser fragment secrets
- use collapsed categories for invalid, expired, or revoked public-link tokens
  so token state is not leaked

Upload logs:

- never log uploaded bytes, plaintext, raw keys, original request bodies,
  multipart part contents, temp file paths, stored paths, object keys, or
  original filenames
- may log safe route class, status, duration, and response byte count
- may log safe error categories such as `too_large`, `hash_mismatch`,
  `duplicate`, `idempotency_conflict`, `incident_closed`, or
  `rate_limit_unavailable`

Deletion and retention logs:

- use safe counts and controlled deletion error codes
- never log stored paths, object keys, private filesystem paths, notes,
  location values, original filenames, raw tokens, uploaded bytes, plaintext,
  raw keys, or backend error strings

Storage and object-store logs:

- never log local filesystem paths, staging paths, stored paths, S3 object keys,
  bucket URLs, access keys, secret keys, session tokens, object-store request
  IDs when they could identify private deployment details, or raw storage
  backend errors
- use safe categories such as `storage`, `filesystem`, `permission`,
  `not_found`, `already_exists`, `unsafe_path`, `timeout`, or
  `dependency_unavailable`

Email logs:

- never log SMTP passwords, private mail hostnames unless deployment-reviewed,
  recipient addresses, verification tokens, message bodies, or raw SMTP errors
  that may quote private endpoint details
- use safe categories such as `email`, `config`, `network`, `timeout`, or
  `dependency_unavailable`

## Error Category Taxonomy

Use these finite, low-cardinality `error_category` values for new or changed
logs:

| Category | Use |
|---|---|
| `config` | General safe configuration failure. |
| `unsupported_backend` | Unsupported metadata, blob, coordination, email, or other enum selector. |
| `missing_required_config` | Required setting is missing. |
| `invalid_config_value` | Non-secret setting is malformed or rejected. |
| `secret_config` | Secret-bearing config is invalid without exposing key/value detail. |
| `secret_file_config` | Secret file config failed without exposing file path or contents. |
| `permission` | Permission denied without exposing path or identity detail. |
| `filesystem` | Filesystem error without exposing path. |
| `network` | Network failure without private endpoint detail. |
| `timeout` | Timeout or deadline exceeded. |
| `dependency_unavailable` | External dependency unavailable. |
| `coordination_unavailable` | Valkey/Redis-compatible coordination unavailable. |
| `storage` | Blob storage failure without path/object detail. |
| `metadata` | SQLite/PostgreSQL metadata failure without DSN/query/raw row detail. |
| `email` | Email sender failure without SMTP private detail. |
| `auth_bootstrap_required` | Startup gate requires an admin account/bootstrap setup. |
| `rate_limit_unavailable` | Rate limiter failed closed or unavailable. |
| `too_large` | Request or upload exceeded a configured size limit. |
| `unsafe_path` | Rejected unsafe server-controlled path segment or stored path. |
| `duplicate` | Duplicate entity or chunk conflict. |
| `idempotency_conflict` | Idempotency key reused with a conflicting fingerprint. |
| `incident_closed` | Write rejected because the incident is closed. |
| `invalid_state` | State transition rejected. |
| `not_found` | Missing resource where it is safe to categorize as missing. |
| `already_exists` | Existing destination or resource. |
| `shutdown` | Graceful or failed shutdown. |
| `canceled` | Context canceled. |
| `unknown` | Fallback only when no safer category is known. |

Avoid categories that include user input, route IDs, incident IDs, stream IDs,
account IDs, object keys, paths, backend hostnames, or raw error strings.

## Safe Detail Taxonomy

`safe_error_detail` is optional and must be a controlled string. It should
explain only the class of problem and a safe next step.

Allowed detail patterns:

- `unsupported backend; supported values: sqlite, postgresql`
- `unsupported backend; supported values: local, s3`
- `unsupported backend; supported values: none, valkey, redis`
- `admin account required before serving authenticated routes`
- `secret file cannot be read`
- `secret file is empty`
- `direct secret and secret file are both configured`
- `route class not configured`
- `dependency startup check failed`
- `operation timed out`

Forbidden detail patterns:

- raw `err.Error()` by default
- raw parser output that quotes secret-adjacent values
- any filesystem path, object key, bucket URL, database DSN, private endpoint,
  SMTP address, access key, session token, wrapped-key ciphertext, request body,
  uploaded bytes, plaintext, raw key, note, location value, or user safety data

## Sensitive Data That Must Never Be Logged

Never log:

- raw viewer tokens
- raw incident tokens
- raw session tokens
- raw verification tokens
- raw CSRF tokens
- raw idempotency keys
- TOTP codes
- TOTP seeds
- `otpauth_url` values
- Authorization headers
- cookies
- request bodies
- uploaded bytes
- plaintext
- raw keys, raw media keys, contact private keys, or unwrapped secrets
- wrapped-key ciphertext
- browser fragment secrets
- passwords or password hashes
- bootstrap secrets
- SMTP credentials
- database DSNs
- object-store access keys, secret keys, or session tokens
- stored paths
- staging paths
- object keys
- private filesystem paths
- private hostnames, private endpoints, or private deployment topology
- user safety narratives, notes, precise location values, route history, or
  evidence contents
- exploit payloads or sensitive vulnerability reproduction details

Synthetic examples in docs and tests must also avoid real-looking secrets,
token values, private hostnames, private paths, or user safety data.

## Filesystem Path, Object Key, and Private Deployment Detail Redaction Rules

Filesystem paths and object keys are private deployment details. Logs should
report a category, stage, component, and safe count instead of the path or key.

Allowed:

```text
component=startup startup_stage=temp_upload_cleanup error_category=filesystem safe_error_detail="temp cleanup failed"
```

Forbidden:

```text
error="<private filesystem path> cannot be removed"
object_key="<object key>"
```

If operators need path-level diagnostics, handle that through a private,
explicit operator workflow outside public logs and public issue text.

## When Raw `err.Error()` Is Forbidden

Raw `err.Error()` is forbidden by default for:

- startup errors
- configuration loading and validation
- request and handler errors
- panic recovery
- auth/session/account/registration flows
- token creation, lookup, revocation, and verification flows
- upload, temp file, storage, object-store, stream, and bundle paths
- idempotency and coordination paths
- wrapped-key, contact-key, sharing-grant, key-custody, or decryption-adjacent
  paths
- email sender paths
- deletion, retention, backup, and restore paths
- any path that may touch user safety data

Raw error strings may be logged only when all of these are true:

1. The error type is known and documented locally as safe.
2. The error string cannot contain secrets, paths, object keys, request data,
   plaintext, raw keys, token values, private deployment details, user safety
   data, or arbitrary user input.
3. A test covers the positive useful field and the negative redaction case.
4. The code review explicitly calls out why raw error logging is safe.

Prefer controlled `error_category` and `safe_error_detail` fields.

## When Config Key Names May Be Logged

Config key names may be logged only when the name is useful and safe by itself.

Generally safe:

- backend selector keys such as `SAFE_METADATA_BACKEND`, `SAFE_BLOB_BACKEND`,
  and `SAFE_COORDINATION_BACKEND`
- finite mode selector keys such as `SAFE_ACCOUNT_REGISTRATION_MODE`
- non-secret duration, limit, or boolean keys after the value has been
  validated and the value itself is not logged

Generally not safe:

- direct secret keys
- secret file keys
- DSN keys
- credential keys
- private endpoint keys
- keys whose value commonly appears in support screenshots or public issue text
  with the key name and value together

For unsafe or secret-adjacent keys, use `config_key_class` with values such as
`secret_config`, `secret_file_config`, `dsn_config`, or
`private_endpoint_config`.

## When Supported Values May Be Logged

Supported values may be logged when the value set is finite, public,
non-secret, and low-cardinality.

Allowed examples:

- `sqlite, postgresql`
- `local, s3`
- `none, valkey, redis`
- `disabled, admin_only, open, paid`

Do not log supported values when the values are deployment-specific or private,
such as configured hostnames, origins, bucket names, prefixes, paths, usernames,
email addresses, recipient lists, credentials, or per-tenant identifiers.

## Test Requirements

When code changes affect logging behavior, tests must cover both usefulness and
redaction.

Required test patterns for future implementation work:

- startup logs include safe `startup_stage` and `error_category` fields
- non-secret config errors include safe `config_key` and supported values where
  appropriate
- secret-related config errors do not expose raw key names when the key name is
  sensitive, raw secret values, secret file contents, or private paths
- filesystem errors do not expose private paths
- object-store and storage errors do not expose object keys, bucket URLs,
  credentials, private endpoints, or raw backend errors
- request logs do not expose Authorization headers, request bodies, uploaded
  bytes, raw tokens, plaintext, raw keys, wrapped-key ciphertext, or query
  strings on token-bearing paths
- viewer token paths are logged only as redacted route patterns
- panic recovery logs do not expose panic values
- worker/operator logs include useful safe counts without exposing sensitive row
  data, stored paths, object keys, notes, location values, original filenames,
  or raw backend errors
- tests use synthetic values that do not look like real secrets or real private
  deployment details

For Go logging implementation changes, run:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
go vet ./...
git diff --check
```

For documentation-only logging policy changes, run:

```bash
git diff --check
git diff --stat
git diff -- README.md AGENTS.md SECURITY.md CHANGELOG.md docs codex
```

## Review Checklist

For every logging change, reviewers should ask:

- Does the log have a safe component, operation, stage, route class, or status
  where useful?
- Is `error_category` stable, finite, and low-cardinality?
- Is `safe_error_detail` controlled and reviewed?
- Is raw `err.Error()` avoided in sensitive paths?
- Are token-bearing viewer routes redacted?
- Are request bodies, uploaded bytes, Authorization headers, cookies, tokens,
  plaintext, raw keys, and wrapped-key ciphertext excluded?
- Are filesystem paths, object keys, bucket URLs, DSNs, private endpoints, and
  private deployment details excluded?
- Are worker/operator logs limited to safe counts, controlled statuses, and
  controlled error codes?
- Are tests covering both useful fields and redaction?
- Does the change preserve main `/v1`/viewer and private-admin listener
  separation?
- Does the change avoid implying production readiness?
- Does the change avoid introducing key custody, browser decryption, backend
  decryption, key escrow, notifications, recording/capture, billing, or sibling
  repository behavior?

## Open Questions / Future Work

- Whether startup logging should add more reviewed safe details per stage beyond
  the current typed startup-stage wrapper.
- Whether request summary logs should add a `route_class` field everywhere.
  Rate-limit dependency-failure logs already include safe route classes.
- Whether private operator diagnostics are needed for bind-address or
  path-level troubleshooting without putting those details in ordinary logs.
- Whether template render logs need more specific categories beyond the current
  raw-error redaction.
- Whether SMTP sender failures need a dedicated safe error wrapper before email
  verification is used in broader deployments.
- Whether future metrics/tracing should share this taxonomy or have a stricter
  schema.
