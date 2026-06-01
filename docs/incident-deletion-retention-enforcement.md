# Incident Deletion And Retention Enforcement

This document describes the implemented baseline for incident deletion and
retention enforcement in Proofline Server, plus remaining future work.

The backend implements private owner-scoped and admin-global deletion request
routes, durable deletion decisions, deletion item retry state, SQLite and
PostgreSQL metadata support, blob deletion through the storage boundary, and an
automatic background scheduler. It also includes local read-only operator
commands for retention preview and deletion job status. It does not add public
`/v1` exposure, public account workflows, backend decryption, key custody, key
escrow, mode-specific retention, object-bucket lifecycle enforcement, or
playable media export.

## Current Behavior

The current backend preserves accepted evidence by default:

- incidents remain until explicit operator action outside the application
- closing an incident stops later uploads but does not delete evidence
- open, complete, and failed media streams remain with the incident
- checkins remain with the incident
- incident viewer token metadata remains as token hashes only, including
  expired and revoked tokens
- encrypted chunk blobs are stored on local disk by default or in optional
  S3-compatible object storage when explicitly configured
- encrypted ZIP bundles are generated on demand and are not persisted by the
  server
- temporary upload files are normally removed after upload success or failure,
  and explicit startup cleanup can remove old crash-orphaned `upload-*` staging
  files from the local temp directory

The backend now has private deletion APIs and a background deletion worker:

- `POST /v1/incidents/{incident_id}/deletion` lets the owning account request
  deletion for its own incident
- `POST /v1/admin/incidents/{incident_id}/deletion` lets an admin request
  deletion for any incident
- matching private `GET` routes return non-sensitive deletion status
- `SAFE_DELETION_WORKER_INTERVAL` controls the automatic scheduler and defaults
  to `1m`
- `SAFE_CLOSED_INCIDENT_RETENTION` is disabled by default and, when positive,
  queues retention-policy deletion for closed incidents older than the window
- `SAFE_TOKEN_METADATA_RETENTION` is disabled by default and, when positive,
  prunes expired or revoked incident-token metadata after the audit window
- `SAFE_DELETION_TOMBSTONE_RETENTION` is disabled by default and, when positive,
  prunes completed minimal tombstones after the retention window
- `operator retention-preview` previews closed incidents that would match a
  retention window without creating deletion decisions
- `operator deletion-status` reports deletion decision counts, retry
  categories, and runnable jobs without exposing stored paths

Manual database or blob deletion outside the application should be avoided
because the metadata and encrypted blob stores need to stay consistent.

## Goals

- Preserve uploaded evidence unless there is an explicit deletion decision.
- Keep public incident viewer routes read-only.
- Keep deletion entry points private/admin-only.
- Delete encrypted blobs only through server-controlled stored paths from
  metadata, never through client-provided filesystem paths, object keys, or
  object-store URLs.
- Make deletion and retention enforcement idempotent so interrupted work can be
  retried safely.
- Keep metadata, local blobs, and S3-compatible blobs consistent enough that a
  partially failed deletion is visible, retryable, and fails closed.
- Treat open and failed streams as possible evidence unless a deletion decision
  explicitly covers them.
- Define how retention windows interact with closed incidents, token metadata,
  backups, and future incident modes.
- Record non-sensitive audit information without logging raw viewer tokens,
  request bodies, uploaded bytes, plaintext, raw keys, or sensitive evidence
  metadata.

## Non-Goals

- No public deletion route or public deletion status route.
- No deletion of generated bundles, downloaded copies, backups, reverse-proxy
  caches, snapshots, or endpoint copies.
- No public admin routes, public `/v1` exposure, OAuth, JWT, public account
  workflows, cloud service automation, Docker Compose, Kubernetes, or public
  dashboard.
- No promise of unrecoverable secure erasure from normal file, object, or
  database row deletion.
- No backend decryption, raw server-held media keys, key escrow, key sharing,
  browser decryption, or key-custody change.
- No attempt to delete copies already downloaded by browsers, trusted contacts,
  operators, reverse proxies, backup systems, or endpoint devices.

## Deletion Decisions

Every incident deletion begins with an explicit deletion decision. A decision
may be created by the owner-scoped main route, the admin-global private-admin
route, or the retention policy worker. It must not be created by public
incident viewer routes.

A deletion decision should record:

- target incident ID
- requested action, such as `delete_incident`
- decision source, such as `account_request`, `admin_request`, or
  `retention_policy`
- reason code or policy ID, not free-form sensitive evidence details
- whether deleting an open incident is explicitly allowed
- requested time and the non-secret actor or process identifier
- current deletion state
- safe counts or summaries needed for review

Do not record raw viewer tokens, Authorization headers, request bodies,
uploaded bytes, plaintext, raw keys, precise stored paths, original filenames,
location values, notes, private deployment details, or object-store URLs in
operator-facing audit logs.

Open incidents should not be auto-deleted by retention policy. A manual
deletion decision may delete an open incident only when it explicitly says that
open evidence is covered. This prevents accidental loss while an upload session
is still active or ambiguous.

## Tombstone And Hard-Delete Model

Deletion should use a two-phase tombstone and hard-delete model.

The metadata backend should create durable deletion state before any blob is
removed. That state should snapshot the server-controlled stored paths that
need deletion and mark the incident as deletion-pending so normal readers fail
closed while deletion is in progress.

Recommended states:

| State | Meaning |
|---|---|
| `active` | Incident is not being deleted. |
| `deletion_pending` | A deletion decision exists and deletion items have been prepared. |
| `deleting` | A worker or CLI is deleting encrypted blobs and metadata. |
| `deletion_failed` | Deletion stopped with retryable failures. |
| `deleted` | Encrypted blobs and sensitive child metadata have been removed or confirmed absent. |

The current `open` and `closed` incident statuses should keep their existing
meaning. Deletion state should be separate so closing an incident never implies
deletion and deletion does not overload upload state.

After deletion finishes, the backend may retain a minimal tombstone for
idempotency and audit. The tombstone should contain only non-sensitive fields,
such as incident ID, deletion timestamps, state, decision source, reason code,
safe item counts, and backend type summaries. It should not retain checkin
location values, notes, original filenames, raw token data, token hashes, stored
paths, object keys, or evidence-specific metadata longer than needed to finish
the deletion job.

Hard-delete should remove or prune sensitive child rows, including streams,
chunks, checkins, and incident viewer token rows, after the deletion process no
longer needs those rows for retry. If a future deployment needs to keep
tombstones for legal or operational reasons, that retention must be explicit
and documented.

## Consistency And Retry Model

Database transactions cannot atomically delete local filesystem blobs or
S3-compatible objects. Deletion therefore uses a durable
metadata-backed work queue, sometimes called an outbox pattern.

Recommended incident deletion flow:

1. Validate the private/admin deletion request or retention decision.
2. In one metadata transaction, confirm the incident exists, check whether open
   incident deletion is allowed, create or find the deletion decision, create
   deletion item rows from existing chunk stored paths, and mark the incident
   `deletion_pending`.
3. Return or continue only after durable deletion state exists.
4. Delete each encrypted blob through the storage backend using the
   server-controlled stored path from metadata.
5. Treat a missing blob as idempotent success only for a deletion item that was
   already created from server metadata.
6. Mark each deletion item deleted, or record a retryable error class without
   logging sensitive paths or private deployment details.
7. After all blob deletion items are complete, delete or prune sensitive child
   metadata in one metadata transaction.
8. Mark the tombstone `deleted`.

If the process fails after deletion state is created, retries should resume
from the deletion item table. A repeated request for the same incident should
return the existing deletion state instead of creating a competing deletion
operation. A repeated request for an already deleted incident should be a safe
success or a clear `410 Gone`-style private/admin response, depending on the
future API contract.

While an incident is `deletion_pending`, `deleting`, or `deleted`, public
viewer token lookups should fail closed with the same generic public failure
shape used for invalid, expired, or revoked tokens. Public routes must not
reveal whether a deletion exists.

## Blob Deletion Rules

Blob deletion must be driven by metadata. The deletion worker and
private/admin routes must not accept final stored paths from clients.

For local storage:

- stored paths must pass the existing safe relative path checks
- deletion should call the storage boundary with stored paths from metadata
- directory cleanup, if any, should remove only empty server-controlled
  directories below the configured data root
- path traversal, absolute paths, backslashes, and unsafe path segments must
  continue to be rejected

For S3-compatible storage:

- object keys must be derived from stored paths and the configured safe prefix
- deletion must not expose bucket URLs or raw object keys in public responses
  or logs
- object versioning, provider replication, lifecycle snapshots, and backup
  copies mean object deletion is not guaranteed secure erasure
- deployment runbooks must define backup and object lifecycle expiry separately

Blob deletion failures should keep the deletion job retryable. Metadata must
not be hard-deleted before the system has either deleted or explicitly
accounted for every blob deletion item.

## Retention Windows

Retention enforcement should be policy-driven and conservative.

Implemented policy settings cover closed-incident retention, token metadata
pruning, tombstone pruning, and orphan temp upload cleanup. Future policy
settings may still cover:

- whether failed streams inherit the incident retention window
- backup generation retention after an incident deletion

Open incidents should not be eligible for automatic retention expiry. Closing
an incident may start the retention window, but it must not itself delete
evidence.

Failed streams should be retained with their incident by default. They may
contain useful encrypted chunks even when they are not downloadable as completed
stream bundles. A first implementation should avoid stream-only retention
deletion unless a later issue designs stream-level deletion semantics.

Expired and revoked incident viewer token rows may be pruned after an audit
window. Token pruning must remove only stored token-hash metadata and labels;
raw tokens are not stored. Token pruning must not delete incidents, streams,
chunks, checkins, or blobs.

Completed minimal deletion tombstones may be pruned after a configured
retention window. Tombstone pruning must not run while deletion retry items
exist or while sensitive child metadata remains. Removing the final tombstone
means local metadata no longer records that incident ID as deleted, so operators
must rely on backup expiry, restore reconciliation, and any external audit logs
needed by the deployment.

Backups must be handled as a separate lifecycle. Deleting live metadata and
blobs does not remove older SQLite backups, PostgreSQL backups, S3 object
versions, filesystem snapshots, volume snapshots, reverse-proxy caches, or
downloaded bundles. Operators must configure backup expiry so deleted
incidents eventually age out of backup sets.

Future incident modes may require different defaults:

| Future mode | Retention implication |
|---|---|
| Emergency incident | May need a longer evidence and backup retention window. |
| Interaction record | May need a shorter default and stronger user-controlled deletion policy. |
| Safety check | May need retention tied to check completion, missed check-in policy, and escalation state. |
| Evidence note | May need separate retention for lightweight notes and attached encrypted media. |

No incident-mode-specific retention should be implemented until first-class
incident-mode, capture-profile, escalation-policy, and sharing-state fields
exist and the retention policy is updated before or alongside that
implementation. The future policy boundary is documented in
[mode-aware retention policy](mode-aware-retention-policy.md).

## Public And Private Entry Points

Implemented deletion entry points are clearly separated from public incident
viewer routes:

- account owner request: `POST /v1/incidents/{incident_id}/deletion`
- account owner status: `GET /v1/incidents/{incident_id}/deletion`
- admin-global request: `POST /v1/admin/incidents/{incident_id}/deletion`
- admin-global status: `GET /v1/admin/incidents/{incident_id}/deletion`

Deletion entry points must:

- run only on authenticated main `/v1` or private-admin surfaces
- require local account authentication and owner/admin authorization
- never be reachable from token-scoped public viewer routes
- never accept client-provided stored paths, filesystem paths, object keys, or
  object-store URLs
- return idempotent status for repeated deletion attempts
- avoid logging raw tokens, request bodies, uploaded bytes, plaintext, raw keys,
  private deployment details, or sensitive evidence metadata

Local read-only operator commands are available through the server binary:

```bash
proofline-server operator retention-preview --closed-incident-retention 720h
proofline-server operator deletion-status
```

The preview command uses the configured metadata backend and reports closed
incident IDs and update times that match the requested retention window. It does
not queue deletion decisions. The status command reports deletion decision
counts, retry categories, and runnable jobs. Both commands produce JSON and
must be run from a trusted local operator environment with the same metadata
configuration as the server. They must not be exposed through public viewer
routes or public dashboards.

Public incident viewer routes must remain read-only. They should never expose
deletion controls, deletion job status, tombstone details, retention policy, or
blob deletion errors.

## Audit Fields

Future audit records should be useful for operational review without becoming a
new source of secrets or sensitive evidence.

Safe audit fields may include:

- audit event ID
- incident ID
- deletion decision ID
- action name, such as `deletion_requested`, `deletion_started`,
  `blob_delete_failed`, `metadata_pruned`, or `deletion_completed`
- actor type, such as `operator_cli`, `private_admin_api`, or
  `retention_policy`
- reason code or policy ID
- deletion state
- non-sensitive item counts
- metadata backend type and blob backend type
- timestamps

Audit records should avoid:

- raw viewer tokens or future token-like values
- token hashes unless a specific internal audit design requires them
- request bodies
- uploaded bytes
- plaintext
- raw keys
- Authorization headers
- checkin location values
- notes
- original filenames
- stored paths, object keys, bucket names, object URLs, and private endpoints
- user safety narrative or exploit details

Detailed retry state that needs stored paths should remain in internal
deletion item tables, not in public issue drafts, public logs, dashboards, or
operator-facing audit summaries.

## Backup And Restore Interaction

Deletion and retention enforcement must be documented with backup and restore
behavior before real-world use.

Deletion should not claim completion across the full deployment until the
operator understands backup retention. Live deletion can mark the active
metadata and blob backends deleted while older backups still contain recoverable
encrypted evidence.

Restore drills should verify both sides of the lifecycle:

- a restored active incident can still reconstruct completed encrypted bundles
- a restored deleted incident remains deleted or is clearly marked as a
  tombstone
- restoring from an older backup may resurrect data that was deleted after the
  backup was taken, unless backup expiry or key retirement prevents it
- public viewer routes still fail closed for deleted incidents after restore

If a restore reintroduces an incident that was deleted in live state, the
operator must have a documented reconciliation process. That process is
deployment-specific and should not rely on public routes.

## Remaining Future Implementation Tasks

The first backend implementation is in place. Remaining work should be split
into separate issues.

Repository and metadata tasks:

- add optional retention policy settings beyond closed-incident age
- preserve SQLite support and optional PostgreSQL support

Storage tasks:

- add empty-directory cleanup for local storage if needed
- add object-store lifecycle guidance for deployments with S3 versioning or
  replication
- avoid exposing object-store keys, bucket URLs, private endpoints, or local
  filesystem paths in logs and responses

Private-admin or CLI tasks:

- add a local operator CLI to request incident deletion
- extend local previews if future deletion policies need additional candidate
  classes
- keep all deletion controls off token-scoped public viewer routes

Retention tasks:

- defer incident-mode-specific retention until mode-driven retention behavior and
  policy are explicitly designed in
  [mode-aware retention policy](mode-aware-retention-policy.md)

Test tasks:

- keep optional S3-compatible deletion smoke coverage current as deletion,
  retention, and blob-store behavior changes
- test failed stream retention and deletion with the parent incident
- test backup and restore documentation examples where practical

Documentation tasks:

- update deployment and backup runbooks for live deletion, backup expiry, and
  restore reconciliation
- document that normal deletion is not guaranteed secure erasure
