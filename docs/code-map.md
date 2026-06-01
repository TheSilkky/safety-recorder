# Code Map

Proofline Server currently contains the Go backend for a private encrypted
incident-capture system. This backend receives already-encrypted recording
chunks, groups them into media streams, records metadata in SQLite by default
or optional PostgreSQL, supports optional Valkey/Redis-compatible short-lived
coordination, serves authenticated incident deletion and optional closed-incident
retention workflows, and serves a scoped read-only incident viewer with
encrypted evidence bundle downloads. It also stores owner-scoped
trusted-contact public-key metadata, incident/stream sharing-grant metadata,
and grant-bound wrapped-key records without adding decryption.

This repository is the server/backend component only. In the planned `open-proofline` organisation layout it corresponds to `open-proofline/server`. Future web-client, iOS-client, Android-client, and protocol implementation should live in separate repositories.

The current backend stores generic incidents by default and can store optional
incident-mode, capture-profile, escalation-policy, and sharing-state metadata on
main incident create/read routes. Those fields do not drive access,
notification, retention, sharing, viewer, or key-custody behavior.
Account-owner contact public-key, sharing-grant, and wrapped-key metadata is
implemented separately behind authenticated `/v1` routes. Mode-driven behavior
boundaries are documented in [incident-modes.md](incident-modes.md), with role
and grant boundaries in [v1-access-control.md](v1-access-control.md) and
contact key-sharing boundaries in
[contact-key-sharing-grants.md](contact-key-sharing-grants.md).

## Package Layout

- `go.mod`: defines the root Go module `github.com/open-proofline/server`.
- `.github/workflows/ci.yml`: runs Go tests with a coverage signal on pull requests and pushes, runs `govulncheck`, builds the `proofline-server-linux-amd64` binary artifact, gates release binary attestation and trusted GHCR publishing on the vulnerability scan, uploads the binary as a GitHub Release asset on `v*` tag pushes, builds the Docker image, and publishes attested images to GitHub Container Registry from a trusted job limited to `main`, `develop`, and `v*` tag pushes.
- `.dockerignore`: excludes local runtime, review, and build artifacts from the root Docker build context used by `Dockerfile`.
- `cmd/api`: starts one main API/viewer HTTP server per main bind address and one private-admin dashboard HTTP server per admin bind address, loads config, enforces the local account bootstrap gate, checks the selected coordination backend, opens the selected metadata backend, creates storage, wires shared handlers including main API, admin JSON API, public viewer rate limiting, upload coordination, and the private `/admin` dashboard, starts the deletion worker, and handles graceful shutdown.
- `cmd/simclient`: simulates future client flows by logging in, creating an incident, creating a media stream, encrypting and uploading complete chunks, completing or failing streams, sending periodic checkins, and optionally testing hash-failure retry, bundle download, local decrypt verification, durable desktop-recorder staging, local file input, ffmpeg segment capture, restart/resume behavior, and poor-network retry controls. Token-bearing viewer URLs are omitted from simulator output.
- `internal/config`: reads environment variables such as backend selectors, backend-specific settings, main and private-admin bind address lists, legacy singular bind addresses, data directory, database path, max upload size, upload coordination lease TTL, main API and public viewer rate limits, HTTP server timeouts, local account bootstrap secret, session TTL, deletion worker interval, closed-incident retention window, token metadata retention window, and tombstone retention window.
- `internal/coordination`: defines the small optional coordination boundary, the default no-coordination backend, and the Valkey/Redis-compatible startup check, main API/public viewer rate-limit counter backend, and short-lived complete-upload lease backend.
- `internal/db`: opens SQLite, enables foreign keys and WAL mode, applies embedded SQLite migrations, records `schema_migrations`, and runs named compatibility migrations.
- `internal/envelope`: implements the simulator/test AES-256-GCM client-side chunk envelope, associated data builder, and local simulator key file helpers.
- `internal/auth`: normalizes local account usernames, validates passwords, hashes passwords with bcrypt, and hashes opaque session tokens before storage.
- `internal/httpapi`: owns separate main and private-admin dashboard muxes, JSON responses, request logging, recovery, local account/session authentication, request validation, upload handling, stream state handlers, contact public-key handlers, sharing-grant handlers, wrapped-key handlers, incident deletion handlers, ZIP bundle streaming, app-level main API, admin JSON API, and public viewer rate limiting, the private admin web surface, the incident viewer, and the narrow metadata repository boundary consumed by handlers.
- `internal/incidents`: defines incident/stream/chunk/checkin/account/session/deletion/contact-key/sharing-grant/wrapped-key models and provides the SQLite metadata repository implementation, including deletion decisions, tombstones, retry item state, contact public-key records, sharing-grant records, wrapped-key records, and write guards for deleting incidents.
- `internal/postgresdb`: opens optional PostgreSQL metadata connections, applies PostgreSQL migrations, and implements the metadata repository behavior with PostgreSQL transaction, row-locking, deletion, and constraint semantics.
- `internal/retention`: runs the background deletion and optional closed-incident retention worker. It claims retryable deletion decisions, removes encrypted blobs through the storage boundary using stored paths snapshotted from metadata, records safe retry state, prunes sensitive child metadata after blob deletion, and logs only non-sensitive counts or error categories.
- `internal/storage`: defines the blob-store boundary used by HTTP handlers and provides local filesystem and optional S3-compatible implementations, including temp uploads, hashing while streaming, server-controlled stored paths, and immutable final commits.
- `migrations`: embeds the SQLite schema.
- `migrations/postgres`: embeds the PostgreSQL schema.
- `compose`: contains local Docker Compose smoke-test stacks and a runner script
  for disposable SQLite/local, PostgreSQL/local, SQLite/S3-compatible MinIO, and
  full PostgreSQL/MinIO/Valkey validation. These files are local release-smoke
  helpers, not production deployment manifests.

There is no implemented `cmd/stream-ingress` package. The future regional
stream-ingress relay is planning-only in
[regional-stream-ingress-relay.md](regional-stream-ingress-relay.md). If added
later, it should be a separate upload-only edge that stages ciphertext
temporarily and lets the core API remain authoritative for authorization,
idempotency, durable blob commits, and metadata.

## Main Request Flow

Main `/v1` routes require `Authorization: Bearer <session_token>` except for
login. Existing `/v1/admin/...` JSON routes are mounted on the main handler and
require an admin account. First-admin bootstrap uses the private
`/admin/bootstrap` form when no admin exists and `SAFE_AUTH_BOOTSTRAP_SECRET`
is configured.
Session tokens are opaque, returned only to the client, and stored as hashes by
the metadata repository.

The current listener split does not mount `/v1/health/live` or
`/v1/health/ready` on either listener.

Incidents are created in `internal/httpapi.createIncident`, which calls
`CreateIncidentForAccount` on the configured metadata repository and records the
authenticated account as the owner. Admin accounts can operate across incidents;
regular user accounts are limited to their own incidents. Legacy unowned
incidents are admin-only until a future private reassignment or quarantine
workflow is implemented; see
[legacy unowned incident reassignment](legacy-unowned-incident-reassignment.md).

Chunks are uploaded through `POST /v1/incidents/{incident_id}/chunks`, handled by `internal/httpapi.uploadChunk`.

Upload handling first checks that the incident exists and is open. The file is then streamed by `internal/httpapi.readChunkUpload` into `internal/storage.BlobStore.SaveTemp`, which the current local implementation writes to `data/tmp` while computing SHA-256 and enforcing the upload byte limit.

Hash verification happens in `internal/httpapi.uploadChunk` by comparing the computed temp-file hash with the client-provided `sha256_hex`.

When configured Valkey/Redis-compatible coordination is available,
`internal/httpapi` acquires a short-lived complete-upload lease keyed by a
server-controlled hash of chunk identity. A busy lease returns
`409 upload_in_progress` with `Retry-After`; lease failures return a safe
retryable error. When `Idempotency-Key` is supplied, `internal/httpapi` hashes
the raw key, builds a canonical complete-upload fingerprint from normalized
chunk identity, timestamps, normalized `original_filename`, ciphertext byte
size, and `sha256_hex`, then reserves or replays durable upload-operation state
through the metadata repository. Equivalent retries return `200 OK` with
`Idempotency-Replayed: true`; uploads without the header keep the existing
duplicate behavior.

`POST /v1/incidents/{incident_id}/chunks/reconcile` is a private read-only
metadata route handled by `internal/httpapi.reconcileChunk`. It compares a
client's expected normalized chunk identity, timestamps, normalized
`original_filename`, ciphertext byte size, and ciphertext SHA-256 against an
accepted chunk row. Matched responses return only safe identity and fingerprint
metadata; conflict responses return mismatched field names without stored
paths, object keys, uploaded bytes, plaintext, raw keys, raw tokens, request
bodies, or conflicting stored values.

After verification, `internal/storage.BlobStore.CommitTemp` commits the encrypted bytes under the server-controlled stored path:

```text
data/incidents/{incident_id}/streams/{stream_id}/{media_type}_{zero_padded_chunk_index}.enc
```

Legacy unstreamed chunks keep the older path:

```text
data/incidents/{incident_id}/{media_type}_{zero_padded_chunk_index}.enc
```

Local storage maps that stored path under `SAFE_DATA_DIR`. Optional S3-compatible storage maps the same stored path under `SAFE_S3_PREFIX` in the configured bucket. Storage uses no-overwrite behavior, so an existing local file or final object is treated as a conflict.

Metadata is written after the file is safely committed, through the configured metadata repository. SQLite uses `internal/incidents.Repository`; PostgreSQL uses `internal/postgresdb.Repository`. Both implementations recheck incident and stream state before inserting chunk metadata so uploads that race with incident close or stream completion are rejected. The schemas enforce separate unique identities for streamed and legacy unstreamed chunks.

New clients can create a media stream with `POST /v1/incidents/{incident_id}/streams` and include the returned `stream_id` during chunk upload. Streamed chunk indexes start at `1`, and streamed chunk identity is `incident_id + stream_id + chunk_index`. Existing chunks without `stream_id` remain valid and readable as legacy chunk metadata, including older index `0` chunks; legacy unstreamed identity remains `incident_id + media_type + chunk_index`. Legacy unstreamed chunks are not included in completed-stream evidence bundles.

Stream completion is handled by `internal/httpapi.completeMediaStream`. Before a stream moves from `open` to `complete`, the handler verifies that chunks `1..expected_chunk_count` exist contiguously for that stream and that each stored blob can be opened from the configured blob store. `internal/incidents.Repository.CompleteMediaStream` then revalidates the chunk rows in the completion transaction before committing the state change. Failed streams preserve uploaded chunks but are not offered as normal downloads.

Contact public-key registration is handled by
`POST /v1/contact-public-keys` and related routes in
`internal/httpapi/sharing_handlers.go`. The handlers use the authenticated
local account as the owner scope and store only public-key metadata through the
configured metadata repository. They reject unknown JSON fields and do not
accept contact private keys, raw media keys, wrapped media keys, plaintext, or
browser fragment secrets.

Sharing grants are handled by
`POST /v1/incidents/{incident_id}/sharing-grants`, incident grant listing, and
grant lookup/revocation routes. Grant creation is owner-only, checks that the
incident is active, optionally checks that the stream belongs to the incident,
and requires an active contact public key owned by the same account. Grants
record incident or stream scope, contact ID, contact key version, data class,
expiry, and revocation state. They do not deliver wrapped media keys, decrypt
evidence, or change public incident viewer behavior.

Wrapped-key records are handled by
`POST /v1/incidents/{incident_id}/wrapped-keys`, incident wrapped-key listing,
and wrapped-key lookup/revocation routes. Creation is owner-only, checks that
the incident is active, optionally checks stream ownership, requires an active
grant for the same owner and incident, requires ciphertext access, and requires
the bound contact public key to remain active. List and read responses are
delivery-filtered so revoked or expired grants, inactive contact keys, and
revoked or rotated wrapped-key records are omitted. These routes deliver
encrypted wrapped-key metadata through authenticated API responses only; public
viewer routes and bundle manifests remain key-free.

## Deletion And Retention Flow

Main owner-scoped deletion requests are handled by
`POST /v1/incidents/{incident_id}/deletion`. Admin-global deletion requests are
handled by `POST /v1/admin/incidents/{incident_id}/deletion` on the main
handler with an admin account. Public incident viewer routes do not expose
deletion controls or deletion status.

The configured metadata repository creates or returns one durable deletion
decision for the incident. In the same transaction, it snapshots
server-controlled chunk `stored_path` values into deletion item rows and marks
the incident deletion state as `deletion_pending`. Repeated requests return the
existing decision instead of creating competing work. Open incidents are
rejected unless the authenticated request explicitly sets `allow_open: true`;
automatic closed-incident retention never selects open incidents.

While an incident is `deletion_pending`, `deleting`, `deletion_failed`, or
`deleted`, normal write paths fail closed in the repository. Public viewer token
lookups also fail closed with the same public error shape used for invalid,
expired, or revoked tokens, so public routes do not reveal deletion state.

`internal/retention.Worker` queues closed-incident retention decisions only
when `SAFE_CLOSED_INCIDENT_RETENTION` is positive. It can also prune
expired/revoked viewer-token metadata and completed minimal tombstones when the
corresponding retention settings are positive. It processes pending, failed, or
stale `deleting` decisions in batches, deletes encrypted blobs through
`storage.BlobStore.Remove` using only metadata-derived stored paths, treats a
missing blob as idempotent success for an existing deletion item, and records
safe retry error classes for failed blob deletions.

After every deletion item is complete or confirmed absent, the repository
prunes sensitive child metadata such as upload operations, incident tokens,
checkins, chunks, and streams, then leaves a minimal incident tombstone and
marks the deletion decision `deleted`.

## Admin Web Flow

`GET /admin` is mounted only on the private-admin listener, outside the `/v1` API
namespace. When no admin account exists and `SAFE_AUTH_BOOTSTRAP_SECRET` is
configured, it renders a first-admin bootstrap screen. After an admin exists,
it renders an admin login screen until a valid admin web session cookie is
present. `POST /admin/login`, `POST /admin/bootstrap`, and
`POST /admin/logout` use the same account and server-side session repository
as the JSON API, with the raw session token stored in an HttpOnly
SameSite=Strict cookie scoped to `/admin`.

Authenticated admin pages list local accounts and support limited password
workflows. `POST /admin/password` changes the current admin account password
after verifying the current password, keeping the current session and revoking
other sessions. `POST /admin/accounts/{account_id}/password` resets another
local account password and revokes that account's sessions. `POST
/admin/logout` revokes the current admin web session. These authenticated
state-changing forms use a session-bound CSRF token.

The page renders `internal/httpapi/web/templates/admin.html` with Go
`html/template`. Token-neutral CSS is embedded from
`internal/httpapi/web/admin/static` and served without authentication under
`/admin/static/...`.

The admin web surface shows only safe route-boundary status, navigation stubs,
and local account-management data. It does not read incident data, expose
tokens or password hashes, expose stored paths or object keys, show uploaded
bytes, decrypt evidence, or add public dashboard behavior. Admin web responses
use no-store behavior and conservative browser security headers.

## Incident Viewer Flow

Viewer tokens are created on the authenticated main API listener by
`POST /v1/incidents/{incident_id}/incident-tokens`. The raw token is returned
once, while the configured metadata repository stores only a SHA-256 hash.

`GET /i/{token}` is mounted on the main API/viewer listener. It renders `internal/httpapi/web/templates/incident_viewer.html` with `html/template`. CSS and JavaScript are embedded from `internal/httpapi/web/static`. `GET /i/{token}/data` returns the same read-only summary as JSON for polling. Pre-rename `/e/{token}` viewer, data, and download paths remain as read-only compatibility aliases for already shared links; new links should use `/i/{token}`.

Token lookup checks the hash, expiry, and revocation state before incident metadata is loaded. Invalid, expired, and revoked tokens all return the same public error. The public viewer limiter groups requests by safe route class and a hash of the socket peer identity before token lookup; limiter keys do not include raw viewer tokens or token-bearing paths. Viewer responses use `Referrer-Policy: no-referrer`, `X-Content-Type-Options: nosniff`, a strict `Content-Security-Policy`, restrictive `Permissions-Policy`, and `Cache-Control: no-store` for token-protected responses.

Completed stream bundle downloads are served by `internal/httpapi/bundles.go`. Bundles are generated on demand as ZIP responses and are not cached on disk. ZIP entry names are server-controlled, manifests are generated from database metadata, and chunk bytes are streamed from storage one file at a time. The first bundle format contains encrypted chunks and JSON manifests only; it does not decrypt, merge, or export playable media.

## Server Repository Boundary

The separate ports are a deployment boundary, not a complete security model.
Local account sessions reduce accidental unauthenticated access, but the main
`/v1` API should still stay behind the reviewed deployment boundary for that
deployment, and the private-admin server should stay behind localhost, LAN,
WireGuard, firewall rules, or a strict private reverse proxy.

This repository should stay focused on server/backend work:

- API handlers and routing
- SQLite migrations and repository code
- encrypted blob storage
- token-scoped incident viewer
- contact public-key, sharing-grant, and wrapped-key metadata
- backend deployment docs
- backend security, retention, and threat-model docs
- simulator/reference backend flow
- planning docs for future decryption clients

Before public exposure, review and add:

- the public product API and separately bound private admin API access-control
  design in [v1-access-control.md](v1-access-control.md), or a strict
  WireGuard/firewall-only deployment for the current main `/v1` API
- the target main API/public viewer and private admin-dashboard listener split
  in [public-api-listener-split.md](public-api-listener-split.md)
- edge rate limits, app-level public viewer limits, and broader abuse controls
- TLS and reverse-proxy settings for the public incident viewer, if reachable over a network
- deployment-specific enforcement of the documented [retention, backup, and deletion policy](retention-backup-deletion.md)
- cluster backup, restore, and failure drills for optional PostgreSQL metadata,
  S3-compatible encrypted blobs, and Valkey/Redis-compatible coordination as
  documented in
  [cluster-backup-restore-runbook.md](cluster-backup-restore-runbook.md)
- operational monitoring for failed uploads and storage/DB errors
- a production review of viewer-token sharing, expiry defaults, and revocation operations
- mode-driven access, escalation, retention, sharing, viewer, key-custody,
  trusted-contact, public product API, and broader
  admin/operator authorization
  design before implementing public account workflows or broad public `/v1`
  exposure

## Out Of Scope Today

The repository does not currently include the web client, iOS app, Android app,
protocol repository, production local recording client, mode-driven access,
escalation, retention, viewer behavior, trusted-contact accounts, wrapped-key
delivery outside owner-authenticated private `/v1`, dead-man switch
notifications, production client key storage, browser/client-side decryption,
server-assisted break-glass key access,
playable media export, push notifications, SMS, Messenger integration, OAuth,
JWT, public account workflows, or a public admin dashboard. The local
desktop-recorder behavior in `cmd/simclient` is simulator/reference flow only.
