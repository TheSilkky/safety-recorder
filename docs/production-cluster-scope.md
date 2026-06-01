# Production Cluster Scope

This document records the production-cluster expansion path for Proofline Server.

It is a planning and scope document for cluster-related work. Optional
PostgreSQL metadata, optional S3-compatible object storage, and optional
Valkey/Redis-compatible coordination startup checks, and local `/v1`
account/session authentication are implemented, but public product API
authentication, public account workflows, cloud deployment automation,
production hardening, and upload-operation use of coordination are not
implemented.

## Current Local-First Scope

The current backend remains local-first and experimental:

- SQLite metadata remains supported and remains the default.
- Optional PostgreSQL metadata is available only when explicitly configured.
- Local filesystem encrypted blob storage remains supported.
- Optional S3-compatible encrypted blob storage is available only when explicitly configured.
- No coordination backend remains the default; Valkey/Redis-compatible
  coordination is available only when explicitly configured.
- The simulator and local development flow remain supported.
- The main `/v1` API remains intended for a reviewed private deployment boundary
  and requires local account sessions.
- The public incident viewer remains token-gated and read-only.
- The backend stores ciphertext only and does not decrypt chunks.

SQLite plus local filesystem storage remains the default development and small self-hosted deployment shape unless a future release deliberately changes defaults.

## Planned Cluster Scope

The planned production-cluster path adds optional backend support for deployments where more than one API node may handle requests.

Planned optional cluster backends:

| Capability | Local/default backend | Planned cluster backend |
|---|---|---|
| Metadata | SQLite | PostgreSQL, implemented as an optional backend |
| Committed encrypted chunks | Local filesystem | S3-compatible object storage, implemented as an optional backend |
| Short-lived coordination | None | Valkey/Redis-compatible coordination, implemented as an optional startup-checked backend |

These backends should be additive. They must not remove or weaken SQLite and local filesystem support.

The current configuration scaffold exposes backend selectors for these
capability groups. It accepts implemented values:
`SAFE_METADATA_BACKEND=sqlite` or `postgresql`, `SAFE_BLOB_BACKEND=local` or
`s3`, and `SAFE_COORDINATION_BACKEND=none`, `valkey`, or `redis`.

## Cluster-Safety Principles

Cluster-aware behavior means duplicate attempts may happen, but duplicate side effects must not happen.

Production-cluster support should rely on:

- stable operation identities
- idempotency keys for retryable client operations
- database uniqueness constraints for metadata
- object-storage conditional writes for immutable encrypted chunks
- retry-safe upload state transitions
- explicit cleanup for abandoned staging state

Valkey or another Redis-compatible service may reduce duplicate work, hold short-lived leases, and support retry coordination, but it must not be the permanent source of truth for incident metadata or committed encrypted chunks.

## PostgreSQL Scope

PostgreSQL support is implemented as the optional production-oriented metadata backend for new deployments.

PostgreSQL stores:

- incidents
- media streams
- chunk metadata
- checkins
- viewer-token metadata
- local account and session metadata
- upload operation and idempotency state for complete chunk uploads
- incident deletion decisions and retry state
- future trusted-contact, device, and broader access-control metadata, after
  that design exists

PostgreSQL support includes:

- a separate PostgreSQL migration path
- schema constraints equivalent to or stronger than the SQLite schema
- uniqueness constraints for stream-scoped and legacy chunk identities
- transaction boundaries for chunk metadata insertion and stream completion
- restore and migration documentation for new deployments

SQLite should remain supported for local development, simulator workflows, and small deployments.

The detailed design and implementation notes for this backend are
[PostgreSQL metadata migration path](postgresql-metadata-migration.md). That
document maps the current SQLite tables and constraints, migration tracking,
transaction boundaries, parity testing, configuration shape, and restore
expectations.

## S3-Compatible Object Storage Scope

S3-compatible object storage is implemented as an optional blob backend for committed encrypted chunks.

The object store should hold opaque encrypted bytes only. It must not require server-side decryption or raw media keys.

Object-storage support includes:

- server-controlled object keys
- final immutable object keys for committed encrypted chunks
- conditional no-overwrite writes for final objects
- local temp-file staging before final object writes
- cleanup guidance for abandoned local staging files
- backup and restore guidance that keeps metadata and blobs consistent

The implementation stages upload bytes under `SAFE_DATA_DIR/tmp`, computes SHA-256 over the uploaded ciphertext, verifies the client-provided hash, and then writes the final S3 object with `If-None-Match: *`. It does not create S3 staging objects. The local filesystem backend remains supported and continues to use relative server-controlled stored paths.

Backup, restore, and failure-mode guidance for PostgreSQL metadata plus
S3-compatible encrypted blobs is documented in the
[cluster backup, restore, and failure runbook](cluster-backup-restore-runbook.md).

## Valkey / Redis-Compatible Coordination Scope

Valkey or another Redis-compatible service is implemented as optional
production coordination, not durable storage. The current backend opens and
checks the configured service at startup, uses it for short-lived route-class
rate-limit counters, and can use it for short-lived complete-upload leases,
in-progress hints, and retry coordination.

It may be used for:

- complete-upload in-progress leases
- short-lived in-progress state
- retry coordination
- public viewer route-class rate-limit counters
- cleanup coordination for abandoned staging uploads

It must not be used as the final source of truth for:

- incident metadata
- chunk metadata
- committed encrypted chunk bytes
- viewer-token metadata
- retention or deletion decisions

If configured Valkey is unavailable at startup, the system fails closed.
Upload coordination failures fail closed for the affected operation with a
retryable private API error. PostgreSQL constraints and object-storage
no-overwrite behavior must still protect committed state from duplicates.

## Upload Operation Semantics

Future cluster upload handling should move toward explicit upload operations.
The detailed planning design is
[Cluster-safe upload operation semantics](cluster-safe-upload-semantics.md).
Resumable upload and partial-upload lease behavior is planned separately in
[Resumable upload and upload lease protocol](resumable-upload-lease-protocol.md);
that design keeps the local desktop recorder simulator on complete encrypted
chunk retries while deferring resumable uploads and partial-upload sessions.

A safe cluster upload flow should be designed around these steps:

1. Reserve or identify the upload operation using stable incident, stream, chunk index, media type, and idempotency metadata.
2. Stage encrypted bytes while computing SHA-256 over the uploaded ciphertext.
3. Verify the computed hash against the client-provided hash.
4. Commit encrypted bytes to the final immutable blob location.
5. Insert or confirm chunk metadata in PostgreSQL.
6. Return an idempotent success response when an equivalent chunk already exists.
7. Return a conflict when the same chunk identity is attempted with different ciphertext or metadata.
8. Clean up abandoned staging state conservatively.

A successful chunk upload should mean encrypted bytes are durably committed outside the staging backend and metadata has been written or confirmed. Loss of pre-commit staging state must be recoverable by client retry.

## Regional Stream Ingress Relay Scope

The future regional stream-ingress relay is planned as an optional upload-only
edge that can run close to users while the core API remains authoritative. It
is documented in
[regional-stream-ingress-relay.md](regional-stream-ingress-relay.md).

The relay may use local in-memory counters for single-node/dev deployments or
optional Valkey/Redis-compatible counters for multi-node relay deployments,
but that state must remain short-lived coordination. It must not become the
source of truth for incident metadata, chunk metadata, upload-operation state,
committed encrypted chunks, viewer-token metadata, deletion decisions, or
retention decisions.

Relay temporary staging is not durable evidence storage. A successful upload
through the relay still requires the core API to commit encrypted bytes to the
configured blob backend and write or confirm metadata in the configured
metadata backend.

## Boundaries And Non-Goals

This scope expansion does not by itself add:

- public exposure of the current main `/v1` API
- public exposure of the full current `/v1` API through a regional relay
- public account workflows
- OAuth, JWT, public account portal, trusted-contact accounts, or external
  identity integration
- web, iOS, Android, or shared protocol implementation in this repository
- backend decryption
- raw server-held media keys
- server-side playable media export
- trusted-contact accounts
- push, SMS, Messenger, or emergency-services integrations
- Docker Compose, Kubernetes, Nomad jobs, Terraform, or provider-specific deployment code

Any future deployment automation must preserve main/private-admin route
separation and must not claim production readiness until the access-control,
retention, backup, restore, observability, and abuse-control work exists.

## Implementation Order

Preferred implementation sequence:

1. Add configuration scaffolding for backend selection while preserving current defaults. Implemented for `sqlite`, `local`, `s3`, and `none`.
2. Introduce metadata and blob-store interfaces around the current SQLite and filesystem implementations. Implemented.
3. Add S3-compatible blob storage as an optional backend. Implemented for committed encrypted chunks.
4. Add PostgreSQL metadata support as an optional backend. Implemented.
5. Add explicit idempotency and upload-operation semantics for complete chunk
   upload retries. Implemented for SQLite and optional PostgreSQL metadata.
6. Add optional Valkey/Redis-compatible coordination. Implemented for explicit
   configuration, startup checks, route-class counters, and short-lived
   complete-upload coordination.
7. Update deployment, backup, restore, security, and threat-model docs before
   recommending any production cluster deployment. Initial cluster backup,
   restore, and failure guidance is documented in
   [Cluster backup, restore, and failure runbook](cluster-backup-restore-runbook.md).

Each step should be small, reviewable, and tested against the existing SQLite/filesystem path before adding new backend-specific behavior.

## Documentation Updates Required Before Implementation

Implementation PRs for this scope should update source-of-truth docs together, as applicable:

- `README.md`
- `AGENTS.md`
- `SECURITY.md`
- `docs/architecture.md`
- `docs/configuration.md`
- `docs/api.md`
- `docs/deployment.md`
- `docs/security-model.md`
- `docs/threat-model.md`
- `docs/retention-backup-deletion.md`
- `docs/code-map.md`
- release and Deep Research report prompts when review scope changes

Backlog issues should be created or updated for each backend and cluster-safety milestone before implementation work starts.
