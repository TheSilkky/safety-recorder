# Regional Stream Ingress Relay Design

This document defines an optional regional stream-ingress relay for complete
encrypted chunk uploads. The current implementation has a separate
`cmd/stream-ingress` service with token-neutral liveness/readiness routes, a
core API route that can issue short-lived signed relay upload capabilities for
authorized open streams, service-authenticated core preflight and commit
endpoints for relay-to-core calls, and a configured complete-chunk upload route
that performs metadata-before-file core preflight, temporary relay-local
ciphertext staging, SHA-256 validation, and core commit forwarding. Optimistic
fanout, metrics, deployment automation, relay Valkey counters, production
service-identity rotation, durable relay storage, decryption, key custody,
web-client code, mobile-client code, protocol code, and public production
readiness remain unimplemented.

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
- backend-issued relay upload capabilities for authorized open media streams,
  disabled until a relay capability secret is configured
- service-authenticated core relay preflight and durable commit endpoints,
  disabled until a relay service auth token and relay capability secret are
  configured
- a configured `cmd/stream-ingress` complete-chunk upload route that stages
  ciphertext temporarily, validates the ciphertext hash, and forwards exact
  encrypted bytes to the core commit endpoint

Those features do not make `/v1` production-ready public infrastructure. The
relay exposes only `/health/live`, `/health/ready`, and
`POST /upload/complete-chunk`; it is not a reason to expose the whole main API
or private admin surfaces.

Parent epic #202 is split into child implementation issues. Issue #289 added
only the service boundary, config surface, route-surface tests, and
implemented-versus-planned documentation. Issue #290 added only backend-issued
relay upload capabilities for authorized open streams. Issue #291 added only
the narrow service-authenticated core relay preflight/commit routes. Issue
#292 adds only relay encrypted upload staging and core forwarding. Later slices
are expected to add, in order, optimistic near-live encrypted fanout, backend
confirmation/rejection propagation, operational guardrails, and final relay
docs/validation alignment.

Future stream variant and supersession behavior is documented in
[capture-stream-variants.md](capture-stream-variants.md). The relay may use
variant roles such as `live_preview`, `audio_priority`, and `evidence_master`
as future policy inputs, but it must still treat the core API as the authority
for backend confirmation and evidence preservation.

## Goals

- Keep a small upload-only service boundary that is easier to deploy close to
  users.
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

- No optimistic fanout, metrics, deployment automation, relay Valkey counters,
  production service-identity rotation, notifications, or viewer confirmation
  propagation in the current relay upload slice.
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

The readiness response reports coarse relay state and whether optional relay
identity and region labels are configured. It does not return the configured
label values, private bind address, service credentials, upstream endpoints,
paths, object keys, tokens, user safety data, or upload state.

Future upload slices may continue to expose only:

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

The current core API can issue a short-lived upload capability for one
authorized open stream:

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
- role, currently `upload`
- incident ID and stream ID binding
- issued-at and expiry timestamps
- maximum chunk byte size
- maximum chunk count
- allowed media types, currently the target stream media type

Relay-side validation must check the signature, expiry, expected role, relay
session ID, incident ID, and stream ID before accepting any future upload.
Capabilities are bearer-like credentials and must not be logged, exposed in
metrics, copied into public issues, used as limiter keys, or stored as durable
evidence metadata. They are only a narrow authorization artifact for later
relay slices; they do not by themselves implement relay listener upload,
staging, fanout, metrics, service identity, or proof of durable evidence
preservation.

## Core Relay Preflight And Commit Routes

The current core API exposes two narrow relay-to-core routes on the main API
mux:

```http
POST /v1/relay/preflight
POST /v1/relay/commit
```

These routes are not mounted on `cmd/stream-ingress`, the public incident
viewer, or the private-admin listener. They require
`X-Proofline-Relay-Service-Token`, backed by `[relay_service].auth_token` or
`[relay_service].auth_token_file`, and that relay-to-core service token is
separate from user bearer sessions, browser cookies, viewer tokens, incident
tokens, and relay upload capabilities. When relay service auth is unset, the
routes fail closed with `503 relay_service_auth_not_configured`. Missing,
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

These core routes preserve the existing direct authenticated chunk upload
route. They do not implement optimistic fanout, relay metrics, notifications,
trusted-contact behavior, public admin behavior, backend decryption, raw key
custody, or key escrow. The separate `cmd/stream-ingress` command implements
the relay upload listener, relay-local temporary staging, and relay forwarding
runtime.

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

## Upload Flow

The current complete-chunk relay flow is:

1. Classify the request by route class before authentication and before
   reading a large body.
2. Apply anonymous pre-body limits using a safe client identity signal, such as
   reviewed proxy client identity or socket peer hash.
3. Parse only cheap metadata needed for preflight, such as incident ID, stream
   ID, chunk index, media type, declared byte size if provided, declared
   `sha256_hex`, and idempotency-key presence.
4. Call the core API preflight over authenticated service-to-service HTTP(S).
5. If preflight denies the upload, return a small safe error without accepting
   the large body.
6. If preflight allows staging, enforce body size, per-session/per-client
   in-flight limits, duplicate in-flight chunk identity limits, and temp disk
   pressure limits.
7. Stream the uploaded ciphertext to local temporary storage while computing
   SHA-256.
8. Compare the computed ciphertext hash with declared `sha256_hex`.
9. On hash mismatch, delete local staging where safe and return a safe failure
   without forwarding bytes to the core API.
10. Forward the complete encrypted chunk and upload metadata to the core API
    commit route.
11. Return success only after the core API confirms committed or equivalent
    success with `201` or `200`.
12. Delete local temporary staging after success or failure where safe.

The relay must not return success for an accepted-but-not-committed upload. If
the final core outcome is ambiguous because of timeout, connection loss, or
core `5xx`, the client should retry the complete encrypted chunk and rely on
the documented idempotency and duplicate reconciliation paths.

Future relay fanout may optimistically forward `live_preview` or
`audio_priority` variants to an authorized live viewer before core confirmation,
but those chunks must be labeled unconfirmed. They become preserved evidence
only after the core API commits the encrypted bytes and metadata.

## Preflight And Abuse Controls

The relay uses layered controls because it cannot fully know whether an upload
credential is valid without asking the core API.

Required layers:

1. Anonymous pre-body limits by route class and safe client identity signal.
2. Core upload preflight using only cheap metadata before accepting large
   bodies.
3. Body and staging limits for max bytes, temp disk pressure, concurrent
   uploads, and per-client in-flight uploads.
4. Backend-denial feedback counters when the core returns `401`, `403`, or
   `429`.
5. No punishment of clients for core `5xx` or infrastructure timeouts.
6. Optional future Valkey/Redis-compatible counters for multi-node relay
   deployments.
7. Local in-memory counters for single-node and development deployments.

The current relay slice implements local in-memory per-session, per-client,
and duplicate chunk in-flight limits. Backend-denial feedback counters,
anonymous denial throttling, and Valkey/Redis-compatible relay counters remain
future slices.

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
| Hash mismatch | Delete local staging where safe and return a safe failure without forwarding bytes. |
| Temp disk pressure | Fail closed before accepting more body bytes. |
| Ingress process crash | Treat local staging as lost; client retry is the recovery model. |

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

## Documentation Updates For Implementation

When implementation work begins, update these source-of-truth docs together as
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
5. Add optimistic near-live encrypted relay fanout. Planned for #293.
6. Add backend confirmation/rejection propagation. Planned for #294.
7. Add operational guardrails for limits, temp pressure, readiness, and safe
   aggregate status. Planned for #295.
8. Run final relay docs and validation alignment. Planned for #296.

Expected implementation tests:

- invalid-token spray is rejected before large body read
- denied-token feedback counters block repeated attempts
- valid upload credential passes through to core
- core `5xx` and timeouts do not poison deny counters
- hash mismatch does not forward to core and cleans staging
- temp disk pressure rejects new uploads
- no raw token, body, path, key, staging path, object key, or secret logging
- relay-fanned near-live chunks remain unconfirmed until core commit
- variant-role fanout does not make the relay authoritative for supersession
- Valkey counter keys use safe non-reversible keys only
- local in-memory limiter works for single-node/dev
- core-confirmed success is required before relay success

## Validation For The Current Relay Upload Slice

The #292 relay upload slice changes Go and Markdown. Validation is:

- stream-ingress tests for config parsing, route surface, valid encrypted
  upload forwarding, core preflight rejection, hash mismatch, core commit
  rejection, temp staging pressure, upstream timeout cleanup, duplicate
  in-flight chunk rejection, and redaction
- `gofmt -w ./cmd ./internal ./migrations`
- `go test ./...`
- `go vet ./...`
- `git diff --check`
- manual review against `README.md`, `AGENTS.md`, `docs/api.md`,
  `docs/architecture.md`, `docs/configuration.md`, `docs/deployment.md`,
  `docs/production-cluster-scope.md`, `docs/security-model.md`,
  `docs/threat-model.md`, and `docs/code-map.md`
