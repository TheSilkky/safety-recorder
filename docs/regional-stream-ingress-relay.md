# Regional Stream Ingress Relay Design

This document defines an optional regional stream-ingress relay for complete
encrypted chunk uploads. The current implementation has a separate
`cmd/stream-ingress` service with token-neutral liveness/readiness routes, a
core API route that can issue short-lived signed relay upload and fanout
capabilities for authorized open streams, service-authenticated core
preflight/commit/fanout authorization endpoints for relay-to-core calls, a
configured complete-chunk upload route that performs metadata-before-file core
preflight, temporary relay-local ciphertext staging, SHA-256 validation, and
core commit forwarding, and an optimistic encrypted SSE fanout route that marks
chunks as near-live/unconfirmed and then emits bounded confirmation, rejection,
or terminal-failure state after the core commit outcome. Readiness now reports
only bounded aggregate categories for upload readiness, core forwarding
configuration, and temp-staging pressure. Replay, metrics, deployment
automation, relay Valkey counters, production service-identity rotation,
durable relay storage, decryption, key custody, web-client code, mobile-client
code, protocol code, and public production readiness remain unimplemented.

## Summary

The target service is a small regional upload helper that can run closer to
users than the core Proofline API, for example on a Melbourne VPS. It should
accept complete encrypted chunk uploads, reject obvious abuse before reading
large bodies, ask the core API for a cheap authorization preflight, stage
ciphertext only in local temporary storage, verify the ciphertext hash, and
forward the complete encrypted chunk to the core API for final commit.

The core API remains the durable source of truth. It continues to own
account/session authorization, incident and stream state, idempotency
decisions, duplicate reconciliation, final blob commits, metadata, deletion,
retention, and bundle reconstruction. The ingress service is disposable: loss
of its local temporary files is recovered by client retry.

Target shape:

```text
client
  -> HTTPS regional stream-ingress relay
  -> HTTPS service-authenticated call to core API
  -> core API commits encrypted blob and metadata
```

## Current Status

Current `develop` already has the prerequisite pieces that make this design
separable from broader cluster work:

- local account/session authentication for main `/v1` routes
- complete encrypted chunk uploads with SHA-256 verification
- metadata-backed `Idempotency-Key` handling for equivalent complete-upload
  retries
- duplicate chunk reconciliation
- optional PostgreSQL metadata and S3-compatible encrypted blob storage
- optional Valkey/Redis-compatible route-class counters and complete-upload
  in-progress leases
- main API and public viewer rate limiting
- main API/viewer and private `/admin` listener separation
- a separate `cmd/stream-ingress` command that can be run and tested without
  changing main API behavior
- backend-issued relay upload and fanout capabilities for authorized open media
  streams, disabled until a relay capability secret is configured
- service-authenticated core relay preflight and durable commit endpoints,
  disabled until a relay service auth token and relay capability secret are
  configured
- a configured `cmd/stream-ingress` complete-chunk upload route that stages
  ciphertext temporarily, validates the ciphertext hash, and forwards exact
  encrypted bytes to the core commit endpoint
- a configured `cmd/stream-ingress` fanout subscription route that authorizes
  subscribers through the core API and sends encrypted chunks only as
  `near_live_unconfirmed` SSE events before sending bounded
  `relay_chunk_state` confirmation, rejection, or terminal-failure events for
  the same ciphertext hash when the core commit outcome is known
- relay readiness categories that report only manual ready state, core
  forwarding configuration, upload readiness, and temp-staging pressure without
  exposing labels, URLs, paths, counts, credentials, or per-user state

Those features do not make `/v1` production-ready public infrastructure. The
relay exposes only `/health/live`, `/health/ready`, and
`POST /upload/complete-chunk`, and `GET /fanout/subscribe`; it is not a reason
to expose the whole main API or private admin surfaces.

Parent epic #202 is split into child implementation issues. Issue #289 added
only the service boundary, config surface, route-surface tests, and
implemented-versus-planned documentation. Issue #290 added only backend-issued
relay upload capabilities for authorized open streams. Issue #291 added only
the narrow service-authenticated core relay preflight/commit routes. Issue
#292 added only relay encrypted upload staging and core forwarding. Issue #293
added only optimistic near-live encrypted fanout. Issue #294 added only
bounded backend confirmation/rejection propagation for fanned-out ciphertext
hashes. Issue #295 added only bounded operational readiness categories and
temp-staging pressure handling. The remaining slice is final relay
docs/validation alignment.

Future stream variant and supersession behavior is documented in
[capture-stream-variants.md](capture-stream-variants.md). The relay may use
variant roles such as `live_preview`, `audio_priority`, and `evidence_master`
as future policy inputs, but it must still treat the core API as the authority
for backend confirmation and evidence preservation.

## Goals

- Keep a small temporary ciphertext relay boundary that is easier to deploy
  close to users.
- Reduce avoidable long-distance upload failures while preserving complete
  encrypted chunk retry semantics.
- Reject excessive anonymous or denied attempts before large request bodies are
  accepted.
- Keep all durable authorization, metadata, idempotency, and blob commit
  decisions in the core API.
- Keep local ingress staging temporary, encrypted-only, and disposable.
- Preserve the backend ciphertext-only posture.
- Support local in-memory counters for single-node/dev relay deployments and
  optional Valkey/Redis-compatible counters for multi-node relay deployments.
- Support future role-aware fanout decisions without making relay-local staging
  durable evidence or canonical supersession truth.

## Non-Goals

- No replay, metrics, deployment automation, relay Valkey counters, production
  service-identity rotation, notifications, or viewer durable-evidence claims
  beyond explicit backend-confirmed relay state.
- No public exposure of the full current `/v1` control plane.
- No admin routes on the ingress service.
- No broad API gateway behavior.
- No durable ingress database.
- No durable evidence blob storage at ingress.
- No async queueing or `202 Accepted` commit semantics.
- No byte-range resumable uploads or partial-upload sessions.
- No upload leases beyond separately scoped future work.
- No backend decryption, browser decryption, raw server-held media keys, key
  escrow, key sharing, or playable media export.
- No trusted-contact account implementation, notification delivery, SMS,
  Messenger, push notifications, web-client, iOS-client, Android-client, or
  shared-protocol implementation.
- No public admin dashboard.
- No cloud-provider deployment automation.

## Service Boundary

The relay boundary is a separate `cmd/stream-ingress` command, not a new route
tree mounted into the existing main API/viewer listener or private-admin
listener.

The current relay command exposes only:

- `GET /health/live`
- `GET /health/ready`
- `POST /upload/complete-chunk`
- `GET /fanout/subscribe`

The readiness response reports only bounded aggregate state:

- `status`: `ready` or `not_ready`
- `uploads`: `ready`, `core_unconfigured`, `temp_staging_pressure`,
  `storage_unavailable`, or `unavailable`
- `core`: `configured` or `unconfigured`; this is configuration state, not a
  live upstream health probe
- `temp_staging`: `ok`, `pressure`, or `unavailable`
- `relay_identity_configured` and `region_configured`: booleans only

It does not return configured label values, private bind addresses, service
credentials, upstream endpoints, data directories, temp paths, object keys,
tokens, uploaded bytes, aggregate counts, per-session counters, per-client
counters, user safety data, or per-upload state.

Any future relay route additions should continue to expose only:

- a narrow complete-chunk upload route family
- token-neutral liveness/readiness routes that reveal only coarse relay status
- token-neutral static or diagnostic-free responses if needed for smoke checks

The ingress service must not expose:

- `/admin` or `/admin/...`
- `/v1/admin/...`
- the whole `/v1` product API
- public incident viewer routes
- bundle download routes
- deletion, retention, backup, restore, migration, support, escrow,
  break-glass, decryption, raw-key, or operator routes
- metrics routes, unless a later issue explicitly designs a safe low-cardinality
  metrics boundary

The relay is not an authorization authority. A trusted ingress service identity
may let it call narrow core preflight and commit endpoints, but it must not
turn a denied user/device/upload credential into an authorized upload.

## Backend-Issued Relay Capabilities

The current core API can issue short-lived upload and fanout capabilities for
one authorized open stream:

```http
POST /v1/incidents/{incident_id}/streams/{stream_id}/relay-session
```

This route is mounted on the authenticated main `/v1` API, not on
`cmd/stream-ingress`. It uses the existing account/session authorization path,
requires write access to the incident, rejects closed incidents, requires the
target stream to be `open`, and returns `503 relay_capability_not_configured`
until `SAFE_RELAY_CAPABILITY_SECRET` or `SAFE_RELAY_CAPABILITY_SECRET_FILE` is
configured.

The signed capability is HMAC-SHA256 over a bounded JSON payload and is not a
raw account session, browser cookie, viewer token, incident token, raw key,
wrapped-key ciphertext, object key, stored path, uploaded byte, plaintext, or
location/safety-data container. Claims are intentionally narrow:

- relay capability version
- random relay session ID
- role, currently `upload` for relay upload and `fanout` for relay fanout
- incident ID and stream ID binding
- issued-at and expiry timestamps
- maximum chunk byte size
- maximum chunk count
- allowed media types, currently the target stream media type

Relay-side and core-side validation must check the signature, expiry, expected
role, relay session ID, incident ID, and stream ID before accepting upload or
fanout behavior. Capabilities are bearer-like credentials and must not be
logged, exposed in metrics, copied into public issues, used as limiter keys, or
stored as durable evidence metadata. They are only narrow authorization
artifacts for relay upload and fanout; they do not by themselves prove durable
evidence preservation or authorize metrics, broad service identity, admin
access, decryption, or key custody.

## Core Relay Preflight And Commit Routes

The current core API exposes three narrow relay-to-core routes on the main API
mux:

```http
POST /v1/relay/preflight
POST /v1/relay/commit
POST /v1/relay/fanout-authorize
```

These routes are not mounted on `cmd/stream-ingress`, the public incident
viewer, or the private-admin listener. They require
`X-Proofline-Relay-Service-Token`, backed by `[relay_service].auth_token` or
`[relay_service].auth_token_file`, and that relay-to-core service token is
separate from user bearer sessions, browser cookies, viewer tokens, incident
tokens, and relay upload or fanout capabilities. When relay service auth is
unset, the routes fail closed with `503 relay_service_auth_not_configured`. Missing,
duplicate, or invalid service tokens return `401 relay_service_auth_required`.

Both routes validate the supplied relay session ID and relay capability against
the expected `upload` role, incident ID, and stream ID. They also enforce
capability expiry, max chunk bytes, max chunk count, and allowed media type
limits before accepting the request as authorized for relay flow.

`POST /v1/relay/preflight` accepts cheap JSON metadata for a complete encrypted
chunk: relay session ID, capability, incident ID, stream ID, chunk index, media
type, start/end timestamps, declared byte size, declared lowercase
`sha256_hex`, and optional original filename. It checks the incident, stream
state, stream media type, duplicate chunk identity, upload byte limit, and
account committed-blob quota. A successful response is only an `accepted`
preflight hint; it is not durable evidence and does not reserve storage.

`POST /v1/relay/commit` accepts the same metadata in multipart form fields plus
the encrypted `file` part. The core API stores the upload through the existing
temporary upload path, verifies declared byte size, verifies the computed
SHA-256 against `sha256_hex`, validates the accepted PQ payload frame, applies
the existing complete-upload coordination lease when configured, commits the
encrypted blob through the configured durable blob backend, and writes chunk
metadata through the configured metadata backend. Commit responses return the
server-generated chunk ID and safe ciphertext metadata only; they do not return
`stored_path`, staging paths, object keys, raw tokens, request bodies, uploaded
bytes, plaintext, raw keys, wrapped-key ciphertext, or private deployment
details.

`POST /v1/relay/fanout-authorize` accepts JSON containing relay session ID,
fanout capability, incident ID, and stream ID. It requires the same relay
service-auth header, validates the `fanout` role and stream binding, and checks
that the incident and stream are still active and open. A successful response
authorizes only a relay-local subscription for optimistic encrypted fanout; it
does not confirm any chunk, commit evidence, create replay state, or grant
viewer-token, trusted-contact, admin, decryption, or key access.

These core routes preserve the existing direct authenticated chunk upload
route. They do not implement relay metrics, notifications, trusted-contact
behavior, public admin behavior, backend decryption, raw key custody, or key
escrow. The separate `cmd/stream-ingress` command implements the relay upload
listener, relay-local temporary staging, relay forwarding runtime, and
optimistic encrypted fanout runtime.

## Core API Boundary

The core API remains responsible for:

- validating account sessions, upload grants, or any future device/upload
  credentials
- confirming incident ownership, incident state, stream state, and stream media
  type
- enforcing complete-upload idempotency and duplicate reconciliation semantics
- committing encrypted blobs to local or S3-compatible durable storage
- inserting or confirming metadata in SQLite or PostgreSQL
- deciding whether an upload is committed, equivalent success, denied,
  rate-limited, conflicted, or retryable
- preserving deletion and retention fail-closed behavior

The relay should treat the core preflight as a cheap hint that lets it decide
whether to accept a large body. It must still treat the final core commit
response as authoritative.

## Relay Complete-Chunk Upload Route

The current relay upload route is:

```http
POST /upload/complete-chunk
```

It accepts `multipart/form-data`. Metadata fields must be sent before the
`file` part so the relay can call core preflight before accepting large
ciphertext bodies where practical. Required metadata fields match the core
preflight/commit shape:

- `relay_session_id`
- `capability`
- `incident_id`
- `stream_id`
- `chunk_index`
- `media_type`
- `started_at`
- `ended_at`
- `byte_size`
- `sha256_hex`
- optional `original_filename`

The relay parses and bounds metadata fields, enforces positive `chunk_index`
and `byte_size`, rejects unsupported media types, rejects declared bytes above
`SAFE_STREAM_INGRESS_MAX_UPLOAD_BYTES`, and sends the metadata to
`POST /v1/relay/preflight` with `X-Proofline-Relay-Service-Token`. Core
preflight remains authoritative for capability signature, expiry, role,
incident/stream binding, incident state, stream state, duplicate identity,
quota, and core upload policy.

After successful preflight, the relay stages only the encrypted `file` part
under its local temp directory while computing SHA-256. It rejects byte-size or
hash mismatches before forwarding. On success, it forwards the exact encrypted
bytes and metadata to `POST /v1/relay/commit`; a relay upload is successful
only after the core commit route returns committed or equivalent success.

Relay responses are intentionally small and safe. Core rejection responses are
mapped to relay errors with a controlled `core_error_code` when available. The
relay must not return or log raw capabilities, service tokens, request bodies,
uploaded bytes, temp paths, stored paths, object keys, plaintext, raw keys,
wrapped-key ciphertext, private deployment details, or user safety data.

## Relay Fanout Subscription Route

The current relay fanout route is:

```http
GET /fanout/subscribe
```

It is mounted only on `cmd/stream-ingress`, not on the core API, private-admin
listener, or public incident viewer. The subscriber must send authorization
context in request headers rather than query parameters so credentials are not
placed in URLs, browser history, proxy URL logs, or referrer paths:

```http
X-Proofline-Relay-Session-ID: <relay_session_id>
X-Proofline-Relay-Fanout-Capability: <signed-fanout-capability>
X-Proofline-Relay-Incident-ID: <incident_id>
X-Proofline-Relay-Stream-ID: <stream_id>
```

The relay calls `POST /v1/relay/fanout-authorize` with
`X-Proofline-Relay-Service-Token` before registering the subscription. Core
authorization remains authoritative for fanout capability signature, expiry,
`fanout` role, relay session binding, incident binding, stream binding,
incident state, and stream state. Missing headers return
`400 invalid_relay_fanout_request`. Core denial is mapped to
`core_fanout_rejected` with a controlled `core_error_code` when available.

Successful subscriptions use `text/event-stream` and receive:

- `relay_ready`: confirms the subscription is active and labels state as
  `near_live_unconfirmed`.
- `relay_chunk`: sends the exact encrypted chunk bytes as base64 in
  `payload_b64`, plus safe ciphertext metadata: `incident_id`, `stream_id`,
  `chunk_index`, `media_type`, `byte_size`, and `sha256_hex`.
- `relay_chunk_state`: sends a bounded state update for the same ciphertext
  metadata after the core commit outcome is known. It does not include
  `payload_b64`.

Fanout chunks are optimistic and unconfirmed when first sent. They are sent
after relay-local metadata preflight, temporary ciphertext staging, byte-size
validation, and SHA-256 validation, but before the core commit call returns. A
`relay_chunk` event does not mean the core API has durably committed the chunk.
Trusted clients must label these chunks as `near_live_unconfirmed` until a
matching `relay_chunk_state` event reports one of:

- `confirmed`: the core commit route returned `201` or `200` and the relay
  returned committed/equivalent success to the uploader for the same
  ciphertext metadata.
- `rejected`: the core returned a non-retryable rejection for the same
  ciphertext metadata. The event includes safe `error_code` and controlled
  `core_error_code` fields when available, `retryable: false`, and
  `terminal: true`; the relay closes the affected fanout stream/session.
- `terminal_failure`: the final outcome is ambiguous or retryable because of
  relay-to-core timeout, network loss, core `429`, core `5xx`, or invalid core
  commit response. The event includes a safe `error_code`, `retryable: true`,
  and `terminal: true`; the relay closes the affected fanout stream/session so
  clients retry the complete encrypted chunk and resubscribe.

The fanout hub is in-memory only. It does not replay old chunks to reconnecting
subscribers, does not store durable fanout metadata, and is lost on relay
restart. Reconnect currently receives only future encrypted chunks and future
state events for the same authorized relay session, incident, and stream.
Confirmation/rejection state is not durable relay state, not replay state, and
not a second evidence source of truth.

The fanout route does not decrypt, inspect, transform, transcode, or rewrite
the encryption envelope. It must not expose raw upload capabilities, fanout
capabilities, service tokens, request bodies, uploaded bytes outside the
authorized encrypted payload transport, temp paths, stored paths, object keys,
plaintext, raw keys, wrapped-key ciphertext, private deployment details, GPS
values, speed, heading, or user safety data in logs, errors, readiness output,
or public artifacts.

## Upload Flow

The current complete-chunk relay flow is:

1. Enforce method and configured multipart body-size limits.
2. Parse bounded metadata fields before the `file` part; metadata sent after
   the file part is rejected.
3. Validate required relay metadata, timestamps, media type, declared byte
   size, and lowercase ciphertext SHA-256.
4. Acquire local in-memory per-session, per-client, and duplicate chunk
   in-flight slots.
5. Call the core API preflight over authenticated service-to-service HTTP(S).
6. If preflight denies the upload, return a small safe error without accepting
   the file body.
7. Stream the uploaded ciphertext to local temporary storage while computing
   SHA-256 and enforcing relay-local temp-staging quota.
8. Compare computed byte size and ciphertext hash with the declared metadata.
9. On mismatch, delete local staging where safe and return a safe failure
   without forwarding bytes to the core API or publishing fanout bytes.
10. After successful local ciphertext validation, publish optimistic encrypted
    fanout if authorized subscribers are present.
11. Forward the exact encrypted chunk and upload metadata to the core API
    commit route.
12. Return success only after the core API confirms committed or equivalent
    success with `201` or `200`.
13. If the chunk was optimistically fanned out, publish a matching
    `relay_chunk_state` event after the core outcome: `confirmed`,
    `rejected`, or `terminal_failure`.
14. Delete local temporary staging after success or failure where safe.

The relay must not return success for an accepted-but-not-committed upload. If
the final core outcome is ambiguous because of timeout, connection loss, or
core `5xx`, the client should retry the complete encrypted chunk and rely on
the documented idempotency and duplicate reconciliation paths.

Current relay fanout may optimistically forward encrypted chunks to an
authorized subscriber before core confirmation, and those chunks are labeled
`near_live_unconfirmed`. They become preserved evidence only after the core API
commits the encrypted bytes and metadata and a matching confirmed state is
observed.

## Preflight And Abuse Controls

The relay uses layered controls because it cannot fully know whether an upload
credential is valid without asking the core API.

Current implemented layers:

1. Multipart body-size limits at the relay listener.
2. Bounded metadata parsing before accepting the file body when the client
   orders fields correctly.
3. Core upload preflight using only cheap metadata before accepting large
   bodies.
4. Body and staging limits for max bytes and temp disk pressure.
5. Local in-memory per-session, per-client, and duplicate chunk in-flight
   limits.
6. Safe retryable behavior for core `5xx` and infrastructure timeouts.

Future hardening layers may add anonymous pre-body route-class limits,
backend-denial feedback counters for core `401`, `403`, or `429`, `Retry-After`
mirroring, and optional Valkey/Redis-compatible relay counters for multi-node
relay deployments. Those counters must not punish clients for core `5xx` or
infrastructure timeouts.

Denial feedback counters should be short-lived. They may help slow repeated
invalid credentials, repeated denied users, or repeated core rate-limit
responses, but they must not become durable evidence metadata.

If a feedback key needs to group attempts by a credential, the relay must use a
non-reversible HMAC or hash fingerprint with an ingress-local secret. It must
not store, log, return, expose in metrics, or use as a Valkey key any raw
credential value.

## Safe Key And Logging Rules

Limiter keys, logs, metrics, traces, errors, readiness output, and staging
paths must never include:

- raw upload grants
- raw session tokens
- raw viewer tokens
- raw incident tokens
- raw idempotency keys
- Authorization headers
- raw `/i/{token}` or `/e/{token}` paths
- request bodies
- uploaded bytes
- full GPS, speed, heading, route history, or location freshness values
- plaintext
- raw keys
- stored paths
- staging paths
- object keys
- object-store credentials
- private deployment details
- user safety data

Safe relay keys should be server-controlled route-class labels combined with
non-reversible hashes or HMAC fingerprints. Avoid high-cardinality labels that
could leak incident IDs, stream IDs, usernames, object keys, paths, private
network topology, or user safety context into logs or metrics.

## Service Identity

Ingress-to-core authentication is separate from user/device/upload
authorization.

The current core preflight and commit routes use an early static service token
for relay-to-core authentication. This is intentionally narrow: it authorizes
only the core relay preflight/commit route set, requires separate relay upload
capability validation for the requested stream context, and does not grant
admin, deletion, bundle download, account management, key delivery, or broad
`/v1` access. Production deployments still need explicit rotation and service
identity review before treating the relay as a hardened public upload edge.

Future options to evaluate for that production identity layer:

| Option | Fit | Notes |
|---|---|---|
| mTLS client certificate | Strong default for fixed relay deployments. | Requires certificate issuance, rotation, revocation, and clear trust anchors. |
| Signed service credential or private-key assertion | Good for multiple relays with explicit service identities. | Requires clock-skew handling, key rotation, and replay controls. |
| Static service token | Acceptable only as a simpler early option. | Requires tight scoping, rotation guidance, redacted logs, and secret handling warnings. |

Whichever stronger service identity is selected later, the core API should
continue to authorize only a narrow ingress preflight and commit route set for
that service identity.

## Failure Behavior

Current relay behavior:

| Condition | Relay behavior |
|---|---|
| Core `201` or `200` | Return committed or equivalent success to the client. |
| Core `401` or `403` | Return a safe denied response; denial counters remain future hardening work. |
| Core `429` | Return a safe rate-limit response; `Retry-After` mirroring and rate-denial counters remain future hardening work. |
| Core `5xx` | Return a retryable safe error without poisoning denial counters. |
| Core timeout or network loss | Return a retryable safe error without poisoning denial counters. |
| Hash mismatch | Delete local staging where safe and return a safe failure without forwarding bytes or publishing fanout bytes. |
| Temp disk pressure | Fail closed before accepting more body bytes and report bounded `temp_staging: pressure` / `uploads: temp_staging_pressure` readiness. |
| Ingress process crash | Treat local staging as lost; client retry is the recovery model. |

For fanned-out chunks, core `201` or `200` produces `confirmed`, non-retryable
core rejection produces `rejected` and closes the affected fanout
stream/session, and core `429`, core `5xx`, timeout, network loss, or invalid
core success response produces `terminal_failure` and closes the affected
fanout stream/session.

Core `5xx`, DNS failure, TLS failure, upstream timeout, and relay-to-core
network loss should not be interpreted as evidence that the client credential
is invalid.

The current relay slice returns safe retryable errors for core timeout,
network, and `5xx` failures. Denial counters and `Retry-After` mirroring remain
future hardening work.

Relay failure must not cause the core system to discard already confirmed
reduced-quality or audio-priority chunks. If a full-quality evidence-master
variant never arrives, backend-confirmed lower-quality chunks remain preserved
evidence under the capture stream variant model.

## Temporary Staging

Ingress staging is for in-flight encrypted bytes only. The current relay stores
only temporary `upload-*` files under `SAFE_STREAM_INGRESS_DATA_DIR` while an
upload is being validated and forwarded.

The relay should:

- stage under a relay-local temporary directory
- compute SHA-256 while streaming to disk
- reject configured max bytes before or while reading the body
- reserve enough temp disk headroom before accepting more uploads
- bound total concurrent uploads and per-client in-flight uploads
- clean request-local temp files after success, denial after body read, hash
  mismatch, relay-to-core failure, or client disconnect where safe
- run conservative age-based cleanup for old relay-local temp files

The current slice cleans request-local temp files after success, hash mismatch,
core rejection, upstream timeout, relay-to-core failure, invalid metadata after
body read, and client upload rejection. Background age-based cleanup remains
future work.

The relay must not:

- make staged files downloadable evidence
- include staged files in bundles
- store durable metadata locally
- use client-provided final paths
- expose staging paths in responses, logs, metrics, or readiness output
- delete committed core blobs or metadata as part of relay cleanup

## Deployment Boundary

A regional relay deployment should be treated as an upload edge. It may be
close to users, but it is not the trusted durable evidence store.

Expected deployment shape:

- client-to-relay HTTPS
- relay-to-core HTTPS
- service identity between relay and core
- reviewed proxy client identity rules if a reverse proxy sits in front of
  ingress
- ingress-local temp disk sized for in-flight encrypted uploads only
- private logs and metrics with token/path redaction
- optional Valkey/Redis-compatible counters for multiple relay nodes
- no durable database at the relay

This design does not make Proofline production-ready public infrastructure.
Any real public deployment still needs deployment-specific TLS, firewall or
reverse-proxy policy, credential handling, abuse controls, logging review,
monitoring, retention, backup, restore, and operational hardening.

## Documentation Updates For Future Relay Changes

Future relay changes should update these source-of-truth docs together as
applicable:

- `README.md`
- `AGENTS.md`
- `docs/api.md`
- `docs/architecture.md`
- `docs/capture-stream-variants.md`
- `docs/configuration.md`
- `docs/deployment.md`
- `docs/production-cluster-scope.md`
- `docs/security-model.md`
- `docs/threat-model.md`
- `docs/code-map.md`

Implementation issues should also update tests and release notes for the exact
behavior changed.

## Follow-Up Implementation Issues

Split implementation into small issues:

1. Add a `cmd/stream-ingress` skeleton with no upload behavior yet. Completed
   by #289.
2. Add backend-issued regional relay session capabilities. Completed by #290.
3. Add narrow core service-authenticated preflight and commit endpoints.
   Completed by #291.
4. Implement relay complete-chunk upload staging, hash verification, and core
   forwarding. Completed by #292.
5. Add optimistic near-live encrypted relay fanout. Completed by #293.
6. Add backend confirmation/rejection propagation. Completed by #294.
7. Add operational guardrails for limits, temp pressure, readiness, and safe
   aggregate status. Completed by #295.
8. Run final relay docs and validation alignment. Completed by #296.

Current implementation validation covers:

- valid upload credential passes through to core
- core `5xx` and timeouts do not poison deny counters
- hash mismatch does not forward to core and cleans staging
- temp disk pressure rejects new uploads
- no raw token, body, path, key, staging path, object key, or secret logging
- relay-fanned near-live chunks remain unconfirmed until core commit
- variant-role fanout does not make the relay authoritative for supersession
- local in-memory limiter works for single-node/dev
- core-confirmed success is required before relay success

Future relay hardening should add tests for anonymous pre-body limiter
behavior, denied-token feedback counters, `Retry-After` handling, and any
Valkey counter keys using safe non-reversible keys.

## Validation For The Current Relay Implementation

The relay implementation changes Go and Markdown. Validation is:

- stream-ingress tests for config parsing, route surface, valid encrypted
  upload forwarding, authorized fanout, unauthorized fanout rejection,
  unconfirmed encrypted chunk delivery, confirmation propagation, rejection
  propagation and fanout termination, terminal failure propagation for core
  `5xx`, network loss, and timeout, hash-mismatch no-fanout behavior, core
  preflight rejection, temp staging pressure readiness, upstream timeout
  cleanup, duplicate in-flight chunk rejection, safe bounded readiness
  categories, and redaction
- `gofmt -w ./cmd ./internal ./migrations`
- `go test ./...`
- `go vet ./...`
- `git diff --check`
- manual review against `README.md`, `AGENTS.md`, `docs/api.md`,
  `docs/architecture.md`, `docs/configuration.md`, `docs/deployment.md`,
  `docs/production-cluster-scope.md`, `docs/security-model.md`,
  `docs/threat-model.md`, and `docs/code-map.md`
