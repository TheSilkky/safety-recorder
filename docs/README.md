# Documentation

This directory contains the detailed documentation for Proofline Server, the Go backend component of the planned Proofline project. The top-level [README](../README.md) is a concise server overview; these docs keep operational, API, deployment, incident-capture, and development details in one place.

## Contents

| Document | Purpose |
|---|---|
| [v1 preview direction](v1-preview-direction.md) | Direction-setting source of truth for v1 preview terminology, repository roles, current-versus-future boundaries, and Codex guidance. |
| [v1 preview readiness checklist](v1-preview-readiness-checklist.md) | Release gate for v1 preview, v1.0.0, or real-user evidence-upload readiness claims. |
| [Getting started](getting-started.md) | Run the backend locally and exercise the simulator flow. |
| [Architecture](architecture.md) | System diagrams, listener boundaries, repository split, and server data flow. |
| [Configuration](configuration.md) | TOML config, `SAFE_*` environment overrides, secret files, backend selectors, bind addresses, upload limits, and data layout. |
| [Production cluster scope](production-cluster-scope.md) | Additive path for optional PostgreSQL metadata, optional S3-compatible object storage, and optional Valkey/Redis-compatible coordination. |
| [Cluster backup, restore, and failure runbook](cluster-backup-restore-runbook.md) | Operational guidance for optional PostgreSQL metadata, S3-compatible encrypted blobs, configuration, coordination, restore validation, and cluster failure modes. |
| [PostgreSQL metadata migration path](postgresql-metadata-migration.md) | PostgreSQL metadata backend schema parity, migrations, transaction boundaries, tests, explicit SQLite-to-PostgreSQL migration runbook, rollback limits, and restore expectations. |
| [Cluster-safe upload operation semantics](cluster-safe-upload-semantics.md) | Complete-upload idempotency-key behavior plus remaining cluster-safe upload operation design for commit ordering, retry success, conflict handling, and cleanup across metadata and blob backends. |
| [Resumable upload and upload lease protocol](resumable-upload-lease-protocol.md) | Planning decision to keep the desktop recorder simulator on complete encrypted chunk retry semantics while deferring resumable uploads and partial-upload lease sessions. |
| [Upload telemetry boundary](upload-telemetry-boundary.md) | Planning boundary that keeps client upload telemetry local before v1 preview and defines the narrow safe shape for any future server-visible telemetry. |
| [Regional stream ingress relay](regional-stream-ingress-relay.md) | Current health/readiness route with safe aggregate readiness categories, backend-issued upload and fanout capabilities, relay complete-chunk upload route, temporary ciphertext staging, service-authenticated core preflight/commit/fanout authorization, optimistic encrypted unconfirmed fanout, bounded backend confirmation/rejection state, and remaining boundary for optional relay hardening. |
| [Incident capture modes](incident-modes.md) | Planned emergency, interaction-record, safety-check, and evidence-note modes, plus future capture-profile, escalation-policy, sharing-state, and migration boundaries. |
| [Capture stream variants and evidence supersession](capture-stream-variants.md) | Future source-timeline grouping, near-live/evidence-master/audio-priority variants, supersession, failed-stream preservation, encrypted context, and relay fanout boundaries without changing runtime behavior. |
| [Encrypted location context](encrypted-location-context.md) | Design boundary for full-fidelity GPS, speed, heading, and freshness context as encrypted evidence, with token-viewer, trusted-contact, relay, logging, and validation expectations. |
| [Mode-aware retention policy](mode-aware-retention-policy.md) | Planning boundary for future retention policy based on incident mode, safety-check state, sharing/export state, grants, wrapped keys, tombstones, and backups. |
| [/v1 access control](v1-access-control.md) | Current local account/session and optional browser-cookie boundary plus future role, grant, public product API, private admin API listener, audit, and migration boundaries for account-owner, trusted-contact, public-link, admin/operator, and optional escrow access. |
| [Main API public exposure listener split](public-api-listener-split.md) | Boundary for keeping main API routes and the read-only incident viewer on `8080` while keeping private `/admin/api/...` JSON routes and the `/admin` dashboard on `8081`. |
| [Public web-client deployment boundary](public-web-client-deployment-boundary.md) | Server route, CORS, CSRF, cookie, cache, edge, logging, and validation boundary before a public web-client origin is treated as a reviewed v1 preview path. |
| [Web-client viewer routing](web-client-viewer-routing.md) | Routing decision for future no-account web-client viewer links, current `/i` and `/e` prototype route handling, and token-leakage controls. |
| [Legacy unowned incident reassignment](legacy-unowned-incident-reassignment.md) | Private admin review, reassignment, and keep-unowned audit workflow for incidents created before account ownership existed. |
| [Stripe subscription billing](stripe-subscription-billing.md) | Cost-recovery hosted-server subscription boundary for future Stripe Checkout, Billing, Customer Portal, webhook, entitlement, and account-lifecycle work. |
| [Contacts, key model, and viewer replacement](contacts-and-viewer-replacement.md) | Backend context for future trusted-contact invite/accept flows, durable account/device/contact recipient keys, incident/stream CEKs, GPS privacy, web-client viewer replacement, and issue-family planning without changing runtime behavior. |
| [Encryption](encryption.md) | Client-side chunk envelope, simulator key file, and local bundle verification. |
| [iOS local recorder prototype](ios-local-recorder-prototype.md) | Future native incident-capture scope, chunking, encrypted staging, retry, and API mapping. |
| [Key custody and emergency access](key-custody.md) | Future production key custody, trusted-contact access, and break-glass design. |
| [Pure post-quantum encryption envelope](post-quantum-envelope.md) | Current v1 preview runtime default for an ML-KEM-768, HKDF-SHA384, and AES-256-GCM evidence envelope, including server metadata validation and simulator defaults. |
| [Contact key sharing, grants, and wrapped-key metadata](contact-key-sharing-grants.md) | Current trusted-contact public-key, grant, and wrapped-key metadata boundaries, plus future trusted-contact delivery, retention, audit, and implementation sequencing. |
| [Contact-wrapped key metadata simulator prototype](contact-wrapped-key-metadata-simulator.md) | Simulator-only prototype for modeling trusted-contact public keys, non-secret key IDs, wrapped stream CEKs, and safe development metadata without production key custody. |
| [Browser-side decryption](browser-decryption.md) | Future incident viewer decryption options, same-origin trust limits, static/signed viewer gate, risks, and phased direction. |
| [Live partial stream access boundary](live-partial-stream-access-boundary.md) | Future live or partial stream access roles, stream-state exposure, partial manifests, caching, and key-custody dependencies. |
| [Break-glass key access](break-glass-key-access.md) | Future optional server-assisted emergency key access and dead-man-switch policy boundary, including wrapped-key-release-first guidance. |
| [API](api.md) | Current main `/v1` routes, private `/admin` dashboard routes, request examples, response examples, and bundle formats. |
| [Deployment](deployment.md) | Local, Docker, SQLite WAL operations, reverse proxy, TLS, and public exposure notes. |
| [Retention, backup, and deletion](retention-backup-deletion.md) | Operational policy for evidence lifecycle, backups, restores, and deletion limits. |
| [Incident deletion and retention enforcement](incident-deletion-retention-enforcement.md) | Current private/admin deletion decisions, retention worker behavior, tombstones, blob deletion retry, and remaining lifecycle boundaries. |
| [Security model](security-model.md) | Current controls, browser headers, logging posture, and security assumptions. |
| [Threat model](threat-model.md) | Assets, trust boundaries, controls, limitations, and next security steps. |
| [Logging requirements](logging-requirements.md) | Standard structured logging fields, safe error categories, redaction rules, and review/test requirements for logging changes. |
| [Simulator](simulator.md) | Simulator commands, durable desktop-recorder staging, poor-network controls, and test flows. |
| [Development](development.md) | Repository layout, commands, AI assistance note, branch rulesets, checks, and release checklist notes. |
| [Compose smoke tests](../compose/README.md) | Local release-smoke stacks for SQLite/local, PostgreSQL/local, SQLite/S3-compatible MinIO, and full PostgreSQL/MinIO/Valkey combinations. |
| [Codex change control](codex-change-control.md) | Rollback points, scoped Codex tasks, review steps, and issue-first backlog rules. |
| [Code map](code-map.md) | Package layout and main backend request flows. |
| [Reports](reports/README.md) | Public technical review reports and report-generation workflow notes. |

## Current Repository Scope

This repository is the Go server backend only. In the current `open-proofline`
organisation it is:

```text
open-proofline/server
```

Companion repositories are separate current or future projects outside this
server repository:

```text
open-proofline/web-client
open-proofline/ios-client
open-proofline/android-client
open-proofline/protocol
```

Those repositories do not exist in this repository and should not be implemented
here by accident. This server repository may keep planning notes for client and
protocol work only as repository-boundary context.

## Current Backend Scope

Proofline Server receives already-encrypted chunks, stores metadata in SQLite by default or optional PostgreSQL, stores encrypted blobs on local disk by default or in optional S3-compatible object storage, loads TOML config with `SAFE_*` environment and secret-file overrides, performs a startup check against optional Valkey/Redis-compatible coordination when explicitly configured, groups chunks into media streams, can issue configured short-lived regional relay upload and fanout capabilities for authorized open streams, exposes service-authenticated core relay preflight/commit/fanout authorization endpoints when relay-to-core auth is configured, includes a separate regional stream-ingress relay route for complete encrypted chunk upload/staging/forwarding, optimistic encrypted unconfirmed fanout, and bounded fanout state when configured, serves private admin JSON routes under `/admin/api/...` and a private admin web surface under `/admin`, applies app-level route-class rate limiting to main API routes, can use Valkey for short-lived complete-upload leases, supports optional browser cookie sessions with CSRF and credentialed CORS for future web-client calls, supports disabled-by-default configurable account registration with SMTP-backed email verification for open self-hosted registration, supports email challenge, TOTP, and disabled-by-default WebAuthn/FIDO2 passkey or roaming security-key second-factor setup for account gating, supports private-admin assisted second-factor reset for lost-factor recovery, and exposes a token-scoped read-only incident viewer with app-level route-class rate limiting. New chunk uploads must use the accepted post-quantum payload envelope by default, and the Go simulator now produces that PQ envelope unless an explicit v1 compatibility flag is used.

The regional stream-ingress relay currently has a separate health/readiness
route with safe aggregate readiness categories, backend-issued upload and
fanout capabilities, a complete encrypted chunk upload route with temporary
relay-local staging, service-authenticated core preflight/commit forwarding,
optimistic encrypted unconfirmed fanout, and bounded backend
confirmation/rejection state;
see
[regional-stream-ingress-relay.md](regional-stream-ingress-relay.md). It does
not add broad public `/v1` exposure, durable ingress storage, replay, metrics,
backend decryption, key custody, or deployment automation.

Future capture stream variants, evidence supersession, and encrypted
GPS/location context are planning-only; see
[capture-stream-variants.md](capture-stream-variants.md) and
[encrypted-location-context.md](encrypted-location-context.md). The current
backend still treats each media stream as one concrete upload lane, and it does
not implement capture stream groups, variant roles, source-timeline
supersession, encrypted location sidecars, or canonical evidence selection.

The planned production-cluster scope is additive: SQLite and local filesystem
storage remain supported, optional PostgreSQL metadata can store incident
metadata, optional S3-compatible object storage can store committed encrypted
chunks, and optional Valkey/Redis-compatible coordination can be configured for
startup-checked short-lived coordination. Current backend selector
configuration accepts only implemented values. See
[production-cluster-scope.md](production-cluster-scope.md).

The PostgreSQL metadata path is implemented for explicit opt-in configuration;
see [postgresql-metadata-migration.md](postgresql-metadata-migration.md). It
does not change the current SQLite default or perform automatic
SQLite-to-PostgreSQL data migration.

The complete-upload idempotency-key path is implemented for authenticated chunk
uploads; see
[cluster-safe-upload-semantics.md](cluster-safe-upload-semantics.md). When
Valkey/Redis-compatible coordination is explicitly configured, complete-chunk
uploads also use short-lived in-progress leases and retry hints. These leases
are not durable evidence truth and do not implement resumable or partial-upload
sessions. The authenticated duplicate chunk reconciliation route is
implemented for comparing an expected chunk fingerprint with already accepted
metadata without re-uploading ciphertext; see [api.md](api.md).
The resumable upload and partial-upload lease path remains planning-only; see
[resumable-upload-lease-protocol.md](resumable-upload-lease-protocol.md). It
continues to defer resumable uploads while the desktop recorder simulator
measures the current complete encrypted chunk upload contract. The
desktop simulator in `cmd/simclient` includes durable encrypted staging,
restart/resume drills, local file input, optional ffmpeg segment capture, and
adjustable poor-network simulation while continuing to use the current local
account/session flow and complete encrypted chunk upload API.

The long-term Proofline product direction is broader than emergency-only
recording. Future clients should support emergency incidents, non-emergency
interaction records, timed safety checks, and evidence notes while keeping
capture, escalation, sharing, and legal/export actions separate. The current
main incident create/read routes support optional incident-mode,
capture-profile, escalation-policy, and sharing-state metadata, and the account
incident list/detail routes return only owner-scoped public-safe metadata for
future web-client reads. Those mode fields do not drive access, notification,
retention, sharing, viewer, or key-custody behavior. Mode-driven behavior and
migration boundaries are documented in
[incident-modes.md](incident-modes.md). Current local account/session behavior
and future account-owner, trusted-contact, public-link, admin/operator, and
optional escrow access boundaries are documented in
[v1-access-control.md](v1-access-control.md).
Legacy unowned incidents remain admin-only unless an admin uses the private
reassignment workflow documented in
[legacy-unowned-incident-reassignment.md](legacy-unowned-incident-reassignment.md).

Authenticated account owners can invite local accounts into trusted-contact
relationships, register or replace trusted-contact public-key metadata, mark
contact keys lost or revoked, and manage incident/stream-scoped sharing grants
for their own incidents. Those routes can
store and deliver wrapped CEK/media-key metadata through private API responses
when an active grant authorizes ciphertext access. Signed-in accepted trusted
contacts can read wrapped-key records only when the relationship,
recipient-bound contact key, grant, and wrapped-key record are active.
Relationship records alone do not add browser or backend decryption, public
viewer changes, notifications, raw key storage, or key escrow.

The future Stripe subscription billing design for Official Proofline hosted
server access is documented in
[stripe-subscription-billing.md](stripe-subscription-billing.md). It is a
cost-recovery subscription boundary and implementation plan only; it does not
implement payment processing, billing webhooks, donations, public production
deployment, or any change to self-hosted operation.

The future iOS incident-capture prototype is planned in
[ios-local-recorder-prototype.md](ios-local-recorder-prototype.md). Future
production key custody is documented in [key-custody.md](key-custody.md), with
the v1 preview post-quantum envelope requirement in
[post-quantum-envelope.md](post-quantum-envelope.md), contact key sharing and
wrapped-key metadata in
[contact key sharing, grants, and wrapped-key metadata](contact-key-sharing-grants.md),
a simulator-only contact-wrapped key metadata prototype in
[contact-wrapped-key-metadata-simulator.md](contact-wrapped-key-metadata-simulator.md),
browser decryption and break-glass follow-up designs in
[browser-decryption.md](browser-decryption.md) and
[break-glass-key-access.md](break-glass-key-access.md), and live or partial
stream access boundaries in
[live-partial-stream-access-boundary.md](live-partial-stream-access-boundary.md).
None of those future designs or preview requirements make the current main
`/v1` API, private `/admin/api/...` JSON routes, or `/admin` surface safe for
broad public exposure.

Evidence bundles are encrypted chunk bundles with JSON manifests. They are not decrypted, playable, or merged media exports.

Retention, backup, and deletion policy is documented in
[retention-backup-deletion.md](retention-backup-deletion.md), with incident
deletion and retention enforcement details in
[incident-deletion-retention-enforcement.md](incident-deletion-retention-enforcement.md).
The backend implements authenticated incident deletion APIs and an automatic
background deletion worker; closed-incident retention is disabled unless
configured with `SAFE_CLOSED_INCIDENT_RETENTION`.
Future mode-aware retention is planning-only and documented in
[mode-aware-retention-policy.md](mode-aware-retention-policy.md).

Cluster-style backup, restore, and failure handling for optional PostgreSQL,
S3-compatible blob storage, and Valkey/Redis-compatible coordination is
documented in
[cluster-backup-restore-runbook.md](cluster-backup-restore-runbook.md). That
runbook is operational guidance only and does not make Proofline
production-ready public infrastructure.

## Security Reminder

The main `/v1` API uses local account sessions, email challenge, TOTP, and optional WebAuthn/FIDO2 second-factor setup gating for new accounts, and app-level route-class limits. Open account registration can be enabled only with email verification, but broad public exposure still needs route-level deployment review. Keep unreviewed main `/v1` route groups behind the reviewed deployment boundary, and keep `/admin/api/...` and `/admin` behind localhost, LAN, WireGuard, firewall rules, or a strict private reverse proxy. Separate main/private-admin bind addresses reduce accidental exposure, but they are not a complete security model.
