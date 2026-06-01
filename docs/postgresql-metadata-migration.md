# PostgreSQL Metadata Backend Migration Path

This document records the PostgreSQL metadata backend path for Proofline
Server. PostgreSQL metadata is implemented as an optional backend for new
deployments. This does not change the current SQLite default, change blob
storage, expose `/v1` publicly, add public account workflows, or change the
backend ciphertext-only encryption posture.

SQLite metadata and local encrypted blob storage remain supported. Optional
S3-compatible encrypted blob storage is implemented separately from this
metadata backend. PostgreSQL is available as an optional metadata backend for
new deployments that need a database suitable for later multi-node work.

## Goals

- Add PostgreSQL as an optional metadata backend without removing SQLite.
- Preserve the current HTTP behavior, token hashing, route separation, and
  encrypted-bundle behavior.
- Keep schema constraints equivalent to or stronger than the current SQLite
  schema.
- Keep PostgreSQL migrations separate from the existing SQLite migration path.
- Document repository transaction boundaries for the second metadata store.
- Define parity and restore expectations before production-cluster use.

## Non-Goals

- No change to current `SAFE_METADATA_BACKEND=sqlite` behavior.
- No migration of existing deployments by default.
- No PostgreSQL requirement for local development or simulator flows.
- No changes to S3-compatible blob storage and no operation-level
  Valkey/Redis-compatible coordination behavior.
- No public `/v1` exposure, public account workflows, OAuth, JWT, public admin
  dashboard, cloud deployment automation, Docker Compose, Kubernetes, or
  Terraform.
- No backend decryption, raw server-held media keys, key escrow, browser
  decryption, or playable media export.

## Current SQLite Model

The current canonical SQLite schema is the result of the embedded SQL files in
`migrations/` plus compatibility migrations in `internal/db`. PostgreSQL should
model the canonical end state, not replay SQLite-specific compatibility DDL.

### `schema_migrations`

| Column | Current SQLite constraint | PostgreSQL expectation |
|---|---|---|
| `id` | `TEXT PRIMARY KEY` | `TEXT PRIMARY KEY` |
| `checksum` | `TEXT NOT NULL` | `TEXT NOT NULL` |
| `applied_at` | `TEXT NOT NULL` | `TIMESTAMPTZ NOT NULL`, normalized to UTC |

PostgreSQL may use the same table name inside the PostgreSQL database, but its
rows must track only PostgreSQL migrations. SQLite migration IDs and checksums
must not be copied into PostgreSQL to imply that SQLite migrations ran there.

### `incidents`

| Column | Current SQLite constraint | PostgreSQL expectation |
|---|---|---|
| `id` | `TEXT PRIMARY KEY` | `TEXT PRIMARY KEY`; keep current prefixed IDs |
| `created_at` | `TEXT NOT NULL` | `TIMESTAMPTZ NOT NULL`, normalized to UTC on write |
| `updated_at` | `TEXT NOT NULL` | `TIMESTAMPTZ NOT NULL`, normalized to UTC on write |
| `status` | `TEXT NOT NULL CHECK (status IN ('open', 'closed'))` | same status values with a `CHECK` constraint |
| `client_label` | nullable `TEXT` | nullable `TEXT` |
| `notes` | nullable `TEXT` | nullable `TEXT` |
| `owner_account_id` | nullable `TEXT REFERENCES accounts(id) ON DELETE SET NULL` | same foreign key, added by the account/session migration |

Use `CHECK` constraints rather than PostgreSQL enum types at first. That keeps
future status additions ordinary migrations and mirrors the SQLite behavior.

### `media_streams`

| Column | Current SQLite constraint | PostgreSQL expectation |
|---|---|---|
| `id` | `TEXT PRIMARY KEY` | `TEXT PRIMARY KEY`; keep current prefixed IDs |
| `incident_id` | `TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE` | same foreign key |
| `media_type` | `TEXT NOT NULL CHECK (media_type IN ('audio', 'video', 'location', 'metadata'))` | same values with a `CHECK` constraint |
| `label` | nullable `TEXT` | nullable `TEXT` |
| `status` | `TEXT NOT NULL CHECK (status IN ('open', 'complete', 'failed'))` | same status values with a `CHECK` constraint |
| `expected_chunk_count` | nullable `INTEGER CHECK (expected_chunk_count IS NULL OR expected_chunk_count > 0)` | same constraint |
| `created_at` | `TEXT NOT NULL` | `TIMESTAMPTZ NOT NULL`, normalized to UTC |
| `updated_at` | `TEXT NOT NULL` | `TIMESTAMPTZ NOT NULL`, normalized to UTC |
| `completed_at` | nullable `TEXT` | nullable `TIMESTAMPTZ` |
| `failed_at` | nullable `TEXT` | nullable `TIMESTAMPTZ` |
| `failure_reason` | nullable `TEXT` | nullable `TEXT` |

Indexes should preserve current lookup behavior:

- `media_streams(incident_id)`
- `media_streams(status)`

PostgreSQL should also add a unique key on
`(incident_id, id, media_type)` if the chunk table uses the stronger composite
foreign key described below.

### `chunks`

The chunk table is the highest-risk parity point because it preserves both new
stream-scoped chunk identity and legacy unstreamed chunk compatibility.

| Column | Current SQLite constraint | PostgreSQL expectation |
|---|---|---|
| `id` | `TEXT PRIMARY KEY` | `TEXT PRIMARY KEY`; keep current prefixed IDs |
| `incident_id` | `TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE` | same foreign key |
| `stream_id` | nullable `TEXT`, normalized so empty strings become `NULL`, checked non-empty when present | nullable `TEXT CHECK (stream_id IS NULL OR length(stream_id) > 0)` |
| `chunk_index` | `INTEGER NOT NULL CHECK (chunk_index >= 0)` | same, plus preferably `CHECK (stream_id IS NULL OR chunk_index > 0)` |
| `media_type` | `TEXT NOT NULL CHECK (media_type IN ('audio', 'video', 'location', 'metadata'))` | same values with a `CHECK` constraint |
| `started_at` | `TEXT NOT NULL` | `TIMESTAMPTZ NOT NULL`, normalized to UTC |
| `ended_at` | `TEXT NOT NULL` | `TIMESTAMPTZ NOT NULL`, normalized to UTC |
| `original_filename` | nullable `TEXT` | nullable `TEXT` |
| `stored_path` | `TEXT NOT NULL` | `TEXT NOT NULL`; still server-controlled relative paths |
| `byte_size` | `INTEGER NOT NULL CHECK (byte_size >= 0)` | `BIGINT NOT NULL CHECK (byte_size >= 0)` |
| `sha256_hex` | lowercase 64-character SHA-256 hex `CHECK` | `TEXT NOT NULL CHECK (sha256_hex ~ '^[0-9a-f]{64}$')` |
| `created_at` | `TEXT NOT NULL` | `TIMESTAMPTZ NOT NULL`, normalized to UTC |

Indexes:

- `chunks(incident_id)`
- `chunks(stream_id)`

Uniqueness:

- Legacy unstreamed chunks: unique
  `(incident_id, media_type, chunk_index) WHERE stream_id IS NULL`
- Streamed chunks: unique
  `(incident_id, stream_id, chunk_index) WHERE stream_id IS NOT NULL`

PostgreSQL should enforce the stream relationship more strongly than SQLite
when practical:

```sql
FOREIGN KEY (incident_id, stream_id, media_type)
REFERENCES media_streams(incident_id, id, media_type)
ON DELETE CASCADE
```

With PostgreSQL's default `MATCH SIMPLE` behavior, this allows legacy rows with
`stream_id IS NULL` while validating streamed rows against the owning incident
and stream media type.

The repository must still check stream state transactionally. A foreign key
proves the stream exists and matches the media type; it does not prove the
stream is still `open`.

### `checkins`

| Column | Current SQLite constraint | PostgreSQL expectation |
|---|---|---|
| `id` | `TEXT PRIMARY KEY` | `TEXT PRIMARY KEY`; keep current prefixed IDs |
| `incident_id` | `TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE` | same foreign key |
| `created_at` | `TEXT NOT NULL` | `TIMESTAMPTZ NOT NULL`, normalized to UTC |
| `device_battery_percent` | nullable `INTEGER` | nullable `INTEGER`; optionally add `0..100` only as an explicit behavior change |
| `device_network` | nullable `TEXT` | nullable `TEXT` |
| `latitude` | nullable `REAL` | nullable `DOUBLE PRECISION` |
| `longitude` | nullable `REAL` | nullable `DOUBLE PRECISION` |
| `accuracy_meters` | nullable `REAL` | nullable `DOUBLE PRECISION` |

Indexes:

- `checkins(incident_id)`

Do not add new location validation casually. The current API and tests define
what is accepted today.

### `accounts`

| Column | Current SQLite constraint | PostgreSQL expectation |
|---|---|---|
| `id` | `TEXT PRIMARY KEY` | `TEXT PRIMARY KEY`; keep current prefixed IDs |
| `username` | `TEXT NOT NULL UNIQUE` | same uniqueness; usernames are normalized by application code |
| `password_hash` | `TEXT NOT NULL` | same; stores bcrypt password hashes only |
| `role` | `TEXT NOT NULL CHECK (role IN ('user', 'admin'))` | same role values with a `CHECK` constraint |
| `created_at` | `TEXT NOT NULL` | `TIMESTAMPTZ NOT NULL`, normalized to UTC |
| `updated_at` | `TEXT NOT NULL` | `TIMESTAMPTZ NOT NULL`, normalized to UTC |
| `password_changed_at` | `TEXT NOT NULL` | `TIMESTAMPTZ NOT NULL`, normalized to UTC |

Indexes:

- `accounts(username)`
- `accounts(role)`

The raw account password must never be stored. Both metadata backends store only
the bcrypt password hash returned by the auth package.

### `auth_sessions`

| Column | Current SQLite constraint | PostgreSQL expectation |
|---|---|---|
| `id` | `TEXT PRIMARY KEY` | `TEXT PRIMARY KEY`; keep current prefixed IDs |
| `account_id` | `TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE` | same foreign key |
| `token_hash` | `TEXT NOT NULL UNIQUE`, lowercase 64-character SHA-256 hex `CHECK` | same uniqueness and `CHECK (token_hash ~ '^[0-9a-f]{64}$')` |
| `created_at` | `TEXT NOT NULL` | `TIMESTAMPTZ NOT NULL`, normalized to UTC |
| `expires_at` | `TEXT NOT NULL` | `TIMESTAMPTZ NOT NULL`, normalized to UTC |
| `revoked_at` | nullable `TEXT` | nullable `TIMESTAMPTZ` |

Indexes:

- `auth_sessions(account_id)`
- `auth_sessions(token_hash)`
- `auth_sessions(expires_at)`

The raw session token must never be stored. PostgreSQL receives only the
SHA-256 token hash, just like SQLite.

### `incident_tokens`

| Column | Current SQLite constraint | PostgreSQL expectation |
|---|---|---|
| `id` | `TEXT PRIMARY KEY` | `TEXT PRIMARY KEY`; keep current prefixed IDs |
| `incident_id` | `TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE` | same foreign key |
| `token_hash` | `TEXT NOT NULL UNIQUE`, lowercase 64-character SHA-256 hex `CHECK` | same uniqueness and `CHECK (token_hash ~ '^[0-9a-f]{64}$')` |
| `label` | nullable `TEXT` | nullable `TEXT` |
| `created_at` | `TEXT NOT NULL` | `TIMESTAMPTZ NOT NULL`, normalized to UTC |
| `expires_at` | nullable `TEXT` | nullable `TIMESTAMPTZ` |
| `revoked_at` | nullable `TEXT` | nullable `TIMESTAMPTZ` |

Indexes:

- `incident_tokens(incident_id)`
- `incident_tokens(token_hash)`

The raw incident viewer token must never be stored. PostgreSQL receives only
the hash, just like SQLite.

### Legacy `emergency_tokens`

`emergency_tokens` is not part of the canonical current schema. SQLite migration
`004_incident_tokens.sql` copies legacy token rows into `incident_tokens` and
drops the old table.

A future SQLite-to-PostgreSQL data migration must first ensure the SQLite
database has completed current migrations, then copy `incident_tokens`. It
should not recreate `emergency_tokens` in PostgreSQL.

## Migration Layout

The existing SQLite migration path must remain stable:

- keep current SQLite SQL files under `migrations/*.sql`
- keep `internal/db.Migrate` applying only the embedded SQLite files and
  SQLite compatibility migration helpers
- keep existing SQLite migration IDs and checksums unchanged

PostgreSQL uses a separate migration source and runner:

```text
migrations/postgres/*.sql
internal/postgresdb
```

The SQLite `migrations.FS` and `internal/db` compatibility migrations are not
repurposed for PostgreSQL.

PostgreSQL migration tracking:

- create `schema_migrations` inside the PostgreSQL database
- record migration ID, checksum, and applied time
- calculate checksums over the exact embedded PostgreSQL migration body
- reject checksum mismatches on already-applied migrations
- serialize migration application with a PostgreSQL advisory lock so concurrent
  API starts do not race on `schema_migrations`
- apply each migration in a transaction where PostgreSQL supports it
- keep migration IDs ordered and backend-specific, for example
  `001_init.sql`, `002_add_upload_operations.sql`, or
  `postgres/001_init.sql`
- avoid compatibility migrations that depend on SQLite pragmas, `GLOB`,
  table-rebuild behavior, or `ALTER TABLE` limitations

The first PostgreSQL migration should create the canonical current schema in
one direct shape. It should not replay the historical SQLite evolution from
`emergency_tokens` through incident-token rename and chunk stream-identity
rebuilds.

## Repository Transaction Boundaries

PostgreSQL must preserve the behavior behind the current
`httpapi.MetadataRepository` boundary. The following transaction boundaries are
the minimum design target.

### Incident Creation And Close

Create incident:

- insert one `open` incident row
- return the generated ID only after the insert succeeds

Close incident:

- update the incident row from any current status to `closed`
- use a transaction or single conditional update that serializes with chunk
  insert checks
- chunk creation must not commit metadata after the incident is closed

### Stream Creation, Completion, And Failure

Create stream:

- insert one `open` stream row with a valid incident foreign key
- preserve current behavior where missing incidents return a not-found style
  error

Complete stream:

- lock the target stream row before changing state
- move only `open` streams to `complete`
- set `expected_chunk_count`, `updated_at`, and `completed_at`
- clear `failed_at` and `failure_reason`
- validate chunk rows in the same transaction before commit
- require exactly chunks `1..expected_chunk_count`
- require every chunk row to match the stream media type
- reject missing, non-contiguous, extra, or wrong-media chunk rows

Failure:

- move only `open` streams to `failed`
- preserve uploaded chunk rows
- clear `completed_at`

PostgreSQL uses row-level locking around stream state transitions so a
chunk insert and stream completion cannot both make decisions from stale stream
state.

### Chunk Insert And Upload Rollback

Current upload order is:

1. validate request metadata and current incident or stream state at the HTTP
   layer
2. stream uploaded ciphertext to temporary storage while hashing
3. compare computed SHA-256 with the client-provided hash
4. commit encrypted bytes to a server-controlled immutable blob path
5. insert chunk metadata
6. remove the committed blob if the metadata insert fails

The PostgreSQL repository insert wraps state checks and metadata insert in
one transaction:

- lock or otherwise serialize against the incident row
- confirm the incident is still `open`
- for streamed chunks, lock or otherwise serialize against the stream row
- confirm the stream exists, belongs to the incident, is `open`, and matches
  `media_type`
- insert the chunk row
- rely on partial unique indexes as the final duplicate guard
- map duplicate identity to the same `duplicate_chunk` behavior as SQLite

If PostgreSQL metadata insertion fails after blob commit, the HTTP/storage layer
must preserve the current rollback expectation: remove the just-committed local
blob when it is safe to do so, and return a failure without leaving a chunk row.
For future object storage, cleanup may mean deleting a staging object or a
server-controlled final object only when the operation identity proves this
request created it.

PostgreSQL stores the implemented complete-upload idempotency records, but it
does not by itself make the upload flow production-cluster safe. Remaining
cluster-safe upload operation semantics are tracked in
[cluster-safe-upload-semantics.md](cluster-safe-upload-semantics.md) before
multi-node production deployment.

### Session And Token Creation, Lookup, And Revocation

Account session creation:

- generate the raw session token in application code using `crypto/rand`
- hash the raw token before database insertion
- insert only the hash and session metadata
- return the raw token only in the login response after successful commit
- never log the raw token, Authorization header, account password, password
  hash, or database connection string

Session lookup:

- hash the presented raw token in application code
- look up by token hash
- keep the constant-time equality check before accepting the session
- reject invalid, expired, and revoked sessions with the same authentication
  failure behavior

Session revocation:

- set `revoked_at` for logout, account password changes, admin password resets,
  and admin session-revocation actions
- do not delete session rows as part of ordinary revocation

Incident viewer token creation:

- generate the raw token in application code using `crypto/rand`
- hash the raw token before database insertion
- insert only the hash and metadata
- return the raw token only after successful commit
- never log the raw token or database connection string

Incident viewer token lookup:

- hash the presented raw token in application code
- look up by token hash
- keep the constant-time equality check before accepting the token
- reject invalid, expired, and revoked tokens with the same public behavior

Incident viewer token revocation:

- update `revoked_at` only when the token exists and is not already revoked
- preserve the current not-found behavior when no row changes
- do not delete token rows as part of revocation

## Configuration Shape

The current configuration scaffold accepts:

```bash
SAFE_METADATA_BACKEND=sqlite
SAFE_METADATA_BACKEND=postgresql
SAFE_BLOB_BACKEND=local
SAFE_BLOB_BACKEND=s3
SAFE_COORDINATION_BACKEND=none
```

PostgreSQL support keeps SQLite as the default and requires explicit
configuration:

```bash
SAFE_METADATA_BACKEND=postgresql
```

The PostgreSQL connection configuration is explicit and must be treated as
secret-bearing if it includes credentials:

```text
SAFE_POSTGRES_DSN
SAFE_POSTGRES_MAX_OPEN_CONNS
SAFE_POSTGRES_MAX_IDLE_CONNS
SAFE_POSTGRES_CONN_MAX_LIFETIME
```

The configuration docs state:

- `SAFE_DB_PATH` remains the SQLite path and is not a PostgreSQL setting
- PostgreSQL DSNs and credentials must not be logged
- unsupported backend names still fail startup without echoing rejected values
- setting `SAFE_METADATA_BACKEND=postgresql` requires `SAFE_POSTGRES_DSN`

## SQLite-To-PostgreSQL Data Migration Runbook

PostgreSQL support is implemented for new deployments. Migrating an existing
SQLite deployment is a separate explicit maintenance operation. It must not run
automatically during normal API startup, and it should remain a reviewed runbook
until repeated operator drills show that a small local-only tool would reduce
risk without hiding the safety gates.

This runbook is for private operator planning. Keep real commands, database
paths, DSNs, hostnames, object-store details, credentials, raw tokens, plaintext,
raw keys, and user safety data in private deployment notes, not in public issues
or PR descriptions.

### Current Support Boundary

Supported today:

- SQLite remains the default metadata backend and continues to be appropriate
  for local-first and small self-hosted deployments.
- PostgreSQL is an optional metadata backend when explicitly configured with
  `SAFE_METADATA_BACKEND=postgresql`.
- Local filesystem and optional S3-compatible blob storage keep storing opaque
  encrypted chunk bytes only.
- PostgreSQL migrations create the PostgreSQL schema; SQLite migrations and
  compatibility migrations still own the SQLite schema.

Not supported by this runbook:

- automatic startup migration from SQLite to PostgreSQL
- removing SQLite support
- moving or rewriting encrypted blob bytes
- decrypting chunks, browser decryption, backend decryption, key escrow, or raw
  server-held media keys
- public `/v1` exposure, public account workflows, cloud-provider automation, or
  production-readiness claims

### When To Stay On SQLite

Keep SQLite when the deployment is local-first, single-node, easy to quiesce,
and backed by a storage layer that can snapshot SQLite sidecar files and local
encrypted blobs together. SQLite remains the simplest supported shape for
development, simulator flows, and private small deployments that do not need a
separate managed metadata service.

Do not migrate merely because optional PostgreSQL exists. A migration adds
operational risk, another secret-bearing service, and a rollback boundary. It is
only justified when the deployment has a reviewed reason such as managed
database backups, future multi-node preparation, or operational requirements
that SQLite cannot satisfy.

### Preconditions

Before copying any data:

1. Confirm the target branch and release contain all SQLite and PostgreSQL
   migrations expected by the deployment.
2. Confirm the deployment can be quiesced. Stop the API process or block all
   authenticated main write routes before taking the migration snapshot.
3. Back up SQLite metadata and encrypted blobs as one evidence set. Include
   `SAFE_DB_PATH`, any live SQLite `-wal` and `-shm` sidecar files when using a
   direct live copy, local blob directories, or the configured S3-compatible
   committed object set.
4. Record a restore checkpoint in private operator notes. The checkpoint should
   describe how to restore the pre-migration SQLite database and matching
   encrypted blobs without exposing private paths or credentials publicly.
5. Apply all current SQLite migrations before export. The old
   `emergency_tokens` table is not part of the target PostgreSQL schema; current
   SQLite data should already be represented in `incident_tokens`.
6. Create an empty PostgreSQL database and apply the embedded PostgreSQL
   migrations through the application migration runner.
7. Keep both listeners private during the drill. Restore or migration
   validation must not expose `/v1`, `/admin`, admin JSON API routes, raw
   viewer tokens, or private deployment details publicly.

If these preconditions cannot be met, stop and keep the deployment on SQLite
until the operator can perform a private restore drill.

### Copy Order

Copy durable metadata in dependency order and preserve IDs exactly. Convert
SQLite timestamp strings to PostgreSQL `TIMESTAMPTZ` values normalized to UTC,
and preserve nullable fields as nulls rather than empty-string placeholders
unless the current SQLite schema already stores a meaningful empty string.

1. `accounts`
   - Preserve account IDs, normalized usernames, roles, bcrypt password hashes,
     creation/update times, and password-change times.
   - Never export or recreate raw passwords.
2. `incidents`
   - Preserve incident IDs, timestamps, status, `client_label`, `notes`,
     `owner_account_id`, optional mode fields, and `deletion_state`.
   - Preserve `owner_account_id` before private owner-scoped access is tested.
   - Legacy rows with no owner must remain unowned and admin-only until a
     separate reassignment workflow is explicitly implemented; see
     [legacy unowned incident reassignment](legacy-unowned-incident-reassignment.md).
3. `media_streams`
   - Preserve stream IDs, owning incident IDs, media type, label, status,
     expected chunk count, completion/failure timestamps, and failure reason.
4. `chunks`
   - Preserve chunk IDs, incident IDs, stream IDs, chunk indexes, media type,
     timestamps, sanitized `original_filename`, ciphertext byte size,
     lowercase SHA-256 hex, and `stored_path` exactly.
   - Do not rewrite `stored_path`, object keys, filesystem layout, or encrypted
     chunk bytes as part of metadata migration.
   - Preserve legacy unstreamed chunks with `stream_id IS NULL`.
5. `checkins`
   - Preserve checkin IDs, incident IDs, timestamps, battery, network, and
     location fields.
6. `auth_sessions`
   - Preserve session IDs, account IDs, SHA-256 token hashes, creation times,
     expiry, and revocation metadata only if the operator deliberately chooses
     to keep active sessions through migration.
   - A safer default is to omit or revoke sessions and require fresh login after
     the configuration switch. Never attempt to recover raw session tokens.
7. `incident_tokens`
   - Preserve token IDs, incident IDs, SHA-256 token hashes, labels, creation
     times, expiry, and revocation metadata.
   - Never attempt to recover raw viewer tokens from hashes.
8. `upload_operations`
   - Preserve complete-upload operation IDs, operation type,
     idempotency-key hashes, normalized chunk identity, immutable request
     fingerprint fields, state, final chunk references, stored paths, and
     timestamps where the current SQLite deployment has this table.
   - Do not include raw idempotency keys in export files or logs.
9. `incident_deletion_decisions` and `incident_deletion_items`
   - Preserve deletion decision IDs, source, reason code, actor account ID,
     `allow_open`, state, item count, safe error code, timestamps, item IDs,
     stored paths, attempts, and item state.
   - Preserve deletion state rather than restarting or clearing in-flight
     deletion work without a separate private operator decision.

Do not copy SQLite `schema_migrations` rows into PostgreSQL. PostgreSQL has its
own migration history and checksum tracking.

### Validation Before Switching

Validate the copied PostgreSQL database in an isolated private environment that
uses the same encrypted blob backend or a private restored copy of it.

Minimum checks:

- Compare row counts for every copied table.
- Compare per-incident child counts for streams, chunks, checkins,
  incident-token rows, upload operations, and deletion rows.
- Verify the critical uniqueness constraints by checking for duplicate streamed
  chunk identities, duplicate legacy chunk identities, duplicate session-token
  hashes, duplicate viewer-token hashes, duplicate idempotency-key hashes, and
  duplicate deletion decisions per incident.
- Confirm every streamed chunk references a stream on the same incident and
  media type.
- Confirm every chunk `stored_path` can be opened through the configured blob
  backend without exposing paths or object keys in public output.
- Generate completed stream and incident encrypted ZIP bundles in the private
  environment and confirm manifests match the expected stream and chunk
  metadata.
- Confirm missing blob or mismatched metadata drills fail closed rather than
  producing partial bundles.
- Test private owner-scoped incident access after preserving
  `owner_account_id`, and confirm legacy unowned incidents remain admin-only.
- Test expired and revoked incident tokens and sessions preserve their existing
  failure behavior.
- Confirm startup, private `/admin` dashboard access, and private account
  operations do not print DSNs, credentials, private hostnames, object keys,
  stored paths, tokens, plaintext, or raw keys.

Only switch `SAFE_METADATA_BACKEND=postgresql` after these checks pass and the
operator has a documented rollback point.

### Rollback And Restore Limits

Before PostgreSQL accepts writes, rollback is straightforward: restore the
pre-migration SQLite metadata and matching encrypted blobs, keep
`SAFE_METADATA_BACKEND=sqlite`, and restart the private deployment from that
checkpoint.

After PostgreSQL accepts new writes, rollback is no longer a simple
configuration flip. New incidents, chunks, sessions, token revocations,
idempotency records, and deletion state may exist only in PostgreSQL. Returning
to SQLite would require a separate reverse migration design with its own
quiesce, copy, validation, and restore gates.

If validation fails before the switch, discard the PostgreSQL target and keep
using the SQLite checkpoint. Do not partially repair copied metadata by editing
stored paths, object keys, token hashes, chunk hashes, or deletion rows unless a
separate private recovery plan explains the evidence-preserving outcome.

### Tooling Direction

A future migration tool may be useful only if it is explicit, local-only,
dry-run friendly, and separate from normal API startup. A safe tool would still
need to print only safe counts, preserve all migration gates from this runbook,
refuse to run against a live writing deployment, avoid raw token or secret
output, and require operator confirmation before writing the PostgreSQL target.

Until that exists, treat this runbook and private restore drills as the source
of truth for SQLite-to-PostgreSQL migration planning.

## Testing Expectations

PostgreSQL support should not be accepted on schema creation alone. Tests need
to prove repository behavior stays aligned.

Implemented test groups:

- opt-in migration tests for fresh PostgreSQL schema creation
- opt-in migration idempotence and checksum-mismatch tests
- opt-in schema constraint tests for statuses, media types, SHA-256 shape,
  stream media matching, token-hash uniqueness, and foreign keys
- opt-in repository behavior tests covering streamed and legacy chunk identity,
  stream completion, closed-incident rejection, token hashing, and revocation

Additional expected test groups before recommending production-cluster use:

- repository contract tests that run against SQLite and PostgreSQL when a
  PostgreSQL test DSN is available
- HTTP handler tests using the metadata repository boundary where practical
- concurrency tests for upload versus incident close, upload versus stream
  completion, duplicate chunk races, and token revocation
- restore-oriented tests or scripted validation that proves metadata plus blobs
  can reconstruct completed bundles

PostgreSQL integration tests are opt-in with `SAFE_POSTGRES_TEST_DSN`. This
repository does not add Docker Compose or cloud dependencies just to run the
default test suite. If a test DSN contains credentials, test setup, failure
output, and CI logs must treat it as secret-bearing and avoid printing it.

Example local invocation with a deployment-specific test database:

```bash
SAFE_POSTGRES_TEST_DSN='<secret test database DSN>' go test ./internal/postgresdb
```

The integration tests create and drop isolated schemas inside the configured
database. Do not point the test DSN at a database where schema creation or
dropping is not acceptable.

For a disposable local test database, a one-off Docker container is enough; the
repository does not use Docker Compose for this path:

```bash
set -euo pipefail

docker run --rm -d --name proofline-postgres-test \
  -e POSTGRES_DB=proofline_test \
  -e POSTGRES_USER=proofline \
  -e POSTGRES_HOST_AUTH_METHOD=trust \
  -p 127.0.0.1:55432:5432 \
  postgres:16-alpine

trap 'docker rm -f proofline-postgres-test >/dev/null 2>&1 || true' EXIT

until docker exec proofline-postgres-test \
  pg_isready -U proofline -d proofline_test >/dev/null 2>&1; do
  sleep 1
done

SAFE_POSTGRES_TEST_DSN='postgres://proofline@127.0.0.1:55432/proofline_test?sslmode=disable' \
  go test ./internal/postgresdb -count=1
```

The `trust` authentication setting above is only for a disposable local test
container bound to loopback. Do not use it for shared, remote, or production
PostgreSQL deployments.

Behavior that should remain SQLite-specific:

- `PRAGMA foreign_keys = ON`
- SQLite WAL setup and WAL verification
- SQLite `:memory:` behavior
- SQLite compatibility migrations that inspect `PRAGMA table_info`
- SQLite `GLOB` checks
- SQLite single-connection behavior
- SQLite database path handling through `SAFE_DB_PATH`

Behavior that must be backend-parity behavior:

- incident and stream state transitions
- stream completion validation
- chunk duplicate identity
- legacy unstreamed chunk compatibility
- password hashing, session-token hashing, expiry, and revocation
- incident-token hashing, expiry, and revocation
- no raw token logging or storage
- no stored path exposure in public responses
- encrypted ZIP bundle reconstruction from completed streams

## Restore And Operations Expectations

PostgreSQL backups must still be paired with encrypted blob backups. Metadata
without blobs can leave evidence impossible to bundle; blobs without metadata
can leave evidence hard to locate, verify, or serve.

Before any production-cluster recommendation, documentation should cover:

- PostgreSQL backup method, such as `pg_dump`, physical backup, or managed
  database snapshot
- backup consistency with local or future object-storage blobs
- restore into an isolated environment with local or otherwise private-boundary
  bind addresses only
- migration and restore ordering
- row-count and checksum-style validation after restore
- completed stream and incident bundle verification after restore
- rollback expectations before and after PostgreSQL accepts writes

Restores must not be used as a reason to expose `/v1` publicly. The same
main/private-admin listener split and token redaction expectations apply.

## PostgreSQL-Specific Implementation Sequence

PostgreSQL implementation status and remaining work:

1. Keep this design current with the SQLite schema and repository boundary.
2. PostgreSQL migration files and a PostgreSQL migration runner are implemented.
3. A PostgreSQL repository implementation exists behind the existing metadata
   repository behavior.
4. Opt-in PostgreSQL integration tests exist for migrations, constraints, and
   repository behavior. Broader shared contract tests can still be expanded.
5. `SAFE_METADATA_BACKEND=postgresql` is implemented while SQLite remains the
   default.
6. Restore documentation for PostgreSQL plus encrypted blobs exists as
   operational guidance and should be strengthened by deployment-specific drills.
7. Complete the remaining upload-operation behavior documented in
   [cluster-safe-upload-semantics.md](cluster-safe-upload-semantics.md) before
   multi-node cluster safety is recommended.
8. Separately design any SQLite-to-PostgreSQL migration tool or runbook.

Each step should be small and reviewable. Do not bundle PostgreSQL support with
S3-compatible blob storage, unrelated Valkey/Redis-compatible coordination
behavior, public account workflows, or key custody work.
