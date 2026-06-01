# Threat Model

This document describes the current Proofline backend-only security posture. It is intentionally conservative and does not claim production readiness.

The backend can store optional incident-mode, capture-profile,
escalation-policy, and sharing-state metadata for emergency incidents,
non-emergency interaction records, timed safety checks, and evidence notes. Those
fields are metadata only. Current controls apply to local accounts, opaque
sessions, generic or mode-labeled incidents, encrypted chunk uploads, checkins,
viewer tokens, contact public-key metadata, owner-scoped sharing-grant
metadata, grant-bound wrapped-key metadata, and encrypted evidence bundles.

## Assets

- Already-encrypted uploaded chunk files under `SAFE_DATA_DIR` for local storage, or committed encrypted objects in the configured S3-compatible bucket
- Incident, media stream, chunk, checkin, and viewer/incident-token metadata in SQLite by default or optional PostgreSQL
- Optional chunk `original_filename` display metadata. The server strips it to a
  basename, but it can still contain user-supplied contextual or personal
  information and may appear in viewer summaries and bundle manifests.
- Optional PostgreSQL metadata schema, migration, transaction, test, and
  restore expectations are documented in
  [postgresql-metadata-migration.md](postgresql-metadata-migration.md)
- Optional incident-mode, capture-profile, escalation-policy, and sharing-state
  metadata stored with incidents. These fields are server-visible metadata but
  do not grant access, send notifications, change retention, change key custody,
  expose trusted-contact workflows, or change public viewer and bundle behavior.
- Trusted-contact public-key metadata, owner-scoped sharing-grant records, and
  grant-bound wrapped-key records in SQLite by default or optional PostgreSQL.
  Wrapped-key records contain encrypted media-key material and public wrapping
  metadata, which is access-enabling metadata. These records do not contain
  contact private keys, raw media keys, plaintext, browser fragment secrets, or
  server-decryptable key material.
- Optional Valkey/Redis-compatible coordination is startup-checked when
  explicitly configured, but it is short-lived coordination state only and is
  not durable evidence storage. It can hold route-class counters and
  complete-upload lease hints, not committed evidence truth.
- The current listener split does not expose `/v1` health/readiness routes on
  either listener
- Complete chunk upload idempotency is implemented with hashed
  `Idempotency-Key` metadata, equivalent retry success, and conflict detection;
  remaining cluster-safe upload semantics and cleanup expectations are
  documented in
  [cluster-safe-upload-semantics.md](cluster-safe-upload-semantics.md)
- Future resumable upload and partial-upload lease behavior is planned but not
  implemented; the local desktop recorder simulator uses complete encrypted
  chunk retries as documented in
  [resumable-upload-lease-protocol.md](resumable-upload-lease-protocol.md)
- Cluster backup, restore, and failure-mode guidance for optional PostgreSQL
  metadata, S3-compatible encrypted blobs, and Valkey/Redis-compatible
  coordination is documented in
  [cluster-backup-restore-runbook.md](cluster-backup-restore-runbook.md)
- On-demand encrypted evidence ZIP bundles generated from completed streams
- Local account records, bcrypt password hashes, and opaque session-token hashes
- Raw session tokens returned once by login and then presented in Authorization
  headers
- Raw viewer/incident tokens returned once at creation time
- Incident viewer URLs containing bearer tokens
- Simulator-only local encryption key files when developers opt into `--key-file`
- Future mobile/web recordings, interaction-record metadata, safety-check
  state, trusted-contact accounts, production client-side keys, browser
  decryption, and break-glass key access are out of scope for the current
  implementation. Planned incident modes are documented
  in [incident-modes.md](incident-modes.md), role and grant boundaries are
  documented in [v1-access-control.md](v1-access-control.md), the intended
  future key custody direction is documented in [key-custody.md](key-custody.md),
  contact public-key lifecycle, trusted-contact grants, and wrapped-key
  metadata are described in
  [contact-key-sharing-grants.md](contact-key-sharing-grants.md),
  the simulator-only contact-wrapped key metadata prototype is documented in
  [contact-wrapped-key-metadata-simulator.md](contact-wrapped-key-metadata-simulator.md),
  browser decryption constraints are documented in
  [browser-decryption.md](browser-decryption.md), and server-assisted access
  design is documented in [break-glass-key-access.md](break-glass-key-access.md).

## Trust Boundaries

- The main API/viewer server binds separately from the private-admin server. By
  default it listens on `127.0.0.1:8080`, and it can listen on multiple
  addresses through `SAFE_MAIN_BIND_ADDRS`.
- The private-admin server binds separately from the main API/viewer server. By
  default it listens on `127.0.0.1:8081`, and it can listen on multiple
  addresses through `SAFE_ADMIN_BIND_ADDRS`.
- Main `/v1` routes are authenticated product and admin JSON routes except for
  `/v1/auth/login`. Authenticated product routes can create incidents, create
  streams, upload chunks, complete/fail streams, close incidents, create viewer
  tokens, revoke tokens, manage account-owned contact public keys, manage
  owner-scoped sharing grants, manage grant-bound wrapped-key records, and read
  encrypted bytes. Existing
  `/v1/admin/...` JSON routes require an admin account and must not be routed
  from public entry points. They are mounted on the main API/viewer server.
- `/v1/bootstrap/admin`, `/v1/health/live`, and `/v1/health/ready` are not
  mounted on either listener.
- `/admin`, `/admin/login`, `/admin/bootstrap`, `/admin/logout`,
  `/admin/password`, and `/admin/accounts/{account_id}/password` are private
  admin web routes. They use the same server-side session store as `/v1`
  authentication, require the admin role after login, and are mounted only on
  the private-admin server. The token-neutral `/admin/static/...` CSS route is
  unauthenticated.
- `/i/{token}`, `/i/{token}/data`, and viewer bundle download routes are
  public-shaped read-only routes gated by a bearer token. Pre-rename
  `/e/{token}` viewer, data, and download paths remain as compatibility
  aliases. These routes are mounted on the main API/viewer server.
- Static assets under `/static/` are embedded and token-neutral.

## Current Controls

- Uploads stream to `data/tmp` while computing SHA-256 and enforcing `SAFE_MAX_UPLOAD_BYTES`.
- Upload-limit configuration rejects non-positive, sub-byte, invalid, and oversized values before request-size limits are applied.
- Uploaded bytes are committed only after hash verification.
- Final local chunk storage uses no-overwrite hard links. Optional S3-compatible storage uses conditional no-overwrite final object writes.
- The simulator encrypts generated test bytes, local pre-recorded files, or
  optional ffmpeg test segments using the documented v1 AES-256-GCM envelope.
- Encryption keys remain client-side; they are not uploaded, stored in SQLite, or added to evidence bundles.
- SQLite and optional PostgreSQL metadata enforce media type, chunk index, byte size, SHA-256 shape, foreign keys, and unique chunk identity.
- Upload-operation metadata stores hashed idempotency keys, normalized chunk
  identity fields, immutable request fingerprint fields, fingerprint hashes,
  and final chunk references for complete-upload replay. Raw idempotency keys
  are not stored.
- Duplicate chunk reconciliation is a private read-only metadata comparison
  for already accepted chunk identities. Conflict responses identify only
  mismatched field names and do not return uploaded bytes, stored paths, object
  keys, plaintext, raw keys, raw tokens, request bodies, private deployment
  details, or conflicting stored values.
- Optional Valkey/Redis-compatible coordination fails closed at startup when
  explicitly configured but unavailable.
- Optional complete-upload coordination uses short-lived Valkey lease keys
  derived from a server-controlled hash of normalized chunk identity. Busy
  leases return `409 upload_in_progress` with a retry hint, while runtime
  coordination failures return a retryable safe error.
- Route-class rate limiting groups main API authentication, account, incident,
  upload, reconciliation, stream, token, and download requests, plus
  admin API requests, by safe class labels and a hash of the socket peer identity. Limiter
  keys do not include raw session tokens, Authorization headers, raw
  idempotency keys, request bodies, uploaded bytes, incident IDs, stored paths,
  object keys, plaintext, raw keys, or private deployment details.
- The current listener split does not expose `/v1/health/live` or
  `/v1/health/ready`; operator readiness details should not be published on
  the main API/viewer origin or on the dashboard-only private-admin listener.
- Media streams must be open before new chunks can be attached. The repository rechecks incident and stream state when chunk metadata is inserted.
- Stream completion verifies contiguous chunks plus readable stored files, and the repository revalidates chunk rows before committing the stream to `complete`.
- Local account passwords are stored as bcrypt hashes. Private `/v1` requests
  use opaque server-side sessions whose raw tokens are returned only by login
  and stored only as SHA-256 hashes.
- Private incident access is authorized by account owner and role. Regular
  users can access their own incidents. Admins can access incidents across
  accounts and use `/v1/admin/...` account routes. Legacy unowned incidents are
  admin-only until a future private reassignment or quarantine workflow exists;
  see
  [legacy unowned incident reassignment](legacy-unowned-incident-reassignment.md).
- Contact public-key routes are scoped to the authenticated account. Current
  sharing-grant and wrapped-key routes are owner-only: users and admins can
  create, list, read, or revoke records only for incidents, grants, or
  wrapped-key records owned by the authenticated account. New grants require an
  active contact public key owned by the same account and can be scoped to an
  incident or one stream. New wrapped-key records require an active, unexpired
  grant that authorizes ciphertext access and an active contact public key.
  The routes do not store or return contact private keys, raw media keys,
  plaintext, browser fragment secrets, request bodies, uploaded bytes, stored
  paths, staging paths, object keys, or private deployment details.
- The private admin web surface uses `html/template`, stores browser admin
  sessions in an HttpOnly SameSite cookie scoped to `/admin`, serves embedded
  token-neutral CSS from the private admin prefix without authentication, and
  shows route-boundary status plus local account-management data rather than
  incident evidence, tokens, password hashes, stored paths, object keys,
  plaintext, raw keys, or private deployment details. Its authenticated
  state-changing forms use a session-bound CSRF token. Admin own-password
  changes require the current password and revoke other sessions; admin resets
  for other accounts revoke that account's sessions; logout revokes the current
  admin web session.
- Viewer tokens use 256 bits from `crypto/rand`; only SHA-256 token hashes are stored. Tokens created without an explicit `expires_at` default to a 24-hour lifetime unless `SAFE_DEFAULT_INCIDENT_TOKEN_TTL` is configured differently.
- Expired, revoked, and invalid viewer tokens return the same public error.
- Public viewer app-level rate limiting groups page lookup, JSON polling,
  encrypted ZIP download, and static asset requests by safe route class and a
  hash of the socket peer identity.
- Incident summaries do not expose `stored_path`. Viewer summaries and bundle
  manifests may expose user-supplied `original_filename` basenames when clients
  provided them. Viewer bundle downloads expose only encrypted chunk bytes and
  generated manifests for completed streams.
- Open and failed streams are exposed to the current public viewer only as
  metadata summaries. Live or partial stream access is planning-only in
  [live-partial-stream-access-boundary.md](live-partial-stream-access-boundary.md).
- ZIP bundle entry names are server-controlled and generated from metadata; clients do not provide stored paths for download.
- Public viewer responses use a strict same-origin `Content-Security-Policy` with `frame-ancestors 'none'`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, and a restrictive camera/microphone/geolocation `Permissions-Policy`.
- Token-protected pages, JSON, errors, authenticated responses, authenticated chunk reads, and bundle downloads use `Cache-Control: no-store`.
- Request logging records method, redacted route pattern, status, byte count, and duration. It does not log request bodies, uploaded bytes, Authorization headers, raw session tokens, raw viewer tokens, raw incident tokens, raw idempotency keys, plaintext, or raw keys.
- Templates use Go `html/template` escaping.
- Storage rejects absolute paths, `..`, slash-containing path segments, and backslash traversal. S3 object keys are derived from server-controlled stored paths and an optional safe prefix.

## Incident Mode Risks To Preserve For Future Design

Future incident-mode work should treat these as explicit design risks rather than incidental frontend labels:

- non-emergency interaction records may include sensitive conversations with police, security, landlords, employers, service providers, or other authorities
- legal recording and sharing rules vary by jurisdiction
- incident mode, capture profile, escalation policy, sharing, export, publication, and legal submission are distinct actions and should not be collapsed into capture
- safety-check or dead-man switch notifications may create false alarms if timers, connectivity, or contact workflows are poorly designed
- trusted contacts need clear context and should decide whether to contact emergency services unless a future emergency-services integration is explicitly implemented
- account-owner, trusted-contact, admin/operator, public-link, and optional
  escrow access must remain separated before public account systems exist; see
  [v1-access-control.md](v1-access-control.md)

The current backend does not implement incident-mode-specific controls yet, so future work must update this threat model before or alongside implementation.

## Known Limitations

- No implemented public product API exposure model for `/v1`; local accounts
  and sessions are authenticated main-API controls, not a complete public
  security model. The `/v1` credential is an Authorization bearer session for
  authenticated main API callers. The private `/admin` web authenticated
  state-changing forms use an
  HttpOnly SameSite cookie with a session-bound CSRF token. Broader browser
  admin flows still need explicit browser credential and CSRF review.
- Separate main and private-admin ports reduce accidental route exposure, but
  they are not a complete security model.
- `/v1` must not be publicly exposed as-is.
- No iOS app, Android app, web client, production local recording client,
  production client key storage, push notifications, SMS, Messenger
  integration, or public admin dashboard. The private `/admin` surface
  is not a complete operator UI, and the local desktop-recorder behavior in
  `cmd/simclient` is simulator/reference flow only.
- No mode-driven access, escalation, retention, key-custody, safety-check
  timer, dead-man switch notification, browser decryption, backend decryption, or
  trusted-contact account behavior.
- No built-in TLS, IP allowlist, or general-purpose abuse-throttling system
  beyond main API and public viewer route-class rate limiting.
- Optional PostgreSQL metadata does not change the main `/v1` deployment
  boundary, token hashing, ciphertext-only storage, or backup/restore expectations
  described in [postgresql-metadata-migration.md](postgresql-metadata-migration.md).
  It also does not make the current upload flow cluster-safe on its own.
- Optional Valkey/Redis-compatible coordination does not change the main
  `/v1` boundary, does not hold durable evidence state, and does not complete
  all cluster-safe upload semantics on its own.
- No implemented resumable or partial-upload protocol beyond the
  complete-upload `Idempotency-Key` path and optional Valkey in-progress
  leases documented in
  [cluster-safe-upload-semantics.md](cluster-safe-upload-semantics.md). Uploads
  without idempotency keys still use the existing `409 duplicate_chunk`
  behavior.
- No implemented resumable upload or partial-upload lease protocol. Current
  clients should retry complete encrypted chunk uploads; the future design is
  planned in
  [resumable-upload-lease-protocol.md](resumable-upload-lease-protocol.md).
- No implemented regional stream-ingress relay. If added later, the relay must
  be upload-only, temporary, ciphertext-only, and subordinate to the core API
  for authorization, idempotency, durable blob commits, and metadata. The
  future design is planned in
  [regional-stream-ingress-relay.md](regional-stream-ingress-relay.md).
- Retention, backup, restore, and deletion policy is documented in
  [retention-backup-deletion.md](retention-backup-deletion.md), with enforcement
  details in
  [incident-deletion-retention-enforcement.md](incident-deletion-retention-enforcement.md).
  The backend implements authenticated incident deletion APIs and optional
  closed-incident retention, but it does not implement mode-specific retention,
  built-in disk encryption, or object-bucket lifecycle policy enforcement.
  Future mode-aware retention is planning-only in
  [mode-aware retention policy](mode-aware-retention-policy.md).
- Cluster backup, restore, and failure runbooks are operational guidance only
  and do not make optional PostgreSQL, S3-compatible storage, or
  Valkey/Redis-compatible coordination production-cluster readiness by
  themselves.
- No malware/content scanning; uploaded bytes are assumed to be client-encrypted blobs.
- Bundle downloads are encrypted chunk bundles, not decrypted or playable media exports.
- No implemented live or partial stream chunk access before stream completion.
- No account self-service recovery, email verification, second factor
  authentication, delegated identity provider, or public account portal.
- Viewer links are bearer tokens and must be shared carefully.
- No implemented production key recovery, Keychain storage, trusted-contact
  account access, browser decryption, break-glass key access, or playable
  export. The future key custody and emergency access design is documented in
  [key-custody.md](key-custody.md), contact key-sharing and wrapped-key grants
  are described in [contact-key-sharing-grants.md](contact-key-sharing-grants.md),
  browser decryption is designed in [browser-decryption.md](browser-decryption.md),
  and break-glass access is designed in [break-glass-key-access.md](break-glass-key-access.md).

## Deployment Guidance

For local/private use, bind the main API/viewer server to localhost or a
private network and restrict access with WireGuard, firewall rules, or a
reverse proxy. If exposing only the incident viewer publicly, route only
viewer paths (`/i/...`, `/e/...`, and `/static/...`) to the main listener and
do not forward public wildcard traffic to `/v1`. Public edges must block
`/v1/admin/...`. The `/admin` dashboard uses the separately bound
private-admin listener and still requires admin authentication.
Inside Docker containers, bind to container addresses such as `0.0.0.0:8080`
and restrict host exposure with port publishing, firewall rules, WireGuard, or
reverse proxy configuration.

Use TLS at the edge for any network access. Apply deployment-edge rate limiting for public incident viewer routes and any private reverse-proxy boundary; the app-level public viewer limiter is a backstop, not a replacement for reviewed edge controls. Keep reverse-proxy logs, metrics, dashboards, and rate-limit keys from recording raw `/i/{token}` paths and pre-rename compatibility `/e/{token}` paths.

The Go app does not set `Strict-Transport-Security` by default because local development uses plain HTTP and MDN guidance expects HSTS only over HTTPS. Enable HSTS at the production HTTPS reverse proxy after the public hostname is consistently available over TLS.

## Next Security Steps

- Extend the current `/v1` access-control implementation before any public
  product API exposure: add public abuse controls, browser credential rules,
  audited trusted-contact grants, and deployment operations.
- Use the first-class incident-mode and escalation design in
  [incident-modes.md](incident-modes.md) before implementing mode-driven
  interaction-record, safety-check, or dead-man switch workflows.
- Define the future public product API and separately bound private admin API,
  including account-owner, trusted-contact, web-client, and admin/operator
  authorization boundaries, using the target listener split in
  [public-api-listener-split.md](public-api-listener-split.md).
- Tune deployment-edge and app-level rate limits for token guesses, uploads,
  downloads, static assets, and admin actions.
- Review viewer-token expiry tuning and revocation workflows.
- Extend the implemented deletion workflow with deployment-specific backup
  expiry, restore reconciliation, and any needed mode-specific retention policy
  using [mode-aware retention policy](mode-aware-retention-policy.md).
- Prototype the documented hybrid key custody model without weakening the current ciphertext-only backend.
- Prototype browser decryption only after accepting the browser trust model and malicious-server limitations.
- Treat server-assisted break-glass access as an optional future mode only after explicit policy, audit, and deployment design.
- Review deployment logging so raw tokens are not captured outside the Go server.
