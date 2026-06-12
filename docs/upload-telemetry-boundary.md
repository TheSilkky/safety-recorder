# Upload Telemetry Boundary

This document is a planning boundary for future client upload telemetry. It
does not implement endpoints, database schema, route limits, retention jobs,
client code, queue-summary routes, resumable uploads, partial-upload lease
sessions, notification delivery, backend decryption, browser decryption, key
escrow, or public `/v1` exposure.

## Decision

Upload telemetry is not required before v1 preview. The current server evidence
truth remains accepted encrypted chunks, media stream state, idempotency-key
metadata, duplicate reconciliation, and completed encrypted bundles. Local
client queue state stays client-local unless a later implementation issue
proves that a narrow server-visible telemetry endpoint is necessary for
reliability support.

Do not add telemetry only because a local recorder can fail, retry, run out of
storage, or lose network access. Current clients and simulator flows should
record those details locally, retry complete encrypted chunks, use
`Idempotency-Key` for ambiguous complete-upload retries, and use duplicate
reconciliation when a duplicate chunk identity needs to be compared with
accepted server metadata.

## Non-Goals

- No telemetry endpoint for v1 preview.
- No public unauthenticated telemetry ingestion.
- No public admin, support, or diagnostics dashboard.
- No server-visible raw local queue, local file inventory, or local path list.
- No telemetry-driven evidence acceptance, deletion, retention, escalation,
  notification, sharing, or viewer behavior.
- No resumable-upload or partial-upload lease protocol.
- No backend decryption, browser decryption, raw-key access, key escrow, or
  key-sharing behavior.

## Future Endpoint Shape

If a later issue implements upload telemetry, it should be a main `/v1`
authenticated owner route scoped to one incident and, when relevant, one media
stream. It should accept only coarse operational counters and controlled reason
codes. It should not accept free-form diagnostics, local paths, filenames,
private deployment details, user safety narratives, exact GPS/location values,
raw tokens, request bodies, uploaded bytes, plaintext, raw keys,
wrapped-key ciphertext, object keys, staging paths, stored paths, or object
store credentials.

Allowed fields should be limited to values like:

- stream ID or chunk identity already known to the authenticated owner
- coarse telemetry window start and end timestamps
- coarse client state code, such as `queued`, `retrying`,
  `storage_pressure`, `network_unavailable`, `interrupted`, or
  `upload_blocked`
- controlled interruption or retry reason code
- bounded integer counts for queued chunks, retry attempts, upload failures,
  or dropped local capture intervals
- coarse byte-count buckets, if needed, rather than raw local file inventory
- client clock timestamp and monotonic sequence number for duplicate detection

Use allowlists for every code and apply strict numeric bounds. Reject unknown
fields rather than storing them for future interpretation.

## Evidence Truth

Telemetry must never become authoritative evidence state.
Current upload durability depends on accepted encrypted chunk metadata,
committed encrypted chunk bytes, and complete stream status, not telemetry
claims.

The server should continue to treat these as authoritative:

- accepted encrypted chunk metadata and immutable encrypted blobs
- media stream status and contiguous completion checks
- upload-operation records for complete-upload idempotency
- duplicate reconciliation results for already accepted chunk identities
- deletion and retention state

Telemetry must not:

- mark a chunk uploaded
- complete or fail a stream
- change idempotency or duplicate reconciliation decisions
- extend, shorten, or bypass retention
- change deletion eligibility
- grant viewer, trusted-contact, admin, or operator access
- trigger escalation, notification, or emergency-services behavior
- expose live or partial stream contents
- imply that local queued chunks are server-preserved evidence

Missing telemetry should not make accepted evidence invalid, and present
telemetry should not make missing evidence valid.

## Authentication And Authorization

A future telemetry route should require the same authenticated main `/v1`
session model as other owner-scoped incident routes. The authenticated account
must own the incident. Admin cross-account support access, trusted-contact
access, public viewer-token access, and browser/no-account viewer access should
not be added through telemetry.

If browser-cookie auth is allowed later, unsafe telemetry writes must use the
same CSRF protection as other cookie-authenticated unsafe `/v1` requests. Bearer
requests should continue using bearer authentication without CSRF.

Telemetry must have a route-limit class before implementation. Limiter labels
and keys must use server-controlled route classes and peer hashes, not account
emails, usernames, incident IDs, stream IDs, telemetry reason codes, raw
tokens, idempotency keys, object keys, local paths, or request bodies.

## Retention And Audit

Telemetry retention should be short and operational, not evidence retention.
A future implementation should define a default retention window before writing
schema, such as days rather than months, and should provide pruning behavior.

Audit records, if any, should use controlled action names, outcome categories,
actor account IDs, incident IDs, stream IDs, counts, and timestamps. They must
not include free-form client messages, local paths, filenames, GPS/location
values, raw tokens, request bodies, uploaded bytes, plaintext, raw keys,
wrapped-key ciphertext, stored paths, staging paths, object keys, private
deployment details, or user safety narratives.

Telemetry should not be copied into evidence bundles or public viewer payloads.

## Failure Behavior

Telemetry writes should fail closed and stay non-authoritative:

- missing telemetry: no effect on upload, bundle, retention, viewer, or
  escalation behavior
- malformed telemetry: reject with a safe validation error and do not store
  partial data
- delayed telemetry: accept only within the documented retention and clock-skew
  window, or reject safely
- duplicate telemetry: deduplicate by server-controlled identity plus client
  sequence or timestamp window without incrementing counters twice
- unauthorized telemetry: return the normal authenticated route error without
  revealing whether another account owns an incident
- rate-limited telemetry: drop or reject the telemetry without changing
  evidence state
- telemetry storage failure: return a safe retryable server error without
  changing upload or stream state

Responses should be small acknowledgements. They should not return stored
telemetry history, local queue summaries, private deployment details, backend
diagnostics, object keys, stored paths, uploaded bytes, plaintext, raw keys, or
token-like values.

## Validation Expectations For Future Implementation

Future implementation tests should cover:

- authentication required
- cross-account denial
- viewer-token and trusted-contact denial
- CSRF behavior if browser-cookie auth is supported
- route-limit classification
- strict field allowlists and numeric bounds
- rejection of local paths, filenames, location values, free-form notes, and
  unknown fields
- no effect on chunk upload, idempotency, duplicate reconciliation, stream
  completion, retention, deletion, escalation, sharing, or viewer payloads
- duplicate telemetry idempotence
- pruning or retention-window behavior
- log redaction for request bodies, raw tokens, local paths, object keys,
  plaintext, raw keys, wrapped-key ciphertext, private deployment details, and
  user safety data
