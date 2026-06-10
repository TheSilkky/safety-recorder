# v1 Preview Direction

This document is the direction-setting source of truth for Proofline v1 preview
planning. It is for maintainers and Codex before implementation work begins. It
does not change runtime behavior, add routes, create migrations, deploy
services, implement client code, remove the built-in viewer, or approve public
production deployment.

Use this document to keep current prototype limits separate from the intended
v1 preview direction. Current code, [API](api.md), [security model](security-model.md),
and [threat model](threat-model.md) remain the source of truth for implemented
behavior. This document records how future work should interpret those
implementation docs when deciding what belongs in v1 preview and what remains a
separate issue.

For release decisions, use the
[v1 preview readiness checklist](v1-preview-readiness-checklist.md) before any
`v1 preview`, `v1.0.0`, or real-user evidence-upload readiness claim.

## Purpose

Proofline v1 preview should mean the intended encrypted capture and review flow
works end to end. It does not mean only that the server builds, users can
register, or a metadata UI exists.

This document should be read before broad feature, backlog, issue, design, or
review work that could affect product direction, repository boundaries,
security posture, viewer behavior, key custody, browser crypto, trusted
contacts, deployment exposure, logging, retention, or capture semantics.

Codex should use it this way:

- treat present-tense statements in implementation docs as current behavior
- treat future-tense statements here as direction, not permission to implement
  unrelated behavior
- preserve repository boundaries unless a task explicitly changes them
- turn newly discovered follow-up work into backlog notes or issue drafts
  instead of implementing it inside an unrelated task
- avoid public issue, PR, docs, test, or summary text containing raw tokens,
  secrets, private deployment details, exploit details, wrapped-key ciphertext,
  plaintext, raw keys, request bodies, uploaded bytes, object keys, stored
  paths, or user safety data

## Repository Roles

The current organization is `open-proofline`.

| Repository | v1 direction |
|---|---|
| `open-proofline/server` | Go backend, authenticated main API, private admin surface, storage, migrations, backend security boundaries, encrypted bundle generation, deployment docs, and server release workflow. |
| `open-proofline/web-client` | User-facing account portal, no-account viewer replacement, authenticated incident review, browser capture when separately designed, browser-held private keys, browser-side encryption/decryption, trusted-contact UX, and device key-sharing UX. |
| `open-proofline/ios-client` | Primary native iOS encrypted capture, local encrypted staging, upload retry, local account/device flows, and platform-specific recording behavior. |
| `open-proofline/android-client` | Primary native Android encrypted capture, local encrypted staging, upload retry, local account/device flows, and platform-specific recording behavior. |
| `open-proofline/protocol` | Shared API, envelope, bundle, compatibility, conformance, and migration contracts after the protocol split is ready. |

This repository is the server/backend component only. It may keep planning docs
for client and protocol work, but it must not implement web, iOS, Android, or
shared protocol code unless the maintainer explicitly changes the repository
strategy.

Current sibling `open-proofline/web-client` docs describe an experimental React
prototype for account, incident, contact-key, sharing-grant, and wrapped-key
metadata review. It does not currently implement recording, browser decryption,
trusted-contact decryption, key escrow, playable export, emergency dispatch,
production billing, or production safety workflows. The server docs remain the
backend source of truth.

## Current Server Posture

The current server is experimental, local-first, and ciphertext-only by default.
It implements local username/password accounts, opaque server-side sessions,
optional browser cookie sessions for future web-client calls, disabled-by-default
public registration with SMTP-backed email verification when open mode is
explicitly configured, owner-scoped incident metadata, contact public-key
metadata, owner-scoped sharing grants, owner-scoped wrapped-key metadata, private
deletion workflows, encrypted chunk upload, and encrypted ZIP evidence bundles.

The current server has two listener groups:

| Listener group | Current routes | Direction |
|---|---|---|
| Main API and viewer | Authenticated non-admin `/v1/...` routes, current prototype/local `/i/{token}` viewer routes, `/e/{token}` aliases only when explicit local/test compatibility needs them, and `/static/...` viewer assets. | Reviewed main API deployment boundary. Viewer paths may be routed publicly only when route paths, logs, headers, TLS, and abuse controls are reviewed. Future canonical no-account viewer links belong to the web-client origin. |
| Private admin | Authenticated admin-only `/v1/admin/...` JSON routes, `/admin`, `/admin/...`, and `/admin/static/...`. | Localhost, LAN, WireGuard, firewall, VPN, or strict private reverse proxy. Still authenticated and authorized. |

Separate bind addresses reduce accidental exposure. They are not a complete
security model. Public edges must not route `/admin`, `/admin/...`,
`/v1/admin/...`, operator diagnostics, escrow, break-glass, decryption, raw-key,
or private support routes.

The Go API is responsible for application security boundaries such as
authentication, authorization, route separation, admin/private listener
separation, request-size limits, route-class limits, safe logging, safe errors,
storage invariants, token hashing, upload validation, and ciphertext-only
evidence handling.

The operator is responsible for secure deployment: TLS termination, reverse
proxy or Cloudflare configuration, WAF and edge rate limits, tunnel and firewall
rules, private-admin reachability, monitoring, backups, host and network
hardening, storage encryption, and operational response.

The release process is responsible for review and validation. The API can be
secure when properly deployed, and a deployment can still be unsafe if the
operator routes the wrong listener, exposes private admin surfaces, disables
required controls, or violates documented boundaries.

## Current Web-Client Posture

The web client is a future v1-critical surface, but it is not yet the v1
product. The sibling prototype currently covers account and metadata review
flows, including mock mode, bearer and browser-cookie client modes, registration
and email verification UI, owner incident metadata views, contact public-key
metadata views, sharing-grant metadata views, wrapped-key metadata views, and
incident deletion request UI.

It does not currently deploy publicly as a production Proofline service. Public
static deployment needs separate server, deployment, abuse-control, CSRF/CORS,
credential-storage, security-header, supply-chain, and served-asset integrity
review before it can be treated as a v1 preview deployment path. The server-side
route, browser credential, CORS, CSRF, cache, edge, and logging boundary is
documented in
[public-web-client-deployment-boundary.md](public-web-client-deployment-boundary.md).

When public web-client deployment exists, deployment integrity should include
built asset manifests, root distribution hashes, post-deploy verification that
served bytes match built bytes, non-sensitive build provenance, Subresource
Integrity where practical, CSP, and documentation that this detects deployment
drift but does not fully prevent targeted same-origin edge tampering.

## Product Direction

Proofline is not only an emergency recorder. It should support private
encrypted capture for:

- emergency incidents
- interaction records
- timed safety checks
- evidence notes

Capture, escalation, sharing, key release, notification, retention, export,
legal use, and public links are separate systems. They must not be silently
inferred from incident labels.

V1 preview should show the intended encrypted capture and review loop:

1. A user can create or start a capture through a reviewed client.
2. The client encrypts evidence before upload using the v1 preview default
   post-quantum envelope.
3. The server accepts complete encrypted chunks, stores ciphertext and metadata,
   and preserves immutable evidence.
4. The account owner can review uploaded incident state.
5. Trusted-contact or viewer access follows explicit grants or viewer-link
   behavior, not hidden mode-label side effects.
6. Authorized review can reach the encrypted evidence and the required wrapped
   key material through a reviewed client-side or contact-side decrypt path.
7. The system stays clear that users or trusted contacts, not Proofline, remain
   responsible for contacting emergency services unless a later
   jurisdiction-specific integration is explicitly designed.

## Canonical Terminology

Use the terminology established in the server docs unless a future protocol
migration explicitly replaces it.

| Term | Meaning |
|---|---|
| `incident_mode` | User-visible reason for capture, such as emergency incident, interaction record, safety check, or evidence note. |
| `capture_profile` | What the client intends to capture, such as audio/video/location, audio/location, location check-ins, notes, attachments, or a custom combination. |
| `escalation_policy` | If and when trusted contacts or other future workflows are triggered. |
| `sharing_state` | What access has actually been granted, exported, revoked, or expired. |
| Capture stream group | A logical capture source or session inside one incident. It owns source timeline identity and groups concrete stream variants. |
| Concrete stream variant | One upload lane inside a capture stream group. The current `MediaStream` should evolve toward this role. |
| Variant role | Why the variant exists, such as `live_preview`, `evidence_master`, `audio_priority`, `location_context`, or `metadata_context`. |
| Source timeline | The source-capture time range shared across variants. |
| Supersession | Evidence-selection metadata, not deletion. A better confirmed variant can become canonical only after source-time coverage validation. |
| CEK | Content-encryption key for an incident, stream, or bounded chunk group. Current `media_key_id` fields are compatibility identifiers for this key material until a protocol naming migration lands. |
| Recipient key | Durable public/private key material owned by an account, device, trusted contact, or explicit escrow target. Private keys stay client/contact side by default. |
| Wrapped-key record | Encrypted CEK/media-key material plus public wrapping metadata. It is access-enabling metadata and must not be logged. |

Do not use `capture_modes` for the product-level mode if the accepted server
field is `incident_mode`. Do not use product labels such as `police mode`;
prefer neutral user-facing language like `Interaction record`.

## Current Behavior Versus v1 Direction

| Area | Implemented now | v1 preview direction |
|---|---|---|
| Server storage | Ciphertext chunks and metadata; no raw media keys or backend decryption. | Preserve ciphertext-only default. Store wrapped/encrypted keys only through explicit custody designs. |
| Incident modes | Optional metadata labels on incident create/read. | Product modes guide capture, escalation, sharing, review, and retention only after explicit policy implementation. |
| Viewer | Built-in token-scoped server-rendered viewer plus encrypted bundle downloads. | Replace the server-rendered viewer before v1 preview with a web-client viewer while preserving backend token/data primitives needed by the web client. |
| Web client | Experimental account and metadata review prototype. | User-facing account portal, no-account viewer replacement, full incident review, browser-held private keys, browser crypto, trusted-contact UX, and device key-sharing UX. |
| Post-quantum envelope | Server upload validation, wrapped-key metadata validation, bundle profile hints, docs, and simulator uploads now default to the accepted PQ profile. Web-client evidence upload and review still belong to companion-repository work. | Keep the accepted PQ profile as the preview default across server and web-client evidence upload, wrapped-key delivery, and review flows. |
| Browser recording | Not implemented. | In scope for v1 preview as a reviewed secondary/fallback capture path, not a replacement for native mobile reliability. |
| Browser decryption | Not implemented. | In scope for authorized account-owner or trusted-contact review after browser trust model, key custody, and deployment-integrity decisions are accepted. |
| Trusted contacts | Owner-scoped metadata for contact public keys, grants, and wrapped keys. | Account-based trusted-contact invite/accept, grant-scoped review, wrapped-key delivery, and client-side decrypt UX. |
| Device sharing | Not implemented. | Each device should have its own key material. Existing trusted devices or a recovery flow should approve new devices, then rewrap relevant CEKs to the new device recipient key. |
| Regional relay | Health/readiness skeleton only. | Optional upload-only, temporary, ciphertext-only relay subordinate to the core API. |
| Public registration and required setup | Disabled by default; open mode requires SMTP verification; paid mode fails closed. New admin-created and open-registration accounts carry required setup state that blocks main product routes until email challenge, TOTP, or configured WebAuthn setup is completed. Active TOTP and WebAuthn factors require per-session verification. | Explicit preview deployments may enable open registration with email verification, rate limits, deployment controls, account-scoped committed blob quota, configured WebAuthn/passkey factors, and a reviewed recovery policy. |
| Billing | Future Stripe-hosted service boundary exists as planning context. | Payment providers, subscriptions, account plans, and hosted entitlements are out of default v1 preview scope unless separately scoped. |

## Server v1 Direction

The server should own backend primitives and server behavior:

- main API authentication and authorization
- owner incident metadata and upload routes
- contact public-key metadata
- sharing-grant metadata
- wrapped-key metadata storage and delivery
- default post-quantum envelope metadata, compatibility checks, and bundle
  documentation for accepted encrypted evidence
- encrypted blob storage and metadata backends
- deletion, retention, and tombstone mechanics
- route-class rate limits and safe request handling
- safe logging and safe error categories
- encrypted bundle generation with server-controlled ZIP entry names
- private admin boundaries
- deployment and operational docs

The server should keep backend token-scoped viewer/data primitives that the web
client needs. The built-in server-rendered incident viewer should not be
carried into v1 preview as the primary fallback or compatibility surface. This
is an experimental pre-v1 project, so removing the built-in viewer before v1 is
acceptable when release notes and migration guidance document the change. That
removal is not part of this document and requires a separate implementation
task. The future no-account viewer link should point at the web-client origin
using the fragment-token routing decision in
[web-client-viewer-routing.md](web-client-viewer-routing.md).

The server must not incidentally add browser crypto, browser recording, web UI,
native client code, protocol repository code, backend decryption, raw
server-held keys, default key escrow, playable export, emergency-services
integration, push/SMS/Messenger notifications, OAuth, JWT, public admin
dashboards, payment processing, or cloud deployment automation.

## Web-Client v1 Direction

The web client should become the primary user-facing review and account surface.
Its v1 direction includes:

- account access and account self-service
- no-account viewer replacement for token-scoped read-only access
- owner incident review in product language
- viewer-link creation and revocation UX
- trusted-contact invite/accept flows
- trusted-contact incident review
- automatic client-managed recipient key creation and publication
- default post-quantum envelope use for browser capture, wrapped-key delivery,
  and encrypted evidence review
- browser-held private keys where the accepted design calls for them
- browser-side encryption for browser capture
- authorized browser-side decryption for review
- full incident review, including status, upload state, access state, latest
  update, safe context, encrypted evidence availability, and next actions

Manual public-key paste flows, algorithm selection, wrapped-key ciphertext
inspection, raw grant objects, raw IDs, and route names may exist in advanced
or developer contexts. They should not be the primary UX for normal users or
trusted contacts.

## Browser Recording Direction

Browser recording is in v1 preview direction, but it must be scoped and
reviewed separately. It is useful for desktop, browser, meeting, call,
interaction-record, evidence-note, fallback, and local testing cases. It must
not be presented as guaranteed background recording or as a replacement for
native mobile capture.

Before implementation, browser recording needs documented decisions for:

- explicit user consent and browser permission prompts
- microphone, camera, screen, window, tab, and system-audio support differences
- tab close, browser crash, sleep, backgrounding, permission loss, and upload
  reliability limits
- local encrypted staging before upload
- chunking and complete encrypted upload retry semantics
- user-visible recording state
- safe failure and restart behavior
- browser token-storage and XSS implications

## Browser Cryptography and Trust Direction

Browser-side encryption and authorized browser-side decryption are in v1
preview direction. They are not implemented now and must not be added as
incidental JavaScript inside server work.

Browser decryption served by the same origin as the backend is useful against
passive storage, database, and blob compromise. It is not full end-to-end
protection against an active malicious server that can serve modified
JavaScript. Mitigations such as strict CSP, static assets, no inline script,
Subresource Integrity where practical, signed or pinned viewer bundles,
independent hosted viewer code, and offline verification tools can reduce risk,
but they do not erase same-origin malicious-code risk. A dynamic same-origin
decrypting viewer is not acceptable as the production trusted-contact decrypt
path by itself; production browser decryption requires the trust gate in
[browser-decryption.md](browser-decryption.md).

Any browser decrypt path must use stable Web Crypto APIs or maintained
libraries where appropriate. It must not implement custom cryptographic
primitives. It must avoid plaintext logging, debug globals, browser storage
surprises, service-worker caches, query parameters, analytics, and DOM leaks.

## Post-Quantum Encryption Requirement

The post-quantum evidence envelope is required for v1 preview. It must be fully
implemented, documented, tested, and default for the server and web client
before real users are invited to upload sensitive evidence.

The reason is not theoretical polish. Proofline evidence may include extremely
sensitive material involving real victims, real legal proceedings, and graphic
evidence of abuse or assault. The storage system must be designed from the
earliest possible preview date to reduce harvest-now/decrypt-later risk and to
protect users from future disclosure of evidence that was uploaded while the
product was young.

For v1 preview, post-quantum default means:

- the web client uses the accepted post-quantum envelope for browser capture,
  encrypted upload, wrapped-key delivery, and encrypted evidence review
- the server accepts, stores, bundles, documents, and validates metadata for
  the post-quantum envelope as the default evidence format
- compatibility or simulator-only legacy envelopes are clearly marked as
  development, migration, or test paths, not the preview user default
- bundle manifests, API docs, encryption docs, tests, and operational guidance
  describe the default post-quantum envelope
- downgrade behavior must fail closed unless a migration path is explicitly
  reviewed and documented
- raw CEKs, ML-KEM shared secrets, decapsulation keys, plaintext, and
  wrapped-key ciphertext must not be logged

This is still a repository-boundary rule. The server repository must not
implement the web client, but server v1 preview work should treat post-quantum
envelope support as required backend readiness for the web-client preview.

## Key Custody Direction

The current backend remains ciphertext-only. The future product still cannot
depend on keys existing only on the user's phone. The user's phone may be lost,
damaged, powered off, taken, destroyed, or unavailable during a dead-man-switch
event.

The default v1 direction is hybrid key custody:

- clients encrypt media before upload
- the server stores ciphertext chunks and metadata
- accounts, devices, and trusted contacts own durable recipient keys
- incidents, streams, or bounded chunk groups own CEKs
- the server may store wrapped or encrypted CEKs as explicitly designed
  metadata
- trusted contacts can eventually decrypt authorized evidence without requiring
  the phone to survive
- client-side, browser-side, or trusted-contact-side decryption is preferred
  where practical
- the v1 preview default envelope is post-quantum so long-lived evidence is not
  uploaded under a merely classical default wrapping scheme
- raw server-side key access or server-side decryption is allowed only as a
  separately configured break-glass or dead-man-switch mode with explicit
  policy, audit, tests, deployment warnings, and threat-model updates

Server storage of wrapped/encrypted keys is acceptable only when explicitly
designed. Default server storage of raw CEKs, raw media keys, recipient private
keys, plaintext, unwrapped shared secrets, browser fragment secrets, or
server-decryptable material remains out of scope.

## Device Key-Sharing Direction

Device key sharing should not copy long-term private keys between devices.
Each device should have its own key material and recipient public-key record.

New devices should be approved by an existing trusted device or by a separately
designed recovery flow. After approval, an authorized client that can access the
relevant CEKs should rewrap those CEKs to the new device's recipient public key.
The server may store the public key, device roster metadata, signatures, grants,
and wrapped-key ciphertext. It must not invent a rewrap path by decrypting
existing wrapped-key ciphertext in the default mode.

Lost-device and key replacement flows should be user-friendly and should stop
future wrapping to lost or revoked keys. They cannot claw back data or keys
already downloaded by an authorized actor.

## Trusted-Contact Direction

Trusted-contact behavior, trusted-contact access, wrapped-key delivery, and full
incident review are v1 direction.

The primary trusted-contact UX should be:

```text
Invite trusted contact
Accept invite
Share access
Stop sharing
Lost device
Replace key
```

Normal users and trusted contacts should not manually create keys, paste public
keys, choose algorithms, inspect wrapped-key ciphertext, or manage fingerprints
as the primary flow. Clients should automatically create required recipient key
material, store private keys locally, publish public key material to the
backend, and use active recipient key versions for wrapping when grants allow
access.

Viewer-token holders are not trusted contacts. Viewer links are bearer public
links for limited read-only access and must remain separate from account-based
trusted-contact identity, grants, wrapped-key delivery, and decryption.

## Full Incident Review Direction

Full incident review belongs in the web client and future native clients, not
the built-in server viewer. A v1 review surface should lead with:

- incident status and whether capture is active, completed, failed, deleted, or
  unavailable
- latest update and freshness wording
- what evidence has uploaded and what remains pending
- who can access the incident
- whether viewer links or trusted-contact grants exist
- what encrypted evidence is available
- whether a user action is needed
- clear warnings that Proofline does not guarantee emergency response

Technical IDs, stream and chunk tables, grant fields, wrapped-key rows,
algorithm IDs, and route names can remain available in advanced details or
security/developer contexts. They should not be the default review experience.

## Capture Stream Groups and Variants

Future capture should support multiple encrypted stream variants for the same
source session:

- `live_preview`
- `evidence_master`
- `audio_priority`
- `location_context`
- `metadata_context`

Reduced-quality near-live chunks are preserved evidence, not disposable
previews. Full-quality evidence-master chunks may supersede lower-quality chunks
for canonical review only after backend confirmation and source-time coverage
validation. Supersession is a selection relationship. It is not deletion.

The preservation rule is:

```text
Preserve the best backend-confirmed evidence for each source time range, and
never discard the only backend-confirmed evidence for a source time range.
```

The current `MediaStream` remains one concrete upload lane. Do not encode
future variant semantics only in free-form labels. Future implementation needs
validated fields for capture stream group identity, variant role, quality
profile, upload priority, source timeline identity, encrypted context bindings,
and supersession state.

## GPS, Location, and Context Direction

GPS and context data can be evidence. Full GPS, speed, heading, route history,
detailed safety state, device state, and private notes must not become server,
relay, edge, log, metric, public issue, or public viewer metadata by accident.
The detailed encrypted evidence, viewer-context, relay, binding, and validation
model is documented in
[encrypted-location-context.md](encrypted-location-context.md).

Preferred layers:

| Layer | Direction |
|---|---|
| Full GPS evidence | Encrypted per chunk, per source segment, or in `location_context` variants. |
| Viewer context | Minimal no-account context only when deliberately reviewed; richer context should be signed-in trusted-contact scoped or encrypted. |
| Server, relay, and edge metadata | No full GPS, speed, heading, route history, plaintext notes, raw keys, stored paths, object keys, or user safety data. |

Map links, map-provider requests, analytics, logs, screenshots, and issue text
must be reviewed for token and location leakage before any public viewer map
experience ships.

## Regional Relay Direction

The regional stream-ingress relay currently exists only as a separate
health/readiness skeleton. Future upload slices should keep it upload-only,
temporary, ciphertext-only, and subordinate to the core API. It should not be a
durable evidence store, broad API gateway, GPS inspection service, admin route
host, decryption service, or public viewer edge.

The core API remains authoritative for:

- account/session or future upload authorization
- incident and stream state
- idempotency decisions
- duplicate reconciliation
- durable blob commits
- metadata
- deletion and retention behavior
- bundle reconstruction

Relay-local staged chunks are not backend-confirmed evidence. Near-live relay
fanout may be useful in the future, but trusted clients must label relay-fanned
chunks as unconfirmed until the core API commits encrypted bytes and metadata.

## Cloudflare and Edge Direction

Cloudflare can be an edge provider, WAF, tunnel provider, CDN/static host, and
abuse-control layer. For Proofline API, capture, viewer, and web-client paths,
treat Cloudflare and similar edges as untrusted metadata-observing
infrastructure, not as a confidentiality boundary.

Sensitive evidence, GPS/location context, detailed safety metadata, raw keys,
wrapped-key ciphertext where possible, verification credentials, session
tokens, viewer tokens, and private deployment details must not depend on
Cloudflare secrecy. Evidence and key material should be encrypted before
reaching the edge whenever the v1 design expects confidentiality from the
service operator or edge provider.

Community services have separate threat models. Challenge-first
Cloudflare/Bot Fight style protection may be appropriate for a community
service such as Redlib to protect a residential egress IP from bot abuse and
upstream blocking. That posture must not be copied blindly onto Proofline API,
capture, trusted-contact, or evidence paths.

## Public Registration and Quota Direction

Public open registration may be allowed only as an explicitly configured preview
deployment mode. It must not become the default.

Open registration requires:

- email verification before login
- email challenge, TOTP, or configured WebAuthn second-factor setup before main product-route access
- route-class rate limits
- reviewed public web origin and credentialed CORS/CSRF behavior when browser
  cookies are used
- deployment-edge abuse controls
- safe email and verification-token logging
- account lifecycle and account-state enforcement
- account-scoped committed blob quota before real public preview use

A 10 GB account committed blob storage limit is an acceptable preview default
after quota enforcement exists. Quota is an abuse and cost-control mechanism,
not billing.

Do not introduce payment providers, paid registration, subscriptions, account
plans, or hosted-account entitlements as default v1 preview requirements. The
Stripe billing design remains future hosted-service planning unless a separate
task explicitly scopes it.

## Deployment Responsibility Split

Do not describe the Go API as inherently unsafe or not public-ready merely
because deployment hardening is required. Instead, be specific.

The application is responsible for:

- authentication and authorization
- route separation
- private-admin listener separation
- request-size limits
- route-class rate limits
- safe logging and safe errors
- token hashing and token state handling
- immutable upload and storage invariants
- ZIP entry name control
- ciphertext-only evidence handling

The operator is responsible for:

- TLS termination and HSTS at the reviewed HTTPS edge
- reverse-proxy path routing
- firewall, VPN, WireGuard, or tunnel policy
- WAF and deployment-edge rate limits
- public/private listener reachability
- proxy log redaction for token-bearing paths
- monitoring and alerting
- backups, restore drills, and storage encryption
- secret management
- incident response and operational review

The release process is responsible for:

- review against source-of-truth docs
- validation commands
- security and route-exposure review
- release notes for behavior or compatibility changes
- no unsupported production-readiness claims

## Admin and Operator Boundary

Private admin and operator surfaces must stay separate from public product and
viewer surfaces. Private placement is not a substitute for admin
authentication. Admin/operator work should be narrow, audited where
appropriate, and careful not to expose evidence contents, raw tokens, raw keys,
plaintext, stored paths, object keys, wrapped-key ciphertext, private
deployment details, or user safety data.

No public admin dashboard is part of v1 preview direction.

## Logging Direction

All logging changes should follow [logging requirements](logging-requirements.md).
Logs should use stable, low-cardinality structured fields such as component,
operation, startup stage, route class, error category, safe detail, status, and
safe counts.

Do not log raw `err.Error()` by default in startup, config, request, upload,
storage, token, key, auth, object-store, email, deletion, retention, backup, or
user-safety paths. Do not log request bodies, uploaded bytes, Authorization
headers, cookies, raw tokens, plaintext, raw keys, contact private keys,
wrapped-key ciphertext, browser fragment secrets, stored paths, staging paths,
object keys, database DSNs, secret file paths, private endpoints, or user safety
data.

If a future route, relay, web-client deployment, browser recorder, decrypt path,
trusted-contact flow, notification flow, billing flow, or operator tool needs
new logs, define safe fields and redaction tests before or alongside the
implementation.

## Documentation Source Map

Read these docs together:

| Topic | Source docs |
|---|---|
| Current server overview and limits | [README](../README.md), [docs index](README.md), [architecture](architecture.md), [code map](code-map.md) |
| V1 preview release gate | [v1 preview readiness checklist](v1-preview-readiness-checklist.md) |
| Current API behavior | [API](api.md), [configuration](configuration.md), [deployment](deployment.md) |
| Security boundaries | [security model](security-model.md), [threat model](threat-model.md), [SECURITY](../SECURITY.md), [public API listener split](public-api-listener-split.md) |
| Incident modes | [incident modes](incident-modes.md), [mode-aware retention policy](mode-aware-retention-policy.md) |
| Capture variants and live access | [capture stream variants](capture-stream-variants.md), [live partial stream access](live-partial-stream-access-boundary.md), [regional stream ingress relay](regional-stream-ingress-relay.md) |
| Post-quantum envelope, key custody, and decryption | [post-quantum envelope](post-quantum-envelope.md), [key custody](key-custody.md), [contact key sharing](contact-key-sharing-grants.md), [contact-wrapped simulator metadata](contact-wrapped-key-metadata-simulator.md), [browser decryption](browser-decryption.md), [break-glass access](break-glass-key-access.md) |
| Retention and deletion | [retention, backup, and deletion](retention-backup-deletion.md), [incident deletion and retention enforcement](incident-deletion-retention-enforcement.md) |
| Logging | [logging requirements](logging-requirements.md) |
| Cluster and deployment operations | [production cluster scope](production-cluster-scope.md), [cluster backup and restore runbook](cluster-backup-restore-runbook.md), [PostgreSQL migration path](postgresql-metadata-migration.md) |
| Billing planning | [Stripe subscription billing](stripe-subscription-billing.md), but treat it as future hosted-service planning, not default v1 preview scope |
| Codex workflow | [codex README](../codex/README.md), reusable prompts under `codex/prompts/` |

Published technical reports under `docs/reports/` are historical
reviewed-commit artifacts. Use them for context, not as fresher truth than
current docs and code.

## Codex Guidance

When a future Codex task touches Proofline v1 direction:

1. Start with the requested project context and change-control prompts when the
   maintainer asks for them.
2. Read this document, then the topic-specific source docs above.
3. Preserve exact maintainer inputs such as issue number, PR number, reviewed
   SHA, base branch, and review mode.
4. Keep changes scoped to the requested repository and task.
5. Use present tense only for behavior verified in current code or current docs.
6. Use future tense for v1 direction not implemented yet.
7. Do not convert this direction into implementation permission.
8. Do not broaden server work into web-client, iOS, Android, protocol,
   deployment automation, billing, notifications, or public admin work.
9. Do not create public GitHub issues from backlog scans until the maintainer
   explicitly asks.
10. If docs conflict, state the conflict and recommend the smallest follow-up
    docs or backlog item instead of silently choosing an implementation.

## Actual v1 Non-Goals Versus Prototype Non-Goals

Some current non-implemented items are v1 direction. Others are actual default
v1 non-goals.

In v1 direction, but not implemented now:

- web-client no-account viewer replacement
- full account-owner incident review UX
- trusted-contact invite/accept UX
- browser-held private keys where the accepted design calls for them
- post-quantum evidence envelope fully implemented, documented, tested, and
  default for server and web-client preview paths
- browser-side encryption for browser capture
- authorized browser-side decryption for review
- browser recording as a reviewed secondary/fallback capture path
- device recipient-key registration, approval, and CEK rewrapping
- capture stream groups, concrete stream variants, and supersession metadata
- encrypted GPS/location/context evidence design
- public open registration only when explicitly configured and controlled
- account-scoped committed blob quota for public preview deployments

Out of default v1 preview scope unless separately designed:

- backend decryption in the normal path
- raw server-held CEKs, raw media keys, recipient private keys, or plaintext
- default key escrow
- server-side decryption outside explicit break-glass or dead-man-switch mode
- emergency-services integration or guaranteed emergency response
- public admin dashboard
- broad unreviewed public `/v1` exposure
- treating Cloudflare or any edge as a confidentiality boundary
- plaintext GPS/location relay metadata
- playable decrypted media exports
- payment providers, subscriptions, account plans, and hosted entitlements as
  v1 preview requirements
- SMS, Messenger, push notifications, or notification delivery as incidental
  behavior
- Docker Compose, Kubernetes, Terraform, or provider-specific deployment
  automation as incidental server work

## Documentation Conflicts To Recheck

Current docs are broadly aligned, but future work should watch these seams:

- Current server docs describe the built-in token viewer as implemented
  behavior. V1 direction expects the web client to replace it before v1 preview.
- Current server and web-client docs describe browser decryption and recording
  as not implemented. That remains true, but neither should be treated as a
  permanent v1 preview non-goal.
- Current post-quantum envelope docs describe an implementation plan, not
  runtime behavior. That remains true now, but the post-quantum envelope should
  be treated as a v1 preview requirement, not optional later hardening.
- Current billing docs describe a future Stripe-hosted service boundary. That
  should not become a default v1 preview requirement unless separately scoped.
- Current runtime protocol and default data-layout identifiers use Proofline
  names. Historical reports and archived prompts may still mention earlier
  `safety-recorder` identifiers; keep any old runtime identifiers limited to
  explicit fail-closed test fixtures.
- Sibling web-client docs may use product-language direction that depends on
  future server behavior. Server truth remains authoritative for implemented
  backend claims.
