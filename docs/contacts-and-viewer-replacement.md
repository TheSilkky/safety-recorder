# Contacts, Key Model, And Viewer Replacement

Status: design context only. This document does not change runtime behavior,
schemas, route registration, migrations, handlers, tests, deployment
configuration, account policy, notifications, or key custody.

This is the backend design note for Proofline's future trusted-contact model,
recipient key lifecycle, no-account viewer replacement, signed-in
trusted-contact access, break-glass/dead-man-switch escalation, GPS privacy
boundary, and post-quantum recipient alignment. It exists so later backlog scans
can produce narrower design and implementation issues.

The encrypted GPS/location evidence model is defined in
[encrypted-location-context.md](encrypted-location-context.md). This document
keeps the product and access-boundary context for that model.

Referenced product direction:
[Proofline Android iOS Concepts](https://www.figma.com/design/C7ojEm3GNfZ7zfFP7jPK4z/Proofline-Android---iOS-Concepts).
The Figma file metadata was accessible during this pass. The inspected page,
`Proofline Android App - Material 3`, includes dedicated frames for trusted
contacts, trusted-contact alert, evidence review, and key custody. It was used
only as product direction for future invite/accept, alert, evidence, and key
custody flows; it does not override the current backend source of truth.

## Purpose

Proofline currently has two related but separate sharing concepts:

- a no-account, bearer-token incident viewer path for people who receive an
  incident viewer link; and
- owner-scoped metadata APIs for contact public keys, sharing grants, and
  wrapped keys that prepare for a future account-based trusted-contact system.

The future product should replace the server-rendered built-in incident viewer
with a web-client viewer while also designing a real trusted-contact account
flow for alerts, recovery help, and wrapped-key delivery. These paths must stay
separate. A viewer-token holder is not an account contact, does not become a
trusted contact, and does not gain account, grant, decryption, recovery,
dispatch, or write capability from the token alone.

The future key architecture should also replace any prototype assumption that
each incident is its own private-key identity. Private keys should belong to
accounts, devices, and trusted-contact recipient key records. Content-encryption
keys should belong to incidents, streams, or bounded chunk groups. Wrapped
access records should connect authorized recipient key versions to those content
keys.

## Experimental Compatibility Policy

Proofline is experimental, maintainer-led, and has no public production
deployments. This design may replace prototype key models rather than preserving
them indefinitely. Backwards compatibility should be explicit and justified, not
automatic.

Do not preserve a per-incident private-key model solely for compatibility. If
old local development data or current tests need a migration path, document that
as a future migration issue. Do not add complex dual-stack compatibility unless
source docs or accepted migration work prove it is needed.

Compatibility still matters for file formats, encryption scheme identifiers,
test vectors, and explicit migration planning. The current runtime v1 envelope
and the v1-required post-quantum envelope must remain distinguishable by
documented scheme and suite identifiers.

## Current Backend State

### Listener And Route Boundaries

The server has separate listener groups and muxes:

- the main API/viewer listener for authenticated non-admin `/v1` routes,
  current prototype/local `/i/{token}` incident viewer routes, legacy
  `/e/{token}` aliases only when explicit local/test compatibility needs them,
  and token-neutral `/static/...` viewer assets; and
- the private admin listener for `/admin`, `/admin/static/...`, and
  authenticated admin-only `/v1/admin/...` JSON routes.

Replacing the built-in incident viewer must not make all of `/v1` public-ready.
It must not move `/admin`, `/v1/admin/...`, private diagnostics, write/admin
routes, operator routes, escrow routes, or break-glass routes onto public
incident-viewer edges.

### Viewer Tokens And Built-In Viewer

The current owner-authenticated API can create incident viewer tokens with
`POST /v1/incidents/{incident_id}/incident-tokens`. The raw token is returned
once, no-store headers are applied, and the metadata backend stores only a
token hash. Tokens can be revoked with
`POST /v1/incident-tokens/{token_id}/revoke`.

The current public viewer routes are read-only:

- `GET /i/{token}`
- `GET /i/{token}/data`
- `GET /i/{token}/streams/{stream_id}/download`
- `GET /i/{token}/incident/download`
- legacy `/e/{token}` aliases for the same viewer behavior

Invalid, expired, and revoked viewer tokens collapse to the same public error.
The viewer-token URL is a bearer secret. It must not be logged, copied into
public issue text, printed in diagnostics, embedded in public examples, or
stored in analytics labels.

The current token-scoped JSON payload exposes server-defined read-only incident
metadata for the built-in viewer, including incident status, latest check-in
device/location metadata when present, chunk summaries, media and stream
summaries, completed-stream information, warning text, and generation time. It
does not expose stored filesystem paths, blob object keys, uploaded bytes,
plaintext, raw media keys, contact private keys, raw viewer tokens, or
decryption material.

The current viewer download routes serve completed encrypted ZIP evidence
bundles only. Completed bundles are encrypted chunk bundles, not decrypted or
playable media exports. Generated ZIP entry names are controlled by the server.

The backend includes owner-authenticated viewer-token metadata list/read routes
for owned incidents. They expose token IDs, labels, active/expired/revoked
state, and timestamps only. They do not return raw viewer tokens, token hashes,
public token lookup by token ID, token replay capability, trusted-contact
access, wrapped-key ciphertext, plaintext, raw keys, or decryption material.

### Accounts, Contacts, Grants, And Wrapped Keys

The current backend implements local account/session authentication on the main
`/v1` API, including username/password login, logout, account read, password
change, disabled-by-default configurable registration, SMTP-backed email
verification for open registration, and optional browser cookie sessions with
CSRF protection for unsafe browser requests.

Browser cookie auth does not make viewer-token access account-based. Viewer
tokens remain no-account bearer links. Account sessions are the expected base
for owner and future trusted-contact flows, but current account sessions do not
create trusted-contact relationships by themselves. The backend now supports
owner-created trusted-contact relationship invites that authenticated recipient
accounts can accept or decline. Relationship records alone do not grant
incident reads, notifications, or decryption, but accepted relationships can
authorize signed-in trusted-contact wrapped-key reads only when the contact key,
grant, and wrapped-key record filters also pass.

Current authenticated owner-scoped metadata APIs exist for:

- trusted-contact relationship metadata:
  `POST/GET /v1/trusted-contact-relationships`,
  `GET /v1/trusted-contact-relationships/{relationship_id}`,
  `POST /v1/trusted-contact-relationships/{relationship_id}/accept`,
  `POST /v1/trusted-contact-relationships/{relationship_id}/decline`,
  `POST /v1/trusted-contact-relationships/{relationship_id}/revoke`, and
  `POST /v1/trusted-contact-relationships/{relationship_id}/replace`;
- contact public-key metadata:
  `POST/GET /v1/contact-public-keys`,
  `GET/PATCH /v1/contact-public-keys/{public_key_id}`, and
  `POST /v1/contact-public-keys/{public_key_id}/revoke`,
  `POST /v1/contact-public-keys/{public_key_id}/lost`, and
  `POST /v1/contact-public-keys/{public_key_id}/replace`;
- incident/stream sharing grants:
  `POST/GET /v1/incidents/{incident_id}/sharing-grants`,
  `GET /v1/sharing-grants/{grant_id}`, and
  `POST /v1/sharing-grants/{grant_id}/revoke`; and
- wrapped-key records:
  `POST/GET /v1/incidents/{incident_id}/wrapped-keys`,
  `GET /v1/wrapped-keys/{wrapped_key_id}`, and
  `POST /v1/wrapped-keys/{wrapped_key_id}/revoke`.
- trusted-contact wrapped-key reads:
  `GET /v1/trusted-contact/incidents/{incident_id}/wrapped-keys` and
  `GET /v1/trusted-contact/wrapped-keys/{wrapped_key_id}`.

These are metadata routes. They do not implement trusted-contact incident read
access, notifications, browser decryption, backend decryption, public viewer
changes, raw key storage, or key escrow. Current wrapped-key records can store
encrypted media-key material and public wrapping metadata behind authenticated
owner and authorized trusted-contact routes, but they must not contain raw
media keys, contact private keys, plaintext, ML-KEM shared secrets,
decapsulation keys, browser fragment secrets, or server-decryptable material in
the default path.

### Current Encryption And Post-Quantum Status

The current runtime envelope is the documented simulator/development v1
client-side encryption envelope using AES-256-GCM chunks with client-held
symmetric key material. The backend stores ciphertext and validates hashes. It
does not decrypt chunks or bundles.

`docs/post-quantum-envelope.md` defines the v1-required, explicit post-quantum
recipient wrapping direction using `ML-KEM-768 + HKDF-SHA384 + AES-256-GCM`.
That document is design and implementation-plan context only. It does not
change the current runtime envelope, stored chunks, viewer behavior,
trusted-contact delivery, or key custody.

## End-User Friendly Contact Requirement

The intended trusted-contact UX is:

```text
Invite trusted contact
Accept invite
Share access
Stop sharing
Lost device
Replace key
```

It must not require normal users or trusted contacts to understand encryption,
manually create keys, paste public keys, choose algorithms, manage fingerprints
manually, or understand wrapping/envelope internals.

Clients should automatically generate and manage required key pairs, publish
required public key material to the backend, and use the trusted contact's
active public recipient key material when granting access to encrypted incident
data. Technical key details belong in backend/client docs, diagnostics, and
security review material, not in the primary user flow.

Manual public-key entry may exist only as a later advanced, reviewed technical
path for local development, migration, or security review. It must not be the
primary trusted-contact experience.

## Target Key Architecture

### Why Per-Incident Private Keys Are Not The Long-Term Model

Any model where each incident owns its own private key identity is a poor
long-term fit:

- a trusted contact public key should be reusable across many incidents;
- same-account multi-device access becomes messy if every incident has a
  separate private-key identity;
- browser, mobile-device, and recovery flows become brittle;
- trusted-contact UX becomes technical because users must reason about
  incident-specific keys rather than people and devices;
- v1-required post-quantum recipient keys fit better as durable account,
  device, or contact recipient key records; and
- per-incident, per-stream, or bounded chunk-group CEKs still provide blast
  radius isolation without turning every incident into a private-key identity.

The target model is:

```text
Private keys belong to accounts/devices/contacts.
Content-encryption keys belong to incidents/streams/chunk groups.
Wrapped-key records connect recipient key versions to content keys.
```

### Account And Device Recipient Keys

Future users may have one or more device, browser, or native-client recipient
key records. Private keys are held by clients/devices by default. Public
recipient key material is stored on the backend with scheme, suite, version,
state, timestamps, and audit metadata.

Keys should be versioned and support replacement, revocation, and lost-device
states. Normal users should see human state such as `Ready`, `Lost device`, or
`Replaced`, not raw key material or algorithm choices.

### Trusted-Contact Recipient Keys

Trusted-contact keys should be created through invite/accept flows. When a
contact accepts, the contact's client should automatically create key material
if needed, hold private key material locally, and publish public recipient key
material to the backend.

Owner clients and backend policy should reference accepted contacts and active
trusted-contact recipient key versions. Contact keys can be reused across many
incidents. They should not be represented as a paste-your-key workflow for
normal users.

### Incident, Stream, And Chunk-Group CEKs

Incidents, streams, or bounded chunk groups should use content-encryption keys.
CEKs are shorter-lived than account/contact recipient keys and are wrapped
separately for authorized recipient key versions.

Future work should decide the CEK scope per data class:

- incident CEK for compact metadata or small evidence bundles, when justified;
- stream CEK for normal audio/video/location streams;
- bounded chunk-group CEK for long-running or live streams that need key
  rotation; and
- separate encrypted metadata CEKs for sensitive GPS/context records if they are
  not stored inside media chunks.

Revocation stops future authorization and delivery. It cannot claw back CEKs,
wrapped records, plaintext, ciphertext, or bundles already received by an
authorized actor.

### Wrapped Access Records

The backend may store wrapped/encrypted key material and non-secret metadata
when explicitly designed. Records connect incidents, streams, or chunk groups to
recipient key versions. They should include scheme, suite, key version, key ID,
recipient role, grant binding, state, creation time, revocation time, and audit
metadata.

Wrapped access records must not contain raw CEKs, raw media keys, plaintext,
ML-KEM shared secrets, ML-KEM decapsulation/private keys, derived KEKs, browser
fragment secrets, request bodies, uploaded bytes, stored paths, object keys, or
private deployment details.

## Post-Quantum Recipient Alignment

The trusted-contact key model should align with the v1-required pure
post-quantum envelope plan:

- trusted contacts should eventually publish ML-KEM-768 encapsulation keys or
  compatible recipient public-key records;
- clients should hold ML-KEM-768 decapsulation keys privately;
- server-side records should store non-secret public key metadata, recipient key
  IDs, key states, scheme/suite IDs, and audit timestamps;
- recipient key IDs are lookup and audit identifiers, not proof of private-key
  control by themselves;
- proof-of-possession or client acceptance checks need separate design;
- key rotation, replacement, and lost-device flows must be user-friendly;
- the backend contract should be algorithm/version aware without requiring end
  users to choose algorithms;
- once runtime support exists, the backend should reject unknown mandatory
  schemes and suite IDs rather than silently downgrade; and
- the current runtime v1 envelope and v1-required post-quantum envelope must
  remain explicitly separated until implementation and migration work lands.

The backend should store public encapsulation key metadata and wrapped-key
records, not decapsulation/private keys, ML-KEM shared secrets, raw CEKs, raw
media keys, or plaintext in the default path. Wrapped/encrypted key material may
be stored only as explicitly designed metadata.

## Access Tier Model

### 1. Basic No-Account Incident Viewer

Purpose:

- basic notification and safety context;
- a clear signal that this person may need help; and
- advice to contact emergency services directly when appropriate.

This viewer may include only intentionally limited data such as:

- incident type or a viewer-safe incident label;
- whether the incident is marked emergency or urgent;
- started time;
- last update or check-in time;
- basic active, ended, unavailable, or revoked status;
- owner-provided message deliberately intended for token viewers; and
- emergency guidance wording.

This viewer must not include:

- full GPS route;
- chunk-by-chunk GPS samples;
- near-live audio/video;
- encrypted evidence;
- wrapped-key ciphertext;
- raw tokens;
- raw keys;
- full backend diagnostics;
- private deployment details; or
- admin/operator details.

If basic viewer location is included during a transition, it must be deliberate,
minimal, documented as `latest shared location` or `last reported location`, and
not described as live tracking or emergency dispatch.

Current legacy token-scoped encrypted bundle download routes are a separate
compatibility surface. Future viewer replacement work should decide whether
those routes remain legacy-only, move behind signed-in trusted-contact access,
or stay available under a separate explicitly named encrypted-bundle capability.

### 2. Signed-In Trusted-Contact Viewer

Purpose:

- full trusted-contact access after account sign-in and authorization.

Future flow:

- viewer link opens a native app if installed;
- otherwise it opens the web client;
- the user signs in or an existing session is reused;
- the backend verifies the trusted-contact relationship, grant, policy, and
  active recipient key state;
- the client receives encrypted/wrapped access material; and
- the client decrypts locally where supported.

This may include, when separately implemented and authorized:

- live or near-live GPS map;
- route lines reconstructed from encrypted chunk-attached GPS samples;
- speed and movement context;
- near-live uploaded audio/video chunks for active incidents;
- full incident timeline;
- uploaded evidence status;
- wrapped-key delivery; and
- trusted-contact actions.

Incident metadata/ciphertext reads, full trusted-contact viewer behavior,
notifications, and trusted-contact actions remain future-tense until
implemented. Signed-in trusted-contact wrapped-key reads exist only under the
explicit relationship, recipient-key, grant, and wrapped-key filters; they must
not be inferred from viewer-token access or incident-mode labels.

### 3. Dead-Man-Switch / Break-Glass Escalation

Purpose:

- serious escalation after a user-defined or system-defined dead-man-switch
  condition is met.

This is not ordinary sharing. Prefer a model that releases incident-scoped,
stream-scoped, or chunk-group-scoped access material, or releases already
wrapped access packages to trusted contacts. Avoid uploading or exposing
long-term account/device private keys where possible.

If any server-side break-glass key handling is proposed, it must be marked as a
separate future design requiring:

- threat model update;
- security model update;
- key-custody and encryption doc updates;
- user consent and policy UX;
- audit events;
- tests;
- deployment warnings; and
- a clear distinction from the default ciphertext-only path.

Critical native notifications to trusted contacts may be a future feature, but
they remain future-tense and platform-gated. Proofline must not claim emergency
dispatch or guaranteed emergency response.

## GPS, Location, And Stream Model

GPS/location data is high-sensitivity user safety data. Treating equivalent GPS
data as plaintext viewer metadata over TLS weakens the system confidentiality
goal even if the media envelope remains cryptographically sound. TLS is
transport protection, not sufficient protection for full-fidelity location
history.

Preferred layered model:

```text
Class A: Full GPS evidence
- per chunk
- full fidelity
- encrypted in the evidence envelope
- long-term record

Class B: Viewer context
- basic no-account viewer gets minimal safety/status context
- richer map-ready context is encrypted or signed-in trusted-contact scoped

Class C: Server/relay operational metadata
- no GPS
- no speed
- no heading
- safe opaque routing/status fields only
```

The detailed field taxonomy, binding requirements, and validation expectations
for this model live in
[encrypted-location-context.md](encrypted-location-context.md).

Full GPS evidence should be captured as stream/chunk context. Each media chunk,
for example every few seconds, should be able to carry its own available
GPS/location context. Fields may include latitude, longitude, accuracy,
altitude, altitude accuracy, speed, heading, client timestamp, server received
timestamp, location source/status, and freshness/staleness context when
available.

Speed is important because it can show whether someone may be travelling by car
or otherwise moving unsafely. That does not mean it belongs in plaintext server
logs, relay metadata, or basic token-viewer payloads.

The backend, relay, logs, metrics, storage paths, limiter keys, and diagnostics
should not see full-fidelity GPS unless a deliberate, documented lower-privacy
mode exists. Basic no-account viewer access should not require full route
access. Signed-in trusted contacts should get richer GPS context through
encrypted access and reviewed grants.

## Regional Stream Ingress Relay Boundary

The planned regional stream-ingress relay remains planning-only and must be
treated as an ingress/forwarding surface, not a GPS-inspection service.

Preferred relay model:

- relay receives encrypted evidence chunks;
- relay receives only opaque/safe routing metadata;
- relay stages ciphertext temporarily and disposably;
- relay asks the core API for authorization and final commit decisions;
- relay should not need plaintext GPS, speed, heading, chunk plaintext, raw
  keys, object keys, stored paths, or user safety data; and
- relay support for viewer context must not create a plaintext GPS side channel.

If viewer context updates are needed, prefer encrypted viewer-context blobs or
signed-in trusted-contact access over plaintext GPS updates. Relay logs, metrics,
readiness output, limiter keys, traces, and staging paths must not include raw
tokens, raw idempotency keys, request bodies, uploaded bytes, plaintext, raw
keys, full-fidelity GPS, object keys, stored paths, private deployment details,
or user safety data.

## Viewer Replacement API

The web-client viewer replacement should be designed as a stable API contract
before changing route ownership. The accepted routing decision is that future
canonical no-account viewer links should point at the web-client origin with a
fragment token, as documented in
[web-client-viewer-routing.md](web-client-viewer-routing.md). Remaining
implementation decisions:

- whether existing `/i/{token}` is removed from production paths, redirected to
  a web-client origin, or kept as an explicitly configured local-development
  fallback;
- when legacy `/e/{token}` aliases are removed or kept only for explicit
  local/test compatibility;
- whether the web-client viewer needs token-scoped JSON endpoints separate from
  authenticated owner/trusted-contact endpoints;
- how token-bearing paths avoid logging, referrer, analytics, browser history,
  screenshots, and public issue leakage;
- whether the server or web client constructs viewer links;
- whether `SAFE_PUBLIC_WEB_ORIGIN` remains email-verification-only or a future
  separate viewer/public-web origin is needed;
- whether basic no-account viewer payload and signed-in trusted-contact payload
  are separate endpoints; and
- which fields the basic viewer may include versus fields only decrypted or
  delivered to authorized trusted contacts.

The viewer replacement API must not expose stored paths, blob object keys, raw
token hashes, raw tokens after creation, uploaded encrypted bytes outside the
intended download routes, wrapped-key ciphertext to token-only viewers,
plaintext, raw keys, or any decryption material.

## Notification Boundary

Notification delivery is future work. The only implemented email path currently
documented is SMTP-backed email verification for open registration, not incident
or trusted-contact notification delivery.

Future notification work should define delivery channels, templates, retry and
suppression behavior, opt-out behavior, audit expectations, rate limits, abuse
controls, and safe logging rules before implementation. Notifications are not
emergency dispatch and are not guaranteed emergency response.

Server-sent notifications should not include decryption keys unless a separate
lower-privacy fallback explicitly designs and labels that behavior. Trusted
contact app notifications may point contacts to sign in or open the app to
decrypt locally. Critical iOS/Android notifications are future native-app and
platform-gated features. Notification templates must avoid leaking sensitive
data, and notification failures must not be treated as emergency-services
failures.

## Security And Privacy Requirements

This design preserves these constraints:

- Do not expose admin/operator surfaces publicly.
- Do not route `/admin`, `/v1/admin/...`, private diagnostics, raw backend
  errors, or operator-only routes from a public edge.
- Do not add backend decryption.
- Do not add browser decryption as an incidental change.
- Do not add trusted-contact decryption as an incidental change.
- Do not add raw server-held keys.
- Do not add key escrow as default behavior.
- Do not add break-glass access as default behavior.
- Do not add playable export.
- Do not add recording/capture implementation.
- Do not add emergency-services integration.
- Do not add notification delivery as an incidental side effect.
- Do not add OAuth/JWT as an incidental side effect.
- Do not add public admin dashboards.
- Do not add payment, billing, Stripe, subscription, or paid-registration
  behavior.
- Keep current behavior and future behavior clearly separated.
- Do not make normal users manage cryptographic details manually.

Key custody wording must stay careful. Do not turn "no server keys ever" into
an accidental permanent absolute if future key custody remains open. Server
storage of wrapped/encrypted keys may be acceptable only when explicitly
designed. Raw server-side key access or server-side decryption requires a
separate explicit break-glass/dead-man-switch/key-custody design, threat model,
security model, tests, and deployment warnings.

Client-managed private keys must not be uploaded to the backend by default.
ML-KEM decapsulation/private keys must remain client-held unless a later
explicit key-custody design says otherwise. ML-KEM shared secrets must never be
serialized, logged, stored, or included in manifests.

## Sensitive-Data Rules

This design note, future backlog seeds, implementation issues, tests, logs,
public examples, and final summaries must not include:

- raw tokens;
- viewer tokens;
- session tokens;
- CSRF token values;
- `Authorization` headers;
- request bodies;
- uploaded bytes;
- plaintext;
- raw keys;
- raw media keys;
- contact private keys;
- ML-KEM decapsulation keys;
- ML-KEM shared secrets;
- content-encryption keys;
- key-encryption keys;
- unwrapped secrets;
- wrapped-key ciphertext;
- exploit payloads;
- object-store credentials;
- stored paths;
- object keys;
- private deployment details;
- user safety data not intentionally shown to the current viewer;
- payment-provider secrets;
- payment account details; or
- public/private contact details from payment providers.

Use safe placeholders only when examples are necessary.

## Backlog Seed: Expected Issue Families

These are dependency-ordered issue-family seeds for a later backlog scan. They
are not backlog drafts and they are not GitHub issues.

1. **Contact/key model finalization.**
   Reasoning: very high. Type: design-only. Areas: this document,
   `docs/key-custody.md`, `docs/security-model.md`, `docs/threat-model.md`,
   `docs/api.md`, web-client coordination docs. Dependencies: current source
   inventory and maintainer acceptance of the durable-recipient-key model.
   Non-goals: no schema, runtime, migration, payment, notification, decryption,
   or manual-key UX implementation. Sensitive data: use only placeholders and
   do not include real contacts, keys, tokens, or safety narratives.
   Validation: manual docs cross-check and `git diff --check`.

2. **Replace per-incident private-key model with durable recipient keys plus
   incident/stream CEKs.**
   Reasoning: very high. Type: design/migration planning. Areas:
   `docs/encryption.md`, `docs/key-custody.md`,
   `docs/contact-key-sharing-grants.md`, future protocol docs. Dependencies:
   family 1. Non-goals: no dual-stack compatibility unless a migration issue
   proves it is needed; no raw key storage. Sensitive data: no CEKs, raw media
   keys, or private keys in examples. Validation: architecture review,
   migration-risk checklist, docs review.

3. **Post-quantum recipient public-key record and key ID contract.**
   Reasoning: very high. Type: design/test-vector support. Areas:
   `docs/post-quantum-envelope.md`, future envelope/protocol package, API docs.
   Dependencies: families 1 and 2. Non-goals: no runtime PQ implementation in
   the design issue; no user-selected algorithms. Sensitive data: no ML-KEM
   decapsulation keys or shared secrets. Validation: future conformance vectors,
   key ID canonicalization tests, unknown-suite rejection tests.

4. **End-user friendly trusted-contact invite/accept UX contract.**
   Reasoning: very high. Type: API implemented, client UX still future. Areas:
   server API docs, web-client/mobile coordination docs, account lifecycle docs.
   Dependencies: family 1. Non-goals: no notification delivery, no payment
   entitlement, no manual public-key paste as primary UX. Sensitive data:
   invites and contact identifiers are personal data. Validation: UX/security
   copy review and API state-machine review.

5. **Automatic client-generated trusted-contact key lifecycle API support.**
   Reasoning: high. Type: API design then implementation. Areas: contact
   public-key routes, accepted-contact relationship routes, client docs.
   Dependencies: families 3 and 4. Non-goals: no client private-key upload, no
   manual key creation flow. Sensitive data: public keys are non-secret but
   linkable metadata; private keys never leave clients. Validation: authz,
   state, versioning, replacement, and redaction tests.

6. **Account/device key registration, rotation, revocation, and lost-device
   state.**
   Reasoning: very high. Type: API design/implementation. Areas: account/device
   metadata, recipient key records, security model. Dependencies: families 2,
   3, and 5. Non-goals: no password recovery or OAuth/JWT unless separately
   scoped. Sensitive data: no device private keys, raw recovery phrases, or
   platform key material. Validation: device ownership, rotation, revocation,
   lost-device, and cross-account denial tests.

7. **Wrapped access record schema aligned with PQ envelope.**
   Reasoning: very high. Type: schema/API design and migration. Areas:
   sharing grants, wrapped-key records, metadata migrations, API docs, envelope
   docs. Dependencies: families 2, 3, 5, and 6. Non-goals: no raw CEKs, no
   backend decryption, no bundle-manifest exposure by default. Sensitive data:
   wrapped-key ciphertext is access-enabling and must be redacted from logs.
   Validation: migration tests, unknown-suite rejection, grant binding, and
   redaction tests.

8. **Sharing-grant policy updates for accepted contacts and active key
   versions.**
   Reasoning: high. Type: API implementation. Areas: sharing-grant repository,
   authz, API docs, audit docs. Dependencies: families 4, 5, and 7. Non-goals:
   no token-only trusted-contact status; no mode label as access grant.
   Sensitive data: contact roles and safety plans are sensitive. Validation:
   accepted/declined/revoked contact tests, active-key-only tests, expiry and
   revocation tests.

9. **Basic no-account viewer payload contract.**
   Reasoning: very high. Type: design/API implementation. Areas: viewer
   payload docs, `internal/httpapi` viewer routes, web-client viewer docs.
   Dependencies: viewer replacement decisions and field privacy review.
   Non-goals: no full GPS route, live media, wrapped keys, account auth, or
   decryption. Sensitive data: viewer links are bearer secrets; location fields
   require explicit review. Validation: field filtering tests, invalid/expired/
   revoked token collapse tests, no-store/header tests, copy review.

10. **Signed-in trusted-contact full incident viewer contract.**
    Reasoning: very high. Type: design/API implementation. Areas:
    trusted-contact authz, grant-scoped incident reads, wrapped-key delivery,
    web/mobile docs. Dependencies: families 4, 5, 7, and 8. Non-goals: no
    no-account access, no public admin, no emergency-services guarantee.
    Sensitive data: contact access and decrypted client views are high
    sensitivity. Validation: cross-account denial, grant/data-class filtering,
    wrapped-key delivery, and audit tests.

11. **Token-scoped web-client viewer replacement plan.**
    Reasoning: high. Type: design/deployment docs then implementation. Areas:
    `/i/{token}`, `/e/{token}`, viewer origin config, web-client viewer, reverse
    proxy guidance. Dependencies: family 9. Non-goals: no broad public `/v1`
    exposure and no analytics/referrer leakage. Sensitive data: token-bearing
    paths and links. Current decision: future canonical no-account viewer links
    should point at the web-client origin with a fragment token, while
    server-rendered `/i` and legacy `/e` routes are prototype/local
    compatibility only until a later runtime issue removes, gates, or redirects
    them. Validation: route/redirect tests, log redaction, no-referrer/no-store
    checks, web-client smoke tests.

12. **GPS/location per-chunk encrypted evidence model.**
    Reasoning: very high. Type: design/protocol/API implementation. Areas:
    stream/chunk metadata, encryption docs, future capture clients. Dependencies:
    family 2 and stream CEK decision. Non-goals: no plaintext route tracking or
    relay GPS inspection. Sensitive data: full location, speed, heading, and
    timestamps. Validation: envelope/AAD tests, chunk context tests, privacy
    review, redaction tests.

13. **Viewer-safe location/privacy model.**
    Reasoning: very high. Type: design/API implementation. Areas: viewer
    payload, trusted-contact viewer, web-client map UI, security/threat docs.
    Dependencies: families 9, 10, and 12. Non-goals: no live tracking claim and
    no map-provider backend integration by default. Sensitive data: coordinates,
    movement, speed, and freshness. Validation: field-level allowlist tests,
    copy review, map-link token leak review.

14. **Regional stream ingress relay privacy model.**
    Reasoning: high. Type: design-only before relay implementation. Areas:
    `docs/regional-stream-ingress-relay.md`, relay preflight/commit API,
    logging/metrics docs. Dependencies: family 12 and existing relay design.
    Non-goals: no relay GPS inspection, admin routes, durable evidence storage,
    or broad API gateway. Sensitive data: no raw tokens, idempotency keys, GPS,
    stored paths, object keys, or uploaded bytes in relay logs. Validation:
    threat-model review and future relay redaction tests.

15. **Dead-man-switch / break-glass escalation design.**
    Reasoning: very high. Type: security design-only first. Areas:
    `docs/break-glass-key-access.md`, `docs/key-custody.md`, security/threat
    docs, incident-mode docs. Dependencies: families 1, 2, 8, and 10. Non-goals:
    no default escrow, backend decryption, raw key access, or notification
    implementation. Sensitive data: policy triggers and emergency context are
    sensitive. Validation: threat-model review, audit design review, deployment
    warning review.

16. **Notification design and later notification implementation issues.**
    Reasoning: high. Type: design then implementation. Areas: future outbox,
    provider integrations, templates, trusted-contact preferences, abuse
    controls. Dependencies: families 4, 8, 10, and 15 as applicable. Non-goals:
    no emergency dispatch, no guaranteed response, no raw key delivery by
    default, no incidental SMS/push/Messenger. Sensitive data: messages,
    provider logs, links, and contact identifiers. Validation: template
    redaction, rate-limit, retry, opt-out, and failure-mode tests.

17. **Post-quantum envelope compatibility and migration planning.**
    Reasoning: very high. Type: migration/design/test support. Areas:
    `docs/post-quantum-envelope.md`, `docs/encryption.md`, simulator/protocol
    docs, future envelope code. Dependencies: families 2, 3, and 7. Non-goals:
    no transparent reinterpretation of v1 envelopes and no downgrade fallback.
    Sensitive data: no raw vectors containing real keys or user data.
    Validation: golden vectors, v1 compatibility tests, unknown-suite rejection,
    migration docs.

18. **Docs, threat-model, security-model, API docs, and prompt updates.**
    Reasoning: medium. Type: docs/prompt maintenance. Areas: `README.md`,
    `AGENTS.md`, `SECURITY.md`, docs index, reusable prompts. Dependencies:
    accepted design decisions or implementation changes. Non-goals: no runtime
    behavior. Sensitive data: no public issue text with secrets or safety data.
    Validation: manual docs consistency review and `git diff --check`.

19. **Migration/storage work needed for new persistent records.**
    Reasoning: high. Type: migration/implementation. Areas: SQLite/PostgreSQL
    migrations, repositories, backup/restore/deletion docs. Dependencies:
    families 5, 6, 7, 8, 10, and 17. Non-goals: no data loss, no public issue
    with private deployment details, no payment state. Sensitive data: migration
    logs must not expose keys, tokens, wrapped ciphertext, object keys, or
    paths. Validation: SQLite/PostgreSQL migration tests, backup/restore tests,
    deletion/tombstone tests.

20. **Tests and simulator/client-flow updates.**
    Reasoning: high. Type: test/client-flow support. Areas: `cmd/simclient`,
    future client docs, route tests, envelope tests. Dependencies: relevant
    implementation families. Non-goals: no production mobile/web implementation
    in this server repo unless separately scoped. Sensitive data: local
    simulator artifacts must stay ignored and must not print raw keys or tokens.
    Validation: focused Go tests, simulator smoke, redaction assertions, manual
    docs review.

Do not create issue families here for Stripe, card payments, payment providers,
donation platforms, paid registration, subscriptions, billing state,
payment-gated account creation, hosted-account entitlements, invoices,
receipts, tax, payout, statement descriptor handling, or manual user-facing key
creation/paste-your-key workflows as the primary trusted-contact flow.

## Explicitly Excluded From This Context

Payment, billing, Stripe, paid-registration design, donation providers,
subscriptions, hosted-account payment gating, invoices, receipts, tax, payout,
and statement descriptor handling are intentionally excluded. Existing
`SAFE_ACCOUNT_REGISTRATION_MODE=paid` behavior is only a fail-closed placeholder
and must not be expanded by this context.

Manual end-user cryptography workflows are intentionally excluded as the primary
trusted-contact experience. Normal users and trusted contacts should not be
asked to paste public keys, choose algorithms, manage fingerprints manually, or
inspect wrapped-key ciphertext as the normal flow.

Backwards compatibility with prototype per-incident private-key design should
not be prioritized over the intended durable-recipient-key model unless a later
migration issue proves it is needed.

This context also excludes runtime implementation, migrations, route handlers,
tests, deployment config, GitHub workflows, sibling repository work, backlog
draft files, public GitHub issue creation, commits, and PRs.

## Open Decisions

- Which later runtime issue removes, gates, or redirects the current
  server-rendered `/i/{token}` page after the web-client viewer exists, and
  whether any local-development fallback remains enabled by configuration.
- Whether `SAFE_PUBLIC_WEB_ORIGIN` should stay email-verification-only or be
  split from future viewer/public-web origin configuration.
- Which basic no-account viewer fields are acceptable after the GPS privacy
  review, especially any latest-location field.
- Whether trusted-contact full incident access should require native-app
  decryption first, web-client decryption first, or a staged approach.
- How proof-of-possession should work for ML-KEM recipient public-key records.
- How many active recipient keys an account/contact/device may hold.
- How lost devices and key replacement affect old wrapping records.
- Whether break-glass should ever include server-side raw key access or remain
  wrapped-key-release-only.
- How notification provider logs and templates avoid becoming token or safety
  data side channels.
- What migration is needed for any old prototype data that assumed per-incident
  private keys.

## Source Context Inspected

This document was updated against current server source and docs, including
`AGENTS.md`, `README.md`, `SECURITY.md`, `docs/README.md`, `docs/api.md`,
`docs/architecture.md`, `docs/deployment.md`, `docs/security-model.md`,
`docs/threat-model.md`, `docs/encryption.md`,
`docs/post-quantum-envelope.md`, `docs/key-custody.md`,
`docs/incident-modes.md`, `docs/v1-access-control.md`,
`docs/public-api-listener-split.md`,
`docs/retention-backup-deletion.md`,
`docs/incident-deletion-retention-enforcement.md`,
`docs/live-partial-stream-access-boundary.md`,
`docs/regional-stream-ingress-relay.md`,
`docs/contact-key-sharing-grants.md`,
`docs/contact-wrapped-key-metadata-simulator.md`, `codex/README.md`,
`codex/prompts/00-project-context-check.md`,
`codex/prompts/05-codex-change-control.md`,
`codex/prompts/35-key-custody-and-emergency-access-design.md`,
`codex/prompts/38-break-glass-and-dead-mans-switch-key-access-design.md`, and
`codex/prompts/75-create-draft-pr-from-current-branch.md`.

Implementation inspection was limited to verifying current backend truth in
`internal/httpapi/routes.go`, `internal/httpapi/incident_viewer.go`, bundle,
sharing-grant, wrapped-key, stream, upload, auth, and admin route tests, plus
incident metadata, checkin, stream, contact public-key, sharing-grant, and
wrapped-key repository models and migrations under `internal/incidents`,
`internal/db`, and `migrations`.

Sibling web-client docs were inspected read-only for coordination context:
`../web-client/README.md`, `../web-client/SECURITY.md`,
`../web-client/docs/api-client.md`,
`../web-client/docs/security-model.md`,
`../web-client/docs/viewer-token-ui-design.md`, and
`../web-client/docs/end-user-web-client-design.md`.

The Figma connector was used read-only to inspect the top-level page metadata
for the linked Proofline Android/iOS concepts file.
