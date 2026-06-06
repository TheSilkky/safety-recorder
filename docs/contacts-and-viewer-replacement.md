# Contacts and Viewer Replacement

Status: design context only. This document does not change runtime behavior,
schemas, route registration, migrations, handlers, tests, deployment
configuration, account policy, notifications, or key custody.

Referenced product direction:
[Proofline Android iOS Concepts](https://www.figma.com/design/C7ojEm3GNfZ7zfFP7jPK4z/Proofline-Android---iOS-Concepts).
The Figma file was available during this pass and was used only as product
direction for future trusted-contact and viewer flows. It does not override the
current backend source of truth.

## Purpose

Proofline currently has two related but separate sharing concepts:

- a no-account, bearer-token incident viewer path for people who receive an
  incident viewer link; and
- owner-scoped metadata APIs for contact public keys, sharing grants, and
  wrapped keys that prepare for a future account-based trusted-contact system.

The intended product direction is to replace the server-rendered built-in
incident viewer with a web-client viewer while also designing a real
trusted-contact account flow for alerts, recovery help, and future wrapped-key
delivery. These two paths must stay separate. A viewer-token holder is not an
account contact, does not become a trusted contact, and does not gain any
account, grant, decryption, recovery, dispatch, or write capability from the
token alone.

This note is a backend context document for later issue drafting. It records
current backend truth, target architecture, non-goals, sensitive-data limits,
and expected issue families. It is not a backlog scan and does not create
backlog draft files or GitHub issues.

## Current Backend State

### Listener and Route Boundaries

The server still has two listener groups and separate muxes:

- the main API/viewer listener for authenticated `/v1` routes, canonical
  `/i/{token}` incident viewer routes, legacy `/e/{token}` compatibility
  aliases, and token-neutral `/static/...` viewer assets; and
- the private admin listener for `/admin` dashboard routes, private admin
  assets, and private-admin JSON behavior.

Replacing the built-in incident viewer with a web-client viewer must not make
all of `/v1` public-ready. It must not move private write/admin routes onto a
public incident-viewer edge. The private `/admin` dashboard and private admin
JSON routes stay on their own listener boundary.

### Viewer Tokens and Built-In Viewer

The current owner-authenticated API can create incident viewer tokens with
`POST /v1/incidents/{incident_id}/incident-tokens`. The raw token is returned
once, no-store headers are applied, and the repository stores only a token
hash. Tokens can be revoked with `POST /v1/incident-tokens/{token_id}/revoke`.

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

A non-secret owner API for listing or reading viewer-token metadata was not
found in the current backend or sibling web-client docs during this pass. Later
viewer-management UI work should treat that as a likely missing API contract,
not as an implemented capability.

### Account Sessions

The current backend implements local account/session authentication on the main
`/v1` API, including username/password login, logout, account read, password
change, configurable registration, email-verification behavior, and optional
browser cookie sessions with CSRF protection for unsafe browser requests.

Browser cookie auth does not make viewer-token access account-based. Viewer
tokens remain no-account bearer links. Account sessions are the expected base
for owner and future trusted-contact flows, but current account sessions do not
create trusted-contact relationships by themselves.

### Contact Public-Key Metadata

The current main API has authenticated, owner-scoped contact public-key metadata
routes:

- `POST /v1/contact-public-keys`
- `GET /v1/contact-public-keys`
- `GET /v1/contact-public-keys/{public_key_id}`
- `PATCH /v1/contact-public-keys/{public_key_id}`
- `POST /v1/contact-public-keys/{public_key_id}/revoke`

Current records include owner account ID, contact ID, key version, display
label, wrapping algorithm, public key material, public-key fingerprint, key
state, and timestamps. They do not contain contact private keys, raw media
keys, plaintext, or decryption capability.

This is not yet a full trusted-contact relationship lifecycle. The existing
`contact_id` is owner-scoped metadata, not proof that another account accepted
an invitation, controls a device key, or agreed to receive alerts or recovery
material.

### Sharing Grants

The current main API supports authenticated owner-scoped sharing-grant metadata
for an owned incident:

- `POST /v1/incidents/{incident_id}/sharing-grants`
- `GET /v1/incidents/{incident_id}/sharing-grants`
- `GET /v1/sharing-grants/{grant_id}`
- `POST /v1/sharing-grants/{grant_id}/revoke`

Sharing grants can reference an incident, optional stream, recipient type,
contact ID, contact public-key record and version, data class, expiry, and
state. They are metadata contracts. They do not carry raw keys or plaintext and
do not by themselves deliver trusted-contact account access.

### Wrapped-Key Metadata

The current main API supports authenticated owner-scoped wrapped-key metadata:

- `POST /v1/incidents/{incident_id}/wrapped-keys`
- `GET /v1/incidents/{incident_id}/wrapped-keys`
- `GET /v1/wrapped-keys/{wrapped_key_id}`
- `POST /v1/wrapped-keys/{wrapped_key_id}/revoke`

Wrapped-key records can store encrypted media-key material, wrapping algorithm
metadata, recipient/contact references, public wrapping metadata, and state.
They must never store raw media keys, contact private keys, plaintext,
unwrapped shared secrets, browser fragment secrets, server escrow keys, or any
server-decryptable material. Public wrapping metadata is constrained to a JSON
object and excludes sensitive key names.

These owner-scoped routes are not a trusted-contact delivery API. A future
contact read path must define which authenticated contact can receive which
grant and which wrapped-key ciphertext. Token-only public viewer access should
not deliver wrapped keys by default.

### Current Encryption and Post-Quantum Status

The current runtime envelope is the existing simulator/development v1
client-side encryption envelope using AES-256-GCM chunks with client-held
symmetric key material. The backend stores ciphertext and validates hashes. It
does not decrypt chunks or bundles.

`docs/post-quantum-envelope.md` defines a future, explicit post-quantum
recipient wrapping direction using `ML-KEM-768 + HKDF-SHA384 + AES-256-GCM`.
That document is design/implementation planning only. It does not change the
current runtime envelope, stored chunks, viewer behavior, trusted-contact
delivery, or key custody.

### Capabilities Not Present Today

The current backend does not implement:

- a trusted-contact account relationship lifecycle with invite, accept,
  decline, revoke, replacement, or recovery-contact role management;
- automatic client-generated trusted-contact key lifecycle across mobile and
  web clients;
- notification delivery by SMS, email, push, Messenger, or other channels;
- emergency-services dispatch;
- mode-driven access, trusted-contact account access, or dead-man switch
  behavior;
- backend decryption, browser decryption, trusted-contact decryption, server
  key escrow, raw server-held keys, or playable media export;
- a broad public `/v1` product deployment model; or
- payment-backed account entitlement.

Paid registration is currently a placeholder/fail-closed registration path when
configured as unavailable. It is not part of this contacts or viewer
replacement context.

## Target Architecture

### No-Account Viewer-Token Path

The viewer-token path remains a no-account contact path. A token holder can see
only the read-only server-defined viewer payload for the incident associated
with that token. The token does not prove identity and does not imply trusted
contact status.

The replacement web-client viewer should preserve the current token semantics:

- token input is a bearer secret;
- invalid, expired, and revoked tokens have indistinguishable public failures;
- responses are no-store and avoid sensitive metadata leakage;
- payload fields are intentionally limited and documented;
- public viewer routes stay read-only; and
- token-only viewer access does not receive account APIs, write APIs, grants,
  raw keys, wrapped keys, plaintext, or decrypted media.

The web-client viewer can replace server-rendered HTML once the backend exposes
a stable viewer API contract and the deployment boundary is documented. This is
a viewer replacement, not a broad public API exposure project.

### React Web-Client Viewer Rationale

The built-in server-rendered incident viewer should move toward the React
web-client because the web-client can provide a substantially better
no-account viewer experience for trusted recipients who open a viewer-token
link. Repository separation is useful, but it is not the main reason to move
the viewer. The main product advantage is a responsive, accessible, client-side
viewer that can present the token-scoped incident status and latest shared
location clearly on mobile and desktop without adding UI-specific behavior to
core backend storage.

The intended web-client viewer should support:

- a simple responsive layout for mobile and desktop;
- clear read-only incident status;
- latest check-in or last update time;
- latest shared location, or last reported location, when the server viewer
  payload includes it;
- an interactive map for location context;
- visible freshness and staleness indicators for location data;
- accessible fallback text for coordinates or location if the map fails;
- explicit `Open in Apple Maps` and `Open in Google Maps` actions; and
- clear warning copy that Proofline is not emergency dispatch and that location
  may be delayed, stale, unavailable, or approximate.

The backend design should treat the map as a client presentation feature. The
backend should expose only a documented, public-safe, token-scoped viewer
payload. It should not embed map provider behavior, map API keys, frontend
navigation URLs, or UI-specific assumptions into core incident storage.

### Viewer Payload for Map Support

When a future web-client viewer payload is formalized, it should support map
presentation without exposing internal storage, key, deployment, or diagnostic
data. Likely token-scoped fields include:

- incident ID or a viewer-safe incident reference;
- incident state;
- latest check-in timestamp;
- latest location timestamp, when present;
- latitude and longitude, when intentionally included in the viewer payload;
- optional accuracy radius;
- optional altitude, heading, or speed only if deliberately supported and
  documented;
- source/client timestamp and server-received timestamp, when both are useful
  for freshness decisions;
- freshness or staleness status derived from documented backend rules;
- optional owner-provided context intended for token viewers; and
- safe device state only if deliberately documented.

The viewer payload must not include:

- raw viewer tokens;
- session tokens;
- `Authorization` headers;
- stored paths;
- object keys;
- private deployment details;
- request bodies;
- uploaded bytes;
- plaintext;
- raw keys;
- raw media keys;
- contact private keys;
- wrapped-key ciphertext;
- user safety narrative not explicitly intended for token viewers;
- admin/operator details; or
- backend diagnostics.

This payload shape should use wording such as `latest shared location` or
`last reported location` unless the backend contract actually supports
real-time streaming updates. The backend should not imply guaranteed live
tracking, emergency dispatch, or emergency-services integration.

### Map and Navigation Boundaries

Map display is a web-client responsibility. Apple Maps and Google Maps
navigation links should be generated client-side from viewer-safe coordinates
already present in the token-scoped payload.

Navigation links must not include viewer tokens unless a separate reviewed
route design explicitly requires it. External map links may disclose the
coordinates to the selected map provider once the viewer clicks them, so the UI
should make the action explicit with labels such as `Open in Apple Maps` and
`Open in Google Maps`.

The backend should not treat map navigation as emergency response. It should
not guarantee that the latest shared location is current, precise, or
available.

### Account-Based Trusted Contacts

Trusted contacts are a separate account-based system. A future trusted contact
should have or create an account, receive an invitation from an owner, and move
through a simple lifecycle such as invited, accepted, declined, revoked, and
replaced.

The target user experience should avoid manual cryptography. Users should not
have to create keys by hand, paste cryptographic material, inspect raw key
records, or understand wrapping algorithms to add a trusted contact. Clients
should generate and rotate device/account key material automatically, publish
public key material to the backend through authenticated APIs, and keep private
key material outside the backend.

The Figma mobile direction supports this separation: trusted contacts appear as
people selected for alerts, location, and recovery help; recovery access is
explained as account/contact behavior; and the design states that Proofline has
not contacted emergency services automatically. Those screens are product
direction, not implemented server behavior.

### Post-Quantum Trusted-Contact Key Model

The future trusted-contact key model should align with the post-quantum envelope
plan:

- client-managed recipient keys for trusted contacts;
- backend storage of public key material, non-secret key IDs, key state, key
  version, and audit metadata;
- future wrapping records that use `ML-KEM-768 + HKDF-SHA384 + AES-256-GCM`
  where the payload actually uses that suite;
- explicit suite IDs and compatibility behavior so v1 encrypted chunks continue
  to work; and
- tests that prove recipient isolation, tamper rejection, unknown-suite
  rejection, and late-recipient metadata behavior.

The backend may store public keys, KEM ciphertexts, salts, nonces, wrapped CEK
ciphertext, public wrapping metadata, and audit metadata when those fields are
explicitly designed. It must not store raw CEKs, decapsulation keys, shared
secrets, derived KEKs, plaintext media, decrypted caches, browser-submitted
private keys, or server-held escrow material unless a separate explicitly
approved emergency-access design changes that boundary.

### Sharing and Wrapped-Key Relationship

The future contact model should make these relationships explicit:

- account relationship: who the owner trusts and what role they accepted;
- contact public-key lifecycle: which account/device key is active for future
  recovery or delivery;
- sharing grant: which incident or stream may be shared, to which contact, for
  which data class, and under which expiry/revocation state;
- wrapped-key record: which encrypted media-key material was prepared for which
  grant and recipient key version; and
- audit/read model: who viewed metadata, downloaded ciphertext bundles, received
  wrapped-key material, or had access revoked.

Revocation should stop future delivery and future access checks. It cannot
claw back data, links, encrypted bundles, or wrapped material that a recipient
already downloaded.

### Viewer Replacement API

The web-client viewer replacement should be designed as a stable API contract
before changing route ownership. Likely requirements include:

- a token-scoped viewer payload endpoint that keeps current read-only semantics;
- optional token-scoped encrypted bundle download routes, still ciphertext-only;
- an owner-authenticated viewer-token management API that can list non-secret
  token metadata, show expiration/revocation state, and revoke tokens without
  exposing raw tokens;
- a clear decision for whether server-rendered `/i/{token}` redirects to the
  web client, serves a minimal fallback, or remains during a compatibility
  window; and
- documentation for CORS, no-store headers, token handling, cache behavior,
  error normalization, and rate limiting.

The viewer replacement API must not expose stored paths, blob object keys,
raw token hashes, raw tokens after creation, uploaded encrypted bytes outside
the intended download routes, wrapped-key ciphertext to token-only viewers,
plaintext, or any decryption material.

### Notification Boundary

Notification delivery is a separate future boundary. Viewer-token links may
eventually be used in notification/contact messages, but the presence of a link
does not make the recipient a trusted contact and does not prove account
identity.

Future notification work should define delivery channels, message templates,
retry and suppression behavior, opt-out behavior, audit expectations, rate
limits, and safe logging rules. It must not log raw viewer tokens, incident
tokens, request bodies, private deployment details, or user safety data.

Emergency-services dispatch is not implemented and is not part of this
replacement design. Product copy and API behavior must not imply automatic
dispatch.

## Security and Privacy Requirements

- Preserve ciphertext-only backend behavior.
- Preserve main `/v1` and private `/admin` listener separation.
- Keep public incident-viewer routes read-only.
- Treat viewer-token URLs as bearer secrets.
- Return raw viewer tokens only once at creation time.
- Store only viewer-token hashes.
- Keep token invalid, expired, and revoked failures indistinguishable on public
  routes.
- Do not log raw tokens, authorization headers, request bodies, uploaded bytes,
  plaintext, raw keys, wrapped-key ciphertext, private deployment details, or
  user safety data.
- Do not expose filesystem paths or blob object keys.
- Do not make encrypted bundles playable server-side.
- Do not deliver wrapped keys through token-only viewer access by default.
- Keep trusted-contact relationship work account-authenticated and
  grant-scoped.
- Keep client private keys outside the backend.
- Update the threat model, security model, encryption docs, API docs, and
  operational guidance before or alongside any implementation that changes key
  custody, decryption, viewer exposure, contact delivery, notification, or
  emergency-access behavior.

## Backlog Seed: Expected Issue Families

These are issue-family seeds for a later backlog scan. They are not backlog
drafts and they are not GitHub issues.

| # | Family | Reasoning level | Work type | Likely affected areas | Dependencies | Non-goals | Sensitive-data warnings | Validation expectations |
|---|---|---|---|---|---|---|---|---|
| 1 | Current route and docs inventory for contacts/viewer | Low | Docs/test planning | `docs/api.md`, `docs/security-model.md`, `internal/httpapi` route inventory | None | No behavior change | Do not paste raw token examples | Manual route/docs cross-check |
| 2 | Owner viewer-token metadata list/read API | Medium | Backend API | Incident-token repository, owner auth handlers, API docs | Current create/revoke token flow | No raw token replay | Never return raw tokens or hashes | Unit tests for ownership, redaction, revoked/expired states |
| 3 | Token-scoped web-client viewer payload for status, latest check-in, and map-ready location context | Very-high | Viewer UI/API | `docs/contacts-and-viewer-replacement.md`, `docs/api.md`, `docs/security-model.md`, `docs/threat-model.md`, viewer-token route handlers, viewer payload schemas, tests for token-scoped viewer payload safety, web-client viewer route and map UI in a later frontend issue | Viewer-token model confirmed; viewer replacement route shape decided; safe viewer payload fields documented; token leakage/referrer/logging risks reviewed | No emergency dispatch, emergency-services integration, guaranteed live tracking, notification delivery, browser decryption, trusted-contact decryption, playable export, map-provider backend integration, backend map API keys, or private incident/storage/key/deployment detail exposure | Location and context are sensitive; never return raw tokens, session tokens, stored paths, object keys, plaintext, raw keys, wrapped-key ciphertext, backend diagnostics, or user safety narrative not intended for token viewers | Go tests for token-scoped viewer payload filtering; route tests for invalid/expired/revoked token behavior; tests proving sensitive fields are not returned; documentation review for emergency-reliance wording; later web-client tests for responsive map fallback and navigation links |
| 4 | Viewer replacement routing plan | Medium | Design/deployment docs | `/i` and `/e` routes, deployment docs, web-client docs | Stable viewer API | No broad public `/v1` exposure | Do not expose deployment-private hostnames | Manual link and redirect behavior review, if implemented |
| 5 | Server-rendered viewer compatibility window | Low | Docs/backend if later approved | Built-in viewer templates/assets, legacy aliases | Replacement routing decision | No removal without migration note | Avoid token-bearing screenshots/logs | Regression tests for aliases and public errors |
| 6 | Trusted-contact relationship lifecycle | High | Backend schema/API/design | Account model, relationship storage, invite handlers, API docs | Account identity decisions | No notification delivery or key custody change in first pass | Do not expose contact private data or safety context | Ownership/auth tests, state-transition tests |
| 7 | Trusted-contact invite acceptance UX contract | Medium | API/design | Web/mobile coordination docs, account registration flow | Relationship lifecycle | No payment entitlement or emergency dispatch | Invitation artifacts are sensitive until threat-modeled | API contract tests and docs examples without real secrets |
| 8 | Contact roles and sharing preferences | Medium | Backend model/API/docs | Trusted-contact relationship model, incident modes, sharing-state docs | Relationship lifecycle | No mode-driven behavior until designed | Role labels can reveal safety plans; keep logs minimal | State validation tests and docs review |
| 9 | Automatic client key generation and public-key publish flow | High | Client/server API design | Contact public-key routes, web/mobile docs, encryption docs | Trusted-contact account identity | No manual key creation as primary UX | Public keys are non-secret but linkable metadata | Tests for key state/version validation and replacement |
| 10 | Post-quantum recipient wrapping API | High | Crypto-adjacent API/design | `docs/post-quantum-envelope.md`, wrapped-key handlers, client protocol docs | Key lifecycle and envelope test vectors | No runtime suite advertisement before payload support | Never send private keys/shared secrets to server | Crypto test vectors, tamper tests, unknown-suite rejection |
| 11 | Grant-scoped wrapped-key delivery to authenticated contacts | High | Backend API | Sharing grants, wrapped keys, auth/session, audit docs | Trusted-contact lifecycle, active key state | No token-only wrapped-key delivery | Wrapped-key ciphertext is sensitive access material | Authz tests for owner/contact/revoked/expired grants |
| 12 | Contact key rotation and lost-device handling | High | Backend API/docs | Contact public-key state machine, grants, wrapped keys | Key publish flow | No backend recovery of private keys | Do not log replacement or recovery artifacts | Tests for active/replaced/revoked/lost transitions |
| 13 | Access history and audit read model | Medium | Backend API/docs | Viewer token use, grant reads, bundle downloads, contact reads | Clear event taxonomy | No raw token or payload logging | Audit entries must not include bearer secrets | Tests for redaction and actor classification |
| 14 | Notification delivery boundary | High | Design/backend later | Future outbox, delivery providers, contact preferences | Trusted-contact lifecycle and safe copy | No emergency-services dispatch | Messages and provider logs must not expose raw tokens unnecessarily | Dry-run tests, redaction tests, opt-out/rate-limit tests |
| 15 | Missed safety-check event model | High | Product/backend design | Incident modes, safety-check state, notifications | Incident-mode behavior design | No automatic dispatch or trusted-contact access by default | Event text can expose safety context | State-machine tests and threat-model review |
| 16 | Location/status exposure policy for viewers and contacts | Medium | Docs/API design | Viewer payload, incident metadata, web/mobile display | Viewer contract and contact roles | No live location streaming until separately scoped | Location and device status are sensitive user-safety data | Field-level tests and privacy review |
| 17 | Revocation and retention semantics for shared material | Medium | Docs/backend design | Sharing grants, wrapped keys, bundle downloads, retention docs | Grant delivery model | No promise to claw back downloaded data | Avoid naming actual recipients/incidents in public logs | Tests for future-deny behavior and docs caveats |
| 18 | Account/web auth hardening for contact APIs | High | Backend/security | Browser cookie sessions, CSRF, CORS, contact/grant routes | Contact API exposure decision | No OAuth/JWT unless separately requested | Do not log auth headers or cookies | Auth matrix tests for bearer, cookie, CSRF, mixed credentials |
| 19 | Client protocol/version coordination | Medium | Docs/protocol planning | Server docs, web-client docs, future mobile/protocol repos | Viewer and contact API contracts | No shared-protocol implementation in this repo | Avoid test fixtures with real safety data | Compatibility matrix and manual docs review |
| 20 | Decryption, restore, and playable export boundary | High | Design/security | Key custody docs, browser decryption docs, web/mobile docs | Key custody and trusted-contact delivery decisions | No backend decryption or playable server export | Never store plaintext/decrypted caches | Threat-model update and client-side proof tests before implementation |
| 21 | Admin or break-glass recovery access boundary | High | Security design | Key custody docs, break-glass docs, admin boundary docs | Explicit maintainer approval | No incidental raw server keys or public admin dashboard | Emergency-access metadata is highly sensitive | Security review, audit tests, deployment warnings |
| 22 | Built-in viewer deprecation and migration notes | Low | Docs/release planning | API docs, deployment docs, changelog | Replacement shipped and validated | No removal of legacy aliases without notice | Do not publish token-bearing examples | Manual docs/link review and compatibility tests |

## Explicitly Excluded From This Context

This context explicitly excludes:

- payments, billing, Stripe, paid registration, donations, subscriptions,
  customer portals, billing webhooks, account entitlement based on payment, and
  any registration behavior tied to payment status;
- manual end-user cryptography as the primary trusted-contact user experience;
- backend decryption, browser decryption, trusted-contact decryption, server
  key escrow, raw server-held keys, break-glass access, or dead-man-switch
  behavior unless a separate explicit security-sensitive task authorizes it;
- emergency-services dispatch or any claim that Proofline contacts emergency
  services automatically;
- live location streaming, playable export, decrypted media export, or
  server-side restore;
- public admin dashboards or moving private admin routes to public listener
  groups;
- broad public `/v1` production readiness;
- implementation in future sibling web, iOS, Android, or protocol
  repositories; and
- backlog draft creation, public GitHub issue creation, commits, or PRs.

## Open Decisions

- Whether the web-client viewer should be reached by redirecting
  `/i/{token}`, by serving a minimal shell from the server, or by giving owners a
  separate web-client URL at token creation time.
- Which viewer-token metadata fields owners need for management without
  exposing raw token material after creation.
- Whether token-only viewer downloads remain exactly bundle-only or gain a more
  granular encrypted-manifest API.
- How trusted-contact account discovery and invitation should work without
  exposing private contact books or safety plans.
- How many trusted-contact key records are allowed per account and device, and
  how lost devices, replacement devices, and inactive accounts affect wrapped
  keys.
- Which actor can receive wrapped-key ciphertext and under what grant state,
  expiry, and audit constraints.
- How notification links should be generated and redacted so provider logs do
  not become a token exposure channel.
- How access history should distinguish token viewer access, owner access,
  authenticated trusted-contact access, admin/operator access, and optional
  future emergency access.

## Source Context Inspected

This document was prepared against current server source and docs, including
`AGENTS.md`, `README.md`, `SECURITY.md`, `docs/README.md`, `docs/api.md`,
`docs/security-model.md`, `docs/threat-model.md`, `docs/encryption.md`,
`docs/post-quantum-envelope.md`, `docs/key-custody.md`,
`docs/incident-modes.md`, `docs/v1-access-control.md`,
`docs/public-api-listener-split.md`,
`docs/retention-backup-deletion.md`, `docs/contact-key-sharing-grants.md`,
`internal/httpapi`, and the incident metadata models.

Sibling web-client docs were inspected read-only for coordination context:
`../web-client/README.md`, `../web-client/SECURITY.md`,
`../web-client/docs/api-client.md`,
`../web-client/docs/security-model.md`, and
`../web-client/docs/viewer-token-ui-design.md`.
