# API

This is the current backend-only HTTP surface for Proofline. The API binary starts a main API/viewer listener and a private-admin listener on one or more configured bind addresses. Main `/v1` routes require local account authentication except for login. Existing `/v1/admin/...` JSON routes require an admin account and are mounted on the main handler; they are not public-ready routes. The private-admin listener serves only the `/admin` dashboard route tree. Incident viewer routes are token-gated, read-only, and mounted on the main listener. Planned web, iOS, and Android clients are not part of this repository yet.

Media bundle downloads are encrypted chunk bundles. The backend does not decrypt, merge, or produce playable media. The simulator's current encrypted uploads use the envelope documented in [encryption.md](encryption.md), but the API treats uploaded bytes as opaque ciphertext.

The current API stores incidents owned by local accounts. Incidents are generic
by default and may include optional `incident_mode`, `capture_profile`,
`escalation_policy`, and `sharing_state` metadata on the main create/read
routes. Those fields are metadata only: they do not grant access, send
notifications, change key custody, expose trusted-contact workflows, or change
public viewer and bundle behavior. Mode-specific retention behavior is not
implemented; deletion and closed-incident retention enforcement are documented
in [retention-backup-deletion.md](retention-backup-deletion.md). Planned
mode-driven behavior is documented in [incident-modes.md](incident-modes.md).
Authenticated account-owner routes can store trusted-contact public-key
metadata, owner-scoped sharing-grant records, and wrapped media-key metadata
for active grants. Trusted-contact accounts, browser or backend decryption,
public product authentication, notifications, raw key storage, and key escrow
do not exist yet.

Default bind addresses:

- main API and incident viewer listener: `127.0.0.1:8080`
- private admin dashboard listener: `127.0.0.1:8081`

Use `SAFE_MAIN_BIND_ADDRS` and `SAFE_ADMIN_BIND_ADDRS` for comma-separated
bind-address lists. Legacy `SAFE_PRIVATE_BIND_ADDRS` still maps to the main
listener, but legacy `SAFE_PUBLIC_BIND_ADDRS` now fails startup so an old
public viewer bind cannot become the private-admin listener by accident.

## Common Responses

Errors use:

```json
{
  "error": {
    "code": "invalid_json",
    "message": "request body must be valid JSON"
  }
}
```

Non-upload JSON bodies are limited to 64 KiB. Upload file bytes are limited by `SAFE_MAX_UPLOAD_BYTES`; multipart metadata has a small fixed overhead allowance. `SAFE_MAX_UPLOAD_BYTES` accepts a positive byte count or binary unit suffixes `B`, `K`/`KB`, `M`/`MB`, and `G`/`GB`. Fractional unit values are allowed when they resolve to at least one byte. Non-positive, sub-byte, invalid, and oversized values are rejected during startup.

Main API route classes are rate limited by default before authentication using
safe server-controlled keys based on route class and a hash of the socket peer
identity. Rate-limit keys do not include raw session tokens, Authorization
headers, raw idempotency keys, request bodies, uploaded bytes, incident IDs,
stored paths, object keys, plaintext, raw keys, or private deployment details.
Exhausted limits return `429 rate_limited` with `Retry-After`. A configured
coordination limiter failure returns `503 rate_limit_unavailable` with a
generic response. See [configuration](configuration.md) for
`SAFE_MAIN_API_RATE_LIMIT_*` settings.

## Health And Readiness

The current listener split does not mount `/v1/health/live` or
`/v1/health/ready` on either listener. The private-admin listener is a
dashboard-only `/admin` surface, and the main listener must not publish
operator readiness details on the same origin as future public product API
routes. Local and CI smoke checks use token-neutral static assets plus the
admin bootstrap/login flow to prove both listener trees are serving.

## Authentication And Accounts

Private `/v1` routes require:

```http
Authorization: Bearer <session_token>
```

Session tokens are opaque server-side credentials. The raw token is returned only by login, while the metadata backend stores only its SHA-256 hash. Sessions expire after `SAFE_SESSION_TTL`, defaulting to `12h`, and can be revoked by logout, password reset, or the admin session-revocation route.

On startup, the server fails closed unless an admin account already exists or
`SAFE_AUTH_BOOTSTRAP_SECRET` is set. With that secret set, create the first
admin through the private `/admin` bootstrap screen or by posting form fields
to `POST /admin/bootstrap`, then remove the environment variable and restart or
redeploy without it. JSON `POST /v1/bootstrap/admin` is not mounted on either
listener.

### `POST /v1/auth/login`

Authenticates a local account and returns a raw session token once.

Request:

```json
{
  "username": "admin",
  "password": "long local password"
}
```

Response `201`:

```json
{
  "session_id": "ses_...",
  "account": {
    "id": "acct_...",
    "username": "admin",
    "role": "admin",
    "created_at": "2026-05-31T10:00:00Z",
    "updated_at": "2026-05-31T10:00:00Z",
    "password_changed_at": "2026-05-31T10:00:00Z"
  },
  "token": "...",
  "created_at": "2026-05-31T10:00:00Z",
  "expires_at": "2026-05-31T22:00:00Z"
}
```

### `POST /v1/auth/logout`

Revokes the current session.

### `GET /v1/account`

Returns the authenticated account.

### `POST /v1/account/password`

Changes the authenticated account password after verifying `current_password`; other sessions for the account are revoked.

### Private Admin Web Routes

The private-admin listener serves a small admin web surface outside the
`/v1` API namespace:

- `GET /admin`
- `POST /admin/login`
- `POST /admin/bootstrap`
- `POST /admin/logout`
- `POST /admin/password`
- `POST /admin/accounts/{account_id}/password`
- `GET /admin/static/styles.css`

`GET /admin` renders either the first-admin bootstrap form, the admin login
form, or the authenticated admin dashboard. The form handlers reuse the same
local account records and opaque server-side session store as the JSON API, but
the browser flow stores the raw session token in an HttpOnly, SameSite=Strict
cookie scoped to `/admin`.

The bootstrap screen is available only when no admin account exists and
`SAFE_AUTH_BOOTSTRAP_SECRET` is configured. It requires the bootstrap secret,
admin username, and admin password. After an admin exists, `/admin` shows the
login screen and requires an admin account. Non-admin sessions are rejected.

The authenticated dashboard lists local accounts and offers password workflows.
`POST /admin/logout` revokes the current admin web session. `POST
/admin/password` changes the current admin account password after verifying the
current password, then revokes other sessions for that account. `POST
/admin/accounts/{account_id}/password` lets an admin reset another local
account password and revokes all sessions for that account. These authenticated
state-changing forms use a session-bound CSRF token.

`/admin/static/styles.css` is unauthenticated because it is token-neutral static
CSS from the AGPL-licensed source tree. It does not contain incident data,
tokens, deployment details, keys, or evidence metadata. The admin HTML pages
use `Cache-Control: no-store` and conservative browser security headers.

The admin web surface shows only route-boundary status, safe navigation stubs,
and local account-management data. It does not expose incident evidence, viewer
tokens, session tokens, password hashes, request bodies, uploaded bytes,
Authorization headers, plaintext, raw keys, stored paths, object keys, private
deployment details, or sensitive evidence metadata. It is not a public admin
dashboard and must stay on the private-admin listener.

### Admin API Routes

The following routes require an admin account session:

- `GET /v1/admin/accounts`
- `POST /v1/admin/accounts`
- `POST /v1/admin/accounts/{account_id}/password`
- `POST /v1/admin/accounts/{account_id}/sessions/revoke`
- `GET /v1/admin/incidents/{incident_id}/deletion`
- `POST /v1/admin/incidents/{incident_id}/deletion`

`POST /v1/admin/accounts` accepts `username`, `password`, and `role`, where `role` is `user` or `admin`. Admin password reset and explicit session revocation revoke all sessions for the selected account.

These routes are mounted on the main `/v1` handler so the private-admin
listener can remain a dashboard-only `/admin` tree. Local account
authentication, admin-role checks, and app-level rate limiting do not by
themselves make `/v1` production-ready public infrastructure. Expose the main
API only after deployment-specific TLS, path routing, abuse controls, browser
credential rules, CSRF decisions, logging review, and production operations are
explicitly designed and reviewed. Public reverse proxies must not route
`/v1/admin/...` from a public edge. Keep private-admin dashboard listeners
behind localhost, LAN, WireGuard, firewall rules, or a strict private reverse
proxy.

## Contact Public Keys

Contact public-key routes are mounted on the main API listener and require a
valid local account session. They are scoped to the authenticated account only;
admins do not use these routes to manage another account's contact keys. The
server stores public-key metadata, wrapping algorithm names, fingerprints,
state, and optional display labels. It does not store contact private keys, raw
media keys, wrapped media keys, browser fragment secrets, plaintext, request
bodies, uploaded bytes, stored paths, staging paths, object keys, or private
deployment details.

Contact key states are:

| State | Meaning |
|---|---|
| `pending_verification` | Registered but not eligible for new sharing grants. |
| `active` | Eligible for new sharing grants. |
| `replaced` | Superseded by another key version. |
| `revoked` | Revoked and not eligible for future grants. |
| `lost` | Marked lost and not eligible for future grants. |

New sharing grants require an `active` contact public key. The API can register
a new contact by omitting `contact_id`, or rotate an existing contact by
providing an account-owned `contact_id`; rotated keys receive the next version.

### `POST /v1/contact-public-keys`

Registers trusted-contact public-key metadata for the authenticated account.
The optional `key_state` defaults to `pending_verification`.

Request:

```json
{
  "display_label": "Trusted contact",
  "wrapping_algorithm": "age-v1-x25519",
  "public_key": "age1...",
  "public_key_fingerprint": "fingerprint-...",
  "key_state": "pending_verification"
}
```

Response `201`:

```json
{
  "contact_public_key": {
    "public_key_id": "cpk_...",
    "owner_account_id": "acct_...",
    "contact_id": "ctc_...",
    "version": 1,
    "display_label": "Trusted contact",
    "wrapping_algorithm": "age-v1-x25519",
    "public_key": "age1...",
    "public_key_fingerprint": "fingerprint-...",
    "key_state": "pending_verification",
    "created_at": "2026-06-01T10:00:00Z",
    "updated_at": "2026-06-01T10:00:00Z"
  }
}
```

### `GET /v1/contact-public-keys`

Lists contact public-key metadata owned by the authenticated account.

### `GET /v1/contact-public-keys/{public_key_id}`

Returns one account-owned contact public-key record. Records owned by another
account return `404 contact_public_key_not_found`.

### `PATCH /v1/contact-public-keys/{public_key_id}`

Updates mutable contact-key metadata. The request may change `display_label`
and `key_state`. Revoked keys cannot be reactivated.

Request:

```json
{
  "display_label": "Verified contact",
  "key_state": "active"
}
```

### `POST /v1/contact-public-keys/{public_key_id}/revoke`

Marks one account-owned contact public key revoked. Revocation prevents the key
from receiving new sharing grants or future wrapped-key records. It does not
delete already accepted ciphertext, bundle contents, or any material a future
authorized actor may already have downloaded.

## Sharing Grants

Sharing-grant routes are mounted on the main API listener and require a valid
local account session. Grant creation and incident-scoped listing are
account-owner actions: admins are not allowed to manage another account's
sharing grants through these product routes unless the admin account owns the
incident. Public viewer routes remain read-only and do not use these grant
records.

Sharing grants authorize metadata, ciphertext, and wrapped-key delivery
decisions. They do not decrypt media, create trusted-contact sessions, send
notifications, alter incident-mode behavior, or change public viewer and bundle
responses.

Grant data classes are:

| Data class | Meaning |
|---|---|
| `metadata` | Metadata access authorization. |
| `ciphertext` | Encrypted evidence access authorization. |
| `metadata_ciphertext` | Metadata and encrypted evidence access authorization. |

### `POST /v1/incidents/{incident_id}/sharing-grants`

Creates an active sharing-grant record for an incident owned by the
authenticated account. `stream_id` is optional; omit it for incident scope.
The referenced contact must have an active contact public key owned by the same
account. If `contact_public_key_id` is omitted, the latest active key version
for the contact is used. `recipient_type` defaults to `trusted_contact`,
`data_class` defaults to `metadata_ciphertext`, and `expires_at`, when present,
must be in the future.

Request:

```json
{
  "stream_id": "str_...",
  "contact_id": "ctc_...",
  "data_class": "metadata_ciphertext",
  "expires_at": "2026-06-08T10:00:00Z"
}
```

Response `201`:

```json
{
  "sharing_grant": {
    "grant_id": "sgr_...",
    "owner_account_id": "acct_...",
    "incident_id": "inc_...",
    "stream_id": "str_...",
    "recipient_type": "trusted_contact",
    "contact_id": "ctc_...",
    "contact_public_key_id": "cpk_...",
    "contact_public_key_version": 1,
    "data_class": "metadata_ciphertext",
    "grant_state": "active",
    "created_at": "2026-06-01T10:00:00Z",
    "updated_at": "2026-06-01T10:00:00Z",
    "expires_at": "2026-06-08T10:00:00Z"
  }
}
```

Missing incidents, streams, or active contact public keys return
`404 sharing_grant_dependency_not_found` without revealing which dependency was
missing outside the owner boundary.

### `GET /v1/incidents/{incident_id}/sharing-grants`

Lists sharing grants for an incident owned by the authenticated account.

### `GET /v1/sharing-grants/{grant_id}`

Returns one sharing grant owned by the authenticated account.

### `POST /v1/sharing-grants/{grant_id}/revoke`

Marks one account-owned sharing grant revoked and records the revoking account.
Revocation stops future grant-based authorization or delivery. It does not
delete encrypted chunks, incident metadata, bundle contents, or anything an
authorized actor may already have downloaded.

## Wrapped Keys

Wrapped-key routes are mounted on the main API listener and require a valid
local account session. They are account-owner routes: admins are not allowed to
store, list, read, or revoke another account's wrapped-key records through
these product routes unless the admin account owns the incident or record.

The backend stores encrypted media-key material only when it is bound to an
active sharing grant that authorizes ciphertext access. The record includes the
incident, optional stream, media key ID, grant ID, trusted-contact public key
ID and version, wrapping algorithm/version, wrapped-key ciphertext, and public
wrapping metadata. It must not include raw media keys, contact private keys,
plaintext, unwrapped shared secrets, browser fragment secrets, raw tokens, or
server escrow material.

Bundle manifests remain key-free. The current public incident viewer does not
deliver wrapped keys, and public viewer bundle downloads keep their existing
ciphertext-only behavior.

### `POST /v1/incidents/{incident_id}/wrapped-keys`

Stores one wrapped media-key record for an incident owned by the authenticated
account. `grant_id` must name an active owner-scoped sharing grant for the same
incident. The grant must authorize `ciphertext` or `metadata_ciphertext`, must
not be expired, and must point at an active contact public key. If the grant is
stream-scoped, `stream_id` must match the grant's stream.

Request:

```json
{
  "stream_id": "str_...",
  "grant_id": "sgr_...",
  "media_key_id": "media-key-2026-06-01-audio",
  "wrapping_algorithm": "age-v1-x25519",
  "wrapping_algorithm_version": "1",
  "wrapped_key_ciphertext": "base64url-or-profile-encoded-ciphertext",
  "public_wrapping_metadata": {
    "profile": "age-v1-x25519"
  }
}
```

Response `201`:

```json
{
  "wrapped_key": {
    "wrapped_key_id": "wkey_...",
    "owner_account_id": "acct_...",
    "incident_id": "inc_...",
    "stream_id": "str_...",
    "grant_id": "sgr_...",
    "recipient_type": "trusted_contact",
    "contact_id": "ctc_...",
    "contact_public_key_id": "cpk_...",
    "contact_public_key_version": 1,
    "media_key_id": "media-key-2026-06-01-audio",
    "wrapping_algorithm": "age-v1-x25519",
    "wrapping_algorithm_version": "1",
    "wrapped_key_ciphertext": "base64url-or-profile-encoded-ciphertext",
    "public_wrapping_metadata": {
      "profile": "age-v1-x25519"
    },
    "wrapped_key_state": "active",
    "created_at": "2026-06-01T10:00:00Z",
    "updated_at": "2026-06-01T10:00:00Z"
  }
}
```

Missing incidents, streams, active grants, or active contact public keys return
`404 wrapped_key_dependency_not_found`. Metadata-only grants return
`409 wrapped_key_grant_not_authorized`. Reusing the same owner, incident,
stream scope, grant, media key ID, and contact public key returns
`409 wrapped_key_duplicate`.

### `GET /v1/incidents/{incident_id}/wrapped-keys`

Lists active wrapped-key records for an incident owned by the authenticated
account. The list is delivery-filtered: records are omitted when their grant is
revoked or expired, their contact public key is no longer active, or the
wrapped-key record itself is revoked or rotated.

### `GET /v1/wrapped-keys/{wrapped_key_id}`

Returns one active wrapped-key record owned by the authenticated account,
subject to the same delivery filter as the incident list route.

### `POST /v1/wrapped-keys/{wrapped_key_id}/revoke`

Marks one account-owned wrapped-key record revoked and records the revoking
account. Revocation stops future delivery of that wrapped-key record. It does
not delete encrypted chunks, incident metadata, bundle contents, or material an
authorized actor may already have downloaded.

## Incidents

Incident routes are mounted on the main API listener and require a valid session. Incidents are owned by the account that creates them. Regular users can access only their own incidents; admins can access incidents across accounts through the main product route set. Legacy unowned incidents are admin-only until a future private reassignment or quarantine workflow is implemented; see [legacy unowned incident reassignment](legacy-unowned-incident-reassignment.md).

### `POST /v1/incidents`

Creates an open incident. When mode fields are omitted, the incident remains a
generic legacy incident. The request may include optional mode metadata, but
these fields do not grant access, create public links, send notifications,
change retention, change key custody, expose trusted-contact workflows, or change
public viewer and bundle behavior.

Request:

```json
{
  "client_label": "iphone",
  "notes": "test incident",
  "incident_mode": "interaction_record",
  "capture_profile": "audio_location",
  "escalation_policy": "none",
  "sharing_state": "private"
}
```

Optional mode values:

| Field | Accepted values |
|---|---|
| `incident_mode` | `emergency`, `interaction_record`, `safety_check`, `evidence_note` |
| `capture_profile` | `audio_video_location`, `audio_location`, `location_checkin`, `note_or_attachment`, `custom` |
| `escalation_policy` | `none`, `trusted_contacts_on_start`, `trusted_contacts_on_missed_checkin`, `urgent_trusted_contact_alert` |
| `sharing_state` | `private`, `trusted_contact_access`, `public_link_created`, `legal_export_created`, `revoked_or_expired` |

Response `201`:

```json
{
  "incident_id": "inc_...",
  "status": "open",
  "incident_mode": "interaction_record",
  "capture_profile": "audio_location",
  "escalation_policy": "none",
  "sharing_state": "private"
}
```

### `GET /v1/incidents/{incident_id}`

Returns incident metadata, chunk metadata, and checkins. Chunk file bytes are not included.

Response `200`:

```json
{
  "incident": {
    "id": "inc_...",
    "created_at": "2026-05-21T10:00:00Z",
    "updated_at": "2026-05-21T10:00:00Z",
    "status": "open",
    "client_label": "iphone",
    "incident_mode": "interaction_record",
    "capture_profile": "audio_location",
    "escalation_policy": "none",
    "sharing_state": "private",
    "deletion_state": "active"
  },
  "streams": [],
  "chunks": [],
  "checkins": []
}
```

### `POST /v1/incidents/{incident_id}/close`

Marks an incident closed. Later chunk uploads return `409 incident_closed`.

Response `200` is the updated incident object. If the incident has
optional mode metadata, the same fields shown in the `GET` incident object can
be present. Closing an incident does not change sharing, retention, viewer,
notification, or key-custody behavior.

### `POST /v1/incidents/{incident_id}/deletion`

Requests deletion for an incident owned by the authenticated account. Admins
can use this route only for incidents they own; use the admin route below for
global deletion. The route creates durable deletion state and snapshots
server-controlled stored paths from metadata before any blob is deleted. It is
mounted only on the main API listener.

Request:

```json
{
  "reason_code": "account_delete",
  "allow_open": true
}
```

`reason_code` is optional and must be a short non-sensitive code using letters,
digits, `_`, `-`, `.`, or `:`. It must not contain raw tokens, request bodies,
evidence notes, private deployment details, plaintext, raw keys, stored paths,
object keys, or user safety narrative. Open incidents are rejected unless
`allow_open` is true. Repeating a deletion request for the same incident returns
the existing deletion decision.

Response `202`:

```json
{
  "deletion": {
    "decision_id": "del_...",
    "incident_id": "inc_...",
    "source": "account_request",
    "reason_code": "account_delete",
    "actor_account_id": "acct_...",
    "allow_open": true,
    "state": "deletion_pending",
    "item_count": 2,
    "requested_at": "2026-05-31T10:00:00Z",
    "updated_at": "2026-05-31T10:00:00Z"
  }
}
```

### `GET /v1/incidents/{incident_id}/deletion`

Returns the non-sensitive deletion status for an incident visible to the
authenticated account.

### `POST /v1/admin/incidents/{incident_id}/deletion`

Requests deletion for any incident visible to an admin account. The request and
response shape match the account route, but the `source` is `admin_request`.

### `GET /v1/admin/incidents/{incident_id}/deletion`

Returns the non-sensitive deletion status for any incident by ID. This route
requires an admin account.

Deletion states are:

| State | Meaning |
|---|---|
| `active` | Incident is not being deleted. |
| `deletion_pending` | A deletion decision exists and blob deletion items have been prepared. |
| `deleting` | The background worker is deleting encrypted blobs and metadata. |
| `deletion_failed` | Blob deletion failed and the deletion is retryable. |
| `deleted` | Encrypted blobs and sensitive child metadata have been removed or confirmed absent. |

While an incident is not `active`, write routes, bundle routes, chunk upload,
and new incident-token creation fail closed. Public viewer token lookups for
the incident return the same `404 incident_token_invalid` shape used for
invalid, expired, or revoked tokens and do not reveal deletion state.

## Chunks

Chunk routes are mounted on the main API listener.

### `POST /v1/incidents/{incident_id}/chunks`

Uploads one already-encrypted chunk as `multipart/form-data`.

Optional header:

- `Idempotency-Key`: stable key for this intended complete chunk upload. The
  value must be 1-255 visible ASCII characters. The server treats it as
  token-like: raw values are not logged, returned in errors, or stored raw.

Fields:

- `file`: encrypted chunk bytes
- `stream_id`: optional media stream ID for new clients
- `chunk_index`: non-negative integer for legacy unstreamed uploads; positive integer when `stream_id` is provided
- `media_type`: `audio`, `video`, `location`, or `metadata`
- `started_at`: RFC3339 timestamp
- `ended_at`: RFC3339 timestamp, not before `started_at`
- `sha256_hex`: lowercase SHA-256 hex of the encrypted bytes
- `original_filename`: optional client-supplied display metadata

Response `201`:

```json
{
  "id": "chk_...",
  "incident_id": "inc_...",
  "stream_id": "str_...",
  "chunk_index": 1,
  "media_type": "audio",
  "started_at": "2026-05-21T10:00:00Z",
  "ended_at": "2026-05-21T10:00:10Z",
  "original_filename": "chunk.enc",
  "stored_path": "incidents/inc_.../streams/str_.../audio_000001.enc",
  "byte_size": 23,
  "sha256_hex": "...",
  "created_at": "2026-05-21T10:00:11Z"
}
```

When `stream_id` is provided, the stream must exist, belong to the same incident, be open, and have the same `media_type` as the uploaded chunk. Streamed chunks must use indexes starting at `1`; `chunk_index <= 0` returns `400 invalid_chunk_index`. Uploads to completed or failed streams return `409 stream_not_open`.

New clients should create a media stream and upload chunks with `stream_id`. `stream_id` remains optional for backwards compatibility with existing chunks and clients. Legacy unstreamed chunks may use `chunk_index = 0`; they are still stored and listed, but they are not included in completed-stream bundle downloads.

Streamed chunk identity is `(incident_id, stream_id, chunk_index)`, so each stream can use normal stream-local chunk numbering. Legacy unstreamed chunk identity remains `(incident_id, media_type, chunk_index)` for chunks without `stream_id`.

Duplicate streamed `(incident_id, stream_id, chunk_index)` uploads and duplicate legacy `(incident_id, media_type, chunk_index)` uploads without an idempotency key return `409 duplicate_chunk`. Hash mismatches return `400 hash_mismatch` and do not commit a final file.

When `Idempotency-Key` is supplied, the server hashes the key and stores durable
upload-operation state in the configured metadata backend. The key is bound to
the `upload_chunk` operation and a request fingerprint covering normalized
chunk identity, `media_type`, `started_at`, `ended_at`, normalized
`original_filename`, ciphertext byte size, and ciphertext `sha256_hex`.

The first successful idempotent upload returns the normal `201` chunk response.
An equivalent retry with the same key and same complete encrypted chunk upload
can return `200 OK` with the same chunk metadata shape and:

```http
Idempotency-Replayed: true
```

Reusing the same `Idempotency-Key` with a different chunk identity, metadata
fingerprint, byte size, or ciphertext hash returns `409 idempotency_conflict`.
The conflict response is intentionally small and does not include uploaded
bytes, stored paths, object keys, raw keys, tokens, raw idempotency keys, or
private deployment details. Replays still upload the complete encrypted chunk;
this is not a resumable upload or partial-commit protocol.

When `SAFE_COORDINATION_BACKEND=valkey` or `redis` is configured, complete
chunk uploads also acquire a short-lived server-controlled coordination lease
for the normalized chunk identity after the server has read and validated the
multipart upload. If another API node is already processing that chunk
identity, the route returns a retryable response:

```json
{
  "error": {
    "code": "upload_in_progress",
    "message": "upload for this chunk identity is already in progress"
  }
}
```

The response status is `409 Conflict` and includes `Retry-After` based on the
configured `SAFE_UPLOAD_COORDINATION_LEASE_TTL`. If configured coordination is
unavailable during upload, the route returns `503 upload_coordination_unavailable`
with a safe `Retry-After` hint. Valkey lease keys are hashes of server-normalized
chunk identity; they do not contain raw tokens, raw idempotency keys, request
bodies, uploaded bytes, stored paths, object keys, plaintext, or raw keys.
These leases are in-progress hints only. Metadata uniqueness constraints,
upload-operation rows, and blob no-overwrite behavior remain final truth.

The repository rechecks incident and stream state when chunk metadata is inserted. If an upload races with incident close or stream completion, the final metadata insert is rejected and the committed blob path is removed.

For clients using the v1 encryption envelope, `sha256_hex` is the SHA-256 of the complete uploaded envelope bytes, not the plaintext.

`original_filename` is metadata, not a storage path. The server trims the value,
normalizes slash and backslash separators to a basename, falls back to the
multipart upload filename when the explicit field is empty, and stores the
resulting basename with chunk metadata. The value may be returned by private
chunk metadata routes, token-scoped public incident viewer summaries, and
completed stream or incident bundle manifests. Server stored paths, staging
paths, local filesystem paths, and object-storage keys are separate
server-controlled values and are not derived from `original_filename`.

Future clients should omit `original_filename` by default or send a generic,
non-identifying basename unless the user or a future protocol mode explicitly
chooses to preserve filename context as evidence metadata. Filenames can still
contain personal or contextual information even after path stripping. Do not use
`original_filename` for identity, authorization, storage lookup, decryption,
legal-record guarantees, or download path construction.

The current API does not implement resumable uploads, partial-upload lease
sessions, or client-side queue summary endpoints. Clients should retry
complete encrypted chunks, use `Idempotency-Key` for ambiguous complete-upload
outcomes, and use the duplicate chunk reconciliation route when they need to
compare a duplicate accepted chunk with a local expected fingerprint. The
resumable-upload planning decision is documented in
[resumable-upload-lease-protocol.md](resumable-upload-lease-protocol.md).

The current API does not implement a regional stream-ingress relay or
service-authenticated relay preflight/commit endpoints. The future relay design
is documented in [regional-stream-ingress-relay.md](regional-stream-ingress-relay.md).
Any implementation must keep the core API authoritative for authorization,
idempotency, final blob commits, and metadata, and must not expose the full
`/v1` control plane or admin routes through the relay.

### `POST /v1/incidents/{incident_id}/chunks/reconcile`

Reconciles a duplicate chunk identity against already accepted metadata without
re-uploading ciphertext. The route is mounted on the main API listener.

This is a separate private query workflow, not a public route and not an
enriched `409 duplicate_chunk` upload response. A separate route lets clients
compare expected metadata without re-uploading ciphertext, keeps duplicate
upload errors small, and coexists with the current idempotency-key
retry-success path.

Request:

```json
{
  "stream_id": "str_...",
  "chunk_index": 1,
  "media_type": "audio",
  "started_at": "2026-05-21T10:00:00Z",
  "ended_at": "2026-05-21T10:00:10Z",
  "byte_size": 23,
  "sha256_hex": "...",
  "original_filename": "chunk.enc"
}
```

For streamed chunks, `stream_id` is required and identity is
`(incident_id, stream_id, chunk_index)`. `media_type` remains required and must
match the stream media type. For legacy unstreamed chunks, omit `stream_id`;
identity is `(incident_id, media_type, chunk_index)`, and `chunk_index = 0`
remains valid for compatibility.

The comparison fingerprint is:

- normalized chunk identity
- `media_type`
- `started_at`
- `ended_at`
- normalized `original_filename`, including empty value
- ciphertext `byte_size`
- ciphertext `sha256_hex`

The route allows reconciliation after an incident is closed or a stream is
complete or failed, because it is read-only and only confirms already accepted
metadata. It does not overwrite, replace, delete, rewrite, or re-commit stored
chunks.

Matched response `200`:

```json
{
  "reconciliation": {
    "status": "matched",
    "identity": {
      "incident_id": "inc_...",
      "stream_id": "str_...",
      "chunk_index": 1,
      "media_type": "audio"
    },
    "chunk_id": "chk_...",
    "byte_size": 23,
    "sha256_hex": "...",
    "started_at": "2026-05-21T10:00:00Z",
    "ended_at": "2026-05-21T10:00:10Z",
    "created_at": "2026-05-21T10:00:11Z"
  }
}
```

Conflict response `409`:

```json
{
  "error": {
    "code": "duplicate_chunk_conflict",
    "message": "existing chunk does not match expected ciphertext or metadata"
  },
  "reconciliation": {
    "status": "conflict",
    "identity": {
      "incident_id": "inc_...",
      "stream_id": "str_...",
      "chunk_index": 1,
      "media_type": "audio"
    },
    "mismatched_fields": ["sha256_hex", "byte_size"]
  }
}
```

The conflict response should identify mismatched field names, not the existing
stored values. If no accepted chunk exists for the requested identity, return
`404 chunk_not_found`. Invalid identity or fingerprint fields should reuse the
existing upload validation error codes where practical, such as
`400 invalid_chunk_index`, `400 invalid_media_type`, or
`400 invalid_sha256_hex`. Invalid or missing `byte_size` returns
`400 invalid_byte_size`.

Safe reconciliation responses may return server-generated chunk ID, normalized
identity fields, timestamps, byte size, ciphertext hash, creation time, and
field names that matched or mismatched. They must not return uploaded bytes,
plaintext, raw keys, raw tokens, request bodies, local filesystem paths,
`stored_path`, staging paths, object-storage keys, or object-storage
credentials.

HTTP coverage in `internal/httpapi/uploads_test.go` includes:

- matched streamed duplicate reconciliation
- conflicting streamed duplicate reconciliation
- matched and conflicting legacy unstreamed reconciliation
- omission of `stored_path` and stored conflicting values from reconciliation
  responses
- read-only reconciliation after stream completion, stream failure, or incident
  close

### `GET /v1/incidents/{incident_id}/chunks`

Lists chunk metadata for one incident.

### `GET /v1/incidents/{incident_id}/chunks/{media_type}/{chunk_index}`

Returns encrypted bytes for a legacy unstreamed chunk as `application/octet-stream`. This route is private/dev-only and is not used by the incident viewer. Streamed chunks are read through completed stream bundle downloads rather than this legacy media/index route.

## Media Streams

Media stream routes are mounted on the main API listener.

### `POST /v1/incidents/{incident_id}/streams`

Creates an open media stream for an incident.

Request:

```json
{
  "media_type": "audio",
  "label": "main audio recording"
}
```

Response `201`:

```json
{
  "stream": {
    "id": "str_...",
    "incident_id": "inc_...",
    "media_type": "audio",
    "label": "main audio recording",
    "status": "open",
    "created_at": "2026-05-21T10:00:00Z",
    "updated_at": "2026-05-21T10:00:00Z"
  }
}
```

Invalid media types return `400 invalid_media_type`.

### `GET /v1/incidents/{incident_id}/streams`

Lists media streams for an incident.

Response `200`:

```json
{
  "streams": []
}
```

### `GET /v1/incidents/{incident_id}/streams/{stream_id}`

Returns one stream as:

```json
{
  "stream": {
    "id": "str_...",
    "incident_id": "inc_...",
    "media_type": "audio",
    "status": "open",
    "created_at": "2026-05-21T10:00:00Z",
    "updated_at": "2026-05-21T10:00:00Z"
  }
}
```

### `POST /v1/incidents/{incident_id}/streams/{stream_id}/complete`

Marks an open stream complete after verifying chunks `1..expected_chunk_count` exist contiguously and each stored file is readable. Completion revalidates chunk rows in the repository before committing the state change.

Request:

```json
{
  "expected_chunk_count": 12
}
```

Response `200`:

```json
{
  "stream": {
    "id": "str_...",
    "incident_id": "inc_...",
    "media_type": "audio",
    "status": "complete",
    "expected_chunk_count": 12,
    "completed_at": "2026-05-21T10:02:00Z",
    "created_at": "2026-05-21T10:00:00Z",
    "updated_at": "2026-05-21T10:02:00Z"
  }
}
```

Missing or non-contiguous chunks return `409 stream_chunks_incomplete` or `409 stream_chunks_not_contiguous`. Completing an already complete or failed stream returns `409`.

### `POST /v1/incidents/{incident_id}/streams/{stream_id}/fail`

Marks an open stream failed while preserving uploaded chunks.

Request:

```json
{
  "failure_reason": "client stopped recording unexpectedly"
}
```

Response `200` is the updated stream object with `status: "failed"` and `failed_at` set.

### `GET /v1/incidents/{incident_id}/streams/{stream_id}/download`

Downloads a completed stream as a ZIP bundle. Open or failed streams return `409 stream_not_complete`.

Successful responses include:

```http
Content-Type: application/zip
Content-Disposition: attachment; filename="incident_inc_..._audio_str_....zip"
Content-Security-Policy: default-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'; object-src 'none'
X-Content-Type-Options: nosniff
Cache-Control: no-store
Referrer-Policy: no-referrer
Permissions-Policy: geolocation=(), microphone=(), camera=()
X-Frame-Options: DENY
```

ZIP contents:

```text
manifest.json
chunks/audio_000001.enc
chunks/audio_000002.enc
```

The manifest is generated from trusted database metadata and includes incident
ID, stream ID, media type, status, chunk count, total bytes, chunk SHA-256
metadata, and any stored `original_filename` basename for each chunk. Server
filesystem paths are not included.
It also includes a non-secret `encryption` hint indicating expected client-side encryption and `server_decrypts: false`.

Future live or partial stream access is planning-only and should not be inferred
from this completed bundle route. See
[live-partial-stream-access-boundary.md](live-partial-stream-access-boundary.md).

### `GET /v1/incidents/{incident_id}/download`

Downloads a ZIP bundle containing all completed streams for an incident:

```text
manifest.json
streams/{stream_id}/manifest.json
streams/{stream_id}/chunks/audio_000001.enc
```

Open, failed, and legacy unstreamed chunks are omitted from this initial bundle format.

If any completed stream cannot be reconstructed, the incident bundle request fails with `409 incident_bundle_inconsistent` rather than returning a partial bundle. The error response does not include server filesystem paths, stored chunk paths, or ZIP entry names.

## Checkins

Checkin routes are mounted on the main API listener.

### `POST /v1/incidents/{incident_id}/checkins`

Adds optional device status and location metadata.

Request:

```json
{
  "device_battery_percent": 82,
  "device_network": "wifi",
  "latitude": -37,
  "longitude": 145,
  "accuracy_meters": 20
}
```

Response `201` is the created checkin.

## Viewer Tokens

Incident-token creation and revocation routes are mounted on the main API listener.

### `POST /v1/incidents/{incident_id}/incident-tokens`

Creates a read-only viewer token for one incident. The raw token is returned only in this response; the configured metadata backend stores only a SHA-256 hash.

`expires_at` is optional. When omitted, the API applies the configured default token lifetime, which is 24 hours unless `SAFE_DEFAULT_INCIDENT_TOKEN_TTL` is changed. Explicit `expires_at` values are preserved; send `null` to explicitly create a token that remains valid until revoked. Setting `SAFE_DEFAULT_INCIDENT_TOKEN_TTL=0` disables the default and lets omitted expiries remain valid until revoked.

Request:

```json
{
  "label": "trusted contact",
  "expires_at": "2030-01-01T00:00:00Z"
}
```

Response `201`:

```json
{
  "token_id": "itk_...",
  "incident_id": "inc_...",
  "token": "...",
  "label": "trusted contact",
  "created_at": "2026-05-21T10:00:00Z",
  "expires_at": "2030-01-01T00:00:00Z"
}
```

The response includes `Cache-Control: no-store`.

### `POST /v1/incident-tokens/{token_id}/revoke`

Revokes a viewer token by ID.

Response `200`:

```json
{
  "token_id": "itk_...",
  "revoked": true
}
```

## Incident Viewer

Incident viewer routes are mounted on the main API/viewer listener.
`/i/{token}` is the canonical path for new links. The pre-rename `/e/{token}`
paths remain as compatibility aliases for already shared viewer URLs, including
the `/data`, stream download, and incident download variants.

### `GET /i/{token}`

Renders a read-only HTML summary for a valid, unexpired, unrevoked token. The page includes embedded static CSS/JS files served from `/static/`.

### `GET /i/{token}/data`

Returns the same read-only summary as JSON for polling.

Response `200`:

```json
{
  "incident": {
    "id": "inc_...",
    "status": "open",
    "client_label": "iphone",
    "created_at": "2026-05-21T10:00:00Z",
    "updated_at": "2026-05-21T10:00:00Z"
  },
  "latest_checkin": null,
  "chunk_count_by_media_type": {},
  "latest_chunk_by_media_type": {},
  "media": [
    {
      "media_type": "audio",
      "chunk_count": 0
    },
    {
      "media_type": "video",
      "chunk_count": 0
    },
    {
      "media_type": "location",
      "chunk_count": 0
    },
    {
      "media_type": "metadata",
      "chunk_count": 0
    }
  ],
  "streams": [],
  "completed_streams": [],
  "warning": "If you are concerned about immediate safety, call emergency services now.",
  "generated_at": "2026-05-21T10:00:12Z"
}
```

Incident viewer responses include `Referrer-Policy: no-referrer`, `X-Content-Type-Options: nosniff`, `Permissions-Policy: geolocation=(), microphone=(), camera=()`, `X-Frame-Options: DENY`, and a strict `Content-Security-Policy` with `frame-ancestors 'none'`. Token-protected pages, JSON, errors, and downloads include `Cache-Control: no-store`. Invalid, expired, and revoked tokens all return `404 incident_token_invalid`. App-level public viewer rate limits return `429 rate_limited` with a safe JSON error body and `Retry-After`; limiter backend failures return `503 rate_limit_unavailable`.

The Go app does not set `Strict-Transport-Security` in local/dev HTTP mode. Set HSTS at the HTTPS reverse proxy or deployment edge for production hostnames.

### `GET /i/{token}/streams/{stream_id}/download`

Downloads a completed stream bundle for the token's incident. The route is read-only and never accepts a client-provided file path. Invalid, expired, and revoked tokens return `404 incident_token_invalid`.

Open and failed streams are visible only as metadata in the current viewer
summary. The current token-scoped viewer does not expose live chunk bytes or
partial stream manifests. See
[live-partial-stream-access-boundary.md](live-partial-stream-access-boundary.md).

### `GET /i/{token}/incident/download`

Downloads all completed streams for the token's incident as one encrypted evidence ZIP. Failed/open streams and legacy unstreamed chunks are omitted.

If any completed stream cannot be reconstructed, the incident bundle request fails with `409 incident_bundle_inconsistent` rather than returning a partial bundle. Invalid, expired, and revoked tokens still return `404 incident_token_invalid`.
