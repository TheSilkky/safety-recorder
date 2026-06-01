# Documentation

This directory contains the detailed documentation for Proofline Server, the Go backend component of the planned Proofline project. The top-level [README](../README.md) is a concise server overview; these docs keep operational, API, deployment, incident-capture, and development details in one place.

## Contents

| Document | Purpose |
|---|---|
| [Getting started](getting-started.md) | Run the backend locally and exercise the simulator flow. |
| [Architecture](architecture.md) | System diagrams, listener boundaries, repository split, and server data flow. |
| [Configuration](configuration.md) | Environment variables, backend selectors, bind addresses, upload limits, and data layout. |
| [Production cluster scope](production-cluster-scope.md) | Additive path for optional PostgreSQL metadata, optional S3-compatible object storage, and optional Valkey/Redis-compatible coordination. |
| [Cluster backup, restore, and failure runbook](cluster-backup-restore-runbook.md) | Operational guidance for optional PostgreSQL metadata, S3-compatible encrypted blobs, configuration, coordination, restore validation, and cluster failure modes. |
| [PostgreSQL metadata migration path](postgresql-metadata-migration.md) | PostgreSQL metadata backend schema parity, migrations, transaction boundaries, tests, explicit SQLite-to-PostgreSQL migration runbook, rollback limits, and restore expectations. |
| [Cluster-safe upload operation semantics](cluster-safe-upload-semantics.md) | Complete-upload idempotency-key behavior plus remaining cluster-safe upload operation design for commit ordering, retry success, conflict handling, and cleanup across metadata and blob backends. |
| [Resumable upload and upload lease protocol](resumable-upload-lease-protocol.md) | Planning decision to keep the desktop recorder simulator on complete encrypted chunk retry semantics while deferring resumable uploads and partial-upload lease sessions. |
| [Regional stream ingress relay](regional-stream-ingress-relay.md) | Planning boundary for a future optional regional upload-only relay that stages ciphertext temporarily and lets the core API remain authoritative for authorization, durable commits, idempotency, and metadata. |
| [Incident capture modes](incident-modes.md) | Planned emergency, interaction-record, safety-check, and evidence-note modes, plus future capture-profile, escalation-policy, sharing-state, and migration boundaries. |
| [Mode-aware retention policy](mode-aware-retention-policy.md) | Planning boundary for future retention policy based on incident mode, safety-check state, sharing/export state, grants, wrapped keys, tombstones, and backups. |
| [/v1 access control](v1-access-control.md) | Current local account/session boundary plus future role, grant, public product API, private admin API listener, audit, and migration boundaries for account-owner, trusted-contact, public-link, admin/operator, and optional escrow access. |
| [Main API public exposure listener split](public-api-listener-split.md) | Planning boundary for keeping main API routes and the read-only incident viewer on `8080` while keeping the private `/admin` dashboard on `8081`. |
| [Legacy unowned incident reassignment](legacy-unowned-incident-reassignment.md) | Planning boundary for future private reassignment or quarantine of incidents created before account ownership existed. |
| [Encryption](encryption.md) | Client-side chunk envelope, simulator key file, and local bundle verification. |
| [iOS local recorder prototype](ios-local-recorder-prototype.md) | Future native incident-capture scope, chunking, encrypted staging, retry, and API mapping. |
| [Key custody and emergency access](key-custody.md) | Future production key custody, trusted-contact access, and break-glass design. |
| [Contact key sharing, grants, and wrapped-key metadata](contact-key-sharing-grants.md) | Current trusted-contact public-key, grant, and wrapped-key metadata boundaries, plus future trusted-contact delivery, retention, audit, and implementation sequencing. |
| [Contact-wrapped key metadata simulator prototype](contact-wrapped-key-metadata-simulator.md) | Simulator-only prototype for modeling trusted-contact public keys, non-secret key IDs, wrapped stream media keys, and safe development metadata without production key custody. |
| [Browser-side decryption](browser-decryption.md) | Future incident viewer decryption options, risks, and phased direction. |
| [Live partial stream access boundary](live-partial-stream-access-boundary.md) | Future live or partial stream access roles, stream-state exposure, partial manifests, caching, and key-custody dependencies. |
| [Break-glass key access](break-glass-key-access.md) | Future optional server-assisted emergency key access and dead-man-switch design. |
| [API](api.md) | Current main `/v1` routes, private `/admin` dashboard routes, request examples, response examples, and bundle formats. |
| [Deployment](deployment.md) | Local, Docker, SQLite WAL operations, reverse proxy, TLS, and public exposure notes. |
| [Retention, backup, and deletion](retention-backup-deletion.md) | Operational policy for evidence lifecycle, backups, restores, and deletion limits. |
| [Incident deletion and retention enforcement](incident-deletion-retention-enforcement.md) | Current private/admin deletion decisions, retention worker behavior, tombstones, blob deletion retry, and remaining lifecycle boundaries. |
| [Security model](security-model.md) | Current controls, browser headers, logging posture, and security assumptions. |
| [Threat model](threat-model.md) | Assets, trust boundaries, controls, limitations, and next security steps. |
| [Simulator](simulator.md) | Simulator commands, durable desktop-recorder staging, poor-network controls, and test flows. |
| [Development](development.md) | Repository layout, commands, AI assistance note, branch rulesets, checks, and release checklist notes. |
| [Compose smoke tests](../compose/README.md) | Local release-smoke stacks for SQLite/local, PostgreSQL/local, SQLite/S3-compatible MinIO, and full PostgreSQL/MinIO/Valkey combinations. |
| [Codex change control](codex-change-control.md) | Rollback points, scoped Codex tasks, review steps, and issue-first backlog rules. |
| [Code map](code-map.md) | Package layout and main backend request flows. |
| [Reports](reports/README.md) | Public technical review reports and report-generation workflow notes. |

## Current Repository Scope

This repository is the Go server backend only. In the planned multi-repo layout it corresponds to:

```text
open-proofline/server
```

Future companion repositories are expected to be separate projects:

```text
open-proofline/web-client
open-proofline/ios-client
open-proofline/android-client
open-proofline/protocol
```

Those repositories do not exist in this repository and should not be implemented here by accident. This server repository may keep planning notes for client and protocol work only while the split is being designed.

## Current Backend Scope

Proofline Server receives already-encrypted chunks, stores metadata in SQLite by default or optional PostgreSQL, stores encrypted blobs on local disk by default or in optional S3-compatible object storage, performs a startup check against optional Valkey/Redis-compatible coordination when explicitly configured, groups chunks into media streams, serves a private admin web surface under `/admin`, applies app-level route-class rate limiting to main API routes, can use Valkey for short-lived complete-upload leases, and exposes a token-scoped read-only incident viewer with app-level route-class rate limiting. The Go simulator can produce the documented v1 client-side encryption envelope for development and test flows.

The future regional stream-ingress relay design is planning-only; see
[regional-stream-ingress-relay.md](regional-stream-ingress-relay.md). It does
not add an implemented upload edge, public `/v1` exposure, durable ingress
storage, backend decryption, key custody, or deployment automation.

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
capture-profile, escalation-policy, and sharing-state metadata, but those fields
do not drive access, notification, retention, sharing, viewer, or key-custody
behavior. Mode-driven behavior and migration boundaries are documented in
[incident-modes.md](incident-modes.md). Current local account/session behavior
and future account-owner, trusted-contact, public-link, admin/operator, and
optional escrow access boundaries are documented in
[v1-access-control.md](v1-access-control.md).
Legacy unowned incidents remain admin-only until a future private reassignment
or quarantine workflow is implemented; the planning boundary is documented in
[legacy-unowned-incident-reassignment.md](legacy-unowned-incident-reassignment.md).

Authenticated account owners can register trusted-contact public-key metadata
and manage incident/stream-scoped sharing grants for their own incidents. Those
routes can store and deliver wrapped media-key metadata through private API
responses when an active grant authorizes ciphertext access. They do not add
trusted-contact accounts, browser or backend decryption, public viewer changes,
notifications, raw key storage, or key escrow.

The future iOS incident-capture prototype is planned in [ios-local-recorder-prototype.md](ios-local-recorder-prototype.md). Future production key custody is documented in [key-custody.md](key-custody.md), with contact key sharing and wrapped-key metadata described in [contact key sharing, grants, and wrapped-key metadata](contact-key-sharing-grants.md), a simulator-only contact-wrapped key metadata prototype in [contact-wrapped-key-metadata-simulator.md](contact-wrapped-key-metadata-simulator.md), browser decryption and break-glass follow-up designs in [browser-decryption.md](browser-decryption.md) and [break-glass-key-access.md](break-glass-key-access.md), and live or partial stream access boundaries in [live-partial-stream-access-boundary.md](live-partial-stream-access-boundary.md). None of those future designs make the current main `/v1` API or `/admin` surface safe for broad public exposure.

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

The main `/v1` API and `/admin` web surface use local account sessions but are still not public product surfaces. Keep main `/v1` behind the reviewed deployment boundary, and keep `/admin` behind localhost, LAN, WireGuard, firewall rules, or a strict private reverse proxy. Separate main/private-admin bind addresses reduce accidental exposure, but they are not a complete security model.
