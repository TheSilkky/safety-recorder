# Codex Prompt: Next Work Order 1-2-3

This prompt is historical/reference-only. Do not re-run it without checking it
against the current `README.md`, `AGENTS.md`, `SECURITY.md`, docs, and reusable
prompts.

Implement the next three backend hardening items:

1. Fix chunk-index semantics for streamed/encrypted chunks.
2. Add a proper schema migration version table.
3. Add configurable HTTP server timeouts without breaking uploads or bundle downloads.

This is a focused backend maintenance task.

Do **not** add new product features.
Do **not** add authentication, OAuth, JWT, user accounts, SMS, Messenger, push notifications, React, Node, npm, Docker Compose, Kubernetes, cloud integrations, or public admin dashboard features.
Do **not** implement iOS code.
Do **not** implement backend decryption.
Do **not** change the encryption envelope format unless required only to align validation with existing documented behaviour.

## Project context

Safety Recorder is an experimental Go backend for a private personal-safety recording system.

Current project shape:

- Go backend only
- private `/v1` write/admin API listener group
- public read-only emergency viewer listener group
- SQLite metadata
- local disk encrypted chunk storage
- immutable chunk uploads
- media streams that can be marked `open`, `complete`, or `failed`
- completed encrypted stream and incident ZIP evidence bundle downloads
- emergency viewer tokens
- simulator CLI
- documented v1 AES-256-GCM simulator encryption envelope
- Docker image build
- GitHub Actions / GHCR publishing
- AGPL-3.0-only license
- repository security policy

Evidence bundles are encrypted chunk bundles, not decrypted/playable media exports.

The backend must remain ignorant of plaintext and keys.

## Source of truth

Read before changing code:

- `README.md`
- `AGENTS.md`
- `CHANGELOG.md`
- `docs/api.md`
- `docs/configuration.md`
- `docs/encryption.md`
- `docs/security-model.md`
- `docs/threat-model.md`
- `docs/code-map.md`
- `server/internal/config`
- `server/internal/db`
- `server/internal/httpapi`
- `server/internal/incidents`
- `server/internal/storage`
- `server/internal/envelope`
- `server/migrations`

## Work item 1: Fix chunk-index semantics

### Problem

Current docs/code allow `chunk_index` to be non-negative in some places, while media stream completion and the encryption associated-data model expect stream chunks to be indexed from `1..expected_chunk_count`.

This creates an inconsistent edge case:

- unstreamed legacy chunks may allow `chunk_index = 0`
- streamed chunks should be positive
- encrypted chunks should be positive because the encryption envelope associated data rejects non-positive chunk indexes
- stream completion already verifies chunks `1..expected_chunk_count`

### Required behaviour

Implement this rule:

```text
If stream_id is provided:
  chunk_index must be >= 1

If stream_id is empty:
  preserve existing legacy behaviour unless changing it is clearly safe
```

Recommended legacy behaviour:

```text
unstreamed chunk_index >= 0 remains accepted for backwards compatibility
streamed chunk_index <= 0 is rejected
```

### API response

When a streamed upload uses `chunk_index <= 0`, return:

```http
400 Bad Request
```

Use an error code such as:

```text
invalid_chunk_index
```

The message should make the stream-specific rule clear, for example:

```text
chunk_index must be positive when stream_id is provided
```

### Do not break

Do not break existing legacy unstreamed chunk tests unless you intentionally update docs and tests.

Do not change the chunk identity model yet. It currently remains:

```text
incident_id + media_type + chunk_index
```

Do not change it to include `stream_id` in this task.

### Tests

Add or update tests for:

- unstreamed `chunk_index = 0` remains accepted, if preserving legacy behaviour
- streamed `chunk_index = 0` is rejected with `400 invalid_chunk_index`
- streamed `chunk_index = -1` is rejected
- normal streamed chunks starting at `1` still upload successfully
- stream completion still requires contiguous chunks from `1..expected_chunk_count`
- simulator still uploads stream chunks starting at `1`
- encryption envelope tests still reject non-positive chunk index

### Docs

Update docs to clearly state:

- new clients should use stream IDs
- streamed chunks must use chunk indexes starting at `1`
- legacy unstreamed chunks may still appear with index `0`, if preserved
- encryption associated data requires positive chunk indexes

Update relevant files:

- `docs/api.md`
- `docs/encryption.md`, if needed
- `docs/simulator.md`, if needed
- `README.md`, only if concise update is useful

## Work item 2: Add schema migration version table

### Problem

Current migration handling applies embedded SQL migrations lexically and also contains compatibility helpers in Go.

That is acceptable for early development, but the repo now needs explicit migration tracking before schema changes become more complicated.

### Goal

Add a proper schema migration table and record applied migrations.

### Required table

Add a table such as:

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
  id TEXT PRIMARY KEY,
  checksum TEXT NOT NULL,
  applied_at TEXT NOT NULL
);
```

The exact name can be `schema_migrations`.

### Migration IDs

Use stable migration IDs based on migration filenames for SQL migrations, for example:

```text
001_init.sql
002_emergency_tokens.sql
003_media_streams.sql
```

If there are Go compatibility migrations, give them stable IDs, for example:

```text
004_chunks_stream_id_compat
005_drop_emergency_token_last_used_compat
```

Only include compatibility migrations that are actually needed by the current code.

### Migration behaviour

Implement:

- create `schema_migrations` before applying application migrations
- apply embedded `.sql` migrations in lexical order
- record each applied migration in `schema_migrations`
- skip migrations already recorded
- store a SHA-256 checksum for SQL migration content
- if a recorded SQL migration checksum does not match current content, return a clear error
- apply each migration in a transaction where practical
- keep foreign keys enabled
- keep WAL mode behaviour
- preserve support for `:memory:` databases

### Existing database compatibility

Do not break existing databases created by the current v0.4 code.

Because earlier SQL migrations mostly use `CREATE TABLE IF NOT EXISTS`, it is acceptable to run them once and then record them if they were not recorded.

Handle existing databases that:

- already have `incidents`, `chunks`, `checkins`, `emergency_tokens`, and/or `media_streams`
- already have `chunks.stream_id`
- do not yet have `schema_migrations`
- are fresh empty databases

### Compatibility helpers

Replace or wrap ad-hoc schema helpers with recorded compatibility migrations.

For example:

- `chunks.stream_id` should be added only if missing and recorded as applied
- existing `last_used_at` cleanup, if still needed, should be treated as a named compatibility migration or removed if no longer appropriate

Do not introduce new destructive migrations.

If a compatibility migration must remove a column or data, document why and add tests.

### Tests

Add tests for:

- fresh DB migration creates `schema_migrations`
- fresh DB records all current SQL migrations
- running migration twice is idempotent
- recorded checksum mismatch fails clearly
- existing DB without `schema_migrations` is handled
- existing DB without `chunks.stream_id` gets the column
- existing DB with `chunks.stream_id` does not fail
- in-memory DB still works
- current app tests still pass

### Docs

Update relevant docs:

- `docs/code-map.md`
- `docs/development.md`, if it mentions DB setup
- `docs/configuration.md`, if it mentions data layout
- `CHANGELOG.md`

Keep the docs concise.

## Work item 3: Add configurable HTTP server timeouts

### Problem

The HTTP servers currently set `ReadHeaderTimeout`, but do not configure a broader timeout story.

The public emergency viewer may eventually be exposed through HTTPS/reverse proxy, while the private API may accept large mobile uploads. Timeouts must improve safety without accidentally breaking slow uploads or evidence bundle downloads.

### Goal

Add a small, documented, configurable timeout model.

### Recommended config model

Add duration configuration for both private and public servers.

Prefer clear environment variables.

Suggested variables:

```text
SAFE_PRIVATE_READ_HEADER_TIMEOUT
SAFE_PRIVATE_READ_TIMEOUT
SAFE_PRIVATE_WRITE_TIMEOUT
SAFE_PRIVATE_IDLE_TIMEOUT

SAFE_PUBLIC_READ_HEADER_TIMEOUT
SAFE_PUBLIC_READ_TIMEOUT
SAFE_PUBLIC_WRITE_TIMEOUT
SAFE_PUBLIC_IDLE_TIMEOUT
```

Recommended defaults:

```text
SAFE_PRIVATE_READ_HEADER_TIMEOUT = 10s
SAFE_PRIVATE_READ_TIMEOUT        = 0s
SAFE_PRIVATE_WRITE_TIMEOUT       = 0s
SAFE_PRIVATE_IDLE_TIMEOUT        = 120s

SAFE_PUBLIC_READ_HEADER_TIMEOUT  = 10s
SAFE_PUBLIC_READ_TIMEOUT         = 30s
SAFE_PUBLIC_WRITE_TIMEOUT        = 300s
SAFE_PUBLIC_IDLE_TIMEOUT         = 120s
```

Rationale:

- private read/write timeouts default to disabled because chunk uploads and private downloads may be large/slow
- public read timeout can be conservative because public viewer requests do not upload bodies
- public write timeout is finite but generous enough for ZIP bundle downloads
- idle timeout is useful for both listener groups
- read header timeout remains conservative

If you choose different defaults, document the rationale.

### Duration parsing

Use Go duration strings via `time.ParseDuration`.

Accept examples:

```text
10s
30s
5m
0s
0
```

`0` or `0s` should mean disabled for read/write timeout fields where appropriate.

Reject negative durations.

Return clear config errors.

### Server application

Private servers should use private timeout settings.

Public servers should use public timeout settings.

Make sure each bind address gets the correct timeout configuration.

### Tests

Add config tests for:

- default timeouts
- valid duration env vars
- `0` and `0s`
- negative duration rejected
- invalid duration rejected
- private and public server configs use the correct timeout groups

If current tests instantiate servers indirectly, add targeted tests for server construction.

### Docs

Update:

- `docs/configuration.md`
- `docs/deployment.md`
- `docs/security-model.md`, if relevant
- `README.md`, only if concise
- `CHANGELOG.md`

Documentation should explain:

- private upload/download timeouts are conservative by default
- public viewer has more defensive defaults
- reverse proxies should also set their own timeouts
- large ZIP downloads may require tuning `SAFE_PUBLIC_WRITE_TIMEOUT`

## Constraints

Do not:

- add authentication
- expose `/v1` publicly
- change token semantics
- change encryption scheme
- change bundle format
- change chunk identity model
- change Docker image behaviour unless needed only for env var documentation
- change CI workflows unless tests require it
- add source-code license headers
- add new third-party dependencies unless strictly necessary

Prefer small, readable changes.

Use Go standard library where practical.

## Validation

After implementation:

```bash
cd server
gofmt -w .
go test ./...
go vet ./...
```

If `go vet ./...` fails because of an existing known harmless issue, document it clearly.

Manual smoke test:

```bash
cd server
go run ./cmd/api
```

In another terminal:

```bash
cd server
go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle
```

Expected:

- simulator encrypts chunks by default
- chunks upload successfully
- stream completes
- bundle downloads
- local bundle decrypt verification succeeds
- no plaintext or keys are printed
- server startup logs still show private/public listener addresses

Also manually test the streamed zero-index rejection if practical:

```bash
# create incident + stream, then attempt streamed upload with chunk_index=0
# expected: 400 invalid_chunk_index
```

## Documentation and changelog

Update `CHANGELOG.md` under `Unreleased` or the next appropriate version.

Suggested entry:

```md
- Rejected non-positive chunk indexes for streamed uploads while preserving legacy unstreamed compatibility.
- Added explicit schema migration tracking with `schema_migrations`.
- Added configurable private/public HTTP server timeout settings.
```

Do not tag a release in this task.

## Output after implementation

Summarize:

1. files changed
2. chunk-index behaviour changes
3. migration table design
4. compatibility handling for existing databases
5. timeout variables and defaults
6. tests added/updated
7. docs updated
8. validation commands run
9. any known follow-up work
