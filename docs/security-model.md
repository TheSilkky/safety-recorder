# Security Model

This document summarizes the current Proofline backend security assumptions and controls. For a threat-oriented view, see [threat-model.md](threat-model.md). For planned incident-mode behavior, see [incident-modes.md](incident-modes.md). For `/v1` role and grant boundaries, see [v1-access-control.md](v1-access-control.md). For future production key custody and emergency access design, see [key-custody.md](key-custody.md), the contact key-sharing and wrapped-key grant design in [contact-key-sharing-grants.md](contact-key-sharing-grants.md), the simulator-only wrapped-key metadata prototype in [contact-wrapped-key-metadata-simulator.md](contact-wrapped-key-metadata-simulator.md), [browser-decryption.md](browser-decryption.md), [live-partial-stream-access-boundary.md](live-partial-stream-access-boundary.md), [regional-stream-ingress-relay.md](regional-stream-ingress-relay.md), and [break-glass-key-access.md](break-glass-key-access.md). For vulnerability reporting, see [../SECURITY.md](../SECURITY.md).

## Maturity

Proofline is experimental and not production-ready public infrastructure. The main `/v1` API has local username/password accounts and opaque server-side sessions. It still has no OAuth, no JWT protection, no complete public product API hardening, and no public account portal.

The current backend stores incidents owned by local accounts. Incidents are
generic by default and may include optional incident-mode, capture-profile,
escalation-policy, and sharing-state metadata. Those fields are not behavior
flags and do not grant access, send notifications, change retention, change key
custody, expose trusted-contact workflows, or change public viewer and bundle
behavior. The backend implements account-owner contact public-key metadata and
owner-scoped sharing-grant records and wrapped-key records for owned incidents,
but it does not yet implement trusted-contact accounts, dead-man switch
notifications, mode-driven sharing, browser decryption, backend decryption, or
public account-based product access.

The `/v1` access-control direction is documented in
[v1-access-control.md](v1-access-control.md). The current implementation covers
local account sessions, owner-scoped incident access, owner-scoped contact
public-key, sharing-grant, and wrapped-key metadata routes, admin account
routes, and route authentication. It does not make `/v1` safe to expose publicly as a
product API. Existing `/v1/admin/...` JSON routes are authenticated admin-only
routes on the main handler and must not be routed from public entry points. The
current topology separates the main API/viewer listener from a separately bound
private `/admin` dashboard listener; see
[public-api-listener-split.md](public-api-listener-split.md).

## Listener Boundary

The API binary starts separate listener groups:

| Listener group | Routes | Intended exposure |
|---|---|---|
| Main API and viewer | Authenticated `/v1/...` routes, existing admin-only JSON APIs, `/i/{token}` and related read-only routes, plus pre-rename `/e/{token}` compatibility aliases | Reviewed main API deployment boundary; viewer paths may be routed publicly when only viewer paths are forwarded. Public edges must not route `/v1/admin/...`. |
| Private admin dashboard | `/admin`, `/admin/...`, and `/admin/static/...` | Localhost, LAN, WireGuard, firewall, or strict reverse proxy only. |

The `/admin` dashboard must not be mounted on the main listener. Incident
viewer routes are read-only.

## Account And Token Handling

Local accounts are stored in the configured metadata backend. Passwords are
stored as bcrypt password hashes, not plaintext. Session tokens are opaque
server-side bearer credentials. The raw session token is returned only by
login; the metadata backend stores only a SHA-256 hash. Sessions expire after
`SAFE_SESSION_TTL`, defaulting to 12 hours, and can be revoked by logout,
account password change, admin password reset, or admin session revocation.

The server fails closed on startup unless an admin account exists or
`SAFE_AUTH_BOOTSTRAP_SECRET` is set for the one-time private `/admin`
bootstrap form. The bootstrap form is disabled once an admin account exists. Treat the bootstrap
secret, account passwords, session tokens, and Authorization headers as
secrets.

The current listener split does not mount `/v1/health/live` or
`/v1/health/ready` on either listener. Avoid publishing operator readiness
details on the main API/viewer origin or on the dashboard-only private-admin
listener.

The private `/admin` page, login form, bootstrap form, and account password
workflows are mounted only on the private-admin mux, not on the main API/viewer
mux. The browser flow uses the same server-side session store as `/v1`
authentication and stores the raw session token in an HttpOnly SameSite=Strict
cookie scoped to `/admin`. Authenticated `/admin` pages require the `admin`
role. Authenticated state-changing admin web forms use a session-bound CSRF
token. The token-neutral CSS under `/admin/static/...` is unauthenticated
because it is public source code and contains no incident data, secrets, tokens,
keys, or deployment details. This does not add a public admin dashboard or
public product API exposure model.

Incident viewer tokens are scoped to one incident. The raw token is returned only at creation time; the configured metadata backend stores only a SHA-256 hash. Tokens created without an explicit `expires_at` default to a 24-hour lifetime unless `SAFE_DEFAULT_INCIDENT_TOKEN_TTL` is configured differently. Expired, revoked, and invalid tokens return the same public error.

Viewer URLs contain bearer tokens and should be treated as secrets. Reverse proxies and operational logs should avoid recording raw `/i/{token}` paths. During upgrades from pre-rename releases, `/e/{token}` compatibility links may also reach the edge proxy and should be redacted.

## Upload And Storage Controls

- Uploads are streamed to a temp directory while SHA-256 is computed.
- Upload file bytes are limited by `SAFE_MAX_UPLOAD_BYTES`.
- Final chunk storage happens only after hash verification.
- Stored chunks are immutable and never overwritten.
- Local storage commits use no-overwrite hard links. Optional S3-compatible storage commits final objects with conditional no-overwrite writes.
- Streamed uploads require positive chunk indexes, while legacy unstreamed uploads may still use index `0`.
- `original_filename` is optional client-supplied display metadata. The server
  strips it to a basename and may return it in authenticated chunk metadata,
  token-scoped public incident viewer summaries, and bundle manifests. Future
  clients should omit it by default or use a generic basename unless preserving
  filename context is an explicit user or protocol decision.
- The simulator can wrap chunks in the documented v1 AES-256-GCM client-side encryption envelope before upload. Desktop-recorder mode can stage encrypted chunks locally and retry complete encrypted uploads without adding server-visible partial upload state.
- The backend validates and stores ciphertext bytes only; it does not store encryption keys or decrypt chunk contents.
- SQLite and optional PostgreSQL metadata enforce media type, chunk index, byte size, SHA-256 shape, foreign keys, and unique chunk identity.
- Complete chunk uploads can include an `Idempotency-Key` header. The backend
  stores only a SHA-256 hash of the key in durable metadata, binds it to the
  normalized chunk identity and immutable request fingerprint, and can return
  `200 OK` with `Idempotency-Replayed: true` for equivalent retries without
  overwriting chunks or evidence metadata.
- When Valkey/Redis-compatible coordination is configured, complete chunk
  uploads use a short-lived server-controlled lease key derived from a hash of
  normalized chunk identity. Busy leases return `409 upload_in_progress` with
  `Retry-After`; coordination failures return a retryable safe error. Lease
  keys and errors do not include raw tokens, raw idempotency keys, request
  bodies, uploaded bytes, stored paths, object keys, plaintext, or raw keys.
- Main API route-class rate limiting is enabled by default for authentication,
  bootstrap, account, incident, upload, reconciliation, stream, token,
  download, and admin API classes. Limiter keys use server-controlled class
  labels and a hash of the socket peer identity. They do not include raw
  session tokens, Authorization headers, raw idempotency keys, request bodies,
  uploaded bytes, incident IDs, stored paths, object keys, plaintext, raw keys,
  or private deployment details.
- The authenticated duplicate chunk reconciliation route compares a requested
  normalized chunk identity and expected immutable fingerprint against accepted
  chunk metadata without re-uploading ciphertext, reading stored bytes, or
  returning stored paths, object keys, uploaded bytes, plaintext, raw keys, raw
  tokens, or conflicting stored values.
- Chunk metadata inserts recheck incident and stream state in the repository so uploads racing with close or completion are rejected.
- Media stream completion verifies contiguous chunks and readable stored files, then rechecks chunk rows transactionally before committing completion.
- Local account authorization binds authenticated incident access to the
  authenticated account, the incident owner, and the role. Current private
  incident routes also pass route-level action and data-class labels, but all
  current incident actions share the same owner-or-admin policy. Regular users
  can access their own incidents. Admins can access incidents across accounts.
  Legacy unowned incidents are admin-only until a future private reassignment
  or quarantine workflow exists; see
  [legacy unowned incident reassignment](legacy-unowned-incident-reassignment.md).
- Contact public-key, sharing-grant, and wrapped-key routes are authenticated
  main `/v1` routes. Contact public-key records are scoped to the authenticated
  account. Sharing-grant and wrapped-key creation, listing, lookup, and
  revocation require the authenticated account to own the incident or record;
  admins do not manage another account's sharing grants or wrapped-key records
  through the product routes unless the admin account also owns that incident.
  New grants require an active contact public key owned by the same account and
  can be scoped to an incident or one stream. Wrapped-key records require an
  active, unexpired grant that authorizes ciphertext access and an active
  contact public key. These routes do not store or return contact private keys,
  raw media keys, plaintext, browser fragment secrets, request bodies, uploaded
  bytes, stored paths, staging paths, object keys, or private deployment
  details.

Optional S3-compatible storage preserves ciphertext-only behavior for committed
encrypted chunks. It uses server-controlled object keys, does not expose object
store URLs in evidence bundles, and does not add backend decryption or key
custody.

Optional PostgreSQL metadata support preserves these controls with equivalent
or stronger constraints, duplicate guards, token-hash storage, and row-locking
transaction boundaries. The implementation and remaining migration limits are
documented in [postgresql-metadata-migration.md](postgresql-metadata-migration.md).

Optional Valkey/Redis-compatible coordination can be configured for
short-lived coordination checks. It is not durable evidence storage, does not
hold incident metadata, viewer-token metadata, committed encrypted bytes,
retention decisions, plaintext, or keys, and does not change the private
`/v1` boundary. Its upload leases are retry hints only; metadata constraints,
upload-operation rows, and blob no-overwrite behavior remain authoritative.

The current HTTP listener split does not expose readiness checks. Future
operator readiness routes should report only coarse metadata, blob, and
coordination backend status; they should not become public diagnostics,
metrics, support dashboards, or evidence-inspection routes.

Cluster-safe upload operation semantics are documented in
[cluster-safe-upload-semantics.md](cluster-safe-upload-semantics.md). The
implemented path is limited to complete-upload idempotency keys plus optional
short-lived Valkey in-progress leases; resumable uploads and partial-upload
lease sessions are still planning-only. The current API still accepts complete
encrypted chunks and retries should resend the complete chunk.
See
[resumable-upload-lease-protocol.md](resumable-upload-lease-protocol.md).

## Bundle Controls

Completed stream and incident bundles are generated on demand as ZIP responses. ZIP entry names are controlled by the server. Manifests are generated from database metadata and do not expose server filesystem paths.

Bundle manifests may include `original_filename` basenames because those values
are current chunk display metadata. They are user/client metadata, not server
stored paths, staging paths, object-storage keys, ZIP entry names, or download
paths.

Incident bundle generation fails closed if any completed stream cannot be reconstructed. It does not silently omit inconsistent completed streams from the ZIP or manifest.

Bundles contain encrypted chunk bytes and JSON manifests only. They are not decrypted, playable, or merged media exports.

Bundle manifests may include a non-secret client-side encryption hint. They do
not include keys. Contact public-key, sharing-grant, and wrapped-key metadata
is implemented in the private authenticated API. Wrapped-key records stay
separate from viewer tokens and ordinary ciphertext bundle access; public-link
viewer bundles remain ciphertext-only unless a later issue explicitly designs
decryption-bearing public links.

## Incident Modes And Escalation Boundary

Incident-mode metadata is currently limited to optional main incident fields.
Emergency incidents, interaction records, safety checks, and evidence notes must
not weaken the current storage, encryption, listener, or logging boundaries. The
schema keeps incident mode, capture profile, escalation policy, and sharing state
separate; see [incident-modes.md](incident-modes.md).

Future escalation policies should keep capture separate from notification and emergency response:

- non-emergency interaction records should not automatically alert trusted contacts by default
- safety checks should alert trusted contacts only after an explicit missed-check-in policy is implemented
- Proofline must not claim to contact emergency services unless a future jurisdiction-specific integration is explicitly designed, implemented, and documented
- sharing, export, publication, and legal submission should remain deliberate user-controlled actions

The current backend does not decide whether an incident is an emergency, does not notify trusted contacts, and does not contact emergency services.

Future incident-mode access must follow the role and grant boundaries in
[v1-access-control.md](v1-access-control.md). Incident labels, capture profiles,
or sharing-state summaries must not silently grant trusted-contact, public-link,
admin/operator, escrow, key, or plaintext access.

## Logging And Headers

Request logging records method, redacted route pattern, status, byte count, and duration. It does not log request bodies, uploaded bytes, Authorization headers, raw session tokens, raw viewer tokens, raw incident tokens, raw idempotency keys, plaintext, or raw keys.

Background deletion maintenance logs only non-sensitive summary counts and safe
error categories. It does not log stored paths, object keys, bucket names,
private endpoints, request bodies, uploaded bytes, plaintext, raw keys, raw
tokens, Authorization headers, or backend error strings.

The Go app sets these headers on public incident viewer pages, JSON responses, static assets, ZIP downloads, and private admin web responses:

- `Content-Security-Policy`
- `X-Content-Type-Options: nosniff`
- `Referrer-Policy: no-referrer`
- `Permissions-Policy: geolocation=(), microphone=(), camera=()`
- `X-Frame-Options: DENY`

Token-protected incident pages, JSON responses, errors, ZIP downloads, and admin web pages also use `Cache-Control: no-store`, including automatic method-mismatch errors on token-bearing paths. Main and private-admin JSON responses use `X-Content-Type-Options: nosniff` and `Cache-Control: no-store`; JSON responses also use `Content-Type: application/json`.

HSTS is not enabled by default in the Go app because local development uses plain HTTP and HSTS should only be sent over HTTPS. Set `Strict-Transport-Security` at the production HTTPS reverse proxy after TLS is established for the public hostname. After deployment, test the public incident viewer with the MDN HTTP Observatory.

HTTP server timeouts are configurable separately for main and private-admin
listener groups. Main read/write timeouts default to disabled for large
uploads/downloads and viewer ZIP downloads; private-admin timeouts are finite
by default and should be coordinated with reverse-proxy timeouts.

The Go app includes app-level public viewer rate limiting by route class for
viewer page lookups, JSON polling, encrypted ZIP download starts, and static
assets. Limiter keys use safe route-class labels and a hash of the socket peer
identity; they do not include raw `/i/{token}` paths, legacy `/e/{token}`
paths, raw tokens, request bodies, Authorization headers, uploaded bytes,
plaintext, raw keys, or private deployment details. Deployment-edge rate
limiting guidance is documented in [deployment.md](deployment.md), and those
proxy controls still do not replace reviewed main `/v1` deployment boundaries,
local account authentication, or future public product API abuse controls.

## Retention, Backups, And Deletion

Retention, backup, restore, secure deletion limits, and disk encryption posture
are documented in
[retention-backup-deletion.md](retention-backup-deletion.md). Incident deletion
and retention enforcement details are documented in
[incident-deletion-retention-enforcement.md](incident-deletion-retention-enforcement.md).
Future mode-aware retention policy is planning-only and documented in
[mode-aware retention policy](mode-aware-retention-policy.md).
The current backend preserves accepted evidence by default, exposes private
owner-scoped and admin-global deletion APIs, and starts a deletion worker by
default. Automatic closed-incident retention is disabled unless
`SAFE_CLOSED_INCIDENT_RETENTION` is configured. Expired/revoked viewer-token
metadata pruning and completed tombstone pruning are disabled unless
`SAFE_TOKEN_METADATA_RETENTION` or `SAFE_DELETION_TOMBSTONE_RETENTION` is
configured.

SQLite WAL file expectations, same-host storage constraints, checkpoint
pressure symptoms, and local file-size checks are documented in
[deployment.md](deployment.md#sqlite-wal-operations).

Cluster backup, restore, and failure-mode guidance for optional PostgreSQL
metadata, S3-compatible encrypted blobs, and Valkey/Redis-compatible
coordination is documented in
[Cluster backup, restore, and failure runbook](cluster-backup-restore-runbook.md).

Normal file or object removal is not treated as guaranteed secure erasure. Deployments that store real incident evidence should use encrypted disks, encrypted volumes, encrypted object buckets, logs, and backups, then rely on explicit backup expiry and encryption-key retirement for stronger deletion outcomes.

## Known Security Gaps

- No implemented public product API exposure model for `/v1`; local account
  sessions are an authenticated main-API control, not a complete public security
  model
- No built-in TLS
- No general-purpose abuse-throttling system beyond main API and public viewer
  route-class rate limiting
- PostgreSQL metadata and Valkey/Redis-compatible coordination are optional
  and experimental; they do not by themselves complete all cluster-safe upload
  semantics or make `/v1` safe for public exposure
- Cluster backup, restore, and failure runbooks are operational guidance only;
  they do not add access control, retention enforcement, observability, abuse
  controls, or production readiness
- No resumable, partial, or leased cluster-safe upload protocol beyond the
  implemented complete-upload `Idempotency-Key` path documented in
  [cluster-safe-upload-semantics.md](cluster-safe-upload-semantics.md)
- No implemented resumable upload or upload lease protocol; the future design
  is planned in
  [resumable-upload-lease-protocol.md](resumable-upload-lease-protocol.md)
- No implemented regional stream-ingress relay; the future design is planned
  in
  [regional-stream-ingress-relay.md](regional-stream-ingress-relay.md)
- No implemented mode-driven access, escalation, retention, key-custody,
  trusted-contact account, dead-man switch notification, browser decryption,
  backend decryption, or public account portal behavior
- No implemented production client key storage, browser decryption, server-assisted break-glass key access, or emergency-contact key access model; the future designs are documented in [key-custody.md](key-custody.md), [contact-key-sharing-grants.md](contact-key-sharing-grants.md), [browser-decryption.md](browser-decryption.md), and [break-glass-key-access.md](break-glass-key-access.md)
- No implemented live or partial stream access beyond current read-only stream
  metadata summaries and completed encrypted bundle downloads; the future
  boundary is documented in
  [live-partial-stream-access-boundary.md](live-partial-stream-access-boundary.md)
- No mode-specific retention, backup lifecycle enforcement, or built-in disk
  encryption; the operational policy is
  documented in [retention-backup-deletion.md](retention-backup-deletion.md),
  with enforcement details in
  [incident-deletion-retention-enforcement.md](incident-deletion-retention-enforcement.md),
  and future policy boundaries in
  [mode-aware retention policy](mode-aware-retention-policy.md)
- No malware/content scanning for uploaded encrypted blobs
- No implemented account self-service recovery, email verification, second
  factor authentication, delegated identity provider, or public account portal
