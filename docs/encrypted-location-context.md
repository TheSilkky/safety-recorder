# Encrypted Location Context

Status: design context only. This document does not change runtime behavior,
schemas, routes, handlers, relay behavior, client behavior, key custody, or
decryption behavior.

This design defines how future clients should record full-fidelity GPS,
movement, and freshness context as encrypted evidence while keeping the current
backend, relay, logs, metrics, and basic token viewer from becoming plaintext
location-history surfaces.

## Summary

Location is high-sensitivity user safety data. A latitude/longitude pair,
movement speed, heading, source timestamp, or route history can reveal where a
person lives, works, travels, waits, or seeks help. Those values must not leak
into server-visible metadata just because an audio or video chunk is encrypted.

The intended model has three data classes:

| Class | Purpose | Default visibility |
|---|---|---|
| Class A encrypted evidence | Full GPS, movement, and freshness context bound to chunks, streams, or bounded chunk groups. | Encrypted evidence only. Accessible only to authorized clients that can decrypt the relevant CEK. |
| Class B viewer context | Deliberately limited status or latest shared location context for a no-account token viewer, or richer context for a signed-in authorized contact. | Allowlisted API fields only after a separate viewer-payload issue. |
| Class C server/relay operational metadata | Routing, authorization, limits, storage, and commit state needed to move ciphertext safely. | Server-visible, low-cardinality, non-location operational fields only. |

Current runtime behavior remains narrower than this design. The current server
accepts encrypted chunks, validates public envelope metadata, stores ciphertext
and metadata, and serves read-only token viewer summaries and encrypted ZIP
bundles. It does not implement encrypted location sidecars, chunk-group context
records, browser decryption, trusted-contact incident reads, map-provider
integration, live tracking, or relay fanout.

## Goals

- Preserve full-fidelity location as encrypted evidence.
- Support future review of location, speed, heading, and freshness alongside
  audio, video, metadata, and evidence notes.
- Bind encrypted location context to the media or source segment it describes
  so swaps, replays, and mismatched context fail closed.
- Keep the no-account token viewer minimal and truthful.
- Keep signed-in trusted-contact access separate from token-only access.
- Keep relay and operational systems ciphertext-only, temporary, and
  location-blind by default.
- Define validation expectations before runtime implementation.

## Non-Goals

- No mobile, web, iOS, Android, or shared-protocol implementation.
- No runtime schema, migration, handler, repository, or relay implementation.
- No change to the current `/i/{token}/data` payload.
- No map-provider backend integration or backend map API keys.
- No live tracking guarantee.
- No browser decryption, trusted-contact decryption, backend decryption,
  server escrow, raw server-held keys, or playable export.
- No emergency-services integration or guaranteed emergency response.

## Class A: Encrypted Evidence

Class A is the durable, full-fidelity evidence layer. It should be encrypted
client-side and bound to the media, stream, source segment, or bounded chunk
group it describes.

Candidate encrypted fields include:

| Field | Notes |
|---|---|
| `latitude` and `longitude` | Full precision coordinates when available. |
| `accuracy_meters` | Horizontal accuracy or uncertainty radius. |
| `altitude_meters` | Optional elevation reported by the platform. |
| `altitude_accuracy_meters` | Optional vertical accuracy. |
| `speed_meters_per_second` | Optional movement speed. Sensitive because it can show driving, running, stopping, or distress context. |
| `heading_degrees` | Optional device or course heading. Sensitive because it can reveal movement direction. |
| `source` | Controlled source value such as `gps`, `network`, `manual`, `fused`, `unknown`, or a future reviewed platform source enum. |
| `status` | Controlled value such as `available`, `unavailable`, `permission_denied`, `stale`, `estimated`, or `redacted_by_client`. |
| `client_recorded_at` | Device-side timestamp for when the sample was observed. |
| `server_received_at` | Optional encrypted copy of server receipt time if a client or later manifest needs to compare capture and arrival. The server may still store ordinary receipt metadata separately. |
| `freshness_seconds` | Optional age of the sample at capture or upload time. |
| `source_clock_id` | Optional clock identifier for source-timeline reconciliation. |
| `source_segment_id` | Identifier for the source segment or bounded chunk group covered by the context. |
| `sequence` | Monotonic sequence within a source timeline when available. |
| `client_state` | Controlled, encrypted context such as app foreground/background or permission state, if separately reviewed. |

Free-form notes, addresses, contact names, narrative safety details, precise
places, and platform diagnostic strings should not be added to Class A by
default. If future clients need narrative location context, it should be a
separate evidence-note design with its own privacy review.

## Encoding Options

Future implementation can choose one or more of these shapes:

| Shape | Fit | Requirements |
|---|---|---|
| Inline encrypted context | Location context is included inside the encrypted payload for a media or metadata chunk. | Simple binding to that payload, but can make non-media context harder to retrieve separately. |
| Encrypted sidecar chunk | A `location` or `metadata` media type chunk carries encrypted structured context. | Must bind to incident, stream, source segment, chunk index, and context schema. |
| `location_context` stream variant | A concrete stream variant in a capture stream group carries encrypted route/context samples. | Requires future capture stream group and variant-role schema. |
| Bounded chunk-group context | A context record covers a small range of chunks or a source-time interval. | Requires explicit group identity, coverage rules, and missing-context handling. |

All shapes must keep plaintext GPS, speed, heading, route history, addresses,
and safety context out of server logs, limiter keys, metrics labels, staging
paths, object keys, and ordinary operational metadata.

## Binding And Fail-Closed Behavior

Encrypted location context must be cryptographically bound to the evidence it
describes. The exact serialization belongs in the future protocol issue, but the
binding must include enough authenticated identity to prevent context swaps
across incidents, streams, chunks, source segments, or envelope schemes.

For the post-quantum envelope, future AAD or authenticated encrypted metadata
should include stable values such as:

- scheme and suite identifiers
- payload type, such as `media`, `location_context`, or `metadata_context`
- incident ID
- stream ID or capture stream group ID
- concrete stream variant ID or role, once implemented
- chunk index or bounded chunk-group ID
- source segment ID and optional source sequence
- media type
- CEK or media-key ID
- context schema version
- digest of canonical encrypted context bytes when context is stored outside
  the media payload

Required fail-closed behavior:

- A location context record for one incident must not validate for another
  incident.
- A context record for one stream, chunk, or source segment must not validate
  for another unless a reviewed bounded-group rule explicitly allows it.
- A media chunk must not silently lose its required context binding during
  supersession or bundle/export selection.
- Legacy or compatibility envelopes must not be reinterpreted as supporting
  location bindings unless their authenticated data actually covers the binding.
- Unknown mandatory context schema versions must fail closed.

Server-side validation can check public envelope identity, declared media type,
chunk index, accepted scheme/suite identifiers, and digest shape without
decrypting the context. The backend must not need plaintext location values to
commit, store, bundle, rate limit, reconcile, or relay encrypted chunks.

## Class B: Viewer Context

Class B is a deliberately reduced presentation layer. It must be defined by
allowlist, not by forwarding Class A fields.

The basic no-account token viewer should default to status and safety context
only. If a future issue adds location to that viewer, it should use wording such
as `latest shared location` or `last reported location`; it must not claim live
tracking unless a real streaming contract exists.

The basic token viewer must not receive by default:

- full routes
- chunk-by-chunk GPS samples
- speed histories
- heading histories
- freshness timelines that imply live tracking
- encrypted evidence bytes as plaintext
- wrapped-key ciphertext
- raw tokens or token hashes
- backend diagnostics
- object keys, stored paths, staging paths, or private deployment details

Possible future token-viewer fields, after separate review, are limited values
such as:

- incident status
- started and last-updated timestamps
- latest check-in timestamp
- latest shared or last reported location timestamp
- optional single latest coordinate pair, if intentionally shared
- optional coarse accuracy radius
- freshness category such as `recent`, `stale`, or `unavailable`
- owner-provided viewer message that is explicitly intended for token viewers

Signed-in trusted-contact access is different. A trusted contact has an account
session and must pass relationship, recipient-key, sharing-grant, wrapped-key,
and policy checks. That path may eventually receive encrypted evidence and
wrapped access material for richer map review. A viewer-token holder does not
become a trusted contact by opening a link, and token access must not grant
trusted-contact incident reads, wrapped-key delivery, notification actions, or
decryption capability.

## Class C: Server And Relay Operational Metadata

Class C is the metadata needed for safe transport and storage. It should be
boring, low-cardinality, and location-blind.

Acceptable examples include:

- route class
- authenticated account or service authorization result
- incident, stream, chunk, or upload identifiers when already required by the
  API and not used as log labels
- media type
- chunk index
- declared byte size
- ciphertext SHA-256 or SHA-384 digest
- envelope scheme and suite identifiers
- controlled stream state
- safe error categories
- safe counts

Class C must not include:

- latitude or longitude
- altitude
- speed
- heading
- route history
- precise place names or addresses
- raw viewer, incident, session, verification, CSRF, upload, or idempotency
  tokens
- Authorization headers or cookies
- request bodies or uploaded bytes
- plaintext or raw keys
- wrapped-key ciphertext
- object keys, stored paths, staging paths, or private filesystem paths
- private endpoint details
- user safety narratives

The regional relay must remain temporary, ciphertext-only, location-blind, and
subordinate to the core API. Relay logs, metrics, traces, limiter keys,
readiness output, preflight metadata, fanout metadata, and staging paths must
not include raw tokens, raw idempotency keys, full GPS, speed, heading, object
keys, stored paths, staging paths, uploaded bytes outside authorized encrypted
payload transport, plaintext, raw keys, or user safety data.

## API And Storage Direction

Future API work should split implementation into narrow issues:

- define the encrypted context schema and canonical serialization
- decide whether location context is inline, sidecar, a stream variant, or a
  bounded chunk-group record
- add metadata only for encrypted-context references, not plaintext coordinates
- update upload validation to bind envelope identity and context references
- update bundle manifests only after deciding what context references are safe
  to expose
- update the token-viewer payload separately from the encrypted evidence model
- add signed-in trusted-contact incident reads separately from no-account token
  viewing

Do not store plaintext full-fidelity location in general incident, stream,
chunk, relay, or object-storage metadata as an accidental convenience field.
Current check-in location fields are existing server-visible metadata and should
not be used as the full-fidelity evidence route model.

## Validation Expectations For Future Implementation

Future runtime changes should add tests for:

- Class A fields staying encrypted and absent from server-visible metadata
- Class B token-viewer payload field allowlists
- invalid, expired, and revoked token behavior remaining indistinguishable
- no full routes, chunk-by-chunk samples, speed histories, or heading histories
  in no-account token responses by default
- signed-in trusted-contact access requiring relationship, recipient-key,
  grant, wrapped-key, and policy filters
- envelope AAD or authenticated metadata rejecting swapped incident, stream,
  chunk, source-segment, schema, and digest values
- bundle and export manifests preserving required encrypted context references
  without exposing plaintext location
- relay logs, metrics, traces, limiter keys, readiness output, and staging
  paths omitting raw tokens, raw idempotency keys, GPS, speed, heading, stored
  paths, object keys, uploaded bytes, plaintext, and raw keys
- request logs and handler errors omitting location values and user safety
  narratives

Documentation review should also check viewer copy. Use `latest shared
location` or `last reported location` for token-viewer context unless a real
live-streaming contract has been designed, implemented, and tested. Do not imply
that Proofline contacts emergency services or guarantees emergency response.

## Related Documents

- [Contacts, key model, and viewer replacement](contacts-and-viewer-replacement.md)
- [Capture stream variants and evidence supersession](capture-stream-variants.md)
- [Pure post-quantum encryption envelope](post-quantum-envelope.md)
- [Encryption](encryption.md)
- [Regional stream ingress relay](regional-stream-ingress-relay.md)
- [Live partial stream access boundary](live-partial-stream-access-boundary.md)
- [/v1 access control](v1-access-control.md)
- [API](api.md)
- [Security model](security-model.md)
- [Threat model](threat-model.md)
- [Logging requirements](logging-requirements.md)
