# Code Map

Proofline Server currently contains the Go backend for a private encrypted
incident-capture system. This backend receives already-encrypted recording
chunks, groups them into media streams, records metadata in SQLite by default
or optional PostgreSQL, supports optional Valkey/Redis-compatible short-lived
coordination, serves authenticated incident deletion and optional closed-incident
retention workflows, and serves a scoped read-only incident viewer with
encrypted evidence bundle downloads. It also stores owner-scoped account/device
recipient public-key metadata, account-to-account trusted-contact relationship
metadata, trusted-contact public-key metadata, incident/stream sharing-grant
metadata, and grant-bound wrapped-key records without adding decryption.

This repository is the server/backend component only. In the current
`open-proofline` organisation it is `open-proofline/server`. Web-client,
iOS-client, Android-client, and protocol implementation should live in separate
repositories.

The current backend stores generic incidents by default and can store optional
incident-mode, capture-profile, escalation-policy, and sharing-state metadata on
main incident create/read routes. The account incident list/detail routes return
owner-only public-safe metadata for future web-client reads. Those mode fields
do not drive access, notification, retention, sharing, viewer, or key-custody behavior.
Account/device recipient-key, trusted-contact relationship, contact public-key,
sharing-grant, and wrapped-key metadata is implemented separately behind
authenticated `/v1` routes. Contact public-key lifecycle routes cover
registration, active/pending verification state, replacement, revocation, and
lost-key marking without storing private keys or decrypting wrapped records.
Mode-driven behavior
boundaries are documented in [incident-modes.md](incident-modes.md), with role
and grant boundaries in [v1-access-control.md](v1-access-control.md) and
contact key-sharing boundaries in
[contact-key-sharing-grants.md](contact-key-sharing-grants.md).

## Package Layout

- `go.mod`: defines the root Go module `github.com/open-proofline/server`.
- `proofline.toml`: safe local-first example TOML config loaded automatically
  when running from the repository root.
- `docker-default-config.toml`: container default TOML config copied into the
  image as `/etc/proofline/proofline.toml`, using `/var/lib/proofline` for
  container state.
- `Dockerfile`: builds the `proofline-server` binary and starts it with
  `--config /etc/proofline/proofline.toml`.
- `.github/workflows/ci.yml`: runs Go tests with a coverage signal on pull requests and pushes, runs `govulncheck`, builds the `proofline-server-linux-amd64` binary artifact, gates release binary attestation and trusted GHCR publishing on the vulnerability scan, uploads the binary as a GitHub Release asset on `v*` tag pushes, builds the Docker image, and publishes attested images to GitHub Container Registry from a trusted job limited to `main`, `develop`, and `v*` tag pushes.
- `.dockerignore`: excludes local runtime, review, and build artifacts from the root Docker build context used by `Dockerfile`.
- `cmd/api`: starts one main API/viewer HTTP server per main bind address and one private-admin HTTP server per admin bind address, loads config, enforces the local account bootstrap gate, checks the selected coordination backend, opens the selected metadata backend, creates storage, wires shared handlers including main API, private admin JSON API, public viewer rate limiting, upload coordination, and the private `/admin` dashboard, starts the deletion worker, and handles graceful shutdown.
- `cmd/stream-ingress`: starts a separate regional relay listener with
  `GET /health/live`, `GET /health/ready`,
  `POST /upload/complete-chunk`, and `GET /fanout/subscribe`. It has
  command-local
  `SAFE_STREAM_INGRESS_*` environment/flag settings for private bind address,
  optional relay identity/region labels, readiness behavior, core API URL,
  relay-to-core service token, temp staging directory, upload size/staging
  quota, upstream timeout, and in-flight upload limits. It can send
  near-live/unconfirmed encrypted fanout chunks and bounded confirmed,
  rejected, or terminal-failure state after core commit outcomes. Readiness
  reports only bounded aggregate categories for manual ready state, core
  forwarding configuration, upload readiness, and temp-staging pressure. It
  does not
  mount `/v1`, `/admin`, public viewer routes, bundle/deletion routes, metrics,
  operator routes, durable relay storage, durable relay coordination,
  decryption, or raw-key behavior.
- `cmd/simclient`: simulates future client flows by logging in, creating an incident, creating a media stream, encrypting and uploading complete chunks, completing or failing streams, sending periodic checkins, and optionally testing hash-failure retry, bundle download, local decrypt verification, durable desktop-recorder staging, local file input, ffmpeg segment capture, restart/resume behavior, and poor-network retry controls. Token-bearing viewer URLs are omitted from simulator output.
- `internal/config`: reads TOML config files, `SAFE_*` environment overrides,
  and `SAFE_*_FILE` secret files for backend selectors, backend-specific
  settings, main and private-admin bind address lists, legacy singular bind
  addresses, data directory, database path, max upload size, upload
  coordination lease TTL, main API and public viewer rate limits, account
  registration and SMTP email settings, optional web-auth cookie/CORS/CSRF
  settings, optional WebAuthn RP/origin policy, relay capability issuance
  settings, relay-to-core service auth settings, HTTP server timeouts, local
  account bootstrap secret, session TTL, deletion worker interval,
  closed-incident retention window, token metadata retention window, and
  tombstone retention window.
- `internal/coordination`: defines the small optional coordination boundary, the default no-coordination backend, and the Valkey/Redis-compatible startup check, main API/public viewer rate-limit counter backend, and short-lived complete-upload lease backend.
- `internal/db`: opens SQLite, enables foreign keys and WAL mode, applies embedded SQLite migrations, records `schema_migrations`, and runs named compatibility migrations.
- `internal/email`: defines the outbound email sender boundary and the SMTP-backed verification email implementation. The backend has no stdout/file development mailer and does not send notification, recovery, billing, or trusted-contact emails.
- `internal/envelope/pq`: implements the accepted PQ evidence envelope used by
  default upload validation and simulator flows, including ML-KEM-768 wrapping
  records, payload frame parsing, public metadata validation, and local test
  helpers.
- `internal/envelope`: implements the explicit v1 compatibility AES-256-GCM
  client-side chunk envelope, associated data builder, and local simulator key
  file helpers.
- `internal/auth`: normalizes local account usernames and email addresses, validates passwords, hashes passwords with bcrypt, hashes opaque session or verification tokens before storage, defines controlled second-factor recovery action/reason values, and maps WebAuthn user and credential records to the go-webauthn library types.
- `internal/httpapi`: owns separate main and private-admin muxes, JSON responses, request logging, recovery, local account/session authentication, request validation, upload handling, stream state handlers, relay session capability issuance, service-authenticated core relay preflight/commit/fanout authorization handlers, trusted-contact relationship handlers, contact public-key handlers, sharing-grant handlers, wrapped-key handlers, incident deletion handlers, ZIP bundle streaming, app-level main API and public viewer rate limiting, private admin JSON API routes including second-factor recovery reset, the private admin web surface, the incident viewer, and the narrow metadata repository boundary consumed by handlers. Logging changes in this package should follow [logging-requirements.md](logging-requirements.md).
- `internal/relaycap`: signs and validates short-lived regional relay upload
  and fanout capability tokens with HMAC-SHA256, explicit expiry, role checks,
  relay session binding, and incident/stream binding. It does not implement
  relay upload, staging, core commit, fanout transport, decryption, key custody,
  or service identity.
- `internal/incidents`: defines incident/stream/chunk/checkin/account/session/deletion/trusted-contact-relationship/contact-key/sharing-grant/wrapped-key/WebAuthn models and provides the SQLite metadata repository implementation, including deletion decisions, tombstones, retry item state, trusted-contact relationship records, contact public-key records, sharing-grant records, wrapped-key records, WebAuthn user/credential/challenge records, second-factor recovery audit events, and write guards for deleting incidents.
- `internal/postgresdb`: opens optional PostgreSQL metadata connections, applies PostgreSQL migrations, and implements the metadata repository behavior, including second-factor recovery audit events, with PostgreSQL transaction, row-locking, deletion, and constraint semantics.
- `internal/retention`: runs the background deletion and optional closed-incident retention worker. It claims retryable deletion decisions, removes encrypted blobs through the storage boundary using stored paths snapshotted from metadata, records safe retry state, prunes sensitive child metadata after blob deletion, and logs only non-sensitive counts or error categories under the standard logging requirements.
- `internal/storage`: defines the blob-store boundary used by HTTP handlers and provides local filesystem and optional S3-compatible implementations, including temp uploads, hashing while streaming, server-controlled stored paths, and immutable final commits.
- `migrations`: embeds the SQLite schema.
- `migrations/postgres`: embeds the PostgreSQL schema.
- `compose`: contains local Docker Compose smoke-test stacks and a runner script
  for disposable SQLite/local, PostgreSQL/local, SQLite/S3-compatible MinIO, and
  full PostgreSQL/MinIO/Valkey validation. The full stack exercises TOML config
  loading and fake secret-file mounts. These files are local release-smoke
  helpers, not production deployment manifests.

The implemented `cmd/stream-ingress` package is a temporary relay edge. The
core API can issue configured short-lived relay upload and fanout capabilities
for authorized open streams and exposes service-authenticated relay
preflight/commit/fanout authorization handlers. The relay listener can accept
configured complete encrypted chunk uploads, stage ciphertext temporarily,
verify declared hashes, forward exact encrypted bytes to the core API, and send
optimistic encrypted `near_live_unconfirmed` SSE events to authorized
subscribers followed by bounded `confirmed`, `rejected`, or
`terminal_failure` state after the core commit outcome. Its readiness output is
safe aggregate status only and does not expose labels, URLs, credentials,
paths, counts, or per-user state. Later relay work should
keep the relay listener separate and let the core API remain authoritative for
authorization, idempotency, durable blob commits, and metadata.

## Main Request Flow

Main `/v1` routes require `Authorization: Bearer <session_token>` except for
login, disabled-by-default registration, and email verification; authenticated
setup routes also include email second-factor challenge, TOTP setup or session
verification, and disabled-by-default WebAuthn registration/assertion
ceremonies.
Authenticated routes can also use a browser session cookie when
`SAFE_WEB_AUTH_ENABLED=true` and no bearer token is present. Browser login uses
`POST /v1/auth/web/login`,
returns safe session metadata without a raw token, and sets an HttpOnly session cookie.
`GET /v1/auth/web/csrf` returns a session-bound CSRF token for unsafe
cookie-authenticated requests, and `POST /v1/auth/web/logout` revokes the
session and clears the cookie. Requests that send both bearer and browser
cookie credentials are rejected. Existing `/v1/admin/...` JSON routes are
mounted on the private-admin handler and require an admin account. First-admin
bootstrap uses the private
`/admin/bootstrap` form when no admin exists and a bootstrap secret is
configured.
Session tokens are opaque, returned only to the client, and stored as hashes by
the metadata repository. `POST /v1/auth/register` is controlled by
`SAFE_ACCOUNT_REGISTRATION_MODE`: disabled and admin-only modes reject public
registration, open mode creates pending email-verification accounts and sends
SMTP verification messages, and paid mode fails closed without creating active
accounts. `POST /v1/auth/email/verify` consumes single-use token hashes and
activates pending accounts without creating a session.

The current listener split does not mount `/v1/health/live` or
`/v1/health/ready` on either listener.

Incidents are created in `internal/httpapi.createIncident`, which calls
`CreateIncidentForAccount` on the configured metadata repository and records the
authenticated account as the owner. `GET /v1/incidents` and
`GET /v1/incidents/{incident_id}` return only public-safe metadata for incidents
owned by the authenticated account; cross-account, deleted, and legacy unowned
incidents use the same not-found shape. Other incident routes retain their
documented owner or owner/admin authorization policies. Private-admin legacy
unowned incident review and reassignment routes are handled by
`internal/httpapi` on the admin handler and call repository methods that list
safe count-oriented candidates or record one `assign_owner`/`keep_unowned`
audit event. Assignment updates only active incidents whose `owner_account_id`
is still empty; quarantine records audit metadata without granting regular-user
access. See
[legacy unowned incident reassignment](legacy-unowned-incident-reassignment.md).

Chunks are uploaded through `POST /v1/incidents/{incident_id}/chunks`, handled by `internal/httpapi.uploadChunk`.

Upload handling first checks that the incident exists and is open. The file is then streamed by `internal/httpapi.readChunkUpload` into `internal/storage.BlobStore.SaveTemp`, which writes to the configured local temp staging directory while computing SHA-256 and enforcing the upload byte limit and temp staging quota.

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

There is no implemented capture stream group, variant-role, source-timeline
supersession, or canonical evidence resolution layer in this codebase. The
future planning boundary for that model is documented in
[capture-stream-variants.md](capture-stream-variants.md).

Stream completion is handled by `internal/httpapi.completeMediaStream`. Before a stream moves from `open` to `complete`, the handler verifies that chunks `1..expected_chunk_count` exist contiguously for that stream and that each stored blob can be opened from the configured blob store. `internal/incidents.Repository.CompleteMediaStream` then revalidates the chunk rows in the completion transaction before committing the state change. Failed streams preserve uploaded chunks but are not offered as normal downloads.

Trusted-contact relationships are handled by
`POST/GET /v1/trusted-contact-relationships`,
`GET /v1/trusted-contact-relationships/{relationship_id}`, and the
`accept`, `decline`, `revoke`, and `replace` transition routes in
`internal/httpapi/trusted_contact_relationship_handlers.go`. The handlers use
the authenticated owner account for invite/revoke/replace actions and the
authenticated recipient account for accept/decline actions. The records carry
identity and lifecycle metadata only; they do not deliver wrapped keys by
themselves, notifications, plaintext, or public viewer privileges.

Contact public-key registration and lifecycle transitions are handled by
`POST /v1/contact-public-keys`,
`GET/PATCH /v1/contact-public-keys/{public_key_id}`,
`POST /v1/contact-public-keys/{public_key_id}/revoke`,
`POST /v1/contact-public-keys/{public_key_id}/lost`, and
`POST /v1/contact-public-keys/{public_key_id}/replace` in
`internal/httpapi/sharing_handlers.go`. The handlers use the authenticated
local account as the owner scope and store only public-key metadata through the
configured metadata repository. Optional `recipient_account_id` bindings
connect contact public-key records to signed-in trusted-contact delivery
authorization. They reject unknown JSON fields and do not accept contact
private keys, raw media keys, wrapped media keys, plaintext, or browser
fragment secrets.

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
wrapped-key lookup/revocation routes, plus read-only
`GET /v1/trusted-contact/incidents/{incident_id}/wrapped-keys` and
`GET /v1/trusted-contact/wrapped-keys/{wrapped_key_id}`. Creation is
owner-only, checks that the incident is active, optionally checks stream
ownership, requires an active grant for the same owner and incident, requires
ciphertext access, and requires the bound contact public key to remain active.
Owner list/read responses are owner scoped. Trusted-contact list/read responses
require the authenticated recipient account to match a bound active contact
key, active accepted relationship, active unexpired ciphertext grant, and
active wrapped-key record. Delivery filters omit revoked or expired grants,
inactive contact keys, and revoked or rotated wrapped-key records. These routes
deliver encrypted wrapped-key metadata through authenticated API responses only;
public viewer routes and bundle manifests remain key-free.

Private sharing audit events are stored by the SQLite and PostgreSQL metadata
repositories when contact-key, sharing-grant, wrapped-key, or deletion-pruning
lifecycle changes occur. They are repository metadata only, not a public route
or dashboard surface, and store controlled IDs, actions, outcomes, and
timestamps without tokens, request bodies, raw keys, wrapped-key ciphertext,
public wrapping metadata, paths, object keys, plaintext, or user safety
narratives.

## Deletion And Retention Flow

Main owner-scoped deletion requests are handled by
`POST /v1/incidents/{incident_id}/deletion`. Admin-global deletion requests are
handled by `POST /v1/admin/incidents/{incident_id}/deletion` on the private-admin
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
namespace. When no admin account exists and a bootstrap secret is configured,
it renders a first-admin bootstrap screen. After an admin exists,
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

`GET /i/{token}` is mounted on the main API/viewer listener. It renders `internal/httpapi/web/templates/incident_viewer.html` with `html/template`. CSS and JavaScript are embedded from `internal/httpapi/web/static`. `GET /i/{token}/data` returns the same read-only summary as JSON for polling. `GET /i/{token}/viewer-payload` returns the narrower token-scoped payload intended for the future web-client viewer. Pre-rename `/e/{token}` viewer, data, and download paths remain read-only aliases only while explicit local/test compatibility needs them; future canonical no-account viewer links should point at the web-client origin as documented in `docs/web-client-viewer-routing.md`.

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
- account/device recipient-key metadata
- trusted-contact relationship, contact public-key, sharing-grant, and wrapped-key metadata
- backend deployment docs
- backend security, retention, and threat-model docs
- simulator/reference backend flow
- planning docs for future decryption clients

For v1 preview terminology, repository roles, and current-versus-future product
direction, read [v1 preview direction](v1-preview-direction.md) before turning
prototype gaps into backlog or implementation assumptions.

Before broad public exposure, review route groups and add:

- the current authenticated `/v1` access-control design and separately bound
  private admin API boundary in [v1-access-control.md](v1-access-control.md),
  including which route groups are intended for a public edge
- the target main API/public viewer and private admin listener split
  in [public-api-listener-split.md](public-api-listener-split.md)
- edge rate limits, existing app-level main API and public viewer limits, and
  broader abuse controls
- TLS and reverse-proxy settings for the public incident viewer, if reachable over a network
- deployment-specific enforcement of the documented [retention, backup, and deletion policy](retention-backup-deletion.md)
- cluster backup, restore, and failure drills for optional PostgreSQL metadata,
  S3-compatible encrypted blobs, and Valkey/Redis-compatible coordination as
  documented in
  [cluster-backup-restore-runbook.md](cluster-backup-restore-runbook.md)
- operational monitoring for failed uploads and storage/DB errors
- a production review of viewer-token sharing, expiry defaults, and revocation operations
- mode-driven access, escalation, retention, sharing, viewer, key-custody,
  trusted-contact, public product API, payment-gated registration, account
  recovery, and broader admin/operator authorization
  design before expanding broad public `/v1`
  exposure

## Out Of Scope Today

The repository does not currently include the web client, iOS app, Android app,
protocol repository, production local recording client, mode-driven access,
escalation, retention, viewer behavior, trusted-contact incident delivery,
wrapped-key delivery outside authenticated owner or trusted-contact private
`/v1` routes, dead-man switch notifications, production client key storage,
browser/client-side decryption, server-assisted break-glass key access, payment
processing, subscriptions, checkout sessions, billing webhooks, password
recovery, playable media export,
push notifications, SMS, Messenger integration, OAuth, JWT, public account
portal, or a public admin dashboard. The local
desktop-recorder behavior in `cmd/simclient` is simulator/reference flow only.
