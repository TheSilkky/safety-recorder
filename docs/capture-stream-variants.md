# Capture Stream Variants and Evidence Supersession

This document defines the planning boundary for future capture stream variants
and evidence-preservation supersession in Proofline. It is documentation only.
It does not add routes, schema, migrations, handlers, relay behavior,
browser/native capture code, protocol code, backend decryption, browser
decryption, trusted-contact decryption, key escrow, raw server-held keys,
notifications, emergency-services integration, plaintext GPS relay metadata, or
playable media export.

## Summary

Future browser and native capture should support multiple encrypted stream
variants for the same source capture session. A low-bitrate near-live variant
may upload first so trusted contacts can eventually see recent encrypted
evidence quickly. A full-quality evidence-master variant may be queued and
uploaded later when bandwidth allows. Audio-priority or location/context
variants may preserve critical evidence when video is unavailable or expensive.

The current `MediaStream` model is one concrete upload lane: it has one
`media_type`, stream-local chunk indexes, and `open`, `complete`, or `failed`
state. That remains the implemented model. Future work should evolve
`MediaStream` into a concrete stream variant inside a capture source group, not
treat one stream row as the whole capture session.

The core preservation rule is:

> Preserve the best backend-confirmed evidence for each source time range, and
> never discard the only backend-confirmed evidence for a source time range.

Reduced-quality near-live chunks are evidence-preserving chunks. They are not
disposable previews. A higher-quality chunk may supersede a reduced-quality
chunk for canonical review or future bundle/export selection only after backend
confirmation and source-time coverage validation. Supersession is a selection
decision first; deletion is a separate retention/deletion policy decision.

## Current Behavior to Preserve

Implemented behavior today:

- clients or the simulator create incident-scoped media streams through
  authenticated main `/v1` routes
- each media stream has one `media_type`, optional label, stream-local positive
  chunk indexes, and `open`, `complete`, or `failed` state
- streamed chunk identity is `(incident_id, stream_id, chunk_index)`
- accepted chunks are immutable and are never overwritten
- stream completion requires contiguous chunks `1..expected_chunk_count` and
  readable committed encrypted blobs
- failing a stream preserves already uploaded chunks
- completed stream and incident bundles include completed streams only
- open and failed streams may appear in viewer summaries as metadata, but the
  current public token viewer does not expose their chunk bytes or partial
  manifests
- retention policy treats failed and open streams as possible evidence with the
  parent incident

Future variant work must not weaken those guarantees. In particular, a
near-live or failed stream that contains backend-confirmed chunks must remain
preserved unless the parent incident is deleted or a later reviewed retention
policy proves another confirmed variant covers the same source time range
without gaps.

## Vocabulary

Future implementation should keep these concepts separate:

| Concept | Meaning |
|---|---|
| Capture stream group | A logical capture source/session inside one incident, such as one phone recording session or one browser capture session. It owns source timeline identity and groups concrete stream variants. |
| Concrete stream variant | One upload lane inside a capture stream group. The current `MediaStream` should evolve into this role. |
| Variant role | Why the variant exists, such as `live_preview`, `evidence_master`, `audio_priority`, `location_context`, or `metadata_context`. |
| Quality profile | Encoding or fidelity intent, such as `adaptive_low_latency`, `full_quality`, `audio_priority`, `location_context`, or `metadata_context`. |
| Upload priority | Scheduling intent, such as `near_live`, `preservation`, or `background_queue`. |
| Source segment | A source-capture time range that may have one or more encrypted chunks across variants. |
| Backend-confirmed chunk | A chunk that the core API accepted and stored with durable metadata and committed encrypted bytes. Relay-local staged bytes are not backend-confirmed evidence. |
| Supersession | A future metadata relationship saying one confirmed chunk or segment is preferred over another for canonical review because it covers the same source timeline with better fidelity or a reviewed superset. |
| Evidence resolution | The future algorithm that selects the best confirmed evidence available for each source time range while preserving fallback coverage. |

Do not encode critical variant semantics only in the existing free-form stream
`label` field. Labels can remain display metadata, but variant role, quality,
upload priority, source timeline identity, and supersession state need
validated fields before the backend can enforce or expose them reliably.

## Source Timeline Identity

Every variant in a capture stream group must map back to the same source
timeline. Future schema or protocol work should define stable source identity
fields before implementing supersession.

Likely fields:

- `capture_stream_group_id`: server-controlled group identifier scoped to an
  incident
- `capture_source_id`: client-controlled or server-issued identifier for the
  recording source/device/session, normalized through an accepted protocol
- `source_segment_id`: identifier for one source time range
- `source_sequence`: monotonic sequence within the capture stream group
- `source_started_at` and `source_ended_at`: UTC source-capture timestamps
- optional `source_clock_id` or clock-quality metadata if clients can record
  monotonic time alongside wall-clock time
- optional `source_timeline_hash` or encrypted context digest when source
  timeline metadata must be bound without exposing plaintext context

Source identity must be good enough to match chunks across variants without
using server filesystem paths, object keys, `original_filename`, plaintext GPS,
free-form labels, or route/log metadata. Source timestamps can be
client-supplied and may be wrong, so future implementation should define
validation, tolerated clock skew, overlap handling, and conflict behavior.

## Variant Roles

Initial roles should be explicit and narrow:

| Variant role | Purpose | Fanout direction |
|---|---|---|
| `live_preview` | Reduced-bitrate audio/video optimized for low latency and short upload intervals. It is evidence-preserving, not disposable. | Eligible for near-live relay fanout after relay and access design. |
| `evidence_master` | Higher-quality preservation variant that may upload later from local encrypted staging. | Not normally optimized for near-live fanout; preservation and canonical review priority. |
| `audio_priority` | Audio-first fallback that can upload under poor network or video interruption. | Eligible for near-live fanout when policy permits because it may be the best available evidence. |
| `location_context` | Encrypted location or route context associated with source segments. | Not fanout metadata as plaintext. Delivery requires encrypted context design. |
| `metadata_context` | Encrypted structured context such as device state, permission state, or user-visible markers. | Delivery requires context-specific access and manifest design. |

Rejected for now:

- `preview_only`: rejected because near-live reduced-quality chunks can be the
  only backend-confirmed evidence if capture ends unexpectedly.
- `delete_after_master`: rejected as a role because deletion depends on a
  separate retention/deletion policy and coverage proof, not upload-lane intent.
- `gps_plaintext_live`: rejected because plaintext GPS, speed, heading, or route
  history must not become relay or server operational metadata through this
  model.

## Evidence Resolution

Future evidence resolution should select canonical evidence per source segment,
not per stream row alone.

Suggested ordering for a source time range:

1. Exclude chunks that are not backend-confirmed by the core API.
2. Group confirmed chunks by incident, capture stream group, source segment,
   source sequence, or validated source time range.
3. Reject candidates whose encrypted-byte hash, envelope metadata, stream
   media type, or source-timeline binding does not validate.
4. Prefer variants by explicit role and quality policy, usually
   `evidence_master` over `audio_priority` over `live_preview` when coverage is
   equivalent.
5. Preserve audio-priority or live-preview chunks where no confirmed
   evidence-master coverage exists.
6. Preserve all confirmed chunks needed to avoid a gap, even if they are lower
   quality.
7. Mark missing master coverage explicitly for review and future escalation
   decisions.
8. Preserve or link equivalent encrypted context coverage before treating media
   supersession as complete.

The output of evidence resolution may drive future canonical review views,
trusted-contact presentation, partial manifests, bundle/export manifests, and
operator diagnostics. It must not by itself delete chunks or weaken current
bundle fail-closed behavior.

## Supersession Rules

A higher-quality chunk or segment may supersede a reduced-quality one only if
all of these are true:

- the superseding chunk is backend-confirmed by the core API
- both chunks belong to the same incident
- both chunks belong to the same capture stream group
- the superseding chunk covers the same `source_segment_id`,
  `source_sequence`, source time range, or an approved superset
- the encrypted-byte SHA-256 and envelope or manifest metadata validate
- the supersession does not create a source timeline gap
- equivalent encrypted GPS/location/context coverage is preserved or linked
  when that context exists
- policy explicitly allows that role or quality profile to supersede the older
  variant

Supersession should record enough metadata for future review:

- superseding stream and chunk or segment identifiers
- superseded stream and chunk or segment identifiers
- coverage basis, such as exact segment match or reviewed superset
- validation state and policy version
- non-sensitive reason code, such as `higher_quality_confirmed`,
  `audio_replaced_by_master`, or `segment_superset_confirmed`
- timestamps and actor or service identity for the decision when applicable

Supersession must not:

- overwrite existing chunk rows or blobs
- mutate encrypted bytes
- remove the only confirmed evidence for a source time range
- expose plaintext, raw keys, request bodies, uploaded bytes, stored paths,
  object keys, private deployment details, or raw tokens
- imply legal admissibility or emergency response

## Failed, Incomplete, and Unexpected Capture End

Failed and incomplete streams can contain critical evidence. The current
`FailMediaStream` behavior preserves uploaded chunks and should remain aligned
with this model.

If capture ends unexpectedly because the device is lost, destroyed, forced off,
offline for too long, or interrupted by platform behavior:

- preserve every backend-confirmed chunk, including reduced-quality,
  audio-only, location/context, and metadata-context chunks
- stop waiting indefinitely for an evidence-master variant that may never
  upload
- mark missing master coverage clearly
- allow future escalation or trusted-contact review to use confirmed uploaded
  evidence and last known state
- keep relay-local unconfirmed chunks out of canonical evidence until core
  confirmation succeeds
- never discard reduced-quality coverage merely because a higher-quality upload
  was planned

This document does not implement dead-man-switch behavior. It defines the
preservation semantics future dead-man-switch or break-glass work must honor.
Notification delivery, emergency-services contact, trusted-contact accounts,
wrapped-key release, browser decryption, backend decryption, and server escrow
remain separate explicit designs.

## GPS and Context

GPS/location context may be evidence, but plaintext GPS, speed, heading, or
route history must not be introduced into relay/server operational metadata as
part of this design.

Future encrypted context options:

- include encrypted context inside each media chunk envelope
- attach encrypted context records per source segment
- create `location_context` or `metadata_context` stream variants in the same
  capture stream group
- bind media chunks to encrypted context using a safe digest, source segment
  identity, or authenticated envelope metadata
- store coarse non-sensitive state only when a later design proves it is safe
  and necessary

Supersession should not treat a full-quality media chunk as a complete
replacement for a reduced-quality chunk if the reduced-quality chunk is the only
confirmed item linked to relevant encrypted context. Either the context must be
shared, separately preserved, or explicitly marked missing for review.

Open issue #231 tracks the encrypted GPS/location chunk context model. Future
implementation should coordinate these designs rather than leaking location
semantics through labels, route paths, relay logs, metrics, or public issue
text.

## Regional Relay Relationship

The regional ingress relay remains upload-only, temporary, and subordinate to
the core API. This variant model gives it future policy inputs without making
the relay authoritative for evidence.

Future relay behavior should distinguish:

- relay-accepted or relay-staged chunks: temporary, encrypted, not durable
  evidence
- near-live relay fanout chunks: useful for low-latency viewing but still
  unconfirmed until the core API commits them
- backend-confirmed chunks: durable evidence eligible for evidence resolution
  and supersession

Likely fanout direction:

- `live_preview` and `audio_priority` variants may be eligible for optimistic
  near-live fanout after role/grant/key-custody design
- `evidence_master` variants are preservation-priority and may be uploaded from
  a background queue as bandwidth allows
- relay sessions may constrain which variant roles are fanout eligible
- trusted-contact clients must label relay-fanned chunks as near-live or
  unconfirmed until backend confirmation is observed
- core API confirmation remains required before chunks count as preserved
  evidence or before they can supersede another chunk

Relay logs, limiter keys, metrics, and preflight metadata must not contain raw
tokens, request bodies, uploaded bytes, stored paths, staging paths, object
keys, raw keys, plaintext, or plaintext GPS/context.

## Bundle, Export, and Viewer Direction

Current completed stream and incident bundles should remain unchanged until a
future implementation issue changes the bundle manifest contract. Today,
completed bundles include completed streams only and fail closed if a completed
stream cannot be reconstructed.

Future canonical bundle or export manifests may use evidence resolution to
choose best confirmed coverage for each source segment. They should still:

- identify manifest kind and whether it is completed, partial, canonical, or
  export-oriented
- include source timeline and variant metadata only after the schema is
  reviewed
- mark missing master coverage and fallback coverage clearly
- preserve lower-quality fallback evidence when it is the only confirmed
  coverage
- keep ZIP entry names server-controlled
- avoid stored paths, object keys, staging paths, raw tokens, request bodies,
  uploaded bytes, plaintext, raw keys, and private deployment details

The public token viewer should remain read-only. It should not gain live chunk
transport, supersession management, relay control, grant management, admin
behavior, decryption, wrapped-key release, or deletion behavior through this
design.

## Likely Future Schema and API Changes

Future implementation should be split into narrow issues. Likely changes:

- add `capture_stream_groups` or equivalent source-session records scoped to an
  incident
- add variant fields to media streams, such as `capture_stream_group_id`,
  `variant_role`, `quality_profile`, `upload_priority`, fanout eligibility, and
  source clock metadata
- add source timeline fields to chunk or segment metadata, such as
  `source_segment_id`, `source_sequence`, `source_started_at`, and
  `source_ended_at`
- add supersession records or canonical evidence resolution metadata without
  deleting underlying chunks
- define encrypted context references for GPS/location and metadata context
- define partial/canonical manifest kinds and cache rules before exposing
  role-scoped live or partial access
- extend API docs for create stream, upload chunk, list streams, partial
  manifests, and future bundle/export selection
- add SQLite and PostgreSQL migrations with parity tests only in a later
  implementation issue
- add tests for source-segment matching, no timeline gaps, missing master
  fallback, failed-stream preservation, relay fanout eligibility, and no
  plaintext GPS leakage

Do not implement these in this planning issue.

## Out of Scope

- Runtime schema changes, migrations, handlers, repository behavior, or relay
  implementation.
- Browser capture, iOS capture, Android capture, or shared protocol repository
  implementation.
- Public product API exposure or making `/v1` a public catch-all.
- Backend decryption, browser decryption, trusted-contact decryption, key
  escrow, raw server-held keys, or playable media export.
- Plaintext GPS/location relay fanout or server-visible route history.
- Emergency-services integration, guaranteed emergency response, notification
  delivery, SMS, Messenger, or push notifications.
- Payment, billing, subscription, or hosted-account entitlement behavior.
- Automatic deletion of superseded chunks without a separate retention/deletion
  policy.

## Validation for This Design

For this design-only milestone:

- run `git diff --check`
- manually review against [api.md](api.md),
  [live-partial-stream-access-boundary.md](live-partial-stream-access-boundary.md),
  [regional-stream-ingress-relay.md](regional-stream-ingress-relay.md),
  [retention-backup-deletion.md](retention-backup-deletion.md),
  [mode-aware-retention-policy.md](mode-aware-retention-policy.md),
  [security-model.md](security-model.md), and [threat-model.md](threat-model.md)

Go tests, `go vet`, simulator smoke tests, migrations, and relay smoke tests are
not required unless a later task changes code, schema, routes, or runtime
behavior.
