# Cluster-Safe Upload Operation Semantics

This document tracks cluster-safe upload operation semantics for Proofline
Server.

The current backend implements a narrow `Idempotency-Key` path for complete
chunk uploads and optional Valkey/Redis-compatible short-lived complete-upload
leases. Remaining cluster semantics, including resumable uploads,
partial-upload lease sessions, changes to the current local account/session
model, public `/v1` exposure, browser decryption, backend decryption, key
custody, and playable media export are still not implemented.

## Current Behavior

The current backend accepts one complete encrypted chunk in a multipart upload,
streams it to local temporary storage while computing SHA-256, verifies the
client-provided ciphertext hash, commits the file to an immutable
server-controlled path, and inserts chunk metadata into the configured metadata
backend.

Accepted chunks are immutable. Duplicate chunk identities without an
idempotency key return `409 duplicate_chunk`, and the storage layer refuses to
overwrite an existing committed blob. When `Idempotency-Key` is supplied for
`POST /v1/incidents/{incident_id}/chunks`, equivalent complete-upload retries
can return `200 OK` with `Idempotency-Replayed: true`.

That local-first behavior remains supported. The implemented idempotency state
is durable metadata in SQLite or optional PostgreSQL. Further cluster-safe
semantics are additive work for optional PostgreSQL metadata and
S3-compatible object storage backends.

## Goals

- Allow clients and API nodes to retry ambiguous chunk uploads safely.
- Prevent duplicate committed evidence for the same chunk identity.
- Distinguish equivalent retries from conflicting duplicate attempts.
- Keep committed encrypted chunks immutable and never overwrite evidence.
- Keep PostgreSQL metadata and object-storage blobs consistent enough for
  bundle reconstruction.
- Keep Valkey/Redis-compatible coordination optional and non-durable.
- Preserve the current ciphertext-only backend posture.

## Non-Goals

- No resumable upload protocol or partial committed chunks.
- Complete-upload Valkey leases are in-progress hints only; they do not store
  idempotency results or committed evidence truth.
- No public `/v1` exposure, public account workflows, or changes to the current
  local account/session model.
- No client repository, protocol repository, or mobile implementation.
- No backend decryption, raw server-held keys, key escrow, key sharing, or
  playable media export.

## Terminology

Chunk identity is the final evidence identity enforced by metadata uniqueness
constraints.

Upload operation identity is the retryable write operation identity used while
one client attempt is being staged, committed, confirmed, or replayed.

Request fingerprint is the immutable metadata and ciphertext summary that must
match for a retry to be considered equivalent.

Equivalent retry success means the same intended chunk is already committed and
its stored metadata matches the retry request, so the server can return success
without writing a second blob or row.

Conflict means the same chunk identity or idempotency key is being used with
different ciphertext or immutable metadata.

## Chunk Identity

Cluster implementations must preserve the existing chunk identities.

Streamed chunks:

```text
incident_id + stream_id + chunk_index
```

Legacy unstreamed chunks:

```text
incident_id + media_type + chunk_index
```

For streamed chunks, `media_type` is still part of validation and the request
fingerprint because the request media type must match the stream media type. It
is not the uniqueness key for streamed chunks because streams already provide a
media-specific namespace.

For legacy unstreamed chunks, `stream_id` is absent and `chunk_index = 0`
remains valid for compatibility.

PostgreSQL uniqueness constraints should be equivalent to or stronger than the
current SQLite behavior:

- one unique constraint or partial index for `(incident_id, stream_id,
  chunk_index)` where `stream_id IS NOT NULL`
- one unique constraint or partial index for `(incident_id, media_type,
  chunk_index)` where `stream_id IS NULL`

These constraints are the final duplicate guard. Idempotency state helps
produce retry-safe responses, but it must not replace durable chunk uniqueness.

## Upload Operation Identity

Clients can send a stable idempotency key for each intended complete chunk
upload using an `Idempotency-Key` header on
`POST /v1/incidents/{incident_id}/chunks`.

The server should bind the idempotency key to:

- route operation, currently `upload_chunk`
- incident ID
- streamed chunk identity or legacy unstreamed chunk identity
- request fingerprint

Idempotency keys should be treated as token-like request identifiers: do not
log raw values, do not include raw values in public errors, and store a stable
hash instead of the raw key when durable lookup is needed.
Future reverse-proxy, tracing, metrics, and error-reporting guidance must also
avoid recording raw `Idempotency-Key` values or using them as labels,
attributes, object keys, or log fields.

Clients without an idempotency key remain supported through the existing chunk
identity constraints and still receive `409 duplicate_chunk` for duplicate
identities. Equivalent retry success is only part of the idempotency-key path.
The current implementation still requires a complete encrypted chunk retry; it
does not resume an in-progress partial transfer.

## Request Fingerprint

The request fingerprint should include all client-controlled fields that become
immutable chunk metadata or verify the ciphertext:

- normalized chunk identity
- `media_type`
- `started_at`
- `ended_at`
- normalized `original_filename`, including empty value
- uploaded ciphertext byte size
- `sha256_hex` over the complete uploaded ciphertext bytes

The fingerprint must not include server-generated fields such as chunk ID,
stored path, creation time, operation row ID, or object-storage staging key.

A retry is equivalent only when the chunk identity and request fingerprint
match. A different ciphertext hash, byte size, time range, media type, or
original filename is a conflict, even if the chunk identity is the same.

## Durable Idempotency State

Durable upload operation state belongs in the metadata backend. SQLite and
optional PostgreSQL store upload-operation records for the implemented
complete-upload idempotency path. For future cluster deployments, PostgreSQL
remains the expected source of truth for idempotency and upload operation
records.

Valkey/Redis-compatible coordination may hold leases and in-progress hints,
but it must not be the durable source of truth for committed chunk metadata,
committed chunk bytes, or completed idempotency decisions.
The implemented Valkey path uses short-lived complete-upload leases keyed by a
server-controlled hash of normalized chunk identity. A busy lease returns
`409 upload_in_progress` with `Retry-After`; a coordination failure returns
`503 upload_coordination_unavailable`. Both responses are retryable and omit
backend details and sensitive request values.

The implemented `upload_operations` table tracks:

- operation ID generated by the server
- hashed idempotency key
- operation type, currently `upload_chunk`
- normalized chunk identity fields
- request fingerprint fields and a stable fingerprint hash
- state, currently `reserved` or `metadata_committed`
- final chunk ID when metadata has been committed or confirmed
- final stored path or object key when known
- timestamps for creation and last update

Future cluster work may add staging, blob-committed, failed, ownership-proof,
expiry, and cleanup fields when operation-level staging or leases are
implemented.

Completed chunk metadata remains the durable evidence index. Idempotency rows
may support a retry window or operational audit trail, but expiring an
idempotency row must not remove committed chunk rows or committed blobs.

## Commit Flow

The current complete-upload idempotency path uses the metadata backend for
reservation, replay lookup, conflict detection, and completion. When optional
Valkey coordination is configured, the server acquires a short-lived
complete-upload lease after it has staged and verified the encrypted request
body. That lease reduces duplicate final commit and metadata work; it is not a
resumable-transfer or bandwidth-saving lease.

1. Validate route parameters and multipart metadata, stage uploaded ciphertext
   in a local temp file while computing SHA-256, and enforce the upload byte
   limit.
2. Compare the computed SHA-256 with `sha256_hex`. On mismatch, remove staging
   bytes and return `400 hash_mismatch`.
3. Recheck incident and stream state in the metadata backend before final blob
   commit. Streamed uploads must still require an open stream with matching
   media type.
4. Acquire a short-lived Valkey complete-upload lease when coordination is
   configured. If the lease is busy, return a retryable in-progress response.
5. Reserve or find the upload operation in the metadata backend when an
   idempotency key is supplied.
6. If the same idempotency key already completed with the same fingerprint,
   return equivalent success using the committed chunk metadata.
7. If the same idempotency key exists with a different chunk identity or
   request fingerprint, return `409 idempotency_conflict`.
8. Commit the staged object to the final server-controlled immutable object key
   using conditional no-overwrite behavior.
9. Insert chunk metadata in the metadata backend, or confirm the existing chunk
   row when a race already inserted an equivalent row.
10. Mark the upload operation `metadata_committed` with the final chunk ID.
11. Release the Valkey lease if one was acquired; TTL expiry must still be safe
    if release fails.
12. Remove local temp staging state.
13. Return success only after final encrypted bytes exist outside staging and
    chunk metadata has been inserted or confirmed.

The final object key should remain server-controlled and derived from the
normalized chunk identity. Clients must never provide stored paths or final
object keys.

## Equivalent Success

Equivalent success is allowed when all of the following are true:

- the normalized chunk identity matches
- the existing chunk row belongs to the same incident and, for streamed chunks,
  the same stream
- the existing chunk row has the same request fingerprint
- the committed blob exists and matches the stored byte size and `sha256_hex`
  when the backend can verify this cheaply or as part of the operation

The implemented HTTP behavior is:

- `201 Created` when this request created the chunk row
- `200 OK` when a retry or racing request confirms an already committed
  equivalent chunk
- the same chunk metadata shape for both responses
- an optional response header, such as `Idempotency-Replayed: true`, for
  clients that need to distinguish replayed success without changing the JSON
  body

The exact response contract is documented in [api.md](api.md).

## Conflicts

The server must return a conflict and must not overwrite evidence when:

- the same idempotency key is reused for a different chunk identity
- the same idempotency key is reused with a different request fingerprint
- the same chunk identity already exists with a different ciphertext hash
- the same chunk identity already exists with different immutable metadata
- a final object already exists but cannot be proven equivalent to the request
  and committed metadata

Recommended future error codes:

- `409 idempotency_conflict` for idempotency-key reuse with different inputs
- `409 duplicate_chunk` for duplicate uploads without idempotency replay, or
  `409 duplicate_chunk_conflict` from the private reconciliation route for same
  chunk identity but different ciphertext or metadata
- `409 upload_in_progress` with `Retry-After` when an equivalent operation is
  still actively staging or committing and no committed chunk row is available

Do not include uploaded bytes, plaintext, raw keys, raw tokens, raw idempotency
keys, request bodies, final stored paths, staging paths, or object-storage
credentials in conflict responses.

## Object Storage Commit Rules

S3-compatible storage should use conditional writes or an equivalent
no-overwrite primitive for final objects. A final object write that would
replace existing evidence must fail.

For object stores that support object versioning, ETags, conditional deletes,
or user metadata, the implementation should record enough server-controlled
ownership proof to decide whether this operation created a final object that is
safe to remove after a later metadata failure.

If a final object exists without a committed chunk row, the system should treat
that state as ambiguous. It may repair or clean up the object only when
operation state and object metadata prove ownership and no committed chunk row
references it. Otherwise, it should fail closed with a retryable operational
error rather than overwrite or delete possible evidence.

## Cleanup

Cleanup must distinguish staging state from committed evidence.

Safe cleanup targets:

- local temporary upload files
- operation-specific object-storage staging keys
- expired upload operation rows that have no committed chunk row
- final objects created by a failed operation only when ownership proof shows
  this operation created the object and no committed chunk row references it

Unsafe cleanup targets:

- committed chunk rows
- committed final blobs referenced by chunk metadata
- final blobs that cannot be tied safely to a failed operation
- any object path or stored path provided by a client

Cleanup should be conservative. Leaving a server-owned staging object for a
later lifecycle rule is preferable to deleting evidence that might have been
committed by another API node.

## Relationship To Other Issues

Issue `#85`, "Design Duplicate Chunk Reconciliation API", chooses a separate
private query workflow for already committed duplicate chunk identities. The
implemented route compares a client's expected ciphertext hash and immutable
metadata with an accepted chunk row without re-uploading ciphertext, overwriting
evidence, or exposing bytes, stored paths, raw tokens, plaintext, keys, or
conflicting stored values. The upload route covers idempotency-key equivalent
success for complete retries; duplicate reconciliation is the fallback for
clients that only know the final chunk identity and expected fingerprint.

Issue `#86`, "Plan Resumable Upload And Upload Lease Protocol", should decide
whether incomplete transfers need explicit leases, resumable multipart upload,
or a simpler complete-chunk retry model. This document does not make partial
uploads visible as committed evidence and does not design a resumable transfer
protocol. That follow-up decision is documented in
[resumable-upload-lease-protocol.md](resumable-upload-lease-protocol.md), which
keeps the local desktop recorder simulator on complete encrypted chunk retries
while deferring resumable uploads and partial-upload lease sessions.

This document focuses on the commit and confirmation semantics once an upload
attempt has enough data to compute the ciphertext hash and decide whether a
final chunk can be committed.

## Required Future Work

API documentation:

- document reverse-proxy, tracing, metrics, and error-reporting redaction for
  raw idempotency keys
- document the relationship between duplicate reconciliation and idempotent
  retry success

Backend tests:

- HTTP tests for same chunk identity with different ciphertext
- cleanup tests proving future staging cleanup does not delete committed
  evidence
- race tests for upload versus incident close and stream completion

Simulator changes:

- retry an upload after simulated ambiguous response loss
- verify conflict behavior for same chunk identity with different ciphertext
- keep the existing hash-mismatch and bundle verification flows

Backend-specific work:

- extend the blob-store boundary for operation-specific staging and conditional
  final object commit
- decide how Valkey/Redis-compatible coordination should be used for leases or
  reducing duplicate in-progress work
- update deployment, backup, restore, security, threat-model, retention, and
  code-map docs before recommending production cluster deployment
