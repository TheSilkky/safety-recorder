# Security Model

This document summarizes the current Proofline backend security assumptions and controls. For a threat-oriented view, see [threat-model.md](threat-model.md). For planned incident-mode behavior, see [incident-modes.md](incident-modes.md). For `/v1` role and grant boundaries, see [v1-access-control.md](v1-access-control.md). For future production key custody and emergency access design, see [key-custody.md](key-custody.md), the contact key-sharing and wrapped-key grant design in [contact-key-sharing-grants.md](contact-key-sharing-grants.md), the simulator-only wrapped-key metadata prototype in [contact-wrapped-key-metadata-simulator.md](contact-wrapped-key-metadata-simulator.md), [browser-decryption.md](browser-decryption.md), [live-partial-stream-access-boundary.md](live-partial-stream-access-boundary.md), [encrypted-location-context.md](encrypted-location-context.md), [regional-stream-ingress-relay.md](regional-stream-ingress-relay.md), and [break-glass-key-access.md](break-glass-key-access.md). For vulnerability reporting, see [../SECURITY.md](../SECURITY.md).

## Maturity

Proofline is experimental and not production-ready public infrastructure. The main `/v1` API has local username/password accounts, opaque server-side sessions, and app-level route-class rate limits. Public self-registration is disabled by default and, when explicitly enabled for self-hosted deployments, requires SMTP-backed email verification before login. Proofline still has no OAuth, no JWT protection, no complete public product deployment model, no password recovery, and no public account portal.

The current backend stores incidents owned by local accounts. Incidents are
generic by default and may include optional incident-mode, capture-profile,
escalation-policy, and sharing-state metadata. Those fields are not behavior
flags and do not grant access, send notifications, change retention, change key
custody, expose trusted-contact workflows, or change public viewer and bundle
behavior. The backend implements account/device recipient public-key metadata,
account-to-account trusted-contact relationship metadata, account-owner contact
public-key metadata, and owner-scoped sharing-grant records and wrapped-key
records for owned incidents. It also implements read-only signed-in
trusted-contact wrapped-key delivery when an accepted relationship,
recipient-bound active contact key, active unexpired ciphertext grant, and
active wrapped-key record all authorize the request. It does not yet implement
account/device wrapped-key delivery, trusted-contact incident reads,
dead-man-switch notifications, mode-driven sharing, browser decryption, backend
decryption, or public account-based product access beyond the narrow owner
incident metadata list/detail read surface for the future web client.

The `/v1` access-control direction is documented in
[v1-access-control.md](v1-access-control.md). The current implementation covers
local account sessions, owner-scoped incident access, owner-scoped account/device
recipient-key metadata, trusted-contact relationship metadata, contact
public-key metadata, sharing-grant metadata, and wrapped-key metadata routes,
admin account routes, route authentication, and route-class limits. It does not
by itself approve broad public `/v1` routing as a product API. The implemented
account incident list/detail routes are owner-only and public-safe, but uploads,
chunk reads, bundle downloads, diagnostics, operator routes, write routes, and
key-custody behavior still need separate review before public exposure.
Existing `/v1/admin/...` JSON routes are
authenticated admin-only routes on the private-admin listener and must not be
routed from public entry points. The current topology separates the main
API/viewer listener from a separately bound private admin listener; see
[public-api-listener-split.md](public-api-listener-split.md).

## Listener Boundary

The API binary starts separate listener groups:

| Listener group | Routes | Intended exposure |
|---|---|---|
| Main API and viewer | Authenticated non-admin `/v1/...` routes, `/i/{token}` and related read-only routes, plus pre-rename `/e/{token}` compatibility aliases | Reviewed main API deployment boundary; viewer paths may be routed publicly when only viewer paths are forwarded. Public edges must not route `/v1/admin/...`. |
| Private admin listener | Authenticated admin-only `/v1/admin/...` JSON routes, `/admin`, `/admin/...`, and `/admin/static/...` | Localhost, LAN, WireGuard, firewall, or strict reverse proxy only. |

The `/admin` dashboard must not be mounted on the main listener. Incident
viewer routes are read-only.

## Account And Token Handling

Local accounts are stored in the configured metadata backend. Passwords are
stored as bcrypt password hashes, not plaintext. Session tokens are opaque
server-side credentials. The raw bearer session token is returned only by
`POST /v1/auth/login`; the optional browser login route sets the raw token only
as an HttpOnly session cookie and does not return it in JSON. The metadata
backend stores only a SHA-256 hash. Sessions expire after `SAFE_SESSION_TTL`,
defaulting to 12 hours, and can be revoked by logout, account password change,
admin password reset, or admin session revocation.

Open account registration creates `pending_email_verification` accounts and
sends a verification email only when `SAFE_ACCOUNT_REGISTRATION_MODE=open`, an
SMTP backend, and `SAFE_PUBLIC_WEB_ORIGIN` are configured. Verification tokens
are opaque single-use credentials. The raw token is sent only in the email link
fragment and accepted only in the verification request body; metadata stores
only a SHA-256 hash, purpose, expiry, and consumed timestamp. Pending accounts
cannot authenticate until verification activates them. The paid registration
mode is a fail-closed placeholder and does not create an active account.

When enabled, main `/v1` browser cookie auth uses a dedicated session cookie
for future web-client calls. Bearer auth remains supported for CLI, simulator,
and API clients. If bearer and browser-cookie credentials are sent together,
the request is rejected as ambiguous. Cookie-authenticated unsafe requests
require a session-bound HMAC CSRF token in the configured header; bearer
requests do not require this CSRF header. Credentialed CORS is emitted only for
exact configured web origins and never with a wildcard origin.

The server fails closed on startup unless an admin account exists or
`SAFE_AUTH_BOOTSTRAP_SECRET` or `SAFE_AUTH_BOOTSTRAP_SECRET_FILE` is set for
the one-time private `/admin` bootstrap form. The bootstrap form is disabled
once an admin account exists. Treat the bootstrap secret, account passwords,
session tokens, and Authorization headers as secrets.

The server can load configuration from built-in defaults, TOML files,
`SAFE_*` environment variables, and selected `SAFE_*_FILE` secret files. Secret
files are read once at startup for the bootstrap secret, PostgreSQL DSN, S3
credentials, Valkey password, and SMTP password. Startup errors for missing,
empty, or conflicting secret files are categorized as configuration errors and
must not log secret values, secret file contents, request bodies, tokens,
plaintext, raw keys, or Authorization headers.

The current listener split does not mount `/v1/health/live` or
`/v1/health/ready` on either listener. Avoid publishing operator readiness
details on the main API/viewer origin or on the private-admin listener.

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
- Streamed uploads require positive chunk indexes. New uploads must pass the
  accepted PQ payload-frame validation for the stream-bound identity; legacy
  unstreamed upload attempts fail closed under the v1 preview default.
- `original_filename` is optional client-supplied display metadata. The server
  strips it to a basename and may return it in authenticated chunk metadata,
  token-scoped public incident viewer summaries, and bundle manifests. Future
  clients should omit it by default or use a generic basename unless preserving
  filename context is an explicit user or protocol decision.
- The simulator wraps chunks in the accepted PQ client-side envelope by default.
  Desktop-recorder mode can stage encrypted chunks locally and retry complete
  encrypted uploads without adding server-visible partial upload state. The old
  v1 AES-GCM envelope remains an explicit simulator compatibility mode.
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
  public registration, email verification, bootstrap, account metadata,
  account/device recipient-key metadata, trusted-contact relationship metadata,
  contact-key metadata, incident metadata, sharing-grant metadata,
  wrapped-key metadata, upload,
  reconciliation, stream, token, and download classes. The legacy admin API
  limit setting is retained only as a documented compatibility setting because
  current `/v1/admin/...` JSON routes are on the private-admin listener. Limiter
  keys use server-controlled class labels and a hash of the socket peer
  identity. They do not include raw email addresses, raw usernames,
  verification tokens, raw session tokens, Authorization headers, raw
  idempotency keys, request bodies, uploaded bytes, incident IDs, stored paths,
  object keys, plaintext, raw keys, wrapped-key ciphertext, or private
  deployment details.
- The authenticated duplicate chunk reconciliation route compares a requested
  normalized chunk identity and expected immutable fingerprint against accepted
  chunk metadata without re-uploading ciphertext, reading stored bytes, or
  returning stored paths, object keys, uploaded bytes, plaintext, raw keys, raw
  tokens, or conflicting stored values.
- Chunk metadata inserts recheck incident and stream state in the repository so uploads racing with close or completion are rejected.
- Media stream completion verifies contiguous chunks and readable stored files, then rechecks chunk rows transactionally before committing completion.
- Future capture stream groups, variant roles, source-timeline matching, and
  evidence supersession are planning-only in
  [capture-stream-variants.md](capture-stream-variants.md). The current backend
  does not select canonical evidence across variants, and a future supersession
  decision must not overwrite immutable chunks or delete the only
  backend-confirmed evidence for a source time range.
- Local account authorization binds authenticated incident access to the
  authenticated account, the incident owner, and the role. Account incident
  list/detail reads are owner-only and return public-safe metadata only; admins
  do not get cross-account reads through those product routes unless the admin
  account also owns the incident. Other current private incident routes still
  pass route-level action and data-class labels and use the established
  owner-or-admin policy where documented. Legacy unowned incidents are hidden
  from account list/detail reads unless an admin uses the private-admin
  reassignment API to assign one incident to an existing account; see
  [legacy unowned incident reassignment](legacy-unowned-incident-reassignment.md).
- The private-admin legacy unowned incident review route returns only
  count-oriented candidate metadata. The reassignment route records controlled
  audit fields for `assign_owner` or `keep_unowned` decisions and rejects
  free-form notes, already-owned incidents, and non-active deletion states.
  Reassignment changes only private owner-scoped access; public viewer routes,
  token hashes, bundles, deletion state, retention state, encrypted blobs, and
  key custody remain unchanged.
- Account/device recipient-key, trusted-contact relationship, contact
  public-key, sharing-grant, and wrapped-key routes are authenticated main
  `/v1` routes. Account/device recipient-key, trusted-contact relationship, and
  contact public-key records are scoped to the authenticated account.
  Trusted-contact relationships record owner account, recipient account, role,
  state, timestamps, and revocation or replacement metadata only. The owner can
  create, revoke, or replace relationships; the recipient can accept or decline
  an invite. A viewer-token holder does not become a trusted contact by opening
  a link. Relationship state does not grant raw keys, wrapped keys, plaintext,
  notification delivery, emergency dispatch, or public viewer privileges.
  Account/device recipient-key records store only public key material,
  non-secret key IDs, scheme/suite identifiers, fingerprints, state,
  timestamps, and optional display labels. Contact public-key records store
  trusted-contact public key material, wrapping algorithm names, fingerprints,
  version, state, timestamps, replacement links, and optional display labels.
  Revoked, replaced, and lost account/device or contact keys are terminal for
  future wrapping eligibility; those state changes do not rewrite old
  wrapped-key records, delete ciphertext, or revoke material already downloaded
  by a future authorized client. Sharing-grant and wrapped-key creation, listing,
  lookup, and revocation require the authenticated account to own the incident
  or record; admins do not manage another account's sharing grants or
  wrapped-key records through the product routes unless the admin account also
  owns that incident. Read-only trusted-contact wrapped-key routes require the
  authenticated recipient account to match the contact public-key
  `recipient_account_id`, have an active accepted relationship with the owner,
  and satisfy the active grant and active record filters. New grants require an
  active contact public key owned by the same account and can be scoped to an
  incident or one stream. Current wrapped-key records remain trusted-contact
  grant scoped and require an active, unexpired grant that authorizes ciphertext
  access and an active contact public key. Wrapped-key record creation validates
  the accepted PQ wrapping
  profile and public metadata without unwrapping CEKs. In the future key model,
  wrapped-key records connect recipient public-key versions to CEKs scoped to
  incidents, streams, or bounded chunk groups; current `media_key_id` fields
  remain compatibility identifiers for that CEK. These routes do not store or
  return recipient private keys, raw CEKs, raw media keys, ML-KEM shared
  secrets, derived KEKs, plaintext, decrypted caches, browser fragment secrets,
  request bodies, uploaded bytes, stored paths, staging paths, object keys,
  private deployment details, or server escrow material. The metadata backends
  also store private sharing-audit events for contact-key, sharing-grant,
  wrapped-key, and deletion-pruning lifecycle changes using controlled IDs,
  action names, outcome categories, and timestamps only. Those audit events do
  not include tokens, request bodies, uploaded bytes, plaintext, raw keys,
  wrapped-key ciphertext, public wrapping metadata, stored paths, object keys,
  private deployment details, or user safety narratives.

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

Configuration files and secret-file references are deployment inputs, not
incident evidence. The committed example TOML and Compose smoke secret files
contain only local placeholder values. Real deployments should mount reviewed
configuration and secret files outside public source history and keep raw
credentials out of logs and support artifacts.

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

Future canonical bundle or export manifests may use capture stream variant
evidence resolution only after a separate manifest design is accepted. Until
then, completed bundle behavior remains stream-based and does not delete
near-live, audio-priority, or failed-stream fallback evidence.

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

## Location Privacy Boundary

Full-fidelity GPS, speed, heading, route history, and freshness context are
high-sensitivity user safety data. Future location context should be Class A
encrypted evidence by default, bound to chunks, streams, source segments, or
bounded chunk groups as documented in
[encrypted-location-context.md](encrypted-location-context.md). The current
backend does not implement encrypted location sidecars, live tracking, browser
decryption, trusted-contact incident reads, or map-provider backend
integration.

Basic no-account token viewers must not receive full routes, chunk-by-chunk GPS
samples, speed histories, heading histories, or live tracking claims by default.
Signed-in trusted-contact access must remain account-authenticated and
grant-scoped rather than inferred from a bearer viewer link.

Future implementation tests should cover token-viewer field allowlists,
redaction assertions, location-context envelope or authenticated metadata
binding, relay/logging omissions, and indistinguishable invalid, expired, and
revoked token behavior.

## Logging And Headers

Logging requirements, standard safe fields, raw-error restrictions, and test
expectations are documented in
[logging-requirements.md](logging-requirements.md).

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

- No complete public product API deployment model for `/v1`; local account
  sessions, optional browser cookie sessions, app-level route-class limits, and
  the narrow owner incident metadata list/detail reads are authenticated
  main-API controls, not a complete public deployment model
- No built-in TLS
- No deployment-edge abuse-throttling system beyond app-level main API and
  public viewer route-class rate limiting
- PostgreSQL metadata and Valkey/Redis-compatible coordination are optional
  and experimental; they do not by themselves complete all cluster-safe upload
  semantics or make every `/v1` route safe for broad public exposure
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
  trusted-contact incident delivery, dead-man switch notification, browser
  decryption, backend decryption, payment-gated registration, password
  recovery, or public account portal behavior
- No implemented production client key storage, browser decryption, server-assisted break-glass key access, or emergency-contact key access model; the future designs are documented in [key-custody.md](key-custody.md), [contact-key-sharing-grants.md](contact-key-sharing-grants.md), [browser-decryption.md](browser-decryption.md), and [break-glass-key-access.md](break-glass-key-access.md)
- No implemented production browser decryption or client key-custody UX.
  Current wrapped-key metadata routes validate and store encrypted PQ key
  metadata only and do not introduce recipient private-key custody, raw CEK
  storage, or decryption. A production trusted-contact browser decrypt path
  requires the static/signed or native/offline trust gate documented in
  [browser-decryption.md](browser-decryption.md).
- No implemented live or partial stream access beyond current read-only stream
  metadata summaries and completed encrypted bundle downloads; the future
  boundary is documented in
  [live-partial-stream-access-boundary.md](live-partial-stream-access-boundary.md)
- No implemented capture stream group, variant-role, source-timeline
  supersession, or canonical evidence resolution behavior beyond the current
  concrete media stream model; the future design is documented in
  [capture-stream-variants.md](capture-stream-variants.md)
- No mode-specific retention, backup lifecycle enforcement, or built-in disk
  encryption; the operational policy is
  documented in [retention-backup-deletion.md](retention-backup-deletion.md),
  with enforcement details in
  [incident-deletion-retention-enforcement.md](incident-deletion-retention-enforcement.md),
  and future policy boundaries in
  [mode-aware retention policy](mode-aware-retention-policy.md)
- No malware/content scanning for uploaded encrypted blobs
- No implemented account self-service recovery, second factor authentication,
  delegated identity provider, or public account portal. The only implemented
  email delivery path is SMTP-backed registration email verification when open
  registration is explicitly enabled.
