# Live Partial Stream Access Boundary

This document defines the product and security boundary for any future live or
partial media-stream access in Proofline. It is a design document only. It does
not add routes, schema, browser decryption, backend decryption, public product
API exposure, trusted-contact accounts, web-client code, mobile-client code,
protocol code, notifications, key escrow, or playable media export.

## Summary

The current backend should continue to expose only completed encrypted stream
and incident bundle downloads through the public incident viewer. Open and
failed streams may appear in token-scoped read-only summaries as metadata, but
their chunk bytes should not be downloadable through the current bearer-token
viewer routes.

Future live or partial access should be designed as role-scoped, grant-scoped
access for authenticated account-owner or trusted-contact flows. A bearer
public link should not automatically gain live chunk access just because it can
read the incident summary or completed bundles.

The core boundary is:

- completed bundles are stable encrypted ZIP evidence bundles
- live or partial access is a moving snapshot of encrypted stream state
- decrypting live chunks depends on future key custody and client-side
  decryption design
- the backend remains ciphertext-only unless a separate explicit
  break-glass or emergency-access design changes that

## Current Behavior To Preserve

Current behavior is documented in [api.md](api.md),
[security-model.md](security-model.md), and
[browser-decryption.md](browser-decryption.md):

- authenticated main `/v1` routes create incidents, create streams, upload chunks,
  complete or fail streams, close incidents, and create or revoke incident
  viewer tokens
- public `/i/{token}` and legacy `/e/{token}` routes are read-only
- viewer summaries may include stream state, chunk counts, byte counts, and
  selected display metadata; these values do not grant decryption capability
  but may still reveal sensitive context
- completed stream and incident downloads are encrypted ZIP evidence bundles
- open, failed, and legacy unstreamed chunks are omitted from incident bundle
  downloads
- stream download routes return only completed streams
- bundle manifests do not expose server filesystem paths, object-store keys,
  staging paths, raw keys, or plaintext
- public token-protected responses use `Cache-Control: no-store` and strict
  viewer security headers

No future live or partial design should weaken those guarantees.

## Access Decision

Open and failed stream access should be treated differently from completed
bundle access.

| Stream state | Current public token viewer | Future account-owner or trusted-contact access | Rationale |
|---|---|---|---|
| `open` | Metadata summary only. No chunk-byte or partial-manifest download. | Possible after authenticated role and grant design, key-custody design, no-store behavior, and polling/reconnect rules are accepted. | Live chunks are a changing evidence surface and may need session keys, rotation, late-contact enrollment, and stricter abuse controls. |
| `complete` | Completed encrypted stream and incident bundle downloads. | Same ciphertext access can remain available through role-scoped product routes after future access control exists. | Completed bundles are stable server-generated artifacts with controlled ZIP entry names and manifests. |
| `failed` | Metadata summary only. No chunk-byte or partial-manifest download. | Possible recovery or owner-only ciphertext access after explicit role, grant, and evidence-integrity rules are accepted. | Failed streams may be non-contiguous, truncated, or sensitive in ways that need clearer wording than normal completed evidence bundles. |

The current token-scoped public viewer is a public-link capability. It should
remain read-only and incident-scoped. It should not become a live surveillance,
grant-management, upload, admin, escrow, wrapped-key release, decryption, or
plaintext route tree.

If a future product intentionally allows public-link live access, that must be
a separate design with a distinct grant type, shorter-lived credentials where
practical, abuse controls, viewer wording, key-custody behavior, and deployment
warnings. It must not be inferred from the existing incident viewer token.

## Role Boundary

Future live or partial access should follow the role and grant model in
[v1-access-control.md](v1-access-control.md).

| Role or capability | Live or partial access direction |
|---|---|
| Account owner | May be allowed to read live or partial ciphertext for their own incident after authenticated product API design exists. |
| Capture device | May upload chunks and stream state for an authorized incident. It should not use public viewer routes as its queue or recovery API. |
| Trusted contact | May be allowed to read selected live or partial ciphertext only when an explicit escalation or sharing grant permits it. |
| Public link | Defaults to metadata summary and completed encrypted bundles only. No live chunk access by default. |
| Admin/operator | Should not casually access user evidence. Any support access must be narrow, audited, and should avoid raw tokens, raw keys, plaintext, and uploaded bytes by default. |
| Optional escrow or break-glass actor | Out of scope for normal live access. Any raw-key or plaintext path requires a separate explicit design, configuration, audit model, and deployment warning. |

Incident mode labels must not silently grant live access. Emergency incidents,
interaction records, safety checks, and evidence notes need explicit sharing,
escalation, and key-custody policy before live or partial ciphertext is exposed
to anyone other than the uploading client.

## Route Boundary

The current public incident viewer route tree should not gain a generic
partial-stream download route.

Do not add these shapes to the current public viewer without a later explicit
grant design:

```text
GET /i/{token}/streams/{stream_id}/partial
GET /i/{token}/streams/{stream_id}/chunks/{chunk_index}
GET /i/{token}/streams/{stream_id}/live
```

Future implementation should prefer a new authenticated product API route
class for role-scoped live access, separate from private admin routes and
separate from the current token-only viewer. The public incident viewer may
keep polling `GET /i/{token}/data` for read-only metadata, but it should not
become a live chunk transport by default.

Main `/v1` routes currently use local account sessions and remain behind the
deployment's reviewed boundary. Do not make new live or partial routes publicly
reachable under `/v1` until the future public product API controls are
implemented.

## Partial Manifest Shape

Live or partial access should not reuse completed bundle manifests without a
clear state marker. A partial manifest is a snapshot, not a completed evidence
bundle contract.

A future partial manifest should include only reviewed metadata fields. Some
fields are server-controlled; some, such as sanitized `original_filename`
basenames, are client-supplied display metadata and may still reveal sensitive
context.

Allowed fields may include:

- manifest version and `manifest_kind: "partial_stream_snapshot"`
- incident ID and stream ID
- stream status
- media type and optional display label
- generated timestamp
- highest contiguous chunk index observed at snapshot time
- advertised chunk count and total ciphertext bytes for chunks included in the
  snapshot
- per-chunk index, started and ended timestamps, byte size, ciphertext
  SHA-256, and optional sanitized `original_filename` basename
- non-secret encryption hints such as `server_decrypts: false`, media key ID,
  or wrapping metadata references after key-custody design accepts them

It must not include:

- stored paths, staging paths, local filesystem paths, object-store keys, or
  bucket URLs
- raw viewer tokens, incident tokens, session tokens, or secret-bearing URLs
- raw media keys, contact private keys, unwrapped keys, URL fragment secrets,
  plaintext, transcripts, or playable exports
- private deployment details or support-only diagnostics

Clients must treat partial manifests as snapshots. They may become stale as
new chunks arrive, as a stream completes, or as a stream fails. A later
implementation must define reconnect, polling, range, and cache invalidation
behavior before adding routes.

## Key Custody Dependencies

Live media access depends on the key-custody direction in
[key-custody.md](key-custody.md) and the browser limitations in
[browser-decryption.md](browser-decryption.md).

Before live or partial decryption is implemented, the project needs decisions
for:

- whether media uses one stream media key, a rotating stream/session key, or a
  per-incident parent key plus per-stream keys
- when wrapped keys are uploaded relative to live chunk upload
- whether late-added contacts can decrypt already uploaded live chunks, future
  chunks only, or both
- how key rotation interacts with reconnects and partial manifest snapshots
- whether browser decryption is acceptable for live access or a trusted-contact
  native client or offline tool is required first
- what metadata remains visible when a contact can read ciphertext but cannot
  decrypt it

The backend must not parse live media for plaintext, transcode media, merge
media, expose raw keys, or perform backend decryption as part of normal live
access. Any optional break-glass or server-assisted decryption mode is separate
security-sensitive work.

## Caching, Headers, And Logging

Any future live or partial access route must use the same conservative privacy
baseline as current token-protected viewer responses:

- `Cache-Control: no-store`
- `Referrer-Policy: no-referrer`
- `X-Content-Type-Options: nosniff`
- strict content security policy for HTML or script-bearing responses
- route-pattern logging that redacts token-bearing path values
- no logging of request bodies, uploaded bytes, raw tokens, Authorization
  headers, raw keys, plaintext, private deployment details, or future
  token-like values

Deployment guidance must also cover reverse-proxy logs, rate-limit keys,
metrics labels, polling rates, long-lived downloads, and token-bearing paths
before public exposure.

## Implementation Gates

Do not implement live or partial access until these design dependencies are
accepted:

1. Future access-control routes and grants are implemented or explicitly
   scoped for the live route class.
2. Incident-mode and escalation behavior defines which modes may grant live
   access and who receives it.
3. Key-custody behavior defines stream or session keys, wrapped-key timing,
   late-contact enrollment, rotation, and recovery behavior.
4. Browser/client decryption trust model is accepted for live chunks.
5. Partial manifest format, cache behavior, polling/reconnect semantics, and
   failed-stream wording are documented.
6. Tests cover route separation, no-store headers, token redaction, manifest
   path safety, and the absence of raw keys and plaintext.

Until then, keep the current backend behavior: public incident viewer metadata
and completed encrypted bundle downloads only.

## Validation For This Design

For this design-only milestone:

- run `git diff --check`
- manually review this document against [browser-decryption.md](browser-decryption.md),
  [key-custody.md](key-custody.md), [api.md](api.md),
  [security-model.md](security-model.md), and
  [ios-local-recorder-prototype.md](ios-local-recorder-prototype.md)

Go tests are not required unless a later task changes Go code.
