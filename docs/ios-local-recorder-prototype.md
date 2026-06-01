# iOS Local Incident-Capture Prototype Plan

This document scopes a future iOS local incident-capture prototype for Proofline. It is a planning document only. It does not add iOS code, Swift packages, backend routes, database schema, key custody implementation, browser decryption, backend decryption, push notifications, SMS, Messenger integration, public account workflows, OAuth, JWT, or public `/v1` exposure.

The prototype should prove that a native iOS client can capture incident media locally, encrypt chunks before upload, stage encrypted chunks durably, retry uploads, and map its local state to the current backend media stream APIs. The Go simulator remains the backend reference flow until a real client exists.

## Prototype Goal

Build the smallest useful native iOS client that exercises the current Proofline backend contract:

- create or resume one generic incident through the main `/v1` API
- create one media stream per local recording track
- record short chunks locally
- encrypt each chunk before upload
- store only encrypted staged chunks durably
- upload chunks with stream-local positive indexes
- retry failed uploads without mutating accepted chunks
- complete or fail streams explicitly
- keep enough local metadata to recover after app restart

The first prototype should be audio-first. Video capture can be added as a foreground-only track after the audio chunk, staging, and retry loop is proven. Location and device checkins can use the existing checkin API as supporting metadata, but they are not required for the first recorder milestone.

The prototype does not need to set incident-mode metadata yet. It should still
be designed so future UI can distinguish emergency incidents, interaction
records, timed safety checks, and evidence notes, as described in
[incident-modes.md](incident-modes.md).

## Non-Goals

- No production-ready safety app claim.
- No stealth or hidden recording behavior.
- No police-specific or authority-specific mode label.
- No legal-advice or admissibility claim.
- No emergency-services contact or automatic emergency calling.
- No public exposure of the current main `/v1` API.
- No backend decryption or server-held raw media keys.
- No key escrow, browser decryption, trusted-contact implementation, or break-glass implementation.
- No playable media export.
- No cloud service dependency beyond the configured backend.
- No notification, SMS, or Messenger integration.
- No user account, OAuth, JWT, or public admin model.

## Planned Incident-Capture UX

The prototype can start with a plain start/stop recording flow, but future client UX should avoid treating every recording as an emergency.

Planned user-facing capture modes:

| Mode | Prototype treatment | Future behavior |
|---|---|---|
| Emergency incident | Generic incident with urgent local label. | Trusted-contact alert or dead-man escalation after explicit design. |
| Interaction record | Generic incident with neutral local label. | Non-emergency record for important interactions such as with police, security, landlords, employers, service providers, or other authorities. |
| Safety check | Out of scope for first prototype. | Timed check-in flow that escalates to trusted contacts if missed. |
| Evidence note | Out of scope for first prototype. | Quick photo, audio, location, or note bundle without live escalation. |

Use neutral language such as `Interaction record`. Avoid product labels such as `police mode`; users may apply tags later once metadata and sharing rules are designed.

The app should make clear that Proofline records and preserves encrypted evidence. It should not claim to report crimes, contact law enforcement, guarantee legal admissibility, or replace emergency services.

## Current Backend Contract

The prototype should follow the streamed upload path documented in [api.md](api.md):

1. `POST /v1/incidents`
2. `POST /v1/incidents/{incident_id}/streams`
3. `POST /v1/incidents/{incident_id}/chunks` with:
   - `stream_id`
   - positive `chunk_index` starting at `1`
   - `media_type`
   - `started_at`
   - `ended_at`
   - `sha256_hex` of the complete ciphertext envelope bytes
   - `file` containing encrypted bytes
4. `POST /v1/incidents/{incident_id}/streams/{stream_id}/complete` with `expected_chunk_count`
5. `POST /v1/incidents/{incident_id}/streams/{stream_id}/fail` when local recording cannot produce a complete contiguous stream

Streamed chunk identity is `(incident_id, stream_id, chunk_index)`. Chunk indexes are stream-local and must be contiguous for stream completion. Accepted chunks are immutable. The client must never overwrite a staged chunk in place or attempt to replace a chunk the backend has accepted.

The incident viewer and bundle download paths are read-only and should not be used by the recorder except for manual development checks.

## Recorder Behavior

The prototype should expose a plain start/stop capture flow:

- request microphone permission before starting audio capture
- request camera permission only if the foreground video track is enabled
- show whether the recorder is idle, recording, retrying uploads, stopping, or failed
- create an incident when recording begins, or resume a locally known open incident after restart
- create an audio stream before the first audio chunk upload
- optionally create a separate video stream before foreground video chunk upload
- persist the current incident ID, stream IDs, next chunk indexes, and staged upload queue
- keep recording and upload concerns separated so recording can continue while older encrypted chunks are retried
- allow the user to add a local label or note that distinguishes emergency versus non-emergency interaction context without changing backend behavior

For the first milestone, do not require viewer token sharing from the iOS app. A development-only option to create a token may be useful later, but token sharing is not necessary to prove local recording, encryption, staging, and upload semantics.

## Chunking Cadence

Use short, fixed-duration chunks so already-uploaded evidence remains useful if the device is lost, damaged, powered off, or taken.

Recommended prototype defaults:

| Media type | Initial cadence | Notes |
|---|---:|---|
| `audio` | 5 seconds | Primary first prototype stream. |
| `video` | 5 seconds | Foreground-only follow-up track. |
| `location` | 10 to 30 seconds | Prefer checkins first; chunked location can wait. |
| `metadata` | event-driven | Use only for structured client events if needed. |

The cadence should be configurable in development builds, but production-facing defaults should stay boring and predictable. Each chunk should carry UTC `started_at` and `ended_at` timestamps that match the captured media interval as closely as practical.

The client should finish and stage the current chunk before moving to the next index. If a recording interruption prevents a chunk from being finalized, the client should not synthesize a successful placeholder. It should either retry finalization locally or mark the stream failed with a clear local reason.

## Local Encrypted Staging

The client should persist only encrypted chunk envelopes in its durable staging queue. Plain capture output may exist briefly in a temporary file or memory buffer while the chunk is finalized and encrypted, but it should be deleted as soon as encryption and ciphertext hash calculation succeed.

Each staged chunk record should include:

- incident ID
- stream ID
- media type
- chunk index
- UTC start and end timestamps
- ciphertext byte size
- ciphertext SHA-256 hex
- local encrypted file path or file identifier
- upload attempt count
- last upload error category
- upload status: `pending`, `uploading`, `uploaded`, or `abandoned`
- local incident label or prototype mode hint, if used

Staged encrypted chunk files are immutable. If encryption must be retried before upload, create a new staged file and metadata record before discarding the old failed staging attempt. Once a staged chunk has been uploaded successfully, keep enough local metadata to avoid reusing its chunk index after app restart. Local retention of uploaded encrypted staging files can be a prototype setting, but deletion must not imply deletion from the backend.

Use iOS file protection and Keychain settings deliberately during implementation. The prototype should record which protection class was chosen and how it behaves when the device is locked. Do not treat the initial prototype choice as the production key custody answer.

## Encryption And Key Assumptions

The backend currently stores opaque ciphertext only. The iOS prototype must encrypt chunks before upload and compute `sha256_hex` over the encrypted envelope bytes. It should not upload raw media, raw media keys, plaintext, Keychain secrets, or recovery material.

For the first prototype:

- use a stable Apple cryptography API such as CryptoKit for AES-GCM
- use a fresh nonce for every encrypted chunk
- bind the same associated-data fields used by the documented v1 envelope: incident ID, stream ID, media type, and positive chunk index
- prefer a per-stream media key for new iOS work
- store prototype keys in the iOS Keychain with documented access behavior
- never log raw keys, plaintext, ciphertext bytes, request bodies, or viewer tokens

This is still not the final production key custody model. Production work must align with [key-custody.md](key-custody.md), which assumes the user's phone may be unavailable and recommends a future hybrid trusted-contact model. If the prototype stores keys only on the device, document that as a prototype availability limitation rather than a production decision.

## Upload And Retry Behavior

The upload queue should treat recording and network upload as separate loops. Recording should continue staging encrypted chunks while upload retries proceed in the background when iOS allows execution.

Retry expectations:

- retry transient network failures with bounded exponential backoff
- keep retries per chunk ordered by stream index unless a later design proves out-of-order upload is safe for the client UX
- do not mutate staged ciphertext when retrying an upload
- on `400 hash_mismatch`, treat the local staged record as corrupt and stop retrying that chunk until the user or developer can inspect it
- on `400 invalid_chunk_index`, `400 invalid_media_type`, or time-range errors, treat the local metadata as a client bug
- on `409 duplicate_chunk`, reconcile against local state before deciding that
  the upload already succeeded; use the authenticated duplicate chunk reconciliation
  route to compare the expected ciphertext hash and immutable metadata with
  accepted server metadata
- on `409 stream_not_open`, stop uploads for that stream and surface a local stream-state error
- on `413 upload_too_large`, reduce the chunk duration or encoded bitrate for future chunks; do not split or rewrite the already staged immutable chunk
- on `5xx` or network loss, keep the chunk pending for retry

Stream completion should happen only after all chunks from `1` through `expected_chunk_count` are locally marked uploaded. If the app cannot produce a contiguous stream, it should call the stream failure endpoint instead of forcing completion.

## Background And Foreground Constraints

The prototype should measure iOS lifecycle behavior instead of assuming the app can run indefinitely.

Audio:

- design audio as the first prototype track
- configure an audio session for recording before capture starts
- test foreground recording, screen lock, interruption, route change, and app backgrounding behavior
- document whether the app has the required background audio capability enabled for the test build

Video:

- treat video as foreground-only in the prototype plan
- assume camera capture can be interrupted or unavailable when the app is backgrounded or the device is locked
- if video stops but audio continues, complete or fail the video stream separately from the audio stream

Uploads:

- use normal foreground uploads while the app is active
- use background-capable URL loading only if it can be kept simple and testable
- persist the queue before starting uploads so an app kill or reboot can resume safely
- do not require upload completion before staging the next recording chunk

## Failure Modes To Test

The prototype should include manual or automated test notes for these cases:

| Failure mode | Expected behavior |
|---|---|
| Network unavailable at start | Record and stage encrypted chunks locally; upload later. |
| Network drops during upload | Keep staged chunk immutable and retry. |
| Backend unavailable | Keep recording until local limits are reached; retry later. |
| App backgrounds during audio | Continue only if configured and observed to work; otherwise finalize or fail stream. |
| App backgrounds during video | Stop, complete, or fail the video stream; do not assume background camera capture. |
| Device locks | Record observed audio behavior; treat video as interrupted. |
| App is killed | Resume from local incident, stream, queue, and next chunk index metadata. |
| Device reboots | Resume only from persisted encrypted staging and metadata. |
| Chunk encryption fails | Do not upload plaintext; mark local chunk failed. |
| Hash mismatch response | Treat staged ciphertext or metadata as corrupt; do not blindly retry forever. |
| Duplicate chunk response | Reconcile with local uploaded state before proceeding. |
| Stream completion rejected | Keep stream local state inspectable and do not delete staged evidence. |
| Local storage full | Stop recording safely, preserve staged encrypted chunks, and surface failure. |
| Permission revoked | Stop affected track and fail or complete its stream according to contiguous chunks. |
| User starts non-emergency interaction record | Record and upload under a generic incident without automatic escalation. |
| User misses future safety check | Future design should alert trusted contacts only after explicit escalation-policy work. |

## Backend API Gaps

Do not implement these as part of the prototype plan. Track them as future backend issues if they become necessary:

- no app-level `/v1` authentication or authorization suitable for public networks
- no iOS flow for setting incident-mode, capture-profile, escalation-policy, or
  sharing-state metadata
- no account-owner, trusted-contact, or web-client access model
- no resumable or partial-upload API; complete-upload idempotency-key retry
  success is documented in
  [cluster-safe-upload-semantics.md](cluster-safe-upload-semantics.md)
- no iOS client implementation for reconciling a duplicate chunk by expected
  ciphertext hash; the server-side private query workflow is documented in
  [api.md](api.md)
- no explicit resumable-upload protocol for partially sent large chunks; the
  current plan uses the local desktop recorder simulator to measure
  complete-chunk retry behavior before revalidating resumable uploads for
  native iOS work, as documented in
  [resumable-upload-lease-protocol.md](resumable-upload-lease-protocol.md)
- no endpoint for client-side local queue summaries or partial-upload lease
  sessions; resumable upload leases are future coordination work, not account
  authentication or public `/v1` access control
- no API for registering or storing contact-wrapped media keys
- no production key custody, recovery, or trusted-contact decryption API
- no server-side retention or incident deletion API
- no endpoint for live partial-stream bundle viewing before stream completion;
  the future access boundary is documented in
  [live-partial-stream-access-boundary.md](live-partial-stream-access-boundary.md)
- no first-class upload telemetry endpoint for client storage pressure, interruption reasons, or retry state
- no notification or dead-man switch delivery system

For the first prototype, avoid expanding the backend unless a gap blocks the basic simulator-equivalent flow.

The simulator-only contact-wrapped key metadata design is documented in
[contact-wrapped-key-metadata-simulator.md](contact-wrapped-key-metadata-simulator.md).
It can help evaluate model contact public keys, non-secret key IDs, and wrapped
stream media keys before native client or production key-custody work begins.

## Validation Plan

Before iOS implementation starts:

- review this plan against [api.md](api.md), [incident-modes.md](incident-modes.md), [encryption.md](encryption.md), [simulator.md](simulator.md), and [key-custody.md](key-custody.md)
- run the simulator smoke test as the backend reference flow:

```bash
go run ./cmd/simclient --chunks 5 --interval 1s --download-bundle
```

During prototype implementation:

- verify audio chunks upload with positive stream-local indexes
- verify downloaded stream bundles can be decrypted locally with the prototype key material
- verify app restart does not reuse chunk indexes
- verify network loss leaves encrypted chunks staged for retry
- verify failed, interrupted, and completed streams map correctly to backend states
- verify emergency and non-emergency local labels do not change backend security behavior before first-class protocol support exists
