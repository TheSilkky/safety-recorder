# Retention, Backup, And Deletion

This document defines the operational retention, backup, restore, and deletion
policy for the current Proofline backend shape. The backend implements
authenticated incident deletion requests, a durable deletion queue, and an automatic
background worker for deletion and optional closed-incident retention, plus
read-only local operator status and retention preview commands. The worker can
also prune expired/revoked viewer-token metadata and completed minimal
tombstones when explicit retention windows are configured. It does not add cloud
backups, key escrow, backend decryption, mode-specific retention, or
object-bucket lifecycle policy enforcement.

Incident deletion and retention enforcement details are documented in
[incident-deletion-retention-enforcement.md](incident-deletion-retention-enforcement.md).

Proofline is still experimental and not production-ready public infrastructure. Before real-world use, an operator must choose concrete retention windows, backup locations, encryption settings, and restore checks that match the user's safety, privacy, and legal risk model.

## Scope

The current backend stores:

- SQLite metadata at `SAFE_DB_PATH` by default, or PostgreSQL metadata when
  `SAFE_METADATA_BACKEND=postgresql`
- encrypted chunk blobs under `SAFE_DATA_DIR` for the local backend, or committed encrypted objects in the configured S3-compatible bucket for the S3 backend
- temporary upload files under `SAFE_DATA_DIR/tmp`
- on-demand encrypted ZIP bundle responses generated from completed streams

Optional Valkey/Redis-compatible coordination is not durable evidence storage
and is not a backup source of truth. Any current or future coordination keys
must be treated as short-lived operational state.

The backend stores ciphertext only. It does not store raw media keys, decrypt chunks, produce playable media, or persist generated ZIP bundle files.

Incident mode metadata such as emergency incidents, interaction records, safety
checks, and evidence notes may eventually need different retention defaults. The
current fields are metadata only and do not change retention. Any future
mode-specific retention behavior must update this policy before or alongside
implementation. The future policy boundary is documented in
[mode-aware retention policy](mode-aware-retention-policy.md).

## Retention Principles

- Preserve uploaded evidence unless there is an explicit deletion decision.
- Keep metadata and encrypted blobs in sync; either both are retained, or both
  are removed by the deletion workflow.
- Treat failed and open streams as possible evidence. Do not discard them just because they are not downloadable as completed stream bundles.
- Keep raw viewer/incident tokens out of storage and logs. Only token hashes are retained in metadata.
- Treat non-emergency interaction records as potentially sensitive even when they are not urgent safety incidents.
- Do not promise unrecoverable deletion from normal file removal.
- Use disk or volume encryption so eventual deletion can rely on cryptographic key destruction, backup expiry, and media retirement instead of overwrite claims.

## Current Retention Choices

The current implementation is evidence-preserving by default. It does not
delete incidents automatically unless `SAFE_CLOSED_INCIDENT_RETENTION` is set
to a positive duration. Explicit private owner-scoped and admin deletion
requests can create deletion decisions at any time, subject to authorization and
the open-incident guard.

| Data | Current retention choice | Notes |
|---|---|---|
| Incidents | Retain open and closed incident rows until an explicit deletion request or configured closed-incident retention decision. | Closing an incident stops later uploads but does not itself delete evidence. Deleted incidents keep a minimal tombstone. |
| Chunks | Retain every accepted encrypted chunk with its incident until incident deletion. | Uploaded chunks are immutable and must not be overwritten. Deletion snapshots stored paths from metadata before blob removal. |
| Media streams | Retain open, complete, and failed stream metadata with the incident until incident deletion. | Failed streams may still contain useful uploaded chunks and are deleted with the parent incident. |
| Checkins | Retain checkin rows with the incident until incident deletion. | Checkins may contain location and device-status metadata, so deletion prunes them with the incident. |
| Viewer token rows | Retain token-hash metadata with the incident until incident deletion, including expired and revoked tokens. | Raw tokens are returned only once and are not stored. Future pruning may remove expired or revoked token rows after an audit window. |
| Generated ZIP bundles | Do not retain on the server. | Stream and incident bundles are generated on demand as HTTP responses. Downloaded copies are outside backend control. |
| Temporary upload files | Remove after successful commit or failed upload cleanup. | Orphaned temp files may exist after crashes and need a future cleanup policy. |

Before real-world use, choose explicit local policy values such as:

- how long closed incidents should remain available
- whether emergency incidents, interaction records, safety checks, and evidence notes need different retention windows
- whether failed streams should follow the same retention window as completed streams
- how long expired or revoked token metadata is useful for audit
- how long backup generations are retained after an incident is deleted
- who is allowed to request deletion and who can approve it

## Backup Policy

Backups must preserve the relationship between metadata and encrypted blobs. A database backup without the matching local blob tree or S3 object set may leave bundles unusable. A blob backup without the matching database may leave evidence hard to locate, verify, or serve.

Optional PostgreSQL metadata has the same consistency requirement. The
PostgreSQL schema, migration, and restore expectations are documented in
[postgresql-metadata-migration.md](postgresql-metadata-migration.md). SQLite
remains the default metadata store.

For cluster-style deployments using optional PostgreSQL metadata,
S3-compatible encrypted blobs, and optional Valkey/Redis-compatible
coordination, use the
[cluster backup, restore, and failure runbook](cluster-backup-restore-runbook.md)
alongside this policy.

Back up at least:

- `SAFE_DB_PATH`
- the PostgreSQL database when `SAFE_METADATA_BACKEND=postgresql`
- `SAFE_DATA_DIR/incidents` when `SAFE_BLOB_BACKEND=local`
- the configured S3 bucket and `SAFE_S3_PREFIX` object set when `SAFE_BLOB_BACKEND=s3`
- SQLite sidecar files if copying a live database directly, including
  `<SAFE_DB_PATH>-wal` and `<SAFE_DB_PATH>-shm` when present
- deployment configuration needed to restore backend selectors, bind addresses, data paths, upload limits, token TTL defaults, and reverse-proxy routing

Do not treat Valkey/Redis-compatible coordination data as a substitute for
metadata or blob backups. Loss of coordination state must be recoverable through
durable metadata, immutable committed blobs, and client retry behavior.

If `SAFE_DB_PATH` points outside `SAFE_DATA_DIR`, include both locations in the same backup set.

Use one of these consistency strategies:

- stop the API process, then copy the database and blob tree
- take an atomic filesystem or volume snapshot that includes both database and blobs
- use SQLite's backup mechanism for the database and coordinate it with a blob snapshot taken while uploads are paused
- pause uploads, back up SQLite with its live state, and take an S3 bucket or prefix inventory/copy for the matching committed objects

Do not copy only `safety.db` from a running WAL-mode database and assume that
is a complete backup. Include the live SQLite state correctly, including WAL
sidecar files when using a direct live copy, or use a database backup
operation. The file name still uses `safety.db` until a separate data-layout
migration is performed. SQLite WAL operational notes, same-host storage
expectations, and simple local size checks are documented in
[deployment.md](deployment.md#sqlite-wal-operations).

Backups should be encrypted at rest and access-controlled. Backup logs, filenames, tickets, and monitoring should not contain raw viewer tokens, private deployment details, request bodies, uploaded bytes, plaintext, or raw keys.

## Restore Expectations

Restores must be tested before relying on the system for real incidents.

A restore test should:

1. Restore SQLite, including any needed WAL live state, and blobs into an
   isolated staging path or isolated S3 bucket/prefix.
2. Start the API with local or otherwise reviewed private-boundary bind
   addresses only.
3. Load known incident metadata through authenticated main `/v1` routes.
4. Verify completed stream or incident bundle downloads can be generated.
5. Confirm generated manifests match expected stream and chunk metadata.
6. Confirm missing blobs or database/blob mismatches fail closed rather than producing partial evidence.

The restore target must preserve the main/private-admin listener split. Do not
use a restore drill as a reason to expose `/v1` publicly.

For S3-compatible storage, restore drills must verify the configured bucket,
prefix, credentials, and endpoint can reconstruct completed stream and incident
bundles without exposing object-store URLs. For a PostgreSQL deployment,
restore drills must also restore the PostgreSQL metadata database and encrypted
blob storage as one logical evidence set, then verify completed stream and
incident bundles before any public viewer exposure.

## Deletion Policy

Incident deletion is an application-level private workflow. A deletion begins
with a durable deletion decision, snapshots server-controlled stored paths from
chunk metadata, marks the incident `deletion_pending`, and then the background
worker deletes encrypted blobs through the configured blob backend. After all
blob deletion items are complete or confirmed absent, the worker prunes
sensitive child metadata and leaves a minimal tombstone.

Deletion behavior:

- account-scoped deletion is available at `POST /v1/incidents/{incident_id}/deletion` for the incident owner
- admin-global deletion is available at `POST /v1/admin/incidents/{incident_id}/deletion`
- deletion status is available through the matching private `GET` routes
- local read-only operator status is available through
  `proofline-server operator deletion-status`
- local read-only closed-incident retention preview is available through
  `proofline-server operator retention-preview --closed-incident-retention <duration>`
- encrypted blob files or objects are removed by server-controlled stored paths only
- client-provided filesystem paths, object keys, and object-store URLs are never accepted for deletion
- repeated deletion requests return the existing deletion status instead of creating competing work
- public incident viewer routes remain read-only and fail closed for deleting or deleted incidents
- open incidents are rejected unless the request explicitly sets `allow_open: true`
- deletion decisions retain only non-sensitive status fields, such as decision ID, incident ID, source, reason code, actor account ID, item count, timestamps, state, and error class

Current deletion policy still distinguishes:

- deleting one incident
- expiring closed incidents after an operator-defined retention window through
  `SAFE_CLOSED_INCIDENT_RETENTION`
- pruning expired or revoked viewer-token metadata after an operator-defined
  audit window through `SAFE_TOKEN_METADATA_RETENTION`
- pruning completed minimal deletion tombstones after an operator-defined
  window through `SAFE_DELETION_TOMBSTONE_RETENTION`
- applying different retention to emergency incidents, interaction records,
  safety checks, and evidence notes after incident-mode, capture-profile,
  escalation-policy, and sharing-state fields exist
- identifying orphaned blobs or rows after interrupted manual operations
- deleting downloaded bundles or plaintext exports if such derived files are ever implemented

Completed stream and incident ZIP bundles are not currently stored by the server, so server-side incident deletion cannot delete copies already downloaded by trusted contacts, operators, browsers, reverse proxies, backup systems, or endpoint devices.

## Secure Deletion Limits

Normal file or object deletion removes a path or object reference. It does not guarantee that encrypted blob bytes, SQLite pages, WAL contents, filesystem journal blocks, SSD wear-leveling copies, object-store replicas, versioned objects, lifecycle snapshots, volume snapshots, backups, caches, or downloaded bundles are unrecoverable.

Do not document or operate Proofline as if `os.Remove`, `rm`, database row deletion, or SQLite vacuuming provides guaranteed secure erasure on modern storage.

The recommended posture is:

- use full-disk, volume, dataset, filesystem, or object-bucket encryption for `SAFE_DATA_DIR`, `SAFE_DB_PATH`, S3-compatible blobs, logs, and backups
- keep encryption keys outside the backup set they protect
- retire or destroy storage-encryption keys when decommissioning a deployment or backup generation
- enforce backup retention so deleted incidents eventually age out of all backup copies
- document where downloaded evidence bundles may land outside server control
- avoid persistent derived plaintext exports unless a future explicit design covers their retention and deletion

If a deployment needs stronger per-incident deletion guarantees, that should be designed as future security-sensitive work. For example, per-incident encrypted data keys could make key destruction part of deletion, but that would interact with key custody, emergency access, backups, and restore testing. It must not be introduced incidentally.

## Disk Encryption Posture

Disk or volume encryption is expected for any deployment that stores real incident evidence. It protects data at rest when disks, snapshots, or offline backups are lost, stolen, retired, or later deleted.

Recommended deployment posture:

- encrypt the host disk or the volume containing `SAFE_DATA_DIR`
- enable appropriate bucket-side encryption and access controls when `SAFE_BLOB_BACKEND=s3`
- encrypt the database path if `SAFE_DB_PATH` is outside `SAFE_DATA_DIR`
- encrypt backup storage separately from the live host
- restrict access to encryption keys and recovery keys
- test boot and restore procedures so encryption does not make urgent evidence unavailable
- document who can unlock production storage during an emergency or restore operation

Disk encryption does not protect data while the host is running and unlocked,
and it does not replace reviewed main `/v1` deployment boundaries, TLS at the
edge, token handling, rate limiting, backup access control, or future
application-level authorization.

## Remaining Future Work

The implemented deletion workflow does not complete every lifecycle policy
area. Remaining future work is expanded in
[incident-deletion-retention-enforcement.md](incident-deletion-retention-enforcement.md).
Likely future work includes:

- retention policy fields or settings for mode-driven incident, capture-profile,
  escalation-policy, and sharing-state behavior
- a local operator CLI to request deletion decisions
- backup and restore runbooks with deployment-specific commands
- documentation updates for any future derived plaintext or persisted bundle outputs
