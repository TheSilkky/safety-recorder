# One-off Codex Work Order: Add TOML Config Provider And Secret File Support

Historical/reference-only prompt after completion.

This is a one-off implementation work order for `open-proofline/server`.

## Goal

Add a recommended TOML configuration file provider and secret-file support for Proofline Server.

The current environment-variable-only configuration surface has become crowded. This work should make a root `proofline.toml` file the recommended default configuration path while preserving existing `SAFE_*` environment variables as overrides and compatibility inputs.

This work must include the local `compose/` smoke-test directory and update smoke-test configuration so TOML config loading and secret-file support are covered by the existing disposable Docker Compose validation paths.

## Source of truth

Before editing, read the current versions of:

* `AGENTS.md`
* `README.md`
* `SECURITY.md`
* `CHANGELOG.md`
* `docs/README.md`
* `docs/api.md`
* `docs/architecture.md`
* `docs/configuration.md`
* `docs/deployment.md`
* `docs/development.md`
* `docs/code-map.md`
* `docs/codex-change-control.md`
* `docs/production-cluster-scope.md`
* `docs/cluster-safe-upload-semantics.md`
* `docs/cluster-backup-restore-runbook.md`
* `codex/README.md`
* `codex/prompts/00-project-context-check.md`
* `codex/prompts/05-codex-change-control.md`
* `codex/prompts/20-code-review.md`
* `codex/prompts/30-security-review.md`
* `codex/prompts/40-documentation-update.md`
* `codex/prompts/70-work-on-github-issue.md`
* `codex/prompts/75-create-draft-pr-from-current-branch.md`
* `codex/prompts/76-request-codex-pr-review.md`
* `compose/`
* `.github/workflows/`

Relevant code areas likely include:

* `cmd/api`
* `internal/config`
* config tests
* Compose smoke scripts/configs
* Docker/entrypoint docs, if present
* deployment docs

Do not rely on stale assumptions from this prompt if current repository files disagree.

## Required prompt stack

Before implementation, apply:

* `codex/prompts/00-project-context-check.md`
* `codex/prompts/05-codex-change-control.md`

After implementation, apply:

* `codex/prompts/20-code-review.md`
* `codex/prompts/30-security-review.md`
* `codex/prompts/40-documentation-update.md`
* `codex/prompts/75-create-draft-pr-from-current-branch.md`
* `codex/prompts/76-request-codex-pr-review.md`

## Scope

Allowed:

* Add TOML config file loading.
* Add a root default example config file: `proofline.toml`.
* Add config-file path selection through environment and/or CLI.
* Add generic secret-file support.
* Update configuration docs to make TOML the recommended default.
* Preserve existing `SAFE_*` environment-variable configuration.
* Preserve local-first defaults.
* Add tests for config precedence, TOML parsing, env overrides, and secret files.
* Update `compose/` smoke-test configs and scripts to exercise TOML config and secret files.
* Update Compose smoke documentation.
* Update changelog and code-map docs.

Not allowed:

* Do not remove existing `SAFE_*` environment variables.
* Do not force TOML use for all deployments.
* Do not put real secrets into `proofline.toml`.
* Do not commit secret files containing real credentials.
* Do not log secret values.
* Do not expose private deployment details.
* Do not change listener security boundaries.
* Do not expose `/admin` publicly.
* Do not make `/v1/admin/...` public-ready.
* Do not add backend decryption.
* Do not add browser decryption.
* Do not add raw server-held media keys.
* Do not add key escrow.
* Do not add break-glass access.
* Do not add playable export.
* Do not add recording/capture behavior.
* Do not add OAuth/JWT.
* Do not add payment provider integration.
* Do not add emergency-services integration.
* Do not add cloud-provider deployment automation.

## Configuration precedence

Implement and document this precedence:

```text
built-in defaults
  < TOML config file
  < environment variables
  < *_FILE environment variables for secret-capable fields
  < CLI flags, if CLI flags are added or already exist
```

If the current code does not use CLI flags, do not add a broad CLI flag system. It is acceptable to add only:

```text
--config /path/to/proofline.toml
```

if that fits the repository style.

Also support an environment variable:

```text
SAFE_CONFIG_FILE=/path/to/proofline.toml
```

Recommended path resolution:

1. explicit `--config`, if implemented
2. `SAFE_CONFIG_FILE`
3. `./proofline.toml` if present
4. `/etc/proofline/proofline.toml` if present
5. built-in defaults and env only

If automatic discovery of `/etc/proofline/proofline.toml` is too surprising for tests or local dev, make discovery explicit and document the decision. Do not silently ignore invalid config files when a config path is explicitly provided.

## TOML format

Use TOML, not YAML.

Add a root `proofline.toml` file with safe local-first defaults and comments.

The root file must not contain real secrets.

Recommended top-level sections:

```toml
[server]
main_bind_addrs = ["127.0.0.1:8080"]
admin_bind_addrs = ["127.0.0.1:8081"]

[paths]
data_dir = "./data"
sqlite_db_path = "./data/proofline.db"

[metadata]
backend = "sqlite"
postgres_dsn_file = ""

[blob_storage]
backend = "local"
s3_endpoint = ""
s3_region = "us-east-1"
s3_bucket = ""
s3_prefix = ""
s3_access_key_id_file = ""
s3_secret_access_key_file = ""
s3_session_token_file = ""
s3_force_path_style = true

[coordination]
backend = "none"
valkey_addr = ""
valkey_username = ""
valkey_password_file = ""
valkey_db = 0
valkey_tls = false
valkey_dial_timeout = "5s"
valkey_read_timeout = "5s"
valkey_write_timeout = "5s"

[uploads]
max_upload_bytes = "250MB"
upload_coordination_lease_ttl = "2m"
temp_upload_cleanup_age = "0"
temp_upload_cleanup_dry_run = false

[auth]
session_ttl = "12h"
bootstrap_secret_file = ""

[retention]
default_incident_token_ttl = "24h"
closed_incident_retention = "0"
token_metadata_retention = "0"
deletion_tombstone_retention = "0"
deletion_worker_interval = "1m"

[rate_limits.main_api]
enabled = true
window = "1m"
auth = 30
bootstrap = 5
account = 120
incident_read = 300
incident_write = 120
upload = 120
reconcile = 120
stream = 120
token = 60
download = 30
admin = 60

[rate_limits.public_viewer]
enabled = true
window = "1m"
page = 60
data = 300
download = 12
static = 600

[http.main]
read_header_timeout = "10s"
read_timeout = "0s"
write_timeout = "0s"
idle_timeout = "120s"

[http.admin]
read_header_timeout = "10s"
read_timeout = "30s"
write_timeout = "300s"
idle_timeout = "120s"
```

If registration/email config already exists in the branch by the time this runs, include safe sections for it too:

```toml
[account_registration]
mode = "disabled"
email_verification_required = true
email_verification_ttl = "24h"

[email]
backend = "none"
smtp_host = ""
smtp_port = 587
smtp_username = ""
smtp_password_file = ""
smtp_from = ""
smtp_starttls = "required"
smtp_timeout = "10s"
```

If browser cookie auth already exists in the branch by the time this runs, include safe sections for it too.

## Naming rules

TOML names should be human-readable and grouped by function.

Do not make TOML keys blindly mirror every `SAFE_*` environment variable if a clearer table/key name is available.

However, documentation must map TOML keys to existing env vars.

Example:

```text
TOML: [metadata].backend
Env:  SAFE_METADATA_BACKEND
```

Existing env vars remain the stable container/platform override names.

## Secret-file support

Add generic secret-file support immediately.

Do not implement TOML config without secret-file support.

For secret-capable settings, support direct env value and file path variants, but avoid direct secret values in TOML examples.

Secret-capable fields include at least:

* `SAFE_AUTH_BOOTSTRAP_SECRET`
* `SAFE_POSTGRES_DSN`
* `SAFE_S3_ACCESS_KEY_ID`
* `SAFE_S3_SECRET_ACCESS_KEY`
* `SAFE_S3_SESSION_TOKEN`
* `SAFE_VALKEY_PASSWORD`

If registration/email config exists, include:

* `SAFE_SMTP_PASSWORD`

If browser-cookie auth exists, include:

* cookie/session signing or CSRF secrets if any were added

If future billing placeholders exist, include only placeholder docs for future:

* `SAFE_BILLING_SERVICE_TOKEN`
* future payment provider API/webhook secrets

Add matching `_FILE` environment variables, for example:

```text
SAFE_AUTH_BOOTSTRAP_SECRET_FILE
SAFE_POSTGRES_DSN_FILE
SAFE_S3_ACCESS_KEY_ID_FILE
SAFE_S3_SECRET_ACCESS_KEY_FILE
SAFE_S3_SESSION_TOKEN_FILE
SAFE_VALKEY_PASSWORD_FILE
SAFE_SMTP_PASSWORD_FILE
```

Rules:

* Read secret files once at startup.
* Missing secret file fails startup.
* Empty secret file fails startup unless a specific field explicitly allows empty values.
* Trim a single trailing newline or CRLF for Docker/Kubernetes/Nomad/systemd secret compatibility.
* Do not trim meaningful internal whitespace.
* Direct secret and file path must not both be set at the same precedence layer.
* `_FILE` env vars should override direct env vars for the same field only if this precedence is explicitly documented and tested.
* Secret values must never be logged.
* Secret file contents must never be logged.
* Prefer not to log secret file paths in public/user-facing errors because paths can reveal deployment details.
* Error messages should name the config field, not print the secret value or secret path.

Add a small internal abstraction rather than ad hoc file-reading per field.

Suggested shape:

```go
type SecretValue struct {
    Value string
    File  string
}
```

or another repository-consistent shape.

Add a resolver similar to:

```go
ResolveSecret(name string, secret SecretValue) (string, error)
```

The resolver must be tested independently.

## Backward compatibility

All existing documented env-only deployments must keep working.

Existing examples like:

```bash
SAFE_METADATA_BACKEND=postgresql \
SAFE_POSTGRES_DSN='postgres://...' \
go run ./cmd/api
```

must remain supported.

Existing Compose smoke stacks must keep working after they are migrated or supplemented with TOML config.

Existing local defaults must keep working if no config file is present.

If `./proofline.toml` is committed at the repository root, local `go run ./cmd/api` from the repository root will now load it. Ensure the file matches current local defaults so this is not surprising.

## Config validation

Validation must apply after all providers are merged.

Validation must catch:

* invalid TOML syntax
* unknown/unsupported enum values
* invalid duration strings
* invalid byte-size strings
* invalid bind address values if currently validated
* invalid backend combinations
* required fields missing after secret-file resolution
* conflicting direct secret and secret-file values
* missing/empty secret files
* unsafe S3/plain HTTP guidance if currently enforced
* legacy env var behavior remains consistent

Unknown TOML keys:

* Prefer failing startup on unknown TOML keys to catch typos.
* If the TOML decoder does not support unknown-key rejection easily, document the limitation and add a follow-up issue.
* Do not silently accept misspelled critical config keys without at least a test or documented limitation.

## Logging

Startup logs may show a safe summary, such as selected backend names and listener counts.

Startup logs must not include:

* PostgreSQL DSN
* S3 access key
* S3 secret key
* S3 session token
* Valkey password
* SMTP password
* bootstrap secret
* cookie/session secrets
* billing tokens
* secret file contents
* Authorization headers
* raw tokens
* request bodies
* uploaded bytes
* stored paths
* object keys
* private deployment details
* user safety data

Avoid logging full secret file paths where practical.

## Compose smoke-test requirements

Include the `compose/` smoke-test directory in this work.

Inspect the current `compose/` layout and documented smoke runner before editing.

Update or add Compose smoke coverage so at least one disposable stack uses TOML config as the primary configuration source.

Preferred coverage:

* SQLite/local stack uses `proofline.toml` or a compose-specific TOML file.
* PostgreSQL/local stack uses TOML config plus secret file for PostgreSQL DSN.
* SQLite/S3-compatible MinIO stack uses TOML config plus secret files for S3 credentials.
* Full PostgreSQL/MinIO/Valkey stack uses TOML config plus secret files for PostgreSQL DSN, S3 credentials, and Valkey password where applicable.

If updating every smoke stack is too large for one PR, update at least the full PostgreSQL/MinIO/Valkey smoke stack and create explicit follow-up issues for the rest.

Do not commit real secrets.

Use clearly fake smoke-test secret files under a test-only path if needed, for example:

```text
compose/secrets.example/
compose/smoke/secrets/
```

If committing fake secret files, make names and docs unmistakable:

```text
DO_NOT_USE_IN_PRODUCTION
example-only
local-smoke-only
```

Ensure `.gitignore` prevents accidental real secret files from being committed.

Run `docker compose config` or the repository's documented equivalent for touched stacks.

Run the full local Compose smoke path when Docker is available.

Do not run Compose smoke against production services.

## Documentation updates

Update:

* `CHANGELOG.md`
* `README.md`
* `docs/configuration.md`
* `docs/deployment.md`
* `docs/development.md`
* `docs/code-map.md`
* `docs/security-model.md`, if secret handling changes warrant it
* `docs/threat-model.md`, if configuration/secret-file risk changes warrant it
* `compose/` README/docs
* `.env.example`, if present
* `.gitignore`, if needed

Docs must explain:

* TOML config is the recommended default for deployments.
* Existing `SAFE_*` environment variables remain supported.
* Env vars override TOML config.
* Secret files are preferred for secret-bearing values.
* `_FILE` env vars are supported for secret-bearing values.
* The root `proofline.toml` is safe local-first example config.
* Do not put real secrets into committed config files.
* Do not publish real config files containing private endpoints or secret paths.
* How to run with a custom config file.
* How Compose smoke tests exercise TOML and secret files.

## Tests

Add focused unit tests for:

* no config file uses built-in defaults
* root/default TOML local config parses successfully
* explicit config file path parses successfully
* malformed TOML fails startup/config loading
* TOML values override built-in defaults
* env vars override TOML values
* `_FILE` env vars resolve secret values
* TOML `*_file` secret values resolve secret values
* missing secret file fails
* empty secret file fails where required
* direct secret and secret file conflict fails
* one trailing newline and CRLF are trimmed from secret files
* internal whitespace is preserved
* secret values are not included in error strings
* secret file contents are not logged
* legacy env aliases still behave as documented
* unsafe legacy public bind variables still fail as currently documented
* PostgreSQL/S3/Valkey required secret-bearing values can be supplied via files

If practical, add tests for unknown TOML keys failing.

## Dependencies

Use a small, maintained TOML parser appropriate for Go.

Do not add YAML.

Do not add a broad configuration framework unless clearly justified.

Keep the dependency surface boring.

If a new dependency is added, update dependency/security docs only if the repository normally does that.

## Suggested implementation phases

Prefer small commits in this order:

1. Add TOML config structs/parser and tests.
2. Add generic secret-file resolver and tests.
3. Merge config providers with precedence and env compatibility.
4. Add root `proofline.toml`.
5. Update Compose smoke configs/secrets to exercise TOML and secret files.
6. Update docs and changelog.
7. Run full validation and smoke tests.

If this becomes too large for one reviewable PR, stop after TOML+secret resolver+docs and create follow-up issues for full Compose migration. Do not leave Compose entirely untouched in this PR; at minimum add one smoke stack using TOML and secret files.

## Validation

Run from the server repo:

```bash
gofmt -w ./cmd ./internal ./migrations
go test ./...
go vet ./...
git diff --check
```

Because this touches configuration providers, Compose, deployment config, coordination/storage-related configuration, and secret handling, run the documented local Compose smoke tests when Docker is available.

At minimum:

```bash
find compose -maxdepth 3 -type f -print
find compose -maxdepth 3 -type f -perm -111 -print
```

Then inspect and run the repository's canonical smoke runner.

Also run `docker compose config` or documented equivalents for all touched Compose stacks.

If Docker is unavailable, stop before merging if Compose smoke is required and report that validation could not be completed.

Do not use real deployment credentials.

## Pull request

Open a PR against `develop`.

Suggested PR title:

```text
config: add TOML provider and secret file support
```

PR body must include:

```md
## Summary

- Added TOML config-file support with `proofline.toml` as the recommended local/default config shape.
- Preserved existing `SAFE_*` environment variable configuration as overrides.
- Added generic secret-file support for secret-bearing settings.
- Updated Compose smoke configuration to exercise TOML config and secret files.
- Updated configuration, deployment, development, security/code-map docs, and changelog.

## Validation

- [ ] `gofmt -w ./cmd ./internal ./migrations`
- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `git diff --check`
- [ ] `docker compose config` or documented equivalent for touched Compose stacks
- [ ] full Compose smoke tests, if Docker is available

## Security and scope

- Existing env-only deployments remain supported.
- Env vars override TOML values.
- Secret files are supported for secret-bearing values.
- Real secrets are not committed.
- Secret values and secret file contents are not logged.
- Secret-bearing fields are not included in public docs/examples with real values.
- `/admin` remains private-only.
- `/v1/admin/...` remains admin-only and not public-ready.
- No backend decryption, browser decryption, raw server-held media keys, key escrow, break-glass access, playable export, recording/capture behavior, OAuth/JWT, payment provider integration, notification delivery, or emergency-services integration was added.
```

## Final output

Report:

1. files changed
2. TOML config path behavior
3. provider precedence
4. root `proofline.toml` contents summary
5. secret-file fields supported
6. direct-secret/secret-file conflict behavior
7. Compose smoke stacks updated
8. tests added
9. validation commands run
10. docs updated
11. follow-up issues needed
